package makemkv

import (
	"reflect"
	"testing"
)

const sampleRobotOutput = `TINFO:0,2,0,"Nightcrawler"
TINFO:0,16,0,"00800"
SINFO:0,0,1,6201,"Audio"
SINFO:0,0,3,0,"eng"
SINFO:0,0,4,0,"English"
SINFO:0,0,5,0,"A_TRUEHD"
SINFO:0,0,6,0,"TrueHD"
SINFO:0,0,14,0,"8"
SINFO:0,0,22,0,"0"
SINFO:0,0,38,0,"default"
SINFO:0,0,39,0,"default"
SINFO:0,0,40,0,"7.1"
SINFO:0,0,30,0,"English Atmos"
SINFO:0,1,1,6201,"Audio"
SINFO:0,1,3,0,"eng"
SINFO:0,1,4,0,"English"
SINFO:0,1,5,0,"A_AC3"
SINFO:0,1,6,0,"AC3"
SINFO:0,1,14,0,"6"
SINFO:0,1,30,0,"English Compatibility Track"
SINFO:0,2,1,6203,"Subtitles"
SINFO:0,2,3,0,"eng"
SINFO:0,2,4,0,"English"
SINFO:0,2,22,0,"4096"
SINFO:0,2,30,0,"English (forced only)"`

func TestParseRobotOutputBuildsPlaylistTracksAndFiltersCompatibilityAudio(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(sampleRobotOutput))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00800.MPLS")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if view.TitleID != 0 {
		t.Fatalf("expected title id 0, got %d", view.TitleID)
	}
	if len(view.Audio) != 1 {
		t.Fatalf("expected compatibility track to be filtered, got %+v", view.Audio)
	}
	if view.Audio[0].ID != "A1" || view.Audio[0].SourceIndex != 0 {
		t.Fatalf("expected visible audio id/sourceIndex A1/0, got %+v", view.Audio[0])
	}
	if !view.Audio[0].Default {
		t.Fatalf("expected audio default=true, got %+v", view.Audio[0])
	}
	if len(view.Subtitles) != 0 {
		t.Fatalf("expected forced subtitles to be filtered from visible list, got %+v", view.Subtitles)
	}
}

func TestTitleByPlaylistMatchesPlaylistWithOrWithoutExtension(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:3,16,0,"00801"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}
	if _, err := parsed.TitleByPlaylist("00801.MPLS"); err != nil {
		t.Fatalf("expected playlist lookup to succeed with extension: %v", err)
	}
	if _, err := parsed.TitleByPlaylist("00801"); err != nil {
		t.Fatalf("expected playlist lookup to succeed without extension: %v", err)
	}
}

func TestBuildTitleViewKeepsCoreTrackWhenLanguageDiffers(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:1,16,0,"00802"
SINFO:1,0,1,6201,"Audio"
SINFO:1,0,3,0,"eng"
SINFO:1,0,5,0,"A_TRUEHD"
SINFO:1,0,14,0,"6"
SINFO:1,0,30,0,"English TrueHD"
SINFO:1,1,1,6201,"Audio"
SINFO:1,1,3,0,"jpn"
SINFO:1,1,5,0,"A_AC3"
SINFO:1,1,14,0,"6"
SINFO:1,1,30,0,"Japanese AC3"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00802")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 2 {
		t.Fatalf("expected language-mismatched AC3 track to remain visible, got %+v", view.Audio)
	}
}

func TestBuildTitleViewFiltersAdjacentAC3WhenChannelsIncrease(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:1,16,0,"00814"
SINFO:1,0,1,6201,"Audio"
SINFO:1,0,3,0,"eng"
SINFO:1,0,5,0,"A_TRUEHD"
SINFO:1,0,14,0,"6"
SINFO:1,0,30,0,"English TrueHD"
SINFO:1,1,1,6201,"Audio"
SINFO:1,1,3,0,"eng"
SINFO:1,1,5,0,"A_AC3"
SINFO:1,1,14,0,"8"
SINFO:1,1,30,0,"English AC3 7.1"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00814")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected adjacent higher-channel AC3 track to be filtered by new rule, got %+v", view.Audio)
	}
}

