package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/wangdazhuo/mkv-maker/internal/media"
)

type stubSourceScanner struct {
	items []media.SourceEntry
	err   error
}

func (s stubSourceScanner) Scan(root string) ([]media.SourceEntry, error) {
	return s.items, s.err
}

func TestSourcesHandlerListReturnsStructuredNotFoundError(t *testing.T) {
	h := NewSourcesHandler("/missing/input", "/remux", stubSourceScanner{
		err: &os.PathError{
			Op:   "readdir",
			Path: "/missing/input",
			Err:  os.ErrNotExist,
		},
	})

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
	})

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

func TestSourcesHandlerResolveBuildsFrontendDraftFromParsedBDInfo(t *testing.T) {
	inputRoot := t.TempDir()
	sourceID := "Nightcrawler"
	sourcePath := filepath.Join(inputRoot, sourceID)
	playlistPath := filepath.Join(sourcePath, "BDMV", "PLAYLIST", "00800.MPLS")
	if err := os.MkdirAll(filepath.Dir(playlistPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(playlistPath, []byte("playlist"), 0o644); err != nil {
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
		SourceID       string `json:"sourceId"`
		PlaylistName   string `json:"playlistName"`
		OutputDir      string `json:"outputDir"`
		Title          string `json:"title"`
		DVMergeEnabled bool   `json:"dvMergeEnabled"`
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
	if body.OutputDir != "/custom/remux" {
		t.Fatalf("expected outputDir /custom/remux, got %q", body.OutputDir)
	}
	if body.Title != "Nightcrawler" {
		t.Fatalf("expected title Nightcrawler, got %q", body.Title)
	}
	if !body.DVMergeEnabled {
		t.Fatal("expected dvMergeEnabled to be true")
	}
	if body.Video.Codec != "HEVC" || body.Video.Resolution != "2160p" || body.Video.HDRType != "HDR.DV" {
		t.Fatalf("unexpected video %+v", body.Video)
	}
	if len(body.Audio) != 3 || len(body.Subtitles) != 2 {
		t.Fatalf("expected 3 audio and 2 subtitles tracks, got %d and %d", len(body.Audio), len(body.Subtitles))
	}
	if !body.Audio[0].Default || !body.Audio[0].Selected {
		t.Fatalf("expected first audio to be default+selected: %+v", body.Audio[0])
	}
	if !body.Subtitles[0].Default || !body.Subtitles[0].Selected {
		t.Fatalf("expected first subtitle to be default+selected: %+v", body.Subtitles[0])
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
	})

	reqBody := `{"sourceId":"NoPlaylistDisc","bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/NoPlaylistDisc/resolve", strings.NewReader(reqBody))
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
	})

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
