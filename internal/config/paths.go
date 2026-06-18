package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the per-workspace Arborist config file. It is hidden (dotfile)
// and lives at the root of an owner workspace.
const FileName = ".arborist.json"

// ErrNotWorkspace is returned by Find when no workspace config is found in the
// start directory or any of its ancestors.
var ErrNotWorkspace = errors.New("not inside an Arborist workspace")

// Workspace is a discovered owner workspace: its parsed config, the root
// directory that contains the config (also the repository root), and the
// absolute path of the config file.
type Workspace struct {
	Config Config
	// Root is the absolute path of the directory holding the config file. Base
	// clones live directly under it as <Root>/<repo>.
	Root string
	// Path is the absolute path of the config file (<Root>/FileName).
	Path string
}

// ConfigPath returns the config file path for a workspace rooted at dir.
func ConfigPath(dir string) string {
	return filepath.Join(dir, FileName)
}

// Find locates the owner workspace containing startDir by walking up from it
// until a FileName config is found, mirroring how git discovers .git. It
// returns the parsed Workspace, or ErrNotWorkspace if no config exists in
// startDir or any ancestor.
func Find(startDir string) (Workspace, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve start directory %q: %w", startDir, err)
	}

	for {
		path := ConfigPath(dir)
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			cfg, err := Load(path)
			if err != nil {
				return Workspace{}, err
			}
			return Workspace{Config: cfg, Root: dir, Path: path}, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Workspace{}, ErrNotWorkspace
		}
		dir = parent
	}
}
