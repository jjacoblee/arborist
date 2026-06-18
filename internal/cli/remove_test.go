package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
	"github.com/jjacoblee/arborist/internal/pickertest"
	"github.com/jjacoblee/arborist/internal/worktree"
)

func runRemove(t *testing.T, fake *exectest.Fake, conf *pickertest.FakeConfirmer, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: fake, confirmer: conf})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// removeFixture sets up a workspace with one worktree for branch "feature/x"
// and returns the workspace dir, the worktree path, and a runner pre-wired for
// it.
func removeFixture(t *testing.T, dirty bool) (dir, wtPath string, fake *exectest.Fake) {
	t.Helper()
	dir = writeWorkspace(t, "acme")
	wtPath = filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x") // nested layout
	mkWorktree(t, wtPath)
	baseRepo := filepath.Join(dir, "web")

	status := exectest.Result{} // clean
	if dirty {
		status = exectest.Result{Out: []byte(" M file.go\n")}
	}
	fake = &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(baseRepo + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   status,
		},
	}
	return dir, wtPath, fake
}

func calledWorktreeRemove(fake *exectest.Fake) bool {
	for _, c := range fake.Calls {
		if c.Name == "git" && len(c.Args) >= 4 && c.Args[2] == "worktree" && c.Args[3] == "remove" {
			return true
		}
	}
	return false
}

func TestRemove_Confirmed(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	if conf.Calls != 1 {
		t.Fatalf("expected one confirmation prompt, got %d", conf.Calls)
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("expected a removal summary, got:\n%s", out)
	}
	if !calledWorktreeRemove(fake) {
		t.Fatalf("expected git worktree remove to be called; calls: %+v", fake.Calls)
	}
}

func TestRemove_ByID(t *testing.T) {
	dir, wtPath, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true}

	id := worktree.ID(wtPath)
	out, err := runRemove(t, fake, conf, "remove", id[:6], "--dir", dir)
	if err != nil {
		t.Fatalf("remove by id: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("expected a removal summary, got:\n%s", out)
	}
	if !calledWorktreeRemove(fake) {
		t.Fatalf("expected git worktree remove; calls: %+v", fake.Calls)
	}
}

func TestRemove_Aborted(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: false}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "Aborted") {
		t.Fatalf("expected an abort message, got:\n%s", out)
	}
	if calledWorktreeRemove(fake) {
		t.Fatal("must not remove anything when the user declines")
	}
}

func TestRemove_NoMatches(t *testing.T) {
	dir, _, fake := removeFixture(t, false)
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "does/not-exist", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "No worktrees found") {
		t.Fatalf("expected a no-matches message, got:\n%s", out)
	}
	if conf.Calls != 0 {
		t.Fatal("should not prompt when there are no matches")
	}
}

func TestRemove_DirtySkippedWithoutForce(t *testing.T) {
	dir, _, fake := removeFixture(t, true) // dirty
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "uncommitted changes") {
		t.Fatalf("expected a dirty warning, got:\n%s", out)
	}
	if conf.Calls != 0 {
		t.Fatal("should not prompt when nothing is removable")
	}
	if calledWorktreeRemove(fake) {
		t.Fatal("must never remove a dirty worktree without --force")
	}
}

func TestRemove_DirtyWithForceAndYes(t *testing.T) {
	dir, _, fake := removeFixture(t, true) // dirty
	conf := &pickertest.FakeConfirmer{}    // unused due to --yes

	out, err := runRemove(t, fake, conf, "remove", "feature/x", "--dir", dir, "--force", "--yes")
	if err != nil {
		t.Fatalf("remove --force --yes: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatal("--yes should skip the confirmation prompt")
	}
	if !calledWorktreeRemove(fake) {
		t.Fatal("expected removal with --force")
	}
	if !strings.Contains(out, "Removed acme/web") {
		t.Fatalf("expected removal summary, got:\n%s", out)
	}
}

func TestPrune(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &exectest.Fake{} // Default success: git --version, IsRepo true, prune ok

	out, err := runRemove(t, fake, &pickertest.FakeConfirmer{}, "prune", "--dir", dir)
	if err != nil {
		t.Fatalf("prune: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Pruned 1 repository") {
		t.Fatalf("expected prune summary, got:\n%s", out)
	}
}
