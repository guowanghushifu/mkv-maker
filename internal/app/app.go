package app

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/auth"
	"github.com/guowanghushifu/mkv-maker/internal/config"
	httpapi "github.com/guowanghushifu/mkv-maker/internal/http"
	"github.com/guowanghushifu/mkv-maker/internal/http/handlers"
	"github.com/guowanghushifu/mkv-maker/internal/http/middleware"
	"github.com/guowanghushifu/mkv-maker/internal/remux"
)

type App struct {
	Config       config.Config
	Handler      http.Handler
	logFile      *os.File
	remuxManager *remux.Manager
}

func New(cfg config.Config) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	logFile, err := initAppLogger(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	cookieAuth := auth.NewCookieAuth(cfg.AppPassword, time.Duration(cfg.SessionMaxAge)*time.Second)
	authHandler := &handlers.AuthHandler{
		AppPassword:   cfg.AppPassword,
		Auth:          cookieAuth,
		SessionMaxAge: cfg.SessionMaxAge,
		SessionSecure: cfg.SessionCookieSecure,
	}
	configHandler := &handlers.ConfigHandler{
		InputDir:  cfg.InputDir,
		OutputDir: cfg.OutputDir,
	}
	sourcesHandler := handlers.NewSourcesHandler(cfg.InputDir, cfg.OutputDir, nil, nil)
	bdinfoHandler := handlers.NewBDInfoHandler()
	draftsHandler := handlers.NewDraftsHandler()
	remuxManager := remux.NewManager(nil)
	jobsHandler := handlers.NewJobsHandler(remuxManager, cfg.InputDir, cfg.OutputDir)

	router := httpapi.NewRouter(httpapi.Dependencies{
		RequireAuth:    middleware.RequireAuth(cookieAuth),
		Login:          authHandler.Login,
		Logout:         authHandler.Logout,
		ConfigGet:      configHandler.Get,
		SourcesScan:    sourcesHandler.Scan,
		SourcesList:    sourcesHandler.List,
		SourcesResolve: sourcesHandler.Resolve,
		BDInfoParse:    bdinfoHandler.Parse,
		DraftsPreview:  draftsHandler.PreviewFilename,
		JobsCreate:     jobsHandler.Create,
		JobsCurrent:    jobsHandler.Current,
		JobsCurrentLog: jobsHandler.CurrentLog,
	})

	handler := withFrontend(router, filepath.Join("web", "dist"))

	return &App{
		Config:       cfg,
		Handler:      handler,
		logFile:      logFile,
		remuxManager: remuxManager,
	}, nil
}

func (a *App) Close() error {
	if a == nil {
		return nil
	}
	if a.remuxManager != nil {
		a.remuxManager.Close()
	}
	var err error
	if a.logFile != nil {
		err = a.logFile.Close()
	}
	return err
}

func initAppLogger(dataDir string) (*os.File, error) {
	logPath := filepath.Join(dataDir, "app.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	log.SetFlags(log.LstdFlags | log.LUTC | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	return logFile, nil
}

func withFrontend(apiHandler http.Handler, distDir string) http.Handler {
	if _, err := os.Stat(filepath.Join(distDir, "index.html")); err != nil {
		return apiHandler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		requestPath := path.Clean("/" + r.URL.Path)
		requestPath = strings.TrimPrefix(requestPath, "/")
		if requestPath != "" {
			candidate := filepath.Join(distDir, filepath.FromSlash(requestPath))
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				http.ServeFile(w, r, candidate)
				return
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				http.Error(w, "failed to read frontend asset", http.StatusInternalServerError)
				return
			}
		}

		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	})
}
