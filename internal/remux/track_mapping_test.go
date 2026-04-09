package remux

import "testing"

func TestBuildResolvedTrackSelectorsBySourceIndexSkipsAdjacentDuplicateSubtitleTracks(t *testing.T) {
	draft := Draft{
		Audio: []AudioTrack{
			{ID: "A0", SourceIndex: 0, Name: "English", Language: "eng", Selected: true},
		},
		Subtitles: []SubtitleTrack{
			{ID: "S0", SourceIndex: 0, Name: "Chinese PGS", Language: "chi", Selected: true},
			{ID: "S1", SourceIndex: 1, Name: "Polish PGS", Language: "pol", Selected: true},
		},
	}

	identifyJSON := []byte(`{
		"tracks":[
			{"id":0,"type":"video","properties":{"number":1}},
			{"id":3,"type":"audio","properties":{"number":2}},
			{"id":22,"type":"subtitles","properties":{"number":29,"tag_source_id":"0012A9"}},
			{"id":23,"type":"subtitles","properties":{"number":30,"tag_source_id":"0012A9"}},
			{"id":24,"type":"subtitles","properties":{"number":31,"tag_source_id":"0012AA"}}
		]
	}`)

	audioSelectors, subtitleSelectors, err := BuildResolvedTrackSelectorsBySourceIndex(draft, identifyJSON)
	if err != nil {
		t.Fatalf("BuildResolvedTrackSelectorsBySourceIndex returned error: %v", err)
	}

	if len(audioSelectors) != 1 {
		t.Fatalf("expected one audio selector, got %+v", audioSelectors)
	}
	if audioSelectors[0] != (ResolvedTrackSelector{SourceIndex: 0, TrackID: "3"}) {
		t.Fatalf("expected audio selector {0 3}, got %+v", audioSelectors[0])
	}

	if len(subtitleSelectors) != 2 {
		t.Fatalf("expected two subtitle selectors, got %+v", subtitleSelectors)
	}
	if subtitleSelectors[0] != (ResolvedTrackSelector{SourceIndex: 0, TrackID: "22"}) {
		t.Fatalf("expected first subtitle selector {0 22}, got %+v", subtitleSelectors[0])
	}
	if subtitleSelectors[1] != (ResolvedTrackSelector{SourceIndex: 1, TrackID: "24"}) {
		t.Fatalf("expected second subtitle selector {1 24}, got %+v", subtitleSelectors[1])
	}
}
