package remux

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubRunner struct {
	lastDraft Draft
	output    string
	err       error
}

func (r *stubRunner) Run(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
	r.lastDraft = draft
	if onOutput != nil && r.output != "" {
		onOutput(r.output)
	}
	return r.output, r.err
}

type controlledRunner struct {
	output  string
	err     error
	started chan struct{}
	release chan struct{}
}

func (r *controlledRunner) Run(ctx context.Context, draft Draft, onOutput func(string)) (string, error) {
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

	return r.output, r.err
}

type streamingRunner struct {
	started  chan struct{}
	progress chan struct{}
	release  chan struct{}
}

func (r *streamingRunner) Run(ctx context.Context, draft Draft, onOutput func(string)) (string, error) {
	_ = draft
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}
	if onOutput != nil {
		onOutput("Progress: 4")
		onOutput("2%\r")
	}
	if r.progress != nil {
		select {
		case <-r.progress:
		default:
			close(r.progress)
		}
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if onOutput != nil {
		onOutput("Progress: 100%\r")
	}
	return "", nil
}

func TestManagerStartRejectsWhenJobAlreadyRunning(t *testing.T) {
	runner := &controlledRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manager := NewManager(runner)
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}
	<-runner.started

	_, err = manager.Start(StartRequest{
		SourceName:   "Second Disc",
		OutputName:   "Second.mkv",
		OutputPath:   "/remux/Second.mkv",
		PlaylistName: "00002.MPLS",
		PayloadJSON:  validPayloadJSON("Second Disc", "/bd_input/Second", "00002.MPLS", "/remux/Second.mkv"),
	})
	if !errors.Is(err, ErrTaskAlreadyRunning) {
		t.Fatalf("expected ErrTaskAlreadyRunning, got %v", err)
	}

	close(runner.release)
}

