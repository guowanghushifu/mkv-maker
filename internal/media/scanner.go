package media

import (
	"os"
	"path/filepath"
	"slices"
	"time"
)

type SourceType string

const (
	SourceBDMV SourceType = "bdmv"
)

type SourceEntry struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	Type       SourceType `json:"type"`
	Size       int64      `json:"size"`
	ModifiedAt time.Time  `json:"modifiedAt"`
}

type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) Scan(root string) ([]SourceEntry, error) {
	var out []SourceEntry
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if !isBDMVRoot(path) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		size, err := directorySize(path)
		if err != nil {
			return err
		}

		out = append(out, SourceEntry{
			ID:         filepath.Base(path),
			Name:       filepath.Base(path),
			Path:       path,
			Type:       SourceBDMV,
			Size:       size,
			ModifiedAt: info.ModTime(),
		})

		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(out, func(a, b SourceEntry) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		default:
			return 0
		}
	})

	return out, nil
}

func isBDMVRoot(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "BDMV", "PLAYLIST")); err == nil {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "BDMV", "index.bdmv"))
	return err == nil
}

func directorySize(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}
