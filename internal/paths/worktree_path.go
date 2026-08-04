package paths

import (
	"path/filepath"
	"strings"
)

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
//	<worktreeRoot>/<repo>/<sanitized-branch>              (group is empty)
//	<worktreeRoot>/<group>/<repo>/<sanitized-branch>      (grouped)
//
// The repository and the sanitized branch are nested so that worktrees for the
// same branch across different repositories never collide. A group nests above
// the repository so everything in one group is a single directory to inspect or
// delete. An empty group keeps the original layout, so worktrees created before
// groups existed never move. worktreeRoot should already be expanded (see
// Expand), and group should already be sanitized (see SanitizeGroupName).
func WorktreePath(worktreeRoot, group, repo, branch string) string {
	if group == "" {
		return filepath.Join(worktreeRoot, repo, SanitizeBranchName(branch))
	}
	return filepath.Join(worktreeRoot, group, repo, SanitizeBranchName(branch))
}

// GroupFor returns the group a worktree path belongs to, or "" when it is
// ungrouped or not under worktreeRoot.
//
// The group is derived from the path rather than recorded anywhere: a branch
// name always sanitizes to one segment, so two segments below the root is the
// ungrouped layout and three means the first segment is the group. That keeps
// grouping stateless — there is nothing to fall out of sync with the tree.
func GroupFor(worktreeRoot, path string) string {
	rel, err := filepath.Rel(worktreeRoot, path)
	if err != nil {
		return ""
	}
	segments := strings.Split(rel, string(filepath.Separator))
	if len(segments) != 3 || segments[0] == ".." {
		return ""
	}
	return segments[0]
}
