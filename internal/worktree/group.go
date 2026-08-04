package worktree

import (
	"sort"

	"github.com/jjacoblee/arborist/internal/paths"
)

// Groups returns the names of the groups that already hold worktrees, sorted
// and each named once.
//
// It reads the directory tree only — no git, no network — because its job is to
// answer "have I used this group name before?" before any work begins, and that
// question should never be the slow part of a command.
func (s Service) Groups() ([]string, error) {
	roots, err := worktreeRootsUnder(s.WorktreeRoot)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var groups []string
	for _, path := range roots {
		group := paths.GroupFor(s.WorktreeRoot, path)
		if group == "" || seen[group] {
			continue
		}
		seen[group] = true
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, nil
}
