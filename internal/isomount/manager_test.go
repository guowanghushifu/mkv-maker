package isomount

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeCommandRunner struct {
	runFn func(ctx context.Context, name string, args ...string) error
	calls map[string]int
}

func (r *fakeCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.calls[name]++
	if r.runFn != nil {
		return r.runFn(ctx, name, args...)
	}
	return nil
}

func TestManagerEnsureMountedMountsISOAndReturnsWorkspace(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "mount" {
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644)
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)

	mountPath, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath)
	if err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}
	if mountPath != filepath.Join(root, "movies-nightcrawler-iso") {
		t.Fatalf("unexpected mount path %q", mountPath)
	}
	if runner.calls["mount"] != 1 {
		t.Fatalf("expected one mount call, got %d", runner.calls["mount"])
	}
}

func TestManagerReleaseSourceIfGenerationSkipsReacquiredMount(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644)
			case "umount":
				umountCalls++
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)

	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatalf("first EnsureMounted returned error: %v", err)
	}
	firstGeneration, ok := manager.CurrentGeneration("movies-nightcrawler-iso")
	if !ok {
		t.Fatal("expected first generation to be available")
	}
	if released, err := manager.ReleaseSourceIfGeneration(context.Background(), "movies-nightcrawler-iso", firstGeneration); err != nil || !released {
		t.Fatalf("expected first release to succeed, released=%v err=%v", released, err)
	}

	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatalf("second EnsureMounted returned error: %v", err)
	}
	secondGeneration, ok := manager.CurrentGeneration("movies-nightcrawler-iso")
	if !ok {
		t.Fatal("expected second generation to be available")
	}
	if secondGeneration == firstGeneration {
		t.Fatal("expected reacquired mount to receive a new generation")
	}

	if released, err := manager.ReleaseSourceIfGeneration(context.Background(), "movies-nightcrawler-iso", firstGeneration); err != nil {
		t.Fatalf("stale release returned error: %v", err)
	} else if released {
		t.Fatal("expected stale generation release to be skipped")
	}
	if umountCalls != 1 {
		t.Fatalf("expected only one umount call, got %d", umountCalls)
	}
	currentGeneration, ok := manager.CurrentGeneration("movies-nightcrawler-iso")
	if !ok || currentGeneration != secondGeneration {
		t.Fatalf("expected current generation %d to remain, got %d ok=%v", secondGeneration, currentGeneration, ok)
	}
}

func TestManagerReleaseSourceIfLeaseGenerationSkipsImmediateSameSourceHandoff(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644)
			case "umount":
				umountCalls++
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)

	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatalf("first EnsureMounted returned error: %v", err)
	}
	firstLease, ok := manager.AcquireLease("movies-nightcrawler-iso")
	if !ok {
		t.Fatal("expected first lease to be available")
	}
	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatalf("second EnsureMounted returned error: %v", err)
	}
	secondLease, ok := manager.AcquireLease("movies-nightcrawler-iso")
	if !ok {
		t.Fatal("expected second lease to be available")
	}
	if secondLease == firstLease {
		t.Fatal("expected immediate handoff to receive a new lease generation")
	}
	if runner.calls["mount"] != 1 {
		t.Fatalf("expected same mount to be reused, got %d mount calls", runner.calls["mount"])
	}

	if released, err := manager.ReleaseSourceIfLeaseGeneration(context.Background(), "movies-nightcrawler-iso", firstLease); err != nil {
		t.Fatalf("stale lease release returned error: %v", err)
	} else if released {
		t.Fatal("expected stale lease release to be skipped")
	}
	if umountCalls != 0 {
		t.Fatalf("expected stale lease release to leave mount intact, got %d umount calls", umountCalls)
	}

	if released, err := manager.ReleaseSourceIfLeaseGeneration(context.Background(), "movies-nightcrawler-iso", secondLease); err != nil || !released {
		t.Fatalf("expected active lease release to succeed, released=%v err=%v", released, err)
	}
	if umountCalls != 1 {
		t.Fatalf("expected one umount call after active lease release, got %d", umountCalls)
	}
	mountPath := filepath.Join(root, "movies-nightcrawler-iso")
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected mount dir cleanup after active release, got stat error %v", statErr)
	}
}

func TestManagerEnsureMountedReusesHealthyMount(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name != "mount" {
				return nil
			}
			mountPath := args[len(args)-1]
			if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644)
		},
	}
	manager := NewManager(root, time.Hour, runner)

	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatal(err)
	}
	if runner.calls["mount"] != 1 {
		t.Fatalf("expected reuse without a second mount, got %d calls", runner.calls["mount"])
	}
}

func TestManagerEnsureMountedRejectsEmptySourceID(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCommandRunner{}
	manager := NewManager(root, time.Hour, runner)

	if _, err := manager.EnsureMounted(context.Background(), "   ", isoPath); err == nil {
		t.Fatal("expected error for empty source ID")
	}
	if runner.calls["mount"] != 0 {
		t.Fatalf("expected no mount call, got %d", runner.calls["mount"])
	}
}

