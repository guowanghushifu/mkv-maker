package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guowanghushifu/mkv-maker/internal/isomount"
)

type stubISOMountReleaseManager struct {
	result isomount.ReleaseResult
}

func (s *stubISOMountReleaseManager) ReleaseIdleMounted(ctx context.Context) isomount.ReleaseResult {
	return s.result
}

func TestISOMountsHandlerReleaseMountedReturnsSummary(t *testing.T) {
	manager := &stubISOMountReleaseManager{
		result: isomount.ReleaseResult{Released: 2, SkippedInUse: 1, Failed: 0},
	}
	h := NewISOMountsHandler(manager)
	req := httptest.NewRequest(http.MethodPost, "/api/iso/release-mounted", nil)
	w := httptest.NewRecorder()

	h.ReleaseMounted(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got isomount.ReleaseResult
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got != manager.result {
		t.Fatalf("expected %+v, got %+v", manager.result, got)
	}
}
