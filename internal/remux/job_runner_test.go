package remux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
