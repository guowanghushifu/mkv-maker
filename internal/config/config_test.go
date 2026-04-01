package config

import (
	"testing"
)

func TestLoadRejectsEmptyPassword(t *testing.T) {
	t.Setenv("APP_PASSWORD", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected empty APP_PASSWORD to fail")
	}
}

func TestLoadDefaultsSessionCookieSecureToFalse(t *testing.T) {
	t.Setenv("APP_PASSWORD", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SessionCookieSecure {
		t.Fatal("expected secure session cookies to be disabled by default for HTTP compatibility")
	}
}

func TestLoadAllowsEnablingSecureCookieForHTTPS(t *testing.T) {
	t.Setenv("APP_PASSWORD", "secret")
	t.Setenv("SESSION_COOKIE_SECURE", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.SessionCookieSecure {
		t.Fatal("expected SESSION_COOKIE_SECURE=1 to enable the secure cookie flag")
	}
}
