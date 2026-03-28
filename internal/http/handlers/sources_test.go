package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/wangdazhuo/mkv-maker/internal/media"
)

type stubSourceScanner struct {
	err error
}

func (s stubSourceScanner) Scan(root string) ([]media.SourceEntry, error) {
	return nil, s.err
}

func TestSourcesHandlerListReturnsStructuredNotFoundError(t *testing.T) {
	h := NewSourcesHandler("/missing/input", stubSourceScanner{
		err: &os.PathError{
			Op:   "readdir",
			Path: "/missing/input",
			Err:  os.ErrNotExist,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Path    string `json:"path"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "input_dir_not_found" {
		t.Fatalf("expected error code input_dir_not_found, got %q", body.Error.Code)
	}
	if body.Error.Path != "/missing/input" {
		t.Fatalf("expected error path /missing/input, got %q", body.Error.Path)
	}
}

func TestSourcesHandlerListReturnsStructuredUnreadableError(t *testing.T) {
	h := NewSourcesHandler("/restricted/input", stubSourceScanner{
		err: &os.PathError{
			Op:   "readdir",
			Path: "/restricted/input",
			Err:  os.ErrPermission,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Path    string `json:"path"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "input_dir_unreadable" {
		t.Fatalf("expected error code input_dir_unreadable, got %q", body.Error.Code)
	}
	if body.Error.Path != "/restricted/input" {
		t.Fatalf("expected error path /restricted/input, got %q", body.Error.Path)
	}
}
