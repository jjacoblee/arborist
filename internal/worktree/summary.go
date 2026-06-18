package worktree

import (
	"fmt"
	"io"
	"strings"

	"github.com/jjacoblee/arborist/internal/github"
)

// BranchSource describes where a created worktree's branch came from.
type BranchSource string

const (
	// SourceLocalBranch: the branch already existed locally.
	SourceLocalBranch BranchSource = "existing local branch"
	// SourceRemoteTracking: a new local branch tracking origin/<branch>.
	SourceRemoteTracking BranchSource = "new tracking branch from origin"
	// SourceNewBranch: a new branch created from the repo's default branch.
	SourceNewBranch BranchSource = "new branch from default branch"
)

// CreatedWorktree records a successfully created worktree.
type CreatedWorktree struct {
	Repository  github.Repository
	Branch      string
	Path        string
	Source      BranchSource
	CopiedEnv   []string // env files copied into the worktree (if any)
	CopiedFiles []string // additional configured files copied (if any)
}

// SkippedWorktree records a repository left unchanged (for example because the
// worktree already exists), along with a human-readable reason.
type SkippedWorktree struct {
	Repository github.Repository
	Branch     string
	Path       string // existing path, when relevant
	Reason     string
}

// FailedWorktree records a repository that could not be processed.
type FailedWorktree struct {
	Repository github.Repository
	Branch     string
	Err        error
}

// CreateResult is the outcome of creating worktrees across several repositories.
type CreateResult struct {
	Created []CreatedWorktree
	Skipped []SkippedWorktree
	Failed  []FailedWorktree
}

// HasFailures reports whether any repository failed.
func (r CreateResult) HasFailures() bool {
	return len(r.Failed) > 0
}

// Write prints a human-readable summary, showing only the sections that have
// entries.
func (r CreateResult) Write(w io.Writer) {
	if len(r.Created) > 0 {
		fmt.Fprintf(w, "Created worktrees (%d)\n\n", len(r.Created))
		for _, c := range r.Created {
			fmt.Fprintf(w, "  %s\n", c.Repository.NameWithOwner)
			fmt.Fprintf(w, "    Branch: %s\n", c.Branch)
			fmt.Fprintf(w, "    Path:   %s\n", c.Path)
			fmt.Fprintf(w, "    Source: %s\n", c.Source)
			if len(c.CopiedEnv) > 0 {
				fmt.Fprintf(w, "    Env:    %s\n", strings.Join(c.CopiedEnv, ", "))
			}
			if len(c.CopiedFiles) > 0 {
				fmt.Fprintf(w, "    Files:  %s\n", strings.Join(c.CopiedFiles, ", "))
			}
		}
		fmt.Fprintln(w)
	}

	if len(r.Skipped) > 0 {
		fmt.Fprintf(w, "Skipped (%d)\n\n", len(r.Skipped))
		for _, s := range r.Skipped {
			fmt.Fprintf(w, "  %s: %s\n", s.Repository.NameWithOwner, s.Reason)
			if s.Path != "" {
				fmt.Fprintf(w, "    Path: %s\n", s.Path)
			}
		}
		fmt.Fprintln(w)
	}

	if len(r.Failed) > 0 {
		fmt.Fprintf(w, "Failed (%d)\n\n", len(r.Failed))
		for _, f := range r.Failed {
			fmt.Fprintf(w, "  %s: %v\n", f.Repository.NameWithOwner, f.Err)
		}
		fmt.Fprintln(w)
	}

	if len(r.Created)+len(r.Skipped)+len(r.Failed) == 0 {
		fmt.Fprintln(w, "Nothing to do.")
	}
}
