package handlers

import (
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
	result PlaylistInspection
	err    error
	path   *string
}

func (s stubPlaylistInspector) Inspect(playlistPath string) (PlaylistInspection, error) {
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
		result: PlaylistInspection{
			AudioTrackIDs:    []string{"2"},
			SubtitleTrackIDs: []string{"9"},
		},
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
		result: PlaylistInspection{
			AudioTrackIDs:    []string{"2", "5", "7"},
			SubtitleTrackIDs: []string{"9", "10"},
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
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nLength: 1:57:49.645 (h:m:s.ms)\nVIDEO:\nMPEG-H HEVC Video       57999 kbps          2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020\n* MPEG-H HEVC Video     2100 kbps           1080p / 23.976 fps / 16:9 / Main 10 / Dolby Vision Enhancement Layer"
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
			ID       string `json:"id"`
			Name     string `json:"name"`
			Selected bool   `json:"selected"`
			Default  bool   `json:"default"`
		} `json:"audio"`
		Subtitles []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Selected bool   `json:"selected"`
			Default  bool   `json:"default"`
		} `json:"subtitles"`
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
	if body.Audio[0].ID != "2" || body.Audio[1].ID != "5" || body.Audio[2].ID != "7" {
		t.Fatalf("expected real audio track ids from inspector, got %+v", body.Audio)
	}
	if !body.Subtitles[0].Default || !body.Subtitles[0].Selected {
		t.Fatalf("expected first subtitle to be default+selected: %+v", body.Subtitles[0])
	}
	if body.Subtitles[0].ID != "9" || body.Subtitles[1].ID != "10" {
		t.Fatalf("expected real subtitle track ids from inspector, got %+v", body.Subtitles)
	}
}

func TestSourcesHandlerResolvePreservesAudioCodecLabelWhenDisplayLabelIsDescriptive(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "Nightcrawler"
	sourcePath := filepath.Join(inputRoot, sourceID)
	if err := os.MkdirAll(filepath.Join(sourcePath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourcePath, "BDMV", "STREAM"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.WriteFile(playlistPath, []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	streamPath := filepath.Join(sourcePath, "BDMV", "STREAM", "00005.m2ts")
	if err := os.WriteFile(streamPath, []byte("stream"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

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
		result: PlaylistInspection{
			AudioTrackIDs: []string{"2"},
		},
	})

	reqBody := `{
		"sourceId":"Nightcrawler",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"discTitle":"Nightcrawler",
			"audioLabels":["英文次世代全景声"],
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nLength: 1:57:49.645 (h:m:s.ms)\nVIDEO:\nMPEG-H HEVC Video       57999 kbps          2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020\nAUDIO:\nDolby TrueHD/Atmos Audio        English         7.1 / 48 kHz / 3984 kbps / 24-bit"
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
		Audio []struct {
			Name       string `json:"name"`
			CodecLabel string `json:"codecLabel"`
			Default    bool   `json:"default"`
		} `json:"audio"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Audio) != 1 {
		t.Fatalf("expected 1 audio track, got %d", len(body.Audio))
	}
	if body.Audio[0].Name != "英文次世代全景声" {
		t.Fatalf("expected display label to stay descriptive, got %+v", body.Audio[0])
	}
	if body.Audio[0].CodecLabel != "TrueHD.7.1.Atmos" {
		t.Fatalf("expected codec label TrueHD.7.1.Atmos, got %+v", body.Audio[0])
	}
}

func TestSourcesHandlerResolveUsesSubtitleLanguageColumnInsteadOfGuessingFromLabel(t *testing.T) {
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

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{{
			ID:   sourceID,
			Name: sourceID,
			Path: sourcePath,
			Type: media.SourceBDMV,
		}},
	}, stubPlaylistInspector{
		result: PlaylistInspection{
			SubtitleTrackIDs: []string{"11", "12", "13", "14", "15"},
		},
	})

	reqBody := `{
		"sourceId":"Zootopia2",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nSUBTITLES:\nCodec                           Language        Bitrate         Description\n-----                           --------        -------         -----------\nPresentation Graphics           English         54.085 kbps\nPresentation Graphics           French          43.255 kbps\nPresentation Graphics           Spanish         42.957 kbps\nPresentation Graphics           Japanese        30.071 kbps\nPresentation Graphics           Chinese         31.415 kbps                    简体中文特效"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/Zootopia2/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)

	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Subtitles []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Language string `json:"language"`
		} `json:"subtitles"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Subtitles) != 5 {
		t.Fatalf("expected 5 subtitle tracks, got %d", len(body.Subtitles))
	}
	gotLanguages := []string{
		body.Subtitles[0].Language,
		body.Subtitles[1].Language,
		body.Subtitles[2].Language,
		body.Subtitles[3].Language,
		body.Subtitles[4].Language,
	}
	wantLanguages := []string{"eng", "fre", "spa", "jpn", "chi"}
	if !equalStringSlices(gotLanguages, wantLanguages) {
		t.Fatalf("expected subtitle languages %+v, got %+v", wantLanguages, gotLanguages)
	}
	if body.Subtitles[0].Name != "English" || body.Subtitles[4].Name != "简体中文特效" {
		t.Fatalf("expected display labels to remain subtitle-facing, got %+v", body.Subtitles)
	}
}

