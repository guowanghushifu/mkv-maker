package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedRouteRejectsAnonymousRequests(t *testing.T) {
	router := NewRouter(testDependencies())
	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func testDependencies() Dependencies {
	noop := func(http.ResponseWriter, *http.Request) {}
	return Dependencies{
		RequireAuth: func(next http.Handler) http.Handler {
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
		JobsList:       noop,
		JobsGet:        noop,
		JobsLog:        noop,
	}
}
