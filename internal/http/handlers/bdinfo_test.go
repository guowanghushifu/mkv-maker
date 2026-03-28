package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBDInfoHandlerParseReturnsBadRequestOnUnparseableLogText(t *testing.T) {
	h := NewBDInfoHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/bdinfo/parse", strings.NewReader(`{"logText":"not bdinfo"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Parse(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