func TestBuildTitleViewFiltersDTSCompatibilityCore(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:2,16,0,"00803"
SINFO:2,0,1,6201,"Audio"
SINFO:2,0,3,0,"eng"
SINFO:2,0,4,0,"English"
SINFO:2,0,5,0,"A_DTS"
SINFO:2,0,6,0,"DTS-HD MA"
SINFO:2,0,14,0,"6"
SINFO:2,0,38,0,"default"
SINFO:2,0,30,0,"English DTS-HD MA"
SINFO:2,1,1,6201,"Audio"
SINFO:2,1,3,0,"eng"
SINFO:2,1,4,0,"English"
SINFO:2,1,5,0,"A_DTS"
SINFO:2,1,6,0,"DTS"
SINFO:2,1,14,0,"6"
SINFO:2,1,30,0,"English DTS Core"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00803")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected DTS compatibility core to be filtered, got %+v", view.Audio)
	}
	if !view.Audio[0].Default {
		t.Fatalf("expected exact default marker to stay default, got %+v", view.Audio[0])
	}
}

func TestBuildTitleViewKeepsStereoCommentaryAfterLosslessMainTrack(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:12,16,0,"00813"
SINFO:12,0,1,6201,"Audio"
SINFO:12,0,3,0,"eng"
SINFO:12,0,5,0,"A_TRUEHD"
SINFO:12,0,6,0,"TrueHD"
SINFO:12,0,14,0,"8"
SINFO:12,0,30,0,"English TrueHD"
SINFO:12,1,1,6201,"Audio"
SINFO:12,1,3,0,"eng"
SINFO:12,1,5,0,"A_AC3"
SINFO:12,1,6,0,"AC3"
SINFO:12,1,14,0,"2"
SINFO:12,1,30,0,"English Commentary"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00813")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 2 {
		t.Fatalf("expected stereo commentary to remain visible, got %+v", view.Audio)
	}
}

func TestBuildTitleViewDoesNotInferDefaultFromNotDefaultText(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:3,16,0,"00804"
SINFO:3,0,1,6201,"Audio"
SINFO:3,0,3,0,"eng"
SINFO:3,0,4,0,"English"
SINFO:3,0,5,0,"A_EAC3"
SINFO:3,0,6,0,"E-AC3"
SINFO:3,0,14,0,"6"
SINFO:3,0,39,0,"not default"
SINFO:3,0,30,0,"English DD+"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00804")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected one audio track, got %+v", view.Audio)
	}
	if view.Audio[0].Default {
		t.Fatalf("expected non-default text not to infer default, got %+v", view.Audio[0])
	}
}

func TestBuildTitleViewFiltersForcedSubtitleTracksFromVisibleSubtitles(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:15,16,0,"00817"
SINFO:15,0,1,6203,"Subtitles"
SINFO:15,0,3,0,"eng"
SINFO:15,0,4,0,"English"
SINFO:15,0,22,0,"0"
SINFO:15,0,30,0,"English"
SINFO:15,1,1,6203,"Subtitles"
SINFO:15,1,3,0,"spa"
SINFO:15,1,4,0,"Spanish"
SINFO:15,1,22,0,"0"
SINFO:15,1,30,0,"PGS Spanish  (forced only)"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00817")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Subtitles) != 1 {
		t.Fatalf("expected only non-forced subtitle to remain visible, got %+v", view.Subtitles)
	}
	if view.Subtitles[0].ID != "S1" {
		t.Fatalf("expected remaining subtitle id S1, got %+v", view.Subtitles[0])
	}
	if view.Subtitles[0].Name != "English" {
		t.Fatalf("expected normal subtitle name to remain unchanged, got %+v", view.Subtitles[0])
	}
	if view.Subtitles[0].Language != "eng" {
		t.Fatalf("expected remaining subtitle language eng, got %+v", view.Subtitles[0])
	}
	if view.Subtitles[0].Forced {
		t.Fatalf("expected remaining visible subtitle not to be forced, got %+v", view.Subtitles[0])
	}
}

