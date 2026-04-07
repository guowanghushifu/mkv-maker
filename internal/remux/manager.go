package remux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrTaskAlreadyRunning = errors.New("task already running")
var ErrTaskNotFound = errors.New("task not found")
var ErrManagerClosed = errors.New("manager is closed")

type Task struct {
	ID              string `json:"id"`
	SourceName      string `json:"sourceName"`
	OutputName      string `json:"outputName"`
	OutputPath      string `json:"outputPath"`
	PlaylistName    string `json:"playlistName"`
	CommandPreview  string `json:"commandPreview,omitempty"`
	ProgressPercent int    `json:"progressPercent"`
	CreatedAt       string `json:"createdAt"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
}

type StartRequest struct {
	SourceID              string
	SourceType            string
	SourceLeaseGeneration uint64
	SourceName            string
	OutputName            string
	OutputPath            string
	PlaylistName          string
	PayloadJSON           string
}

type taskState struct {
	task              Task
	log               string
	progressRemainder string
	cancel            context.CancelFunc
	stopRequested     bool
}

type Manager struct {
	mu             sync.RWMutex
	current        *taskState
	latest         *taskState
	executor       *JobRunner
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	closed         bool
	onTaskFinished func(StartRequest, Task)
	beforeSuccessFinalize func()
}

func NewManager(runner CommandRunner) *Manager {
	return NewManagerWithTempDir(runner, "")
}

func NewManagerWithTempDir(runner CommandRunner, tempDir string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	executor := NewJobRunner(runner)
	manager := &Manager{
		executor: executor,
		ctx:      ctx,
		cancel:   cancel,
	}
	executor.onCommandPreview = manager.updateCommandPreview
	if strings.TrimSpace(tempDir) != "" {
		executor.tempDir = func() string {
			return tempDir
		}
	}
	return manager
}

func (m *Manager) SetOnTaskFinished(fn func(StartRequest, Task)) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTaskFinished = fn
}

func (m *Manager) Start(req StartRequest) (Task, error) {
	if m == nil {
		return Task{}, ErrTaskNotFound
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Task{}, ErrManagerClosed
	}
	if m.current != nil {
		m.mu.Unlock()
		return Task{}, ErrTaskAlreadyRunning
	}

	id, err := generateTaskID()
	if err != nil {
		m.mu.Unlock()
		return Task{}, err
	}
	commandPreview, err := m.executor.CommandPreview(req)
	if err != nil {
		m.mu.Unlock()
		return Task{}, err
	}

	task := Task{
		ID:              id,
		SourceName:      strings.TrimSpace(req.SourceName),
		OutputName:      strings.TrimSpace(req.OutputName),
		OutputPath:      strings.TrimSpace(req.OutputPath),
		PlaylistName:    strings.TrimSpace(req.PlaylistName),
		CommandPreview:  commandPreview,
		ProgressPercent: 0,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Status:          "running",
	}
	taskCtx, taskCancel := context.WithCancel(m.ctx)
	state := &taskState{
		task:   task,
		log:    logLine("remux started"),
		cancel: taskCancel,
	}
	m.current = state
	m.latest = state
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		defer taskCancel()
		m.execute(taskCtx, state, req)
	}()

	return task, nil
}

func (m *Manager) StopCurrent() error {
	if m == nil {
		return ErrTaskNotFound
	}

	m.mu.Lock()
	state := m.current
	if state == nil || state.cancel == nil {
		m.mu.Unlock()
		return ErrTaskNotFound
	}

	state.stopRequested = true
	cancel := state.cancel
	state.cancel = nil
	m.mu.Unlock()
	cancel()
	return nil
}

func (m *Manager) Current() (Task, error) {
	if m == nil {
		return Task{}, ErrTaskNotFound
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current != nil {
		return m.current.task, nil
	}
	if m.latest != nil {
		return m.latest.task, nil
	}
	return Task{}, ErrTaskNotFound
}

func (m *Manager) CurrentLog() (string, error) {
	if m == nil {
		return "", ErrTaskNotFound
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current != nil {
		return m.current.log, nil
	}
	if m.latest != nil {
		return m.latest.log, nil
	}
	return "", ErrTaskNotFound
}

func (m *Manager) execute(ctx context.Context, state *taskState, req StartRequest) {
	if m == nil || state == nil || m.executor == nil {
		return
	}

	m.appendLog(state, logLine("running"))
	handleOutput := func(chunk string) {
		m.updateProgressFromOutput(state, chunk)
		m.appendLog(state, normalizeLogChunk(chunk))
	}
	output, streamed, err := m.executor.Execute(ctx, req, handleOutput)
	if !streamed && output != "" {
		handleOutput(output)
	}
	m.flushProgressRemainder(state)

	if err == nil && m.beforeSuccessFinalize != nil {
		m.beforeSuccessFinalize()
	}

	m.finishExecution(state, req, ctx, err)
}

func (m *Manager) finishExecution(state *taskState, req StartRequest, ctx context.Context, err error) {
	if m == nil || state == nil {
		return
	}

	var message string
	var status string
	if err != nil {
		message = normalizeRunnerError(err)
		status = "failed"
	}

	m.mu.Lock()
	canceled := state.stopRequested || (ctx != nil && ctx.Err() != nil)
	switch {
	case canceled:
		message = "remux canceled"
		status = "failed"
		state.log += logLine(message)
	case err != nil:
		state.log += logLine(message)
	default:
		status = "succeeded"
		state.task.ProgressPercent = 100
		state.log += logLine("completed")
	}
	state.task.Status = status
	state.task.Message = strings.TrimSpace(message)
	m.latest = state
	if m.current == state {
		m.current = nil
	}
	hook := m.onTaskFinished
	task := state.task
	m.mu.Unlock()

	if hook != nil {
		hook(req, task)
	}
}

func (m *Manager) Close() {
	if m == nil {
		return
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.wg.Wait()
		return
	}
	m.closed = true
	cancel := m.cancel
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

func (m *Manager) appendLog(state *taskState, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if state.log == "" {
		state.log = content
		return
	}
	state.log += content
}

func (m *Manager) updateCommandPreview(preview string) {
	if m == nil || strings.TrimSpace(preview) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil {
		m.current.task.CommandPreview = preview
	}
	if m.latest != nil {
		m.latest.task.CommandPreview = preview
	}
}

func (m *Manager) updateProgressFromOutput(state *taskState, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	percents, remainder := extractProgressPercentsFromChunk(state.progressRemainder, output)
	state.progressRemainder = remainder
	for _, progress := range percents {
		state.task.ProgressPercent = progress
	}
}

func (m *Manager) flushProgressRemainder(state *taskState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state.progressRemainder == "" {
		return
	}

	percents, remainder := extractProgressPercentsFromChunk(state.progressRemainder, "\n")
	state.progressRemainder = remainder
	for _, progress := range percents {
		state.task.ProgressPercent = progress
	}
}

func generateTaskID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "task-" + hex.EncodeToString(buf), nil
}
