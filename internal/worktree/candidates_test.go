package worktree

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// candidateService sets up a worktree root holding a clean worktree
// (web/feature-x) and a dirty one (web/spike).
func candidateService(t *testing.T, g *fakeGit) Service {
	t.Helper()
	wtRoot := t.TempDir()
	mkWorktreeDir(t, filepath.Join(wtRoot, "web", "feature-x"))
	mkWorktreeDir(t, filepath.Join(wtRoot, "web", "spike"))

	g.MainRepoPathFn = func(string) (string, error) { return "/clones/web", nil }
	g.CurrentBranchFn = func(p string) (string, error) { return filepath.Base(p), nil }
	g.IsDirtyFn = func(p string) (bool, error) { return strings.HasSuffix(p, "spike"), nil }

	return Service{Git: g, Owner: "acme", WorktreeRoot: wtRoot}
}

func TestRemovalCandidates_ReportsDirtyState(t *testing.T) {
	s := candidateService(t, &fakeGit{})

	got, err := s.RemovalCandidates(context.Background())
	if err != nil {
		t.Fatalf("RemovalCandidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want every worktree: %+v", len(got), got)
	}
	for _, c := range got {
		want := c.Worktree.Branch == "spike"
		if c.Worktree.Dirty != want {
			t.Fatalf("%s dirty = %v, want %v", c.Worktree.Branch, c.Worktree.Dirty, want)
		}
	}
}

func TestRemovalCandidates_MarksUnpushedWork(t *testing.T) {
	s := candidateService(t, &fakeGit{
		HasUnpushedFn: func(p string) (bool, error) { return strings.HasSuffix(p, "feature-x"), nil },
	})

	got, err := s.RemovalCandidates(context.Background())
	if err != nil {
		t.Fatalf("RemovalCandidates: %v", err)
	}
	for _, c := range got {
		want := c.Worktree.Branch == "feature-x"
		if c.Unpushed != want {
			t.Fatalf("%s unpushed = %v, want %v", c.Worktree.Branch, c.Unpushed, want)
		}
	}
}

func TestRemovalCandidates_UnpushedCheckIsBestEffort(t *testing.T) {
	// The label is advisory; a repository that can't answer must not break the
	// whole picker.
	s := candidateService(t, &fakeGit{
		HasUnpushedFn: func(string) (bool, error) { return false, errors.New("no HEAD") },
	})

	got, err := s.RemovalCandidates(context.Background())
	if err != nil {
		t.Fatalf("a failed unpushed check must not fail the listing: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want both", len(got))
	}
}
