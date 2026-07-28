package worktree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/jjacoblee/arborist/internal/git"
)

// BranchRef identifies one local branch inside one base repository. The same
// branch name can live in several repositories, so a name alone is never enough
// to act on.
type BranchRef struct {
	Repo     string // base repository directory name, for display
	RepoPath string // base repository path, where the branch ref lives
	Branch   string
}

// BranchResult summarizes a branch-deletion run. Like RemoveResult, every
// outcome is recorded rather than returned as an error, so one refusal never
// stops the branches after it.
type BranchResult struct {
	Deleted []BranchRef
	Skipped []SkippedBranch
	Failed  []FailedBranch
}

// Write prints a human-readable branch-deletion summary.
func (r BranchResult) Write(w io.Writer) {
	for _, ref := range r.Deleted {
		fmt.Fprintf(w, "Deleted branch %s in %s\n", ref.Branch, ref.Repo)
	}
	for _, s := range r.Skipped {
		fmt.Fprintf(w, "Skipped branch %s in %s: %s\n", s.Ref.Branch, s.Ref.Repo, s.Reason)
	}
	for _, f := range r.Failed {
		fmt.Fprintf(w, "Failed  branch %s in %s: %v\n", f.Ref.Branch, f.Ref.Repo, f.Err)
	}
}

// SkippedBranch records a branch that was deliberately not deleted.
type SkippedBranch struct {
	Ref    BranchRef
	Reason string
}

// FailedBranch records a branch deletion that failed.
type FailedBranch struct {
	Ref BranchRef
	Err error
}

// DeleteBranches deletes each branch in refs from its own repository.
//
// A branch holding commits git cannot see anywhere else is never deleted
// silently: without force it is recorded as skipped, mirroring how Remove
// treats a dirty worktree. force maps to git's -D and deletes it anyway.
func (s Service) DeleteBranches(ctx context.Context, refs []BranchRef, force bool) BranchResult {
	var res BranchResult
	for _, ref := range refs {
		err := s.Git.DeleteBranch(ctx, ref.RepoPath, ref.Branch, force)
		switch {
		case err == nil:
			res.Deleted = append(res.Deleted, ref)
		case errors.Is(err, git.ErrBranchNotMerged):
			res.Skipped = append(res.Skipped, SkippedBranch{
				Ref:    ref,
				Reason: "has unmerged commits; rerun with --force to delete it",
			})
		default:
			res.Failed = append(res.Failed, FailedBranch{Ref: ref, Err: err})
		}
	}
	return res
}

// OrphanedBranchesAfter returns the branches of the just-removed worktrees that
// no longer have any worktree in their repository — the branches whose last
// checkout the removal took away.
//
// It asks git for the surviving worktrees rather than reasoning from the
// removal result, so a worktree Arborist does not manage (one created by hand
// outside the worktree root) still counts as a reason to keep the branch.
func (s Service) OrphanedBranchesAfter(ctx context.Context, removed []ManagedWorktree) ([]BranchRef, error) {
	var orphans []BranchRef
	seen := map[BranchRef]bool{}
	keep := map[string]map[string]bool{} // repoPath -> branch -> must not be deleted

	for _, wt := range removed {
		if wt.Branch == "" {
			continue // detached; there is no branch to delete
		}
		ref := BranchRef{Repo: wt.Repo, RepoPath: wt.RepoPath, Branch: wt.Branch}
		if seen[ref] {
			continue
		}
		seen[ref] = true

		if _, ok := keep[wt.RepoPath]; !ok {
			branches, err := s.branchesToKeep(ctx, wt.RepoPath)
			if err != nil {
				return nil, err
			}
			keep[wt.RepoPath] = branches
		}
		if !keep[wt.RepoPath][wt.Branch] {
			orphans = append(orphans, ref)
		}
	}
	return orphans, nil
}

// OrphanedBranches returns every local branch across the workspace's base
// repositories that no worktree has checked out.
func (s Service) OrphanedBranches(ctx context.Context) ([]BranchRef, error) {
	repos, err := s.baseRepos(ctx)
	if err != nil {
		return nil, err
	}

	var orphans []BranchRef
	for _, repoPath := range repos {
		refs, err := s.orphanedBranchesIn(ctx, repoPath)
		if err != nil {
			return nil, err
		}
		orphans = append(orphans, refs...)
	}
	return orphans, nil
}

// orphanedBranchesIn returns the local branches of one repository that no
// worktree has checked out.
func (s Service) orphanedBranchesIn(ctx context.Context, repoPath string) ([]BranchRef, error) {
	branches, err := s.Git.LocalBranches(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	if len(branches) == 0 {
		return nil, nil
	}
	keep, err := s.branchesToKeep(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	var orphans []BranchRef
	for _, branch := range branches {
		if keep[branch] {
			continue
		}
		orphans = append(orphans, BranchRef{
			Repo:     filepath.Base(repoPath),
			RepoPath: repoPath,
			Branch:   branch,
		})
	}
	return orphans, nil
}

// branchesToKeep returns the branches of repoPath that must never be offered
// for deletion: those a worktree still has checked out, plus the default
// branch.
//
// The default branch needs guarding on its own because a base clone left on a
// feature branch leaves it with no worktree at all — nothing else would stop it
// being offered up. It is read from the local origin/HEAD only, since this runs
// across every repository in the workspace and a scan should not reach the
// network; when origin/HEAD is not recorded the conventional names stand in, so
// the repository Arborist knows least about is guarded most.
func (s Service) branchesToKeep(ctx context.Context, repoPath string) (map[string]bool, error) {
	worktrees, err := s.Git.ListWorktrees(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	keep := make(map[string]bool, len(worktrees)+1)
	for _, wt := range worktrees {
		if wt.Branch != "" {
			keep[wt.Branch] = true
		}
	}
	if branch, err := s.Git.DefaultBranch(ctx, repoPath); err == nil && branch != "" {
		keep[branch] = true
	} else {
		keep["main"], keep["master"] = true, true
	}
	return keep, nil
}
