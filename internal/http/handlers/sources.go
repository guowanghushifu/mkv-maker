package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/wangdazhuo/mkv-maker/internal/media"
	mediabdinfo "github.com/wangdazhuo/mkv-maker/internal/media/bdinfo"
)

type SourceScanner interface {
	Scan(root string) ([]media.SourceEntry, error)
}

type SourcesHandler struct {
	InputDir  string
	OutputDir string
	Scanner   SourceScanner
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

var playlistNamePattern = regexp.MustCompile(`(?i)^\d{5}\.MPLS$`)

func NewSourcesHandler(inputDir, outputDir string, scanner SourceScanner) *SourcesHandler {
	if scanner == nil {
		scanner = media.NewScanner()
	}
	return &SourcesHandler{
		InputDir:  inputDir,
		OutputDir: outputDir,
		Scanner:   scanner,
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
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
	playlistPath := filepath.Join(source.Path, "BDMV", "PLAYLIST", playlistName)
	if _, err := os.Stat(playlistPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to validate playlist", http.StatusInternalServerError)
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
		Video:          video,
		Audio:          buildResolveTracks(audioLabels, false, 0),
		Subtitles:      buildResolveTracks(subtitleLabels, true, len(audioLabels)),
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

func isPathWithinRoot(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
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

func buildResolveTracks(labels []string, subtitles bool, baseIndex int) []resolveTrack {
	tracks := make([]resolveTrack, 0, len(labels))
	for i, label := range labels {
		track := resolveTrack{
			ID:         strconv.Itoa(baseIndex + i + 1),
			Name:       label,
			Language:   inferLanguage(label),
			CodecLabel: normalizeCodecLabel(label),
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
	upper := strings.ToUpper(label)
	switch {
	case strings.Contains(label, "中"), strings.Contains(upper, "CHINESE"), strings.Contains(upper, "MANDARIN"):
		return "chi"
	case strings.Contains(label, "日"), strings.Contains(upper, "JAPANESE"):
		return "jpn"
	case strings.Contains(label, "英"), strings.Contains(upper, "ENGLISH"):
		return "eng"
	default:
		return "und"
	}
}
