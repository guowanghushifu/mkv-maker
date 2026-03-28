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
