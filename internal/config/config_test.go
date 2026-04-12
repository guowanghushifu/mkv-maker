package config

import (
	"testing"
	"time"
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

func TestLoadDefaultsRemuxTempDir(t *testing.T) {
	t.Setenv("APP_PASSWORD", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.RemuxTempDir != "/remux_tmp" {
		t.Fatalf("expected default remux temp dir /remux_tmp, got %q", cfg.RemuxTempDir)
	}
}

func TestLoadAllowsOverridingRemuxTempDir(t *testing.T) {
	t.Setenv("APP_PASSWORD", "secret")
	t.Setenv("REMUX_TMP_DIR", "/custom/remux-tmp")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.RemuxTempDir != "/custom/remux-tmp" {
		t.Fatalf("expected overridden remux temp dir, got %q", cfg.RemuxTempDir)
	}
}

func TestLoadParsesMakeMKVExpireDate(t *testing.T) {
	t.Setenv("APP_PASSWORD", "secret")
	t.Setenv("MAKEMKV_EXPIRE_DATE", "2026-04-11")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MakeMKVExpireDate == nil {
		t.Fatal("expected valid MAKEMKV_EXPIRE_DATE to be parsed")
	}
	if got := cfg.MakeMKVExpireDate.Format("2006-01-02"); got != "2026-04-11" {
		t.Fatalf("expected parsed expire date 2026-04-11, got %s", got)
	}
}

func TestLoadIgnoresInvalidMakeMKVExpireDate(t *testing.T) {
	t.Setenv("APP_PASSWORD", "secret")
	t.Setenv("MAKEMKV_EXPIRE_DATE", "2026/04/11")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MakeMKVExpireDate != nil {
		t.Fatalf("expected invalid MAKEMKV_EXPIRE_DATE to be ignored, got %s", cfg.MakeMKVExpireDate.Format(time.RFC3339))
	}
}
