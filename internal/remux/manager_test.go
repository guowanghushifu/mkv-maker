package remux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubRunner struct {
	lastDraft Draft
	lastArgs  []string
	output    string
	err       error
	wait      time.Duration
}

func (r *stubRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	r.lastDraft = draft
	r.lastArgs = append([]string(nil), args...)
	if r.wait > 0 {
		timer := time.NewTimer(r.wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if onOutput != nil && r.output != "" {
		onOutput(r.output)
	}
	if r.err == nil {
		if err := writeSuccessfulTempOutput(draft.OutputPath); err != nil {
			return r.output, err
		}
	}
	return r.output, r.err
}

type controlledRunner struct {
	output  string
	err     error
	started chan struct{}
	release chan struct{}
}

func (r *controlledRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	_ = draft
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

	if r.err == nil {
		if err := writeSuccessfulTempOutput(draft.OutputPath); err != nil {
			return r.output, err
		}
	}
	return r.output, r.err
}

type signalKilledRunner struct {
	started chan struct{}
}

func (r *signalKilledRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	_ = draft
	_ = args
	_ = onOutput
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}

	<-ctx.Done()
	return "", errors.New("signal: killed")
}

type streamingRunner struct {
	started  chan struct{}
	progress chan struct{}
	release  chan struct{}
}

type makeMKVSavingRunner struct {
	started  chan struct{}
	progress chan struct{}
	release  chan struct{}
}

type controlledFileRunner struct {
	started chan struct{}
	release chan struct{}
	run     func(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error)
}

func (r *controlledFileRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}
	if r.run != nil {
		return r.run(ctx, draft, args, onOutput)
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", nil
}

func (r *streamingRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	_ = args
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
	if err := writeSuccessfulTempOutput(draft.OutputPath); err != nil {
		return "", err
	}
	if onOutput != nil {
		onOutput("Progress: 100%\r")
	}
	return "", nil
}

func (r *makeMKVSavingRunner) Run(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
	_ = args
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}
	if onOutput != nil {
		onOutput("Current action: Saving to MKV file\n")
		onOutput("Current progress - 2%  , Total progress - 2%\n")
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
	if err := writeSuccessfulTempOutput(draft.OutputPath); err != nil {
		return "", err
	}
	return "", nil
}

func writeSuccessfulTempOutput(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("muxed"), 0o644)
}

func TestNewManagerWithTempDirOverridesExecutorTempDir(t *testing.T) {
	manager := NewManagerWithTempDir(&stubRunner{}, "/custom/remux-tmp")
	defer manager.Close()

	if manager.executor == nil || manager.executor.tempDir == nil {
		t.Fatal("expected manager executor tempDir override to be configured")
	}
	if got := manager.executor.tempDir(); got != "/custom/remux-tmp" {
		t.Fatalf("expected custom remux temp dir, got %q", got)
	}
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
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", "/remux/Nightcrawler.mkv"),
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
		PayloadJSON:  validIntermediatePayloadJSON("Second Disc", "/tmp/Second.mkv", "00002.MPLS", "/remux/Second.mkv"),
	})
	if !errors.Is(err, ErrTaskAlreadyRunning) {
		t.Fatalf("expected ErrTaskAlreadyRunning, got %v", err)
	}

	close(runner.release)
}

func TestManagerCurrentReturnsRunningAndLatestLog(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
	manager := NewManager(&stubRunner{output: "mkvmerge progress", wait: 100 * time.Millisecond})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00800.MPLS", outputPath),
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
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
	manager := NewManager(&stubRunner{output: "mkvmerge progress"})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", outputPath),
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
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", "/remux/Nightcrawler.mkv"),
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
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
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
		OutputPath:   outputPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler", "/tmp/Nightcrawler.mkv", "00800.MPLS", outputPath),
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
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", "/remux/Nightcrawler.mkv"),
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

func TestManagerCloseTreatsSignalKilledAsCanceled(t *testing.T) {
	runner := &signalKilledRunner{started: make(chan struct{})}
	manager := NewManager(runner)

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	manager.Close()

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status after cancel, got %q", done.Status)
	}
	if !strings.Contains(strings.ToLower(done.Message), "canceled") {
		t.Fatalf("expected canceled message after Close, got %q", done.Message)
	}
}

