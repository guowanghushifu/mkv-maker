package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/guowanghushifu/mkv-maker/internal/isomount"
	"github.com/guowanghushifu/mkv-maker/internal/media"
	mediabdinfo "github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
	mediampls "github.com/guowanghushifu/mkv-maker/internal/media/mpls"
)

type SourceScanner interface {
	Scan(root string) ([]media.SourceEntry, error)
}

type SourcesHandler struct {
	InputDir   string
	OutputDir  string
	Scanner    SourceScanner
	Inspector  PlaylistInspector
	ISOManager *isomount.Manager
}

type sourcesErrorResponse struct {
	Error sourcesErrorDetail `json:"error"`
}

type sourcesErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type resolveSourceRequest struct {
	SourceID string             `json:"sourceId"`
	BDInfo   resolveBDInfoInput `json:"bdinfo"`
}

type resolveBDInfoInput struct {
	PlaylistName   string   `json:"playlistName"`
	DiscTitle      string   `json:"discTitle"`
	AudioLabels    []string `json:"audioLabels"`
	SubtitleLabels []string `json:"subtitleLabels"`
	RawText        string   `json:"rawText"`
}

type resolveSourceResponse struct {
	SourceID       string         `json:"sourceId"`
	PlaylistName   string         `json:"playlistName"`
	OutputDir      string         `json:"outputDir"`
	Title          string         `json:"title"`
	DVMergeEnabled bool           `json:"dvMergeEnabled"`
	SegmentPaths   []string       `json:"segmentPaths,omitempty"`
	Video          resolveVideo   `json:"video"`
	Audio          []resolveTrack `json:"audio"`
	Subtitles      []resolveTrack `json:"subtitles"`
}

type resolveVideo struct {
	Name       string `json:"name"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution"`
	HDRType    string `json:"hdrType,omitempty"`
}

type resolveTrack struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Language   string `json:"language"`
	CodecLabel string `json:"codecLabel,omitempty"`
	Selected   bool   `json:"selected"`
	Default    bool   `json:"default"`
	Forced     bool   `json:"forced,omitempty"`
}

type PlaylistInspector interface {
	Inspect(playlistPath string) (PlaylistInspection, error)
}

type PlaylistInspection struct {
	AudioTrackIDs     []string
	AudioLanguages    []string
	SubtitleTrackIDs  []string
	SubtitleLanguages []string
}

var playlistNamePattern = regexp.MustCompile(`(?i)^\d{5}\.MPLS$`)

const resolveSourceBodyLimit = 2 << 20

func NewSourcesHandler(inputDir, outputDir string, scanner SourceScanner, inspector PlaylistInspector, isoManager ...*isomount.Manager) *SourcesHandler {
	if scanner == nil {
		scanner = media.NewScanner(filepath.Join(inputDir, "iso_auto_mount"), true)
	}
	if inspector == nil {
		inspector = MKVMergePlaylistInspector{}
	}
	var manager *isomount.Manager
	if len(isoManager) > 0 {
		manager = isoManager[0]
	}
	return &SourcesHandler{
		InputDir:   inputDir,
		OutputDir:  outputDir,
		Scanner:    scanner,
		Inspector:  inspector,
		ISOManager: manager,
	}
}

func (h *SourcesHandler) Scan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.writeScannedSources(w)
}

func (h *SourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.writeScannedSources(w)
}

