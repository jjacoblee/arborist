package worktree

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jjacoblee/arborist/internal/github"
)

// recordReporter records the progress lifecycle for assertions.
type recordReporter struct {
	total   int
	steps   []string
	events  []string // Log/Note/Fail lines, prefixed with their kind
	started int
	done    int
	stopped int
}

func (r *recordReporter) Start(total int)   { r.started++; r.total = total }
func (r *recordReporter) Step(label string) { r.steps = append(r.steps, label) }
func (r *recordReporter) Log(event string)  { r.events = append(r.events, "log: "+event) }
func (r *recordReporter) Note(event string) { r.events = append(r.events, "note: "+event) }
func (r *recordReporter) Fail(event string) { r.events = append(r.events, "fail: "+event) }
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

	assertLifecycle(t, rep, 2, 2)
	// Steps narrate the phases per repository: the item itself, then each
	// long-running action in flight.
	wantSteps := []string{
		"acme/web", "fetching acme/web", "creating worktree web/feature/x",
		"acme/api", "fetching acme/api", "creating worktree api/feature/x",
	}
	if !reflect.DeepEqual(rep.steps, wantSteps) {
		t.Fatalf("steps = %v, want %v", rep.steps, wantSteps)
	}
	// Completed actions become permanent log events — the ✓ lines.
	wantEvents := []string{
		"log: acme/web already cloned", "log: created worktree web/feature/x",
		"log: acme/api already cloned", "log: created worktree api/feature/x",
	}
	if !reflect.DeepEqual(rep.events, wantEvents) {
		t.Fatalf("events = %v, want %v", rep.events, wantEvents)
	}
}

func TestRemove_ReportsProgress(t *testing.T) {
	s, _ := serviceWithWorktrees(t, &fakeGit{})
	rep := &recordReporter{}
	s.Progress = rep

	s.Remove(context.Background(), []ManagedWorktree{
		{Owner: "acme", Repo: "web", Branch: "feature/x", Path: "/wt/x", RepoPath: "/repos/acme/web"},
		{Owner: "acme", Repo: "api", Branch: "feature/x", Path: "/wt/y", RepoPath: "/repos/acme/api"},
	}, false)

	assertLifecycle(t, rep, 2, 2)
	wantSteps := []string{"removing worktree web/x", "removing worktree api/y"}
	if !reflect.DeepEqual(rep.steps, wantSteps) {
		t.Fatalf("steps = %v, want %v", rep.steps, wantSteps)
	}
	wantEvents := []string{"log: removed worktree web/x", "log: removed worktree api/y"}
	if !reflect.DeepEqual(rep.events, wantEvents) {
		t.Fatalf("events = %v, want %v", rep.events, wantEvents)
	}
}

// TestRemove_ReportsProgressForSkippedWorktree guards the removeOne refactor: a
// dirty worktree is skipped rather than removed, but the indicator must still
// advance for it — and the skip surfaces as a Note event, never a silent drop.
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
	assertLifecycle(t, rep, 1, 1)
	wantEvents := []string{"note: skipped web/x — has uncommitted changes"}
	if !reflect.DeepEqual(rep.events, wantEvents) {
		t.Fatalf("events = %v, want %v", rep.events, wantEvents)
	}
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

	assertLifecycle(t, rep, 1, 1)
	if !reflect.DeepEqual(rep.steps, []string{"pruning web"}) {
		t.Fatalf("steps = %v, want [pruning web]", rep.steps)
	}
	if !reflect.DeepEqual(rep.events, []string{"log: pruned web"}) {
		t.Fatalf("events = %v, want [log: pruned web]", rep.events)
	}
}

// assertLifecycle checks the Start/Done/Stop bookkeeping: one Start with the
// item total, one Done per completed item (so an indicator never reads ahead of
// the work), and one Stop.
func assertLifecycle(t *testing.T, rep *recordReporter, wantTotal, wantDone int) {
	t.Helper()
	if rep.started != 1 || rep.stopped != 1 {
		t.Fatalf("expected one Start and one Stop, got started=%d stopped=%d", rep.started, rep.stopped)
	}
	if rep.total != wantTotal {
		t.Fatalf("Start total = %d, want %d", rep.total, wantTotal)
	}
	if rep.done != wantDone {
		t.Fatalf("Done calls = %d, want %d (one per item)", rep.done, wantDone)
	}
}
