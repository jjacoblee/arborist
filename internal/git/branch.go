package git

import (
	"context"
	"fmt"
	"strings"
)

// LocalBranchExists reports whether a local branch named branch exists in the
// repository at repoPath.
//
// It relies on `git rev-parse --verify --quiet`, which exits non-zero when the
// ref is unknown, so any error from the command is treated as "does not exist".
func (c Client) LocalBranchExists(ctx context.Context, repoPath, branch string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", repoPath,
		"rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// RemoteBranchExists reports whether branch exists on the origin remote, based
// on the remote-tracking ref refs/remotes/origin/<branch>. Run Fetch first so
// the tracking refs are current.
func (c Client) RemoteBranchExists(ctx context.Context, repoPath, branch string) bool {
	_, err := c.runner.Run(ctx, "git", "-C", repoPath,
		"rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return err == nil
}

// CurrentBranch returns the checked-out branch of the worktree (or repo) at
// path. It returns an empty string with no error when HEAD is detached.
func (c Client) CurrentBranch(ctx context.Context, path string) (string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", path, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("determine current branch for %s: %w", path, err)
	}
	return strings.TrimSpace(string(out)), nil
}
