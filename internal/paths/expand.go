// Package paths handles filesystem path expansion, sanitization, and worktree
// path generation for Arborist. It deals only with path strings and never runs
// commands or touches the network.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Expand expands a leading "~" or "~/" in path to the user's home directory:
//
//	"~"      -> <home>
//	"~/code" -> <home>/code
//
// Paths that do not start with "~" are returned unchanged. The "~user" form
// (another user's home) is not supported and is returned unchanged, so callers
// should validate input separately if needed.
func Expand(path string) (string, error) {
	switch {
	case path == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		return home, nil
	case strings.HasPrefix(path, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		return filepath.Join(home, path[len("~/"):]), nil
	default:
		return path, nil
	}
}
