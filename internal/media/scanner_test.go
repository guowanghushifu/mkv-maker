package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerFindsISOAndBDMVFolders(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "movie.iso"), []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "DiscA", "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}
