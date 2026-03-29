package app

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wangdazhuo/mkv-maker/internal/config"
	httpapi "github.com/wangdazhuo/mkv-maker/internal/http"
	"github.com/wangdazhuo/mkv-maker/internal/http/handlers"
	"github.com/wangdazhuo/mkv-maker/internal/http/middleware"
	"github.com/wangdazhuo/mkv-maker/internal/store"
)

type App struct {
	Config   config.Config
	DB       *sql.DB
	Sessions *store.SessionStore
	Handler  http.Handler
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
	sourcesHandler := handlers.NewSourcesHandler(cfg.InputDir, nil)
	bdinfoHandler := handlers.NewBDInfoHandler()
	draftsHandler := handlers.NewDraftsHandler()
	jobsHandler := handlers.NewJobsHandler()

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

	return &App{
		Config:   cfg,
		DB:       db,
		Sessions: sessionStore,
		Handler:  router,
	}, nil
}

func (a *App) Close() error {
	if a == nil || a.DB == nil {
		return nil
	}
	return a.DB.Close()
}
