package isomount

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
