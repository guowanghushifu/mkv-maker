package remux

import "testing"

func TestBuildFilenameIncludesHDRAndDefaultAudio(t *testing.T) {
	draft := Draft{
		Title: "Nightcrawler",
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
			HDRType:    "HDR.DV",
		},
		Audio: []AudioTrack{
			{Name: "English", CodecLabel: "TrueHD.7.1.Atmos", Default: true, Selected: true},
		},
	}

	got := BuildFilename(draft)
	want := "Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv"
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
			HDRType:    "HDR.DV",
		},
		Audio: []AudioTrack{
			{Name: "English", CodecLabel: "DTS-HD.MA.5.1", Selected: true},
		},
	}

	got := BuildFilename(draft)
	want := "Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.DTS-HD.MA.5.1.mkv"
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
	want := "Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.UnknownAudio.mkv"
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
