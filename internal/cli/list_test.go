package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
)

// mkWorktree creates a worktree directory under a workspace's worktree root,
// marked with a ".git" file so the scanner treats it as a worktree root.
func mkWorktree(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestList_PrintsWorktrees(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	wtPath := filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x")
	mkWorktree(t, wtPath)
	baseRepo := filepath.Join(dir, "web")

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   {}, // clean
		},
	}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir)
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	for _, want := range []string{"ID", "REPOSITORY", "acme/web", "feature/x", "web/feature-x"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	// Path is shown relative to the worktree root, not as an absolute path.
	if strings.Contains(out, workspaceWorktreeRoot(dir)) {
		t.Fatalf("expected a relative path, but output has the absolute root:\n%s", out)
	}
	// Header plus exactly one worktree row.
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 2 {
		t.Fatalf("expected header + one row, got %d lines:\n%s", len(lines), out)
	}
}

func TestList_FullPaths(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	wtPath := filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x")
	mkWorktree(t, wtPath)
	baseRepo := filepath.Join(dir, "web")

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   {},
		},
	}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir, "--full")
	if err != nil {
		t.Fatalf("list --full: %v\n%s", err, out)
	}
	if !strings.Contains(out, wtPath) {
		t.Fatalf("--full should show the absolute path %q:\n%s", wtPath, out)
	}
}

func TestList_Empty(t *testing.T) {
	dir := writeWorkspace(t, "acme") // no worktrees created
	fake := &exectest.Fake{}
	out, err := runRootWithRunner(t, fake, "list", "--dir", dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "No worktrees found.") {
		t.Fatalf("expected empty message, got:\n%s", out)
	}
}
