package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/media"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type stubTasksManager struct {
	startReq     remux.StartRequest
	startCalled  bool
	startFn      func(remux.StartRequest) (remux.Task, error)
	currentFn    func() (remux.Task, error)
	currentLogFn func() (string, error)
	stopFn       func() error
	stopCalled   bool
}

func (s *stubTasksManager) Start(req remux.StartRequest) (remux.Task, error) {
	s.startCalled = true
	s.startReq = req
	if s.startFn != nil {
		return s.startFn(req)
	}
	return remux.Task{}, nil
}

func (s *stubTasksManager) Current() (remux.Task, error) {
	if s.currentFn != nil {
		return s.currentFn()
	}
	return remux.Task{}, remux.ErrTaskNotFound
}

func (s *stubTasksManager) CurrentLog() (string, error) {
	if s.currentLogFn != nil {
		return s.currentLogFn()
	}
	return "", remux.ErrTaskNotFound
}

func (s *stubTasksManager) StopCurrent() error {
	s.stopCalled = true
	if s.stopFn != nil {
		return s.stopFn()
	}
	return nil
}

type stubRunner struct {
	output string
	err    error
	wait   time.Duration
}

func (r *stubRunner) Run(ctx context.Context, draft remux.Draft, args []string, emit func(string)) (string, error) {
	_ = args
	wait := r.wait
	if wait <= 0 {
		wait = 50 * time.Millisecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		if emit != nil && r.output != "" {
			emit(r.output)
		}
		if r.err == nil {
			if err := writeSuccessfulTempOutput(draft.OutputPath); err != nil {
				return r.output, err
			}
		}
		return r.output, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type controlledRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *controlledRunner) Run(ctx context.Context, draft remux.Draft, args []string, emit func(string)) (string, error) {
	_ = args
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
	if emit != nil {
		emit("ok")
	}
	if err := writeSuccessfulTempOutput(draft.OutputPath); err != nil {
		return "", err
	}
	return "ok", nil
}

func writeSuccessfulTempOutput(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("muxed"), 0o644)
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
	reqBody := validCreateBody(sourcePath, outputRoot, "Nightcrawler - 2160p.mkv", "00800.MPLS", "Nightcrawler Disc")
	manager := &stubTasksManager{
		startFn: func(_ remux.StartRequest) (remux.Task, error) {
			return remux.Task{
				ID:           "task-123",
				SourceName:   "Nightcrawler Disc",
				OutputName:   "Nightcrawler - 2160p.mkv",
				OutputPath:   filepath.Join(outputRoot, "Nightcrawler - 2160p.mkv"),
				PlaylistName: "00800.MPLS",
				CreatedAt:    "2026-03-29T12:00:00Z",
				Status:       "running",
			}, nil
		},
	}
	h := NewJobsHandler(manager, inputRoot, outputRoot)

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
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
	if !manager.startCalled {
		t.Fatal("expected Start to be called")
	}
	if manager.startReq.SourceName != "Nightcrawler Disc" {
		t.Fatalf("unexpected forwarded source name %q", manager.startReq.SourceName)
	}
	if manager.startReq.OutputName != "Nightcrawler - 2160p.mkv" {
		t.Fatalf("unexpected forwarded output name %q", manager.startReq.OutputName)
	}
	if manager.startReq.PlaylistName != "00800.MPLS" {
		t.Fatalf("unexpected forwarded playlist name %q", manager.startReq.PlaylistName)
	}
	expectedOutputPath := filepath.Join(outputRoot, "Nightcrawler - 2160p.mkv")
	if manager.startReq.OutputPath != expectedOutputPath {
		t.Fatalf("unexpected forwarded output path %q", manager.startReq.OutputPath)
	}
	if manager.startReq.PayloadJSON != strings.TrimSpace(reqBody) {
		t.Fatalf("expected payload json to preserve request body, got %q", manager.startReq.PayloadJSON)
	}
}

