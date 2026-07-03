package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/config"
	"github.com/jjacoblee/arborist/internal/exectest"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/pickertest"
	"github.com/jjacoblee/arborist/internal/worktree"
)

func runWithShell(t *testing.T, fake *exectest.Fake, shell *exectest.FakeShell, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: fake, shell: shell})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// setupFixture builds a workspace with one worktree (repo "web", branch
// "feature/x") and the given setup config.
func setupFixture(t *testing.T, setup map[string][]string) (dir, wtPath string, fake *exectest.Fake) {
	t.Helper()
	dir = t.TempDir()
	if err := config.Save(config.ConfigPath(dir), config.Config{Owner: "acme", Setup: setup}); err != nil {
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

func TestSetup_ByID_RunsCommands(t *testing.T) {
	dir, wtPath, fake := setupFixture(t, map[string][]string{"web": {"pnpm install", "uv sync"}})
	shell := &exectest.FakeShell{}
	id := worktree.ID(wtPath)

	out, err := runWithShell(t, fake, shell, "setup", id[:6], "--dir", dir)
	if err != nil {
		t.Fatalf("setup: %v\n%s", err, out)
	}
	if len(shell.Calls) != 2 {
		t.Fatalf("expected 2 commands, got %+v", shell.Calls)
	}
	for _, c := range shell.Calls {
		if c.Dir != wtPath {
			t.Fatalf("command ran in %q, want %q", c.Dir, wtPath)
		}
	}
	if shell.Calls[0].Command != "pnpm install" || shell.Calls[1].Command != "uv sync" {
		t.Fatalf("commands = %+v", shell.Calls)
	}
}

func TestSetup_NoCommandsConfigured(t *testing.T) {
	dir, wtPath, fake := setupFixture(t, nil)
	shell := &exectest.FakeShell{}
	id := worktree.ID(wtPath)

	out, err := runWithShell(t, fake, shell, "setup", id[:6], "--dir", dir)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.Contains(out, "No setup commands configured") {
		t.Fatalf("expected a no-commands message, got:\n%s", out)
	}
	if len(shell.Calls) != 0 {
		t.Fatalf("nothing should run, got %+v", shell.Calls)
	}
}

func TestSetup_CommandFailureStops(t *testing.T) {
	dir, wtPath, fake := setupFixture(t, map[string][]string{"web": {"pnpm install", "boom", "never"}})
	shell := &exectest.FakeShell{Err: errors.New("exit 1"), FailOn: "boom"}
	id := worktree.ID(wtPath)

	_, err := runWithShell(t, fake, shell, "setup", id[:6], "--dir", dir)
	if err == nil || !strings.Contains(err.Error(), "setup command") {
		t.Fatalf("expected a setup failure error, got: %v", err)
	}
	// Ran pnpm install + boom, then stopped before "never".
	if len(shell.Calls) != 2 {
		t.Fatalf("expected to stop at the failing command, got %+v", shell.Calls)
	}
}

func runNewShell(t *testing.T, runner *exectest.Fake, sel *pickertest.Fake, shell *exectest.FakeShell, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: runner, selector: sel, shell: shell})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func newWithSetupWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Owner: "acme", Setup: map[string][]string{"web": {"pnpm install"}}}
	if err := config.Save(config.ConfigPath(dir), cfg); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNew_RunsSetupAfterCreate(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Result: []github.Repository{repo}}
	shell := &exectest.FakeShell{}
	dir := newWithSetupWorkspace(t)

	out, err := runNewShell(t, runner, sel, shell, "new", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if len(shell.Calls) != 1 || shell.Calls[0].Command != "pnpm install" {
		t.Fatalf("expected setup to run, got %+v", shell.Calls)
	}
	wantDir := filepath.Join(dir, "worktrees", "web", "feature-x")
	if shell.Calls[0].Dir != wantDir {
		t.Fatalf("setup ran in %q, want %q", shell.Calls[0].Dir, wantDir)
	}
}

func TestNew_SetupSuccess_DoesNotPrintInstallLog(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Result: []github.Repository{repo}}
	// The setup command "succeeds" but produces noisy output that must stay hidden.
	shell := &exectest.FakeShell{Out: []byte("PNPM_NOISE: added 1200 packages\n")}
	dir := newWithSetupWorkspace(t)

	out, err := runNewShell(t, runner, sel, shell, "new", "feature/x", "--dir", dir)
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if len(shell.Calls) != 1 {
		t.Fatalf("expected setup to run once, got %+v", shell.Calls)
	}
	if strings.Contains(out, "PNPM_NOISE") {
		t.Fatalf("install output should be hidden on success, got:\n%s", out)
	}
}

