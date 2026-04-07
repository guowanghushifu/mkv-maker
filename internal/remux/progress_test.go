package remux

import (
	"io"
	"strings"
	"testing"
)

func TestFormatCommandPreviewRendersReadableMultiLineCommand(t *testing.T) {
	got := FormatCommandPreview("mkvmerge", []string{
		"--output", "/remux/Nightcrawler.mkv",
		"--track-name", "1:English Atmos",
		"--audio-tracks", "2,3",
		"/bd_input/Nightcrawler/BDMV/PLAYLIST/00003.MPLS",
	})

	if !strings.HasPrefix(got, "mkvmerge\n") {
		t.Fatalf("expected preview to start with mkvmerge, got %q", got)
	}
	if !strings.Contains(got, "\n  --output /remux/Nightcrawler.mkv\n") {
		t.Fatalf("expected output option and value on one line, got %q", got)
	}
	if !strings.Contains(got, "\n  --track-name 1:English Atmos\n") {
		t.Fatalf("expected track-name option and value on one line, got %q", got)
	}
	if !strings.Contains(got, "\n  --audio-tracks 2,3\n") {
		t.Fatalf("expected audio-tracks option and value on one line, got %q", got)
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

func TestFormatCommandPreviewKeepsProvidedBinaryName(t *testing.T) {
	got := FormatCommandPreview("/opt/homebrew/bin/mkvmerge", []string{"--output", "/remux/out.mkv"})
	if !strings.HasPrefix(got, "/opt/homebrew/bin/mkvmerge\n") {
		t.Fatalf("expected preview to keep provided binary name, got %q", got)
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
	if len(percents) != 2 || percents[0] != 76 || percents[1] != 90 {
		t.Fatalf("expected parsed percents [76 90], got %v", percents)
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

func TestExtractProgressPercentsFromChunkIgnoresMakeMKVProgressBeforeSavingPhase(t *testing.T) {
	percents, remainder := extractProgressPercentsFromChunk("", "Total progress - 12%\n")
	if len(percents) != 0 {
		t.Fatalf("expected no progress before MakeMKV saving phase, got %v", percents)
	}
	if remainder != "" {
		t.Fatalf("expected empty remainder, got %q", remainder)
	}
}

func TestExtractProgressPercentsFromChunkMapsMakeMKVSavingPhaseToFirstSixtyPercent(t *testing.T) {
	percents, remainder := extractProgressPercentsFromChunk("", "Current action: Saving to MKV file\nTotal progress - 50%\n")
	if len(percents) != 1 || percents[0] != 30 {
		t.Fatalf("expected mapped MakeMKV progress [30], got %v", percents)
	}
	if remainder != "" {
		t.Fatalf("expected empty remainder, got %q", remainder)
	}
}

func TestExtractProgressPercentsFromChunkParsesMakeMKVCompositeSavingLine(t *testing.T) {
	percents, remainder := extractProgressPercentsFromChunk(
		"",
		"Current action: Saving to MKV file\nCurrent progress - 97%  , Total progress - 96%\n",
	)
	if len(percents) != 1 || percents[0] != 57 {
		t.Fatalf("expected mapped MakeMKV composite progress [57], got %v", percents)
	}
	if remainder != "" {
		t.Fatalf("expected empty remainder, got %q", remainder)
	}
}

func TestExtractProgressPercentsFromChunkIgnoresCompositeMakeMKVProgressBeforeSavingAction(t *testing.T) {
	percents, remainder := extractProgressPercentsFromChunk(
		"",
		"Current progress - 97%  , Total progress - 96%\n",
	)
	if len(percents) != 0 {
		t.Fatalf("expected no MakeMKV progress before saving action, got %v", percents)
	}
	if remainder != "" {
		t.Fatalf("expected empty remainder, got %q", remainder)
	}
}

func TestFormatCommandPreviewUsesProvidedBinaryName(t *testing.T) {
	got := FormatCommandPreview("makemkvcon", []string{"--messages=-null", "mkv", "file:/bd_input/Disc", "4", "/remux_tmp"})
	if !strings.HasPrefix(got, "makemkvcon\n") {
		t.Fatalf("expected preview to start with makemkvcon, got %q", got)
	}
}

func TestExtractProgressPercentsFromChunkMapsMkvmergeToLastFortyPercent(t *testing.T) {
	percents, remainder := extractProgressPercentsFromChunk("", "Progress: 50%\n")
	if len(percents) != 1 || percents[0] != 80 {
		t.Fatalf("expected mapped mkvmerge progress [80], got %v", percents)
	}
	if remainder != "" {
		t.Fatalf("expected empty remainder, got %q", remainder)
	}
}
