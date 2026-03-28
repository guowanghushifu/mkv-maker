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
	if err := os.WriteFile(filepath.Join(root, "DiscA", "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "DiscA", "BDMV", "PLAYLIST", "00000.mpls"), []byte("playlist"), 0o644); err != nil {
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

	var bdmvSize int64 = -1
	for _, item := range items {
		if item.Type == SourceBDMV {
			bdmvSize = item.Size
			break
		}
	}
	if bdmvSize == -1 {
		t.Fatal("expected BDMV source in scan results")
	}
	if bdmvSize <= 0 {
		t.Fatalf("expected BDMV source size to be > 0, got %d", bdmvSize)
	}
}
