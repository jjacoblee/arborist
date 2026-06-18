package paths

import "path/filepath"

// RepoPath returns the local clone path for a repository inside an owner
// workspace:
//
//	<workspaceRoot>/<repo>
//
// The workspace root is already scoped to a single owner (it holds that owner's
// .arborist.json), so the owner is not repeated in the path. workspaceRoot
// should already be expanded (see Expand); RepoPath does not expand "~" itself.
func RepoPath(workspaceRoot, repo string) string {
	return filepath.Join(workspaceRoot, repo)
}

// WorktreePath returns the worktree path for a repository and branch:
//
//	<worktreeRoot>/<repo>/<sanitized-branch>
//
// The repository and the sanitized branch are nested so that worktrees for the
// same branch across different repositories never collide. worktreeRoot should
// already be expanded (see Expand).
func WorktreePath(worktreeRoot, repo, branch string) string {
	return filepath.Join(worktreeRoot, repo, SanitizeBranchName(branch))
}
