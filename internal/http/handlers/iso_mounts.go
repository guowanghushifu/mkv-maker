package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/guowanghushifu/mkv-maker/internal/isomount"
)

type ISOReleaseManager interface {
	ReleaseIdleMounted(ctx context.Context) isomount.ReleaseResult
}

type ISOMountsHandler struct {
	Manager ISOReleaseManager
}

func NewISOMountsHandler(manager ISOReleaseManager) *ISOMountsHandler {
	return &ISOMountsHandler{Manager: manager}
}

func (h *ISOMountsHandler) ReleaseMounted(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Manager == nil {
		http.Error(w, "iso mount service is not configured", http.StatusInternalServerError)
		return
	}

	result := h.Manager.ReleaseIdleMounted(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