func TestBuildTitleViewFiltersEAC3CompatibilityCore(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:5,16,0,"00806"
SINFO:5,0,1,6201,"Audio"
SINFO:5,0,3,0,"eng"
SINFO:5,0,5,0,"A_EAC3"
SINFO:5,0,6,0,"E-AC3"
SINFO:5,0,14,0,"6"
SINFO:5,0,30,0,"English DD+"
SINFO:5,1,1,6201,"Audio"
SINFO:5,1,3,0,"eng"
SINFO:5,1,5,0,"A_AC3"
SINFO:5,1,6,0,"AC3"
SINFO:5,1,14,0,"6"
SINFO:5,1,30,0,"English AC3 Compatibility"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00806")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected E-AC3 compatibility core to be filtered, got %+v", view.Audio)
	}
}

func TestBuildTitleViewFiltersDTSHDRACompatibilityCore(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:6,16,0,"00807"
SINFO:6,0,1,6201,"Audio"
SINFO:6,0,3,0,"eng"
SINFO:6,0,5,0,"A_DTS"
SINFO:6,0,6,0,"DTS-HD HRA"
SINFO:6,0,14,0,"6"
SINFO:6,0,30,0,"English DTS-HD HRA"
SINFO:6,1,1,6201,"Audio"
SINFO:6,1,3,0,"eng"
SINFO:6,1,5,0,"A_DTS"
SINFO:6,1,6,0,"DTS"
SINFO:6,1,14,0,"6"
SINFO:6,1,30,0,"English DTS Compatibility"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00807")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected DTS-HD HRA compatibility core to be filtered, got %+v", view.Audio)
	}
}

func TestBuildTitleViewKeepsPlainDTSPairVisible(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:13,16,0,"00815"
SINFO:13,0,1,6201,"Audio"
SINFO:13,0,3,0,"eng"
SINFO:13,0,5,0,"A_DTS"
SINFO:13,0,6,0,"DTS"
SINFO:13,0,14,0,"6"
SINFO:13,0,30,0,"English DTS"
SINFO:13,1,1,6201,"Audio"
SINFO:13,1,3,0,"eng"
SINFO:13,1,5,0,"A_DTS"
SINFO:13,1,6,0,"DTS"
SINFO:13,1,14,0,"6"
SINFO:13,1,30,0,"English DTS Duplicate"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00815")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 2 {
		t.Fatalf("expected plain DTS pair to remain visible, got %+v", view.Audio)
	}
	if view.Audio[0].CodecLabel != "DTS" || view.Audio[1].CodecLabel != "DTS" {
		t.Fatalf("expected plain DTS codec labels without layout metadata, got %+v", view.Audio)
	}
}

func TestBuildTitleViewNormalizesPlainDTSWithChannelLayout(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:16,16,0,"00818"
SINFO:16,0,1,6201,"Audio"
SINFO:16,0,3,0,"eng"
SINFO:16,0,4,0,"English"
SINFO:16,0,5,0,"A_DTS"
SINFO:16,0,6,0,"DTS"
SINFO:16,0,14,0,"6"
SINFO:16,0,30,0,"English DTS"
SINFO:16,0,40,0,"5.1(side)"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00818")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 || view.Audio[0].CodecLabel != "DTS.5.1" {
		t.Fatalf("expected plain DTS codec label with layout metadata, got %+v", view.Audio)
	}
}

