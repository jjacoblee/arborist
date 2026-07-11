package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ManagedWorktree describes one Arborist-managed worktree found under the
// configured worktree root.
type ManagedWorktree struct {
	ID       string // stable id derived from Path (see ID)
	Owner    string // GitHub owner (the workspace owner; may be empty)
	Repo     string // repository name
	Branch   string // checked-out branch ("" if detached)
	Path     string // absolute worktree path
	RepoPath string // base (main) repository path, used for removal/prune
	Dirty    bool   // has uncommitted changes or untracked files
	// Err records any problem git reported while inspecting this worktree
	// (for example corrupt objects). The worktree is still listed — with
	// possibly incomplete Branch/Dirty information — so a broken worktree can
	// be seen and removed instead of blocking the whole listing.
	Err error
}

// List enumerates the worktrees Arborist manages by scanning the worktree root
// for git worktree roots and asking git about each one. A worktree root is any
// directory containing a ".git" entry (a file for a linked worktree, a directory
// for a main checkout); the scan does not descend into one, so a worktree's
// own subdirectories are never mistaken for separate worktrees.
//
// The base repository for each worktree comes from git (its common dir), so the
// clones may live anywhere — they need not sit directly under the workspace
// root.
func (s Service) List(ctx context.Context) ([]ManagedWorktree, error) {
	roots, err := worktreeRootsUnder(s.WorktreeRoot)
	if err != nil {
		return nil, err
	}

	var result []ManagedWorktree
	for _, path := range roots {
		// Confirm with git and resolve the base repo; skip anything git does
		// not recognize as a worktree rather than failing the whole listing.
		repoPath, err := s.Git.MainRepoPath(ctx, path)
		if err != nil {
			continue
		}

		// A broken worktree (for example one with corrupt objects) must not
		// block the listing either: record the failure on the entry and keep
		// going, so the caller can report it and the user can still act on
		// the rest — including removing the broken worktree itself.
		wt := ManagedWorktree{
			ID:       ID(path),
			Owner:    s.Owner,
			Repo:     filepath.Base(repoPath),
			Path:     path,
			RepoPath: repoPath,
		}
		branch, branchErr := s.Git.CurrentBranch(ctx, path)
		dirty, dirtyErr := s.Git.IsDirty(ctx, path)
		wt.Branch = branch
		wt.Dirty = dirty
		wt.Err = errors.Join(branchErr, dirtyErr)

		result = append(result, wt)
	}
	return result, nil
}

// worktreeRootsUnder walks root and returns the directories that are git
// worktree roots — those containing a ".git" entry. It does not descend into a
// worktree once found, and skips symlinked directories. A missing root yields no
// results.
func worktreeRootsUnder(root string) ([]string, error) {
	var roots []string

	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("read %s: %w", dir, err)
		}

		for _, e := range entries {
			if e.Name() == ".git" {
				roots = append(roots, dir)
				return nil // a worktree root; do not descend into it
			}
		}
		for _, e := range entries {
			// e.IsDir() is false for symlinks, so symlinked dirs are skipped.
			if e.IsDir() {
				if err := walk(filepath.Join(dir, e.Name())); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(root); err != nil {
		return nil, err
	}
	return roots, nil
}
