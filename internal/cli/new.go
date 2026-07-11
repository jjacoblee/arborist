package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/paths"
	"github.com/jjacoblee/arborist/internal/picker"
)

// newNewCmd builds the flagship "arb new <branch-name>" command.
func newNewCmd(d deps) *cobra.Command {
	var (
		dir     string
		name    string
		base    string
		limit   int
		noSetup bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "new <branch-name> [owner/repo ...]",
		Short: "Create worktrees for a branch across selected repositories",
		Long: `Create worktrees for a branch across one or more repositories.

Run inside an owner workspace. Arborist validates the branch name, checks that
git and the GitHub CLI are ready, discovers the workspace owner's repositories,
and shows an interactive picker. For each selected repository it clones the repo
if needed, fetches, and creates a worktree for the branch (reusing an existing
branch, tracking a remote branch, or creating a new branch from the default
branch as appropriate).

Name one or more repositories as owner/repo after the branch to skip both
account-wide discovery and the interactive picker:

  arb new feature/x acme/web acme/api

Use --base to branch off something other than the default branch (for example
another feature branch); it applies only when the branch is newly created.

Use --json for machine-readable output (for scripts and coding agents). It is
non-interactive, so repositories must be named; the result reports each created
worktree's absolute path and the outcome of its setup commands:

  arb new feature/x acme/web --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			repoRefs := args[1:]
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if err := paths.ValidateBranchName(branch); err != nil {
				return err
			}
			if err := validateRepoRefs(repoRefs); err != nil {
				return err
			}
			// --json is a non-interactive contract: it can never open the
			// repository picker, so the caller must say which repos it wants.
			// Fail before running any commands, with the fix in the message.
			if jsonOut && len(repoRefs) == 0 {
				return fmt.Errorf("--json runs non-interactively; name repositories explicitly:\n"+
					"  arb new %s <owner/repo> [<owner/repo> ...] --json", branch)
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

			var selected []github.Repository
			if len(repoRefs) > 0 {
				// The caller already knows which repositories they want: skip
				// account-wide discovery and the interactive picker entirely.
				selected, err = resolveRepoRefs(ctx, h, repoRefs)
				if err != nil {
					return err
				}
			} else {
				repos, err := h.ListRepos(ctx, ws.Config.Owner, limit)
				if err != nil {
					return err
				}
				if len(repos) == 0 {
					return fmt.Errorf("no repositories found for %q", ws.Config.Owner)
				}

				selected, err = d.selector.Select(ctx, branch, repos)
				if err != nil {
					if errors.Is(err, picker.ErrCanceled) {
						fmt.Fprintln(out, "Canceled. No worktrees were created.")
						return nil
					}
					if errors.Is(err, picker.ErrNotATerminal) {
						// A script or agent is driving and there is no terminal
						// for the picker. Point at the non-interactive form
						// instead of surfacing a prompt-rendering failure.
						return fmt.Errorf("no terminal is available for the interactive repository picker; "+
							"name repositories explicitly:\n  arb new %s <owner/repo> [<owner/repo> ...]", branch)
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

			// Show a progress bar on stderr while repositories are cloned and
			// worktrees created, so the slow steps don't look like a hang. It
			// draws only on a terminal and is a no-op otherwise, which keeps
			// stdout (the summary) clean for pipes and tests.
			svc.Progress = newStepsReporter(cmd.ErrOrStderr())
			result := svc.Create(ctx, branch, name, selected)

			if !jsonOut {
				result.Write(out)
			}

			// Run configured setup commands in each newly created worktree,
			// quietly: a progress bar shows activity and only failures are
			// reported — printed for humans, embedded per-worktree for --json.
			// Setup failures are warnings — the worktree still exists, and the
			// user can re-run with `arb setup`.
			var setupFailures []setupFailure
			if !noSetup {
				setupFailures = runAutoSetup(ctx, cmd.ErrOrStderr(), d.shell, ws.Config, result.Created)
				if !jsonOut {
					printSetupFailures(out, setupFailures)
				}
			}

			if jsonOut {
				if err := writeNewJSON(out, result, ws.Config, noSetup, setupFailures); err != nil {
					return err
				}
			}

			if result.HasFailures() {
				return fmt.Errorf("%d of %d selected repositories failed; see the summary above",
					len(result.Failed), len(selected))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", github.DefaultRepoLimit, "maximum number of repositories to fetch")
	cmd.Flags().StringVar(&name, "name", "",
		"short worktree folder name to use instead of the full branch (branch is unchanged)")
	cmd.Flags().StringVar(&base, "base", "",
		"branch or ref to create the new branch from (default: the repo's default branch; only used when the branch doesn't already exist)")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false,
		"skip the configured setup commands for newly created worktrees")
	cmd.Flags().BoolVar(&jsonOut, "json", false,
		"print the result as JSON (non-interactive; repositories must be named)")
	addDirFlag(cmd, &dir)
	return cmd
}

// validateRepoRefs checks that every positional repository is in "owner/repo"
// form. It runs before any command execution, so a malformed reference fails
// fast just like an invalid branch name does.
func validateRepoRefs(repoRefs []string) error {
	for _, ref := range repoRefs {
		owner, name, ok := strings.Cut(ref, "/")
		if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
			return fmt.Errorf("invalid repository %q: expected the form owner/repo", ref)
		}
	}
	return nil
}

// resolveRepoRefs looks up each "owner/repo" via gh, in the order given,
// skipping duplicates. repoRefs must already be validated (see
// validateRepoRefs). It lets named repositories bypass ListRepos and the
// interactive picker when the caller already knows which repositories they
// want.
func resolveRepoRefs(ctx context.Context, h github.Client, repoRefs []string) ([]github.Repository, error) {
	seen := make(map[string]bool, len(repoRefs))
	repos := make([]github.Repository, 0, len(repoRefs))
	for _, ref := range repoRefs {
		if seen[ref] {
			continue
		}
		seen[ref] = true

		repo, err := h.ViewRepo(ctx, ref)
		if err != nil {
			if errors.Is(err, github.ErrNotInstalled) {
				return nil, errGHNotInstalled
			}
			return nil, fmt.Errorf("repository %q: %w", ref, err)
		}
		repos = append(repos, repo)
	}
	return repos, nil
}
