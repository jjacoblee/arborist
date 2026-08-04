package worktree

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGroups_ListsExistingGroupsOnly(t *testing.T) {
	wtRoot := t.TempDir()
	mkWorktreeDir(t, filepath.Join(wtRoot, "web", "feature-x"))          // ungrouped
	mkWorktreeDir(t, filepath.Join(wtRoot, "spike", "web", "try-thing")) // grouped
	mkWorktreeDir(t, filepath.Join(wtRoot, "review", "web", "pr-1234"))
	mkWorktreeDir(t, filepath.Join(wtRoot, "review", "api", "pr-1234")) // same group again

	s := Service{Owner: "acme", WorktreeRoot: wtRoot}

	got, err := s.Groups()
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}
	want := []string{"review", "spike"} // sorted, each named once
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Groups() = %v, want %v", got, want)
	}
}

func TestGroups_EmptyWorktreeRoot(t *testing.T) {
	s := Service{Owner: "acme", WorktreeRoot: filepath.Join(t.TempDir(), "missing")}

	got, err := s.Groups()
	if err != nil {
		t.Fatalf("a missing worktree root is not an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Groups() = %v, want none", got)
	}
}
