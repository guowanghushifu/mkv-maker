package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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
	JobsCreate     http.HandlerFunc
	JobsCurrent    http.HandlerFunc
	JobsCurrentLog http.HandlerFunc
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Post("/api/login", deps.Login)

	r.Group(func(protected chi.Router) {
		protected.Use(deps.RequireAuth)
		protected.Post("/api/logout", deps.Logout)
		protected.Get("/api/config", deps.ConfigGet)
		protected.Post("/api/sources/scan", deps.SourcesScan)
		protected.Get("/api/sources", deps.SourcesList)
		protected.Post("/api/sources/{id}/resolve", deps.SourcesResolve)
		protected.Post("/api/bdinfo/parse", deps.BDInfoParse)
		protected.Post("/api/drafts/preview-filename", deps.DraftsPreview)
		protected.Post("/api/jobs", deps.JobsCreate)
		protected.Get("/api/jobs/current", deps.JobsCurrent)
		protected.Get("/api/jobs/current/log", deps.JobsCurrentLog)
	})

	return r
}
