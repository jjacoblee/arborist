package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
)

// fakeGit is a configurable Git for tests. Unset hooks use safe defaults
// (success, branch-absent, repo-valid).
type fakeGit struct {
	IsRepoFn        func(path string) bool
	FetchFn         func(repoPath string) error
	SetOriginHeadFn func(repoPath string) error
	DefaultBranchFn func(repoPath string) (string, error)
	LocalExistsFn   func(repoPath, branch string) bool
	RemoteExistsFn  func(repoPath, branch string) bool
	AddWorktreeFn   func(repoPath string, opts git.WorktreeAddOptions) error
	ListFn          func(repoPath string) ([]git.Worktree, error)
	CurrentBranchFn func(path string) (string, error)
	IsDirtyFn       func(path string) (bool, error)
	MainRepoPathFn  func(path string) (string, error)
	RemoveFn        func(repoPath, worktreePath string, force bool) error
	PruneFn         func(repoPath string) error

	Added   []git.WorktreeAddOptions
	Removed []string
	Pruned  []string
}

func (f *fakeGit) IsRepo(_ context.Context, path string) bool {
	if f.IsRepoFn != nil {
		return f.IsRepoFn(path)
	}
	return true
}
func (f *fakeGit) Fetch(_ context.Context, repoPath string) error {
	if f.FetchFn != nil {
		return f.FetchFn(repoPath)
	}
	return nil
}
func (f *fakeGit) SetOriginHead(_ context.Context, repoPath string) error {
	if f.SetOriginHeadFn != nil {
		return f.SetOriginHeadFn(repoPath)
	}
	return nil
}
func (f *fakeGit) DefaultBranch(_ context.Context, repoPath string) (string, error) {
	if f.DefaultBranchFn != nil {
		return f.DefaultBranchFn(repoPath)
	}
	return "main", nil
}
func (f *fakeGit) LocalBranchExists(_ context.Context, repoPath, branch string) bool {
	if f.LocalExistsFn != nil {
		return f.LocalExistsFn(repoPath, branch)
	}
	return false
}
func (f *fakeGit) RemoteBranchExists(_ context.Context, repoPath, branch string) bool {
	if f.RemoteExistsFn != nil {
		return f.RemoteExistsFn(repoPath, branch)
	}
	return false
}
func (f *fakeGit) AddWorktree(_ context.Context, repoPath string, opts git.WorktreeAddOptions) error {
	f.Added = append(f.Added, opts)
	if f.AddWorktreeFn != nil {
		return f.AddWorktreeFn(repoPath, opts)
	}
	return nil
}
func (f *fakeGit) ListWorktrees(_ context.Context, repoPath string) ([]git.Worktree, error) {
	if f.ListFn != nil {
		return f.ListFn(repoPath)
	}
	return nil, nil
}
func (f *fakeGit) CurrentBranch(_ context.Context, path string) (string, error) {
	if f.CurrentBranchFn != nil {
		return f.CurrentBranchFn(path)
	}
	return "", nil
}
func (f *fakeGit) IsDirty(_ context.Context, path string) (bool, error) {
	if f.IsDirtyFn != nil {
		return f.IsDirtyFn(path)
	}
	return false, nil
}
func (f *fakeGit) MainRepoPath(_ context.Context, path string) (string, error) {
	if f.MainRepoPathFn != nil {
		return f.MainRepoPathFn(path)
	}
	return "", nil
}
func (f *fakeGit) RemoveWorktree(_ context.Context, repoPath, worktreePath string, force bool) error {
	f.Removed = append(f.Removed, worktreePath)
	if f.RemoveFn != nil {
		return f.RemoveFn(repoPath, worktreePath, force)
	}
	return nil
}
func (f *fakeGit) PruneWorktrees(_ context.Context, repoPath string) error {
	f.Pruned = append(f.Pruned, repoPath)
	if f.PruneFn != nil {
		return f.PruneFn(repoPath)
	}
	return nil
}

// fakeCloner records clone destinations and can be scripted to fail.
type fakeCloner struct {
	Fn     func(nameWithOwner, dest string) error
	Cloned []string
}

