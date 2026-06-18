package git

import (
	"context"
	"fmt"
	"strings"
)

// Worktree is a single entry parsed from `git worktree list --porcelain`.
type Worktree struct {
	Path     string // absolute path of the worktree
	Head     string // commit SHA the worktree is checked out at
	Branch   string // short branch name; empty when detached
	Detached bool
}

// WorktreeAddOptions describes how to create a worktree.
type WorktreeAddOptions struct {
	// Path is the directory to create for the worktree.
	Path string
	// Branch is the branch to check out (and, when CreateNew is set, to create).
	Branch string
	// CreateNew creates the branch with -b. When false, Branch must already
	// exist locally.
	CreateNew bool
	// BaseRef is the starting point for a newly created branch (for example a
	// default branch like "main" or a remote ref like "origin/feature/x"). It is
	// only used when CreateNew is set, and may be empty to branch from HEAD.
	BaseRef string
}

// AddWorktree creates a worktree in the repository at repoPath.
//
// Arborist never passes --force, so git refuses to overwrite an existing path
// or reuse a branch already checked out elsewhere; those conditions surface as
// errors for the caller to report.
func (c Client) AddWorktree(ctx context.Context, repoPath string, opts WorktreeAddOptions) error {
	args := []string{"-C", repoPath, "worktree", "add"}
	if opts.CreateNew {
		args = append(args, "-b", opts.Branch, opts.Path)
		if opts.BaseRef != "" {
			args = append(args, opts.BaseRef)
		}
	} else {
		args = append(args, opts.Path, opts.Branch)
	}

	if _, err := c.runner.Run(ctx, "git", args...); err != nil {
		return fmt.Errorf("add worktree at %s: %w", opts.Path, err)
	}
	return nil
}

// IsDirty reports whether the worktree at path has uncommitted changes or
// untracked files. It uses `git status --porcelain`, which lists both modified
// and untracked entries, so any output means the worktree is not clean.
func (c Client) IsDirty(ctx context.Context, path string) (bool, error) {
	out, err := c.runner.Run(ctx, "git", "-C", path, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("check status for %s: %w", path, err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// RemoveWorktree removes worktreePath from the repository at repoPath. force
// maps to git's --force; without it, git refuses to remove a worktree that has
// changes, which is an extra safety backstop.
func (c Client) RemoveWorktree(ctx context.Context, repoPath, worktreePath string, force bool) error {
	args := []string{"-C", repoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	if _, err := c.runner.Run(ctx, "git", args...); err != nil {
		return fmt.Errorf("remove worktree %s: %w", worktreePath, err)
	}
	return nil
}

// PruneWorktrees removes stale administrative worktree entries for the
// repository at repoPath (worktrees whose directories no longer exist).
func (c Client) PruneWorktrees(ctx context.Context, repoPath string) error {
	if _, err := c.runner.Run(ctx, "git", "-C", repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("prune worktrees for %s: %w", repoPath, err)
	}
	return nil
}

// ListWorktrees returns the worktrees registered for the repository at repoPath.
func (c Client) ListWorktrees(ctx context.Context, repoPath string) ([]Worktree, error) {
	out, err := c.runner.Run(ctx, "git", "-C", repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees for %s: %w", repoPath, err)
	}
	return parseWorktreeList(out), nil
}

// parseWorktreeList parses the output of `git worktree list --porcelain`.
//
// Records are separated by blank lines, each beginning with a "worktree <path>"
// line followed by attributes such as "HEAD <sha>", "branch <ref>", or
// "detached".
func parseWorktreeList(data []byte) []Worktree {
	var (
		worktrees []Worktree
		cur       *Worktree
	)
	flush := func() {
		if cur != nil {
			worktrees = append(worktrees, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "detached":
			cur.Detached = true
		case line == "":
			flush()
		}
	}
	flush()
	return worktrees
}
