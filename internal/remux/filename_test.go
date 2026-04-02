package remux

import "testing"

func TestBuildFilenameIncludesHDRAndDefaultAudio(t *testing.T) {
	draft := Draft{
		Title: "Nightcrawler",
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
			HDRType:    "DV.HDR",
		},
		Audio: []AudioTrack{
			{Name: "English", CodecLabel: "TrueHD.7.1.Atmos", Default: true, Selected: true},
		},
	}

	got := BuildFilename(draft)
	want := "Nightcrawler - 2160p.BluRay.DV.HDR.HEVC.TrueHD.7.1.Atmos.mkv"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildFilenameUsesSelectedAudioWhenNoSelectedDefault(t *testing.T) {
	draft := Draft{
		Title: "Nightcrawler",
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
			HDRType:    "DV.HDR",
		},
		Audio: []AudioTrack{
			{Name: "English", CodecLabel: "DTS-HD.MA.5.1", Selected: true},
		},
	}

	got := BuildFilename(draft)
	want := "Nightcrawler - 2160p.BluRay.DV.HDR.HEVC.DTS-HD.MA.5.1.mkv"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildFilenameAddsDVWhenEnabled(t *testing.T) {
	draft := Draft{
		Title:    "Nightcrawler",
		EnableDV: true,
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
		},
	}

	got := BuildFilename(draft)
	want := "Nightcrawler - 2160p.BluRay.DV.HDR.HEVC.UnknownAudio.mkv"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildFilenameSanitizesIllegalCharacters(t *testing.T) {
	draft := Draft{
		Title: "Foo/Bar: The Cut",
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
			HDRType:    "HDR",
		},
		Audio: []AudioTrack{
			{CodecLabel: "DTS-HD.MA.5.1", Selected: true, Default: true},
		},
	}

	got := BuildFilename(draft)
	want := "FooBar The Cut - 2160p.BluRay.HDR.HEVC.DTS-HD.MA.5.1.mkv"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildFilenamePreservesASCIIParenthesesAndRewritesUnderscores(t *testing.T) {
	draft := Draft{
		Title: "Alien_(1979)",
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
			HDRType:    "HDR",
		},
		Audio: []AudioTrack{
			{CodecLabel: "DTS_HD.MA.5.1", Selected: true, Default: true},
		},
	}

	got := BuildFilename(draft)
	want := "Alien.(1979) - 2160p.BluRay.HDR.HEVC.DTS.HD.MA.5.1.mkv"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
