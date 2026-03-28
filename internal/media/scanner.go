package media

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type SourceType string

const (
	SourceISO  SourceType = "iso"
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
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var out []SourceEntry
	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		switch {
		case !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".iso"):
			out = append(out, SourceEntry{
				ID:         entry.Name(),
				Name:       strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
				Path:       fullPath,
				Type:       SourceISO,
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
		case entry.IsDir() && isBDMVRoot(fullPath):
			size, err := directorySize(fullPath)
			if err != nil {
				return nil, err
			}

			out = append(out, SourceEntry{
				ID:         entry.Name(),
				Name:       entry.Name(),
				Path:       fullPath,
				Type:       SourceBDMV,
				Size:       size,
				ModifiedAt: info.ModTime(),
			})
		}
	}

	slices.SortFunc(out, func(a, b SourceEntry) int {
		return strings.Compare(a.Name, b.Name)
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
