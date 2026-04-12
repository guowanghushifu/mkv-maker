package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/guowanghushifu/mkv-maker/internal/media"
	mediabdinfo "github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
	makemkv "github.com/guowanghushifu/mkv-maker/internal/media/makemkv"
	mediampls "github.com/guowanghushifu/mkv-maker/internal/media/mpls"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type SourceScanner interface {
	Scan(root string) ([]media.SourceEntry, error)
}

type SourcesHandler struct {
	InputDir  string
	OutputDir string
	Scanner   SourceScanner
	Inspector PlaylistInspector
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
	SourceID       string                  `json:"sourceId"`
	PlaylistName   string                  `json:"playlistName"`
	OutputDir      string                  `json:"outputDir"`
	Title          string                  `json:"title"`
	DVMergeEnabled bool                    `json:"dvMergeEnabled"`
	SegmentPaths   []string                `json:"segmentPaths,omitempty"`
	Video          resolveVideo            `json:"video"`
	Audio          []resolveTrack          `json:"audio"`
	Subtitles      []resolveTrack          `json:"subtitles"`
	MakeMKV        remux.MakeMKVTitleCache `json:"makemkv"`
}

type resolveVideo struct {
	Name       string `json:"name"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution"`
	HDRType    string `json:"hdrType,omitempty"`
}

