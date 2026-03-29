package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/wangdazhuo/mkv-maker/internal/media"
)

type SourceScanner interface {
	Scan(root string) ([]media.SourceEntry, error)
}

type SourcesHandler struct {
	InputDir string
	Scanner  SourceScanner
}

type sourcesErrorResponse struct {
	Error sourcesErrorDetail `json:"error"`
}

type sourcesErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
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

func (h *SourcesHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	http.Error(w, "not implemented", http.StatusNotImplemented)
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
