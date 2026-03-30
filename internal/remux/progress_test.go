package remux

import (
	"io"
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

func TestFormatCommandPreviewAlwaysUsesLiteralMkvmergeBinary(t *testing.T) {
	got := FormatCommandPreview("/opt/homebrew/bin/mkvmerge", []string{"--output", "/remux/out.mkv"})
	if !strings.HasPrefix(got, "mkvmerge\n") {
		t.Fatalf("expected preview to start with literal mkvmerge, got %q", got)
	}
}

func TestExtractProgressPercentsFromChunkParsesCarriageReturnAndSplitTokens(t *testing.T) {
	percents, remainder := extractProgressPercentsFromChunk("", "Progress: 4")
	if len(percents) != 0 {
		t.Fatalf("expected no completed percent from partial chunk, got %v", percents)
	}
	if remainder != "Progress: 4" {
		t.Fatalf("expected partial remainder to be kept, got %q", remainder)
	}

	percents, remainder = extractProgressPercentsFromChunk(remainder, "2%\r#GUI#progress 77%\r")
	if len(percents) != 2 || percents[0] != 42 || percents[1] != 77 {
		t.Fatalf("expected parsed percents [42 77], got %v", percents)
	}
	if remainder != "" {
		t.Fatalf("expected empty remainder, got %q", remainder)
	}
}

func TestStreamOutputEmitsCarriageReturnChunkWithoutNewline(t *testing.T) {
	reader, writer := io.Pipe()
	chunks := make(chan string, 4)
	done := make(chan struct{})

	go func() {
		defer close(done)
		streamOutput(reader, func(chunk string) {
			chunks <- chunk
		})
		close(chunks)
	}()

	if _, err := writer.Write([]byte("Progress: 42%\r")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	_ = writer.Close()

	var combined string
	for chunk := range chunks {
		combined += chunk
	}
	<-done

	if !strings.Contains(combined, "Progress: 42%\r") {
		t.Fatalf("expected carriage-return progress chunk, got %q", combined)
	}
}