func TestManagerEnsureMountedCleansUpInvalidMountedContent(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			mountPath := args[len(args)-1]
			switch name {
			case "mount":
				return os.MkdirAll(filepath.Join(mountPath, "BDMV"), 0o755)
			case "umount":
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)

	mountPath, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath)
	if err == nil {
		t.Fatal("expected error for invalid mounted content")
	}
	if mountPath != "" {
		t.Fatalf("expected empty mount path on failure, got %q", mountPath)
	}
	if runner.calls["mount"] != 1 {
		t.Fatalf("expected one mount call, got %d", runner.calls["mount"])
	}
	if runner.calls["umount"] != 1 {
		t.Fatalf("expected one umount call, got %d", runner.calls["umount"])
	}
	if _, statErr := os.Stat(filepath.Join(root, "movies-nightcrawler-iso")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected mount directory cleanup, got stat error %v", statErr)
	}
}

func TestManagerEnsureMountedRetriesInvalidContentCleanupAfterUnmountFailure(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	mountPath := filepath.Join(root, "movies-nightcrawler-iso")
	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV"), 0o755); err != nil {
					return err
				}
				return nil
			case "umount":
				umountCalls++
				if umountCalls == 1 {
					return errors.New("device busy")
				}
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)

	if _, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err == nil {
		t.Fatal("expected invalid mounted content to fail")
	}
	if _, statErr := os.Stat(mountPath); statErr != nil {
		t.Fatalf("expected mount dir to remain after failed unmount, got %v", statErr)
	}

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if umountCalls != 2 {
		t.Fatalf("expected residual cleanup to retry unmount, got %d umount calls", umountCalls)
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected leftover directory cleanup, got stat error %v", statErr)
	}
}

func TestManagerEnsureMountedLeavesStaleTrackedEntryCleanupRecoverableAfterFailedRemount(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/stale-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(mountPath, "BDMV")); err != nil {
		t.Fatal(err)
	}
	isoPath := filepath.Join(t.TempDir(), "stale.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mountCalls int
	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountCalls++
				if err := os.RemoveAll(mountPath); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV"), 0o755); err != nil {
					return err
				}
				return nil
			case "umount":
				umountCalls++
				if umountCalls == 1 {
					return errors.New("device busy")
				}
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.entries[sourceID] = &entry{ISOPath: "/bd_input/stale.iso", MountPath: mountPath, LastTouchedAt: time.Now()}
	manager.mountOwners[mountPath] = sourceID
	manager.pendingDirs[mountPath] = struct{}{}

	if _, err := manager.EnsureMounted(context.Background(), sourceID, isoPath); err == nil {
		t.Fatal("expected remount attempt to fail validation")
	}
	if mountCalls != 1 {
		t.Fatalf("expected one remount attempt, got %d", mountCalls)
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected stale tracked entry to be cleared after failed remount")
	}
	if _, ok := manager.mountOwners[mountPath]; ok {
		t.Fatal("expected stale mount owner to be cleared after failed remount")
	}
	if _, retrying := manager.retryMounts[mountPath]; !retrying {
		t.Fatal("expected failed remount to leave retry state")
	}
	if _, pending := manager.pendingDirs[mountPath]; !pending {
		t.Fatal("expected failed remount to preserve pending state")
	}

	residual := manager.CleanupResidualMountDirs(context.Background())
	if residual.Released != 1 || residual.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", residual)
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected stale tracked entry to be removed after cleanup")
	}
	if _, ok := manager.mountOwners[mountPath]; ok {
		t.Fatal("expected stale mount owner to be removed after cleanup")
	}
	if _, ok := manager.retryMounts[mountPath]; ok {
		t.Fatal("expected retry state to be cleared after cleanup")
	}
	if _, ok := manager.pendingDirs[mountPath]; ok {
		t.Fatal("expected pending state to be cleared after residual cleanup")
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected mount dir to be removed after residual cleanup, got stat error %v", statErr)
	}
}

func TestManagerEnsureMountedSerializesSameSourceCalls(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name != "mount" {
				return nil
			}
			select {
			case <-started:
			default:
				close(started)
			}
			<-release
			mountPath := args[len(args)-1]
			if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644)
		},
	}
	manager := NewManager(root, time.Hour, runner)

	firstDone := make(chan error, 1)
	go func() {
		_, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath)
		firstDone <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first mount to start")
	}

	secondDone := make(chan error, 1)
	go func() {
		_, err := manager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath)
		secondDone <- err
	}()

	close(release)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first EnsureMounted returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first EnsureMounted")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second EnsureMounted returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second EnsureMounted")
	}

	if runner.calls["mount"] != 1 {
		t.Fatalf("expected exactly one mount call, got %d", runner.calls["mount"])
	}
}

