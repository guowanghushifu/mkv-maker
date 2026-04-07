package remux

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMKVMergeArgsIncludesTrackMetadata(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/Nightcrawler - 2160p.BluRay.DV.HDR.HEVC.TrueHD.7.1.Atmos.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Playlist:   "00800.MPLS",
		EnableDV:   true,
		Video:      VideoTrack{Name: "Main Video"},
		Audio:      []AudioTrack{{ID: "A1", SourceIndex: 0, Name: "English Atmos", Language: "eng", Default: true, Selected: true}},
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

func TestBuildMKVMergeArgsUsesPlaylistInputEvenWhenSegmentPathsArePresent(t *testing.T) {
	draft := Draft{
		OutputPath:   "/remux/out.mkv",
		SourcePath:   "/bd_input/Nightcrawler",
		Playlist:     "00800.MPLS",
		SegmentPaths: []string{"/bd_input/Nightcrawler/BDMV/STREAM/00005.m2ts", "/bd_input/Nightcrawler/BDMV/STREAM/00006.m2ts"},
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

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "/BDMV/STREAM/00005.m2ts") || strings.Contains(joined, "/BDMV/STREAM/00006.m2ts") {
		t.Fatalf("expected segment paths to be excluded from mkvmerge input, got %q", joined)
	}
}

func TestBuildMKVMergeArgsUsesSourceIndexSelectors(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "A1", SourceIndex: 7, Name: "English", Language: "eng", Selected: true},
			{ID: "A2", SourceIndex: 12, Name: "Japanese", Language: "jpn", Selected: true},
		},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--language 7:eng") || !strings.Contains(joined, "--track-name 7:English") {
		t.Fatalf("expected first audio selector to use sourceIndex, got %q", joined)
	}
	if !strings.Contains(joined, "--language 12:jpn") || !strings.Contains(joined, "--track-name 12:Japanese") {
		t.Fatalf("expected second audio selector to use sourceIndex, got %q", joined)
	}
}

func TestBuildMKVMergeArgsUsesIntermediateMKVInputPath(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/tmp/intermediate.mkv",
		Playlist:   "00800.MPLS",
	}

	args := BuildMKVMergeArgs(draft)
	if len(args) == 0 {
		t.Fatalf("expected args to be non-empty")
	}

	gotInput := args[len(args)-1]
	if gotInput != "/tmp/intermediate.mkv" {
		t.Fatalf("expected intermediate mkv input path, got %q", gotInput)
	}
}

func TestBuildMKVMergeArgsAudioTracksIncludesOnlySelectedSourceIndexes(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Audio: []AudioTrack{
			{ID: "A1", SourceIndex: 7, Name: "English", Language: "eng", Selected: true},
			{ID: "A2", SourceIndex: 8, Name: "French", Language: "fra", Selected: false},
			{ID: "A3", SourceIndex: 12, Name: "Japanese", Language: "jpn", Selected: true},
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
			{ID: "A1", SourceIndex: 9, Name: "Commentary", Language: "eng", Selected: false},
			{ID: "A2", SourceIndex: 7, Name: "English", Language: "eng", Selected: true},
			{ID: "A3", SourceIndex: 12, Name: "Japanese", Language: "jpn", Selected: true},
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
			{ID: "A1", SourceIndex: 7, Name: "English", Language: "eng", Selected: true},
		},
		Subtitles: []SubtitleTrack{
			{ID: "S1", SourceIndex: 12, Name: "English PGS", Language: "eng", Selected: true, Forced: true},
			{ID: "S2", SourceIndex: 13, Name: "Chinese PGS", Language: "chi", Selected: true, Default: true},
			{ID: "S3", SourceIndex: 14, Name: "French PGS", Language: "fra", Selected: false},
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
			{ID: "A1", SourceIndex: 7, Name: "English", Language: "eng", Selected: true, Default: true},
			{ID: "A2", SourceIndex: 8, Name: "Commentary", Language: "eng", Selected: true, Default: false},
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

func TestBuildMKVMergeArgsWithResolvedSelectorsUsesResolvedTrackIDs(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/tmp/intermediate.mkv",
		Video:      VideoTrack{Name: "Main Video"},
		Audio: []AudioTrack{
			{ID: "audio-0", SourceIndex: 0, Name: "English", Language: "eng", Selected: true, Default: true},
			{ID: "audio-1", SourceIndex: 1, Name: "Commentary", Language: "eng", Selected: true, Default: false},
		},
		Subtitles: []SubtitleTrack{
			{ID: "subtitle-0", SourceIndex: 0, Name: "English PGS", Language: "eng", Selected: true, Forced: true},
		},
	}

	args, err := BuildMKVMergeArgsWithResolvedSelectors(
		draft,
		[]ResolvedTrackSelector{{SourceIndex: 0, TrackID: "3"}, {SourceIndex: 1, TrackID: "8"}},
		[]ResolvedTrackSelector{{SourceIndex: 0, TrackID: "14"}},
	)
	if err != nil {
		t.Fatalf("BuildMKVMergeArgsWithResolvedSelectors returned error: %v", err)
	}

	if value, ok := optionValue(args, "--audio-tracks"); !ok || value != "3,8" {
		t.Fatalf("expected resolved audio selectors 3,8, got %q (present=%t)", value, ok)
	}
	if value, ok := optionValue(args, "--subtitle-tracks"); !ok || value != "14" {
		t.Fatalf("expected resolved subtitle selector 14, got %q (present=%t)", value, ok)
	}
	if value, ok := optionValue(args, "--track-order"); !ok || value != "0:0,0:3,0:8,0:14" {
		t.Fatalf("expected resolved track order, got %q (present=%t)", value, ok)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--track-name 3:English") || !strings.Contains(joined, "--track-name 8:Commentary") {
		t.Fatalf("expected resolved audio track-name selectors, got %q", joined)
	}
	if !strings.Contains(joined, "--language 14:eng") || !strings.Contains(joined, "--forced-display-flag 14:yes") {
		t.Fatalf("expected resolved subtitle selectors, got %q", joined)
	}
}

func TestBuildMKVMergeArgsWithResolvedSelectorsFailsWhenMappingMissing(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/out.mkv",
		SourcePath: "/tmp/intermediate.mkv",
		Audio:      []AudioTrack{{ID: "audio-0", SourceIndex: 0, Name: "English", Language: "eng", Selected: true}},
	}

	_, err := BuildMKVMergeArgsWithResolvedSelectors(draft, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "audio sourceIndex 0") {
		t.Fatalf("expected missing audio mapping error, got %v", err)
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
