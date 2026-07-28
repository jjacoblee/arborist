package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/paths"
	"github.com/jjacoblee/arborist/internal/picker"
)

// newNewCmd builds the flagship "arb new <branch-name>" command.
func newNewCmd(d deps) *cobra.Command {
	var (
		dir       string
		name      string
		base      string
		group     string
		limit     int
		noSetup   bool
		assumeYes bool
		repoFlags []string
	)

	cmd := &cobra.Command{
		Use:   "new <branch-name>",
		Short: "Create worktrees for a branch across selected repositories",
		Long: `Create worktrees for a branch across one or more repositories.

Run inside an owner workspace. Arborist validates the branch name, checks that
git and the GitHub CLI are ready, discovers the workspace owner's repositories,
and shows an interactive picker. For each selected repository it clones the repo
if needed, fetches, and creates a worktree for the branch (reusing an existing
branch, tracking a remote branch, or creating a new branch from the default
branch as appropriate).

Use --repo to name the repositories up front and skip the picker entirely, which
makes the command usable from a script. It is repeatable and also accepts comma-
or space-separated names, so these are equivalent:

  arb new my-branch --repo api --repo web
  arb new my-branch --repo api,web
  arb new my-branch --repo "api web"

A name may be bare ("api") or owner-qualified ("acme/api"). If any name matches
no repository the command fails without creating anything.

Use --base to branch off something other than the default branch (for example
another feature branch); it applies only when the branch is newly created.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if err := paths.ValidateBranchName(branch); err != nil {
				return err
			}

			ws, err := requireWorkspace(dir)
			if err != nil {
				return err
			}

			g := git.New(d.runner)
			h := github.New(d.runner)

			if err := checkPrerequisites(ctx, g, h); err != nil {
				return err
			}

			repos, err := h.ListRepos(ctx, ws.Config.Owner, limit)
			if err != nil {
				return err
			}
			if len(repos) == 0 {
				return fmt.Errorf("no repositories found for %q", ws.Config.Owner)
			}

			var selected []github.Repository
			if names := parseRepoFlag(repoFlags); len(names) > 0 {
				// Naming repositories is a statement of intent: resolve them and
				// go straight to work, with no interactive step to hang a script.
				selected, err = github.MatchRepos(repos, names)
				if err != nil {
					return err
				}
			} else {
				selected, err = d.selector.Select(ctx, branch, repos)
				if err != nil {
					if errors.Is(err, picker.ErrCanceled) {
						fmt.Fprintln(out, "Canceled. No worktrees were created.")
						return nil
					}
					return err
				}
				if len(selected) == 0 {
					fmt.Fprintln(out, "No repositories selected. Nothing to do.")
					return nil
				}
			}

			svc, err := newWorktreeService(g, h, ws)
			if err != nil {
				return err
			}
			svc.Base = base

			svc.Group, err = resolveGroup(cmd, d, svc, ws.Config, group, assumeYes)
			if err != nil {
				if errors.Is(err, errGroupDeclined) {
					return nil // already reported; not a failure
				}
				return err
			}
			if svc.Group != "" {
				fmt.Fprintf(out, "Group: %s\n\n", svc.Group)
			}

			// Show a progress bar on stderr while repositories are cloned and
			// worktrees created, so the slow steps don't look like a hang. It
			// draws only on a terminal and is a no-op otherwise, which keeps
			// stdout (the summary) clean for pipes and tests.
			svc.Progress = newStepsReporter(cmd.ErrOrStderr())
			result := svc.Create(ctx, branch, name, selected)

			result.Write(out)

			// Run configured setup commands in each newly created worktree,
			// quietly: a progress bar shows activity and only failures are
			// printed (with the failing command's output). Setup failures are
			// warnings — the worktree still exists, and the user can re-run with
			// `arb setup`.
			if !noSetup {
				runAutoSetup(ctx, out, cmd.ErrOrStderr(), d.shell, ws.Config, result.Created)
			}

			if result.HasFailures() {
				return fmt.Errorf("%d of %d selected repositories failed; see the summary above",
					len(result.Failed), len(selected))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&group, "group", "",
		`group folder to create the worktrees under, e.g. --group review (pass --group "" to opt out of a configured defaultGroup)`)
	cmd.Flags().BoolVar(&assumeYes, "yes", false,
		"skip confirmation prompts (for example when using a group name for the first time)")
	cmd.Flags().StringSliceVar(&repoFlags, "repo", nil,
		"repositories to use instead of opening the picker; repeatable, and accepts comma- or space-separated names (e.g. --repo api,web)")
	cmd.Flags().IntVar(&limit, "limit", github.DefaultRepoLimit, "maximum number of repositories to fetch")
	cmd.Flags().StringVar(&name, "name", "",
		"short worktree folder name to use instead of the full branch (branch is unchanged)")
	cmd.Flags().StringVar(&base, "base", "",
		"branch or ref to create the new branch from (default: the repo's default branch; only used when the branch doesn't already exist)")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false,
		"skip the configured setup commands for newly created worktrees")
	addDirFlag(cmd, &dir)
	return cmd
}
