package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/git"
)

func TestOrphanedBranches_ScansBaseRepos(t *testing.T) {
	workspaceRoot := t.TempDir()
	webRepo := filepath.Join(workspaceRoot, "web")
	if err := os.MkdirAll(webRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	g := &fakeGit{
		LocalBranchesFn: func(string) ([]string, error) {
			return []string{"main", "feature/x", "feature/y"}, nil
		},
		ListFn: func(string) ([]git.Worktree, error) {
			return []git.Worktree{
				{Path: webRepo, Branch: "main"},
				{Path: "/wt/web/feature-x", Branch: "feature/x"},
			}, nil
		},
	}
	s := Service{Git: g, Owner: "acme", WorkspaceRoot: workspaceRoot, WorktreeRoot: t.TempDir()}

	got, err := s.OrphanedBranches(context.Background())
	if err != nil {
		t.Fatalf("OrphanedBranches: %v", err)
	}
	want := []BranchRef{{Repo: "web", RepoPath: webRepo, Branch: "feature/y"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("orphans = %+v, want %+v", got, want)
	}
}

func TestOrphanedBranches_NeverReportsTheDefaultBranch(t *testing.T) {
	// A base clone left on a feature branch means the default branch has no
	// worktree of its own. It is still never a deletion candidate.
	workspaceRoot := t.TempDir()
	webRepo := filepath.Join(workspaceRoot, "web")
	if err := os.MkdirAll(webRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	g := &fakeGit{
		LocalBranchesFn: func(string) ([]string, error) { return []string{"main", "feature/x"}, nil },
		ListFn: func(string) ([]git.Worktree, error) {
			return []git.Worktree{{Path: webRepo, Branch: "feature/x"}}, nil
		},
		DefaultBranchFn: func(string) (string, error) { return "main", nil },
	}
	s := Service{Git: g, Owner: "acme", WorkspaceRoot: workspaceRoot, WorktreeRoot: t.TempDir()}

	got, err := s.OrphanedBranches(context.Background())
	if err != nil {
		t.Fatalf("OrphanedBranches: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("orphans = %+v, want none (main is protected, feature/x is in use)", got)
	}
}

func TestOrphanedBranches_GuardsConventionalNamesWhenDefaultUnknown(t *testing.T) {
	workspaceRoot := t.TempDir()
	webRepo := filepath.Join(workspaceRoot, "web")
	if err := os.MkdirAll(webRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	g := &fakeGit{
		LocalBranchesFn: func(string) ([]string, error) {
			return []string{"main", "master", "feature/x"}, nil
		},
		ListFn:          func(string) ([]git.Worktree, error) { return nil, nil },
		DefaultBranchFn: func(string) (string, error) { return "", errors.New("origin/HEAD is not set") },
	}
	s := Service{Git: g, Owner: "acme", WorkspaceRoot: workspaceRoot, WorktreeRoot: t.TempDir()}

	got, err := s.OrphanedBranches(context.Background())
	if err != nil {
		t.Fatalf("OrphanedBranches: %v", err)
	}
	if len(got) != 1 || got[0].Branch != "feature/x" {
		t.Fatalf("orphans = %+v, want only feature/x", got)
	}
}

func TestOrphanedBranchesAfter_NeverReportsTheDefaultBranch(t *testing.T) {
	g := &fakeGit{
		ListFn:          func(string) ([]git.Worktree, error) { return nil, nil },
		DefaultBranchFn: func(string) (string, error) { return "main", nil },
	}
	s := Service{Git: g, Owner: "acme"}

	removed := []ManagedWorktree{{Repo: "web", Branch: "main", RepoPath: "/clones/web"}}
	got, err := s.OrphanedBranchesAfter(context.Background(), removed)
	if err != nil {
		t.Fatalf("OrphanedBranchesAfter: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("orphans = %+v, want none (main is protected)", got)
	}
}

func TestDeleteBranches_Deletes(t *testing.T) {
	var calls []string
	g := &fakeGit{
		DeleteBranchFn: func(repoPath, branch string, force bool) error {
			calls = append(calls, repoPath+" "+branch+" force="+boolText(force))
			return nil
		},
	}
	s := Service{Git: g, Owner: "acme"}

	res := s.DeleteBranches(context.Background(),
		[]BranchRef{{Repo: "web", RepoPath: "/clones/web", Branch: "feature/x"}}, false)

	if len(res.Deleted) != 1 || res.Deleted[0].Branch != "feature/x" {
		t.Fatalf("deleted = %+v, want feature/x", res.Deleted)
	}
	if len(calls) != 1 || calls[0] != "/clones/web feature/x force=false" {
		t.Fatalf("git calls = %v", calls)
	}
}

func TestDeleteBranches_UnmergedIsSkippedNotFailed(t *testing.T) {
	g := &fakeGit{
		DeleteBranchFn: func(_, _ string, _ bool) error {
			return fmt.Errorf("delete branch: %w", git.ErrBranchNotMerged)
		},
	}
	s := Service{Git: g, Owner: "acme"}

	res := s.DeleteBranches(context.Background(),
		[]BranchRef{{Repo: "web", RepoPath: "/clones/web", Branch: "feature/x"}}, false)

	if len(res.Failed) != 0 {
		t.Fatalf("failed = %+v, want none (a refusal is not a failure)", res.Failed)
	}
	if len(res.Deleted) != 0 {
		t.Fatalf("deleted = %+v, want none", res.Deleted)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("skipped = %+v, want 1", res.Skipped)
	}
	if !strings.Contains(res.Skipped[0].Reason, "--force") {
		t.Fatalf("reason = %q, want it to point at --force", res.Skipped[0].Reason)
	}
}

func TestDeleteBranches_ForceIsForwarded(t *testing.T) {
	var gotForce bool
	g := &fakeGit{
		DeleteBranchFn: func(_, _ string, force bool) error {
			gotForce = force
			return nil
		},
	}
	s := Service{Git: g, Owner: "acme"}

	res := s.DeleteBranches(context.Background(),
		[]BranchRef{{Repo: "web", RepoPath: "/clones/web", Branch: "feature/x"}}, true)

	if !gotForce {
		t.Fatal("force was not forwarded to git")
	}
	if len(res.Deleted) != 1 {
		t.Fatalf("deleted = %+v, want 1", res.Deleted)
	}
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestOrphanedBranchesAfter_LastWorktreeGone(t *testing.T) {
	// feature/x was removed from both repos, but api still has a second
	// worktree on it, so only web's branch is orphaned.
	g := &fakeGit{
		ListFn: func(repoPath string) ([]git.Worktree, error) {
			if repoPath == "/clones/api" {
				return []git.Worktree{
					{Path: "/clones/api", Branch: "main"},
					{Path: "/wt/api/feature-x-2", Branch: "feature/x"},
				}, nil
			}
			return []git.Worktree{{Path: "/clones/web", Branch: "main"}}, nil
		},
	}
	s := Service{Git: g, Owner: "acme"}

	removed := []ManagedWorktree{
		{Repo: "web", Branch: "feature/x", RepoPath: "/clones/web", Path: "/wt/web/feature-x"},
		{Repo: "api", Branch: "feature/x", RepoPath: "/clones/api", Path: "/wt/api/feature-x"},
	}
	got, err := s.OrphanedBranchesAfter(context.Background(), removed)
	if err != nil {
		t.Fatalf("OrphanedBranchesAfter: %v", err)
	}

	want := []BranchRef{{Repo: "web", RepoPath: "/clones/web", Branch: "feature/x"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("orphans = %+v, want %+v", got, want)
	}
}
