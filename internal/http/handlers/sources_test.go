package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/guowanghushifu/mkv-maker/internal/isomount"
	"github.com/guowanghushifu/mkv-maker/internal/media"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

func TestNewSourcesHandlerStoresISOManager(t *testing.T) {
	manager := isomount.NewManager(t.TempDir(), time.Hour, nil)
	h := NewSourcesHandler("/input", "/output", stubSourceScanner{}, stubPlaylistInspector{}, manager)

	if h.ISOManager != manager {
		t.Fatalf("expected ISO manager to be stored")
	}
}

type stubSourceScanner struct {
	items []media.SourceEntry
	err   error
}

func (s stubSourceScanner) Scan(root string) ([]media.SourceEntry, error) {
	return s.items, s.err
}

type stubPlaylistInspector struct {
	result MakeMKVInspection
	err    error
	path   *string
}

func (s stubPlaylistInspector) Inspect(ctx context.Context, sourcePath, playlistPath string) (MakeMKVInspection, error) {
	if s.path != nil {
		*s.path = playlistPath
	}
	return s.result, s.err
}

type stubISOMountManager struct {
	ensureMountedFn func(context.Context, string, string) (string, error)
	touched         []string
}

func (s *stubISOMountManager) EnsureMounted(ctx context.Context, sourceID, isoPath string) (string, error) {
	if s.ensureMountedFn != nil {
		return s.ensureMountedFn(ctx, sourceID, isoPath)
	}
	return "", nil
}

func (s *stubISOMountManager) Touch(sourceID string) {
	s.touched = append(s.touched, sourceID)
}

func TestSourcesHandlerListReturnsStructuredNotFoundError(t *testing.T) {
	h := NewSourcesHandler("/missing/input", "/remux", stubSourceScanner{
		err: &os.PathError{
			Op:   "readdir",
			Path: "/missing/input",
			Err:  os.ErrNotExist,
		},
	}, stubPlaylistInspector{})

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Path    string `json:"path"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "input_dir_not_found" {
		t.Fatalf("expected error code input_dir_not_found, got %q", body.Error.Code)
	}
	if body.Error.Path != "/missing/input" {
		t.Fatalf("expected error path /missing/input, got %q", body.Error.Path)
	}
}

func TestSourcesHandlerListReturnsStructuredUnreadableError(t *testing.T) {
	h := NewSourcesHandler("/restricted/input", "/remux", stubSourceScanner{
		err: &os.PathError{
			Op:   "readdir",
			Path: "/restricted/input",
			Err:  os.ErrPermission,
		},
	}, stubPlaylistInspector{})

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Path    string `json:"path"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "input_dir_unreadable" {
		t.Fatalf("expected error code input_dir_unreadable, got %q", body.Error.Code)
	}
	if body.Error.Path != "/restricted/input" {
		t.Fatalf("expected error path /restricted/input, got %q", body.Error.Path)
	}
}

func TestSourcesHandlerScanReturnsEmptyJSONArrayWhenNoSourcesFound(t *testing.T) {
	h := NewSourcesHandler("/input", "/remux", stubSourceScanner{
		items: nil,
	}, stubPlaylistInspector{})

	req := httptest.NewRequest(http.MethodPost, "/api/sources/scan", nil)
	w := httptest.NewRecorder()
	h.Scan(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Fatalf("expected empty JSON array, got %q", body)
	}
}

