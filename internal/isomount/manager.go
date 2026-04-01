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

var ErrSourceInUse = errors.New("source is in use")

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
	mountLocks  map[string]*sync.Mutex
	mountOwners map[string]string
	pendingDirs map[string]struct{}
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
		mountLocks:  map[string]*sync.Mutex{},
		mountOwners: map[string]string{},
		pendingDirs: map[string]struct{}{},
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
	m.mu.Unlock()

	mountPath := filepath.Join(m.root, sanitizeID(sourceKey))
	mountLock := m.mountLockFor(mountPath)
	mountLock.Lock()
	defer mountLock.Unlock()
	m.mu.Lock()
	delete(m.pendingDirs, mountPath)
	m.mountOwners[mountPath] = sourceKey
	m.mu.Unlock()
	mounted := false
	defer func() {
		if mounted {
			return
		}
		m.mu.Lock()
		delete(m.mountOwners, mountPath)
		m.mu.Unlock()
	}()

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
	mounted = true
	return mountPath, nil
}

func (m *Manager) Touch(sourceID string) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry := m.entries[sourceID]; entry != nil {
		entry.LastTouchedAt = m.now()
	}
}

func (m *Manager) MarkInUse(sourceID string) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry := m.entries[sourceID]; entry != nil {
		entry.InUse = true
		entry.LastTouchedAt = m.now()
	}
}

func (m *Manager) MarkIdle(sourceID string) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if entry := m.entries[sourceID]; entry != nil {
		entry.InUse = false
		entry.LastTouchedAt = m.now()
	}
}

func (m *Manager) ReleaseIdleMounted(ctx context.Context) ReleaseResult {
	m.mu.Lock()
	ids := make([]string, 0, len(m.entries))
	for sourceID := range m.entries {
		ids = append(ids, sourceID)
	}
	m.mu.Unlock()

	var result ReleaseResult
	for _, sourceID := range ids {
		released, err := m.ReleaseSource(ctx, sourceID)
		switch {
		case released:
			result.Released++
		case errors.Is(err, ErrSourceInUse):
			result.SkippedInUse++
		case err == nil:
			// The source disappeared before release time; nothing to count.
		default:
			result.Failed++
		}
	}

	residual := m.CleanupResidualMountDirs(ctx)
	result.Released += residual.Released
	result.SkippedInUse += residual.SkippedInUse
	result.Failed += residual.Failed
	return result
}

func (m *Manager) releaseExpiredSource(ctx context.Context, sourceID string, now time.Time) (bool, error) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	entry := m.entries[sourceID]
	if entry == nil {
		m.mu.Unlock()
		return false, nil
	}
	if entry.InUse || now.Sub(entry.LastTouchedAt) <= m.idleTimeout {
		m.mu.Unlock()
		return false, nil
	}
	mountPath := entry.MountPath
	m.mu.Unlock()

	mountLock := m.mountLockFor(mountPath)
	mountLock.Lock()
	defer mountLock.Unlock()

	m.mu.Lock()
	current := m.entries[sourceID]
	currentOwner, ok := m.mountOwners[mountPath]
	if current == nil || current.MountPath != mountPath || (ok && currentOwner != sourceID) {
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()

	if !m.cleanupMountPath(ctx, mountPath) {
		return false, errors.New("failed to release source")
	}

	m.mu.Lock()
	if current = m.entries[sourceID]; current != nil && current.MountPath == mountPath {
		delete(m.entries, sourceID)
		delete(m.mountOwners, mountPath)
		delete(m.pendingDirs, mountPath)
	}
	m.mu.Unlock()
	return true, nil
}

func (m *Manager) CleanupExpiredIdle(ctx context.Context, now time.Time) ReleaseResult {
	m.mu.Lock()
	ids := make([]string, 0, len(m.entries))
	for sourceID, entry := range m.entries {
		if entry == nil || entry.InUse || now.Sub(entry.LastTouchedAt) <= m.idleTimeout {
			continue
		}
		ids = append(ids, sourceID)
	}
	m.mu.Unlock()

	var result ReleaseResult
	for _, sourceID := range ids {
		released, err := m.releaseExpiredSource(ctx, sourceID, now)
		switch {
		case released:
			result.Released++
		case errors.Is(err, ErrSourceInUse):
			result.SkippedInUse++
		case err == nil:
			// The entry was re-checked and is no longer idle, so it is left alone.
		default:
			result.Failed++
		}
	}
	return result
}

