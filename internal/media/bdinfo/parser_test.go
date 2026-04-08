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
	if parsed.Video.HDRType != "DV.HDR" {
		t.Fatalf("expected DV.HDR type, got %q", parsed.Video.HDRType)
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

	expectedSubtitles := []string{
		"国配简体特效",
		"简英特效",
		"English",
	}
	if !reflect.DeepEqual(parsed.SubtitleLabels, expectedSubtitles) {
		t.Fatalf("expected subtitle labels %+v, got %+v", expectedSubtitles, parsed.SubtitleLabels)
	}
	expectedSubtitleLanguages := []string{
		"Chinese",
		"English",
		"English",
	}
	if !reflect.DeepEqual(parsed.SubtitleLanguages, expectedSubtitleLanguages) {
		t.Fatalf("expected subtitle languages %+v, got %+v", expectedSubtitleLanguages, parsed.SubtitleLanguages)
	}
	if !reflect.DeepEqual(parsed.StreamFiles, []string{"00005.M2TS"}) {
		t.Fatalf("expected parsed stream files, got %+v", parsed.StreamFiles)
	}
}

func TestParseExtractsSubtitleLanguageFromCodecLanguageBitrateDescriptionTable(t *testing.T) {
	raw := `PLAYLIST REPORT:
Name: 00800.MPLS

SUBTITLES:

Codec                           Language        Bitrate         Description
-----                           --------        -------         -----------
Presentation Graphics           English         54.085 kbps
Presentation Graphics           French          43.255 kbps
Presentation Graphics           Chinese         31.415 kbps                    简体中文特效
`

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !reflect.DeepEqual(parsed.SubtitleLanguages, []string{"English", "French", "Chinese"}) {
		t.Fatalf("expected subtitle languages from Language column, got %+v", parsed.SubtitleLanguages)
	}
	if !reflect.DeepEqual(parsed.SubtitleLabels, []string{"English", "French", "简体中文特效"}) {
		t.Fatalf("expected subtitle labels to preserve display text, got %+v", parsed.SubtitleLabels)
	}
}

func TestParseExtractsSubtitleDescriptionWhenBitrateAndDescriptionUseSingleSpace(t *testing.T) {
	raw := `PLAYLIST REPORT:
Name: 00801.MPLS

SUBTITLES:

Codec                           Language        Bitrate         Description
-----                           --------        -------         -----------
Presentation Graphics           Chinese         126.713 kbps 简英特效
`

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !reflect.DeepEqual(parsed.SubtitleLanguages, []string{"Chinese"}) {
		t.Fatalf("expected subtitle language Chinese, got %+v", parsed.SubtitleLanguages)
	}
	if !reflect.DeepEqual(parsed.SubtitleLabels, []string{"简英特效"}) {
		t.Fatalf("expected subtitle label 简英特效, got %+v", parsed.SubtitleLabels)
	}
}

