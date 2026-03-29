package remux

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMKVMergeArgsIncludesTrackMetadata(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv",
		SourcePath: "/bd_input/Nightcrawler.iso",
		Playlist:   "00800.MPLS",
		EnableDV:   true,
		Video:      VideoTrack{Name: "Main Video"},
		Audio:      []AudioTrack{{ID: "a1", Name: "English Atmos", Language: "eng", Default: true, Selected: true}},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--track-name") || !strings.Contains(joined, "English Atmos") {
		t.Fatalf("expected mkvmerge args to include track naming, got %q", joined)
	}
	if !strings.Contains(joined, "--output") {
		t.Fatalf("expected mkvmerge args to include output path, got %q", joined)
	}
}

func TestBuildMKVMergeArgsUsesPlaylistFileForBluRayFolderSource(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Playlist:   "00800.MPLS",
	}

	args := BuildMKVMergeArgs(draft)
	if len(args) == 0 {
		t.Fatalf("expected args to be non-empty")
	}

	wantInput := filepath.Join("/bd_input/Nightcrawler", "BDMV", "PLAYLIST", "00800.MPLS")
	gotInput := args[len(args)-1]
	if gotInput != wantInput {
		t.Fatalf("expected playlist input %q, got %q", wantInput, gotInput)
	}
}

func TestBuildMKVMergeArgsPrefersNumericAudioIDAndFallsBackToIndex(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler.iso",
		Audio: []AudioTrack{
			{ID: "7", Name: "English", Language: "eng", Selected: true},
			{ID: "a1", Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--language 7:eng") || !strings.Contains(joined, "--track-name 7:English") {
		t.Fatalf("expected numeric audio ID selector for first track, got %q", joined)
	}
	if !strings.Contains(joined, "--language 2:jpn") || !strings.Contains(joined, "--track-name 2:Japanese") {
		t.Fatalf("expected index fallback selector for second track, got %q", joined)
	}
}