func TestSourcesHandlerResolveMountsISOSourceBeforeInspection(t *testing.T) {
	isoPath := filepath.Join(t.TempDir(), "Nightcrawler.iso")
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	mountedRoot := filepath.Join(t.TempDir(), "iso_auto_mount", "movies-nightcrawler-iso")
	playlistPath := filepath.Join(mountedRoot, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatal(err)
	}
	streamPath := filepath.Join(mountedRoot, "BDMV", "STREAM", "00005.m2ts")
	if err := os.MkdirAll(filepath.Dir(streamPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(streamPath, []byte("stream"), 0o644); err != nil {
		t.Fatal(err)
	}
	inputRoot := t.TempDir()
	var inspectedPath string

	isoManager := &stubISOMountManager{
		ensureMountedFn: func(_ context.Context, sourceID, sourcePath string) (string, error) {
			if sourceID != "movies-nightcrawler-iso" || sourcePath != isoPath {
				t.Fatalf("unexpected source %q %q", sourceID, sourcePath)
			}
			return mountedRoot, nil
		},
	}
	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{
		ID: "movies-nightcrawler-iso", Name: "Nightcrawler", Path: isoPath, Type: media.SourceISO,
	}}}, stubPlaylistInspector{
		result: MakeMKVInspection{},
		path: &inspectedPath,
	}, isoManager)
	reqBody := `{
		"sourceId":"movies-nightcrawler-iso",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"discTitle":"Nightcrawler",
			"audioLabels":["English Dolby TrueHD/Atmos Audio"],
			"subtitleLabels":["English PGS"],
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/movies-nightcrawler-iso/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", "movies-nightcrawler-iso")
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	if inspectedPath != playlistPath {
		t.Fatalf("expected ISO-mounted playlist path %q, got %q", playlistPath, inspectedPath)
	}
}

func TestSourcesHandlerResolveBuildsFrontendDraftFromParsedBDInfo(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "Nightcrawler"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	streamPath := filepath.Join(sourcePath, "BDMV", "STREAM", "00005.m2ts")
	if err := os.MkdirAll(filepath.Dir(streamPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(streamPath, []byte("stream"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	var inspectedPath string
	h := NewSourcesHandler(inputRoot, "/custom/remux", stubSourceScanner{
		items: []media.SourceEntry{
			{
				ID:   sourceID,
				Name: "Nightcrawler (2014)",
				Path: sourcePath,
				Type: media.SourceBDMV,
			},
		},
	}, stubPlaylistInspector{
		result: MakeMKVInspection{
			TitleID:      0,
			PlaylistName: "00800.MPLS",
			Audio: []resolveTrack{
				{ID: "A1", Name: "English Atmos", Language: "eng", CodecLabel: "TrueHD.7.1", Selected: true, Default: true, SourceIndex: 0},
				{ID: "A2", Name: "普通话", Language: "chi", Selected: true, Default: false, SourceIndex: 1},
				{ID: "A3", Name: "国配简体特效", Language: "chi", Selected: true, Default: false, SourceIndex: 2},
			},
			Subtitles: []resolveTrack{
				{ID: "S1", Name: "国配简体特效", Language: "chi", Selected: true, Default: true, SourceIndex: 0},
				{ID: "S2", Name: "简英特效", Language: "chi", Selected: true, Default: false, SourceIndex: 1},
			},
			Cache: remux.MakeMKVTitleCache{
				PlaylistName: "00800.MPLS",
				TitleID:      0,
				Audio: []remux.AudioTrack{{ID: "A1", Name: "English Atmos", Language: "eng", CodecLabel: "TrueHD.7.1", Default: true, Selected: true, SourceIndex: 0}},
				Subtitles: []remux.SubtitleTrack{{ID: "S1", Name: "国配简体特效", Language: "chi", Default: true, Selected: true, SourceIndex: 0}},
			},
		},
		path: &inspectedPath,
	})

	reqBody := `{
		"sourceId":"Nightcrawler",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"discTitle":"Nightcrawler",
			"audioLabels":["English Dolby TrueHD/Atmos Audio","普通话","国配简体特效"],
			"subtitleLabels":["国配简体特效","简英特效"],
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nLength: 1:57:49.645 (h:m:s.ms)\nVIDEO:\nMPEG-H HEVC Video       57999 kbps          2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020\n* MPEG-H HEVC Video     2100 kbps           1080p / 23.976 fps / 16:9 / Main 10 / Dolby Vision Enhancement Layer\nAUDIO:\nDolby TrueHD/Atmos Audio        English         3984 kbps       7.1 / 48 kHz / 3984 kbps / 24-bit\nDolby Digital Audio             Chinese         640 kbps        5.1 / 48 kHz / 640 kbps / 普通话\nDTS-HD Master Audio             Chinese         2123 kbps       5.1 / 48 kHz / 2123 kbps / 国配简体特效\nSUBTITLES:\nChinese                         23.123 kbps     国配简体特效\nEnglish                         18.200 kbps     简英特效"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/Nightcrawler/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)

	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		SourceID       string   `json:"sourceId"`
		PlaylistName   string   `json:"playlistName"`
		OutputDir      string   `json:"outputDir"`
		Title          string   `json:"title"`
		DVMergeEnabled bool     `json:"dvMergeEnabled"`
		SegmentPaths   []string `json:"segmentPaths"`
		Video          struct {
			Name       string `json:"name"`
			Codec      string `json:"codec"`
			Resolution string `json:"resolution"`
			HDRType    string `json:"hdrType"`
		} `json:"video"`
		Audio []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Language    string `json:"language"`
			CodecLabel  string `json:"codecLabel"`
			SourceIndex int    `json:"sourceIndex"`
			Selected    bool   `json:"selected"`
			Default     bool   `json:"default"`
		} `json:"audio"`
		Subtitles []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Language    string `json:"language"`
			SourceIndex int    `json:"sourceIndex"`
			Selected    bool   `json:"selected"`
			Default     bool   `json:"default"`
		} `json:"subtitles"`
		MakeMKV struct {
			PlaylistName string `json:"playlistName"`
			TitleID      int    `json:"titleId"`
			Audio        []struct {
				ID          string `json:"id"`
				SourceIndex int    `json:"sourceIndex"`
				Name        string `json:"name"`
				CodecLabel  string `json:"codecLabel"`
				Language    string `json:"language"`
				Selected    bool   `json:"selected"`
				Default     bool   `json:"default"`
			} `json:"audio"`
			Subtitles []struct {
				ID          string `json:"id"`
				SourceIndex int    `json:"sourceIndex"`
				Name        string `json:"name"`
				Language    string `json:"language"`
				Selected    bool   `json:"selected"`
				Default     bool   `json:"default"`
				Forced      bool   `json:"forced"`
			} `json:"subtitles"`
		} `json:"makemkv"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body.SourceID != sourceID {
		t.Fatalf("expected source id %q, got %q", sourceID, body.SourceID)
	}
	if body.PlaylistName != "00800.MPLS" {
		t.Fatalf("expected playlist 00800.MPLS, got %q", body.PlaylistName)
	}
	if inspectedPath != playlistPath {
		t.Fatalf("expected inspector to receive playlist path %q, got %q", playlistPath, inspectedPath)
	}
	if body.OutputDir != "/custom/remux" {
		t.Fatalf("expected outputDir /custom/remux, got %q", body.OutputDir)
	}
	if body.Title != "Nightcrawler" {
		t.Fatalf("expected title Nightcrawler, got %q", body.Title)
	}
	if !body.DVMergeEnabled {
		t.Fatal("expected dvMergeEnabled to be true")
	}
	if len(body.SegmentPaths) != 1 || !strings.HasSuffix(strings.ToLower(body.SegmentPaths[0]), "/bdmv/stream/00005.m2ts") {
		t.Fatalf("expected segment path from FILES section, got %+v", body.SegmentPaths)
	}
	if body.Video.Codec != "HEVC" || body.Video.Resolution != "2160p" || body.Video.HDRType != "DV.HDR" {
		t.Fatalf("unexpected video %+v", body.Video)
	}
	if len(body.Audio) != 3 || len(body.Subtitles) != 2 {
		t.Fatalf("expected 3 audio and 2 subtitles tracks, got %d and %d", len(body.Audio), len(body.Subtitles))
	}
	if !body.Audio[0].Default || !body.Audio[0].Selected {
		t.Fatalf("expected first audio to be default+selected: %+v", body.Audio[0])
	}
	if body.Audio[0].ID != "A1" || body.Audio[1].ID != "A2" || body.Audio[2].ID != "A3" {
		t.Fatalf("expected MakeMKV audio ids, got %+v", body.Audio)
	}
	if body.Audio[0].SourceIndex != 0 || body.Audio[1].SourceIndex != 1 || body.Audio[2].SourceIndex != 2 {
		t.Fatalf("expected MakeMKV audio source indexes, got %+v", body.Audio)
	}
	if body.Audio[0].Name != "English Dolby TrueHD/Atmos Audio" || body.Audio[1].Name != "普通话" || body.Audio[2].Name != "国配简体特效" {
		t.Fatalf("expected BDInfo audio labels to override MakeMKV names, got %+v", body.Audio)
	}
	if body.Audio[0].CodecLabel != "TrueHD.7.1.Atmos" || body.Audio[1].CodecLabel != "DD.5.1" || body.Audio[2].CodecLabel != "DTS-HD.MA.5.1" {
		t.Fatalf("expected release-style audio codec labels, got %+v", body.Audio)
	}
	if !body.Subtitles[0].Default || !body.Subtitles[0].Selected {
		t.Fatalf("expected first subtitle to be default+selected: %+v", body.Subtitles[0])
	}
	if body.Subtitles[0].ID != "S1" || body.Subtitles[1].ID != "S2" {
		t.Fatalf("expected MakeMKV subtitle ids, got %+v", body.Subtitles)
	}
	if body.Subtitles[0].SourceIndex != 0 || body.Subtitles[1].SourceIndex != 1 {
		t.Fatalf("expected MakeMKV subtitle source indexes, got %+v", body.Subtitles)
	}
	if body.Subtitles[0].Name != "国配简体特效" || body.Subtitles[1].Name != "简英特效" {
		t.Fatalf("expected BDInfo subtitle labels to override MakeMKV names, got %+v", body.Subtitles)
	}
	if body.MakeMKV.PlaylistName != "00800.MPLS" || body.MakeMKV.TitleID != 0 {
		t.Fatalf("expected MakeMKV cache metadata, got %+v", body.MakeMKV)
	}
	if len(body.MakeMKV.Audio) != 3 || body.MakeMKV.Audio[0].ID != "A1" || body.MakeMKV.Audio[0].SourceIndex != 0 {
		t.Fatalf("expected fused MakeMKV cache audio payload, got %+v", body.MakeMKV.Audio)
	}
	if body.MakeMKV.Audio[0].Name != "English Dolby TrueHD/Atmos Audio" || body.MakeMKV.Audio[0].CodecLabel != "TrueHD.7.1.Atmos" || body.MakeMKV.Audio[0].Language != "eng" || !body.MakeMKV.Audio[0].Selected || !body.MakeMKV.Audio[0].Default {
		t.Fatalf("expected fused MakeMKV cache audio track fields, got %+v", body.MakeMKV.Audio[0])
	}
	if body.MakeMKV.Audio[1].Name != "普通话" || body.MakeMKV.Audio[1].CodecLabel != "DD.5.1" || body.MakeMKV.Audio[1].Language != "chi" || !body.MakeMKV.Audio[1].Selected || body.MakeMKV.Audio[1].Default {
		t.Fatalf("expected second fused MakeMKV cache audio track fields, got %+v", body.MakeMKV.Audio[1])
	}
	if len(body.MakeMKV.Subtitles) != 2 || body.MakeMKV.Subtitles[0].ID != "S1" || body.MakeMKV.Subtitles[0].SourceIndex != 0 {
		t.Fatalf("expected fused MakeMKV cache subtitle payload, got %+v", body.MakeMKV.Subtitles)
	}
	if body.MakeMKV.Subtitles[0].Name != "国配简体特效" || body.MakeMKV.Subtitles[0].Language != "chi" || !body.MakeMKV.Subtitles[0].Selected || !body.MakeMKV.Subtitles[0].Default {
		t.Fatalf("expected fused MakeMKV cache subtitle track fields, got %+v", body.MakeMKV.Subtitles[0])
	}
	if body.MakeMKV.Subtitles[1].Name != "简英特效" || body.MakeMKV.Subtitles[1].Language != "chi" || !body.MakeMKV.Subtitles[1].Selected || body.MakeMKV.Subtitles[1].Default {
		t.Fatalf("expected second fused MakeMKV cache subtitle track fields, got %+v", body.MakeMKV.Subtitles[1])
	}
}

func TestSourcesHandlerResolveReturnsMakeMKVAudioCodecLabel(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "Nightcrawler"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/custom/remux", stubSourceScanner{
		items: []media.SourceEntry{{ID: sourceID, Name: "Nightcrawler (2014)", Path: sourcePath, Type: media.SourceBDMV}},
	}, stubPlaylistInspector{
		result: MakeMKVInspection{Audio: []resolveTrack{{ID: "A1", Name: "English Atmos", Language: "eng", CodecLabel: "TrueHD.7.1.Atmos", Selected: true, Default: true, SourceIndex: 0}}},
	})

	reqBody := `{"sourceId":"Nightcrawler","bdinfo":{"playlistName":"00800.MPLS","discTitle":"Nightcrawler","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/Nightcrawler/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Audio []struct {
			Name       string `json:"name"`
			CodecLabel string `json:"codecLabel"`
		} `json:"audio"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Audio) != 1 || body.Audio[0].Name != "English Atmos" || body.Audio[0].CodecLabel != "TrueHD.7.1.Atmos" {
		t.Fatalf("expected MakeMKV audio track payload, got %+v", body.Audio)
	}
}

func TestSourcesHandlerResolveNormalizesFallbackAudioCodecLabelsInResponseAndCache(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "FallbackCodecDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/custom/remux", stubSourceScanner{
		items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}},
	}, stubPlaylistInspector{
		result: MakeMKVInspection{Audio: []resolveTrack{{ID: "A1", Name: "English DD+ Stereo", Language: "eng", Selected: true, Default: true, SourceIndex: 0}}},
	})

	reqBody := `{"sourceId":"FallbackCodecDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nAUDIO:\nCodec                           Language        Bitrate         Description\n-----                           --------        -------         -----------\nDolby Digital Plus Audio        English         768 kbps        stereo / 48 kHz / 768 kbps"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/FallbackCodecDisc/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Audio []struct {
			Name       string `json:"name"`
			CodecLabel string `json:"codecLabel"`
		} `json:"audio"`
		MakeMKV struct {
			Audio []struct {
				Name       string `json:"name"`
				CodecLabel string `json:"codecLabel"`
			} `json:"audio"`
		} `json:"makemkv"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Audio) != 1 || body.Audio[0].Name != "English Dolby Digital Plus Audio" || body.Audio[0].CodecLabel != "DDP.2.0" {
		t.Fatalf("expected normalized fallback audio codec label in resolve response, got %+v", body.Audio)
	}
	if len(body.MakeMKV.Audio) != 1 || body.MakeMKV.Audio[0].Name != "English Dolby Digital Plus Audio" || body.MakeMKV.Audio[0].CodecLabel != "DDP.2.0" {
		t.Fatalf("expected normalized fallback audio codec label in makemkv cache, got %+v", body.MakeMKV.Audio)
	}
}

func TestSourcesHandlerResolveReturnsMakeMKVSubtitleLanguages(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "Zootopia2"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}}}, stubPlaylistInspector{
		result: MakeMKVInspection{Subtitles: []resolveTrack{{ID: "S1", Name: "English", Language: "eng", Selected: true, Default: true, SourceIndex: 0}, {ID: "S2", Name: "简体中文特效", Language: "chi", Selected: true, Default: false, SourceIndex: 1}}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/sources/Zootopia2/resolve", strings.NewReader(`{"sourceId":"Zootopia2","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Subtitles []struct {
			Language string `json:"language"`
			Name     string `json:"name"`
		} `json:"subtitles"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Subtitles) != 2 || body.Subtitles[0].Language != "eng" || body.Subtitles[1].Language != "chi" {
		t.Fatalf("expected MakeMKV subtitle languages, got %+v", body.Subtitles)
	}
}

func TestSourcesHandlerResolveSerializesEmptyTracksAsJSONArrays(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "BrokenDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}}}, stubPlaylistInspector{result: MakeMKVInspection{}})
	req := httptest.NewRequest(http.MethodPost, "/api/sources/BrokenDisc/resolve", strings.NewReader(`{"sourceId":"BrokenDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if string(body["audio"]) != "[]" {
		t.Fatalf("expected top-level audio to serialize as [], got %s", string(body["audio"]))
	}
	if string(body["subtitles"]) != "[]" {
		t.Fatalf("expected top-level subtitles to serialize as [], got %s", string(body["subtitles"]))
	}

	var makemkv struct {
		Audio     json.RawMessage `json:"audio"`
		Subtitles json.RawMessage `json:"subtitles"`
	}
	if err := json.Unmarshal(body["makemkv"], &makemkv); err != nil {
		t.Fatalf("decode makemkv failed: %v", err)
	}
	if string(makemkv.Audio) != "[]" {
		t.Fatalf("expected makemkv audio to serialize as [], got %s", string(makemkv.Audio))
	}
	if string(makemkv.Subtitles) != "[]" {
		t.Fatalf("expected makemkv subtitles to serialize as [], got %s", string(makemkv.Subtitles))
	}
}

func TestSourcesHandlerResolvePreservesRequestLabelAlignmentAndForcedSubtitles(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "AlignedDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}}}, stubPlaylistInspector{
		result: MakeMKVInspection{
			Audio: []resolveTrack{
				{ID: "A1", Name: "Track 1", Language: "eng", Selected: true, Default: true, SourceIndex: 0},
				{ID: "A2", Name: "Track 2", Language: "jpn", Selected: true, Default: false, SourceIndex: 1},
				{ID: "A3", Name: "Track 3", Language: "eng", Selected: true, Default: false, SourceIndex: 2},
			},
			Subtitles: []resolveTrack{
				{ID: "S1", Name: "Subtitle 1", Language: "eng", Selected: true, Default: true, SourceIndex: 0, Forced: true},
				{ID: "S2", Name: "Subtitle 2", Language: "spa", Selected: true, Default: false, SourceIndex: 1, Forced: false},
				{ID: "S3", Name: "Subtitle 3", Language: "eng", Selected: true, Default: false, SourceIndex: 2, Forced: false},
			},
		},
	})
	reqBody := `{
		"sourceId":"AlignedDisc",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"audioLabels":["English Main","","Commentary"],
			"subtitleLabels":["Forced English","","Signs"],
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nAUDIO:\nEnglish         1500 kbps       Parsed Audio One\nJapanese        768 kbps        Parsed Audio Two\nEnglish         640 kbps        Parsed Audio Three\nSUBTITLES:\nEnglish                         Parsed Subtitle One\nSpanish                         Parsed Subtitle Two\nEnglish                         Parsed Subtitle Three"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/AlignedDisc/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Audio []struct {
			Name string `json:"name"`
		} `json:"audio"`
		Subtitles []struct {
			Name   string `json:"name"`
			Forced bool   `json:"forced"`
		} `json:"subtitles"`
		MakeMKV struct {
			Audio []struct {
				Name string `json:"name"`
			} `json:"audio"`
			Subtitles []struct {
				Name   string `json:"name"`
				Forced bool   `json:"forced"`
			} `json:"subtitles"`
		} `json:"makemkv"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got := []string{body.Audio[0].Name, body.Audio[1].Name, body.Audio[2].Name}; !equalStringSlices(got, []string{"English Main", "Track 2", "Commentary"}) {
		t.Fatalf("expected aligned audio labels, got %+v", got)
	}
	if got := []string{body.MakeMKV.Audio[0].Name, body.MakeMKV.Audio[1].Name, body.MakeMKV.Audio[2].Name}; !equalStringSlices(got, []string{"English Main", "Track 2", "Commentary"}) {
		t.Fatalf("expected aligned makemkv audio labels, got %+v", got)
	}
	if got := []string{body.Subtitles[0].Name, body.Subtitles[1].Name, body.Subtitles[2].Name}; !equalStringSlices(got, []string{"Forced English", "Subtitle 2", "Signs"}) {
		t.Fatalf("expected aligned subtitle labels, got %+v", got)
	}
	if !body.Subtitles[0].Forced {
		t.Fatalf("expected forced subtitle to stay forced in resolve response, got %+v", body.Subtitles[0])
	}
	if !body.MakeMKV.Subtitles[0].Forced {
		t.Fatalf("expected forced subtitle to stay forced in makemkv cache, got %+v", body.MakeMKV.Subtitles[0])
	}
	if got := []string{body.MakeMKV.Subtitles[0].Name, body.MakeMKV.Subtitles[1].Name, body.MakeMKV.Subtitles[2].Name}; !equalStringSlices(got, []string{"Forced English", "Subtitle 2", "Signs"}) {
		t.Fatalf("expected aligned makemkv subtitle labels, got %+v", got)
	}
}

