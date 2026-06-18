package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/config"
)

// newInitCmd builds the "arb init" command, which creates an owner workspace by
// writing a hidden .arborist.json config in the target directory.
func newInitCmd() *cobra.Command {
	var (
		dir          string
		owner        string
		worktreeRoot string
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "init --owner <github-owner>",
		Short: "Create an Arborist workspace in the current directory",
		Long: `Create an Arborist workspace.

Run this inside the folder you want to use as an owner workspace. It writes a
hidden .arborist.json config at the workspace root recording the GitHub owner to
work with. Base repositories are cloned directly under this folder, and
worktrees go in a sibling "worktrees" folder by default.

If a workspace config already exists, init makes no changes unless --force is
given.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			if owner == "" {
				return errors.New("an owner is required.\n\nRun:\n  arb init --owner <github-owner>")
			}

			target := dir
			if target == "" {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("determine current directory: %w", err)
				}
				target = wd
			}
			path := config.ConfigPath(target)

			switch _, statErr := os.Stat(path); {
			case statErr == nil && !force:
				fmt.Fprintf(out, "Workspace already configured at %s\n", path)
				fmt.Fprintln(out, "Use --force to overwrite it.")
				return nil
			case statErr != nil && !errors.Is(statErr, os.ErrNotExist):
				return fmt.Errorf("check config path %s: %w", path, statErr)
			}

			cfg := config.Config{Owner: owner, WorktreeRoot: worktreeRoot}
			if err := config.Save(path, cfg); err != nil {
				return err
			}

			fmt.Fprintf(out, "Created Arborist workspace at %s\n\n", target)
			fmt.Fprintf(out, "  config:        %s\n", path)
			fmt.Fprintf(out, "  owner:         %s\n", cfg.Owner)
			fmt.Fprintf(out, "  worktreeRoot:  %s\n", cfg.ResolveWorktreeRoot(target))
			fmt.Fprintf(out, "  copyEnvFiles:  %v\n", cfg.CopyEnvFiles)
			return nil
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "GitHub user or organization for this workspace (required)")
	cmd.Flags().StringVar(&worktreeRoot, "worktree-root", "", `where worktrees go (default: "<workspace>/worktrees")`)
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing workspace config")
	addDirFlag(cmd, &dir)
	return cmd
}
