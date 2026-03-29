package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/wangdazhuo/mkv-maker/internal/config"
	httpapi "github.com/wangdazhuo/mkv-maker/internal/http"
	"github.com/wangdazhuo/mkv-maker/internal/http/handlers"
	"github.com/wangdazhuo/mkv-maker/internal/http/middleware"
	"github.com/wangdazhuo/mkv-maker/internal/queue"
	"github.com/wangdazhuo/mkv-maker/internal/store"
)

type App struct {
	Config      config.Config
	DB          *sql.DB
	Sessions    *store.SessionStore
	Handler     http.Handler
	queueCancel context.CancelFunc
	queueDone   chan struct{}
}

func New(cfg config.Config) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(cfg.DataDir, "app.db")
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := store.Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	sessionStore := store.NewSessionStore(db, time.Duration(cfg.SessionMaxAge)*time.Second)
	authHandler := &handlers.AuthHandler{
		AppPassword:   cfg.AppPassword,
		Sessions:      sessionStore,
		SessionMaxAge: cfg.SessionMaxAge,
	}
	configHandler := &handlers.ConfigHandler{
		InputDir:  cfg.InputDir,
		OutputDir: cfg.OutputDir,
	}
	sourcesHandler := handlers.NewSourcesHandler(cfg.InputDir, cfg.OutputDir, nil)
	bdinfoHandler := handlers.NewBDInfoHandler()
	draftsHandler := handlers.NewDraftsHandler()
	jobStore := store.NewSQLiteJobStore(db, filepath.Join(cfg.DataDir, "logs"))
	jobsHandler := handlers.NewJobsHandler(jobStore)

	router := httpapi.NewRouter(httpapi.Dependencies{
		RequireAuth:    middleware.RequireAuth(sessionStore),
		Login:          authHandler.Login,
		Logout:         authHandler.Logout,
		ConfigGet:      configHandler.Get,
		SourcesScan:    sourcesHandler.Scan,
		SourcesList:    sourcesHandler.List,
		SourcesResolve: sourcesHandler.Resolve,
		BDInfoParse:    bdinfoHandler.Parse,
		DraftsPreview:  draftsHandler.PreviewFilename,
		JobsList:       jobsHandler.List,
		JobsCreate:     jobsHandler.Create,
		JobsGet:        jobsHandler.Get,
		JobsLog:        jobsHandler.Log,
	})

	handler := withFrontend(router, filepath.Join("web", "dist"))

	queueCtx, queueCancel := context.WithCancel(context.Background())
	queueDone := make(chan struct{})
	go func() {
		manager := queue.NewManager(jobStore, queue.NewExecutor(jobStore, nil))
		manager.Run(queueCtx)
		close(queueDone)
	}()

	return &App{
		Config:      cfg,
		DB:          db,
		Sessions:    sessionStore,
		Handler:     handler,
		queueCancel: queueCancel,
		queueDone:   queueDone,
	}, nil
}

func (a *App) Close() error {
	if a == nil {
		return nil
	}
	if a.queueCancel != nil {
		a.queueCancel()
	}
	if a.queueDone != nil {
		<-a.queueDone
	}
	if a.DB == nil {
		return nil
	}
	return a.DB.Close()
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
