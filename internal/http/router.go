package httpapi

import (
	"net/http"

	"github.com/wangdazhuo/mkv-maker/internal/http/handlers"
)

func NewRouter(
	authHandler *handlers.AuthHandler,
	configHandler *handlers.ConfigHandler,
	requireAuth func(http.Handler) http.Handler,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", authHandler.Login)

	var configRoute http.Handler = http.HandlerFunc(configHandler.Get)
	if requireAuth != nil {
		configRoute = requireAuth(configRoute)
	}
	mux.Handle("/api/config", configRoute)

	sourcesHandler := handlers.NewSourcesHandler(configHandler.InputDir, nil)

	var sourceScanRoute http.Handler = http.HandlerFunc(sourcesHandler.Scan)
	if requireAuth != nil {
		sourceScanRoute = requireAuth(sourceScanRoute)
	}
	mux.Handle("/api/sources/scan", sourceScanRoute)

	var sourcesRoute http.Handler = http.HandlerFunc(sourcesHandler.List)
	if requireAuth != nil {
		sourcesRoute = requireAuth(sourcesRoute)
	}
	mux.Handle("/api/sources", sourcesRoute)

	return mux
}