func TestManagerEnsureMountedRejectsCollidingClaimAfterMountLock(t *testing.T) {
	root := t.TempDir()
	primaryID := "library/../disc"
	secondaryID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	mountPath := filepath.Join(root, sanitizeID(primaryID))
	primaryEntered := make(chan struct{})
	releasePrimary := make(chan struct{})
	var mountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name != "mount" {
				return nil
			}
			mountCalls++
			if mountCalls == 1 {
				close(primaryEntered)
				<-releasePrimary
			}
			if err := os.MkdirAll(filepath.Join(args[len(args)-1], "BDMV", "PLAYLIST"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(args[len(args)-1], "BDMV", "index.bdmv"), []byte("index"), 0o644)
		},
	}
	manager := NewManager(root, time.Hour, runner)

	primaryDone := make(chan struct {
		path string
		err  error
	}, 1)
	go func() {
		path, err := manager.EnsureMounted(context.Background(), primaryID, isoPath)
		primaryDone <- struct {
			path string
			err  error
		}{path: path, err: err}
	}()

	select {
	case <-primaryEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for primary mount to start")
	}

	secondaryStarted := make(chan struct{})
	secondaryDone := make(chan struct {
		path string
		err  error
	}, 1)
	go func() {
		close(secondaryStarted)
		path, err := manager.EnsureMounted(context.Background(), secondaryID, isoPath)
		secondaryDone <- struct {
			path string
			err  error
		}{path: path, err: err}
	}()

	select {
	case <-secondaryStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for secondary claimant to start")
	}

	close(releasePrimary)

	select {
	case outcome := <-primaryDone:
		if outcome.err != nil {
			t.Fatalf("primary EnsureMounted returned error: %v", outcome.err)
		}
		if outcome.path != mountPath {
			t.Fatalf("unexpected primary mount path %q", outcome.path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for primary EnsureMounted")
	}

	select {
	case outcome := <-secondaryDone:
		if outcome.err == nil {
			t.Fatal("expected secondary claimant to be rejected before mount")
		}
		if outcome.path != "" {
			t.Fatalf("expected no mount path for rejected claimant, got %q", outcome.path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for secondary EnsureMounted")
	}

	if mountCalls != 1 {
		t.Fatalf("expected only the primary mount attempt, got %d calls", mountCalls)
	}
	if owner := manager.mountOwners[mountPath]; owner != primaryID {
		t.Fatalf("expected primary owner %q to remain, got %q", primaryID, owner)
	}
	if _, ok := manager.entries[secondaryID]; ok {
		t.Fatal("expected rejected claimant to stay untracked")
	}
}

func TestManagerReleaseIdleMountedSkipsInUseAndContinuesAfterUnmountFailure(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	orphanPath := filepath.Join(root, "orphan-disc")
	if err := os.MkdirAll(filepath.Join(orphanPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	busyID := "library/busy-disc"
	busyPath := filepath.Join(root, sanitizeID(busyID))
	if err := os.MkdirAll(filepath.Join(busyPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(busyPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	var umountPaths []string
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" {
				umountPaths = append(umountPaths, args[0])
			}
			if name == "umount" && strings.Contains(args[0], "broken-disc") {
				return errors.New("device busy")
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.now = func() time.Time { return now }
	manager.entries["idle-disc"] = &entry{ISOPath: "/bd_input/idle.iso", MountPath: filepath.Join(root, "idle-disc"), LastTouchedAt: now.Add(-2 * time.Hour)}
	manager.entries[busyID] = &entry{ISOPath: "/bd_input/busy.iso", MountPath: busyPath, LastTouchedAt: now.Add(-2 * time.Hour), InUse: true}
	manager.entries["broken-disc"] = &entry{ISOPath: "/bd_input/broken.iso", MountPath: filepath.Join(root, "broken-disc"), LastTouchedAt: now.Add(-2 * time.Hour)}
	manager.mountOwners[busyPath] = busyID

	result := manager.ReleaseIdleMounted(context.Background())
	if result.Released != 2 || result.SkippedInUse != 1 || result.Failed != 1 {
		t.Fatalf("unexpected release summary %+v", result)
	}
	if _, statErr := os.Stat(orphanPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected orphan mount dir cleanup, got stat error %v", statErr)
	}
	foundOrphan := false
	for _, mountPath := range umountPaths {
		if mountPath == orphanPath {
			foundOrphan = true
			break
		}
	}
	if !foundOrphan {
		t.Fatalf("expected orphan mount dir to be unmounted, saw paths %v", umountPaths)
	}
	for _, mountPath := range umountPaths {
		if mountPath == busyPath {
			t.Fatalf("expected sanitized in-use mount to stay mounted, saw paths %v", umountPaths)
		}
	}
}

func TestManagerReleaseIdleMountedCountsOnlyActualReleasesWhenSourceDisappears(t *testing.T) {
	root := t.TempDir()
	firstID := "first-disc"
	secondID := "second-disc"
	firstPath := filepath.Join(root, firstID)
	secondPath := filepath.Join(root, secondID)
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Join(path, "BDMV", "PLAYLIST"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	releaseFirst := make(chan struct{})
	firstUnmounted := make(chan string, 1)
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name != "umount" {
				return nil
			}
			select {
			case firstUnmounted <- args[0]:
				<-releaseFirst
			default:
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.entries[firstID] = &entry{ISOPath: "/bd_input/first.iso", MountPath: firstPath, LastTouchedAt: time.Now()}
	manager.entries[secondID] = &entry{ISOPath: "/bd_input/second.iso", MountPath: secondPath, LastTouchedAt: time.Now()}
	manager.mountOwners[firstPath] = firstID
	manager.mountOwners[secondPath] = secondID

	done := make(chan ReleaseResult, 1)
	go func() {
		done <- manager.ReleaseIdleMounted(context.Background())
	}()

	blockedPath := <-firstUnmounted
	missingID := secondID
	missingPath := secondPath
	if blockedPath == secondPath {
		missingID = firstID
		missingPath = firstPath
	}
	manager.mu.Lock()
	delete(manager.entries, missingID)
	delete(manager.mountOwners, missingPath)
	manager.mu.Unlock()
	if err := os.RemoveAll(missingPath); err != nil {
		t.Fatal(err)
	}
	close(releaseFirst)

	result := <-done
	if result.Released != 1 || result.SkippedInUse != 0 || result.Failed != 0 {
		t.Fatalf("unexpected release summary %+v", result)
	}
}

func TestManagerCleanupResidualMountDirsRemovesLeftoverUnmountedDirAfterRestart(t *testing.T) {
	root := t.TempDir()
	leftoverPath := filepath.Join(root, "movies-nightcrawler-iso")
	if err := os.MkdirAll(leftoverPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leftoverPath, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(root, time.Hour, &fakeCommandRunner{})

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if _, statErr := os.Stat(leftoverPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected leftover directory cleanup, got stat error %v", statErr)
	}
}

func TestManagerCleanupExpiredIdleReleasesOnlyStaleEntries(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	manager := NewManager(root, time.Hour, &fakeCommandRunner{})
	manager.now = func() time.Time { return now }
	manager.entries["stale-disc"] = &entry{ISOPath: "/bd_input/stale.iso", MountPath: filepath.Join(root, "stale-disc"), LastTouchedAt: now.Add(-2 * time.Hour)}
	manager.entries["exact-disc"] = &entry{ISOPath: "/bd_input/exact.iso", MountPath: filepath.Join(root, "exact-disc"), LastTouchedAt: now.Add(-1 * time.Hour)}
	manager.entries["fresh-disc"] = &entry{ISOPath: "/bd_input/fresh.iso", MountPath: filepath.Join(root, "fresh-disc"), LastTouchedAt: now.Add(-15 * time.Minute)}

	result := manager.CleanupExpiredIdle(context.Background(), now)
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}
	if _, ok := manager.entries["exact-disc"]; !ok {
		t.Fatal("expected entry touched exactly at idle timeout to remain tracked")
	}
}

func TestManagerCleanupExpiredIdleRetainsEntryOnFailure(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	mountPath := filepath.Join(root, "stale-disc")
	for _, path := range []string{mountPath} {
		if err := os.MkdirAll(filepath.Join(path, "BDMV", "PLAYLIST"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" {
				return errors.New("device busy")
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.now = func() time.Time { return now }
	manager.entries["stale-disc"] = &entry{ISOPath: "/bd_input/stale.iso", MountPath: mountPath, LastTouchedAt: now.Add(-2 * time.Hour)}

	result := manager.CleanupExpiredIdle(context.Background(), now)
	if result.Released != 0 || result.Failed != 1 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}
	if _, ok := manager.entries["stale-disc"]; !ok {
		t.Fatal("expected tracked entry to remain for retry after cleanup failure")
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected best-effort directory removal, got stat error %v", statErr)
	}
}

func TestManagerCleanupExpiredIdleSkipsTouchedCandidateAfterSelection(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	firstPath := filepath.Join(root, "first-disc")
	secondPath := filepath.Join(root, "second-disc")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Join(path, "BDMV", "PLAYLIST"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	started := make(chan struct{})
	releaseFirst := make(chan struct{})
	blockedPath := make(chan string, 1)
	var umountPaths []string
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" {
				umountPaths = append(umountPaths, args[0])
				select {
				case blockedPath <- args[0]:
					select {
					case <-started:
					default:
						close(started)
					}
					<-releaseFirst
				default:
				}
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.now = func() time.Time { return now }
	manager.entries["first-disc"] = &entry{ISOPath: "/bd_input/first.iso", MountPath: firstPath, LastTouchedAt: now.Add(-2 * time.Hour)}
	manager.entries["second-disc"] = &entry{ISOPath: "/bd_input/second.iso", MountPath: secondPath, LastTouchedAt: now.Add(-2 * time.Hour)}

	done := make(chan ReleaseResult, 1)
	go func() {
		done <- manager.CleanupExpiredIdle(context.Background(), now)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cleanup to start")
	}

	blocked := <-blockedPath
	var touchedID, touchedPath string
	switch blocked {
	case firstPath:
		touchedID = "second-disc"
		touchedPath = secondPath
	case secondPath:
		touchedID = "first-disc"
		touchedPath = firstPath
	default:
		t.Fatalf("unexpected blocked path %q", blocked)
	}
	manager.Touch(touchedID)
	close(releaseFirst)

	result := <-done
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}
	if _, ok := manager.entries[touchedID]; !ok {
		t.Fatalf("expected touched entry %q to remain tracked", touchedID)
	}
	releasedID := "first-disc"
	if touchedID == "first-disc" {
		releasedID = "second-disc"
	}
	if _, ok := manager.entries[releasedID]; ok {
		t.Fatalf("expected stale entry %q to be released", releasedID)
	}
	for _, mountPath := range umountPaths {
		if mountPath == touchedPath {
			t.Fatalf("expected refreshed entry not to be unmounted, saw paths %v", umountPaths)
		}
	}
}

func TestManagerCleanupResidualMountDirsBestEffort(t *testing.T) {
	root := t.TempDir()
	okPath := filepath.Join(root, "ok-disc")
	failPath := filepath.Join(root, "fail-disc")
	for _, path := range []string{okPath, failPath} {
		if err := os.MkdirAll(filepath.Join(path, "BDMV", "PLAYLIST"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" && strings.Contains(args[0], "fail-disc") {
				return errors.New("device busy")
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 1 || result.Failed != 1 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}
	if _, statErr := os.Stat(failPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected failed residual dir to be removed best-effort, got stat error %v", statErr)
	}
}

func TestManagerCleanupResidualMountDirsSkipsInProgressMount(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/in-progress-disc"
	isoPath := filepath.Join(t.TempDir(), "in-progress.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	mountPath := filepath.Join(root, sanitizeID(sourceID))
	mounted := make(chan struct{})
	releaseMount := make(chan struct{})
	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
					return err
				}
				close(mounted)
				<-releaseMount
				return nil
			case "umount":
				umountCalls++
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)

	done := make(chan struct {
		path string
		err  error
	}, 1)
	go func() {
		path, err := manager.EnsureMounted(context.Background(), sourceID, isoPath)
		done <- struct {
			path string
			err  error
		}{path: path, err: err}
	}()

	select {
	case <-mounted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mount to reach validation")
	}

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 0 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if umountCalls != 0 {
		t.Fatalf("expected in-progress mount not to be unmounted, got %d calls", umountCalls)
	}
	if _, statErr := os.Stat(mountPath); statErr != nil {
		t.Fatalf("expected mount dir to remain during in-progress mount, got %v", statErr)
	}

	close(releaseMount)

	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatalf("EnsureMounted returned error: %v", outcome.err)
		}
		if outcome.path != mountPath {
			t.Fatalf("unexpected mount path %q", outcome.path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EnsureMounted to finish")
	}

	if _, ok := manager.entries[sourceID]; !ok {
		t.Fatal("expected completed mount to remain tracked")
	}
}

func TestManagerEnsureMountedClearsStalePendingDirState(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/reused-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	isoPath := filepath.Join(t.TempDir(), "reused.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountCalls++
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
					return err
				}
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.pendingDirs[mountPath] = struct{}{}
	mountPathResult, err := manager.EnsureMounted(context.Background(), sourceID, isoPath)
	if err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}
	if mountPathResult != mountPath {
		t.Fatalf("unexpected mount path %q", mountPathResult)
	}
	if _, ok := manager.pendingDirs[mountPath]; ok {
		t.Fatal("expected stale pending state to be cleared on reuse")
	}
	if mountCalls != 1 {
		t.Fatalf("expected one mount call, got %d", mountCalls)
	}
	released, releaseErr := manager.ReleaseSource(context.Background(), sourceID)
	if releaseErr != nil {
		t.Fatalf("ReleaseSource returned error: %v", releaseErr)
	}
	if !released {
		t.Fatal("expected ReleaseSource to report an actual release")
	}
}

func TestManagerEnsureMountedPreservesPendingCleanupStateOnValidationFailure(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/pending-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	isoPath := filepath.Join(t.TempDir(), "pending.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				return os.MkdirAll(mountPath, 0o755)
			case "umount":
				umountCalls++
				if umountCalls > 1 {
					return errors.New("unexpected second umount")
				}
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.entries[sourceID] = &entry{ISOPath: "/bd_input/old.iso", MountPath: mountPath, LastTouchedAt: time.Now()}
	manager.mountOwners[mountPath] = sourceID
	manager.pendingDirs[mountPath] = struct{}{}

	if _, err := manager.EnsureMounted(context.Background(), sourceID, isoPath); err == nil {
		t.Fatal("expected EnsureMounted to fail validation")
	}
	residual := manager.CleanupResidualMountDirs(context.Background())
	if residual.Released != 0 || residual.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", residual)
	}
	if umountCalls != 1 {
		t.Fatalf("expected a single umount call across remount failure and cleanup, got %d", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected entry to be removed after cleanup")
	}
	if _, ok := manager.mountOwners[mountPath]; ok {
		t.Fatal("expected mount owner to be removed after cleanup")
	}
	if _, ok := manager.pendingDirs[mountPath]; ok {
		t.Fatal("expected pending state to be cleared after cleanup")
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected mount dir to be removed after cleanup, got stat error %v", statErr)
	}
}

func TestManagerCleanupResidualMountDirsRetriesInvalidContentCleanupWhenPendingAndRetryTracked(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/pending-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	if err := os.MkdirAll(filepath.Join(mountPath, "BDMV"), 0o755); err != nil {
		t.Fatal(err)
	}
	isoPath := filepath.Join(t.TempDir(), "pending.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV"), 0o755); err != nil {
					return err
				}
				return nil
			case "umount":
				umountCalls++
				if umountCalls == 1 {
					return errors.New("device busy")
				}
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.entries[sourceID] = &entry{ISOPath: "/bd_input/old.iso", MountPath: mountPath, LastTouchedAt: time.Now()}
	manager.mountOwners[mountPath] = sourceID
	manager.pendingDirs[mountPath] = struct{}{}

	if _, err := manager.EnsureMounted(context.Background(), sourceID, isoPath); err == nil {
		t.Fatal("expected invalid mounted content to fail")
	}
	if _, pending := manager.pendingDirs[mountPath]; !pending {
		t.Fatal("expected pending state to remain tracked after invalid cleanup failure")
	}
	if _, retrying := manager.retryMounts[mountPath]; !retrying {
		t.Fatal("expected retry state to be tracked after invalid cleanup failure")
	}

	residual := manager.CleanupResidualMountDirs(context.Background())
	if residual.Released != 1 || residual.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", residual)
	}
	if umountCalls != 2 {
		t.Fatalf("expected residual cleanup to retry umount, got %d calls", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected entry to be removed after successful retry")
	}
	if _, ok := manager.pendingDirs[mountPath]; ok {
		t.Fatal("expected pending state to be cleared after successful retry")
	}
	if _, ok := manager.retryMounts[mountPath]; ok {
		t.Fatal("expected retry state to be cleared after successful retry")
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected mount dir to be removed after retry, got stat error %v", statErr)
	}
}

func TestManagerEnsureMountedRetainsPendingStateOnMountFailure(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/pending-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	isoPath := filepath.Join(t.TempDir(), "pending.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "mount" {
				mountCalls++
				return errors.New("simulated mount failure")
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.pendingDirs[mountPath] = struct{}{}
	manager.mountOwners[mountPath] = sourceID

	if _, err := manager.EnsureMounted(context.Background(), sourceID, isoPath); err == nil {
		t.Fatal("expected EnsureMounted to fail when mount command fails")
	}
	if mountCalls != 1 {
		t.Fatalf("expected one mount call, got %d", mountCalls)
	}
	if _, pending := manager.pendingDirs[mountPath]; !pending {
		t.Fatal("expected pending cleanup state to survive mount failure")
	}
}

func TestManagerReleaseSourceAliasDoesNotExposeLiveMountToResidualCleanup(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/../disc"
	aliasID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
					return err
				}
				return nil
			case "umount":
				umountCalls++
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	mountPath, err := manager.EnsureMounted(context.Background(), sourceID, isoPath)
	if err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}
	if mountPath != filepath.Join(root, sanitizeID(sourceID)) {
		t.Fatalf("unexpected mount path %q", mountPath)
	}

	released, err := manager.ReleaseSource(context.Background(), aliasID)
	if err != nil {
		t.Fatalf("ReleaseSource alias returned error: %v", err)
	}
	if released {
		t.Fatal("expected alias release to be a no-op")
	}
	if _, ok := manager.mountOwners[mountPath]; !ok {
		t.Fatal("expected live mount ownership to remain after alias no-op release")
	}

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 0 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if umountCalls != 0 {
		t.Fatalf("expected residual cleanup to ignore live mount, got %d umount calls", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; !ok {
		t.Fatal("expected live mount entry to remain tracked")
	}
}

func TestManagerForceReleaseSourceAliasDoesNotExposeLiveMountToResidualCleanup(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/../disc"
	aliasID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
					return err
				}
				return nil
			case "umount":
				umountCalls++
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	mountPath, err := manager.EnsureMounted(context.Background(), sourceID, isoPath)
	if err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}

	released, err := manager.forceReleaseSource(context.Background(), aliasID)
	if err != nil {
		t.Fatalf("forceReleaseSource alias returned error: %v", err)
	}
	if released {
		t.Fatal("expected alias shutdown cleanup to be a no-op")
	}
	if _, ok := manager.mountOwners[mountPath]; !ok {
		t.Fatal("expected live mount ownership to remain after alias no-op shutdown cleanup")
	}

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 0 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if umountCalls != 0 {
		t.Fatalf("expected residual cleanup to ignore live mount, got %d umount calls", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; !ok {
		t.Fatal("expected live mount entry to remain tracked")
	}
}

func TestManagerReleaseSourceDoesNotClearReclaimedOwnerDuringCleanup(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/../disc"
	reclaimID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	releaseStarted := make(chan struct{})
	releaseGate := make(chan struct{})
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
					return err
				}
				return nil
			case "umount":
				select {
				case <-releaseStarted:
				default:
					close(releaseStarted)
				}
				<-releaseGate
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	mountPath, err := manager.EnsureMounted(context.Background(), sourceID, isoPath)
	if err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}

	releaseDone := make(chan struct {
		released bool
		err      error
	}, 1)
	go func() {
		released, err := manager.ReleaseSource(context.Background(), sourceID)
		releaseDone <- struct {
			released bool
			err      error
		}{released: released, err: err}
	}()

	select {
	case <-releaseStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for release to start")
	}

	reclaimDone := make(chan error, 1)
	go func() {
		_, err := manager.EnsureMounted(context.Background(), reclaimID, isoPath)
		reclaimDone <- err
	}()

	select {
	case err := <-reclaimDone:
		t.Fatalf("reclaiming EnsureMounted finished before release completed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseGate)

	result := <-releaseDone
	if result.err != nil {
		t.Fatalf("ReleaseSource returned error: %v", result.err)
	}
	if !result.released {
		t.Fatal("expected ReleaseSource to report an actual release")
	}

	select {
	case err := <-reclaimDone:
		if err != nil {
			t.Fatalf("reclaiming EnsureMounted returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reclaiming EnsureMounted")
	}

	if owner := manager.mountOwners[mountPath]; owner != reclaimID {
		t.Fatalf("expected reclaimed owner %q, got %q", reclaimID, owner)
	}
	if _, ok := manager.entries[reclaimID]; !ok {
		t.Fatal("expected reclaimed entry to remain tracked")
	}

	residual := manager.CleanupResidualMountDirs(context.Background())
	if residual.Released != 0 || residual.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", residual)
	}
}

func TestManagerCleanupExpiredIdleDoesNotClearReclaimedOwnerDuringCleanup(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/../disc"
	reclaimID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	releaseStarted := make(chan struct{})
	releaseGate := make(chan struct{})
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountPath := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
					return err
				}
				return nil
			case "umount":
				select {
				case <-releaseStarted:
				default:
					close(releaseStarted)
				}
				<-releaseGate
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	mountPath, err := manager.EnsureMounted(context.Background(), sourceID, isoPath)
	if err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}
	manager.entries[sourceID].LastTouchedAt = time.Now().Add(-2 * time.Hour)

	cleanupDone := make(chan ReleaseResult, 1)
	go func() {
		cleanupDone <- manager.CleanupExpiredIdle(context.Background(), time.Now())
	}()

	select {
	case <-releaseStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for idle cleanup to start")
	}

	reclaimDone := make(chan error, 1)
	go func() {
		_, err := manager.EnsureMounted(context.Background(), reclaimID, isoPath)
		reclaimDone <- err
	}()

	select {
	case err := <-reclaimDone:
		t.Fatalf("reclaiming EnsureMounted finished before cleanup completed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseGate)

	result := <-cleanupDone
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}

	select {
	case err := <-reclaimDone:
		if err != nil {
			t.Fatalf("reclaiming EnsureMounted returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reclaiming EnsureMounted")
	}

	if owner := manager.mountOwners[mountPath]; owner != reclaimID {
		t.Fatalf("expected reclaimed owner %q, got %q", reclaimID, owner)
	}
	if _, ok := manager.entries[reclaimID]; !ok {
		t.Fatal("expected reclaimed entry to remain tracked")
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected expired entry to be removed")
	}
}

func TestManagerEnsureMountedFailureDoesNotOrphanPriorSanitizedOwner(t *testing.T) {
	root := t.TempDir()
	primaryID := "library/../disc"
	secondaryID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mountCalls int
	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountCalls++
				mountPath := args[len(args)-1]
				if mountCalls == 1 {
					if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
						return err
					}
					return nil
				}
				return errors.New("simulated mount failure")
			case "umount":
				umountCalls++
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	mountPath, err := manager.EnsureMounted(context.Background(), primaryID, isoPath)
	if err != nil {
		t.Fatalf("primary EnsureMounted returned error: %v", err)
	}

	if _, err := manager.EnsureMounted(context.Background(), secondaryID, isoPath); err == nil {
		t.Fatal("expected secondary claimant to fail mounting the shared sanitized path")
	}
	if owner := manager.mountOwners[mountPath]; owner != primaryID {
		t.Fatalf("expected original owner %q to remain, got %q", primaryID, owner)
	}
	if _, ok := manager.entries[primaryID]; !ok {
		t.Fatal("expected original entry to remain tracked")
	}

	residual := manager.CleanupResidualMountDirs(context.Background())
	if residual.Released != 0 || residual.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", residual)
	}
	if umountCalls != 0 {
		t.Fatalf("expected live mount not to be torn down, got %d umount calls", umountCalls)
	}
	if mountCalls != 1 {
		t.Fatalf("expected the collision to be rejected before a second mount, got %d mount attempts", mountCalls)
	}
}

func TestManagerEnsureMountedRejectsClaimWhenSanitizedPathAlreadyHasTrackedPendingOwner(t *testing.T) {
	root := t.TempDir()
	primaryID := "library/../disc"
	secondaryID := "disc"
	isoPath := filepath.Join(t.TempDir(), "disc.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				mountCalls++
				mountPath := args[len(args)-1]
				if mountCalls == 1 {
					if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
						return err
					}
					return nil
				}
				return errors.New("simulated mount failure")
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)
	mountPath, err := manager.EnsureMounted(context.Background(), primaryID, isoPath)
	if err != nil {
		t.Fatalf("primary EnsureMounted returned error: %v", err)
	}
	manager.pendingDirs[mountPath] = struct{}{}

	if _, err := manager.EnsureMounted(context.Background(), secondaryID, isoPath); err == nil {
		t.Fatal("expected secondary claimant to be rejected while pending cleanup still exists")
	}
	if owner := manager.mountOwners[mountPath]; owner != primaryID {
		t.Fatalf("expected original owner %q to remain, got %q", primaryID, owner)
	}
	if _, ok := manager.entries[primaryID]; !ok {
		t.Fatal("expected original entry to remain tracked")
	}
	residual := manager.CleanupResidualMountDirs(context.Background())
	if residual.Released != 0 || residual.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", residual)
	}
}

func TestManagerCleanupResidualMountDirsSkipsTrackedMount(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/tracked-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" {
				umountCalls++
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.entries[sourceID] = &entry{ISOPath: "/bd_input/tracked.iso", MountPath: mountPath, LastTouchedAt: time.Now()}
	manager.mountOwners[mountPath] = sourceID

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 0 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if umountCalls != 0 {
		t.Fatalf("expected tracked mount to be left alone, got %d umount calls", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; !ok {
		t.Fatal("expected tracked mount entry to remain")
	}
}

func TestManagerCleanupResidualMountDirsRetriesPendingUnmountedDir(t *testing.T) {
	root := t.TempDir()
	mountPath := filepath.Join(root, "pending-disc")
	if err := os.MkdirAll(mountPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" {
				umountCalls++
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.pendingDirs[mountPath] = struct{}{}

	result := manager.CleanupResidualMountDirs(context.Background())
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected residual cleanup summary %+v", result)
	}
	if umountCalls != 0 {
		t.Fatalf("expected retry-pending dir not to be unmounted again, got %d calls", umountCalls)
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected pending dir to be removed, got stat error %v", statErr)
	}
}

func TestManagerCleanupAllCountsOnlyActualReleasesWhenSourceDisappears(t *testing.T) {
	root := t.TempDir()
	firstID := "first-disc"
	secondID := "second-disc"
	firstPath := filepath.Join(root, firstID)
	secondPath := filepath.Join(root, secondID)
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Join(path, "BDMV", "PLAYLIST"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	releaseFirst := make(chan struct{})
	firstUnmounted := make(chan string, 1)
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name != "umount" {
				return nil
			}
			select {
			case firstUnmounted <- args[0]:
				<-releaseFirst
			default:
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.entries[firstID] = &entry{ISOPath: "/bd_input/first.iso", MountPath: firstPath, LastTouchedAt: time.Now()}
	manager.entries[secondID] = &entry{ISOPath: "/bd_input/second.iso", MountPath: secondPath, LastTouchedAt: time.Now()}
	manager.mountOwners[firstPath] = firstID
	manager.mountOwners[secondPath] = secondID

	done := make(chan ReleaseResult, 1)
	go func() {
		done <- manager.CleanupAll(context.Background())
	}()

	blockedPath := <-firstUnmounted
	missingID := secondID
	missingPath := secondPath
	if blockedPath == secondPath {
		missingID = firstID
		missingPath = firstPath
	}
	manager.mu.Lock()
	delete(manager.entries, missingID)
	delete(manager.mountOwners, missingPath)
	manager.mu.Unlock()
	if err := os.RemoveAll(missingPath); err != nil {
		t.Fatal(err)
	}
	close(releaseFirst)

	result := <-done
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}
}

func TestManagerCleanupAllRetriesFreshInvalidContentCleanupFailure(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/fresh-invalid-disc"
	isoPath := filepath.Join(t.TempDir(), "fresh-invalid.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	mountPath := filepath.Join(root, sanitizeID(sourceID))
	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			switch name {
			case "mount":
				if err := os.MkdirAll(filepath.Join(mountPath, "BDMV"), 0o755); err != nil {
					return err
				}
				return nil
			case "umount":
				umountCalls++
				if umountCalls == 1 {
					return errors.New("device busy")
				}
				return nil
			default:
				return nil
			}
		},
	}
	manager := NewManager(root, time.Hour, runner)

	if _, err := manager.EnsureMounted(context.Background(), sourceID, isoPath); err == nil {
		t.Fatal("expected invalid mounted content to fail")
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected fresh invalid mount to remain untracked")
	}
	if _, retrying := manager.retryMounts[mountPath]; !retrying {
		t.Fatal("expected retry state to be tracked after failed invalid-content cleanup")
	}

	result := manager.CleanupAll(context.Background())
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup summary %+v", result)
	}
	if umountCalls != 2 {
		t.Fatalf("expected CleanupAll to retry unmount, got %d calls", umountCalls)
	}
	if _, ok := manager.retryMounts[mountPath]; ok {
		t.Fatal("expected retry state to be cleared after CleanupAll")
	}
	if _, statErr := os.Stat(mountPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected mount dir to be removed after CleanupAll, got stat error %v", statErr)
	}
}

func TestManagerReleaseSourceRetriesAfterRemovalFailureWithoutReUnmounting(t *testing.T) {
	root := t.TempDir()
	sourceID := "library/retry-disc"
	mountPath := filepath.Join(root, sanitizeID(sourceID))
	if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(root, 0o755) }()

	var umountCalls int
	runner := &fakeCommandRunner{
		runFn: func(_ context.Context, name string, args ...string) error {
			if name == "umount" {
				umountCalls++
			}
			return nil
		},
	}
	manager := NewManager(root, time.Hour, runner)
	manager.mountOwners[mountPath] = sourceID
	manager.entries[sourceID] = &entry{ISOPath: "/bd_input/retry.iso", MountPath: mountPath, LastTouchedAt: time.Now()}

	released, err := manager.ReleaseSource(context.Background(), sourceID)
	if err == nil {
		t.Fatal("expected first release attempt to fail because removal is denied")
	}
	if released {
		t.Fatal("expected first release attempt not to count as released")
	}
	if umountCalls != 1 {
		t.Fatalf("expected one umount call on first attempt, got %d", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; !ok {
		t.Fatal("expected entry to remain after failed cleanup")
	}

	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}

	released, err = manager.ReleaseSource(context.Background(), sourceID)
	if err != nil {
		t.Fatalf("expected retry to succeed without a second umount, got %v", err)
	}
	if !released {
		t.Fatal("expected retry to count as released")
	}
	if umountCalls != 1 {
		t.Fatalf("expected retry to skip umount, got %d total calls", umountCalls)
	}
	if _, ok := manager.entries[sourceID]; ok {
		t.Fatal("expected entry to be removed after successful retry")
	}
}
