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
		dir   string
		full  bool
		group string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the worktrees Arborist manages",
		Long: `List the worktrees Arborist manages, each with a short id usable with
"arb open" and "arb remove".

A GROUP column appears once any worktree lives in a group; --group narrows the
listing to one of them.`,
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
			if cmd.Flags().Changed("group") {
				worktrees = filterByGroup(worktrees, group)
			}
			printWorktrees(cmd.OutOrStdout(), worktrees, svc.WorktreeRoot, full)
			return nil
		},
	}

	cmd.Flags().StringVar(&group, "group", "",
		`only show worktrees in this group (pass --group "" for the ungrouped ones)`)
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

	// The GROUP column earns its width only once something is grouped, so a
	// workspace that never uses groups sees the table it always saw.
	grouped := anyGrouped(worktrees)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	if grouped {
		fmt.Fprintln(tw, "ID\tREPOSITORY\tGROUP\tBRANCH\tSTATUS\tPATH")
	} else {
		fmt.Fprintln(tw, "ID\tREPOSITORY\tBRANCH\tSTATUS\tPATH")
	}
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
		path := displayPath(wt.Path, worktreeRoot, full)
		if grouped {
			group := wt.Group
			if group == "" {
				group = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", shortIDs[i], repo, group, branch, status, path)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", shortIDs[i], repo, branch, status, path)
	}
	tw.Flush()
}

func anyGrouped(worktrees []worktree.ManagedWorktree) bool {
	for _, wt := range worktrees {
		if wt.Group != "" {
			return true
		}
	}
	return false
}

// filterByGroup keeps only the worktrees in group; an empty group selects the
// ungrouped ones, which is what --group "" asks for.
func filterByGroup(worktrees []worktree.ManagedWorktree, group string) []worktree.ManagedWorktree {
	var kept []worktree.ManagedWorktree
	for _, wt := range worktrees {
		if wt.Group == group {
			kept = append(kept, wt)
		}
	}
	return kept
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
