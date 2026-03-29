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

type Task struct {
	ID           string `json:"id"`
	SourceName   string `json:"sourceName"`
	OutputName   string `json:"outputName"`
	OutputPath   string `json:"outputPath"`
	PlaylistName string `json:"playlistName"`
	CreatedAt    string `json:"createdAt"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
}

type StartRequest struct {
	SourceName   string
	OutputName   string
	OutputPath   string
	PlaylistName string
	PayloadJSON  string
}

type taskState struct {
	task        Task
	payloadJSON string
	log         string
}

type Manager struct {
	mu       sync.RWMutex
	current  *taskState
	latest   *taskState
	executor *JobRunner
}

func NewManager(runner CommandRunner) *Manager {
	return &Manager{
		executor: NewJobRunner(runner),
	}
}

func (m *Manager) Start(req StartRequest) (Task, error) {
	if m == nil {
		return Task{}, ErrTaskNotFound
	}

	m.mu.Lock()
	if m.current != nil {
		m.mu.Unlock()
		return Task{}, ErrTaskAlreadyRunning
	}

	id, err := generateTaskID()
	if err != nil {
		m.mu.Unlock()
		return Task{}, err
	}

	task := Task{
		ID:           id,
		SourceName:   strings.TrimSpace(req.SourceName),
		OutputName:   strings.TrimSpace(req.OutputName),
		OutputPath:   strings.TrimSpace(req.OutputPath),
		PlaylistName: strings.TrimSpace(req.PlaylistName),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		Status:       "running",
	}
	state := &taskState{
		task:        task,
		payloadJSON: req.PayloadJSON,
		log:         logLine("remux started"),
	}
	m.current = state
	m.latest = state
	m.mu.Unlock()

	go m.execute(state, req)

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
	output, err := m.executor.Execute(context.Background(), req)
	if output != "" {
		m.appendLog(state, normalizeLogChunk(output))
	}

	if err != nil {
		message := normalizeRunnerError(err)
		m.appendLog(state, logLine(message))
		m.finish(state, "failed", message)
		return
	}

	m.appendLog(state, logLine("completed"))
	m.finish(state, "succeeded", "")
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

func generateTaskID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "task-" + hex.EncodeToString(buf), nil
}