func TestBuildTitleViewNormalizesDTSHDMAFromMakeMKVFields(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:15,16,0,"00817"
SINFO:15,0,1,6201,"Audio"
SINFO:15,0,3,0,"eng"
SINFO:15,0,4,0,"English"
SINFO:15,0,5,0,"A_DTS"
SINFO:15,0,6,0,"DTS-HD MA"
SINFO:15,0,7,0,"DTS-HD Master Audio"
SINFO:15,0,14,0,"8"
SINFO:15,0,30,0,"DTS-HD MA Surround 7.1 English"
SINFO:15,0,40,0,"7.1"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00817")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 || view.Audio[0].CodecLabel != "DTS-HD.MA.7.1" {
		t.Fatalf("expected DTS-HD MA codec label from MakeMKV fields, got %+v", view.Audio)
	}
}

func TestBuildTitleViewNormalizesPlainDDWithChannelLayout(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:17,16,0,"00819"
SINFO:17,0,1,6201,"Audio"
SINFO:17,0,3,0,"spa"
SINFO:17,0,4,0,"Spanish"
SINFO:17,0,5,0,"A_AC3"
SINFO:17,0,6,0,"DD"
SINFO:17,0,14,0,"6"
SINFO:17,0,30,0,"DD Surround 5.1 Spanish"
SINFO:17,0,40,0,"5.1(side)"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00819")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 || view.Audio[0].CodecLabel != "DD.5.1" {
		t.Fatalf("expected DD codec label with 5.1 layout metadata, got %+v", view.Audio)
	}
}

func TestBuildTitleViewNormalizesPlainDDStereo(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:18,16,0,"00820"
SINFO:18,0,1,6201,"Audio"
SINFO:18,0,3,0,"eng"
SINFO:18,0,4,0,"English"
SINFO:18,0,5,0,"A_AC3"
SINFO:18,0,6,0,"DD"
SINFO:18,0,14,0,"2"
SINFO:18,0,30,0,"DD Stereo English"
SINFO:18,0,40,0,"stereo"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00820")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 || view.Audio[0].CodecLabel != "DD.2.0" {
		t.Fatalf("expected DD codec label with stereo layout metadata, got %+v", view.Audio)
	}
}

func TestBuildTitleViewNormalizesPlainDDMono(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:19,16,0,"00821"
SINFO:19,0,1,6201,"Audio"
SINFO:19,0,3,0,"eng"
SINFO:19,0,4,0,"English"
SINFO:19,0,5,0,"A_AC3"
SINFO:19,0,6,0,"DD"
SINFO:19,0,14,0,"1"
SINFO:19,0,30,0,"DD Mono English"
SINFO:19,0,40,0,"mono"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00821")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 || view.Audio[0].CodecLabel != "DD.1.0" {
		t.Fatalf("expected DD codec label with mono layout metadata, got %+v", view.Audio)
	}
}

func TestBuildTitleViewFiltersDTSCompatibilityCoreWhenPrimaryShortNameMissing(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:14,16,0,"00816"
SINFO:14,0,1,6201,"Audio"
SINFO:14,0,3,0,"eng"
SINFO:14,0,5,0,"A_DTS"
SINFO:14,0,6,0,""
SINFO:14,0,14,0,"6"
SINFO:14,0,30,0,"English DTS-HD"
SINFO:14,1,1,6201,"Audio"
SINFO:14,1,3,0,"eng"
SINFO:14,1,5,0,"A_DTS"
SINFO:14,1,6,0,"DTS"
SINFO:14,1,14,0,"6"
SINFO:14,1,30,0,"English DTS Core"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00816")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected DTS compatibility core to be filtered even without primary short name, got %+v", view.Audio)
	}
}

