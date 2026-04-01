package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerFindsBDMVFoldersOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "DiscA", "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "DiscA", "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "DiscA", "BDMV", "PLAYLIST", "00000.mpls"), []byte("playlist"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("", true)
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
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

func TestScannerFindsBDMVFoldersInNestedSubdirectories(t *testing.T) {
	root := t.TempDir()
	discPath := filepath.Join(root, "iso", "Furious Seven")
	if err := os.MkdirAll(filepath.Join(discPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(discPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(discPath, "BDMV", "PLAYLIST", "00801.mpls"), []byte("playlist"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("", true)
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Path != discPath {
		t.Fatalf("expected nested disc path %q, got %q", discPath, items[0].Path)
	}
	if items[0].Name != "Furious Seven" {
		t.Fatalf("expected nested disc name %q, got %q", "Furious Seven", items[0].Name)
	}
}

func TestScannerDoesNotReturnNestedBDMVInsideRecognizedRoot(t *testing.T) {
	root := t.TempDir()
	parentDiscPath := filepath.Join(root, "DiscA")
	if err := os.MkdirAll(filepath.Join(parentDiscPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentDiscPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentDiscPath, "BDMV", "PLAYLIST", "00000.mpls"), []byte("playlist"), 0o644); err != nil {
		t.Fatal(err)
	}

	nestedDiscPath := filepath.Join(parentDiscPath, "nested-copy")
	if err := os.MkdirAll(filepath.Join(nestedDiscPath, "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDiscPath, "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDiscPath, "BDMV", "PLAYLIST", "00001.mpls"), []byte("playlist"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("", true)
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only top-level recognized root, got %d items", len(items))
	}
	if items[0].Path != parentDiscPath {
		t.Fatalf("expected top-level disc path %q, got %q", parentDiscPath, items[0].Path)
	}
}

func TestScannerFindsISOFilesWhenEnabled(t *testing.T) {
	root := t.TempDir()
	isoPath := filepath.Join(root, "movies", "Nightcrawler.iso")
	if err := os.MkdirAll(filepath.Dir(isoPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(isoPath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(filepath.Join(root, "iso_auto_mount"), true)
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 1 || items[0].Type != SourceISO {
		t.Fatalf("expected one ISO item, got %+v", items)
	}
	if items[0].Name != "Nightcrawler" {
		t.Fatalf("expected ISO name Nightcrawler, got %q", items[0].Name)
	}
}

func TestScannerSkipsAutoMountRootAndOmitsISOWhenDisabled(t *testing.T) {
	root := t.TempDir()
	autoMountRoot := filepath.Join(root, "iso_auto_mount")
	if err := os.MkdirAll(filepath.Join(autoMountRoot, "old-disc", "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(autoMountRoot, "old-disc", "BDMV", "index.bdmv"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "DiscA.iso"), []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(autoMountRoot, false)
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items when ISO scanning disabled and auto-mount root skipped, got %+v", items)
	}
}
