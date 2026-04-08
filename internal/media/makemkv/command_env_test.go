package makemkv

import (
	"slices"
	"strings"
	"testing"
)

func TestCommandEnvAlwaysSetsConfigHome(t *testing.T) {
	t.Setenv("FAKETIME", "")

	env := CommandEnv([]string{"PATH=/usr/bin"})

	if !slices.Contains(env, "HOME=/config") {
		t.Fatalf("expected HOME=/config in env, got %+v", env)
	}
	if slices.Contains(env, "LD_PRELOAD=/usr/local/lib/libfaketime.so.1") {
		t.Fatalf("expected LD_PRELOAD to be absent without FAKETIME, got %+v", env)
	}
	if slices.Contains(env, "FAKETIME_DONT_FAKE_MONOTONIC=1") {
		t.Fatalf("expected FAKETIME_DONT_FAKE_MONOTONIC to be absent without FAKETIME, got %+v", env)
	}
	if hasEnvKey(env, "FAKETIME") {
		t.Fatalf("expected FAKETIME override to be absent without FAKETIME, got %+v", env)
	}
}

func TestCommandEnvAddsFaketimeSettingsWhenConfigured(t *testing.T) {
	t.Setenv("FAKETIME", "@2026-04-10 00:00:00")

	env := CommandEnv([]string{"PATH=/usr/bin"})

	if !slices.Contains(env, "HOME=/config") {
		t.Fatalf("expected HOME=/config in env, got %+v", env)
	}
	if !slices.Contains(env, "LD_PRELOAD=/usr/local/lib/libfaketime.so.1") {
		t.Fatalf("expected LD_PRELOAD override, got %+v", env)
	}
	if !slices.Contains(env, "FAKETIME=@2026-04-10 00:00:00") {
		t.Fatalf("expected FAKETIME override, got %+v", env)
	}
	if !slices.Contains(env, "FAKETIME_DONT_FAKE_MONOTONIC=1") {
		t.Fatalf("expected FAKETIME_DONT_FAKE_MONOTONIC override, got %+v", env)
	}
}

func hasEnvKey(env []string, key string) bool {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}
