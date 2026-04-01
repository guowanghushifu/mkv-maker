package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/guowanghushifu/mkv-maker/internal/isomount"
	mediabdinfo "github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type JobsHandler struct {
	Tasks      tasksManager
	InputDir   string
	OutputDir  string
	ISOManager *isomount.Manager
}

type tasksManager interface {
	Start(req remux.StartRequest) (remux.Task, error)
	Current() (remux.Task, error)
	CurrentLog() (string, error)
}

type createJobRequest struct {
	Source struct {
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

func NewJobsHandler(tasks tasksManager, inputDir, outputDir string, isoManager ...*isomount.Manager) *JobsHandler {
	var manager *isomount.Manager
	if len(isoManager) > 0 {
		manager = isoManager[0]
	}
	return &JobsHandler{
		Tasks:      tasks,
		InputDir:   strings.TrimSpace(inputDir),
		OutputDir:  strings.TrimSpace(outputDir),
		ISOManager: manager,
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
	if sourceType := strings.TrimSpace(req.Source.Type); sourceType != "" && !strings.EqualFold(sourceType, "bdmv") {
		http.Error(w, "only bdmv sources are supported", http.StatusBadRequest)
		return
	}
	if !isPathWithinRoot(h.InputDir, req.Source.Path) {
		http.Error(w, "source path is outside input root", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(req.Source.Path); err != nil {
		http.Error(w, "source path does not exist", http.StatusBadRequest)
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
	payloadJSON := strings.TrimSpace(string(body))
	if payloadJSON == "" {
		encoded, err := json.Marshal(req)
		if err != nil {
			http.Error(w, "failed to encode job payload", http.StatusInternalServerError)
			return
		}
		payloadJSON = string(encoded)
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
	sourcePlaylistRoot := strings.TrimSpace(req.Source.Path)
	if strings.EqualFold(filepath.Base(sourcePlaylistRoot), "BDMV") {
		sourcePlaylistRoot = filepath.Dir(sourcePlaylistRoot)
	}
	if _, err := findPlaylistFilePath(sourcePlaylistRoot, playlistName); err != nil {
		http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
		return
	}
	startRequest := remux.StartRequest{
		SourceName:   strings.TrimSpace(req.Source.Name),
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
