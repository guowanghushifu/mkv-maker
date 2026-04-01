package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBodyLimitedRejectsOversizedPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"rawText":"`+strings.Repeat("a", 32)+`"}`))
	w := httptest.NewRecorder()

	var body struct {
		RawText string `json:"rawText"`
	}
	ok := decodeJSONBodyLimited(w, req, 16, &body)
	if ok {
		t.Fatal("expected oversized payload to be rejected")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}
