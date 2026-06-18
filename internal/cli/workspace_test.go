package cli

import (
	"path/filepath"
	"testing"

	"github.com/jjacoblee/arborist/internal/config"
)

// writeWorkspace creates a workspace directory containing a .arborist.json for
// owner and returns the directory (which is also the workspace/repo root).
func writeWorkspace(t *testing.T, owner string) string {
	t.Helper()
	dir := t.TempDir()
	if err := config.Save(config.ConfigPath(dir), config.Config{Owner: owner}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// workspaceWorktreeRoot returns the default worktree root for a workspace dir.
func workspaceWorktreeRoot(dir string) string {
	return filepath.Join(dir, "worktrees")
}