func TestManagerStopCurrentReturnsNotFoundWithoutRunningTask(t *testing.T) {
	manager := NewManager(&stubRunner{})
	defer manager.Close()

	if err := manager.StopCurrent(); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestManagerStopCurrentCancelsRunningTask(t *testing.T) {
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
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	if err := manager.StopCurrent(); err != nil {
		t.Fatalf("StopCurrent returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status, got %q", done.Status)
	}
	if !strings.Contains(strings.ToLower(done.Message), "canceled") {
		t.Fatalf("expected canceled message, got %q", done.Message)
	}

	logText, err := manager.CurrentLog()
	if err != nil {
		t.Fatalf("CurrentLog returned error: %v", err)
	}
	if !strings.Contains(strings.ToLower(logText), "canceled") {
		t.Fatalf("expected canceled log line, got %q", logText)
	}
}

func TestManagerStopCurrentRemovesTemporaryOutputAndKeepsFinalOutputPath(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Nightcrawler.mkv")
	tempPath := finalPath + ".tmp"

	runner := &controlledFileRunner{
		started: make(chan struct{}),
		run: func(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
			_ = args
			if draft.OutputPath != tempPath {
				t.Fatalf("expected runner output path %q, got %q", tempPath, draft.OutputPath)
			}
			if err := os.WriteFile(draft.OutputPath, []byte("partial"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	manager := NewManager(runner)
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", finalPath),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	if task.OutputPath != finalPath {
		t.Fatalf("expected task output path %q, got %q", finalPath, task.OutputPath)
	}
	if err := manager.StopCurrent(); err != nil {
		t.Fatalf("StopCurrent returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.OutputPath != finalPath {
		t.Fatalf("expected terminal task output path %q, got %q", finalPath, done.OutputPath)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary output to be removed, got %v", err)
	}
}

func TestManagerCloseRemovesTemporaryOutputAndKeepsFinalOutputPath(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Nightcrawler.mkv")
	tempPath := finalPath + ".tmp"

	runner := &controlledFileRunner{
		started: make(chan struct{}),
		run: func(ctx context.Context, draft Draft, args []string, onOutput func(string)) (string, error) {
			_ = args
			if draft.OutputPath != tempPath {
				t.Fatalf("expected runner output path %q, got %q", tempPath, draft.OutputPath)
			}
			if err := os.WriteFile(draft.OutputPath, []byte("partial"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	manager := NewManager(runner)

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", finalPath),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	if task.OutputPath != finalPath {
		t.Fatalf("expected task output path %q, got %q", finalPath, task.OutputPath)
	}
	manager.Close()

	done := waitForTerminalTask(t, manager)
	if done.OutputPath != finalPath {
		t.Fatalf("expected terminal task output path %q, got %q", finalPath, done.OutputPath)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary output to be removed, got %v", err)
	}
}

func TestManagerStopCurrentTreatsSignalKilledAsCanceled(t *testing.T) {
	runner := &signalKilledRunner{started: make(chan struct{})}
	manager := NewManager(runner)
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	if err := manager.StopCurrent(); err != nil {
		t.Fatalf("StopCurrent returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status, got %q", done.Status)
	}
	if !strings.Contains(strings.ToLower(done.Message), "canceled") {
		t.Fatalf("expected canceled message, got %q", done.Message)
	}
}

func TestManagerStopCurrentMarksTaskCanceledWhenStopWinsBeforeSuccessfulCompletion(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
	runner := &controlledRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manager := NewManager(runner)
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", outputPath),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	manager.mu.Lock()
	originalCancel := manager.current.cancel
	manager.current.cancel = func() {
		close(runner.release)
		time.Sleep(20 * time.Millisecond)
		originalCancel()
	}
	manager.mu.Unlock()

	if err := manager.StopCurrent(); err != nil {
		t.Fatalf("StopCurrent returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status after stop request, got %q", done.Status)
	}
	if !strings.Contains(strings.ToLower(done.Message), "canceled") {
		t.Fatalf("expected canceled message after stop request, got %q", done.Message)
	}
}

func TestManagerStopCurrentMarksTaskCanceledWhenStopArrivesDuringSuccessFinalization(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
	runner := &controlledRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manager := NewManager(runner)
	defer manager.Close()

	beforeFinishEntered := make(chan struct{})
	releaseFinish := make(chan struct{})
	manager.beforeSuccessFinalize = func() {
		close(beforeFinishEntered)
		<-releaseFinish
	}

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00800.MPLS", outputPath),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	<-runner.started

	close(runner.release)
	<-beforeFinishEntered

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- manager.StopCurrent()
	}()

	time.Sleep(20 * time.Millisecond)
	close(releaseFinish)

	if err := <-stopDone; err != nil {
		t.Fatalf("StopCurrent returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status after stop request, got %q", done.Status)
	}
	if !strings.Contains(strings.ToLower(done.Message), "canceled") {
		t.Fatalf("expected canceled message after stop request, got %q", done.Message)
	}
}

func TestManagerSuccessTransitionSetsCommandPreviewAndHundredPercent(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
	manager := NewManager(&stubRunner{output: "Progress: 42%\nProgress: 100%"})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00003.MPLS", outputPath),
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
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status, got %q", done.Status)
	}
	if done.ProgressPercent != 85 {
		t.Fatalf("expected last known progress 85, got %d", done.ProgressPercent)
	}
}

func TestManagerProgressUpdatesBeforeTerminalCompletion(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
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
		OutputPath:   outputPath,
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00003.MPLS", outputPath),
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
	if current.ProgressPercent != 76 {
		t.Fatalf("expected running progress 76 before release, got %d", current.ProgressPercent)
	}

	close(runner.release)
	done := waitForTerminalTask(t, manager)
	if done.ProgressPercent != 100 {
		t.Fatalf("expected final progress 100, got %d", done.ProgressPercent)
	}
}

func TestManagerProgressUpdatesFromCarriageReturnChunks(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
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
		OutputPath:   outputPath,
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00003.MPLS", outputPath),
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
	if current.ProgressPercent != 76 {
		t.Fatalf("expected carriage-return progress 76 while running, got %d", current.ProgressPercent)
	}

	close(runner.release)
	done := waitForTerminalTask(t, manager)
	if done.ProgressPercent != 100 {
		t.Fatalf("expected final progress 100, got %d", done.ProgressPercent)
	}
}

func TestManagerProgressUpdatesFromSplitMakeMKVSavingChunks(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")
	runner := &makeMKVSavingRunner{
		started:  make(chan struct{}),
		progress: make(chan struct{}),
		release:  make(chan struct{}),
	}
	manager := NewManager(runner)
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validIntermediatePayloadJSON("Nightcrawler Disc", "/tmp/Nightcrawler.mkv", "00003.MPLS", outputPath),
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
	if current.ProgressPercent != 1 {
		t.Fatalf("expected running MakeMKV progress 1 from split saving chunks, got %d", current.ProgressPercent)
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
		"draft":{
			"playlistName":"` + playlistName + `",
			"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},
			"audio":[],
			"subtitles":[],
			"makemkv":{"playlistName":"` + playlistName + `","titleId":4,"audio":[],"subtitles":[]}
		},
		"outputPath":"` + outputPath + `"
	}`
}

func validIntermediatePayloadJSON(sourceName, sourcePath, playlistName, outputPath string) string {
	return `{
		"source":{"name":"` + sourceName + `","path":"` + sourcePath + `"},
		"bdinfo":{"playlistName":"` + playlistName + `"},
		"draft":{"playlistName":"` + playlistName + `","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
		"outputPath":"` + outputPath + `"
	}`
}
