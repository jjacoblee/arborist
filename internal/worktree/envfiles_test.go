package worktree

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jjacoblee/arborist/internal/github"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCopyEnvFiles(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "wt") // not pre-created

	writeFile(t, filepath.Join(src, ".env"), "A=1")
	writeFile(t, filepath.Join(src, ".env.local"), "B=2")
	writeFile(t, filepath.Join(src, "README.md"), "x")     // not an env file
	writeFile(t, filepath.Join(src, "pkg", ".env"), "C=3") // nested, must not copy
	if err := os.Symlink(filepath.Join(src, ".env"), filepath.Join(src, ".env.link")); err != nil {
		t.Fatal(err) // symlink named like env, must not copy
	}

	copied, err := copyEnvFiles(src, dst)
	if err != nil {
		t.Fatalf("copyEnvFiles: %v", err)
	}
	sort.Strings(copied)
	if len(copied) != 2 || copied[0] != ".env" || copied[1] != ".env.local" {
		t.Fatalf("copied = %v, want [.env .env.local]", copied)
	}
	if got := readFile(t, filepath.Join(dst, ".env")); got != "A=1" {
		t.Fatalf(".env content = %q", got)
	}
	for _, name := range []string{"README.md", "pkg", ".env.link"} {
		if _, err := os.Stat(filepath.Join(dst, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should not have been copied", name)
		}
	}
	info, err := os.Stat(filepath.Join(dst, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 600 (env files are private)", info.Mode().Perm())
	}
}

func TestCopyEnvFiles_MissingSrcIsNoOp(t *testing.T) {
	copied, err := copyEnvFiles(filepath.Join(t.TempDir(), "nope"), t.TempDir())
	if err != nil || len(copied) != 0 {
		t.Fatalf("missing src should be a no-op: copied=%v err=%v", copied, err)
	}
}

func TestCreate_CopiesEnvFiles(t *testing.T) {
	g := &fakeGit{LocalExistsFn: func(_, _ string) bool { return true }}
	s := newService(t, g, &fakeCloner{})
	s.CopyEnvFiles = true

	// Existing base clone with an .env.
	mkdirAll(t, s.WorkspaceRoot, "web")
	writeFile(t, filepath.Join(s.WorkspaceRoot, "web", ".env"), "SECRET=1")

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})
	if len(res.Created) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(res.Created[0].CopiedEnv) != 1 || res.Created[0].CopiedEnv[0] != ".env" {
		t.Fatalf("CopiedEnv = %v, want [.env]", res.Created[0].CopiedEnv)
	}
	if got := readFile(t, filepath.Join(res.Created[0].Path, ".env")); got != "SECRET=1" {
		t.Fatalf("worktree .env = %q, want SECRET=1", got)
	}
}

func TestCopyExtraFiles(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "wt") // not pre-created

	writeFile(t, filepath.Join(src, "secrets.env"), "S=1")
	writeFile(t, filepath.Join(src, "config", "local.json"), "{}") // nested
	if err := os.Symlink(filepath.Join(src, "secrets.env"), filepath.Join(src, "link.env")); err != nil {
		t.Fatal(err)
	}

	copied, err := copyExtraFiles(src, dst, []string{
		"secrets.env", "config/local.json", "missing.txt", "../escape", "link.env",
	})
	if err != nil {
		t.Fatalf("copyExtraFiles: %v", err)
	}
	sort.Strings(copied)
	if len(copied) != 2 || copied[0] != "config/local.json" || copied[1] != "secrets.env" {
		t.Fatalf("copied = %v, want [config/local.json secrets.env]", copied)
	}
	if got := readFile(t, filepath.Join(dst, "secrets.env")); got != "S=1" {
		t.Fatalf("secrets.env = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "config", "local.json")); err != nil {
		t.Fatalf("nested file should be copied: %v", err)
	}
	// The escaping path must not have written anything outside dst.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "escape")); !os.IsNotExist(err) {
		t.Fatal("'..' path must not escape the worktree")
	}
	info, err := os.Stat(filepath.Join(dst, "secrets.env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 600 (copies are private)", info.Mode().Perm())
	}
}

func TestCreate_CopiesExtraFiles(t *testing.T) {
	g := &fakeGit{LocalExistsFn: func(_, _ string) bool { return true }}
	s := newService(t, g, &fakeCloner{})
	s.CopyFiles = []string{"secrets.env"}

	mkdirAll(t, s.WorkspaceRoot, "web")
	writeFile(t, filepath.Join(s.WorkspaceRoot, "web", "secrets.env"), "TOKEN=1")

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})
	if len(res.Created) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if got := res.Created[0].CopiedFiles; len(got) != 1 || got[0] != "secrets.env" {
		t.Fatalf("CopiedFiles = %v, want [secrets.env]", got)
	}
	if got := readFile(t, filepath.Join(res.Created[0].Path, "secrets.env")); got != "TOKEN=1" {
		t.Fatalf("worktree secrets.env = %q, want TOKEN=1", got)
	}
}
