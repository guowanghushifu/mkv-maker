package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateNewOutputPathWithinRoot(root, candidate string) error {
	if !isPathWithinRoot(root, candidate) {
		return fmt.Errorf("output path is outside output root")
	}

	info, err := os.Lstat(candidate)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("output path must not be a symlink")
	case info.IsDir():
		return fmt.Errorf("output path must be a file")
	case info.Mode().IsRegular():
		return nil
	default:
		return fmt.Errorf("output path must be a regular file")
	}
}

func isPathWithinRoot(root, path string) bool {
	resolvedRoot, err := resolvePathForContainment(root, true)
	if err != nil {
		return false
	}
	resolvedPath, err := resolvePathForContainment(path, false)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func resolvePathForContainment(path string, mustExist bool) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if mustExist {
		return filepath.EvalSymlinks(absPath)
	}

	dir := absPath
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		dir = absPath
	} else {
		dir = filepath.Dir(absPath)
	}

	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	if dir == absPath {
		return resolvedDir, nil
	}
	return filepath.Join(resolvedDir, filepath.Base(absPath)), nil
}