func TestParseSkipsHiddenAudioAndSubtitleRowsButKeepsHiddenDVVideo(t *testing.T) {
	raw := `PLAYLIST REPORT:
Name: 00802.MPLS

VIDEO:
Codec                   Bitrate             Description
-----                   -------             -----------
MPEG-H HEVC Video       57999 kbps          2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020
* MPEG-H HEVC Video     2100 kbps           1080p / 23.976 fps / 16:9 / Main 10 / Dolby Vision Enhancement Layer

AUDIO:
Codec                           Language        Bitrate         Description
-----                           --------        -------         -----------
Dolby TrueHD/Atmos Audio        English         3984 kbps       7.1 / 48 kHz / 3984 kbps / 24-bit
* Dolby Digital Audio           Chinese         640 kbps        5.1 / 48 kHz / 640 kbps / 普通话
DTS-HD Master Audio             Japanese        2123 kbps       5.1 / 48 kHz / 2123 kbps / 日语评论音轨

SUBTITLES:
Codec                           Language        Bitrate         Description
-----                           --------        -------         -----------
Presentation Graphics           English         54.085 kbps
* Presentation Graphics         Chinese         31.415 kbps                    简体中文特效
Presentation Graphics           Japanese        30.071 kbps                    日文注释
`

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !reflect.DeepEqual(parsed.AudioLabels, []string{"English Dolby TrueHD/Atmos Audio", "日语评论音轨"}) {
		t.Fatalf("expected hidden audio row to be skipped, got %+v", parsed.AudioLabels)
	}
	if !reflect.DeepEqual(parsed.AudioLanguages, []string{"English", "Japanese"}) {
		t.Fatalf("expected visible audio languages only, got %+v", parsed.AudioLanguages)
	}
	if !reflect.DeepEqual(parsed.AudioSourceIndexes, []int{0, 2}) {
		t.Fatalf("expected visible audio source indexes [0 2], got %+v", parsed.AudioSourceIndexes)
	}
	if !reflect.DeepEqual(parsed.SubtitleLabels, []string{"English", "日文注释"}) {
		t.Fatalf("expected hidden subtitle row to be skipped, got %+v", parsed.SubtitleLabels)
	}
	if !reflect.DeepEqual(parsed.SubtitleLanguages, []string{"English", "Japanese"}) {
		t.Fatalf("expected visible subtitle languages only, got %+v", parsed.SubtitleLanguages)
	}
	if !reflect.DeepEqual(parsed.SubtitleSourceIndexes, []int{0, 2}) {
		t.Fatalf("expected visible subtitle source indexes [0 2], got %+v", parsed.SubtitleSourceIndexes)
	}
	if parsed.Video.HDRType != "DV.HDR" || !parsed.DVMergeEnabled {
		t.Fatalf("expected hidden DV video row to remain effective, got video=%+v merge=%v", parsed.Video, parsed.DVMergeEnabled)
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
}

func TestParseNormalizesUnicodeWhitespaceAndFullwidthColons(t *testing.T) {
	raw := `DISC INFO：

Disc Title：     Dame, König, As, Spion - 4K UHD

PLAYLIST REPORT：

Name：                   00002.MPLS
Length：                 2:07:22.134 (h:m:s.ms)

VIDEO：

Codec                   Bitrate             Description     
-----                   -------             -----------     
MPEG-H HEVC Video       77112 kbps          2160p / 23.976 fps / 16:9 / Main 10 @ Level 5.1 @ High / 4:2:0 / 10 bits / HDR10 / Limited Range / BT.2020 / 
* MPEG-H HEVC Video     76 kbps             1080p / 23.976 fps / 16:9 / Main 10 @ Level 5.1 @ High / 4:2:0 / 10 bits / Dolby Vision / Limited Range / BT.2020 /

AUDIO：

Codec                           Language        Bitrate         Description     
-----                           --------        -------         -----------     
DTS-HD Master Audio             German          2358 kbps       5.1 / 48 kHz /  2358 kbps / 24-bit
DTS-HD Master Audio             English         3230 kbps       5.1 / 48 kHz /  3230 kbps / 24-bit
Dolby Digital Audio             Chinese         192 kbps        2.0 / 48 kHz /   192 kbps / 普通话

SUBTITLES：

Codec                           Language        Bitrate         Description     
-----                           --------        -------         -----------     
Presentation Graphics           German          0.945 kbps
Presentation Graphics           English         25.265 kbps
Presentation Graphics           Chinese         99.216 kbps                     简体中文`

	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.RawText != raw {
		t.Fatal("expected raw text to be preserved exactly")
	}
	if parsed.DiscTitle != "Dame, König, As, Spion - 4K UHD" {
		t.Fatalf("expected normalized disc title, got %q", parsed.DiscTitle)
	}
	if parsed.PlaylistName != "00002.MPLS" {
		t.Fatalf("expected playlist 00002.MPLS, got %q", parsed.PlaylistName)
	}
	if parsed.Duration != "2:07:22.134 (h:m:s.ms)" {
		t.Fatalf("expected normalized duration, got %q", parsed.Duration)
	}
	if parsed.Video.Codec != "HEVC" {
		t.Fatalf("expected HEVC video codec, got %q", parsed.Video.Codec)
	}
	if parsed.Video.Resolution != "2160p" {
		t.Fatalf("expected 2160p resolution, got %q", parsed.Video.Resolution)
	}
	if parsed.Video.HDRType != "DV.HDR" {
		t.Fatalf("expected DV.HDR type, got %q", parsed.Video.HDRType)
	}
	if !reflect.DeepEqual(parsed.AudioLabels, []string{"German DTS-HD Master Audio", "English DTS-HD Master Audio", "普通话"}) {
		t.Fatalf("expected normalized audio labels, got %+v", parsed.AudioLabels)
	}
	if !reflect.DeepEqual(parsed.SubtitleLabels, []string{"German", "English", "简体中文"}) {
		t.Fatalf("expected normalized subtitle labels, got %+v", parsed.SubtitleLabels)
	}
}

