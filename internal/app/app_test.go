package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/isomount"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type closeAwareRemuxRunner struct {
	started chan struct{}
	done    chan struct{}
}

func (r *closeAwareRemuxRunner) Run(ctx context.Context, _ remux.Draft, _ func(string)) (string, error) {
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}
	<-ctx.Done()
	if r.done != nil {
		select {
		case <-r.done:
		default:
			close(r.done)
		}
	}
	return "", ctx.Err()
}

type orderAwareISORunner struct {
	remuxDone  <-chan struct{}
	tooEarlyCh chan struct{}
}

func (r *orderAwareISORunner) Run(_ context.Context, name string, args ...string) error {
	switch name {
	case "mount":
		mountPath := args[len(args)-1]
		if err := os.MkdirAll(filepath.Join(mountPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(mountPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
			return err
		}
	case "umount":
		select {
		case <-r.remuxDone:
		default:
			if r.tooEarlyCh != nil {
				select {
				case <-r.tooEarlyCh:
				default:
					close(r.tooEarlyCh)
				}
			}
		}
	}
	return nil
}

func TestAppCloseStopsRemuxBeforeISOCleanup(t *testing.T) {
	inputRoot := t.TempDir()
	isoRoot := filepath.Join(inputRoot, "iso_auto_mount")
	remuxRunner := &closeAwareRemuxRunner{
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
	tooEarlyCh := make(chan struct{})
	remuxManager := remux.NewManager(remuxRunner)
	isoManager := isomount.NewManager(isoRoot, time.Hour, &orderAwareISORunner{
		remuxDone:  remuxRunner.done,
		tooEarlyCh: tooEarlyCh,
	})

	isoPath := filepath.Join(inputRoot, "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := isoManager.EnsureMounted(context.Background(), "movies-nightcrawler-iso", isoPath); err != nil {
		t.Fatalf("EnsureMounted returned error: %v", err)
	}
	outputPath := filepath.Join(t.TempDir(), "Nightcrawler.mkv")

	_, err := remuxManager.Start(remux.StartRequest{
		SourceName:   "Nightcrawler",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   outputPath,
		PlaylistName: "00800.MPLS",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler","path":"` + isoPath + `","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"` + outputPath + `"
		}`,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	select {
	case <-remuxRunner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for remux task to start")
	}

	app := &App{
		remuxManager: remuxManager,
		isoManager:   isoManager,
	}
	if err := app.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	select {
	case <-remuxRunner.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for remux shutdown")
	}
	select {
	case <-tooEarlyCh:
		t.Fatal("ISO cleanup ran before remux shutdown")
	default:
	}
}
