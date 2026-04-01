package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedRouteRejectsAnonymousRequests(t *testing.T) {
	router := NewRouter(testDependencies())
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestProtectedLogoutRejectsAnonymousRequests(t *testing.T) {
	router := NewRouter(testDependencies())
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestProtectedPostJobsUsesCreateHandler(t *testing.T) {
	router := NewRouter(testDependenciesWithAuthBypass(func(deps *Dependencies) {
		deps.JobsCreate = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}
		deps.JobsCurrent = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", res.Code)
	}
}

func TestProtectedGetCurrentJobUsesCurrentHandler(t *testing.T) {
	router := NewRouter(testDependenciesWithAuthBypass(func(deps *Dependencies) {
		deps.JobsCurrent = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
		deps.JobsCreate = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestProtectedGetCurrentJobLogUsesCurrentLogHandler(t *testing.T) {
	router := NewRouter(testDependenciesWithAuthBypass(func(deps *Dependencies) {
		deps.JobsCurrentLog = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
		deps.JobsCurrent = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current/log", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func testDependencies() Dependencies {
	return testDependenciesWithAuthBypass(nil)
}

func testDependenciesWithAuthBypass(mutator func(*Dependencies)) Dependencies {
	noop := func(http.ResponseWriter, *http.Request) {}
	deps := Dependencies{
		RequireAuth: func(next http.Handler) http.Handler {
			if mutator != nil {
				return next
			}
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			})
		},
		Login:          noop,
		Logout:         noop,
		ConfigGet:      noop,
		SourcesScan:    noop,
		SourcesList:    noop,
		SourcesResolve: noop,
		BDInfoParse:    noop,
		DraftsPreview:  noop,
		JobsCreate:     noop,
		JobsCurrent:    noop,
		JobsCurrentLog: noop,
	}
	if mutator != nil {
		mutator(&deps)
	}
	return deps
}