func TestSourcesHandlerResolveRejectsIncompleteSubtitleTrackIDs(t *testing.T) {
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

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{{
			ID:   sourceID,
			Name: sourceID,
			Path: sourcePath,
			Type: media.SourceBDMV,
		}},
	}, stubPlaylistInspector{
		result: PlaylistInspection{
			SubtitleTrackIDs: []string{"11"},
		},
	})

	reqBody := `{
		"sourceId":"BrokenDisc",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS\nSUBTITLES:\nCodec                           Language        Bitrate         Description\n-----                           --------        -------         -----------\nPresentation Graphics           English         54.085 kbps\nPresentation Graphics           Chinese         31.415 kbps                    简体中文特效"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/BrokenDisc/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)

	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "subtitle track ids") {
		t.Fatalf("expected subtitle track id error, got %q", w.Body.String())
	}
}

func TestSourcesHandlerResolveFallsBackToMKVMergeLanguagesWhenBDInfoLanguagesAreUnavailable(t *testing.T) {
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

	h := NewSourcesHandler(inputRoot, "/remux", stubSourceScanner{
		items: []media.SourceEntry{{
			ID:   sourceID,
			Name: sourceID,
			Path: sourcePath,
			Type: media.SourceBDMV,
		}},
	}, stubPlaylistInspector{
		result: PlaylistInspection{
			AudioTrackIDs:     []string{"2", "4"},
			AudioLanguages:    []string{"eng", "fre"},
			SubtitleTrackIDs:  []string{"11", "12"},
			SubtitleLanguages: []string{"spa", "chi"},
		},
	})

	reqBody := `{
		"sourceId":"FallbackDisc",
		"bdinfo":{
			"playlistName":"00800.MPLS",
			"audioLabels":["Dolby TrueHD/Atmos Audio","Dolby Digital Plus Audio"],
			"subtitleLabels":["Signs & Songs","简体中文特效"],
			"rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/FallbackDisc/resolve", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = withRouteParam(req, "id", sourceID)

	w := httptest.NewRecorder()
	h.Resolve(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		Audio []struct {
			Language string `json:"language"`
		} `json:"audio"`
		Subtitles []struct {
			Name     string `json:"name"`
			Language string `json:"language"`
		} `json:"subtitles"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	gotAudioLanguages := []string{body.Audio[0].Language, body.Audio[1].Language}
	if !equalStringSlices(gotAudioLanguages, []string{"eng", "fre"}) {
		t.Fatalf("expected audio language fallback [eng fre], got %+v", gotAudioLanguages)
	}
	gotSubtitleLanguages := []string{body.Subtitles[0].Language, body.Subtitles[1].Language}
	if !equalStringSlices(gotSubtitleLanguages, []string{"spa", "chi"}) {
		t.Fatalf("expected subtitle language fallback [spa chi], got %+v", gotSubtitleLanguages)
	}
	if body.Subtitles[0].Name != "Signs & Songs" || body.Subtitles[1].Name != "简体中文特效" {
		t.Fatalf("expected subtitle labels to remain unchanged, got %+v", body.Subtitles)
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
		result: PlaylistInspection{},
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

func TestCollectTrackIDsCollapsesTrueHDCoreMultiplexedAudioTracks(t *testing.T) {
	payload := mkvmergeIdentifyPayload{
		Tracks: []mkvmergeTrack{
			{ID: 0, Type: "video"},
			{
				ID:         1,
				Type:       "audio",
				Codec:      "TrueHD Atmos",
				Properties: mkvmergeTrackProperties{AudioChannels: 8, Number: 4352, StreamID: 4352, MultiplexedTracks: []int{1, 2}},
			},
			{
				ID:         2,
				Type:       "audio",
				Codec:      "AC-3",
				Properties: mkvmergeTrackProperties{AudioChannels: 6, Number: 4352, StreamID: 4352, MultiplexedTracks: []int{1, 2}},
			},
			{
				ID:         3,
				Type:       "audio",
				Codec:      "DTS-HD Master Audio",
				Properties: mkvmergeTrackProperties{AudioChannels: 6, Number: 4353, StreamID: 4353},
			},
			{
				ID:    4,
				Type:  "subtitles",
				Codec: "HDMV PGS",
			},
		},
	}

	audioIDs, subtitleIDs := collectTrackIDs(payload)
	if !equalStringSlices(audioIDs, []string{"1", "3"}) {
		t.Fatalf("expected multiplexed audio ids to collapse to [1 3], got %+v", audioIDs)
	}
	if !equalStringSlices(subtitleIDs, []string{"4"}) {
		t.Fatalf("expected subtitle ids [4], got %+v", subtitleIDs)
	}
}

func TestCollectTrackIDsPrefersHigherChannelCountWithinMultiplexedGroup(t *testing.T) {
	payload := mkvmergeIdentifyPayload{
		Tracks: []mkvmergeTrack{
			{
				ID:         1,
				Type:       "audio",
				Codec:      "AC-3",
				Properties: mkvmergeTrackProperties{AudioChannels: 2, Number: 5000, StreamID: 5000, MultiplexedTracks: []int{1, 2}},
			},
			{
				ID:         2,
				Type:       "audio",
				Codec:      "DTS-HD Master Audio",
				Properties: mkvmergeTrackProperties{AudioChannels: 6, Number: 5000, StreamID: 5000, MultiplexedTracks: []int{1, 2}},
			},
		},
	}

	audioIDs, _ := collectTrackIDs(payload)
	if !equalStringSlices(audioIDs, []string{"2"}) {
		t.Fatalf("expected higher channel track id [2], got %+v", audioIDs)
	}
}

func TestCollectTrackIDsKeepsFirstTrackWhenChannelCountMatches(t *testing.T) {
	payload := mkvmergeIdentifyPayload{
		Tracks: []mkvmergeTrack{
			{
				ID:         7,
				Type:       "audio",
				Codec:      "DTS-HD Master Audio",
				Properties: mkvmergeTrackProperties{AudioChannels: 6, Number: 6000, StreamID: 6000, MultiplexedTracks: []int{7, 8}},
			},
			{
				ID:         8,
				Type:       "audio",
				Codec:      "AC-3",
				Properties: mkvmergeTrackProperties{AudioChannels: 6, Number: 6000, StreamID: 6000, MultiplexedTracks: []int{7, 8}},
			},
		},
	}

	audioIDs, _ := collectTrackIDs(payload)
	if !equalStringSlices(audioIDs, []string{"7"}) {
		t.Fatalf("expected first track id [7] when channel counts match, got %+v", audioIDs)
	}
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
