package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/wangdazhuo/mkv-maker/internal/store"
)

type stubJobsRepository struct {
	createFn func(input store.CreateJobInput) (store.APIJob, error)
	listFn   func() ([]store.APIJob, error)
	getFn    func(id string) (store.APIJob, error)
	logFn    func(id string) (string, error)
}

func (s stubJobsRepository) CreateQueuedJob(input store.CreateJobInput) (store.APIJob, error) {
	if s.createFn == nil {
		return store.APIJob{}, errors.New("create not configured")
	}
	return s.createFn(input)
}

func (s stubJobsRepository) ListJobs() ([]store.APIJob, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn()
}

func (s stubJobsRepository) GetJob(id string) (store.APIJob, error) {
	if s.getFn == nil {
		return store.APIJob{}, store.ErrJobNotFound
	}
	return s.getFn(id)
}

func (s stubJobsRepository) GetJobLog(id string) (string, error) {
	if s.logFn == nil {
		return "", store.ErrJobNotFound
	}
	return s.logFn(id)
}

func TestJobsHandlerCreateReturnsCreatedQueuedJob(t *testing.T) {
	h := NewJobsHandler(stubJobsRepository{
		createFn: func(input store.CreateJobInput) (store.APIJob, error) {
			if input.SourceName != "Nightcrawler Disc" {
				t.Fatalf("unexpected source name %q", input.SourceName)
			}
			if input.PlaylistName != "00800.MPLS" {
				t.Fatalf("unexpected playlist name %q", input.PlaylistName)
			}
			if input.OutputName != "Nightcrawler - 2160p.mkv" {
				t.Fatalf("unexpected output name %q", input.OutputName)
			}
			if !strings.Contains(input.PayloadJSON, `"outputFilename":"Nightcrawler - 2160p.mkv"`) {
				t.Fatalf("expected payload json to preserve request, got %q", input.PayloadJSON)
			}
			return store.APIJob{
				ID:           "job-123",
				SourceName:   input.SourceName,
				OutputName:   input.OutputName,
				OutputPath:   input.OutputPath,
				PlaylistName: input.PlaylistName,
				CreatedAt:    "2026-03-29T12:00:00Z",
				Status:       "queued",
			}, nil
		},
	})

	reqBody := `{
		"source":{"id":"Nightcrawler","name":"Nightcrawler Disc"},
		"bdinfo":{"playlistName":"00800.MPLS"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"00800.MPLS"},
		"outputFilename":"Nightcrawler - 2160p.mkv",
		"outputPath":"/remux/Nightcrawler - 2160p.mkv"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}
	var body store.APIJob
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.ID != "job-123" || body.Status != "queued" {
		t.Fatalf("unexpected response body %+v", body)
	}
}

func TestJobsHandlerListGetAndLog(t *testing.T) {
	h := NewJobsHandler(stubJobsRepository{
		listFn: func() ([]store.APIJob, error) {
			return []store.APIJob{
				{
					ID:           "job-123",
					SourceName:   "Nightcrawler Disc",
					OutputName:   "Nightcrawler - 2160p.mkv",
					OutputPath:   "/remux/Nightcrawler - 2160p.mkv",
					PlaylistName: "00800.MPLS",
					CreatedAt:    "2026-03-29T12:00:00Z",
					Status:       "queued",
				},
			}, nil
		},
		getFn: func(id string) (store.APIJob, error) {
			if id != "job-123" {
				return store.APIJob{}, store.ErrJobNotFound
			}
			return store.APIJob{
				ID:           "job-123",
				SourceName:   "Nightcrawler Disc",
				OutputName:   "Nightcrawler - 2160p.mkv",
				OutputPath:   "/remux/Nightcrawler - 2160p.mkv",
				PlaylistName: "00800.MPLS",
				CreatedAt:    "2026-03-29T12:00:00Z",
				Status:       "queued",
			}, nil
		},
		logFn: func(id string) (string, error) {
			if id != "job-123" {
				return "", store.ErrJobNotFound
			}
			return "[2026-03-29T12:00:00Z] queued", nil
		},
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	listW := httptest.NewRecorder()
	h.List(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("expected status %d for list, got %d", http.StatusOK, listW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/jobs/job-123", nil)
	getReq = withRouteParamJobs(getReq, "id", "job-123")
	getW := httptest.NewRecorder()
	h.Get(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected status %d for get, got %d", http.StatusOK, getW.Code)
	}

	logReq := httptest.NewRequest(http.MethodGet, "/api/jobs/job-123/log", nil)
	logReq = withRouteParamJobs(logReq, "id", "job-123")
	logW := httptest.NewRecorder()
	h.Log(logW, logReq)
	if logW.Code != http.StatusOK {
		t.Fatalf("expected status %d for log, got %d", http.StatusOK, logW.Code)
	}
	if strings.TrimSpace(logW.Body.String()) == "" {
		t.Fatal("expected non-empty log body")
	}
}

func TestJobsHandlerGetReturnsNotFound(t *testing.T) {
	h := NewJobsHandler(stubJobsRepository{
		getFn: func(id string) (store.APIJob, error) {
			return store.APIJob{}, store.ErrJobNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/missing", nil)
	req = withRouteParamJobs(req, "id", "missing")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func withRouteParamJobs(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
