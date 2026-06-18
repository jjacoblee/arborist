package git

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
)

func lastArgs(f *exectest.Fake) []string {
	if len(f.Calls) == 0 {
		return nil
	}
	return f.Calls[len(f.Calls)-1].Args
}

func TestIsRepo(t *testing.T) {
	ok := &exectest.Fake{}
	if !New(ok).IsRepo(context.Background(), "/repo") {
		t.Fatal("IsRepo should be true when rev-parse succeeds")
	}
	wantArgs := []string{"-C", "/repo", "rev-parse", "--is-inside-work-tree"}
	if got := lastArgs(ok); !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("args = %v, want %v", got, wantArgs)
	}

	bad := &exectest.Fake{Default: exectest.Result{Err: errors.New("not a repo")}}
	if New(bad).IsRepo(context.Background(), "/x") {
		t.Fatal("IsRepo should be false when rev-parse fails")
	}
}

func TestFetch(t *testing.T) {
	f := &exectest.Fake{}
	if err := New(f).Fetch(context.Background(), "/repo"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	want := []string{"-C", "/repo", "fetch", "--all", "--prune"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestDefaultBranch(t *testing.T) {
	f := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", "/repo", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"): {Out: []byte("origin/main\n")},
		},
	}
	got, err := New(f).DefaultBranch(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "main" {
		t.Fatalf("DefaultBranch = %q, want main", got)
	}

	bad := &exectest.Fake{Default: exectest.Result{Err: errors.New("no HEAD")}}
	if _, err := New(bad).DefaultBranch(context.Background(), "/repo"); err == nil {
		t.Fatal("DefaultBranch should error when origin/HEAD is unavailable")
	}
}

func TestSetOriginHead(t *testing.T) {
	f := &exectest.Fake{}
	if err := New(f).SetOriginHead(context.Background(), "/repo"); err != nil {
		t.Fatalf("SetOriginHead: %v", err)
	}
	want := []string{"-C", "/repo", "remote", "set-head", "origin", "--auto"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestMainRepoPath(t *testing.T) {
	f := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", "/wt/web-feature-x", "rev-parse", "--path-format=absolute", "--git-common-dir"): {Out: []byte("/repos/acme/web/.git\n")},
		},
	}
	got, err := New(f).MainRepoPath(context.Background(), "/wt/web-feature-x")
	if err != nil {
		t.Fatalf("MainRepoPath: %v", err)
	}
	if got != "/repos/acme/web" {
		t.Fatalf("MainRepoPath = %q, want /repos/acme/web", got)
	}

	bad := &exectest.Fake{Default: exectest.Result{Err: errors.New("not a repo")}}
	if _, err := New(bad).MainRepoPath(context.Background(), "/x"); err == nil {
		t.Fatal("MainRepoPath should error when git fails")
	}
}

func TestBranchExists(t *testing.T) {
	f := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", "/repo", "rev-parse", "--verify", "--quiet", "refs/heads/main"):          {},
			exectest.Key("git", "-C", "/repo", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/feat"): {},
		},
		Default: exectest.Result{Err: errors.New("unknown ref")},
	}
	c := New(f)
	if !c.LocalBranchExists(context.Background(), "/repo", "main") {
		t.Fatal("LocalBranchExists(main) should be true")
	}
	if c.LocalBranchExists(context.Background(), "/repo", "nope") {
		t.Fatal("LocalBranchExists(nope) should be false")
	}
	if !c.RemoteBranchExists(context.Background(), "/repo", "feat") {
		t.Fatal("RemoteBranchExists(feat) should be true")
	}
	if c.RemoteBranchExists(context.Background(), "/repo", "missing") {
		t.Fatal("RemoteBranchExists(missing) should be false")
	}
}

