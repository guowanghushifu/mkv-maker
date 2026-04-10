package media

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type SourceType string

const (
	SourceBDMV SourceType = "bdmv"
	SourceISO  SourceType = "iso"
)

type SourceEntry struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	Type       SourceType `json:"type"`
	Size       int64      `json:"size"`
	ModifiedAt time.Time  `json:"modifiedAt"`
}

type Scanner struct {
}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) Scan(root string) ([]SourceEntry, error) {
	var out []SourceEntry
	err := s.scanDir(root, root, &out)
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

func (s *Scanner) scanDir(root, dir string, out *[]SourceEntry) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if isBDMVRoot(path) {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				size, err := directorySize(path)
				if err != nil {
					return err
				}

				*out = append(*out, SourceEntry{
					ID:         filepath.Base(path),
					Name:       filepath.Base(path),
					Path:       path,
					Type:       SourceBDMV,
					Size:       size,
					ModifiedAt: info.ModTime(),
				})
				continue
			}
			if err := s.scanDir(root, path, out); err != nil {
				return err
			}
			continue
		}

		if !strings.EqualFold(filepath.Ext(path), ".iso") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		*out = append(*out, SourceEntry{
			ID:         stableISOID(root, path),
			Name:       strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Path:       path,
			Type:       SourceISO,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
		})
	}

	return nil
}

func stableISOID(root, path string) string {
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil {
			if rel != "." {
				return url.PathEscape(filepath.ToSlash(filepath.Clean(rel)))
			}
		}
	}
	return url.PathEscape(filepath.ToSlash(filepath.Base(path)))
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
