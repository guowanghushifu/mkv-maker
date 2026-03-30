package remux

import (
	"strings"
	"testing"
)

func TestFormatCommandPreviewRendersReadableMultiLineCommand(t *testing.T) {
	got := FormatCommandPreview("mkvmerge", []string{
		"--output", "/remux/Nightcrawler.mkv",
		"--audio-tracks", "2,3",
		"/bd_input/Nightcrawler/BDMV/PLAYLIST/00003.MPLS",
	})

	if !strings.HasPrefix(got, "mkvmerge\n") {
		t.Fatalf("expected preview to start with mkvmerge, got %q", got)
	}
	if !strings.Contains(got, "\n  --output\n  /remux/Nightcrawler.mkv\n") {
		t.Fatalf("expected output arg in multiline preview, got %q", got)
	}
	if !strings.Contains(got, "/bd_input/Nightcrawler/BDMV/PLAYLIST/00003.MPLS") {
		t.Fatalf("expected input path in preview, got %q", got)
	}
}

func TestExtractProgressPercentParsesExplicitMkvmmergePercentages(t *testing.T) {
	tests := []struct {
		line string
		want int
		ok   bool
	}{
		{line: "Progress: 42%", want: 42, ok: true},
		{line: "#GUI#progress 77%", want: 77, ok: true},
		{line: "muxing took 3 seconds", want: 0, ok: false},
	}

	for _, tc := range tests {
		got, ok := ExtractProgressPercent(tc.line)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ExtractProgressPercent(%q) = (%d, %t), want (%d, %t)", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}
