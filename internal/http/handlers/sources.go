package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/wangdazhuo/mkv-maker/internal/media"
)

type SourceScanner interface {
	Scan(root string) ([]media.SourceEntry, error)
}

type SourcesHandler struct {
	InputDir string
	Scanner  SourceScanner
}

func NewSourcesHandler(inputDir string, scanner SourceScanner) *SourcesHandler {
	if scanner == nil {
		scanner = media.NewScanner()
	}
	return &SourcesHandler{
		InputDir: inputDir,
		Scanner:  scanner,
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

func (h *SourcesHandler) writeScannedSources(w http.ResponseWriter) {
	items, err := h.Scanner.Scan(h.InputDir)
	if err != nil {
		http.Error(w, "failed to scan sources", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
