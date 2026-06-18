package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jjacoblee/arborist/internal/config"
	"github.com/jjacoblee/arborist/internal/paths"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// errNotInWorkspace is the actionable error shown when a command is run outside
// any Arborist workspace.
var errNotInWorkspace = errors.New(
	"not inside an Arborist workspace.\n\n" +
		"cd into an owner workspace folder, or create one here with:\n" +
		"  arb init --owner <github-owner>")

// addDirFlag registers the common --dir flag, which overrides the directory
// Arborist starts its workspace search from (default: the current directory).
func addDirFlag(cmd *cobra.Command, dir *string) {
	cmd.Flags().StringVar(dir, "dir", "",
		"workspace directory to operate in (default: current directory)")
}

// requireWorkspace locates the owner workspace containing dir (or the current
// directory when dir is empty), walking up to find .arborist.json. It returns
// an actionable error when no workspace is found.
func requireWorkspace(dir string) (config.Workspace, error) {
	start := dir
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return config.Workspace{}, fmt.Errorf("determine current directory: %w", err)
		}
		start = wd
	}

	ws, err := config.Find(start)
	if err != nil {
		if errors.Is(err, config.ErrNotWorkspace) {
			return config.Workspace{}, errNotInWorkspace
		}
		return config.Workspace{}, err
	}
	return ws, nil
}

// newWorktreeService builds a worktree.Service for the given workspace,
// resolving and expanding the worktree root.
func newWorktreeService(g worktree.Git, cloner worktree.Cloner, ws config.Workspace) (worktree.Service, error) {
	worktreeRoot, err := paths.Expand(ws.Config.ResolveWorktreeRoot(ws.Root))
	if err != nil {
		return worktree.Service{}, err
	}
	return worktree.Service{
		Git:           g,
		Cloner:        cloner,
		Owner:         ws.Config.Owner,
		WorkspaceRoot: ws.Root,
		WorktreeRoot:  worktreeRoot,
		CopyEnvFiles:  ws.Config.CopyEnvFiles,
		CopyFiles:     ws.Config.CopyFiles,
	}, nil
}
