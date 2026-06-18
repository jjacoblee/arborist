package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// IsRepo reports whether path is inside a Git working tree. It returns false for
// a missing path or a non-repository directory rather than an error.
func (c Client) IsRepo(ctx context.Context, path string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", path, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// Fetch updates all remotes and prunes deleted refs for the repository at
// repoPath.
func (c Client) Fetch(ctx context.Context, repoPath string) error {
	if _, err := c.runner.Run(ctx, "git", "-C", repoPath, "fetch", "--all", "--prune"); err != nil {
		return fmt.Errorf("fetch refs for %s: %w", repoPath, err)
	}
	return nil
}

// SetOriginHead asks git to determine origin's default branch and record it as
// refs/remotes/origin/HEAD. Call it before DefaultBranch when origin/HEAD may
// not be set yet (for example, right after cloning with some git versions).
func (c Client) SetOriginHead(ctx context.Context, repoPath string) error {
	if _, err := c.runner.Run(ctx, "git", "-C", repoPath, "remote", "set-head", "origin", "--auto"); err != nil {
		return fmt.Errorf("set origin HEAD for %s: %w", repoPath, err)
	}
	return nil
}

// MainRepoPath returns the path of the base (main) repository that the worktree
// at path belongs to. It resolves the common git directory and returns its
// parent, so for a linked worktree it yields the original clone's directory.
// This lets Arborist recover a worktree's repository without encoding it in the
// worktree's path.
func (c Client) MainRepoPath(ctx context.Context, path string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", path,
		"rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve main repo for %s: %w", path, err)
	}
	commonDir := strings.TrimSpace(string(out)) // <repo>/.git
	if commonDir == "" {
		return "", fmt.Errorf("resolve main repo for %s: empty git-common-dir", path)
	}
	return filepath.Dir(commonDir), nil
}

// DefaultBranch returns the repository's default branch (the short name that
// refs/remotes/origin/HEAD points to, e.g. "main"). It is not hardcoded.
func (c Client) DefaultBranch(ctx context.Context, repoPath string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", repoPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", fmt.Errorf("detect default branch for %s: %w", repoPath, err)
	}
	ref := strings.TrimSpace(string(out))
	ref = strings.TrimPrefix(ref, "origin/")
	if ref == "" {
		return "", errors.New("detect default branch: origin/HEAD is not set")
	}
	return ref, nil
}
