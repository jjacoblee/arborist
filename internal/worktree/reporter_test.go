package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jjacoblee/arborist/internal/github"
)

// recordReporter records the progress lifecycle for assertions.
type recordReporter struct {
	total   int
	steps   []string
	started int
	done    int
	stopped int
}

func (r *recordReporter) Start(total int)   { r.started++; r.total = total }
func (r *recordReporter) Step(label string) { r.steps = append(r.steps, label) }
func (r *recordReporter) Done()             { r.done++ }
func (r *recordReporter) Stop()             { r.stopped++ }

func TestCreate_ReportsProgress(t *testing.T) {
	wsRoot := t.TempDir()
	for _, name := range []string{"web", "api"} {
		if err := os.MkdirAll(filepath.Join(wsRoot, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := Service{Git: &fakeGit{}, WorkspaceRoot: wsRoot, WorktreeRoot: t.TempDir()}
	rep := &recordReporter{}
	s.Progress = rep

	s.Create(context.Background(), "feature/x", "", []github.Repository{
		{Name: "web", NameWithOwner: "acme/web"},
		{Name: "api", NameWithOwner: "acme/api"},
	})

	assertLifecycle(t, rep, 2, []string{"acme/web", "acme/api"})
}

func TestRemove_ReportsProgress(t *testing.T) {
	s, _ := serviceWithWorktrees(t, &fakeGit{})
	rep := &recordReporter{}
	s.Progress = rep

	s.Remove(context.Background(), []ManagedWorktree{
		{Owner: "acme", Repo: "web", Branch: "feature/x", Path: "/wt/x", RepoPath: "/repos/acme/web"},
		{Owner: "acme", Repo: "api", Branch: "feature/x", Path: "/wt/y", RepoPath: "/repos/acme/api"},
	}, false)

	assertLifecycle(t, rep, 2, []string{"acme/web", "acme/api"})
}

// TestRemove_ReportsProgressForSkippedWorktree guards the removeOne refactor: a
// dirty worktree is skipped rather than removed, but the indicator must still
// advance for it. The skip path used to `continue`, which — once progress moved
// to a per-completion Done — would have left the bar one short of full.
func TestRemove_ReportsProgressForSkippedWorktree(t *testing.T) {
	s, _ := serviceWithWorktrees(t, &fakeGit{})
	rep := &recordReporter{}
	s.Progress = rep

	res := s.Remove(context.Background(), []ManagedWorktree{
		{Owner: "acme", Repo: "web", Branch: "feature/x", Path: "/wt/x", RepoPath: "/repos/acme/web", Dirty: true},
	}, false) // not forced, so the dirty worktree is skipped

	if len(res.Skipped) != 1 {
		t.Fatalf("expected the dirty worktree to be skipped, got %+v", res)
	}
	// One Step and one Done, even though the item was skipped rather than removed.
	assertLifecycle(t, rep, 1, []string{"acme/web"})
}

func TestPrune_ReportsProgress(t *testing.T) {
	wsRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wsRoot, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := Service{
		Git:           &fakeGit{IsRepoFn: func(string) bool { return true }},
		WorkspaceRoot: wsRoot,
		WorktreeRoot:  filepath.Join(wsRoot, "worktrees"),
	}
	rep := &recordReporter{}
	s.Progress = rep

	if _, err := s.Prune(context.Background()); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	assertLifecycle(t, rep, 1, []string{"web"})
}

func assertLifecycle(t *testing.T, rep *recordReporter, wantTotal int, wantSteps []string) {
	t.Helper()
	if rep.started != 1 || rep.stopped != 1 {
		t.Fatalf("expected one Start and one Stop, got started=%d stopped=%d", rep.started, rep.stopped)
	}
	if rep.total != wantTotal {
		t.Fatalf("Start total = %d, want %d", rep.total, wantTotal)
	}
	if len(rep.steps) != len(wantSteps) {
		t.Fatalf("steps = %v, want %v", rep.steps, wantSteps)
	}
	// Every started item must also be reported Done, so the indicator advances
	// once per completed item rather than reading ahead of the work.
	if rep.done != len(wantSteps) {
		t.Fatalf("Done calls = %d, want %d (one per item)", rep.done, len(wantSteps))
	}
	for i, s := range wantSteps {
		if rep.steps[i] != s {
			t.Fatalf("step %d = %q, want %q (all: %v)", i, rep.steps[i], s, rep.steps)
		}
	}
}
