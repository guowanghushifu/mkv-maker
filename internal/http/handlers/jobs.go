package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	mediabdinfo "github.com/wangdazhuo/mkv-maker/internal/media/bdinfo"
	"github.com/wangdazhuo/mkv-maker/internal/store"
)

type JobsHandler struct {
	Store     store.JobsRepository
	InputDir  string
	OutputDir string
}

type listJobsResponse struct {
	Jobs []store.APIJob `json:"jobs"`
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

func NewJobsHandler(jobStore store.JobsRepository, inputDir, outputDir string) *JobsHandler {
	return &JobsHandler{
		Store:     jobStore,
		InputDir:  strings.TrimSpace(inputDir),
		OutputDir: strings.TrimSpace(outputDir),
	}
}

func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.Store == nil {
		http.Error(w, "jobs store is not configured", http.StatusInternalServerError)
		return
	}

	jobs, err := h.Store.ListJobs()
	if err != nil {
		http.Error(w, "failed to list jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(listJobsResponse{Jobs: jobs}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.Store == nil {
		http.Error(w, "jobs store is not configured", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
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
	if !isPathWithinRoot(h.OutputDir, req.OutputPath) {
		http.Error(w, "output path is outside output root", http.StatusBadRequest)
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
	if requested := strings.ToUpper(strings.TrimSpace(req.BDInfo.PlaylistName)); requested != "" && requested != playlistName {
		http.Error(w, "bdinfo playlist mismatch", http.StatusBadRequest)
		return
	}
	if draftPlaylist := strings.ToUpper(strings.TrimSpace(req.Draft.PlaylistName)); draftPlaylist != "" && draftPlaylist != playlistName {
		http.Error(w, "draft playlist mismatch", http.StatusBadRequest)
		return
	}
	playlistPath := filepath.Join(req.Source.Path, "PLAYLIST", playlistName)
	if !strings.EqualFold(filepath.Base(strings.TrimSpace(req.Source.Path)), "BDMV") {
		playlistPath = filepath.Join(req.Source.Path, "BDMV", "PLAYLIST", playlistName)
	}
	if _, err := os.Stat(playlistPath); err != nil {
		http.Error(w, "playlist does not exist in selected source", http.StatusBadRequest)
		return
	}
	input := store.CreateJobInput{
		SourceName:   strings.TrimSpace(req.Source.Name),
		OutputName:   strings.TrimSpace(req.OutputFilename),
		OutputPath:   strings.TrimSpace(req.OutputPath),
		PlaylistName: playlistName,
		PayloadJSON:  payloadJSON,
	}
	job, err := h.Store.CreateQueuedJob(input)
	if err != nil {
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(job); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Store == nil {
		http.Error(w, "jobs store is not configured", http.StatusInternalServerError)
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	job, err := h.Store.GetJob(id)
	if err != nil {
		if errors.Is(err, store.ErrJobNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) Log(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Store == nil {
		http.Error(w, "jobs store is not configured", http.StatusInternalServerError)
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	logText, err := h.Store.GetJobLog(id)
	if err != nil {
		if errors.Is(err, store.ErrJobNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load job log", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(logText)); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}