type resolveTrack struct {
	ID          string `json:"id"`
	SourceIndex int    `json:"sourceIndex"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	CodecLabel  string `json:"codecLabel,omitempty"`
	Selected    bool   `json:"selected"`
	Default     bool   `json:"default"`
	Forced      bool   `json:"forced,omitempty"`
}

type PlaylistInspector interface {
	Inspect(ctx context.Context, sourcePath, playlistPath string) (MakeMKVInspection, error)
}

type MakeMKVInspection struct {
	TitleID      int
	PlaylistName string
	Audio        []resolveTrack
	Subtitles    []resolveTrack
	Cache        remux.MakeMKVTitleCache
}

var playlistNamePattern = regexp.MustCompile(`(?i)^\d{5}\.MPLS$`)

const resolveSourceBodyLimit = 2 << 20

func NewSourcesHandler(inputDir, outputDir string, scanner SourceScanner, inspector PlaylistInspector) *SourcesHandler {
	if scanner == nil {
		scanner = media.NewScanner()
	}
	if inspector == nil {
		inspector = MakeMKVPlaylistInspector{}
	}
	return &SourcesHandler{
		InputDir:  inputDir,
		OutputDir: outputDir,
		Scanner:   scanner,
		Inspector: inspector,
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
		log.Printf("resolve: missing source id in route")
		http.Error(w, "missing source id", http.StatusBadRequest)
		return
	}
	normalizedPathSourceID := normalizeSourceID(pathSourceID)

	var req resolveSourceRequest
	if !decodeJSONBodyLimited(w, r, resolveSourceBodyLimit, &req) {
		return
	}
	if normalizedBodySourceID := normalizeSourceID(req.SourceID); normalizedBodySourceID != "" && normalizedBodySourceID != normalizedPathSourceID {
		logResolveFailure(pathSourceID, req.BDInfo.PlaylistName, "source id mismatch bodySourceID=%s", req.SourceID)
		http.Error(w, "source id mismatch", http.StatusBadRequest)
		return
	}

	sources, err := h.Scanner.Scan(h.InputDir)
	if err != nil {
		logResolveFailure(pathSourceID, req.BDInfo.PlaylistName, "scan failed: %v", err)
		h.writeScanError(w, err)
		return
	}
	source, ok := findSourceByID(sources, normalizedPathSourceID)
	if !ok {
		logResolveFailure(pathSourceID, req.BDInfo.PlaylistName, "source not found")
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	if !isPathWithinRoot(h.InputDir, source.Path) {
		logResolveFailure(pathSourceID, req.BDInfo.PlaylistName, "source path is outside input root path=%s inputRoot=%s", source.Path, h.InputDir)
		http.Error(w, "source path is outside input root", http.StatusBadRequest)
		return
	}
	sourcePath := source.Path
	switch source.Type {
	case media.SourceISO:
		// ISO sources are inspected directly through MakeMKV.
	case media.SourceBDMV:
		// already rooted at the BDMV source directory
	default:
		logResolveFailure(pathSourceID, req.BDInfo.PlaylistName, "unsupported source type=%s", source.Type)
		http.Error(w, "only bdmv and iso sources are supported", http.StatusBadRequest)
		return
	}

	parsed, err := mediabdinfo.Parse(req.BDInfo.RawText)
	if err != nil {
		logResolveFailure(pathSourceID, req.BDInfo.PlaylistName, "bdinfo parse failed: %v", err)
		http.Error(w, "invalid bdinfo payload: "+bdinfoParseErrorMessage(err), http.StatusBadRequest)
		return
	}

	playlistName := strings.ToUpper(strings.TrimSpace(parsed.PlaylistName))
	if requested := strings.ToUpper(strings.TrimSpace(req.BDInfo.PlaylistName)); requested != "" && requested != playlistName {
		logResolveFailure(pathSourceID, playlistName, "bdinfo playlist mismatch requested=%s parsed=%s", requested, playlistName)
		http.Error(w, "bdinfo playlist mismatch", http.StatusBadRequest)
		return
	}
	playlistName = strings.ToUpper(filepath.Base(playlistName))
	if !playlistNamePattern.MatchString(playlistName) {
		logResolveFailure(pathSourceID, playlistName, "invalid playlist name")
		http.Error(w, "invalid playlist name", http.StatusBadRequest)
		return
	}
	playlistRef := playlistName
	segmentPaths := []string{}
	if source.Type == media.SourceBDMV {
		playlistPath, err := findPlaylistFilePath(sourcePath, playlistName)
		if err != nil {
			log.Printf("resolve source=%s playlist=%s: playlist lookup failed: %v", sourcePath, playlistName, err)
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to validate playlist", http.StatusInternalServerError)
			return
		}
		playlistRef = playlistPath
		clipNames, err := mediampls.ParseClipNames(playlistPath)
		if err != nil {
			log.Printf("resolve source=%s playlist=%s playlistPath=%s: mpls parse failed: %v", sourcePath, playlistName, playlistPath, err)
		}
		if len(clipNames) == 0 {
			clipNames = parsed.StreamFiles
		}
		segmentPaths = buildSegmentPaths(sourcePath, clipNames)
	}
	inspection, err := h.Inspector.Inspect(r.Context(), sourcePath, playlistRef)
	if err != nil {
		log.Printf(
			"resolve source=%s playlist=%s playlistRef=%s segments=%q: playlist inspection failed: %v",
			sourcePath,
			playlistName,
			playlistRef,
			segmentPaths,
			err,
		)
		if source.Type == media.SourceBDMV && len(segmentPaths) == 0 {
			http.Error(w, "failed to inspect playlist tracks: no stream files resolved from playlist", http.StatusInternalServerError)
			return
		}
		http.Error(w, "failed to inspect playlist tracks", http.StatusInternalServerError)
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

	audioLabels := parsed.AudioLabels
	if hasNonEmptyLabels(req.BDInfo.AudioLabels) {
		audioLabels = req.BDInfo.AudioLabels
	}
	subtitleLabels := parsed.SubtitleLabels
	if hasNonEmptyLabels(req.BDInfo.SubtitleLabels) {
		subtitleLabels = req.BDInfo.SubtitleLabels
	}

	audio := overlayResolveTrackNames(inspection.Audio, audioLabels)
	subtitles := overlayResolveTrackNames(inspection.Subtitles, subtitleLabels)

	response := resolveSourceResponse{
		SourceID:       source.ID,
		PlaylistName:   playlistName,
		OutputDir:      fallbackString(h.OutputDir, "/remux"),
		Title:          title,
		DVMergeEnabled: dvMergeEnabled,
		SegmentPaths:   segmentPaths,
		Video:          video,
		Audio:          audio,
		Subtitles:      subtitles,
		MakeMKV:        buildRawResolveMakeMKVCache(inspection, playlistName),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logResolveFailure(pathSourceID, playlistName, "failed to encode response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func logResolveFailure(sourceID, playlistName, format string, args ...any) {
	logArgs := append([]any{sourceID, strings.ToUpper(strings.TrimSpace(playlistName))}, args...)
	log.Printf("resolve sourceID=%s playlist=%s: "+format, logArgs...)
}

func (h *SourcesHandler) writeScannedSources(w http.ResponseWriter) {
	items, err := h.Scanner.Scan(h.InputDir)
	if err != nil {
		h.writeScanError(w, err)
		return
	}
	if items == nil {
		items = []media.SourceEntry{}
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
	normalizedID := normalizeSourceID(id)
	for _, source := range sources {
		if normalizeSourceID(source.ID) == normalizedID {
			return source, true
		}
	}
	return media.SourceEntry{}, false
}

func normalizeSourceID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	decoded, err := url.PathUnescape(trimmed)
	if err != nil {
		return trimmed
	}
	return decoded
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

func hasNonEmptyLabels(labels []string) bool {
	for _, label := range labels {
		if strings.TrimSpace(label) != "" {
			return true
		}
	}
	return false
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

func makeMKVSourceArg(sourcePath string) string {
	trimmed := strings.TrimSpace(sourcePath)
	if strings.EqualFold(filepath.Base(trimmed), "BDMV") {
		return "file:" + filepath.Dir(trimmed)
	}
	return "file:" + trimmed
}

type MakeMKVPlaylistInspector struct {
	Binary       string
	ExpireDate   *time.Time
	dateOverride makemkv.CommandDateOverride
}

func (i MakeMKVPlaylistInspector) Inspect(ctx context.Context, sourcePath, playlistPath string) (MakeMKVInspection, error) {
	binary := strings.TrimSpace(i.Binary)
	if binary == "" {
		binary = "/opt/makemkv/bin/makemkvcon"
	}
	override := i.dateOverride
	if !override.IsConfigured() {
		override = makemkv.NewCommandDateOverride(i.ExpireDate)
	}

	output, err := makemkv.RunWithCommandDateOverride(override, ctx, func(runCtx context.Context) ([]byte, error) {
		return exec.CommandContext(runCtx, binary, "info", makeMKVSourceArg(sourcePath), "--robot").Output()
	})
	if err != nil {
		return MakeMKVInspection{}, err
	}

	parsed, err := makemkv.ParseRobotOutput(output)
	if err != nil {
		return MakeMKVInspection{}, err
	}

	title, err := parsed.TitleByPlaylist(filepath.Base(strings.TrimSpace(playlistPath)))
	if err != nil {
		return MakeMKVInspection{}, err
	}

	view, err := makemkv.BuildTitleView(title)
	if err != nil {
		return MakeMKVInspection{}, err
	}

	inspection := MakeMKVInspection{
		TitleID:      view.TitleID,
		PlaylistName: view.PlaylistName,
		Audio:        makeResolveAudioTracks(view.Audio),
		Subtitles:    makeResolveSubtitleTracks(view.Subtitles),
	}
	inspection.Cache = remux.MakeMKVTitleCache{
		PlaylistName: inspection.PlaylistName,
		TitleID:      inspection.TitleID,
		Audio:        makeCacheAudioTracks(inspection.Audio),
		Subtitles:    makeCacheSubtitleTracks(inspection.Subtitles),
	}
	return inspection, nil
}

func makeResolveAudioTracks(tracks []makemkv.VisibleTrack) []resolveTrack {
	resolved := make([]resolveTrack, 0, len(tracks))
	for _, track := range tracks {
		resolved = append(resolved, resolveTrack{
			ID:          track.ID,
			SourceIndex: track.SourceIndex,
			Name:        track.Name,
			Language:    track.Language,
			CodecLabel:  track.CodecLabel,
			Selected:    track.Selected,
			Default:     track.Default,
		})
	}
	return resolved
}

func makeResolveSubtitleTracks(tracks []makemkv.VisibleTrack) []resolveTrack {
	resolved := make([]resolveTrack, 0, len(tracks))
	for _, track := range tracks {
		resolved = append(resolved, resolveTrack{
			ID:          track.ID,
			SourceIndex: track.SourceIndex,
			Name:        track.Name,
			Language:    track.Language,
			Selected:    track.Selected,
			Default:     track.Default,
			Forced:      track.Forced,
		})
	}
	return resolved
}

func overlayResolveTrackNames(base []resolveTrack, labels []string) []resolveTrack {
	tracks := append([]resolveTrack{}, base...)
	for i := range tracks {
		if i < len(labels) && strings.TrimSpace(labels[i]) != "" {
			tracks[i].Name = strings.TrimSpace(labels[i])
		}
	}
	return tracks
}

func buildRawResolveMakeMKVCache(inspection MakeMKVInspection, playlistName string) remux.MakeMKVTitleCache {
	cache := inspection.Cache
	cache.PlaylistName = firstNonEmpty(cache.PlaylistName, inspection.PlaylistName, playlistName)
	cache.TitleID = inspection.TitleID
	if cache.Audio == nil {
		cache.Audio = makeCacheAudioTracks(inspection.Audio)
	}
	if cache.Subtitles == nil {
		cache.Subtitles = makeCacheSubtitleTracks(inspection.Subtitles)
	}
	if cache.Audio == nil {
		cache.Audio = []remux.AudioTrack{}
	}
	if cache.Subtitles == nil {
		cache.Subtitles = []remux.SubtitleTrack{}
	}
	return cache
}

func makeCacheAudioTracks(tracks []resolveTrack) []remux.AudioTrack {
	resolved := make([]remux.AudioTrack, 0, len(tracks))
	for _, track := range tracks {
		resolved = append(resolved, remux.AudioTrack{
			ID:          track.ID,
			SourceIndex: track.SourceIndex,
			Name:        track.Name,
			Language:    track.Language,
			CodecLabel:  track.CodecLabel,
			Default:     track.Default,
			Selected:    track.Selected,
		})
	}
	return resolved
}

func makeCacheSubtitleTracks(tracks []resolveTrack) []remux.SubtitleTrack {
	resolved := make([]remux.SubtitleTrack, 0, len(tracks))
	for _, track := range tracks {
		resolved = append(resolved, remux.SubtitleTrack{
			ID:          track.ID,
			SourceIndex: track.SourceIndex,
			Name:        track.Name,
			Language:    track.Language,
			Default:     track.Default,
			Selected:    track.Selected,
			Forced:      track.Forced,
		})
	}
	return resolved
}

func inferLanguage(label string) string {
	if normalized := normalizeLanguageCode(label); normalized != "" {
		return normalized
	}
	return "und"
}

func normalizeLanguageCode(value string) string {
	upper := strings.ToUpper(value)
	tokens := languageTokens(upper)
	switch {
	case strings.Contains(value, "中"), hasLanguageToken(tokens, "CHINESE", "MANDARIN", "CANTONESE", "CHI", "ZHO"):
		return "chi"
	case strings.Contains(value, "日"), hasLanguageToken(tokens, "JAPANESE", "JPN"):
		return "jpn"
	case strings.Contains(value, "韩"), strings.Contains(value, "韓"), hasLanguageToken(tokens, "KOREAN", "KOR"):
		return "kor"
	case strings.Contains(value, "葡"), hasLanguageToken(tokens, "PORTUGUESE", "PORTUGUES", "PORTUGUESA", "POR"):
		return "por"
	case strings.Contains(value, "俄"), hasLanguageToken(tokens, "RUSSIAN", "RUS", "РУССКИЙ"):
		return "rus"
	case strings.Contains(value, "英"), hasLanguageToken(tokens, "ENGLISH", "ENG"):
		return "eng"
	case hasLanguageToken(tokens, "FRENCH", "FRE", "FRA", "FRANCAIS"):
		return "fre"
	case hasLanguageToken(tokens, "GERMAN", "DEUTSCH", "GER", "DEU"):
		return "ger"
	case hasLanguageToken(tokens, "ITALIAN", "ITALIANO", "ITA"):
		return "ita"
	case hasLanguageToken(tokens, "DUTCH", "NEDERLANDS", "DUT", "NLD"):
		return "dut"
	case hasLanguageToken(tokens, "SPANISH", "SPA", "ESPANOL"):
		return "spa"
	default:
		return ""
	}
}

func languageTokens(value string) map[string]struct{} {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		tokens[part] = struct{}{}
	}
	return tokens
}

func hasLanguageToken(tokens map[string]struct{}, candidates ...string) bool {
	for _, candidate := range candidates {
		if _, ok := tokens[candidate]; ok {
			return true
		}
	}
	return false
}
