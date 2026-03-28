package handlers

import (
	"encoding/json"
	"net/http"
)

type ConfigHandler struct {
	InputDir  string
	OutputDir string
}

type configResponse struct {
	InputDir  string `json:"inputDir"`
	OutputDir string `json:"outputDir"`
}

func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(configResponse{
		InputDir:  h.InputDir,
		OutputDir: h.OutputDir,
	}); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
