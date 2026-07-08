package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// newRemoveCmd builds "arb remove <id-or-branch>".
func newRemoveCmd(d deps) *cobra.Command {
	var (
		dir       string
		force     bool
		assumeYes bool
	)

	cmd := &cobra.Command{
		Use:     "remove <id-or-branch>",
		Aliases: []string{"rm"},
		Short:   "Remove worktrees by id or branch",
		Long: `Remove worktrees, identified either by the short id shown in "arb list"
(removes that one worktree) or by a branch name (removes every worktree on that
branch across your repositories).

Arborist shows exactly which worktrees and paths will be removed and asks for
confirmation first. A worktree with uncommitted changes or untracked files is
never removed unless you pass --force.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
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

			matches, err := svc.Find(ctx, ref)
			if err != nil {
				var amb *worktree.AmbiguousIDError
				if errors.As(err, &amb) {
					fmt.Fprintf(out, "Worktree id %q is ambiguous. Matches:\n\n", ref)
					ids := make([]string, len(amb.Matches))
					for i, wt := range amb.Matches {
						ids[i] = wt.ID
					}
					short := worktree.ShortenIDs(ids)
					for i, wt := range amb.Matches {
						fmt.Fprintf(out, "  %s  %s/%s  %s\n", short[i], wt.Owner, wt.Repo, wt.Branch)
					}
					return fmt.Errorf("ambiguous worktree id %q; use more characters", ref)
				}
				return err
			}
			if len(matches) == 0 {
				fmt.Fprintf(out, "No worktrees found for %q.\n", ref)
				return nil
			}

			// Show exactly what is involved.
			ids := make([]string, len(matches))
			for i, wt := range matches {
				ids[i] = wt.ID
			}
			short := worktree.ShortenIDs(ids)
			fmt.Fprintf(out, "Worktrees matching %q:\n\n", ref)
			removable := 0
			for i, wt := range matches {
				marker := ""
				if wt.Dirty {
					marker = "  [dirty]"
				}
				if !wt.Dirty || force {
					removable++
				}
				fmt.Fprintf(out, "  %s  %s/%s%s\n    %s\n", short[i], wt.Owner, wt.Repo, marker, wt.Path)
			}
			fmt.Fprintln(out)

			if removable == 0 {
				fmt.Fprintln(out, "All matching worktrees have uncommitted changes.")
				fmt.Fprintln(out, "Rerun with --force to remove them.")
				return nil
			}
			if removable < len(matches) {
				fmt.Fprintf(out, "%d worktree(s) with changes will be skipped (use --force to include them).\n\n",
					len(matches)-removable)
			}

			if !assumeYes {
				ok, err := d.confirmer.Confirm(ctx, fmt.Sprintf("Remove %d worktree(s)?", removable))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(out, "Aborted. Nothing was removed.")
					return nil
				}
			}

			// Show a progress bar on stderr while worktrees are removed (each
			// removal also prunes its base repo, so this can take a moment). The
			// bar is inert off a terminal, keeping the stdout summary clean.
			svc.Progress = newStepsReporter(cmd.ErrOrStderr())
			result := svc.Remove(ctx, matches, force)
			result.Write(out)

			if result.HasFailures() {
				return fmt.Errorf("%d worktree(s) failed to remove", len(result.Failed))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "remove worktrees even if they have uncommitted changes")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	addDirFlag(cmd, &dir)
	return cmd
}

// newPruneCmd builds "arb prune".
func newPruneCmd(d deps) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Clean up stale worktree references",
		Long:  "Run git's worktree prune on each managed base repository to clear references to worktrees whose directories no longer exist.",
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

			svc.Progress = newStepsReporter(cmd.ErrOrStderr())
			pruned, err := svc.Prune(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d repositor%s.\n", len(pruned), plural(len(pruned)))
			return nil
		},
	}

	addDirFlag(cmd, &dir)
	return cmd
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
