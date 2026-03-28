package handlers

import (
	"encoding/json"
	"net/http"

	mediabdinfo "github.com/wangdazhuo/mkv-maker/internal/media/bdinfo"
)

type BDInfoHandler struct {
	Parser func(input string) (mediabdinfo.Parsed, error)
}

type bdinfoParseRequest struct {
	LogText string `json:"logText"`
}

func NewBDInfoHandler() *BDInfoHandler {
	return &BDInfoHandler{
		Parser: mediabdinfo.Parse,
	}
}

func (h *BDInfoHandler) Parse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req bdinfoParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	parse := h.Parser
	if parse == nil {
		parse = mediabdinfo.Parse
	}

	parsed, err := parse(req.LogText)
	if err != nil {
		http.Error(w, "failed to parse bdinfo", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(parsed); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