func (h *SourcesHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pathSourceID := chi.URLParam(r, "id")
	if strings.TrimSpace(pathSourceID) == "" {
		http.Error(w, "missing source id", http.StatusBadRequest)
		return
	}

	var req resolveSourceRequest
	if !decodeJSONBodyLimited(w, r, resolveSourceBodyLimit, &req) {
		return
	}
	if strings.TrimSpace(req.SourceID) != "" && req.SourceID != pathSourceID {
		http.Error(w, "source id mismatch", http.StatusBadRequest)
		return
	}

	sources, err := h.Scanner.Scan(h.InputDir)
	if err != nil {
		h.writeScanError(w, err)
		return
	}
	source, ok := findSourceByID(sources, pathSourceID)
	if !ok {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	if source.Type != media.SourceBDMV {
		http.Error(w, "only bdmv sources are supported", http.StatusBadRequest)
		return
	}
	if !isPathWithinRoot(h.InputDir, source.Path) {
		http.Error(w, "source path is outside input root", http.StatusBadRequest)
		return
	}

	parsed, err := mediabdinfo.Parse(req.BDInfo.RawText)
	if err != nil {
		http.Error(w, "invalid bdinfo payload", http.StatusBadRequest)
		return
	}

	playlistName := strings.ToUpper(strings.TrimSpace(parsed.PlaylistName))
	if requested := strings.ToUpper(strings.TrimSpace(req.BDInfo.PlaylistName)); requested != "" && requested != playlistName {
		http.Error(w, "bdinfo playlist mismatch", http.StatusBadRequest)
		return
	}
	playlistName = strings.ToUpper(filepath.Base(playlistName))
	if !playlistNamePattern.MatchString(playlistName) {
		http.Error(w, "invalid playlist name", http.StatusBadRequest)
		return
	}
	playlistPath, err := findPlaylistFilePath(source.Path, playlistName)
	if err != nil {
		log.Printf("resolve source=%s playlist=%s: playlist lookup failed: %v", source.Path, playlistName, err)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to validate playlist", http.StatusInternalServerError)
		return
	}
	clipNames, err := mediampls.ParseClipNames(playlistPath)
	if err != nil {
		log.Printf("resolve source=%s playlist=%s playlistPath=%s: mpls parse failed: %v", source.Path, playlistName, playlistPath, err)
	}
	if len(clipNames) == 0 {
		clipNames = parsed.StreamFiles
	}
	segmentPaths := buildSegmentPaths(source.Path, clipNames)
	inspection, err := h.Inspector.Inspect(playlistPath)
	if err != nil {
		log.Printf(
			"resolve source=%s playlist=%s playlistPath=%s segments=%q: playlist inspection failed: %v",
			source.Path,
			playlistName,
			playlistPath,
			segmentPaths,
			err,
		)
		if len(segmentPaths) == 0 {
			http.Error(w, "failed to inspect playlist tracks: no stream files resolved from playlist", http.StatusInternalServerError)
			return
		}
		http.Error(w, "failed to inspect playlist tracks", http.StatusInternalServerError)
		return
	}

	audioLabels := compactLabels(req.BDInfo.AudioLabels)
	if len(audioLabels) == 0 {
		audioLabels = compactLabels(parsed.AudioLabels)
	}
	subtitleLabels := compactLabels(req.BDInfo.SubtitleLabels)
	if len(subtitleLabels) == 0 {
		subtitleLabels = compactLabels(parsed.SubtitleLabels)
	}
	if err := validateResolvedTrackIDs(audioLabels, inspection.AudioTrackIDs, "audio"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateResolvedTrackIDs(subtitleLabels, inspection.SubtitleTrackIDs, "subtitle"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	video := resolveVideo{
		Name:       "Main Video",
		Codec:      fallbackString(parsed.Video.Codec, "HEVC"),
		Resolution: fallbackString(parsed.Video.Resolution, "2160p"),
		HDRType:    parsed.Video.HDRType,
	}
	title := firstNonEmpty(req.BDInfo.DiscTitle, parsed.DiscTitle, source.Name)
	dvMergeEnabled := parsed.DVMergeEnabled || strings.Contains(strings.ToUpper(parsed.Video.HDRType), "DV")

	response := resolveSourceResponse{
		SourceID:       source.ID,
		PlaylistName:   playlistName,
		OutputDir:      fallbackString(h.OutputDir, "/remux"),
		Title:          title,
		DVMergeEnabled: dvMergeEnabled,
		SegmentPaths:   segmentPaths,
		Video:          video,
		Audio:          buildResolveTracks(audioLabels, parsed.AudioLanguages, inspection.AudioLanguages, parsed.AudioCodecInfo, inspection.AudioTrackIDs, false),
		Subtitles:      buildResolveTracks(subtitleLabels, parsed.SubtitleLanguages, inspection.SubtitleLanguages, nil, inspection.SubtitleTrackIDs, true),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *SourcesHandler) writeScannedSources(w http.ResponseWriter) {
	items, err := h.Scanner.Scan(h.InputDir)
	if err != nil {
		h.writeScanError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *SourcesHandler) writeScanError(w http.ResponseWriter, err error) {
	response := sourcesErrorResponse{
		Error: sourcesErrorDetail{
			Code:    "scan_failed",
			Message: "failed to scan sources",
		},
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		response.Error.Path = pathErr.Path
	}

	switch {
	case errors.Is(err, os.ErrNotExist):
		response.Error.Code = "input_dir_not_found"
		response.Error.Message = "input directory does not exist"
		if response.Error.Path == "" {
			response.Error.Path = h.InputDir
		}
	case errors.Is(err, os.ErrPermission):
		response.Error.Code = "input_dir_unreadable"
		response.Error.Message = "input directory is not readable"
		if response.Error.Path == "" {
			response.Error.Path = h.InputDir
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(response)
}

func findSourceByID(sources []media.SourceEntry, id string) (media.SourceEntry, bool) {
	for _, source := range sources {
		if source.ID == id {
			return source, true
		}
	}
	return media.SourceEntry{}, false
}

func findPlaylistFilePath(sourcePath, playlistName string) (string, error) {
	playlistDir := filepath.Join(sourcePath, "BDMV", "PLAYLIST")
	exactPath := filepath.Join(playlistDir, playlistName)
	if _, err := os.Stat(exactPath); err == nil {
		return exactPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), playlistName) {
			return filepath.Join(playlistDir, entry.Name()), nil
		}
	}
	return "", os.ErrNotExist
}

func findStreamFilePath(sourcePath, streamName string) (string, error) {
	streamDir := filepath.Join(sourcePath, "BDMV", "STREAM")
	exactPath := filepath.Join(streamDir, streamName)
	if _, err := os.Stat(exactPath); err == nil {
		return exactPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	entries, err := os.ReadDir(streamDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), streamName) {
			return filepath.Join(streamDir, entry.Name()), nil
		}
	}
	return "", os.ErrNotExist
}

func compactLabels(labels []string) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			out = append(out, label)
		}
	}
	return out
}

func fallbackString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func buildResolveTracks(labels []string, languages []string, fallbackLanguages []string, codecLabels []string, trackIDs []string, subtitles bool) []resolveTrack {
	tracks := make([]resolveTrack, 0, len(labels))
	for i, label := range labels {
		trackID := strconv.Itoa(i + 1)
		if i < len(trackIDs) && strings.TrimSpace(trackIDs[i]) != "" {
			trackID = strings.TrimSpace(trackIDs[i])
		}
		codecLabel := ""
		if i < len(codecLabels) {
			codecLabel = strings.TrimSpace(codecLabels[i])
		}
		if codecLabel == "" && !subtitles {
			codecLabel = normalizeCodecLabel(label)
		}
		language := ""
		if i < len(languages) {
			language = normalizeLanguageCode(languages[i])
		}
		if language == "" && i < len(fallbackLanguages) {
			language = normalizeLanguageCode(fallbackLanguages[i])
		}
		if language == "" {
			language = inferLanguage(label)
		}
		track := resolveTrack{
			ID:         trackID,
			Name:       label,
			Language:   language,
			CodecLabel: codecLabel,
			Selected:   true,
			Default:    i == 0,
		}
		if subtitles {
			track.Forced = strings.Contains(strings.ToLower(label), "forced")
		}
		tracks = append(tracks, track)
	}
	return tracks
}

func validateResolvedTrackIDs(labels []string, trackIDs []string, kind string) error {
	if len(labels) == 0 {
		return nil
	}
	if len(trackIDs) < len(labels) {
		return errors.New(kind + " track ids are incomplete")
	}
	for i := range labels {
		if strings.TrimSpace(trackIDs[i]) == "" {
			return errors.New(kind + " track ids are incomplete")
		}
	}
	return nil
}

func buildSegmentPaths(sourceRoot string, streamFiles []string) []string {
	paths := make([]string, 0, len(streamFiles))
	for _, name := range streamFiles {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if filepath.Ext(name) == "" {
			name += ".m2ts"
		}
		path, err := findStreamFilePath(sourceRoot, name)
		if err != nil {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

type MKVMergePlaylistInspector struct {
	Binary string
}

type mkvmergeTrack struct {
	ID         int                     `json:"id"`
	Type       string                  `json:"type"`
	Codec      string                  `json:"codec"`
	Properties mkvmergeTrackProperties `json:"properties"`
}

type mkvmergeTrackProperties struct {
	AudioChannels     int    `json:"audio_channels"`
	Language          string `json:"language"`
	Number            int    `json:"number"`
	StreamID          int    `json:"stream_id"`
	MultiplexedTracks []int  `json:"multiplexed_tracks"`
}

type mkvmergeIdentifyPayload struct {
	Tracks []mkvmergeTrack `json:"tracks"`
}

func (i MKVMergePlaylistInspector) Inspect(playlistPath string) (PlaylistInspection, error) {
	binary := strings.TrimSpace(i.Binary)
	if binary == "" {
		binary = "mkvmerge"
	}

	output, err := exec.Command(binary, "-J", playlistPath).Output()
	if err != nil {
		return PlaylistInspection{}, err
	}

	var payload mkvmergeIdentifyPayload
	if err := json.Unmarshal(output, &payload); err != nil {
		return PlaylistInspection{}, err
	}

	audioSelections, subtitleSelections := collectTrackSelections(payload)
	return PlaylistInspection{
		AudioTrackIDs:     collectSelectionIDs(audioSelections),
		AudioLanguages:    collectSelectionLanguages(audioSelections),
		SubtitleTrackIDs:  collectSelectionIDs(subtitleSelections),
		SubtitleLanguages: collectSelectionLanguages(subtitleSelections),
	}, nil
}

func collectTrackIDs(payload mkvmergeIdentifyPayload) ([]string, []string) {
	audioSelections, subtitleSelections := collectTrackSelections(payload)
	return collectSelectionIDs(audioSelections), collectSelectionIDs(subtitleSelections)
}

type trackSelection struct {
	ID       string
	Language string
}

func collectTrackSelections(payload mkvmergeIdentifyPayload) ([]trackSelection, []trackSelection) {
	audioSelections := make([]trackSelection, 0, len(payload.Tracks))
	subtitleSelections := make([]trackSelection, 0, len(payload.Tracks))
	groupBest := make(map[string]mkvmergeTrack, len(payload.Tracks))
	audioDirect := make(map[string]trackSelection, len(payload.Tracks))
	audioOrder := make([]string, 0, len(payload.Tracks))

	for _, track := range payload.Tracks {
		switch strings.ToLower(strings.TrimSpace(track.Type)) {
		case "audio":
			if key := multiplexedGroupKey(track); key != "" {
				if _, ok := groupBest[key]; !ok {
					audioOrder = append(audioOrder, key)
					groupBest[key] = track
					continue
				}
				if shouldPreferMultiplexedTrack(track, groupBest[key]) {
					groupBest[key] = track
				}
				continue
			}
			key := strconv.Itoa(track.ID)
			audioOrder = append(audioOrder, key)
			audioDirect[key] = trackSelection{
				ID:       strconv.Itoa(track.ID),
				Language: strings.TrimSpace(track.Properties.Language),
			}
		case "subtitles":
			subtitleSelections = append(subtitleSelections, trackSelection{
				ID:       strconv.Itoa(track.ID),
				Language: strings.TrimSpace(track.Properties.Language),
			})
		}
	}

	for _, entry := range audioOrder {
		if track, ok := groupBest[entry]; ok {
			audioSelections = append(audioSelections, trackSelection{
				ID:       strconv.Itoa(track.ID),
				Language: strings.TrimSpace(track.Properties.Language),
			})
			continue
		}
		if selection, ok := audioDirect[entry]; ok {
			audioSelections = append(audioSelections, selection)
		}
	}
	return audioSelections, subtitleSelections
}

func collectSelectionIDs(selections []trackSelection) []string {
	ids := make([]string, 0, len(selections))
	for _, selection := range selections {
		ids = append(ids, selection.ID)
	}
	return ids
}

func collectSelectionLanguages(selections []trackSelection) []string {
	languages := make([]string, 0, len(selections))
	for _, selection := range selections {
		languages = append(languages, selection.Language)
	}
	return languages
}

func multiplexedGroupKey(track mkvmergeTrack) string {
	if len(track.Properties.MultiplexedTracks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(track.Properties.MultiplexedTracks)+2)
	if track.Properties.Number > 0 {
		parts = append(parts, strconv.Itoa(track.Properties.Number))
	}
	if track.Properties.StreamID > 0 {
		parts = append(parts, strconv.Itoa(track.Properties.StreamID))
	}
	for _, id := range track.Properties.MultiplexedTracks {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ":")
}

func shouldPreferMultiplexedTrack(candidate, current mkvmergeTrack) bool {
	if candidate.Properties.AudioChannels != current.Properties.AudioChannels {
		return candidate.Properties.AudioChannels > current.Properties.AudioChannels
	}
	return false
}

func normalizeCodecLabel(label string) string {
	builder := strings.Builder{}
	lastDot := false
	for _, r := range label {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDot = false
		case r == '+' || r == '-' || r == '.':
			builder.WriteRune(r)
			lastDot = r == '.'
		case unicode.IsSpace(r) || r == '/' || r == '_' || r == ':':
			if !lastDot && builder.Len() > 0 {
				builder.WriteRune('.')
				lastDot = true
			}
		}
	}
	value := strings.Trim(builder.String(), ".")
	if value == "" {
		return ""
	}
	return value
}

func inferLanguage(label string) string {
	if normalized := normalizeLanguageCode(label); normalized != "" {
		return normalized
	}
	return "und"
}

func normalizeLanguageCode(value string) string {
	upper := strings.ToUpper(value)
	switch {
	case strings.Contains(value, "中"), strings.Contains(upper, "CHINESE"), strings.Contains(upper, "MANDARIN"), strings.Contains(upper, "CANTONESE"), strings.Contains(upper, "CHI"), strings.Contains(upper, "ZHO"):
		return "chi"
	case strings.Contains(value, "日"), strings.Contains(upper, "JAPANESE"), strings.Contains(upper, "JPN"):
		return "jpn"
	case strings.Contains(value, "英"), strings.Contains(upper, "ENGLISH"), strings.Contains(upper, "ENG"):
		return "eng"
	case strings.Contains(upper, "FRENCH"), strings.Contains(upper, "FRE"), strings.Contains(upper, "FRA"), strings.Contains(upper, "FRANCAIS"):
		return "fre"
	case strings.Contains(upper, "SPANISH"), strings.Contains(upper, "SPA"), strings.Contains(upper, "ESPANOL"):
		return "spa"
	default:
		return ""
	}
}
