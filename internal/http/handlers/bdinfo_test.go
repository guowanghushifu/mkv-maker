package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBDInfoHandlerParseSupportsRawTextPayloadAndFrontendShape(t *testing.T) {
	h := NewBDInfoHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/bdinfo/parse", strings.NewReader(`{"rawText":"`+escapeJSON(sampleBDInfoForHandler)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Parse(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		PlaylistName   string   `json:"playlistName"`
		DiscTitle      string   `json:"discTitle"`
		Duration       string   `json:"duration"`
		AudioLabels    []string `json:"audioLabels"`
		SubtitleLabels []string `json:"subtitleLabels"`
		RawText        string   `json:"rawText"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.PlaylistName != "00800.MPLS" {
		t.Fatalf("expected playlist 00800.MPLS, got %q", body.PlaylistName)
	}
	if body.DiscTitle != "Nightcrawler" {
		t.Fatalf("expected disc title Nightcrawler, got %q", body.DiscTitle)
	}
	if len(body.AudioLabels) == 0 || len(body.SubtitleLabels) == 0 {
		t.Fatalf("expected non-empty audio and subtitle labels, got %+v %+v", body.AudioLabels, body.SubtitleLabels)
	}
	if body.RawText == "" {
		t.Fatal("expected rawText to be echoed back")
	}
}

func TestBDInfoHandlerParseReturnsBadRequestOnUnparseableLogText(t *testing.T) {
	h := NewBDInfoHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/bdinfo/parse", strings.NewReader(`{"rawText":"not bdinfo"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Parse(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestBDInfoHandlerParseRejectsOversizedPayload(t *testing.T) {
	h := NewBDInfoHandler()
	oversized := `{"rawText":"` + strings.Repeat("A", 3<<20) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/api/bdinfo/parse", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Parse(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

const sampleBDInfoForHandler = `Disc Title: Nightcrawler
PLAYLIST REPORT:
Name: 00800.MPLS
Length: 1:57:49.645 (h:m:s.ms)
VIDEO:
MPEG-H HEVC Video       57999 kbps          2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020
* MPEG-H HEVC Video     2100 kbps           1080p / 23.976 fps / 16:9 / Main 10 / Dolby Vision Enhancement Layer
AUDIO:
Dolby TrueHD/Atmos Audio        English         3984 kbps       7.1 / 48 kHz / 3984 kbps / 24-bit
Dolby Digital Audio             Chinese         640 kbps        5.1 / 48 kHz / 640 kbps / 普通话
SUBTITLES:
Chinese                         23.123 kbps     国配简体特效`

func escapeJSON(input string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", ``,
	)
	return replacer.Replace(input)
}
