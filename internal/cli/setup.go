package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
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

// runSetup runs commands in dir through the shell, echoing each one. It stops at
// the first failure and returns its error.
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
