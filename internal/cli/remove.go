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

// newRemoveCmd builds "arb remove [id-or-branch]".
func newRemoveCmd(d deps) *cobra.Command {
	var (
		dir          string
		force        bool
		assumeYes    bool
		deleteBranch bool
	)

	cmd := &cobra.Command{
		Use:     "remove [id-or-branch]",
		Aliases: []string{"rm"},
		Short:   "Remove worktrees by id or branch, or pick them from a list",
		Long: `Remove worktrees, identified either by the short id shown in "arb list"
(removes that one worktree) or by a branch name (removes every worktree on that
branch across your repositories).

With no argument, a searchable multi-select picker opens listing every worktree
that is safe to remove; with --force it lists all of them, marking those with
uncommitted changes or unpushed commits.

Arborist shows exactly which worktrees and paths will be removed and asks for
confirmation first. A worktree with uncommitted changes or untracked files is
never removed unless you pass --force.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if len(args) == 0 {
				return removeFromPicker(cmd, d, svc, force, assumeYes, deleteBranch)
			}
			ref := args[0]

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

			return executeRemoval(cmd, d, svc, matches, force, assumeYes, deleteBranch)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "remove worktrees even if they have uncommitted changes, and delete branches with unmerged commits")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&deleteBranch, "delete-branch", false, "also delete the local branch when its last worktree is removed")
	addDirFlag(cmd, &dir)
	return cmd
}

// removeFromPicker runs the interactive flow for a bare "arb remove": it offers
// the worktrees that may be removed, then removes whichever ones are chosen.
//
// Without force the picker only lists clean worktrees, so nothing risky is even
// selectable; with force it lists everything, and the entries carry their own
// dirty/unpushed markers so a risky choice is a deliberate one.
func removeFromPicker(cmd *cobra.Command, d deps, svc worktree.Service,
	force, assumeYes, deleteBranch bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	candidates, err := svc.RemovalCandidates(ctx)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		fmt.Fprintln(out, "No worktrees found.")
		return nil
	}

	var (
		choices []picker.WorktreeChoice
		byID    = make(map[string]worktree.ManagedWorktree, len(candidates))
	)
	for _, c := range candidates {
		if c.Worktree.Dirty && !force {
			continue
		}
		choices = append(choices, picker.WorktreeChoice{
			ID:       c.Worktree.ID,
			Repo:     c.Worktree.Repo,
			Branch:   c.Worktree.Branch,
			Dirty:    c.Worktree.Dirty,
			Unpushed: c.Unpushed,
		})
		byID[c.Worktree.ID] = c.Worktree
	}
	if len(choices) == 0 {
		fmt.Fprintln(out, "No worktrees are safe to remove (all have uncommitted changes).")
		fmt.Fprintln(out, "Rerun with --force to include them.")
		return nil
	}

	selected, err := d.worktreeSelector.SelectWorktrees(ctx, choices)
	if err != nil {
		if errors.Is(err, picker.ErrCanceled) {
			fmt.Fprintln(out, "Aborted. Nothing was removed.")
			return nil
		}
		return err
	}

	targets := make([]worktree.ManagedWorktree, 0, len(selected))
	for _, id := range selected {
		if wt, ok := byID[id]; ok {
			targets = append(targets, wt)
		}
	}
	if len(targets) == 0 {
		fmt.Fprintln(out, "Nothing selected. No worktrees were removed.")
		return nil
	}

	// The picker is a selection, not an agreement to delete: show the paths and
	// ask, exactly as the by-name flow does.
	fmt.Fprintf(out, "\nSelected %d worktree(s):\n\n", len(targets))
	for _, wt := range targets {
		marker := ""
		if wt.Dirty {
			marker = "  [dirty]"
		}
		fmt.Fprintf(out, "  %s/%s%s\n    %s\n", wt.Owner, wt.Repo, marker, wt.Path)
	}
	fmt.Fprintln(out)

	return executeRemoval(cmd, d, svc, targets, force, assumeYes, deleteBranch)
}

// executeRemoval confirms the removal of targets, carries it out, and then
// offers to delete any branch the removal left with no worktree. It is the tail
// shared by the by-name and picker-driven flows.
func executeRemoval(cmd *cobra.Command, d deps, svc worktree.Service,
	targets []worktree.ManagedWorktree, force, assumeYes, deleteBranch bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	if !assumeYes {
		ok, err := d.confirmer.Confirm(ctx,
			fmt.Sprintf("Remove %d worktree(s)?", countRemovable(targets, force)))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "Aborted. Nothing was removed.")
			return nil
		}
	}

	// Show a progress bar on stderr while worktrees are removed (each removal
	// also prunes its base repo, so this can take a moment). The bar is inert
	// off a terminal, keeping the stdout summary clean.
	svc.Progress = newStepsReporter(cmd.ErrOrStderr())
	result := svc.Remove(ctx, targets, force)
	result.Write(out)

	// Removing the checkout leaves the branch ref behind; offer to take it too,
	// so a stale branch can't be re-checked-out by the next "arb new".
	branchFailed, err := cleanupBranches(cmd, d, svc, result.Removed, deleteBranch, force, assumeYes)
	if err != nil {
		return err
	}

	switch {
	case result.HasFailures():
		return fmt.Errorf("%d worktree(s) failed to remove", len(result.Failed))
	case branchFailed > 0:
		return fmt.Errorf("%d branch(es) failed to delete", branchFailed)
	}
	return nil
}

// countRemovable reports how many targets will actually be removed; a dirty
// worktree is only removable with force.
func countRemovable(targets []worktree.ManagedWorktree, force bool) int {
	n := 0
	for _, wt := range targets {
		if !wt.Dirty || force {
			n++
		}
	}
	return n
}

// cleanupBranches deletes the local branches whose last worktree the removal
// just took away, returning how many deletions failed.
//
// deleteBranch (the --delete-branch flag) opts in outright. Without it the user
// is asked, unless --yes made the run non-interactive — an unattended run must
// not quietly delete a branch nobody asked it to touch. force maps through to
// git's -D so a branch with unmerged commits can be deleted deliberately.
func cleanupBranches(cmd *cobra.Command, d deps, svc worktree.Service, removed []worktree.ManagedWorktree,
	deleteBranch, force, assumeYes bool) (int, error) {
	if len(removed) == 0 {
		return 0, nil
	}
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	orphans, err := svc.OrphanedBranchesAfter(ctx, removed)
	if err != nil {
		return 0, err
	}
	if len(orphans) == 0 {
		return 0, nil
	}

	if !deleteBranch {
		if assumeYes {
			return 0, nil
		}
		fmt.Fprintln(out)
		for _, ref := range orphans {
			fmt.Fprintf(out, "Branch %s in %s now has no worktrees.\n", ref.Branch, ref.Repo)
		}
		ok, err := d.confirmer.Confirm(ctx, branchPrompt(len(orphans)))
		if err != nil {
			return 0, err
		}
		if !ok {
			fmt.Fprintln(out, "Kept the local branch(es).")
			return 0, nil
		}
	}

	res := svc.DeleteBranches(ctx, orphans, force)
	res.Write(out)
	return len(res.Failed), nil
}

func branchPrompt(n int) string {
	if n == 1 {
		return "Delete the local branch too?"
	}
	return fmt.Sprintf("Delete these %d local branches too?", n)
}

// newPruneCmd builds "arb prune".
func newPruneCmd(d deps) *cobra.Command {
	var (
		dir            string
		force          bool
		assumeYes      bool
		deleteBranches bool
	)

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Clean up stale worktree references",
		Long: `Run git's worktree prune on each managed base repository to clear references to
worktrees whose directories no longer exist.

Any local branch left with no worktree is then listed, and Arborist offers to
delete those branches. The repository's default branch is never listed, and a
branch holding unmerged commits is never deleted without --force.`,
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

			svc.Progress = newStepsReporter(cmd.ErrOrStderr())
			pruned, err := svc.Prune(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d repositor%s.\n", len(pruned), plural(len(pruned)))

			failed, err := pruneBranches(cmd, d, svc, deleteBranches, force, assumeYes)
			if err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d branch(es) failed to delete", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "delete orphaned branches even if they have unmerged commits")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt (orphaned branches are only reported)")
	cmd.Flags().BoolVar(&deleteBranches, "delete-branches", false, "delete every orphaned branch without asking")
	addDirFlag(cmd, &dir)
	return cmd
}

// pruneBranches lists the local branches left with no worktree and, with the
// user's agreement, deletes them. It returns how many deletions failed.
//
// Reporting is unconditional — knowing which branches are stale is useful on
// its own. Deleting is not: --delete-branches opts in outright, otherwise the
// user is asked, and --yes (an unattended run) reports without deleting.
func pruneBranches(cmd *cobra.Command, d deps, svc worktree.Service,
	deleteBranches, force, assumeYes bool) (int, error) {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	orphans, err := svc.OrphanedBranches(ctx)
	if err != nil {
		return 0, err
	}
	if len(orphans) == 0 {
		return 0, nil
	}

	fmt.Fprintf(out, "\n%d local branch(es) have no worktree:\n\n", len(orphans))
	for _, ref := range orphans {
		fmt.Fprintf(out, "  %s  %s\n", ref.Repo, ref.Branch)
	}
	fmt.Fprintln(out)

	if !deleteBranches {
		if assumeYes {
			fmt.Fprintln(out, "Rerun with --delete-branches to delete them.")
			return 0, nil
		}
		ok, err := d.confirmer.Confirm(ctx, fmt.Sprintf("Delete %d local branch(es)?", len(orphans)))
		if err != nil {
			return 0, err
		}
		if !ok {
			fmt.Fprintln(out, "Kept the local branch(es).")
			return 0, nil
		}
	}

	res := svc.DeleteBranches(ctx, orphans, force)
	res.Write(out)
	return len(res.Failed), nil
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
