package httpapi

import (
	"net/http"
	"strings"
)

type Dependencies struct {
	RequireAuth    func(http.Handler) http.Handler
	Login          http.HandlerFunc
	Logout         http.HandlerFunc
	ConfigGet      http.HandlerFunc
	SourcesScan    http.HandlerFunc
	SourcesList    http.HandlerFunc
	SourcesResolve http.HandlerFunc
	BDInfoParse    http.HandlerFunc
	DraftsPreview  http.HandlerFunc
	JobsList       http.HandlerFunc
	JobsGet        http.HandlerFunc
	JobsLog        http.HandlerFunc
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/login", asHandler(deps.Login))
	mux.Handle("/api/logout", asHandler(deps.Logout))

	mux.Handle("/api/config", protect(deps.RequireAuth, deps.ConfigGet))
	mux.Handle("/api/sources/scan", protect(deps.RequireAuth, deps.SourcesScan))
	mux.Handle("/api/sources", protect(deps.RequireAuth, deps.SourcesList))
	mux.Handle("/api/sources/", protect(deps.RequireAuth, sourcesResolveRoute(deps.SourcesResolve)))
	mux.Handle("/api/bdinfo/parse", protect(deps.RequireAuth, deps.BDInfoParse))
	mux.Handle("/api/drafts/preview-filename", protect(deps.RequireAuth, deps.DraftsPreview))
	mux.Handle("/api/jobs", protect(deps.RequireAuth, deps.JobsList))
	mux.Handle("/api/jobs/", protect(deps.RequireAuth, jobsSubRoutes(deps.JobsGet, deps.JobsLog)))

	return mux
}

func protect(middleware func(http.Handler) http.Handler, next http.HandlerFunc) http.Handler {
	handler := asHandler(next)
	if middleware == nil {
		return handler
	}
	return middleware(handler)
}

func asHandler(next http.HandlerFunc) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		})
	}
	return next
}

func sourcesResolveRoute(resolve http.HandlerFunc) http.HandlerFunc {
	handler := asHandler(resolve)
	return func(w http.ResponseWriter, r *http.Request) {
		if !isSourcesResolvePath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	}
}

func jobsSubRoutes(getJob, jobLog http.HandlerFunc) http.HandlerFunc {
	getJobHandler := asHandler(getJob)
	jobLogHandler := asHandler(jobLog)
	return func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		parts := strings.Split(rest, "/")
		switch {
		case len(parts) == 1 && parts[0] != "":
			getJobHandler.ServeHTTP(w, r)
			return
		case len(parts) == 2 && parts[0] != "" && parts[1] == "log":
			jobLogHandler.ServeHTTP(w, r)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}
}

func isSourcesResolvePath(path string) bool {
	rest := strings.TrimPrefix(path, "/api/sources/")
	parts := strings.Split(rest, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] == "resolve"
}