func (f *fakeCloner) CloneRepo(_ context.Context, nameWithOwner, dest string) error {
	f.Cloned = append(f.Cloned, dest)
	if f.Fn != nil {
		return f.Fn(nameWithOwner, dest)
	}
	return nil
}

func testRepo() github.Repository {
	return github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
}

// newService returns a Service rooted in two fresh temp dirs.
func newService(t *testing.T, g Git, c Cloner) Service {
	t.Helper()
	return Service{
		Git:           g,
		Cloner:        c,
		Owner:         "acme",
		WorkspaceRoot: t.TempDir(),
		WorktreeRoot:  t.TempDir(),
	}
}

func TestCreate_NewBranchFromDefault_ClonesMissingRepo(t *testing.T) {
	g := &fakeGit{}
	c := &fakeCloner{}
	s := newService(t, g, c)

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Created) != 1 || len(res.Failed) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	created := res.Created[0]
	if created.Source != SourceNewBranch {
		t.Fatalf("source = %q, want %q", created.Source, SourceNewBranch)
	}
	wantPath := filepath.Join(s.WorktreeRoot, "web", "feature-x")
	if created.Path != wantPath {
		t.Fatalf("path = %q, want %q", created.Path, wantPath)
	}
	if len(c.Cloned) != 1 {
		t.Fatalf("expected a clone, got %v", c.Cloned)
	}
	if len(g.Added) != 1 || !g.Added[0].CreateNew || g.Added[0].BaseRef != "main" {
		t.Fatalf("AddWorktree opts = %+v, want CreateNew from main", g.Added)
	}
}

func TestCreate_ExistingLocalBranch_ReusesRepo(t *testing.T) {
	g := &fakeGit{LocalExistsFn: func(_, _ string) bool { return true }}
	c := &fakeCloner{}
	s := newService(t, g, c)
	mkRepoDir(t, s.WorkspaceRoot, "web") // repo already cloned locally

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Created) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Created[0].Source != SourceLocalBranch {
		t.Fatalf("source = %q, want %q", res.Created[0].Source, SourceLocalBranch)
	}
	if len(c.Cloned) != 0 {
		t.Fatalf("should reuse the existing repo, but cloned %v", c.Cloned)
	}
	if g.Added[0].CreateNew {
		t.Fatalf("existing local branch must not use -b: %+v", g.Added[0])
	}
}

func TestCreate_BaseOverride_LocalBase(t *testing.T) {
	// New branch "brad/new" doesn't exist; the base "feature-y" exists locally.
	g := &fakeGit{LocalExistsFn: func(_, branch string) bool { return branch == "feature-y" }}
	s := newService(t, g, &fakeCloner{})
	s.Base = "feature-y"
	mkRepoDir(t, s.WorkspaceRoot, "web")

	res := s.Create(context.Background(), "brad/new", "", []github.Repository{testRepo()})
	if len(res.Created) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(g.Added) != 1 || !g.Added[0].CreateNew || g.Added[0].BaseRef != "feature-y" {
		t.Fatalf("AddWorktree opts = %+v, want new branch from feature-y", g.Added)
	}
	if !strings.Contains(string(res.Created[0].Source), "feature-y") {
		t.Fatalf("source = %q, want it to mention base feature-y", res.Created[0].Source)
	}
}

func TestCreate_BaseOverride_RemoteBase(t *testing.T) {
	// Base exists only on the remote -> resolved to origin/<base>.
	g := &fakeGit{RemoteExistsFn: func(_, branch string) bool { return branch == "feature-y" }}
	s := newService(t, g, &fakeCloner{})
	s.Base = "feature-y"
	mkRepoDir(t, s.WorkspaceRoot, "web")

	res := s.Create(context.Background(), "brad/new", "", []github.Repository{testRepo()})
	if len(res.Created) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if g.Added[0].BaseRef != "origin/feature-y" {
		t.Fatalf("BaseRef = %q, want origin/feature-y", g.Added[0].BaseRef)
	}
}

