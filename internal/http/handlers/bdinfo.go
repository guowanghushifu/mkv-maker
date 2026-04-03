package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	mediabdinfo "github.com/guowanghushifu/mkv-maker/internal/media/bdinfo"
)

type BDInfoHandler struct {
	Parser func(input string) (mediabdinfo.Parsed, error)
}

type bdinfoParseRequest struct {
	RawText string `json:"rawText"`
	LogText string `json:"logText"`
}

const bdinfoBodyLimit = 2 << 20

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
	if !decodeJSONBodyLimited(w, r, bdinfoBodyLimit, &req) {
		return
	}

	parse := h.Parser
	if parse == nil {
		parse = mediabdinfo.Parse
	}

	input := strings.TrimSpace(req.RawText)
	if input == "" {
		input = req.LogText
	}

	parsed, err := parse(input)
	if err != nil {
		inputField := "rawText"
		if strings.TrimSpace(req.RawText) == "" {
			inputField = "logText"
		}
		log.Printf("bdinfo parse failed inputField=%s inputLen=%d: %v", inputField, len(input), err)
		http.Error(w, bdinfoParseErrorMessage(err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(parsed); err != nil {
		log.Printf("bdinfo parse encode failed playlist=%s: %v", parsed.PlaylistName, err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func bdinfoParseErrorMessage(err error) string {
	switch {
	case errors.Is(err, mediabdinfo.ErrNoRecognizedFields):
		return "no recognized bdinfo fields; please paste the full BDInfo output"
	case errors.Is(err, mediabdinfo.ErrMissingPlaylist):
		return "missing playlist name; please confirm the BDInfo text includes the playlist report"
	default:
		return err.Error()
	}
}
