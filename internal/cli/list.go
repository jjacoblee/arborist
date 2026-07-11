package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// newListCmd builds "arb list", which shows the worktrees Arborist manages.
func newListCmd(d deps) *cobra.Command {
	var (
		dir     string
		full    bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the worktrees Arborist manages",
		Long: `List the worktrees Arborist manages.

Each row leads with a short, stable id usable with "arb open" and "arb remove".

Use --json for machine-readable output (for scripts and coding agents): a JSON
array with each worktree's full id, repository, branch, status ("clean",
"dirty", or "broken"), and absolute path.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			g := git.New(d.runner)

			if err := requireGit(ctx, g); err != nil {
				return err
			}

			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}
			svc, err := newWorktreeService(g, github.New(d.runner), ws)
			if err != nil {
				return err
			}

			worktrees, err := svc.List(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				// Inspection errors are embedded per-entry (status "broken"
				// plus an "error" field), so no separate warnings are printed.
				return writeListJSON(cmd.OutOrStdout(), worktrees)
			}
			printWorktrees(cmd.OutOrStdout(), worktrees, svc.WorktreeRoot, full)
			printWorktreeWarnings(cmd.ErrOrStderr(), worktrees)
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "show absolute worktree paths instead of paths relative to the worktree root")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print worktrees as JSON (absolute paths; for scripts and coding agents)")
	addDirFlag(cmd, &dir)
	return cmd
}

// printWorktrees writes managed worktrees as an aligned table. Each row leads
// with a short, stable id (usable with "arb remove <id>"). Paths are shown
// relative to worktreeRoot unless full is set.
func printWorktrees(w io.Writer, worktrees []worktree.ManagedWorktree, worktreeRoot string, full bool) {
	if len(worktrees) == 0 {
		fmt.Fprintln(w, "No worktrees found.")
		return
	}

	ids := make([]string, len(worktrees))
	for i, wt := range worktrees {
		ids[i] = wt.ID
	}
	shortIDs := worktree.ShortenIDs(ids)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tREPOSITORY\tBRANCH\tSTATUS\tPATH")
	for i, wt := range worktrees {
		repo := wt.Repo
		if wt.Owner != "" {
			repo = wt.Owner + "/" + wt.Repo
		}
		branch := wt.Branch
		if branch == "" {
			branch = "(detached)"
			if wt.Err != nil {
				branch = "(unknown)"
			}
		}
		status := "clean"
		if wt.Dirty {
			status = "dirty"
		}
		if wt.Err != nil {
			status = "broken"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", shortIDs[i], repo, branch, status, displayPath(wt.Path, worktreeRoot, full))
	}
	tw.Flush()
}

// printWorktreeWarnings reports the errors behind any "broken" rows — worktrees
// git could not fully inspect (for example because of corrupt objects). They go
// to stderr so the table on stdout stays clean.
func printWorktreeWarnings(w io.Writer, worktrees []worktree.ManagedWorktree) {
	first := true
	for _, wt := range worktrees {
		if wt.Err == nil {
			continue
		}
		if first {
			fmt.Fprintln(w)
			first = false
		}
		fmt.Fprintf(w, "warning: %s: %v\n", wt.Path, wt.Err)
	}
}

// displayPath returns the worktree path relative to worktreeRoot, or the
// absolute path when full is set or the path is not under the root.
func displayPath(path, worktreeRoot string, full bool) string {
	if full {
		return path
	}
	rel, err := filepath.Rel(worktreeRoot, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}