func (m *Manager) CleanupResidualMountDirs(ctx context.Context) ReleaseResult {
	entries, err := os.ReadDir(m.root)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return ReleaseResult{}
	}
	if err != nil {
		return ReleaseResult{Failed: 1}
	}

	var result ReleaseResult
	for _, dirEntry := range entries {
		if !dirEntry.IsDir() {
			continue
		}
		mountPath := filepath.Join(m.root, dirEntry.Name())
		mountLock := m.mountLockFor(mountPath)
		if !mountLock.TryLock() {
			continue
		}

		m.mu.Lock()
		_, tracked := m.mountOwners[mountPath]
		_, pending := m.pendingDirs[mountPath]
		m.mu.Unlock()

		if tracked {
			mountLock.Unlock()
			continue
		}

		if pending {
			if !m.cleanupMountPath(ctx, mountPath) {
				mountLock.Unlock()
				result.Failed++
				continue
			}
			m.mu.Lock()
			delete(m.pendingDirs, mountPath)
			m.mu.Unlock()
			mountLock.Unlock()
			result.Released++
			continue
		}

		if !isMountedBDMVRoot(mountPath) {
			mountLock.Unlock()
			continue
		}
		if !m.cleanupMountPath(ctx, mountPath) {
			mountLock.Unlock()
			result.Failed++
			continue
		}
		mountLock.Unlock()
		result.Released++
	}
	return result
}

func (m *Manager) CleanupAll(ctx context.Context) ReleaseResult {
	m.mu.Lock()
	ids := make([]string, 0, len(m.entries))
	for sourceID := range m.entries {
		ids = append(ids, sourceID)
	}
	m.mu.Unlock()

	var result ReleaseResult
	for _, sourceID := range ids {
		released, err := m.forceReleaseSource(ctx, sourceID)
		if err != nil {
			result.Failed++
			continue
		}
		if released {
			result.Released++
		}
	}
	return result
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

func (m *Manager) ReleaseSource(ctx context.Context, sourceID string) (bool, error) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	entry := m.entries[sourceID]
	if entry == nil {
		m.mu.Unlock()
		return false, nil
	}
	if entry.InUse {
		m.mu.Unlock()
		return false, ErrSourceInUse
	}
	mountPath := entry.MountPath
	m.mu.Unlock()

	mountLock := m.mountLockFor(mountPath)
	mountLock.Lock()
	defer mountLock.Unlock()

	m.mu.Lock()
	current := m.entries[sourceID]
	currentOwner, ok := m.mountOwners[mountPath]
	if current == nil || current.MountPath != mountPath || (ok && currentOwner != sourceID) {
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()

	if !m.cleanupMountPath(ctx, mountPath) {
		return false, errors.New("failed to release source")
	}

	m.mu.Lock()
	if current = m.entries[sourceID]; current != nil && current.MountPath == mountPath {
		delete(m.entries, sourceID)
		delete(m.mountOwners, mountPath)
		delete(m.pendingDirs, mountPath)
	}
	m.mu.Unlock()
	return true, nil
}

func (m *Manager) forceReleaseSource(ctx context.Context, sourceID string) (bool, error) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	mountPath := filepath.Join(m.root, sanitizeID(sourceID))

	m.mu.Lock()
	entry := m.entries[sourceID]
	if entry == nil {
		m.mu.Unlock()
		return false, nil
	}
	mountPath = entry.MountPath
	m.mu.Unlock()

	mountLock := m.mountLockFor(mountPath)
	mountLock.Lock()
	defer mountLock.Unlock()

	m.mu.Lock()
	current := m.entries[sourceID]
	currentOwner, ok := m.mountOwners[mountPath]
	if current == nil || current.MountPath != mountPath || (ok && currentOwner != sourceID) {
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()

	if !m.cleanupMountPath(ctx, mountPath) {
		return false, errors.New("failed to release source")
	}

	m.mu.Lock()
	if current = m.entries[sourceID]; current != nil && current.MountPath == mountPath {
		delete(m.entries, sourceID)
		delete(m.mountOwners, mountPath)
		delete(m.pendingDirs, mountPath)
	}
	m.mu.Unlock()
	return true, nil
}

func (m *Manager) cleanupMount(ctx context.Context, mountPath string) {
	_ = m.runner.Run(ctx, "umount", mountPath)
	_ = os.RemoveAll(mountPath)
}

func (m *Manager) cleanupMountPath(ctx context.Context, mountPath string) bool {
	m.mu.Lock()
	_, alreadyUnmounted := m.pendingDirs[mountPath]
	m.mu.Unlock()

	umountSucceeded := false
	if !alreadyUnmounted {
		if err := m.runner.Run(ctx, "umount", mountPath); err == nil {
			umountSucceeded = true
			m.mu.Lock()
			m.pendingDirs[mountPath] = struct{}{}
			m.mu.Unlock()
		}
	}

	if err := os.RemoveAll(mountPath); err != nil {
		return false
	}

	if alreadyUnmounted || umountSucceeded {
		m.mu.Lock()
		delete(m.pendingDirs, mountPath)
		m.mu.Unlock()
		return true
	}
	return false
}

func (m *Manager) mountLockFor(mountPath string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lock := m.mountLocks[mountPath]; lock != nil {
		return lock
	}

	lock := &sync.Mutex{}
	m.mountLocks[mountPath] = lock
	return lock
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

func (m *Manager) RunJanitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CleanupExpiredIdle(ctx, m.now())
		}
	}
}
