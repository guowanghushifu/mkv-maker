package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/wangdazhuo/mkv-maker/internal/store"
)

type JobsHandler struct {
	Store store.JobsRepository
}

type listJobsResponse struct {
	Jobs []store.APIJob `json:"jobs"`
}

type createJobRequest struct {
	Source struct {
		Name string `json:"name"`
	} `json:"source"`
	BDInfo struct {
		PlaylistName string `json:"playlistName"`
	} `json:"bdinfo"`
	Draft struct {
		PlaylistName string `json:"playlistName"`
	} `json:"draft"`
	OutputFilename string `json:"outputFilename"`
	OutputPath     string `json:"outputPath"`
}

func NewJobsHandler(jobStore store.JobsRepository) *JobsHandler {
	return &JobsHandler{Store: jobStore}
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

	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	payloadJSON, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "failed to encode job payload", http.StatusInternalServerError)
		return
	}

	playlistName := strings.TrimSpace(req.BDInfo.PlaylistName)
	if playlistName == "" {
		playlistName = strings.TrimSpace(req.Draft.PlaylistName)
	}
	input := store.CreateJobInput{
		SourceName:   strings.TrimSpace(req.Source.Name),
		OutputName:   strings.TrimSpace(req.OutputFilename),
		OutputPath:   strings.TrimSpace(req.OutputPath),
		PlaylistName: playlistName,
		PayloadJSON:  string(payloadJSON),
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
