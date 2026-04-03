package remux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fileWritingRunner struct {
	run func(ctx context.Context, draft Draft, onOutput func(string)) (string, error)
}

func (r fileWritingRunner) Run(ctx context.Context, draft Draft, onOutput func(string)) (string, error) {
	return r.run(ctx, draft, onOutput)
}

func TestBuildExecutionDraftUsesExistingPlaylistPathCaseInsensitive(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Disc", "BDMV")
	playlistPath := filepath.Join(sourcePath, "PLAYLIST", "00801.mpls")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validPayloadJSON("Disc", sourcePath, "00801.MPLS", "/remux/Disc.mkv"),
	}

	draft, err := runner.BuildExecutionDraft(req)
	if err != nil {
		t.Fatalf("BuildExecutionDraft returned error: %v", err)
	}
	if draft.SourcePath != playlistPath {
		t.Fatalf("expected SourcePath %q, got %q", playlistPath, draft.SourcePath)
	}

	preview, err := runner.CommandPreview(req)
	if err != nil {
		t.Fatalf("CommandPreview returned error: %v", err)
	}
	if !strings.Contains(preview, playlistPath) {
		t.Fatalf("expected preview to contain %q, got %q", playlistPath, preview)
	}
}

func TestJobRunnerCommandPreviewUsesTemporaryOutputPath(t *testing.T) {
	runner := NewJobRunner(&stubRunner{})
	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   "/remux/Disc.mkv",
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validPayloadJSON("Disc", "/bd_input/Disc", "00801.MPLS", "/remux/Disc.mkv"),
	}

	preview, err := runner.CommandPreview(req)
	if err != nil {
		t.Fatalf("CommandPreview returned error: %v", err)
	}
	if !strings.Contains(preview, "/remux/Disc.mkv.tmp") {
		t.Fatalf("expected preview to use temporary output path, got %q", preview)
	}
	if strings.Contains(preview, "\n  --output /remux/Disc.mkv\n") {
		t.Fatalf("expected preview not to use final output path directly, got %q", preview)
	}
}

func TestJobRunnerExecuteRenamesTemporaryOutputAfterSuccessfulRun(t *testing.T) {
	outputRoot := t.TempDir()
	finalPath := filepath.Join(outputRoot, "Disc.mkv")
	tempPath := finalPath + ".tmp"

	runner := NewJobRunner(fileWritingRunner{
		run: func(_ context.Context, draft Draft, onOutput func(string)) (string, error) {
			if draft.OutputPath != tempPath {
				t.Fatalf("expected runner output path %q, got %q", tempPath, draft.OutputPath)
			}
			if err := os.WriteFile(draft.OutputPath, []byte("muxed"), 0o644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if onOutput != nil {
				onOutput("Progress: 100%")
			}
			return "Progress: 100%", nil
		},
	})

	req := StartRequest{
		SourceName:   "Disc",
		OutputName:   "Disc.mkv",
		OutputPath:   finalPath,
		PlaylistName: "00801.MPLS",
		PayloadJSON:  validPayloadJSON("Disc", "/bd_input/Disc", "00801.MPLS", finalPath),
	}

	_, _, err := runner.Execute(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("expected final output to exist: %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary output to be removed, got %v", err)
	}
}
