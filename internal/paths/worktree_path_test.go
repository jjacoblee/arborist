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
	got := WorktreePath("/home/u/work/acme/worktrees", "", "web-app", "feature/company-migration-flow")
	want := filepath.Join("/home/u/work/acme/worktrees", "web-app", "feature-company-migration-flow")
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}

func TestWorktreePath_NestedBranch(t *testing.T) {
	got := WorktreePath("/wt", "", "r", "user/feature/x")
	want := filepath.Join("/wt", "r", "user-feature-x")
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}

func TestWorktreePath_GroupNestsAboveTheRepo(t *testing.T) {
	// The group is the outermost segment so a whole group is one directory to
	// delete, which is the point of grouping.
	got := WorktreePath("/wt", "review", "web", "pr/1234")
	want := filepath.Join("/wt", "review", "web", "pr-1234")
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}

func TestGroupFor(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		// Two segments below the root is the ungrouped layout.
		{filepath.Join("/wt", "web", "feature-x"), ""},
		// Three means the first segment names the group.
		{filepath.Join("/wt", "review", "web", "pr-1234"), "review"},
		// Anything shallower or outside the root has no group to report.
		{filepath.Join("/wt", "loose"), ""},
		{filepath.Join("/elsewhere", "review", "web", "x"), ""},
	}
	for _, tt := range tests {
		if got := GroupFor("/wt", tt.path); got != tt.want {
			t.Fatalf("GroupFor(/wt, %q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
