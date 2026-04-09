package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	AppPassword         string
	InputDir            string
	OutputDir           string
	DataDir             string
	ListenAddr          string
	RemuxTempDir        string
	SessionMaxAge       int
	SessionCookieSecure bool
}

func Load() (Config, error) {
	cfg := Config{
		AppPassword:         os.Getenv("APP_PASSWORD"),
		InputDir:            getenvDefault("BD_INPUT_DIR", "/bd_input"),
		OutputDir:           getenvDefault("REMUX_OUTPUT_DIR", "/remux"),
		DataDir:             getenvDefault("APP_DATA_DIR", "/app/data"),
		ListenAddr:          getenvDefault("LISTEN_ADDR", ":8080"),
		RemuxTempDir:        getenvDefault("REMUX_TMP_DIR", "/remux_tmp"),
		SessionMaxAge:       86400,
		SessionCookieSecure: getenvBoolDefault("SESSION_COOKIE_SECURE", false),
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

func getenvBoolDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "", "default":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
