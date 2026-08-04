package github

import (
	"fmt"
	"strings"
)

// UnknownRepositoriesError reports names that matched no repository. Every
// unknown name is collected into one error so a mistyped list is fixed in one
// pass instead of one failed run per typo.
type UnknownRepositoriesError struct {
	Names []string
}

func (e *UnknownRepositoriesError) Error() string {
	if len(e.Names) == 1 {
		return fmt.Sprintf("unknown repository %q", e.Names[0])
	}
	quoted := make([]string, len(e.Names))
	for i, n := range e.Names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return "unknown repositories " + strings.Join(quoted, ", ")
}

// MatchRepos resolves names against repos, returning them in the order given.
//
// A name may be bare ("api") or owner-qualified ("acme/api"), and matching
// ignores case, since GitHub treats repository names case-insensitively. Any
// name that matches nothing makes the whole call fail with an
// UnknownRepositoriesError: a caller naming repositories up front is scripting
// a known set, and half of it is not a useful outcome.
func MatchRepos(repos []Repository, names []string) ([]Repository, error) {
	index := make(map[string]Repository, len(repos)*2)
	for _, r := range repos {
		index[strings.ToLower(r.Name)] = r
		index[strings.ToLower(r.NameWithOwner)] = r
	}

	var (
		matched []Repository
		unknown []string
	)
	for _, name := range names {
		r, ok := index[strings.ToLower(name)]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		matched = append(matched, r)
	}
	if len(unknown) > 0 {
		return nil, &UnknownRepositoriesError{Names: unknown}
	}
	return matched, nil
}
