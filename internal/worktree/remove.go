package worktree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SkippedRemoval records a worktree that was intentionally not removed.
type SkippedRemoval struct {
	Worktree ManagedWorktree
	Reason   string
}

// FailedRemoval records a worktree that could not be removed.
type FailedRemoval struct {
	Worktree ManagedWorktree
	Err      error
}

// RemoveResult summarizes a removal run.
type RemoveResult struct {
	Removed []ManagedWorktree
	Skipped []SkippedRemoval
	Failed  []FailedRemoval
}

// HasFailures reports whether any removal failed.
func (r RemoveResult) HasFailures() bool { return len(r.Failed) > 0 }

// Write prints a human-readable removal summary.
func (r RemoveResult) Write(w io.Writer) {
	for _, rm := range r.Removed {
		fmt.Fprintf(w, "Removed %s/%s (%s)\n  %s\n", rm.Owner, rm.Repo, branchLabel(rm.Branch), rm.Path)
	}
	for _, s := range r.Skipped {
		fmt.Fprintf(w, "Skipped %s/%s: %s\n  %s\n", s.Worktree.Owner, s.Worktree.Repo, s.Reason, s.Worktree.Path)
	}
	for _, f := range r.Failed {
		fmt.Fprintf(w, "Failed  %s/%s: %v\n  %s\n", f.Worktree.Owner, f.Worktree.Repo, f.Err, f.Worktree.Path)
	}
}

func branchLabel(b string) string {
	if b == "" {
		return "detached"
	}
	return b
}

// AmbiguousIDError is returned when a remove reference matches the id prefix of
// more than one worktree.
type AmbiguousIDError struct {
	Ref     string
	Matches []ManagedWorktree
}

func (e *AmbiguousIDError) Error() string {
	return fmt.Sprintf("worktree id %q matches %d worktrees", e.Ref, len(e.Matches))
}

// Find returns the managed worktrees that match ref. ref is treated first as a
// worktree id prefix: if it is hex and prefixes exactly one worktree's id, that
// single worktree is returned; if it prefixes several, an AmbiguousIDError is
// returned. Otherwise ref is matched as a branch name, returning every worktree
// on that branch (across repositories). It is shared by "remove" and "open".
func (s Service) Find(ctx context.Context, ref string) ([]ManagedWorktree, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	if isHexID(ref) {
		var byID []ManagedWorktree
		for _, wt := range all {
			if strings.HasPrefix(wt.ID, ref) {
				byID = append(byID, wt)
			}
		}
		switch {
		case len(byID) == 1:
			return byID, nil
		case len(byID) > 1:
			return nil, &AmbiguousIDError{Ref: ref, Matches: byID}
		}
		// No id matched; fall through to branch matching.
	}

	var byBranch []ManagedWorktree
	for _, wt := range all {
		if wt.Branch == ref {
			byBranch = append(byBranch, wt)
		}
	}
	return byBranch, nil
}

// Remove removes the given worktrees. A dirty worktree is removed only when
// force is true; otherwise it is reported as skipped (never deleted silently).
// After each successful removal the owning base repository is pruned.
func (s Service) Remove(ctx context.Context, targets []ManagedWorktree, force bool) RemoveResult {
	r := s.report()
	r.Start(len(targets))
	defer r.Stop()

	var res RemoveResult
	for _, wt := range targets {
		r.Step(wt.Owner + "/" + wt.Repo)
		s.removeOne(ctx, wt, force, &res)
		r.Done()
	}
	return res
}

// removeOne removes a single worktree, recording the outcome in res: a dirty
// worktree is skipped unless force is set, a git failure is recorded as failed,
// and a successful removal also prunes the owning base repository. Every outcome
// is a recorded result rather than a returned error, so the caller advances its
// progress indicator exactly once per item no matter which path is taken —
// including a skip, which as an inline `continue` used to bypass that update.
func (s Service) removeOne(ctx context.Context, wt ManagedWorktree, force bool, res *RemoveResult) {
	if wt.Dirty && !force {
		res.Skipped = append(res.Skipped, SkippedRemoval{
			Worktree: wt,
			Reason:   "has uncommitted changes; rerun with --force to remove",
		})
		return
	}
	if err := s.Git.RemoveWorktree(ctx, wt.RepoPath, wt.Path, force); err != nil {
		res.Failed = append(res.Failed, FailedRemoval{Worktree: wt, Err: err})
		return
	}
	// Best-effort cleanup of stale admin entries.
	_ = s.Git.PruneWorktrees(ctx, wt.RepoPath)
	res.Removed = append(res.Removed, wt)
}

// Prune runs `git worktree prune` on every base repository under the repo root,
// returning the repo paths that were pruned.
func (s Service) Prune(ctx context.Context) ([]string, error) {
	repos, err := s.baseRepos(ctx)
	if err != nil {
		return nil, err
	}

	r := s.report()
	r.Start(len(repos))
	defer r.Stop()

	pruned := make([]string, 0, len(repos))
	for _, repoPath := range repos {
		r.Step(filepath.Base(repoPath))
		if err := s.Git.PruneWorktrees(ctx, repoPath); err != nil {
			// Abort the whole prune; the deferred Stop clears the bar. The failed
			// repo is deliberately left un-Done — the operation didn't complete it.
			return pruned, err
		}
		r.Done()
		pruned = append(pruned, repoPath)
	}
	return pruned, nil
}

// baseRepos finds base clones directly under the workspace root, laid out as
// <workspaceRoot>/<repo>. The worktree root (a sibling "worktrees" folder by
// default) is skipped so its linked worktrees are never mistaken for base
// clones.
func (s Service) baseRepos(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.WorkspaceRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workspace root %s: %w", s.WorkspaceRoot, err)
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(s.WorkspaceRoot, entry.Name())
		if repoPath == s.WorktreeRoot {
			continue // the worktrees tree, not a base clone
		}
		if s.Git.IsRepo(ctx, repoPath) {
			repos = append(repos, repoPath)
		}
	}
	return repos, nil
}
