package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/guowanghushifu/mkv-maker/internal/media"
	mediabdinfo "github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type JobsHandler struct {
	Tasks     tasksManager
	InputDir  string
	OutputDir string
	Scanner   SourceScanner
}

type tasksManager interface {
	Start(req remux.StartRequest) (remux.Task, error)
	Current() (remux.Task, error)
	CurrentLog() (string, error)
	StopCurrent() error
}

type createJobRequest struct {
	Source struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"source"`
	BDInfo struct {
		PlaylistName string `json:"playlistName"`
		RawText      string `json:"rawText"`
	} `json:"bdinfo"`
	Draft struct {
		PlaylistName string `json:"playlistName"`
	} `json:"draft"`
	OutputFilename string `json:"outputFilename"`
	OutputPath     string `json:"outputPath"`
}

const createJobBodyLimit = 2 << 20

func NewJobsHandler(tasks tasksManager, inputDir, outputDir string, deps ...any) *JobsHandler {
	var scanner SourceScanner
	for _, dep := range deps {
		if v, ok := dep.(SourceScanner); ok {
			scanner = v
		}
	}
	return &JobsHandler{
		Tasks:     tasks,
		InputDir:  strings.TrimSpace(inputDir),
		OutputDir: strings.TrimSpace(outputDir),
		Scanner:   scanner,
	}
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, createJobBodyLimit)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var req createJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	sourceType := strings.ToLower(strings.TrimSpace(req.Source.Type))
	if sourceType != "" && sourceType != "bdmv" && sourceType != "iso" {
		http.Error(w, "only bdmv sources are supported", http.StatusBadRequest)
		return
	}
	if sourceType != "iso" && !isPathWithinRoot(h.InputDir, req.Source.Path) {
		http.Error(w, "source path is outside input root", http.StatusBadRequest)
		return
	}
	if err := validateNewOutputPathWithinRoot(h.OutputDir, req.OutputPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsed, err := mediabdinfo.Parse(req.BDInfo.RawText)
	if err != nil {
		http.Error(w, "invalid bdinfo payload", http.StatusBadRequest)
		return
	}

	playlistName := strings.ToUpper(strings.TrimSpace(parsed.PlaylistName))
	if playlistName == "" {
		http.Error(w, "missing playlist name", http.StatusBadRequest)
		return
	}
	playlistName = strings.ToUpper(filepath.Base(playlistName))
	if !playlistNamePattern.MatchString(playlistName) {
		http.Error(w, "invalid playlist name", http.StatusBadRequest)
		return
	}
	if requested := strings.ToUpper(strings.TrimSpace(req.BDInfo.PlaylistName)); requested != "" && requested != playlistName {
		http.Error(w, "bdinfo playlist mismatch", http.StatusBadRequest)
		return
	}
	if draftPlaylist := strings.ToUpper(strings.TrimSpace(req.Draft.PlaylistName)); draftPlaylist != "" && draftPlaylist != playlistName {
		http.Error(w, "draft playlist mismatch", http.StatusBadRequest)
		return
	}

	sourceID := strings.TrimSpace(req.Source.ID)
	sourceName := strings.TrimSpace(req.Source.Name)
	sourcePath := strings.TrimSpace(req.Source.Path)
	sourcePlaylistRoot := sourcePath

	switch sourceType {
	case "iso":
		if h.Scanner == nil {
			http.Error(w, "jobs source scanner is not configured", http.StatusInternalServerError)
			return
		}
		if sourceID == "" {
			http.Error(w, "missing source id", http.StatusBadRequest)
			return
		}
		sources, err := h.Scanner.Scan(h.InputDir)
		if err != nil {
			http.Error(w, "failed to scan sources", http.StatusInternalServerError)
			return
		}
		source, ok := findSourceByID(sources, sourceID)
		if !ok {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}
		if source.Type != media.SourceISO {
			http.Error(w, "selected source is not an iso", http.StatusBadRequest)
			return
		}
		if !isPathWithinRoot(h.InputDir, source.Path) {
			http.Error(w, "source path is outside input root", http.StatusBadRequest)
			return
		}
		sourceID = source.ID
		sourceName = source.Name
		sourcePath = source.Path
		if _, err := os.Stat(sourcePath); err != nil {
			http.Error(w, "source path does not exist", http.StatusBadRequest)
			return
		}
		sourcePlaylistRoot = sourcePath
		payloadJSON, err := rewriteSourcePayloadJSON(body, sourcePath, sourceID, sourceName, sourceType)
		if err != nil {
			http.Error(w, "failed to encode job payload", http.StatusInternalServerError)
			return
		}
		startRequest := remux.StartRequest{
			SourceID:     sourceID,
			SourceType:   strings.TrimSpace(string(source.Type)),
			SourceName:   sourceName,
			OutputName:   strings.TrimSpace(req.OutputFilename),
			OutputPath:   strings.TrimSpace(req.OutputPath),
			PlaylistName: playlistName,
			PayloadJSON:  payloadJSON,
		}
		task, err := h.Tasks.Start(startRequest)
		if err != nil {
			if errors.Is(err, remux.ErrTaskAlreadyRunning) {
				http.Error(w, "task already running", http.StatusConflict)
				return
			}
			http.Error(w, "failed to create job", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(task); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
		return
	case "bdmv":
		if _, err := os.Stat(sourcePath); err != nil {
			http.Error(w, "source path does not exist", http.StatusBadRequest)
			return
		}
		if strings.EqualFold(filepath.Base(sourcePlaylistRoot), "BDMV") {
			sourcePlaylistRoot = filepath.Dir(sourcePlaylistRoot)
		}
	default:
		http.Error(w, "only bdmv and iso sources are supported", http.StatusBadRequest)
		return
	}
	if sourceType != "iso" {
		if _, err := findPlaylistFilePath(sourcePlaylistRoot, playlistName); err != nil {
			http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
			return
		}
	}
	payloadJSON := strings.TrimSpace(string(body))
	if payloadJSON == "" {
		encoded, err := json.Marshal(req)
		if err != nil {
			http.Error(w, "failed to encode job payload", http.StatusInternalServerError)
			return
		}
		payloadJSON = string(encoded)
	}
	startRequest := remux.StartRequest{
		SourceID:     sourceID,
		SourceType:   strings.TrimSpace(req.Source.Type),
		SourceName:   sourceName,
		OutputName:   strings.TrimSpace(req.OutputFilename),
		OutputPath:   strings.TrimSpace(req.OutputPath),
		PlaylistName: playlistName,
		PayloadJSON:  payloadJSON,
	}
	task, err := h.Tasks.Start(startRequest)
	if err != nil {
		if errors.Is(err, remux.ErrTaskAlreadyRunning) {
			http.Error(w, "task already running", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func rewriteSourcePayloadJSON(body []byte, sourcePath, sourceID, sourceName, sourceType string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	source, _ := payload["source"].(map[string]any)
	if source == nil {
		source = map[string]any{}
		payload["source"] = source
	}
	source["id"] = sourceID
	source["name"] = sourceName
	source["path"] = sourcePath
	source["type"] = sourceType

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (h *JobsHandler) Current(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	task, err := h.Tasks.Current()
	if err != nil {
		if errors.Is(err, remux.ErrTaskNotFound) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load current task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) CurrentLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	logText, err := h.Tasks.CurrentLog()
	if err != nil {
		if errors.Is(err, remux.ErrTaskNotFound) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load current task log", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(logText)); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) StopCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Tasks == nil {
		http.Error(w, "jobs service is not configured", http.StatusInternalServerError)
		return
	}

	if err := h.Tasks.StopCurrent(); err != nil {
		if errors.Is(err, remux.ErrTaskNotFound) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to stop current task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