func TestSourcesHandlerResolveReturnsMakeMKVTrackLanguages(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "FallbackDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}}}, stubPlaylistInspector{
		result: MakeMKVInspection{Audio: []resolveTrack{{ID: "A1", Language: "eng", Name: "Atmos", Selected: true, Default: true, SourceIndex: 0}, {ID: "A2", Language: "fre", Name: "DD+", Selected: true, Default: false, SourceIndex: 1}}, Subtitles: []resolveTrack{{ID: "S1", Name: "Signs & Songs", Language: "spa", Selected: true, Default: true, SourceIndex: 0}, {ID: "S2", Name: "简体中文特效", Language: "chi", Selected: true, Default: false, SourceIndex: 1}}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/sources/FallbackDisc/resolve", strings.NewReader(`{"sourceId":"FallbackDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Audio []struct{ Language string `json:"language"` } `json:"audio"`
		Subtitles []struct {
			Name     string `json:"name"`
			Language string `json:"language"`
		} `json:"subtitles"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Audio) != 2 || body.Audio[0].Language != "eng" || body.Audio[1].Language != "fre" {
		t.Fatalf("expected MakeMKV audio languages, got %+v", body.Audio)
	}
	if len(body.Subtitles) != 2 || body.Subtitles[0].Language != "spa" || body.Subtitles[1].Language != "chi" {
		t.Fatalf("expected MakeMKV subtitle languages, got %+v", body.Subtitles)
	}
}

func TestSourcesHandlerResolveReturnsMakeMKVAdditionalLanguages(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "EuropeanDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}}}, stubPlaylistInspector{
		result: MakeMKVInspection{Audio: []resolveTrack{{ID: "A1", Language: "ger", Name: "German", Selected: true, Default: true, SourceIndex: 0}, {ID: "A2", Language: "ita", Name: "Italian", Selected: true, Default: false, SourceIndex: 1}, {ID: "A3", Language: "kor", Name: "Korean", Selected: true, Default: false, SourceIndex: 2}}, Subtitles: []resolveTrack{{ID: "S1", Language: "ger", Name: "German", Selected: true, Default: true, SourceIndex: 0}, {ID: "S2", Language: "ita", Name: "Italian", Selected: true, Default: false, SourceIndex: 1}, {ID: "S3", Language: "dut", Name: "Dutch", Selected: true, Default: false, SourceIndex: 2}, {ID: "S4", Language: "kor", Name: "Korean", Selected: true, Default: false, SourceIndex: 3}}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/sources/EuropeanDisc/resolve", strings.NewReader(`{"sourceId":"EuropeanDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestSourcesHandlerResolveReturnsMakeMKVPortugueseAndRussian(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "LanguageDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, buildTestMPLS([]string{"00005"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{items: []media.SourceEntry{{ID: sourceID, Name: sourceID, Path: sourcePath, Type: media.SourceBDMV}}}, stubPlaylistInspector{
		result: MakeMKVInspection{Audio: []resolveTrack{{ID: "A1", Language: "por", Name: "Portuguese", Selected: true, Default: true, SourceIndex: 0}, {ID: "A2", Language: "rus", Name: "Russian", Selected: true, Default: false, SourceIndex: 1}}, Subtitles: []resolveTrack{{ID: "S1", Language: "por", Name: "Portuguese", Selected: true, Default: true, SourceIndex: 0}, {ID: "S2", Language: "rus", Name: "Russian", Selected: true, Default: false, SourceIndex: 1}}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/sources/LanguageDisc/resolve", strings.NewReader(`{"sourceId":"LanguageDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestSourcesHandlerResolveRejectsMissingPlaylist(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "NoPlaylistDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	if err := os.MkdirAll(filepath.Join(sourcePath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{
			{
				ID:   sourceID,
				Name: sourceID,
				Path: sourcePath,
				Type: media.SourceBDMV,
			},
		},
	}, stubPlaylistInspector{})

	reqBody := `{"sourceId":"NoPlaylistDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/NoPlaylistDisc/resolve", strings.NewReader(reqBody))
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestSourcesHandlerResolveFindsPlaylistCaseInsensitively(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "CaseDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00003.mpls")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{{
			ID:   sourceID,
			Name: sourceID,
			Path: sourcePath,
			Type: media.SourceBDMV,
		}},
	}, stubPlaylistInspector{
		result: MakeMKVInspection{},
	})

	reqBody := `{"sourceId":"CaseDisc","bdinfo":{"playlistName":"00003.MPLS","rawText":"PLAYLIST REPORT:\nName: 00003.MPLS"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/CaseDisc/resolve", strings.NewReader(reqBody))
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestSourcesHandlerResolveRejectsBDInfoPlaylistMismatch(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "MismatchDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	if err := os.MkdirAll(filepath.Join(sourcePath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS"), buildTestMPLS([]string{"00008"}), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{{
			ID:   sourceID,
			Name: sourceID,
			Path: sourcePath,
			Type: media.SourceBDMV,
		}},
	}, stubPlaylistInspector{})

	reqBody := `{"sourceId":"MismatchDisc","bdinfo":{"playlistName":"00999.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/MismatchDisc/resolve", strings.NewReader(reqBody))
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestSourcesHandlerResolveRejectsUnknownSource(t *testing.T) {
	h := NewSourcesHandler(t.TempDir(), "/remux", stubSourceScanner{
		items: []media.SourceEntry{
			{
				ID:   "Known",
				Name: "Known",
				Path: "/input/Known",
				Type: media.SourceBDMV,
			},
		},
	}, stubPlaylistInspector{})

	reqBody := `{"sourceId":"Missing","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/Missing/resolve", strings.NewReader(reqBody))
	req = withRouteParam(req, "id", "Missing")
	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestSourcesHandlerResolveReturnsDetailedBDInfoErrorAndLogsCause(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "BrokenDisc"
	sourcePath := filepath.Join(inputRoot, sourceID)
	if err := os.MkdirAll(filepath.Join(sourcePath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	var logBuffer bytes.Buffer
	restoreLogs := captureStandardLogger(&logBuffer)
	defer restoreLogs()

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{
			{
				ID:   sourceID,
				Name: sourceID,
				Path: sourcePath,
				Type: media.SourceBDMV,
			},
		},
	}, stubPlaylistInspector{})

	reqBody := `{"sourceId":"BrokenDisc","bdinfo":{"rawText":"not bdinfo"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/BrokenDisc/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)
	w := httptest.NewRecorder()

	h.Resolve(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, "no recognized bdinfo fields") {
		t.Fatalf("expected response body to include parse detail, got %q", body)
	}
	if logs := logBuffer.String(); !strings.Contains(logs, "no recognized bdinfo fields") || !strings.Contains(logs, "BrokenDisc") {
		t.Fatalf("expected logs to include source id and parse detail, got %q", logs)
	}
}

func withRouteParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func buildTestMPLS(clipNames []string) []byte {
	size := 32 + len(clipNames)*11
	data := make([]byte, size)
	copy(data[:4], []byte("MPLS"))
	copy(data[4:8], []byte("0300"))
	binary.BigEndian.PutUint32(data[8:12], 20)

	playlistStart := 20
	binary.BigEndian.PutUint16(data[playlistStart+6:playlistStart+8], uint16(len(clipNames)))
	cursor := playlistStart + 10
	for _, clip := range clipNames {
		binary.BigEndian.PutUint16(data[cursor:cursor+2], 9)
		copy(data[cursor+2:cursor+7], []byte(clip))
		copy(data[cursor+7:cursor+11], []byte("M2TS"))
		cursor += 11
	}
	return data
}


func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
