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
			{ID: "audio-12", Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--language 7:eng") || !strings.Contains(joined, "--track-name 7:English") {
		t.Fatalf("expected numeric audio ID selector for first track, got %q", joined)
	}
	if !strings.Contains(joined, "--language 12:jpn") || !strings.Contains(joined, "--track-name 12:Japanese") {
		t.Fatalf("expected selector parsed from second track id, got %q", joined)
	}
}

func TestBuildMKVMergeArgsAudioTracksIncludesOnlySelectedSelectors(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "7", Name: "English", Language: "eng", Selected: true},
			{ID: "8", Name: "French", Language: "fra", Selected: false},
			{ID: "audio-12", Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	value, ok := optionValue(args, "--audio-tracks")
	if !ok {
		t.Fatalf("expected --audio-tracks option in args: %q", strings.Join(args, " "))
	}
	if value != "7,12" {
		t.Fatalf("expected selected selectors \"7,12\", got %q", value)
	}
}

func TestBuildMKVMergeArgsTrackOrderIsVideoThenSelectedAudiosInInputOrder(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "9", Name: "Commentary", Language: "eng", Selected: false},
			{ID: "7", Name: "English", Language: "eng", Selected: true},
			{ID: "audio-12", Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	value, ok := optionValue(args, "--track-order")
	if !ok {
		t.Fatalf("expected --track-order option in args: %q", strings.Join(args, " "))
	}
	if value != "0:0,0:7,0:12" {
		t.Fatalf("expected track order \"0:0,0:7,0:12\", got %q", value)
	}
}

func TestBuildMKVMergeArgsIncludesSelectedSubtitlesAndTrackOrder(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "7", Name: "English", Language: "eng", Selected: true},
		},
		Subtitles: []SubtitleTrack{
			{ID: "12", Name: "English PGS", Language: "eng", Selected: true, Forced: true},
			{ID: "13", Name: "Chinese PGS", Language: "chi", Selected: true, Default: true},
			{ID: "14", Name: "French PGS", Language: "fra", Selected: false},
		},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")

	subtitleTracks, ok := optionValue(args, "--subtitle-tracks")
	if !ok {
		t.Fatalf("expected --subtitle-tracks option in args: %q", joined)
	}
	if subtitleTracks != "12,13" {
		t.Fatalf("expected selected subtitle selectors \"12,13\", got %q", subtitleTracks)
	}
	if !strings.Contains(joined, "--language 12:eng") || !strings.Contains(joined, "--track-name 12:English PGS") {
		t.Fatalf("expected subtitle language/name for selector 12, got %q", joined)
	}
	if !strings.Contains(joined, "--forced-display-flag 12:yes") {
		t.Fatalf("expected forced display flag for selector 12, got %q", joined)
	}
	if !strings.Contains(joined, "--default-track-flag 12:no") {
		t.Fatalf("expected non-default subtitle selector 12 to be set to no, got %q", joined)
	}
	if !strings.Contains(joined, "--default-track-flag 13:yes") {
		t.Fatalf("expected default track flag for subtitle selector 13, got %q", joined)
	}

	trackOrder, ok := optionValue(args, "--track-order")
	if !ok {
		t.Fatalf("expected --track-order option in args: %q", joined)
	}
	if trackOrder != "0:0,0:7,0:12,0:13" {
		t.Fatalf("expected track order \"0:0,0:7,0:12,0:13\", got %q", trackOrder)
	}
}

func TestBuildMKVMergeArgsMarksNonDefaultAudioTracksAsNo(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "7", Name: "English", Language: "eng", Selected: true, Default: true},
			{ID: "8", Name: "Commentary", Language: "eng", Selected: true, Default: false},
		},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--default-track-flag 7:yes") {
		t.Fatalf("expected selector 7 to be marked default, got %q", joined)
	}
	if !strings.Contains(joined, "--default-track-flag 8:no") {
		t.Fatalf("expected selector 8 to be marked non-default, got %q", joined)
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