func TestJobsHandlerCreateUsesCanonicalISOPathDirectly(t *testing.T) {
	inputRoot := t.TempDir()
	isoPath := filepath.Join(inputRoot, "library", "Nightcrawler.iso")
	if err := os.MkdirAll(filepath.Dir(isoPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputRoot := t.TempDir()
	scanner := stubSourceScanner{items: []media.SourceEntry{{
		ID:   "movies-nightcrawler-iso",
		Name: "Nightcrawler",
		Path: isoPath,
		Type: media.SourceISO,
	}}}
	manager := &stubTasksManager{
		startFn: func(_ remux.StartRequest) (remux.Task, error) {
			return remux.Task{
				ID:           "task-123",
				SourceName:   "Nightcrawler",
				OutputName:   "Nightcrawler.mkv",
				OutputPath:   filepath.Join(outputRoot, "Nightcrawler.mkv"),
				PlaylistName: "00800.MPLS",
				CreatedAt:    "2026-04-01T00:00:00Z",
				Status:       "running",
			}, nil
		},
	}
	h := NewJobsHandler(manager, inputRoot, outputRoot, scanner)
	forgedPath := filepath.Join(t.TempDir(), "wrong.iso")
	reqBody := `{
		"source":{"id":"movies-nightcrawler-iso","name":"Nightcrawler","path":"` + forgedPath + `","type":"iso"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"playlistName":"00800.MPLS","makemkv":{"playlistName":"00800.MPLS","titleId":4,"audio":[],"subtitles":[]}},
		"outputFilename":"Nightcrawler.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}
	if manager.startReq.SourceType != "iso" || manager.startReq.SourceID != "movies-nightcrawler-iso" {
		t.Fatalf("unexpected start request %+v", manager.startReq)
	}
	if manager.startReq.SourceLeaseGeneration != 0 {
		t.Fatalf("expected no ISO lease generation, got %d", manager.startReq.SourceLeaseGeneration)
	}
	if !strings.Contains(manager.startReq.PayloadJSON, isoPath) {
		t.Fatalf("expected payload JSON to reference canonical ISO path, got %q", manager.startReq.PayloadJSON)
	}
	if strings.Contains(manager.startReq.PayloadJSON, forgedPath) {
		t.Fatalf("expected forged request path to be ignored, got %q", manager.startReq.PayloadJSON)
	}
	if strings.Contains(manager.startReq.PayloadJSON, `"type":"bdmv"`) {
		t.Fatalf("expected payload JSON to keep source type iso, got %q", manager.startReq.PayloadJSON)
	}
}

func TestJobsHandlerCreateUsesCanonicalISOScanData(t *testing.T) {
	inputRoot := t.TempDir()
	canonicalPath := filepath.Join(inputRoot, "canonical", "Nightcrawler.iso")
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canonicalPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputRoot := t.TempDir()
	forgedPath := filepath.Join(t.TempDir(), "forged.iso")
	scanner := stubSourceScanner{items: []media.SourceEntry{{
		ID:   "movies-nightcrawler-iso",
		Name: "Nightcrawler",
		Path: canonicalPath,
		Type: media.SourceISO,
	}}}
	manager := &stubTasksManager{
		startFn: func(_ remux.StartRequest) (remux.Task, error) {
			return remux.Task{ID: "task-123", Status: "running"}, nil
		},
	}
	h := NewJobsHandler(manager, inputRoot, outputRoot, scanner)
	reqBody := `{
		"source":{"id":"movies-nightcrawler-iso","name":"Sneaky","path":"` + forgedPath + `","type":"iso"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"playlistName":"00800.MPLS","makemkv":{"playlistName":"00800.MPLS","titleId":4,"audio":[],"subtitles":[]}},
		"outputFilename":"Nightcrawler.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}
	if !strings.Contains(manager.startReq.PayloadJSON, canonicalPath) {
		t.Fatalf("expected canonical ISO path in payload, got %q", manager.startReq.PayloadJSON)
	}
	if strings.Contains(manager.startReq.PayloadJSON, forgedPath) {
		t.Fatalf("expected forged request path to be ignored, got %q", manager.startReq.PayloadJSON)
	}
	if manager.startReq.SourceName != "Nightcrawler" {
		t.Fatalf("expected canonical source name, got %q", manager.startReq.SourceName)
	}
	if manager.startReq.SourceID != "movies-nightcrawler-iso" {
		t.Fatalf("expected canonical source id, got %q", manager.startReq.SourceID)
	}
}

func TestJobsHandlerStopCurrentReturnsAccepted(t *testing.T) {
	manager := &stubTasksManager{
		stopFn: func() error { return nil },
	}
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/current/stop", nil)
	w := httptest.NewRecorder()
	h.StopCurrent(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if !manager.stopCalled {
		t.Fatal("expected StopCurrent to be called")
	}
}

func TestJobsHandlerStopCurrentReturnsNotFoundWithoutRunningTask(t *testing.T) {
	manager := &stubTasksManager{
		stopFn: func() error { return remux.ErrTaskNotFound },
	}
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/current/stop", nil)
	w := httptest.NewRecorder()
	h.StopCurrent(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestJobsHandlerStopCurrentReturnsInternalServerErrorOnUnexpectedError(t *testing.T) {
	manager := &stubTasksManager{
		stopFn: func() error { return errors.New("boom") },
	}
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/current/stop", nil)
	w := httptest.NewRecorder()
	h.StopCurrent(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
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
		"draft":{"playlistName":"00800.MPLS","makemkv":{"playlistName":"00800.MPLS","titleId":4,"audio":[],"subtitles":[]}},
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

func TestJobsHandlerCreateAllowsExistingOutputFile(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Disc", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	outputRoot := t.TempDir()
	existingOutput := filepath.Join(outputRoot, "Disc.mkv")
	if err := os.WriteFile(existingOutput, []byte("old"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	manager := remux.NewManager(&stubRunner{})
	defer manager.Close()
	h := NewJobsHandler(manager, inputRoot, outputRoot)
	reqBody := `{
		"source":{"name":"Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"playlistName":"00800.MPLS","makemkv":{"playlistName":"00800.MPLS","titleId":4,"audio":[],"subtitles":[]}},
		"outputFilename":"Disc.mkv",
		"outputPath":"` + existingOutput + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
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
		"draft":{"playlistName":"00003.MPLS","makemkv":{"playlistName":"00003.MPLS","titleId":4,"audio":[],"subtitles":[]}},
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

func TestJobsHandlerCurrentReturnsTask(t *testing.T) {
	manager := &stubTasksManager{
		currentFn: func() (remux.Task, error) {
			return remux.Task{
				ID:           "task-123",
				SourceName:   "Nightcrawler Disc",
				OutputName:   "Nightcrawler.mkv",
				OutputPath:   "/remux/Nightcrawler.mkv",
				PlaylistName: "00800.MPLS",
				CreatedAt:    "2026-03-29T12:00:00Z",
				Status:       "running",
			}, nil
		},
	}
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	w := httptest.NewRecorder()
	h.Current(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body remux.Task
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body.ID != "task-123" {
		t.Fatalf("unexpected task id %q", body.ID)
	}
}

func TestJobsHandlerCurrentLogReturnsText(t *testing.T) {
	manager := &stubTasksManager{
		currentLogFn: func() (string, error) {
			return "[2026-03-29T12:00:00Z] running\n", nil
		},
	}
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current/log", nil)
	w := httptest.NewRecorder()
	h.CurrentLog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "running") {
		t.Fatalf("unexpected log body %q", w.Body.String())
	}
}

func TestJobsHandlerCurrentReturnsInternalServerErrorOnUnexpectedError(t *testing.T) {
	manager := &stubTasksManager{
		currentFn: func() (remux.Task, error) {
			return remux.Task{}, errors.New("boom")
		},
	}
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	w := httptest.NewRecorder()
	h.Current(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
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
			"source":{"name":"Nightcrawler Disc","path":"/tmp/Nightcrawler.mkv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[],"makemkv":{"playlistName":"00800.MPLS","titleId":4,"audio":[],"subtitles":[]}},
			"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler.mkv") + `"
		}`,
	})
	<-runner.started

	h := NewJobsHandler(manager, inputRoot, outputRoot)
	reqBody := `{
		"source":{"id":"Nightcrawler","name":"Nightcrawler Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"00800.MPLS","makemkv":{"playlistName":"00800.MPLS","titleId":4,"audio":[],"subtitles":[]}},
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
		"draft":{"sourceId":"Nightcrawler","playlistName":"` + playlistName + `","makemkv":{"playlistName":"` + playlistName + `","titleId":4,"audio":[],"subtitles":[]}},
		"outputFilename":"` + outputFilename + `",
		"outputPath":"` + filepath.Join(outputRoot, outputFilename) + `"
	}`
}
