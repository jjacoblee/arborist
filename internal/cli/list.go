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
		dir  string
		full bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the worktrees Arborist manages",
		Args:  cobra.NoArgs,
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
			printWorktrees(cmd.OutOrStdout(), worktrees, svc.WorktreeRoot, full)
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "show absolute worktree paths instead of paths relative to the worktree root")
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
		}
		status := "clean"
		if wt.Dirty {
			status = "dirty"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", shortIDs[i], repo, branch, status, displayPath(wt.Path, worktreeRoot, full))
	}
	tw.Flush()
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