func TestCreate_RemoteTrackingBranch(t *testing.T) {
	g := &fakeGit{RemoteExistsFn: func(_, _ string) bool { return true }}
	s := newService(t, g, &fakeCloner{})
	mkRepoDir(t, s.WorkspaceRoot, "web")

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Created) != 1 || res.Created[0].Source != SourceRemoteTracking {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !g.Added[0].CreateNew || g.Added[0].BaseRef != "origin/feature/x" {
		t.Fatalf("opts = %+v, want tracking from origin/feature/x", g.Added[0])
	}
}

func TestCreate_WorktreePathExists_Skips(t *testing.T) {
	g := &fakeGit{}
	s := newService(t, g, &fakeCloner{})
	wt := filepath.Join(s.WorktreeRoot, "web", "feature-x")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Skipped) != 1 || len(res.Created) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !strings.Contains(res.Skipped[0].Reason, "already exists") {
		t.Fatalf("reason = %q", res.Skipped[0].Reason)
	}
	if len(g.Added) != 0 {
		t.Fatal("must not create a worktree when the path exists")
	}
}

func TestCreate_BranchAlreadyHasWorktree_Skips(t *testing.T) {
	existing := "/somewhere/acme/web/feature-x"
	g := &fakeGit{
		ListFn: func(_ string) ([]git.Worktree, error) {
			return []git.Worktree{{Path: existing, Branch: "feature/x"}}, nil
		},
	}
	s := newService(t, g, &fakeCloner{})
	mkRepoDir(t, s.WorkspaceRoot, "web")

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Skipped) != 1 || len(res.Created) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Skipped[0].Path != existing {
		t.Fatalf("skipped path = %q, want %q", res.Skipped[0].Path, existing)
	}
}

func TestCreate_PathExistsButNotRepo_Fails(t *testing.T) {
	g := &fakeGit{IsRepoFn: func(string) bool { return false }}
	s := newService(t, g, &fakeCloner{})
	mkRepoDir(t, s.WorkspaceRoot, "web")

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Failed) != 1 || len(res.Created) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestCreate_CloneFails(t *testing.T) {
	c := &fakeCloner{Fn: func(_, _ string) error { return errors.New("network down") }}
	s := newService(t, &fakeGit{}, c)

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{testRepo()})

	if len(res.Failed) != 1 || len(res.Created) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestCreate_PartialFailure(t *testing.T) {
	c := &fakeCloner{
		Fn: func(_, dest string) error {
			if strings.Contains(dest, "bad") {
				return errors.New("clone failed")
			}
			return nil
		},
	}
	s := newService(t, &fakeGit{}, c)

	good := testRepo()
	bad := github.Repository{Name: "bad", Owner: "acme", NameWithOwner: "acme/bad"}

	res := s.Create(context.Background(), "feature/x", "", []github.Repository{good, bad})

	if len(res.Created) != 1 || len(res.Failed) != 1 {
		t.Fatalf("unexpected result: created=%d failed=%d", len(res.Created), len(res.Failed))
	}
	if !res.HasFailures() {
		t.Fatal("HasFailures() should be true")
	}
	if res.Created[0].Repository.Name != "web" || res.Failed[0].Repository.Name != "bad" {
		t.Fatalf("mismatched outcomes: %+v", res)
	}
}

func TestCreate_CustomName_ShortensFolder(t *testing.T) {
	g := &fakeGit{}
	s := newService(t, g, &fakeCloner{})

	res := s.Create(context.Background(), "jacob/s26-11998-fe-build-onboarding-ui", "s26-11998",
		[]github.Repository{testRepo()})

	if len(res.Created) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	created := res.Created[0]
	// Folder uses the short name; the branch is still the full branch.
	if want := filepath.Join(s.WorktreeRoot, "web", "s26-11998"); created.Path != want {
		t.Fatalf("path = %q, want %q", created.Path, want)
	}
	if created.Branch != "jacob/s26-11998-fe-build-onboarding-ui" {
		t.Fatalf("branch = %q, want the full branch", created.Branch)
	}
	if len(g.Added) != 1 || g.Added[0].Branch != "jacob/s26-11998-fe-build-onboarding-ui" {
		t.Fatalf("git should create the full branch, got %+v", g.Added)
	}
}

func mkRepoDir(t *testing.T, workspaceRoot, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, repo), 0o755); err != nil {
		t.Fatal(err)
	}
}
