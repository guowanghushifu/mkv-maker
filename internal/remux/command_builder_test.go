package remux

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMKVMergeArgsIncludesTrackMetadata(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv",
		SourcePath: "/bd_input/Nightcrawler",
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
		SourcePath: "/bd_input/Nightcrawler",
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

func TestBuildMKVMergeArgsAudioTracksIncludesOnlySelectedSelectors(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "7", Name: "English", Language: "eng", Selected: true},
			{ID: "8", Name: "French", Language: "fra", Selected: false},
			{ID: "a1", Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	value, ok := optionValue(args, "--audio-tracks")
	if !ok {
		t.Fatalf("expected --audio-tracks option in args: %q", strings.Join(args, " "))
	}
	if value != "7,3" {
		t.Fatalf("expected selected selectors \"7,3\", got %q", value)
	}
}

func TestBuildMKVMergeArgsTrackOrderIsVideoThenSelectedAudiosInInputOrder(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "9", Name: "Commentary", Language: "eng", Selected: false},
			{ID: "7", Name: "English", Language: "eng", Selected: true},
			{ID: "a1", Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	value, ok := optionValue(args, "--track-order")
	if !ok {
		t.Fatalf("expected --track-order option in args: %q", strings.Join(args, " "))
	}
	if value != "0:0,0:7,0:3" {
		t.Fatalf("expected track order \"0:0,0:7,0:3\", got %q", value)
	}
}

func optionValue(args []string, option string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == option {
			return args[i+1], true
		}
	}
	return "", false
}
