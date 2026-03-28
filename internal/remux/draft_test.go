package remux

import "testing"

func TestDraftDefaultSelectedAudioPrefersSelectedDefaultTrack(t *testing.T) {
	draft := Draft{
		Audio: []AudioTrack{
			{Name: "English", CodecLabel: "AAC.2.0", Selected: true},
			{Name: "English", CodecLabel: "TrueHD.7.1.Atmos", Selected: true, Default: true},
		},
	}

	got, ok := draft.DefaultSelectedAudio()
	if !ok {
		t.Fatal("expected a default selected audio track")
	}
	if got.CodecLabel != "TrueHD.7.1.Atmos" {
		t.Fatalf("expected TrueHD.7.1.Atmos, got %q", got.CodecLabel)
	}
}
