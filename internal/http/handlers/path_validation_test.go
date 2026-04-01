package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateNewOutputPathWithinRootAllowsMissingLeafWithinRoot(t *testing.T) {
	outputRoot := t.TempDir()
	candidate := filepath.Join(outputRoot, "Disc.mkv")

	if err := validateNewOutputPathWithinRoot(outputRoot, candidate); err != nil {
		t.Fatalf("expected missing in-root output path to be allowed, got %v", err)
	}
}

func TestValidateNewOutputPathWithinRootRejectsExistingSymlinkLeaf(t *testing.T) {
	outputRoot := t.TempDir()
	outsideRoot := t.TempDir()
	linkPath := filepath.Join(outputRoot, "Disc.mkv")

	if err := os.Symlink(filepath.Join(outsideRoot, "escape.mkv"), linkPath); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	if err := validateNewOutputPathWithinRoot(outputRoot, linkPath); err == nil {
		t.Fatal("expected existing symlink output path to be rejected")
	}
}

func TestValidateNewOutputPathWithinRootRejectsExistingRegularFile(t *testing.T) {
	outputRoot := t.TempDir()
	candidate := filepath.Join(outputRoot, "Disc.mkv")

	if err := os.WriteFile(candidate, []byte("old"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := validateNewOutputPathWithinRoot(outputRoot, candidate); err == nil {
		t.Fatal("expected existing regular output file to be rejected")
	}
}
