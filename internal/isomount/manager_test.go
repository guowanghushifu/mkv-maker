package isomount

import (
	"context"
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
