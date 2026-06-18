package paths

import (
	"path/filepath"
	"testing"
)

func TestRepoPath(t *testing.T) {
	got := RepoPath("/home/u/work/acme", "web-app")
	want := filepath.Join("/home/u/work/acme", "web-app")
	if got != want {
		t.Fatalf("RepoPath() = %q, want %q", got, want)
	}
}

func TestWorktreePath_SanitizesBranch(t *testing.T) {
	got := WorktreePath("/home/u/work/acme/worktrees", "web-app", "feature/company-migration-flow")
	want := filepath.Join("/home/u/work/acme/worktrees", "web-app", "feature-company-migration-flow")
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}

func TestWorktreePath_NestedBranch(t *testing.T) {
	got := WorktreePath("/wt", "r", "user/feature/x")
	want := filepath.Join("/wt", "r", "user-feature-x")
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}
