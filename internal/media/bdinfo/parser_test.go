package bdinfo

import "testing"

func TestParseExtractsPlaylistAndTracks(t *testing.T) {
	logText := `
PLAYLIST: 00800.MPLS
VIDEO: MPEG-H HEVC Video / 57999 kbps / 2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020
AUDIO: English / Dolby TrueHD/Atmos Audio / 7.1 / 48 kHz / 3984 kbps / 24-bit
SUBTITLE: English / 20.123 kbps
`

	parsed, err := Parse(logText)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.PlaylistName != "00800.MPLS" {
		t.Fatalf("expected playlist 00800.MPLS, got %q", parsed.PlaylistName)
	}
	if len(parsed.AudioTracks) != 1 || len(parsed.SubtitleTracks) != 1 {
		t.Fatal("expected one audio track and one subtitle track")
	}
}

func TestParseReturnsErrorWhenNoRecognizedFields(t *testing.T) {
	_, err := Parse("this is not a bdinfo log")
	if err == nil {
		t.Fatal("expected Parse to return error for unrecognized input")
	}
}