func TestAddWorktree_ExistingBranch(t *testing.T) {
	f := &exectest.Fake{}
	err := New(f).AddWorktree(context.Background(), "/repo", WorktreeAddOptions{
		Path:   "/wt/feature",
		Branch: "feature/x",
	})
	if err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	want := []string{"-C", "/repo", "worktree", "add", "/wt/feature", "feature/x"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestAddWorktree_NewBranchWithBase(t *testing.T) {
	f := &exectest.Fake{}
	err := New(f).AddWorktree(context.Background(), "/repo", WorktreeAddOptions{
		Path:      "/wt/feature",
		Branch:    "feature/x",
		CreateNew: true,
		BaseRef:   "main",
	})
	if err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	want := []string{"-C", "/repo", "worktree", "add", "-b", "feature/x", "/wt/feature", "main"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestAddWorktree_NewBranchNoBase(t *testing.T) {
	f := &exectest.Fake{}
	err := New(f).AddWorktree(context.Background(), "/repo", WorktreeAddOptions{
		Path:      "/wt/feature",
		Branch:    "feature/x",
		CreateNew: true,
	})
	if err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	want := []string{"-C", "/repo", "worktree", "add", "-b", "feature/x", "/wt/feature"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestCurrentBranch(t *testing.T) {
	f := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", "/wt", "branch", "--show-current"): {Out: []byte("feature/x\n")},
		},
	}
	got, err := New(f).CurrentBranch(context.Background(), "/wt")
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "feature/x" {
		t.Fatalf("CurrentBranch = %q, want feature/x", got)
	}

	// Detached HEAD: empty output, no error.
	det := &exectest.Fake{}
	if b, err := New(det).CurrentBranch(context.Background(), "/wt"); err != nil || b != "" {
		t.Fatalf("detached: got (%q, %v), want (\"\", nil)", b, err)
	}
}

func TestIsDirty(t *testing.T) {
	dirty := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", "/wt", "status", "--porcelain"): {Out: []byte(" M file.go\n?? new.txt\n")},
		},
	}
	if d, err := New(dirty).IsDirty(context.Background(), "/wt"); err != nil || !d {
		t.Fatalf("dirty: got (%v, %v), want (true, nil)", d, err)
	}

	clean := &exectest.Fake{} // empty output
	if d, err := New(clean).IsDirty(context.Background(), "/wt"); err != nil || d {
		t.Fatalf("clean: got (%v, %v), want (false, nil)", d, err)
	}
}

func TestRemoveWorktree(t *testing.T) {
	f := &exectest.Fake{}
	if err := New(f).RemoveWorktree(context.Background(), "/repo", "/wt/feature", false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	want := []string{"-C", "/repo", "worktree", "remove", "/wt/feature"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}

	ff := &exectest.Fake{}
	if err := New(ff).RemoveWorktree(context.Background(), "/repo", "/wt/feature", true); err != nil {
		t.Fatalf("RemoveWorktree force: %v", err)
	}
	wantForce := []string{"-C", "/repo", "worktree", "remove", "--force", "/wt/feature"}
	if got := lastArgs(ff); !reflect.DeepEqual(got, wantForce) {
		t.Fatalf("force args = %v, want %v", got, wantForce)
	}
}

func TestPruneWorktrees(t *testing.T) {
	f := &exectest.Fake{}
	if err := New(f).PruneWorktrees(context.Background(), "/repo"); err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}
	want := []string{"-C", "/repo", "worktree", "prune"}
	if got := lastArgs(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestListWorktrees_Parse(t *testing.T) {
	porcelain := "worktree /repos/acme/web\n" +
		"HEAD abc123\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree /wt/acme/web/feature-x\n" +
		"HEAD def456\n" +
		"branch refs/heads/feature/x\n" +
		"\n" +
		"worktree /wt/acme/web/detached\n" +
		"HEAD 999000\n" +
		"detached\n"

	f := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "-C", "/repos/acme/web", "worktree", "list", "--porcelain"): {Out: []byte(porcelain)},
		},
	}
	got, err := New(f).ListWorktrees(context.Background(), "/repos/acme/web")
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	want := []Worktree{
		{Path: "/repos/acme/web", Head: "abc123", Branch: "main"},
		{Path: "/wt/acme/web/feature-x", Head: "def456", Branch: "feature/x"},
		{Path: "/wt/acme/web/detached", Head: "999000", Detached: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListWorktrees:\n got = %+v\nwant = %+v", got, want)
	}
}
