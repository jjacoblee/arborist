package worktree

import "context"

// RemovalCandidate is one worktree offered for removal, together with the state
// a user needs to see before choosing it.
type RemovalCandidate struct {
	Worktree ManagedWorktree
	// Unpushed reports commits that exist on no origin ref. It is advisory —
	// what actually protects that work is git refusing to delete the branch.
	Unpushed bool
}

// RemovalCandidates returns every managed worktree together with the state that
// bears on removing it.
//
// It reports rather than filters: which of these a caller is willing to offer
// (and whether it needs --force to offer the risky ones) is a presentation
// decision. The actual safety rule lives in Remove, which never deletes a dirty
// worktree without force however it was chosen.
func (s Service) RemovalCandidates(ctx context.Context) ([]RemovalCandidate, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	candidates := make([]RemovalCandidate, 0, len(all))
	for _, wt := range all {
		// Best effort: a repository that cannot answer just goes unmarked
		// rather than failing the whole listing.
		unpushed, _ := s.Git.HasUnpushedCommits(ctx, wt.Path)
		candidates = append(candidates, RemovalCandidate{Worktree: wt, Unpushed: unpushed})
	}
	return candidates, nil
}
