package isomount

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type ReleaseResult struct {
	Released     int `json:"released"`
	SkippedInUse int `json:"skippedInUse"`
	Failed       int `json:"failed"`
}

type entry struct {
	ISOPath       string
	MountPath     string
	LastTouchedAt time.Time
	InUse         bool
}

type Manager struct {
	root        string
	idleTimeout time.Duration
	now         func() time.Time
	runner      CommandRunner

	mu          sync.Mutex
	entries     map[string]*entry
	sourceLocks map[string]*sync.Mutex
}

type systemRunner struct{}

func (systemRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

func NewManager(root string, idleTimeout time.Duration, runner CommandRunner) *Manager {
	if runner == nil {
		runner = systemRunner{}
	}
	return &Manager{
		root:        filepath.Clean(root),
		idleTimeout: idleTimeout,
		now:         time.Now,
		runner:      runner,
		entries:     map[string]*entry{},
		sourceLocks: map[string]*sync.Mutex{},
	}
}

func (m *Manager) EnsureMounted(ctx context.Context, sourceID, isoPath string) (string, error) {
	sourceKey := strings.TrimSpace(sourceID)
	if sourceKey == "" {
		return "", errors.New("source ID is required")
	}

	sourceLock := m.sourceLockFor(sourceKey)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	if existing := m.entries[sourceKey]; existing != nil && isMountedBDMVRoot(existing.MountPath) {
		existing.LastTouchedAt = m.now()
		m.mu.Unlock()
		return existing.MountPath, nil
	}
	mountPath := filepath.Join(m.root, sanitizeID(sourceKey))
	m.mu.Unlock()

	if err := os.MkdirAll(mountPath, 0o755); err != nil {
		return "", err
	}
	if err := m.runner.Run(ctx, "mount", "-o", "loop,ro", isoPath, mountPath); err != nil {
		return "", err
	}
	if !isMountedBDMVRoot(mountPath) {
		m.cleanupMount(ctx, mountPath)
		return "", errors.New("mounted ISO does not contain a valid BDMV structure")
	}

	m.mu.Lock()
	m.entries[sourceKey] = &entry{
		ISOPath:       isoPath,
		MountPath:     mountPath,
		LastTouchedAt: m.now(),
	}
	m.mu.Unlock()
	return mountPath, nil
}

func (m *Manager) sourceLockFor(sourceID string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lock := m.sourceLocks[sourceID]; lock != nil {
		return lock
	}

	lock := &sync.Mutex{}
	m.sourceLocks[sourceID] = lock
	return lock
}

func (m *Manager) cleanupMount(ctx context.Context, mountPath string) {
	_ = m.runner.Run(ctx, "umount", mountPath)
	_ = os.RemoveAll(mountPath)
}

func sanitizeID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	return url.PathEscape(filepath.ToSlash(filepath.Clean(trimmed)))
}

func isMountedBDMVRoot(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "BDMV", "PLAYLIST")); err == nil {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "BDMV", "index.bdmv"))
	return err == nil
}
