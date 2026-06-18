package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serviceWithWorktrees returns a Service rooted in fresh temp dirs.
func serviceWithWorktrees(t *testing.T, g *fakeGit) (Service, string) {
	t.Helper()
	wtRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	return Service{Git: g, Owner: "acme", WorkspaceRoot: workspaceRoot, WorktreeRoot: wtRoot}, wtRoot
}

func TestFindForRemoval_MatchesBranch(t *testing.T) {
	wtRoot := t.TempDir()
	// Same branch across two repos, plus an unrelated worktree.
	webWt := filepath.Join(wtRoot, "web", "feature-x")
	apiWt := filepath.Join(wtRoot, "api", "feature-x")
	webOther := filepath.Join(wtRoot, "web", "other")
	mkWorktreeDir(t, webWt)
	mkWorktreeDir(t, apiWt)
	mkWorktreeDir(t, webOther)

	g := &fakeGit{
		MainRepoPathFn: func(p string) (string, error) {
			if strings.Contains(p, string(filepath.Separator)+"api"+string(filepath.Separator)) {
				return "/clones/api", nil
			}
			return "/clones/web", nil
		},
		CurrentBranchFn: func(p string) (string, error) {
			if strings.HasSuffix(p, "other") {
				return "other", nil
			}
			return "feature/x", nil
		},
	}
	s := Service{Git: g, Owner: "acme", WorktreeRoot: wtRoot}

	matches, err := s.Find(context.Background(), "feature/x")
	if err != nil {
		t.Fatalf("FindForRemoval: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2 (web + api feature/x): %+v", len(matches), matches)
	}
}

func TestFindForRemoval_ByID(t *testing.T) {
	wtRoot := t.TempDir()
	a := filepath.Join(wtRoot, "web", "feature-a")
	b := filepath.Join(wtRoot, "web", "feature-b")
	mkWorktreeDir(t, a)
	mkWorktreeDir(t, b)

	g := &fakeGit{
		MainRepoPathFn: func(string) (string, error) { return "/clones/web", nil },
		CurrentBranchFn: func(p string) (string, error) {
			if strings.HasSuffix(p, "feature-a") {
				return "feat/a", nil
			}
			return "feat/b", nil
		},
	}
	s := Service{Git: g, Owner: "acme", WorktreeRoot: wtRoot}

	matches, err := s.Find(context.Background(), ID(a)[:8])
	if err != nil {
		t.Fatalf("FindForRemoval by id: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != a {
		t.Fatalf("got %d matches, want exactly worktree a: %+v", len(matches), matches)
	}
}

func TestFindForRemoval_AmbiguousID(t *testing.T) {
	wtRoot := t.TempDir()
	// 20 worktrees over 16 possible first hex digits guarantees, by pigeonhole,
	// that at least two ids share a first character.
	var paths []string
	for i := 0; i < 20; i++ {
		p := filepath.Join(wtRoot, "web", fmt.Sprintf("wt-%d", i))
		mkWorktreeDir(t, p)
		paths = append(paths, p)
	}
	g := &fakeGit{
		MainRepoPathFn:  func(string) (string, error) { return "/clones/web", nil },
		CurrentBranchFn: func(string) (string, error) { return "feat", nil },
	}
	s := Service{Git: g, Owner: "acme", WorktreeRoot: wtRoot}

	counts := map[byte]int{}
	for _, p := range paths {
		counts[ID(p)[0]]++
	}
	var shared byte
	for c, n := range counts {
		if n >= 2 {
			shared = c
			break
		}
	}

	_, err := s.Find(context.Background(), string(shared))
	var amb *AmbiguousIDError
	if !errors.As(err, &amb) {
		t.Fatalf("expected AmbiguousIDError for shared prefix %q, got: %v", string(shared), err)
	}
}

func TestRemove_CleanIsRemoved(t *testing.T) {
	g := &fakeGit{}
	s, _ := serviceWithWorktrees(t, g)
	target := ManagedWorktree{
		Owner: "acme", Repo: "web", Branch: "feature/x",
		Path: "/wt/web-feature-x", RepoPath: "/repos/acme/web",
	}

	res := s.Remove(context.Background(), []ManagedWorktree{target}, false)
	if len(res.Removed) != 1 || len(res.Skipped) != 0 || len(res.Failed) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(g.Removed) != 1 || g.Removed[0] != target.Path {
		t.Fatalf("expected RemoveWorktree(%q), got %v", target.Path, g.Removed)
	}
	if len(g.Pruned) != 1 || g.Pruned[0] != target.RepoPath {
		t.Fatalf("expected a prune of %q after removal, got %v", target.RepoPath, g.Pruned)
	}
}

func TestRemove_DirtySkippedWithoutForce(t *testing.T) {
	g := &fakeGit{}
	s, _ := serviceWithWorktrees(t, g)
	target := ManagedWorktree{Repo: "web", Branch: "feature/x", Path: "/wt/x", RepoPath: "/repos/acme/web", Dirty: true}

	res := s.Remove(context.Background(), []ManagedWorktree{target}, false)
	if len(res.Skipped) != 1 || len(res.Removed) != 0 {
		t.Fatalf("dirty worktree must be skipped without --force: %+v", res)
	}
	if len(g.Removed) != 0 {
		t.Fatal("RemoveWorktree must not be called for a skipped dirty worktree")
	}
}

func TestRemove_DirtyRemovedWithForce(t *testing.T) {
	g := &fakeGit{}
	s, _ := serviceWithWorktrees(t, g)
	target := ManagedWorktree{Repo: "web", Branch: "feature/x", Path: "/wt/x", RepoPath: "/repos/acme/web", Dirty: true}

	res := s.Remove(context.Background(), []ManagedWorktree{target}, true)
	if len(res.Removed) != 1 || len(res.Skipped) != 0 {
		t.Fatalf("with --force a dirty worktree should be removed: %+v", res)
	}
}

func TestRemove_FailureReported(t *testing.T) {
	g := &fakeGit{RemoveFn: func(_, _ string, _ bool) error { return errors.New("locked") }}
	s, _ := serviceWithWorktrees(t, g)
	target := ManagedWorktree{Repo: "web", Branch: "feature/x", Path: "/wt/x", RepoPath: "/repos/acme/web"}

	res := s.Remove(context.Background(), []ManagedWorktree{target}, false)
	if !res.HasFailures() || len(res.Removed) != 0 {
		t.Fatalf("expected a failure: %+v", res)
	}
}

func TestPrune_RunsOnBaseRepos(t *testing.T) {
	g := &fakeGit{IsRepoFn: func(string) bool { return true }}
	workspaceRoot := t.TempDir()
	s := Service{Git: g, WorkspaceRoot: workspaceRoot, WorktreeRoot: filepath.Join(workspaceRoot, "worktrees")}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "api"), 0o755); err != nil {
		t.Fatal(err)
	}

	pruned, err := s.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(pruned) != 2 {
		t.Fatalf("expected 2 base repos pruned, got %d (%v)", len(pruned), pruned)
	}
}
