package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/config"
	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/exectest"
	"github.com/jjacoblee/arborist/internal/worktree"
)

func runOpen(t *testing.T, fake *exectest.Fake, launcher *exectest.FakeLauncher, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: fake, launcher: launcher})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// openFixture builds a workspace with one worktree (branch "feature/x") and the
// given editor config, returning the dir, the worktree path, and a wired runner.
func openFixture(t *testing.T, editor string) (dir, wtPath string, fake *exectest.Fake) {
	t.Helper()
	dir = t.TempDir()
	if err := config.Save(config.ConfigPath(dir), config.Config{Owner: "acme", Editor: editor}); err != nil {
		t.Fatal(err)
	}
	wtPath = filepath.Join(dir, "worktrees", "web", "feature-x")
	mkWorktree(t, wtPath)
	base := filepath.Join(dir, "web")
	fake = &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", wtPath, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(base + "/.git\n")},
			exectest.Key("git", "-C", wtPath, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", wtPath, "status", "--porcelain"):                                   {},
		},
	}
	return dir, wtPath, fake
}

func lastArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}

func TestOpen_ByID_Cursor(t *testing.T) {
	dir, wtPath, fake := openFixture(t, "")
	launcher := &exectest.FakeLauncher{}
	id := worktree.ID(wtPath)

	out, err := runOpen(t, fake, launcher, "open", id[:6], "--cursor", "--dir", dir)
	if err != nil {
		t.Fatalf("open: %v\n%s", err, out)
	}
	if len(launcher.Calls) != 1 {
		t.Fatalf("expected one launch, got %+v", launcher.Calls)
	}
	c := launcher.Calls[0]
	if c.Name != "cursor" || lastArg(c.Args) != wtPath {
		t.Fatalf("launched %s %v, want cursor ... %s", c.Name, c.Args, wtPath)
	}
}

func TestOpen_UsesConfigEditorWithArgs(t *testing.T) {
	dir, wtPath, fake := openFixture(t, "code --wait")
	launcher := &exectest.FakeLauncher{}
	id := worktree.ID(wtPath)

	if _, err := runOpen(t, fake, launcher, "open", id[:6], "--dir", dir); err != nil {
		t.Fatalf("open: %v", err)
	}
	c := launcher.Calls[0]
	if c.Name != "code" || len(c.Args) != 2 || c.Args[0] != "--wait" || c.Args[1] != wtPath {
		t.Fatalf("launched %s %v, want code [--wait %s]", c.Name, c.Args, wtPath)
	}
}

func TestOpen_Print_DoesNotLaunch(t *testing.T) {
	dir, wtPath, fake := openFixture(t, "cursor")
	launcher := &exectest.FakeLauncher{}
	id := worktree.ID(wtPath)

	out, err := runOpen(t, fake, launcher, "open", id[:6], "--print", "--dir", dir)
	if err != nil {
		t.Fatalf("open --print: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != wtPath {
		t.Fatalf("print output = %q, want %q", strings.TrimSpace(out), wtPath)
	}
	if len(launcher.Calls) != 0 {
		t.Fatalf("--print must not launch an editor, got %+v", launcher.Calls)
	}
}

func TestOpen_EditorNotFound(t *testing.T) {
	dir, wtPath, fake := openFixture(t, "")
	launcher := &exectest.FakeLauncher{Err: exec.ErrNotFound}
	id := worktree.ID(wtPath)

	_, err := runOpen(t, fake, launcher, "open", id[:6], "--cursor", "--dir", dir)
	if err == nil || !strings.Contains(err.Error(), "not found on your PATH") {
		t.Fatalf("expected a missing-editor error, got: %v", err)
	}
}

func TestOpen_NoEditorConfigured(t *testing.T) {
	dir, wtPath, fake := openFixture(t, "")
	t.Setenv("EDITOR", "") // no fallback
	launcher := &exectest.FakeLauncher{}
	id := worktree.ID(wtPath)

	_, err := runOpen(t, fake, launcher, "open", id[:6], "--dir", dir)
	if err == nil || !strings.Contains(err.Error(), "no editor configured") {
		t.Fatalf("expected a no-editor error, got: %v", err)
	}
	if len(launcher.Calls) != 0 {
		t.Fatal("must not launch when no editor is resolved")
	}
}

func TestOpen_NotFound(t *testing.T) {
	dir, _, fake := openFixture(t, "cursor")
	launcher := &exectest.FakeLauncher{}

	_, err := runOpen(t, fake, launcher, "open", "does-not-exist", "--dir", dir)
	if err == nil || !strings.Contains(err.Error(), "no worktree found") {
		t.Fatalf("expected a not-found error, got: %v", err)
	}
}

func TestOpen_AmbiguousBranch(t *testing.T) {
	dir := t.TempDir()
	if err := config.Save(config.ConfigPath(dir), config.Config{Owner: "acme", Editor: "cursor"}); err != nil {
		t.Fatal(err)
	}
	webWt := filepath.Join(dir, "worktrees", "web", "feature-x")
	apiWt := filepath.Join(dir, "worktrees", "api", "feature-x")
	mkWorktree(t, webWt)
	mkWorktree(t, apiWt)

	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", webWt, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(filepath.Join(dir, "web") + "/.git\n")},
			exectest.Key("git", "-C", apiWt, "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte(filepath.Join(dir, "api") + "/.git\n")},
			exectest.Key("git", "-C", webWt, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", apiWt, "branch", "--show-current"):                                {Out: []byte("feature/x\n")},
			exectest.Key("git", "-C", webWt, "status", "--porcelain"):                                   {},
			exectest.Key("git", "-C", apiWt, "status", "--porcelain"):                                   {},
		},
	}
	launcher := &exectest.FakeLauncher{}

	out, err := runOpen(t, fake, launcher, "open", "feature/x", "--dir", dir)
	if err == nil || !strings.Contains(err.Error(), "matches 2 worktrees") {
		t.Fatalf("expected an ambiguous-branch error, got: %v\n%s", err, out)
	}
	if len(launcher.Calls) != 0 {
		t.Fatal("must not open when the branch is ambiguous")
	}
}
