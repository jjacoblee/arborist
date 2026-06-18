package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkWorktreeDir creates dir and marks it as a git worktree root by writing a
// ".git" file (as `git worktree add` does for a linked worktree).
func mkWorktreeDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /base/.git/worktrees/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestList_ScansWorktreeRoots(t *testing.T) {
	wtRoot := t.TempDir()

	// A shallow worktree (depth 1) and a nested one (depth 2 under a container),
	// mirroring real-world layouts.
	shallow := filepath.Join(wtRoot, "standalone")
	nested := filepath.Join(wtRoot, "web", "feature-x")
	mkWorktreeDir(t, shallow)
	mkWorktreeDir(t, nested)

	// The nested worktree has many subdirectories — none may be reported as a
	// separate worktree (the bug this guards against).
	for _, sub := range []string{"src", "node_modules", ".github", "packages"} {
		if err := os.MkdirAll(filepath.Join(nested, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	g := &fakeGit{
		MainRepoPathFn: func(string) (string, error) { return "/clones/web", nil },
		CurrentBranchFn: func(p string) (string, error) {
			if strings.Contains(p, "standalone") {
				return "standalone", nil
			}
			return "feature/x", nil
		},
		IsDirtyFn: func(p string) (bool, error) { return strings.Contains(p, "feature-x"), nil },
	}
	s := Service{Git: g, Owner: "acme", WorktreeRoot: wtRoot}

	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d worktrees, want 2 (no per-subdirectory rows): %+v", len(got), got)
	}

	byPath := map[string]ManagedWorktree{}
	for _, w := range got {
		byPath[w.Path] = w
		if w.Owner != "acme" || w.Repo != "web" || w.RepoPath != "/clones/web" {
			t.Fatalf("unexpected owner/repo: %+v", w)
		}
	}
	if w := byPath[nested]; w.Branch != "feature/x" || !w.Dirty {
		t.Fatalf("nested = %+v (want branch feature/x, dirty)", w)
	}
	if w := byPath[shallow]; w.Branch != "standalone" || w.Dirty {
		t.Fatalf("shallow = %+v (want branch standalone, clean)", w)
	}
}

func TestList_SkipsNonWorktreeDirs(t *testing.T) {
	wtRoot := t.TempDir()
	// A plain directory tree with no .git anywhere.
	if err := os.MkdirAll(filepath.Join(wtRoot, "scratch", "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := Service{Git: &fakeGit{}, Owner: "acme", WorktreeRoot: wtRoot}

	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no worktrees, got %+v", got)
	}
}

func TestList_MissingWorktreeRoot(t *testing.T) {
	s := Service{Git: &fakeGit{}, WorktreeRoot: filepath.Join(t.TempDir(), "does-not-exist")}
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List of a missing root should not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d, want 0", len(got))
	}
}

// mkdirAll creates the joined directory path, failing the test on error.
func mkdirAll(t *testing.T, parts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(parts...), 0o755); err != nil {
		t.Fatal(err)
	}
}
