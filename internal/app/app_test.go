package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/config"
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
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[],"makemkv":{"playlistName":"00800.MPLS","titleId":0,"audio":[],"subtitles":[]}},
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

func TestNewDisablesISOSupportOutsideLinux(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()
	dataRoot := t.TempDir()

	isoPath := filepath.Join(inputRoot, "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() {
		runtimeGOOS = originalGOOS
	})

	app, err := New(config.Config{
		AppPassword:   "secret",
		InputDir:      inputRoot,
		OutputDir:     outputRoot,
		DataDir:       dataRoot,
		EnableISOScan: true,
		SessionMaxAge: 3600,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = app.Close()
	})

	if app.isoManager != nil {
		t.Fatal("expected iso manager to be disabled outside linux")
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"secret"}`))
	loginRec := httptest.NewRecorder()
	app.Handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("expected login to succeed, got %d", loginRec.Code)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie from login")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	for _, cookie := range cookies {
		listReq.AddCookie(cookie)
	}
	listRec := httptest.NewRecorder()
	app.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected source list to succeed, got %d", listRec.Code)
	}

	var sources []struct {
		Type string `json:"type"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &sources); err != nil {
		t.Fatalf("failed to decode source list: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected no sources when ISO scan is disabled outside linux, got %+v", sources)
	}
}
