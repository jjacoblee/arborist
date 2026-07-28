package git

import (
	"context"
	"errors"
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

// ErrBranchNotMerged indicates git declined to delete a branch because its
// commits are not reachable from its upstream or HEAD — the branch still holds
// work that would be lost. Callers match it with errors.Is to offer --force
// rather than reporting a generic failure.
var ErrBranchNotMerged = errors.New("branch has unmerged commits")

// DeleteBranch deletes the local branch in the repository at repoPath.
//
// force maps to git's -D, which deletes regardless of merge state. Without it
// the safe -d is used, and git refuses to delete a branch whose commits are not
// reachable from its upstream or HEAD; that refusal comes back wrapping
// ErrBranchNotMerged.
func (c Client) DeleteBranch(ctx context.Context, repoPath, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := c.runner.Run(ctx, "git", "-C", repoPath, "branch", flag, branch)
	if err == nil {
		return nil
	}
	// git has no distinct exit code for this; the wording is the only signal,
	// and the Runner folds stderr into the error text.
	if strings.Contains(err.Error(), "not fully merged") {
		return fmt.Errorf("delete branch %s in %s: %w", branch, repoPath, ErrBranchNotMerged)
	}
	return fmt.Errorf("delete branch %s in %s: %w", branch, repoPath, err)
}

// LocalBranches returns the short names of every local branch in the repository
// at repoPath.
//
// It uses for-each-ref rather than `git branch`, whose output is decorated for
// humans (a "*" on the current branch, a "+" on one checked out in a worktree)
// and would need stripping.
func (c Client) LocalBranches(ctx context.Context, repoPath string) ([]string, error) {
	out, err := c.runner.Run(ctx, "git", "-C", repoPath,
		"for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("list branches for %s: %w", repoPath, err)
	}

	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			branches = append(branches, name)
		}
	}
	return branches, nil
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
