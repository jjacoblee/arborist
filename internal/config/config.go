// Package config defines Arborist's per-owner workspace configuration: its
// schema, on-disk discovery, and JSON read/write.
//
// Arborist follows a git-like, workspace-rooted model. Each GitHub owner (user
// or organization) gets a self-contained workspace folder that holds the
// owner's cloned repositories, a sibling "worktrees" folder, and a hidden
// .arborist.json config file at its root. Commands locate that config by
// walking up from the current directory (see Find), exactly like git locates
// .git.
//
// The package is intentionally small: it reads and writes a single JSON file
// with the standard library rather than pulling in a configuration framework
// such as viper.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the user-editable per-owner workspace configuration, serialized as
// JSON to .arborist.json at the workspace root.
//
// The workspace root itself is the repository root: base clones live directly
// under it as <workspaceRoot>/<repo>. The root is implicit (it is the directory
// containing the config) and is therefore not stored in the file.
//
// Arborist clones repositories through the GitHub CLI (`gh repo clone`), so
// there is no clone-protocol setting: gh handles transport and authentication.
type Config struct {
	// Owner is the GitHub user or organization this workspace is scoped to.
	// Repository discovery lists this account's repositories. Required.
	Owner string `json:"owner"`
	// WorktreeRoot is where this workspace's worktrees live, laid out as
	// <worktreeRoot>/<repo>/<sanitized-branch>. It is optional: when empty it
	// defaults to <workspaceRoot>/worktrees (a sibling of the cloned repos).
	// A relative value is resolved against the workspace root; a leading "~"
	// is expanded by callers via the paths package.
	WorktreeRoot string `json:"worktreeRoot,omitempty"`
	// CopyEnvFiles, when true, copies top-level .env / .env.* files from a
	// repository's base clone into each newly created worktree (best effort).
	// Off by default, since those files commonly contain secrets.
	CopyEnvFiles bool `json:"copyEnvFiles"`
	// CopyFiles lists additional repo-relative files to copy from a repository's
	// base clone into each newly created worktree, for files that CopyEnvFiles
	// doesn't match (for example "secrets.env"). Paths are relative to the repo
	// root and may not escape it; missing files and symlinks are skipped. Copies
	// are written private (0600), since they often hold secrets.
	CopyFiles []string `json:"copyFiles,omitempty"`
	// Editor is the command used by `arb open` when no editor flag is given,
	// for example "cursor" or "code". May include arguments ("code --wait").
	// Optional; when empty `arb open` falls back to the $EDITOR environment
	// variable.
	Editor string `json:"editor,omitempty"`
	// Setup maps a repository name to the shell commands run in each newly
	// created worktree for that repo (for example "pnpm install", "uv sync").
	// The key "*" applies to any repo without an exact entry. These commands run
	// through a shell, so they are honored only from this trust-checked config —
	// never from a repository.
	Setup map[string][]string `json:"setup,omitempty"`
}

// Validate reports whether the configuration is usable, returning a descriptive
// error if not.
func (c Config) Validate() error {
	if c.Owner == "" {
		return errors.New("owner must not be empty (set it to your GitHub user or organization)")
	}
	if strings.ContainsAny(c.Owner, " \t/") {
		return fmt.Errorf("owner %q must be a single GitHub user or organization name (no spaces or '/')", c.Owner)
	}
	return nil
}

// SetupCommands returns the setup commands configured for repo: its exact entry
// when present, otherwise the "*" fallback, otherwise none.
func (c Config) SetupCommands(repo string) []string {
	if cmds, ok := c.Setup[repo]; ok {
		return cmds
	}
	return c.Setup["*"]
}

// ResolveWorktreeRoot returns the worktree root for a workspace rooted at
// workspaceRoot: the configured WorktreeRoot when set (a relative value is
// resolved against workspaceRoot), otherwise the default <workspaceRoot>/worktrees.
// The result may still contain a leading "~"; callers expand it with the paths
// package.
func (c Config) ResolveWorktreeRoot(workspaceRoot string) string {
	if c.WorktreeRoot == "" {
		return filepath.Join(workspaceRoot, "worktrees")
	}
	if strings.HasPrefix(c.WorktreeRoot, "~") || filepath.IsAbs(c.WorktreeRoot) {
		return c.WorktreeRoot
	}
	return filepath.Join(workspaceRoot, c.WorktreeRoot)
}

// ErrUntrustedConfig indicates a workspace config that fails Arborist's trust
// check: a symbolic link, a file not owned by the current user, or one writable
// by group or others. Such a file could be planted by another user and would
// otherwise be trusted to direct file operations and (via "editor") launch
// commands, so Arborist refuses to load it.
var ErrUntrustedConfig = errors.New("untrusted Arborist config")

// Load reads and validates the config at path. It first runs a trust check
// (ownership and permissions), since Arborist discovers configs automatically by
// walking up from the current directory. If the file does not exist, the
// returned error wraps fs.ErrNotExist, so callers can detect that case with
// errors.Is(err, os.ErrNotExist).
func Load(path string) (Config, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Config{}, fmt.Errorf("%w: %s is a symbolic link", ErrUntrustedConfig, path)
	}
	if err := checkTrust(path, info); err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return c, nil
}

// Save writes the config to path as pretty-printed JSON, creating parent
// directories as needed. It validates the config first and refuses to write an
// invalid one.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("refusing to save invalid config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	// 0o600: the config holds no secrets, but there's no reason for it to be
	// world-readable.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
