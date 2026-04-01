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
	manager.entries["busy-disc"] = &entry{ISOPath: "/bd_input/busy.iso", MountPath: filepath.Join(root, "busy-disc"), LastTouchedAt: now.Add(-2 * time.Hour), InUse: true}
	manager.entries["broken-disc"] = &entry{ISOPath: "/bd_input/broken.iso", MountPath: filepath.Join(root, "broken-disc"), LastTouchedAt: now.Add(-2 * time.Hour)}

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
}

func TestManagerCleanupExpiredIdleReleasesOnlyStaleEntries(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	manager := NewManager(root, time.Hour, &fakeCommandRunner{})
	manager.now = func() time.Time { return now }
	manager.entries["stale-disc"] = &entry{ISOPath: "/bd_input/stale.iso", MountPath: filepath.Join(root, "stale-disc"), LastTouchedAt: now.Add(-2 * time.Hour)}
	manager.entries["fresh-disc"] = &entry{ISOPath: "/bd_input/fresh.iso", MountPath: filepath.Join(root, "fresh-disc"), LastTouchedAt: now.Add(-15 * time.Minute)}

	result := manager.CleanupExpiredIdle(context.Background(), now)
	if result.Released != 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup summary %+v", result)
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
