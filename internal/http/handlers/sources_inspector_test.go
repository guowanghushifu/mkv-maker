package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	makemkv "github.com/guowanghushifu/mkv-maker/internal/media/makemkv"
)

func TestMakeMKVPlaylistInspectorAppliesDateOverrideToInfoCommand(t *testing.T) {
	root := t.TempDir()
	stubBinary := filepath.Join(root, "makemkvcon")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf 'TINFO:4,16,0,\"00801\"\\n'",
		"printf 'SINFO:4,0,1,6201,\"Audio\"\\n'",
		"printf 'SINFO:4,0,3,0,\"eng\"\\n'",
		"printf 'SINFO:4,0,5,0,\"A_TRUEHD\"\\n'",
		"printf 'SINFO:4,0,14,0,\"8\"\\n'",
		"printf 'SINFO:4,0,38,0,\"default\"\\n'",
		"printf 'SINFO:4,0,39,0,\"default\"\\n'",
	}, "\n")
	if err := os.WriteFile(stubBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	expireDate := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	override := makemkv.NewCommandDateOverride(&expireDate)
	override = override.WithNow(func() time.Time {
		return time.Date(2026, 4, 12, 10, 20, 30, 0, time.UTC)
	})
	override = override.WithSince(func(time.Time) time.Duration { return 1 * time.Second })
	override = override.WithAfter(func(time.Duration) <-chan time.Time { return make(chan time.Time) })

	var calls []time.Time
	override = override.WithSetSystemDate(func(_ context.Context, target time.Time) error {
		calls = append(calls, target)
		return nil
	})

	inspector := MakeMKVPlaylistInspector{
		Binary:       stubBinary,
		dateOverride: override,
	}
	inspection, err := inspector.Inspect(context.Background(), "/bd_input/Disc/BDMV", "00801.MPLS")
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if inspection.TitleID != 4 {
		t.Fatalf("expected title id 4, got %d", inspection.TitleID)
	}
	if len(calls) != 2 {
		t.Fatalf("expected rollback and restore around info command, got %d date changes", len(calls))
	}
}
