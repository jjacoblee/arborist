package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/picker"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// newRemoveCmd builds "arb remove <id-or-branch>".
func newRemoveCmd(d deps) *cobra.Command {
	var (
		dir       string
		force     bool
		assumeYes bool
		jsonOut   bool
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
never removed unless you pass --force.

Use --json for machine-readable output (for scripts and coding agents). It is
non-interactive and requires --yes, so removal stays an explicit decision:

  arb remove feature/x --yes --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			g := git.New(d.runner)

			// --json is a non-interactive contract: it can never show the
			// confirmation prompt, and removal must never happen without an
			// explicit go-ahead — so --yes has to be spelled out.
			if jsonOut && !assumeYes {
				return errors.New("--json runs non-interactively; rerun with --yes to confirm removal")
			}

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
					// The match listing is a human aid; in --json mode stdout
					// stays reserved for the JSON document, and the error
					// (with its fix) reaches the caller on stderr.
					if !jsonOut {
						fmt.Fprintf(out, "Worktree id %q is ambiguous. Matches:\n\n", ref)
						ids := make([]string, len(amb.Matches))
						for i, wt := range amb.Matches {
							ids[i] = wt.ID
						}
						short := worktree.ShortenIDs(ids)
						for i, wt := range amb.Matches {
							fmt.Fprintf(out, "  %s  %s/%s  %s\n", short[i], wt.Owner, wt.Repo, wt.Branch)
						}
					}
					return fmt.Errorf("ambiguous worktree id %q; use more characters", ref)
				}
				return err
			}
			if len(matches) == 0 {
				if jsonOut {
					// An empty result document, so consumers can tell "nothing
					// matched" apart from an error.
					return writeRemoveJSON(out, worktree.RemoveResult{})
				}
				fmt.Fprintf(out, "No worktrees found for %q.\n", ref)
				return nil
			}

			if !jsonOut {
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
						if errors.Is(err, picker.ErrNotATerminal) {
							// A script or agent is driving and there is no
							// terminal for the prompt. Point at the explicit
							// non-interactive form.
							return errors.New("no terminal is available for the confirmation prompt; rerun with --yes to confirm removal")
						}
						return err
					}
					if !ok {
						fmt.Fprintln(out, "Aborted. Nothing was removed.")
						return nil
					}
				}
			}

			// Show a progress bar on stderr while worktrees are removed (each
			// removal also prunes its base repo, so this can take a moment). The
			// bar is inert off a terminal, keeping the stdout summary clean.
			svc.Progress = newStepsReporter(cmd.ErrOrStderr())
			result := svc.Remove(ctx, matches, force)
			if jsonOut {
				// Dirty worktrees the service refused to remove appear under
				// "skipped" with their reason, so the document is complete even
				// when nothing was removable.
				if err := writeRemoveJSON(out, result); err != nil {
					return err
				}
			} else {
				result.Write(out)
			}

			if result.HasFailures() {
				return fmt.Errorf("%d worktree(s) failed to remove", len(result.Failed))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "remove worktrees even if they have uncommitted changes")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the result as JSON (non-interactive; requires --yes)")
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
