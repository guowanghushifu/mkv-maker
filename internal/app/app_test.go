package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guowanghushifu/mkv-maker/internal/config"
)

func TestNewListsISOSourcesWithoutMountSetup(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()
	dataRoot := t.TempDir()

	isoPath := filepath.Join(inputRoot, "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := New(config.Config{
		AppPassword:   "secret",
		InputDir:      inputRoot,
		OutputDir:     outputRoot,
		DataDir:       dataRoot,
		SessionMaxAge: 3600,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = app.Close()
	})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	loginReq.Body = http.NoBody
	loginReq = withPassword(loginReq, `{"password":"secret"}`)
	loginRec := httptest.NewRecorder()
	app.Handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("expected login to succeed, got %d", loginRec.Code)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie from login")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	for _, cookie := range cookies {
		listReq.AddCookie(cookie)
	}
	listRec := httptest.NewRecorder()
	app.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected source list to succeed, got %d", listRec.Code)
	}

	var sources []struct {
		Type string `json:"type"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &sources); err != nil {
		t.Fatalf("failed to decode source list: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected one source, got %+v", sources)
	}
	if sources[0].Type != "iso" {
		t.Fatalf("expected ISO source, got %+v", sources[0])
	}
	if sources[0].Path != isoPath {
		t.Fatalf("expected ISO path %q, got %q", isoPath, sources[0].Path)
	}
}

func withPassword(req *http.Request, body string) *http.Request {
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(body))
	return req
}
