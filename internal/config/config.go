package config

import (
	"errors"
	"os"
)

type Config struct {
	AppPassword   string
	InputDir      string
	OutputDir     string
	DataDir       string
	ListenAddr    string
	SessionMaxAge int
}

func Load() (Config, error) {
	cfg := Config{
		AppPassword:   os.Getenv("APP_PASSWORD"),
		InputDir:      getenvDefault("BD_INPUT_DIR", "/bd_input"),
		OutputDir:     getenvDefault("REMUX_OUTPUT_DIR", "/remux"),
		DataDir:       getenvDefault("APP_DATA_DIR", "/app/data"),
		ListenAddr:    getenvDefault("LISTEN_ADDR", ":8080"),
		SessionMaxAge: 86400,
	}
	if cfg.AppPassword == "" {
		return Config{}, errors.New("APP_PASSWORD is required")
	}
	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
