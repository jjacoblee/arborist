package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isEnvFile reports whether name is a top-level env file: ".env" or ".env.*"
// (for example ".env.local", ".env.production").
func isEnvFile(name string) bool {
	return name == ".env" || strings.HasPrefix(name, ".env.")
}

// copyEnvFiles copies the top-level .env / .env.* files from srcDir into dstDir
// and returns the names copied. It is best effort and intentionally narrow:
//
//   - only the top level is scanned (no recursion),
//   - only regular files are copied (symlinks and directories are skipped, so
//     a symlinked .env cannot redirect the copy outside the repo),
//   - copies are written with private (0600) permissions, since env files
//     commonly hold secrets,
//   - a missing srcDir is treated as "nothing to copy", not an error.
func copyEnvFiles(srcDir, dstDir string) ([]string, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read env source %s: %w", srcDir, err)
	}

	var copied []string
	for _, entry := range entries {
		name := entry.Name()
		if !isEnvFile(name) || !entry.Type().IsRegular() {
			continue
		}
		if err := copyFile(filepath.Join(srcDir, name), filepath.Join(dstDir, name)); err != nil {
			return copied, fmt.Errorf("copy %s: %w", name, err)
		}
		copied = append(copied, name)
	}
	return copied, nil
}

// copyExtraFiles copies the explicitly listed repo-relative files from srcDir
// into dstDir, returning the paths copied. It is best effort and safe:
//
//   - each path is confined to the repo (absolute paths and paths escaping via
//     ".." are refused),
//   - missing files are skipped (not an error),
//   - only regular files are copied (symlinks and directories are skipped, so a
//     symlinked entry cannot redirect the copy outside the repo),
//   - copies are written private (0600), since these files often hold secrets.
func copyExtraFiles(srcDir, dstDir string, rels []string) ([]string, error) {
	var copied []string
	for _, rel := range rels {
		clean := filepath.Clean(rel)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			continue // refuse to read or write outside the repo / worktree
		}

		src := filepath.Join(srcDir, clean)
		info, err := os.Lstat(src)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // not present in the base clone; nothing to copy
			}
			return copied, fmt.Errorf("stat %s: %w", clean, err)
		}
		if !info.Mode().IsRegular() {
			continue // skip symlinks and directories
		}

		if err := copyFile(src, filepath.Join(dstDir, clean)); err != nil {
			return copied, fmt.Errorf("copy %s: %w", clean, err)
		}
		copied = append(copied, clean)
	}
	return copied, nil
}

// copyFile copies a single regular file's contents to dst with 0600
// permissions, creating dst's parent directory if needed.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
