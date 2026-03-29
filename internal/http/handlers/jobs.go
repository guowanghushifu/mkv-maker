package handlers

import (
	"encoding/json"
	"net/http"
)

type JobsHandler struct{}

type listJobsResponse struct {
	Jobs []jobResponse `json:"jobs"`
}

type createJobResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type jobResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func NewJobsHandler() *JobsHandler {
	return &JobsHandler{}
}

func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(listJobsResponse{Jobs: []jobResponse{}}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(createJobResponse{
		ID:     "",
		Status: "queued",
	}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *JobsHandler) Log(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