func TestNew_SetupFailure_ShowsOutputAndHint(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Result: []github.Repository{repo}}
	shell := &exectest.FakeShell{
		Err: errors.New("exit 1"),
		Out: []byte("npm warn deprecated\nERR_PNPM_LOCKFILE_BREAKING_CHANGE: bad lockfile\n"),
	}
	dir := newWithSetupWorkspace(t)

	out, err := runNewShell(t, runner, sel, shell, "new", "feature/x", "--dir", dir)
	// A setup failure is a warning, not a command error.
	if err != nil {
		t.Fatalf("setup failure should not fail the command, got: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Setup failed for acme/web") {
		t.Fatalf("expected a setup-failure notice, got:\n%s", out)
	}
	if !strings.Contains(out, "ERR_PNPM_LOCKFILE_BREAKING_CHANGE") {
		t.Fatalf("expected the captured error output to be shown, got:\n%s", out)
	}
	if !strings.Contains(out, "arb setup feature/x") {
		t.Fatalf("expected a re-run hint, got:\n%s", out)
	}
}

// TestPlanSetup_CountsCommandsNotRepos is the regression guard for the progress
// bar that read full from the start: the bar must be sized by the number of
// setup commands (steps), not the number of repositories. A single repo with
// several commands must yield several steps, and repos with no commands must not
// appear in the plan at all.
func TestPlanSetup_CountsCommandsNotRepos(t *testing.T) {
	cfg := config.Config{Owner: "acme", Setup: map[string][]string{
		"web": {"pnpm install", "pnpm build"}, // 2 commands
		"api": {"go mod download"},            // 1 command
		// "db" has no setup configured.
	}}
	created := []worktree.CreatedWorktree{
		{Repository: github.Repository{Name: "web", NameWithOwner: "acme/web"}},
		{Repository: github.Repository{Name: "api", NameWithOwner: "acme/api"}},
		{Repository: github.Repository{Name: "db", NameWithOwner: "acme/db"}},
	}

	jobs := planSetup(cfg, created)
	if len(jobs) != 2 {
		t.Fatalf("planSetup should skip repos with no commands, got %d jobs", len(jobs))
	}
	if got := setupSteps(jobs); got != 3 {
		t.Fatalf("setupSteps = %d, want 3 (commands, not repos)", got)
	}
}

// TestPlanSetup_SingleRepoSingleCommand documents the reported case: one repo
// with one command is exactly one step, so the bar goes 0/1 -> 1/1 instead of
// being sized to the repo count.
func TestPlanSetup_SingleRepoSingleCommand(t *testing.T) {
	cfg := config.Config{Owner: "acme", Setup: map[string][]string{"web": {"pnpm install"}}}
	created := []worktree.CreatedWorktree{
		{Repository: github.Repository{Name: "web", NameWithOwner: "acme/web"}},
	}
	if got := setupSteps(planSetup(cfg, created)); got != 1 {
		t.Fatalf("setupSteps = %d, want 1", got)
	}
}

func TestLastLines(t *testing.T) {
	if got := lastLines("", 5); got != "" {
		t.Fatalf("empty input = %q, want empty", got)
	}
	if got := lastLines("   \n\n", 5); got != "" {
		t.Fatalf("blank input = %q, want empty", got)
	}
	got := lastLines("a\nb\nc\nd", 2)
	if got != "  c\n  d" {
		t.Fatalf("tail = %q, want %q", got, "  c\n  d")
	}
}

func TestNew_NoSetupFlagSkips(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Result: []github.Repository{repo}}
	shell := &exectest.FakeShell{}
	dir := newWithSetupWorkspace(t)

	if _, err := runNewShell(t, runner, sel, shell, "new", "feature/x", "--dir", dir, "--no-setup"); err != nil {
		t.Fatalf("new --no-setup: %v", err)
	}
	if len(shell.Calls) != 0 {
		t.Fatalf("--no-setup should skip setup, got %+v", shell.Calls)
	}
}
