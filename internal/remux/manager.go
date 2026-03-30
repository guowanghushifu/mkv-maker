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
	SourceName   string
	OutputName   string
	OutputPath   string
	PlaylistName string
	PayloadJSON  string
}

type taskState struct {
	task              Task
	log               string
	progressRemainder string
}

type Manager struct {
	mu       sync.RWMutex
	current  *taskState
	latest   *taskState
	executor *JobRunner
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	closed   bool
}

func NewManager(runner CommandRunner) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		executor: NewJobRunner(runner),
		ctx:      ctx,
		cancel:   cancel,
	}
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
	state := &taskState{
		task: task,
		log:  logLine("remux started"),
	}
	m.current = state
	m.latest = state
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		m.execute(state, req)
	}()

	return task, nil
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

func (m *Manager) execute(state *taskState, req StartRequest) {
	if m == nil || state == nil || m.executor == nil {
		return
	}

	m.appendLog(state, logLine("running"))
	handleOutput := func(chunk string) {
		m.updateProgressFromOutput(state, chunk)
		m.appendLog(state, normalizeLogChunk(chunk))
	}
	output, streamed, err := m.executor.Execute(m.ctx, req, handleOutput)
	if !streamed && output != "" {
		handleOutput(output)
	}
	m.flushProgressRemainder(state)

	if err != nil {
		message := normalizeRunnerError(err)
		m.appendLog(state, logLine(message))
		m.finish(state, "failed", message)
		return
	}

	m.setProgress(state, 100)
	m.appendLog(state, logLine("completed"))
	m.finish(state, "succeeded", "")
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

func (m *Manager) finish(state *taskState, status, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state.task.Status = status
	state.task.Message = strings.TrimSpace(message)
	m.latest = state
	if m.current == state {
		m.current = nil
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

func (m *Manager) setProgress(state *taskState, progress int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state.task.ProgressPercent = progress
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
