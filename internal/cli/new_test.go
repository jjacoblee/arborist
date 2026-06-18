package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/paths"
	"github.com/jjacoblee/arborist/internal/picker"
	"github.com/jjacoblee/arborist/internal/pickertest"
)

func runNew(t *testing.T, runner *exectest.Fake, sel *pickertest.Fake, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: runner, selector: sel})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

const oneRepoJSON = `[{"name":"web","nameWithOwner":"acme/web","isPrivate":false}]`

// ghOK wires gh repo discovery for the workspace owner "acme".
func ghOK(repoJSON string) map[string]exectest.Result {
	return map[string]exectest.Result{
		exectest.Key("gh", "repo", "list", "acme", "--limit", "200", "--json",
			"name,nameWithOwner,isPrivate"): {Out: []byte(repoJSON)},
		// gh --version and gh auth status use the runner Default (success).
	}
}

func TestNew_InvalidBranch_FailsFastWithoutCommands(t *testing.T) {
	runner := &exectest.Fake{}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "bad branch")
	if err == nil {
		t.Fatal("expected an error for an invalid branch name")
	}
	if !errors.Is(err, paths.ErrInvalidBranchName) {
		t.Fatalf("error should wrap ErrInvalidBranchName, got: %v", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("no commands should run for an invalid branch; got %+v", runner.Calls)
	}
}

func TestNew_NotInWorkspace_TellsUserToInit(t *testing.T) {
	runner := &exectest.Fake{}
	sel := &pickertest.Fake{}
	empty := t.TempDir() // a plain dir, not a workspace

	_, err := runNew(t, runner, sel, "new", "feature/x", "--dir", empty)
	if err == nil || !strings.Contains(err.Error(), "arb init") {
		t.Fatalf("expected an actionable not-in-workspace error, got: %v", err)
	}
}

func TestNew_HappyPath_CreatesWorktree(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)} // git ops succeed via Default
	sel := &pickertest.Fake{Result: []github.Repository{repo}}

	out, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Created worktrees") || !strings.Contains(out, "acme/web") {
		t.Fatalf("expected a created-worktree summary, got:\n%s", out)
	}
	// The picker was asked to present the branch and the discovered repo.
	if sel.GotBranch != "feature/x" || len(sel.GotRepos) != 1 {
		t.Fatalf("selector inputs = branch %q, repos %d", sel.GotBranch, len(sel.GotRepos))
	}
}

func TestNew_DiscoversWorkspaceOwner(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Result: []github.Repository{repo}}

	out, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	var found bool
	for _, c := range runner.Calls {
		if c.Name == "gh" && len(c.Args) >= 3 && c.Args[0] == "repo" && c.Args[2] == "acme" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected gh repo list scoped to the workspace owner acme, got %+v", runner.Calls)
	}
}

func TestNew_NoRepositoriesFound(t *testing.T) {
	runner := &exectest.Fake{Responses: ghOK("[]")}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err == nil || !strings.Contains(err.Error(), "no repositories found") {
		t.Fatalf("expected a no-repositories error, got: %v", err)
	}
}

func TestNew_PickerCanceled_NoError(t *testing.T) {
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Err: picker.ErrCanceled}

	out, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err != nil {
		t.Fatalf("cancel should not be an error, got: %v", err)
	}
	if !strings.Contains(out, "Canceled") {
		t.Fatalf("expected a cancellation message, got:\n%s", out)
	}
}

func TestNew_NothingSelected(t *testing.T) {
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Result: nil} // confirmed with no repos checked

	out, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if !strings.Contains(out, "Nothing to do") {
		t.Fatalf("expected nothing-to-do message, got:\n%s", out)
	}
}

func TestNew_NotAuthenticated_GuidesUser(t *testing.T) {
	runner := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "auth", "status"): {Err: errors.New("exit status 1")},
		},
	}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("expected gh auth guidance, got: %v", err)
	}
}

func TestNew_WorktreeFailure_ExitsNonZero(t *testing.T) {
	repo := github.Repository{Name: "web", Owner: "acme", NameWithOwner: "acme/web"}
	// Preflight + discovery succeed explicitly; everything else (the worktree
	// git operations) falls through to the Default error, so creation fails.
	resp := ghOK(oneRepoJSON)
	resp[exectest.Key("git", "--version")] = exectest.Result{}
	resp[exectest.Key("gh", "--version")] = exectest.Result{}
	resp[exectest.Key("gh", "auth", "status")] = exectest.Result{}
	runner := &exectest.Fake{
		Responses: resp,
		Default:   exectest.Result{Err: errors.New("git boom")},
	}
	sel := &pickertest.Fake{Result: []github.Repository{repo}}

	out, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err == nil {
		t.Fatal("expected a non-nil error when a repository fails")
	}
	if !strings.Contains(out, "Failed") {
		t.Fatalf("expected a Failed section in the summary, got:\n%s", out)
	}
}
