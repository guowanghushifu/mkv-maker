package isomount

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
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
	ISOPath         string
	MountPath       string
	MountGeneration uint64
	LeaseGeneration uint64
	LastTouchedAt   time.Time
	InUse           bool
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
	retryMounts map[string]struct{}
	nextGen     uint64
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
		retryMounts: map[string]struct{}{},
		nextGen:     1,
	}
}

func (m *Manager) EnsureMounted(ctx context.Context, sourceID, isoPath string) (string, error) {
	mountPath, _, err := m.ensureMounted(ctx, sourceID, isoPath, false)
	return mountPath, err
}

func (m *Manager) EnsureMountedAndAcquireLease(ctx context.Context, sourceID, isoPath string) (string, uint64, error) {
	return m.ensureMounted(ctx, sourceID, isoPath, true)
}

func (m *Manager) ensureMounted(ctx context.Context, sourceID, isoPath string, claimLease bool) (string, uint64, error) {
	sourceKey := strings.TrimSpace(sourceID)
	if sourceKey == "" {
		return "", 0, errors.New("source ID is required")
	}

	sourceLock := m.sourceLockFor(sourceKey)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	existing := m.entries[sourceKey]
	staleMountPath := ""
	if existing != nil && isMountedBDMVRoot(existing.MountPath) {
		existing.LastTouchedAt = m.now()
		var leaseGeneration uint64
		if claimLease {
			leaseGeneration = m.nextGen
			m.nextGen++
			existing.LeaseGeneration = leaseGeneration
			existing.InUse = true
		}
		m.mu.Unlock()
		return existing.MountPath, leaseGeneration, nil
	}
	if existing != nil {
		staleMountPath = existing.MountPath
	}
	mountPath := filepath.Join(m.root, buildMountDirName(m.root, isoPath))
	preexistingPending := false
	staleTracked := existing != nil
	if _, pending := m.pendingDirs[mountPath]; pending {
		preexistingPending = true
		if currentOwner, ok := m.mountOwners[mountPath]; ok && currentOwner != sourceKey {
			m.mu.Unlock()
			return "", 0, errors.New("mount path is already owned")
		}
	}
	m.mu.Unlock()

	mountLock := m.mountLockFor(mountPath)
	mountLock.Lock()
	defer mountLock.Unlock()
	m.mu.Lock()
	currentOwner, owned := m.mountOwners[mountPath]
	_, pending := m.pendingDirs[mountPath]
	if (owned && currentOwner != sourceKey) || (pending && currentOwner != "" && currentOwner != sourceKey) {
		m.mu.Unlock()
		return "", 0, errors.New("mount path is already owned")
	}
	if staleTracked {
		delete(m.entries, sourceKey)
		delete(m.mountOwners, staleMountPath)
		delete(m.mountOwners, mountPath)
	}
	m.mu.Unlock()

	if err := os.MkdirAll(mountPath, 0o755); err != nil {
		return "", 0, err
	}
	if err := m.runner.Run(ctx, "mount", "-o", "loop,ro", isoPath, mountPath); err != nil {
		return "", 0, err
	}
	if !isMountedBDMVRoot(mountPath) {
		m.cleanupInvalidMountedContent(ctx, mountPath, preexistingPending)
		return "", 0, errors.New("mounted ISO does not contain a valid BDMV structure")
	}

	m.mu.Lock()
	mountGeneration := m.nextGen
	m.nextGen++
	m.mountOwners[mountPath] = sourceKey
	delete(m.pendingDirs, mountPath)
	leaseGeneration := uint64(0)
	newEntry := &entry{
		ISOPath:         isoPath,
		MountPath:       mountPath,
		MountGeneration: mountGeneration,
		LastTouchedAt:   m.now(),
	}
	if claimLease {
		leaseGeneration = m.nextGen
		m.nextGen++
		newEntry.LeaseGeneration = leaseGeneration
		newEntry.InUse = true
	}
	m.entries[sourceKey] = newEntry
	m.mu.Unlock()
	return mountPath, leaseGeneration, nil
}

func (m *Manager) CurrentGeneration(sourceID string) (uint64, bool) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entries[sourceID]
	if entry == nil {
		return 0, false
	}
	return entry.MountGeneration, true
}

