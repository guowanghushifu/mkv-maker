package bdinfo

import (
	"reflect"
	"testing"
)

const sampleBDInfo = `Disc Title: Nightcrawler
Disc Label: NIGHTCRAWLER_UHD

PLAYLIST REPORT:

Name:                   00800.MPLS
Length:                 1:57:49.645 (h:m:s.ms)

VIDEO:

Codec                   Bitrate             Description
-----                   -------             -----------
MPEG-H HEVC Video       57999 kbps          2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020
* MPEG-H HEVC Video     2100 kbps           1080p / 23.976 fps / 16:9 / Main 10 / Dolby Vision Enhancement Layer

AUDIO:

Codec                           Language        Bitrate         Description
-----                           --------        -------         -----------
Dolby TrueHD/Atmos Audio        English         3984 kbps       7.1 / 48 kHz / 3984 kbps / 24-bit
Dolby Digital Audio             Chinese         640 kbps        5.1 / 48 kHz / 640 kbps / 普通话
DTS-HD Master Audio             Chinese         2123 kbps       5.1 / 48 kHz / 2123 kbps / 国配简体特效

SUBTITLES:

Language                        Bitrate         Description
--------                        -------         -----------
Chinese                         23.123 kbps     国配简体特效
English                         18.200 kbps     简英特效
English                         17.000 kbps

FILES:

Name            Time In         Length          Size            Total Bitrate
----            -------         ------          ----            -------------
00005.M2TS      0:00:00.000     1:57:49.645     86,624,038,080  76,229`

func TestParseExtractsPlaylistAndFrontendFieldsFromTables(t *testing.T) {
	parsed, err := Parse(sampleBDInfo)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.PlaylistName != "00800.MPLS" {
		t.Fatalf("expected playlist 00800.MPLS, got %q", parsed.PlaylistName)
	}
	if parsed.DiscTitle != "Nightcrawler" {
		t.Fatalf("expected disc title Nightcrawler, got %q", parsed.DiscTitle)
	}
	if parsed.Duration != "1:57:49.645 (h:m:s.ms)" {
		t.Fatalf("expected duration 1:57:49.645 (h:m:s.ms), got %q", parsed.Duration)
	}
	if parsed.RawText != sampleBDInfo {
		t.Fatalf("expected raw text to be preserved")
	}
	if parsed.Video.Codec != "HEVC" {
		t.Fatalf("expected HEVC video codec, got %q", parsed.Video.Codec)
	}
	if parsed.Video.Resolution != "2160p" {
		t.Fatalf("expected 2160p resolution, got %q", parsed.Video.Resolution)
	}
	if parsed.Video.HDRType != "HDR.DV" {
		t.Fatalf("expected HDR.DV type, got %q", parsed.Video.HDRType)
	}
	if !parsed.DVMergeEnabled {
		t.Fatal("expected DV merge to be enabled when hidden DV enhancement row exists")
	}

	expectedAudio := []string{
		"English Dolby TrueHD/Atmos Audio",
		"普通话",
		"国配简体特效",
	}
	if !reflect.DeepEqual(parsed.AudioLabels, expectedAudio) {
		t.Fatalf("expected audio labels %+v, got %+v", expectedAudio, parsed.AudioLabels)
	}
	expectedAudioCodecInfo := []string{
		"TrueHD.7.1.Atmos",
		"DD.5.1",
		"DTS-HD.MA.5.1",
	}
	if !reflect.DeepEqual(parsed.AudioCodecInfo, expectedAudioCodecInfo) {
		t.Fatalf("expected audio codec info %+v, got %+v", expectedAudioCodecInfo, parsed.AudioCodecInfo)
	}

	expectedSubtitles := []string{
		"国配简体特效",
		"简英特效",
		"English",
	}
	if !reflect.DeepEqual(parsed.SubtitleLabels, expectedSubtitles) {
		t.Fatalf("expected subtitle labels %+v, got %+v", expectedSubtitles, parsed.SubtitleLabels)
	}
	if !reflect.DeepEqual(parsed.StreamFiles, []string{"00005.M2TS"}) {
		t.Fatalf("expected parsed stream files, got %+v", parsed.StreamFiles)
	}
}

func TestParseReturnsErrorWhenNoRecognizedFields(t *testing.T) {
	_, err := Parse("this is not a bdinfo log")
	if err == nil {
		t.Fatal("expected Parse to return error for unrecognized input")
	}
}

func TestParseSupportsLegacySingleLineFormat(t *testing.T) {
	raw := `
PLAYLIST: 00800.MPLS
VIDEO: MPEG-H HEVC Video / 57999 kbps / 2160p / 23.976 fps / HDR10
AUDIO: English / Dolby TrueHD/Atmos Audio / 7.1 / 48 kHz / 3984 kbps / 24-bit
SUBTITLE: Chinese / 国配简体特效
`

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.PlaylistName != "00800.MPLS" {
		t.Fatalf("expected playlist 00800.MPLS, got %q", parsed.PlaylistName)
	}
	if len(parsed.AudioLabels) != 1 || parsed.AudioLabels[0] == "" {
		t.Fatalf("expected one parsed audio label, got %+v", parsed.AudioLabels)
	}
	if len(parsed.AudioCodecInfo) != 1 || parsed.AudioCodecInfo[0] != "TrueHD.7.1.Atmos" {
		t.Fatalf("expected legacy codec info to be preserved, got %+v", parsed.AudioCodecInfo)
	}
	if len(parsed.SubtitleLabels) != 1 || parsed.SubtitleLabels[0] != "国配简体特效" {
		t.Fatalf("expected legacy subtitle label to be preserved, got %+v", parsed.SubtitleLabels)
	}
}

func TestParseStripsEmbeddedMetadataBeforeChineseAudioSuffix(t *testing.T) {
	raw := `PLAYLIST REPORT:
Name: 00003.MPLS

AUDIO:

Codec                           Language        Bitrate         Description
-----                           --------        -------         -----------
Dolby TrueHD/Atmos Audio        English         3886 kbps       7.1 / 48 kHz / 3246 kbps / 16-bit (AC3 Embedded: 5.1 / 48 kHz / 640 kbps / DN -29dB)   英文次世代全景声
`

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(parsed.AudioLabels) != 1 {
		t.Fatalf("expected one parsed audio label, got %+v", parsed.AudioLabels)
	}
	if parsed.AudioLabels[0] != "英文次世代全景声" {
		t.Fatalf("expected cleaned chinese audio suffix, got %+v", parsed.AudioLabels)
	}
	if len(parsed.AudioCodecInfo) != 1 || parsed.AudioCodecInfo[0] != "TrueHD.7.1.Atmos" {
		t.Fatalf("expected codec info TrueHD.7.1.Atmos, got %+v", parsed.AudioCodecInfo)
	}
}
