package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/config"
	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/progress"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// newSetupCmd builds "arb setup <id-or-branch>", which runs the configured setup
// commands for a worktree (the same commands `arb new` runs after creating one).
func newSetupCmd(d deps) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "setup <id-or-branch>",
		Short: "Run the configured setup commands in a worktree",
		Long: `Run this workspace's setup commands in a worktree (by short id or branch).

Setup commands are configured per repository under "setup" in your workspace
config, e.g. {"setup": {"web": ["pnpm install"], "*": ["pnpm install"]}}. They
run through a shell in the worktree directory; because that is your own
ownership-checked config, Arborist runs them without prompting. The same
commands run automatically after "arb new" (unless --no-setup).`,
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

			wt, err := resolveOne(ctx, svc, out, ref)
			if err != nil {
				return err
			}

			cmds := ws.Config.SetupCommands(wt.Repo)
			if len(cmds) == 0 {
				fmt.Fprintf(out, "No setup commands configured for %q.\n", wt.Repo)
				return nil
			}
			return runSetup(ctx, out, d.shell, wt.Path, wt.Owner+"/"+wt.Repo, cmds)
		},
	}

	addDirFlag(cmd, &dir)
	return cmd
}

// runSetup runs commands in dir through the shell, echoing each one and
// streaming its output to the terminal. It stops at the first failure and
// returns its error. This is the visible path used by the explicit `arb setup`
// command, where the user is watching the install run.
func runSetup(ctx context.Context, out io.Writer, shell exec.ShellRunner, dir, label string, commands []string) error {
	fmt.Fprintf(out, "Setting up %s\n  %s\n", label, dir)
	for _, c := range commands {
		fmt.Fprintf(out, "  $ %s\n", c)
		if err := shell.RunShell(ctx, dir, c); err != nil {
			return fmt.Errorf("setup command %q failed: %w", c, err)
		}
	}
	fmt.Fprintf(out, "Setup complete for %s.\n", label)
	return nil
}

// setupFailure records a setup command that failed, with its captured output.
type setupFailure struct {
	repo    string
	branch  string
	command string
	output  string
}

// setupJob is one repository's auto-setup work: a newly created worktree and the
// setup commands configured for its repository.
type setupJob struct {
	wt   worktree.CreatedWorktree
	cmds []string
}

// planSetup enumerates the auto-setup work up front: one job per created
// worktree whose repository has setup commands configured. Building the full
// plan before running anything lets the caller size the progress bar by the
// number of commands (see setupSteps) rather than the number of repositories,
// so the bar reflects real progress instead of jumping to full on the first
// repository.
func planSetup(cfg config.Config, created []worktree.CreatedWorktree) []setupJob {
	var jobs []setupJob
	for _, c := range created {
		if cmds := cfg.SetupCommands(c.Repository.Name); len(cmds) > 0 {
			jobs = append(jobs, setupJob{c, cmds})
		}
	}
	return jobs
}

// setupSteps is the total number of setup commands across all jobs. This is the
// unit count that drives the progress bar: each command is one step, so a single
// repository with several commands still fills the bar smoothly.
func setupSteps(jobs []setupJob) int {
	n := 0
	for _, j := range jobs {
		n += len(j.cmds)
	}
	return n
}

// runAutoSetup runs the configured setup commands for each newly created
// worktree quietly. Output is captured rather than streamed; a progress bar on
// errw names the repository being set up and advances once per completed
// command, and only failures are reported on out (with the failing command's
// captured output). This keeps a successful `arb new` from being buried under
// install logs.
//
// The bar is sized by the total number of commands and advances only after each
// command finishes, so it starts empty and never reads full before the work is
// actually done.
//
// Setup failures are warnings, not hard errors: the worktrees still exist and
// the user can re-run `arb setup <branch>`.
func runAutoSetup(ctx context.Context, out, errw io.Writer, shell exec.ShellRunner, cfg config.Config, created []worktree.CreatedWorktree) {
	jobs := planSetup(cfg, created)
	if len(jobs) == 0 {
		return
	}

	bar := progress.New(errw, setupSteps(jobs))
	bar.Start()
	var failures []setupFailure
	for _, j := range jobs {
		label := "Setting up " + j.wt.Repository.NameWithOwner
		for _, c := range j.cmds {
			bar.Show(label) // name the command about to run; the bar still reflects completed work
			captured, err := shell.RunShellCapture(ctx, j.wt.Path, c)
			if err != nil {
				failures = append(failures, setupFailure{
					repo:    j.wt.Repository.NameWithOwner,
					branch:  j.wt.Branch,
					command: c,
					output:  string(captured),
				})
				break // stop this repository's setup at the first failure
			}
			bar.Advance(label) // mark the command done only after it succeeds
		}
	}
	bar.Stop()

	for _, f := range failures {
		fmt.Fprintf(out, "\nSetup failed for %s on `%s`.\n", f.repo, f.command)
		if detail := lastLines(f.output, 20); detail != "" {
			fmt.Fprintln(out, detail)
		}
		fmt.Fprintf(out, "Re-run with: arb setup %s\n", f.branch)
	}
}

// lastLines returns up to the last n lines of s, each indented two spaces, so a
// failure shows the tail of the install log (where the real error usually is)
// without dumping the entire output. It returns "" when s has no content.
func lastLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if strings.TrimSpace(s) == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, ln := range lines {
		lines[i] = "  " + ln
	}
	return strings.Join(lines, "\n")
}
