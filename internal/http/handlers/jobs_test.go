package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type stubRunner struct {
	output string
	err    error
	wait   time.Duration
}

func (r *stubRunner) Run(ctx context.Context, _ remux.Draft) (string, error) {
	wait := r.wait
	if wait <= 0 {
		wait = 50 * time.Millisecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return r.output, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type controlledRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *controlledRunner) Run(ctx context.Context, _ remux.Draft) (string, error) {
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "ok", nil
}

func TestJobsHandlerCreateReturnsAcceptedTask(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Nightcrawler", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	outputRoot := t.TempDir()

	manager := remux.NewManager(&stubRunner{})
	defer manager.Close()
	h := NewJobsHandler(manager, inputRoot, outputRoot)

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(validCreateBody(sourcePath, outputRoot, "Nightcrawler - 2160p.mkv", "00800.MPLS", "Nightcrawler Disc")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}

	var body remux.Task
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.SourceName != "Nightcrawler Disc" {
		t.Fatalf("unexpected source name %q", body.SourceName)
	}
	if body.PlaylistName != "00800.MPLS" {
		t.Fatalf("unexpected playlist name %q", body.PlaylistName)
	}
	if body.OutputName != "Nightcrawler - 2160p.mkv" {
		t.Fatalf("unexpected output name %q", body.OutputName)
	}
}

func TestJobsHandlerCreateRejectsPlaylistTraversal(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Disc", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	outputRoot := t.TempDir()

	manager := remux.NewManager(&stubRunner{})
	defer manager.Close()
	h := NewJobsHandler(manager, inputRoot, outputRoot)
	reqBody := `{
		"source":{"name":"Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"../INDEX.BDMV","rawText":"PLAYLIST REPORT:\nName: ../INDEX.BDMV"},
		"draft":{"playlistName":"../INDEX.BDMV"},
		"outputFilename":"Disc.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Disc.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestJobsHandlerCreateRejectsSymlinkEscapeInOutputPath(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Disc", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	outputRoot := t.TempDir()
	outsideRoot := t.TempDir()
	if err := os.Symlink(outsideRoot, filepath.Join(outputRoot, "outside-link")); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	manager := remux.NewManager(&stubRunner{})
	defer manager.Close()
	h := NewJobsHandler(manager, inputRoot, outputRoot)
	reqBody := `{
		"source":{"name":"Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"playlistName":"00800.MPLS"},
		"outputFilename":"Disc.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "outside-link", "Disc.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestJobsHandlerCreateAcceptsLowercasePlaylistFileOnDisk(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Disc", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00003.mpls"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	outputRoot := t.TempDir()

	manager := remux.NewManager(&stubRunner{})
	defer manager.Close()
	h := NewJobsHandler(manager, inputRoot, outputRoot)

	reqBody := `{
		"source":{"name":"Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00003.MPLS","rawText":"PLAYLIST REPORT:\nName: 00003.MPLS"},
		"draft":{"playlistName":"00003.MPLS"},
		"outputFilename":"Disc.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Disc.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}
}

func TestJobsHandlerCurrentReturnsNotFoundWithoutTask(t *testing.T) {
	manager := remux.NewManager(&stubRunner{})
	defer manager.Close()
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	w := httptest.NewRecorder()
	h.Current(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestJobsHandlerCreateReturnsConflictWhenTaskRunning(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Nightcrawler", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	outputRoot := t.TempDir()
	runner := &controlledRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manager := remux.NewManager(runner)
	defer manager.Close()
	_, _ = manager.Start(remux.StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   filepath.Join(outputRoot, "Nightcrawler.mkv"),
		PlaylistName: "00800.MPLS",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"` + sourcePath + `","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler.mkv") + `"
		}`,
	})
	<-runner.started

	h := NewJobsHandler(manager, inputRoot, outputRoot)
	reqBody := `{
		"source":{"id":"Nightcrawler","name":"Nightcrawler Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"00800.MPLS"},
		"outputFilename":"Nightcrawler.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func validCreateBody(sourcePath, outputRoot, outputFilename, playlistName, sourceName string) string {
	return `{
		"source":{"id":"Nightcrawler","name":"` + sourceName + `","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"` + playlistName + `","rawText":"PLAYLIST REPORT:\nName: ` + playlistName + `"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"` + playlistName + `"},
		"outputFilename":"` + outputFilename + `",
		"outputPath":"` + filepath.Join(outputRoot, outputFilename) + `"
	}`
}
