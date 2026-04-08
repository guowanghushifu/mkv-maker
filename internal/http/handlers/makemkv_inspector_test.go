package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeMKVPlaylistInspectorSetsConfigHomeWithoutFaketime(t *testing.T) {
	t.Setenv("FAKETIME", "")

	root := t.TempDir()
	logPath := filepath.Join(root, "env.log")
	stubBinary := filepath.Join(root, "makemkvcon")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf 'HOME=%s\n' \"${HOME-<unset>}\" > \"" + logPath + "\"",
		"printf 'LD_PRELOAD=%s\n' \"${LD_PRELOAD-<unset>}\" >> \"" + logPath + "\"",
		"printf 'FAKETIME=%s\n' \"${FAKETIME-<unset>}\" >> \"" + logPath + "\"",
		"printf 'FAKETIME_DONT_FAKE_MONOTONIC=%s\n' \"${FAKETIME_DONT_FAKE_MONOTONIC-<unset>}\" >> \"" + logPath + "\"",
		"printf 'TINFO:4,16,0,\"00801\"\n'",
		"printf 'SINFO:4,0,1,6201,\"Audio\"\n'",
		"printf 'SINFO:4,0,3,0,\"eng\"\n'",
		"printf 'SINFO:4,0,4,0,\"English\"\n'",
		"printf 'SINFO:4,0,5,0,\"A_TRUEHD\"\n'",
		"printf 'SINFO:4,0,6,0,\"TrueHD\"\n'",
		"printf 'SINFO:4,0,14,0,\"8\"\n'",
		"printf 'SINFO:4,0,30,0,\"English Atmos\"\n'",
	}, "\n")
	if err := os.WriteFile(stubBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	inspector := MakeMKVPlaylistInspector{Binary: stubBinary}
	inspection, err := inspector.Inspect(context.Background(), "/bd_input/Disc/BDMV", "/bd_input/Disc/BDMV/PLAYLIST/00801.MPLS")
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if inspection.PlaylistName != "00801.MPLS" {
		t.Fatalf("expected playlist 00801.MPLS, got %+v", inspection)
	}

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	logged := string(contents)
	if !strings.Contains(logged, "HOME=/config\n") {
		t.Fatalf("expected HOME=/config, got %q", logged)
	}
	if !strings.Contains(logged, "LD_PRELOAD=<unset>\n") {
		t.Fatalf("expected LD_PRELOAD to stay unset, got %q", logged)
	}
	if !strings.Contains(logged, "FAKETIME=<unset>\n") {
		t.Fatalf("expected FAKETIME to stay unset, got %q", logged)
	}
	if !strings.Contains(logged, "FAKETIME_DONT_FAKE_MONOTONIC=<unset>\n") {
		t.Fatalf("expected FAKETIME_DONT_FAKE_MONOTONIC to stay unset, got %q", logged)
	}
}

func TestMakeMKVPlaylistInspectorAddsConfiguredFaketime(t *testing.T) {
	t.Setenv("FAKETIME", "@2026-04-10 00:00:00")

	root := t.TempDir()
	logPath := filepath.Join(root, "env.log")
	stubBinary := filepath.Join(root, "makemkvcon")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf 'HOME=%s\n' \"${HOME-<unset>}\" > \"" + logPath + "\"",
		"printf 'LD_PRELOAD=%s\n' \"${LD_PRELOAD-<unset>}\" >> \"" + logPath + "\"",
		"printf 'FAKETIME=%s\n' \"${FAKETIME-<unset>}\" >> \"" + logPath + "\"",
		"printf 'FAKETIME_DONT_FAKE_MONOTONIC=%s\n' \"${FAKETIME_DONT_FAKE_MONOTONIC-<unset>}\" >> \"" + logPath + "\"",
		"printf 'TINFO:4,16,0,\"00801\"\n'",
		"printf 'SINFO:4,0,1,6201,\"Audio\"\n'",
		"printf 'SINFO:4,0,3,0,\"eng\"\n'",
		"printf 'SINFO:4,0,4,0,\"English\"\n'",
		"printf 'SINFO:4,0,5,0,\"A_TRUEHD\"\n'",
		"printf 'SINFO:4,0,6,0,\"TrueHD\"\n'",
		"printf 'SINFO:4,0,14,0,\"8\"\n'",
		"printf 'SINFO:4,0,30,0,\"English Atmos\"\n'",
	}, "\n")
	if err := os.WriteFile(stubBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	inspector := MakeMKVPlaylistInspector{Binary: stubBinary}
	inspection, err := inspector.Inspect(context.Background(), "/bd_input/Disc/BDMV", "/bd_input/Disc/BDMV/PLAYLIST/00801.MPLS")
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if inspection.PlaylistName != "00801.MPLS" {
		t.Fatalf("expected playlist 00801.MPLS, got %+v", inspection)
	}

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	logged := string(contents)
	if !strings.Contains(logged, "HOME=/config\n") {
		t.Fatalf("expected HOME=/config, got %q", logged)
	}
	if !strings.Contains(logged, "LD_PRELOAD=/usr/local/lib/libfaketime.so.1\n") {
		t.Fatalf("expected LD_PRELOAD override, got %q", logged)
	}
	if !strings.Contains(logged, "FAKETIME=@2026-04-10 00:00:00\n") {
		t.Fatalf("expected FAKETIME override, got %q", logged)
	}
	if !strings.Contains(logged, "FAKETIME_DONT_FAKE_MONOTONIC=1\n") {
		t.Fatalf("expected FAKETIME_DONT_FAKE_MONOTONIC override, got %q", logged)
	}
}
