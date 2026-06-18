// Package cli defines the Arborist command-line interface and wires the
// application's commands together.
//
// Command files in this package are intentionally thin. They parse arguments,
// load dependencies, call into the focused internal packages (config, git,
// github, picker, paths, worktree), and print results. Business logic does not
// belong here.
package cli

import (
	"github.com/spf13/cobra"
)

const longDescription = `Arborist is a guided workflow tool for managing Git worktrees across
multiple repositories.

Instead of memorizing long "git worktree" commands and repository paths, you
run a single command:

  arb new <branch-name>

Arborist then helps you pick repositories, clone any that are missing, and
create predictable, isolated worktrees for the branch. Worktrees get short, stable
ids you can use to open or remove them.

Run commands inside an owner workspace (created with "arb init --owner <owner>").`

// NewRootCmd constructs the root Arborist command wired with the real system
// dependencies (a live command runner). The version is injected from main so it
// can be overridden at build time.
func NewRootCmd(version string) *cobra.Command {
	return newRootCmd(version, defaultDeps())
}

// newRootCmd builds the command tree with explicit dependencies, so tests can
// inject fakes in place of the real command runner.
func newRootCmd(version string, d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "arb",
		Short:   "Guided Git worktree management across multiple repositories",
		Long:    longDescription,
		Version: version,
		// Reject stray positional arguments so "arborist bogus" fails clearly
		// (an unrecognized subcommand reports "unknown command" on its own).
		Args: cobra.NoArgs,
		// Print the error once (returned from Execute) instead of dumping full
		// usage on every runtime error. Usage is still shown for argument and
		// flag mistakes.
		SilenceUsage: true,
		// With no subcommand, show help rather than returning an error.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newNewCmd(d))
	cmd.AddCommand(newListCmd(d))
	cmd.AddCommand(newOpenCmd(d))
	cmd.AddCommand(newSetupCmd(d))
	cmd.AddCommand(newRemoveCmd(d))
	cmd.AddCommand(newPruneCmd(d))
	cmd.AddCommand(newRepoCmd(d))
	cmd.AddCommand(newConfigCmd(d))

	return cmd
}

// Execute builds the root command and runs it. It returns any error to the
// caller (main), which is responsible for the process exit code. Cobra prints
// the error message itself.
func Execute(version string) error {
	return NewRootCmd(version).Execute()
}