func TestManagerCurrentReturnsRunningAndLatestLog(t *testing.T) {
	manager := NewManager(&stubRunner{output: "mkvmerge progress"})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"/remux/Nightcrawler.mkv"
		}`,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if task.Status != "running" {
		t.Fatalf("expected running status, got %q", task.Status)
	}

	current, err := manager.Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if current.ID != task.ID {
		t.Fatalf("expected current id %q, got %q", task.ID, current.ID)
	}
}

func TestManagerSuccessTransitionKeepsLatestAndCompletionLog(t *testing.T) {
	manager := NewManager(&stubRunner{output: "mkvmerge progress"})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.ID != task.ID {
		t.Fatalf("expected latest task id %q, got %q", task.ID, done.ID)
	}
	if done.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %q", done.Status)
	}
	if done.Message != "" {
		t.Fatalf("expected empty message on success, got %q", done.Message)
	}

	logText, err := manager.CurrentLog()
	if err != nil {
		t.Fatalf("CurrentLog returned error: %v", err)
	}
	if !strings.Contains(logText, "mkvmerge progress") {
		t.Fatalf("expected command output in log, got %q", logText)
	}
	if !strings.Contains(logText, "completed") {
		t.Fatalf("expected completion line in log, got %q", logText)
	}
}

func TestManagerFailureTransitionKeepsLatestAndFailureLog(t *testing.T) {
	manager := NewManager(&stubRunner{
		output: "stderr output",
		err:    errors.New("runner exploded"),
	})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.ID != task.ID {
		t.Fatalf("expected latest task id %q, got %q", task.ID, done.ID)
	}
	if done.Status != "failed" {
		t.Fatalf("expected failed status, got %q", done.Status)
	}
	if !strings.Contains(done.Message, "runner exploded") {
		t.Fatalf("expected failure message to include runner error, got %q", done.Message)
	}

	logText, err := manager.CurrentLog()
	if err != nil {
		t.Fatalf("CurrentLog returned error: %v", err)
	}
	if !strings.Contains(logText, "stderr output") {
		t.Fatalf("expected command output in log, got %q", logText)
	}
	if !strings.Contains(logText, "runner exploded") {
		t.Fatalf("expected failure line in log, got %q", logText)
	}
}

func TestManagerCallsOnTaskFinishedHook(t *testing.T) {
	runner := &stubRunner{output: "ok"}
	manager := NewManager(runner)
	defer manager.Close()

	done := make(chan StartRequest, 1)
	manager.SetOnTaskFinished(func(req StartRequest, task Task) {
		if task.Status == "succeeded" {
			done <- req
		}
	})
	_, err := manager.Start(StartRequest{
		SourceID:     "movies-nightcrawler-iso",
		SourceType:   "iso",
		SourceName:   "Nightcrawler",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler", "/bd_input/Nightcrawler", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	select {
	case req := <-done:
		if req.SourceID != "movies-nightcrawler-iso" {
			t.Fatalf("unexpected hook request %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("expected terminal hook to run")
	}
}

func TestManagerCloseCancelsInFlightTask(t *testing.T) {
	runner := &controlledRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manager := NewManager(runner)

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	closeDone := make(chan struct{})
	go func() {
		manager.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return")
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status after cancel, got %q", done.Status)
	}
	if !strings.Contains(strings.ToLower(done.Message), "canceled") {
		t.Fatalf("expected canceled message after Close, got %q", done.Message)
	}
}

func TestManagerSuccessTransitionSetsCommandPreviewAndHundredPercent(t *testing.T) {
	manager := NewManager(&stubRunner{output: "Progress: 42%\nProgress: 100%"})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.ID != task.ID {
		t.Fatalf("expected same task id, got %q", done.ID)
	}
	if done.ProgressPercent != 100 {
		t.Fatalf("expected 100 percent, got %d", done.ProgressPercent)
	}
	if !strings.HasPrefix(done.CommandPreview, "mkvmerge\n") {
		t.Fatalf("expected command preview, got %q", done.CommandPreview)
	}
}

func TestManagerFailureKeepsLastKnownProgressPercent(t *testing.T) {
	manager := NewManager(&stubRunner{
		output: "Progress: 63%\nstderr output",
		err:    errors.New("runner exploded"),
	})
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status, got %q", done.Status)
	}
	if done.ProgressPercent != 63 {
		t.Fatalf("expected last known progress 63, got %d", done.ProgressPercent)
	}
}

func TestManagerProgressUpdatesBeforeTerminalCompletion(t *testing.T) {
	runner := &streamingRunner{
		started:  make(chan struct{}),
		progress: make(chan struct{}),
		release:  make(chan struct{}),
	}
	manager := NewManager(runner)
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started
	<-runner.progress

	current, err := manager.Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if current.Status != "running" {
		t.Fatalf("expected running status before release, got %q", current.Status)
	}
	if current.ProgressPercent != 42 {
		t.Fatalf("expected running progress 42 before release, got %d", current.ProgressPercent)
	}

	close(runner.release)
	done := waitForTerminalTask(t, manager)
	if done.ProgressPercent != 100 {
		t.Fatalf("expected final progress 100, got %d", done.ProgressPercent)
	}
}

func TestManagerProgressUpdatesFromCarriageReturnChunks(t *testing.T) {
	runner := &streamingRunner{
		started:  make(chan struct{}),
		progress: make(chan struct{}),
		release:  make(chan struct{}),
	}
	manager := NewManager(runner)
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started
	<-runner.progress

	current, err := manager.Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if current.ProgressPercent != 42 {
		t.Fatalf("expected carriage-return progress 42 while running, got %d", current.ProgressPercent)
	}

	close(runner.release)
	done := waitForTerminalTask(t, manager)
	if done.ProgressPercent != 100 {
		t.Fatalf("expected final progress 100, got %d", done.ProgressPercent)
	}
}

func waitForTerminalTask(t *testing.T, manager *Manager) Task {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := manager.Current()
		if err == nil && task.Status != "running" {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for terminal task state")
	return Task{}
}

func validPayloadJSON(sourceName, sourcePath, playlistName, outputPath string) string {
	return `{
		"source":{"name":"` + sourceName + `","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"` + playlistName + `"},
		"draft":{"playlistName":"` + playlistName + `","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
		"outputPath":"` + outputPath + `"
	}`
}
