package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type DraftsHandler struct{}

type previewFilenameResponse struct {
	Filename string `json:"filename"`
}

func NewDraftsHandler() *DraftsHandler {
	return &DraftsHandler{}
}

func (h *DraftsHandler) PreviewFilename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var draft remux.Draft
	if err := json.NewDecoder(r.Body).Decode(&draft); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(previewFilenameResponse{
		Filename: remux.BuildFilename(draft),
	}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
