package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/guowanghushifu/mkv-maker/internal/app"
	"github.com/guowanghushifu/mkv-maker/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("load config: %v", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Printf("initialize app: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := application.Close(); err != nil {
			log.Printf("close app: %v", err)
		}
	}()

	server := app.NewHTTPServer(cfg, application.Handler)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("listen on %s: %v", cfg.ListenAddr, err)
		os.Exit(1)
	}
}