func TestBuildTitleViewKeepsNonAdjacentCompatibilityCore(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:7,16,0,"00808"
SINFO:7,0,1,6201,"Audio"
SINFO:7,0,3,0,"eng"
SINFO:7,0,5,0,"A_TRUEHD"
SINFO:7,0,6,0,"TrueHD"
SINFO:7,0,14,0,"8"
SINFO:7,0,30,0,"English TrueHD"
SINFO:7,1,1,6201,"Audio"
SINFO:7,1,3,0,"eng"
SINFO:7,1,5,0,"A_PCM/INT/LIT"
SINFO:7,1,6,0,"PCM"
SINFO:7,1,14,0,"2"
SINFO:7,1,30,0,"English PCM"
SINFO:7,2,1,6201,"Audio"
SINFO:7,2,3,0,"eng"
SINFO:7,2,5,0,"A_AC3"
SINFO:7,2,6,0,"AC3"
SINFO:7,2,14,0,"6"
SINFO:7,2,30,0,"English AC3 Compatibility"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00808")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 3 {
		t.Fatalf("expected non-adjacent compatibility track to remain visible, got %+v", view.Audio)
	}
}

func TestParseRobotOutputRetainsCodecLongFromSINFO(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:8,16,0,"00809"
SINFO:8,0,1,6201,"Audio"
SINFO:8,0,5,0,"A_TRUEHD"
SINFO:8,0,6,0,"TrueHD"
SINFO:8,0,7,0,"Dolby TrueHD Audio"
SINFO:8,0,14,0,"8"
SINFO:8,0,40,0,"7.1"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00809")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	if len(title.Tracks) != 1 {
		t.Fatalf("expected one track, got %+v", title.Tracks)
	}
	field := reflect.ValueOf(title.Tracks[0]).FieldByName("CodecLong")
	if !field.IsValid() {
		t.Fatalf("expected TrackInfo to retain CodecLong field, got %+v", title.Tracks[0])
	}
	if field.String() != "Dolby TrueHD Audio" {
		t.Fatalf("expected codec long to be retained, got %+v", title.Tracks[0])
	}
}

func TestBuildTitleViewDerivesReadableAudioNameWithoutDisplayName(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:9,16,0,"00810"
SINFO:9,0,1,6201,"Audio"
SINFO:9,0,3,0,"eng"
SINFO:9,0,4,0,"English"
SINFO:9,0,5,0,"A_TRUEHD"
SINFO:9,0,6,0,""
SINFO:9,0,7,0,"Dolby TrueHD Audio"
SINFO:9,0,14,0,"8"
SINFO:9,0,30,0,"   "
SINFO:9,0,40,0,"7.1"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00810")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Audio) != 1 {
		t.Fatalf("expected one audio track, got %+v", view.Audio)
	}
	if view.Audio[0].Name != "Dolby TrueHD Audio 7.1 English" {
		t.Fatalf("expected derived audio name, got %+v", view.Audio[0])
	}
}

func TestBuildTitleViewSubtitleNamePrefersLangNameOverDisplayName(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:10,16,0,"00811"
SINFO:10,0,1,6203,"Subtitles"
SINFO:10,0,3,0,"eng"
SINFO:10,0,4,0,"English"
SINFO:10,0,30,0,"English SDH"`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00811")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	view, err := BuildTitleView(title)
	if err != nil {
		t.Fatalf("BuildTitleView returned error: %v", err)
	}

	if len(view.Subtitles) != 1 {
		t.Fatalf("expected one subtitle track, got %+v", view.Subtitles)
	}
	if view.Subtitles[0].Name != "English" {
		t.Fatalf("expected subtitle name to prefer langName, got %+v", view.Subtitles[0])
	}
}

func TestBuildTitleViewReturnsErrorWhenVisibleTrackCannotBeBuilt(t *testing.T) {
	parsed, err := ParseRobotOutput([]byte(`TINFO:11,16,0,"00812"
SINFO:11,0,1,6201,"Audio"
SINFO:11,0,30,0,"   "`))
	if err != nil {
		t.Fatalf("ParseRobotOutput returned error: %v", err)
	}

	title, err := parsed.TitleByPlaylist("00812")
	if err != nil {
		t.Fatalf("TitleByPlaylist returned error: %v", err)
	}

	if _, err := BuildTitleView(title); err == nil {
		t.Fatalf("expected BuildTitleView to reject audio tracks missing required visible fields")
	}
}