// AcquireLease mints a new job lease token for the mounted source.
func (m *Manager) AcquireLease(sourceID string) (uint64, bool) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entries[sourceID]
	if entry == nil {
		return 0, false
	}
	generation := m.nextGen
	m.nextGen++
	entry.LeaseGeneration = generation
	entry.InUse = true
	entry.LastTouchedAt = m.now()
	return generation, true
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
		_, retrying := m.retryMounts[mountPath]
		_, pending := m.pendingDirs[mountPath]
		m.mu.Unlock()

		if tracked {
			mountLock.Unlock()
			continue
		}

		if retrying {
			if err := m.runner.Run(ctx, "umount", mountPath); err != nil {
				mountLock.Unlock()
				result.Failed++
				continue
			}
			m.mu.Lock()
			delete(m.retryMounts, mountPath)
			m.mu.Unlock()
			if err := os.RemoveAll(mountPath); err != nil {
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
			if err := os.RemoveAll(mountPath); err != nil {
				mountLock.Unlock()
				result.Failed++
				continue
			}
			mountLock.Unlock()
			result.Released++
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
	residual := m.CleanupResidualMountDirs(ctx)
	result.Released += residual.Released
	result.SkippedInUse += residual.SkippedInUse
	result.Failed += residual.Failed
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
	return m.releaseSource(ctx, sourceID, 0, false, false)
}

func (m *Manager) ReleaseSourceIfGeneration(ctx context.Context, sourceID string, generation uint64) (bool, error) {
	return m.releaseSource(ctx, sourceID, generation, true, false)
}

// ReleaseSourceIfLeaseGeneration ignores stale completion hooks from earlier leases.
func (m *Manager) ReleaseSourceIfLeaseGeneration(ctx context.Context, sourceID string, generation uint64) (bool, error) {
	return m.releaseSource(ctx, sourceID, generation, true, true)
}

func (m *Manager) releaseSource(ctx context.Context, sourceID string, generation uint64, guardGeneration, guardLease bool) (bool, error) {
	sourceLock := m.sourceLockFor(sourceID)
	sourceLock.Lock()
	defer sourceLock.Unlock()

	m.mu.Lock()
	entry := m.entries[sourceID]
	if entry == nil {
		m.mu.Unlock()
		return false, nil
	}
	currentGeneration := entry.MountGeneration
	if guardLease {
		currentGeneration = entry.LeaseGeneration
	}
	if guardGeneration && currentGeneration != generation {
		m.mu.Unlock()
		return false, nil
	}
	if entry.InUse && !guardGeneration {
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
	currentGeneration = 0
	if current != nil {
		currentGeneration = current.MountGeneration
		if guardLease {
			currentGeneration = current.LeaseGeneration
		}
	}
	if current == nil || current.MountPath != mountPath || (ok && currentOwner != sourceID) || (guardGeneration && currentGeneration != generation) {
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()

	if !m.cleanupMountPath(ctx, mountPath) {
		m.mu.Lock()
		if current = m.entries[sourceID]; current != nil && current.MountPath == mountPath {
			current.InUse = false
			if guardLease {
				current.LeaseGeneration = 0
			}
		}
		m.mu.Unlock()
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

func (m *Manager) cleanupInvalidMountedContent(ctx context.Context, mountPath string, preservePending bool) bool {
	if err := m.runner.Run(ctx, "umount", mountPath); err != nil {
		m.mu.Lock()
		m.retryMounts[mountPath] = struct{}{}
		if preservePending {
			m.pendingDirs[mountPath] = struct{}{}
		}
		m.mu.Unlock()
		return false
	}

	m.mu.Lock()
	delete(m.retryMounts, mountPath)
	if !preservePending {
		m.pendingDirs[mountPath] = struct{}{}
	}
	m.mu.Unlock()

	if err := os.RemoveAll(mountPath); err != nil {
		m.mu.Lock()
		if !preservePending {
			m.pendingDirs[mountPath] = struct{}{}
		}
		m.mu.Unlock()
		return false
	}

	m.mu.Lock()
	delete(m.retryMounts, mountPath)
	delete(m.pendingDirs, mountPath)
	m.mu.Unlock()
	return true
}

func (m *Manager) cleanupMountPath(ctx context.Context, mountPath string) bool {
	m.mu.Lock()
	_, alreadyUnmounted := m.pendingDirs[mountPath]
	_, retrying := m.retryMounts[mountPath]
	m.mu.Unlock()

	umountSucceeded := false
	if retrying || !alreadyUnmounted {
		if err := m.runner.Run(ctx, "umount", mountPath); err == nil {
			umountSucceeded = true
			m.mu.Lock()
			m.pendingDirs[mountPath] = struct{}{}
			delete(m.retryMounts, mountPath)
			m.mu.Unlock()
		}
	}

	if err := os.RemoveAll(mountPath); err != nil {
		return false
	}

	if alreadyUnmounted || umountSucceeded {
		m.mu.Lock()
		delete(m.pendingDirs, mountPath)
		delete(m.retryMounts, mountPath)
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

func buildMountDirName(root, isoPath string) string {
	hashInput := normalizedMountHashInput(root, isoPath)
	prefix := normalizeMountPrefix(strings.TrimSuffix(filepath.Base(isoPath), filepath.Ext(isoPath)))
	if prefix == "" {
		prefix = "iso"
	}
	prefix = truncateRunes(prefix, 40)
	return prefix + "-" + shortMountHash(hashInput)
}

func normalizedMountHashInput(root, isoPath string) string {
	inputRoot := filepath.Dir(filepath.Clean(root))
	if rel, err := filepath.Rel(inputRoot, isoPath); err == nil {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel != "." && !strings.HasPrefix(rel, "../") {
			return rel
		}
	}
	return filepath.ToSlash(filepath.Clean(isoPath))
}

func normalizeMountPrefix(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case unicode.IsSpace(r) || strings.ContainsRune("._-()[]{}", r):
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func shortMountHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
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
