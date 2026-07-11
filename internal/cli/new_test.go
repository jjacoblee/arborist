package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
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

func TestNew_NamedRepos_SkipDiscoveryAndPicker(t *testing.T) {
	runner := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "view", "acme/web", "--json", "name,nameWithOwner,isPrivate"): {
				Out: []byte(`{"name":"web","nameWithOwner":"acme/web","isPrivate":false}`),
			},
		},
	}
	sel := &pickertest.Fake{}

	out, err := runNew(t, runner, sel, "new", "feature/x", "acme/web", "--dir", writeWorkspace(t, "acme"))
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Created worktrees") || !strings.Contains(out, "acme/web") {
		t.Fatalf("expected a created-worktree summary, got:\n%s", out)
	}
	if sel.Calls != 0 {
		t.Fatalf("expected the interactive picker to be skipped, got %d calls", sel.Calls)
	}
	for _, c := range runner.Calls {
		if c.Name == "gh" && len(c.Args) >= 2 && c.Args[0] == "repo" && c.Args[1] == "list" {
			t.Fatalf("expected gh repo list to be skipped when repos are named, got %+v", runner.Calls)
		}
	}
}

func TestNew_NamedRepos_Multiple(t *testing.T) {
	runner := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "view", "acme/web", "--json", "name,nameWithOwner,isPrivate"): {
				Out: []byte(`{"name":"web","nameWithOwner":"acme/web","isPrivate":false}`),
			},
			exectest.Key("gh", "repo", "view", "acme/api", "--json", "name,nameWithOwner,isPrivate"): {
				Out: []byte(`{"name":"api","nameWithOwner":"acme/api","isPrivate":true}`),
			},
		},
	}
	sel := &pickertest.Fake{}

	out, err := runNew(t, runner, sel, "new", "feature/x",
		"acme/web", "acme/api", "--dir", writeWorkspace(t, "acme"))
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if !strings.Contains(out, "acme/web") || !strings.Contains(out, "acme/api") {
		t.Fatalf("expected both repos in the summary, got:\n%s", out)
	}
}

func TestNew_NamedRepos_InvalidFormat(t *testing.T) {
	runner := &exectest.Fake{}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "feature/x", "web", "--dir", writeWorkspace(t, "acme"))
	if err == nil || !strings.Contains(err.Error(), "owner/repo") {
		t.Fatalf("expected an owner/repo format error, got: %v", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("no gh commands should run for an invalid repository, got %+v", runner.Calls)
	}
}

func TestNew_NamedRepos_NotFound(t *testing.T) {
	runner := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "view", "acme/ghost", "--json", "name,nameWithOwner,isPrivate"): {
				Err: errors.New("GraphQL: Could not resolve to a Repository"),
			},
		},
	}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "feature/x", "acme/ghost", "--dir", writeWorkspace(t, "acme"))
	if err == nil || !strings.Contains(err.Error(), "acme/ghost") {
		t.Fatalf("expected an error naming the unresolved repo, got: %v", err)
	}
}

// newJSONResult mirrors the `arb new --json` output shape for assertions.
type newJSONResult struct {
	Created []struct {
		Repository string `json:"repository"`
		Branch     string `json:"branch"`
		Path       string `json:"path"`
		Source     string `json:"source"`
		Setup      struct {
			Status  string `json:"status"`
			Command string `json:"command"`
			Output  string `json:"output"`
		} `json:"setup"`
	} `json:"created"`
	Skipped []json.RawMessage `json:"skipped"`
	Failed  []struct {
		Repository string `json:"repository"`
		Error      string `json:"error"`
	} `json:"failed"`
}

func TestNew_JSON_RequiresNamedRepos(t *testing.T) {
	runner := &exectest.Fake{}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "feature/x", "--json", "--dir", writeWorkspace(t, "acme"))
	if err == nil || !strings.Contains(err.Error(), "name repositories explicitly") {
		t.Fatalf("expected an instructive named-repos error, got: %v", err)
	}
	if sel.Calls != 0 {
		t.Fatalf("the picker must never run under --json, got %d calls", sel.Calls)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("no commands should run before the --json check, got %+v", runner.Calls)
	}
}

func TestNew_JSON_Output(t *testing.T) {
	runner := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "view", "acme/web", "--json", "name,nameWithOwner,isPrivate"): {
				Out: []byte(`{"name":"web","nameWithOwner":"acme/web","isPrivate":false}`),
			},
		},
	}
	sel := &pickertest.Fake{}
	dir := writeWorkspace(t, "acme")

	out, err := runNew(t, runner, sel, "new", "feature/x", "acme/web", "--json", "--dir", dir)
	if err != nil {
		t.Fatalf("new --json: %v\n%s", err, out)
	}

	// stdout must be exactly one JSON document — no human summary around it.
	var res newJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(res.Created) != 1 || len(res.Skipped) != 0 || len(res.Failed) != 0 {
		t.Fatalf("created/skipped/failed = %d/%d/%d, want 1/0/0\n%s",
			len(res.Created), len(res.Skipped), len(res.Failed), out)
	}
	c := res.Created[0]
	if c.Repository != "acme/web" || c.Branch != "feature/x" {
		t.Fatalf("created = %+v", c)
	}
	// The path is the contract's centerpiece: absolute, so a caller can cd
	// straight into it.
	if !filepath.IsAbs(c.Path) || c.Path != filepath.Join(workspaceWorktreeRoot(dir), "web", "feature-x") {
		t.Fatalf("path = %q, want absolute worktree path under %q", c.Path, dir)
	}
	// No setup commands are configured in this workspace.
	if c.Setup.Status != "none" {
		t.Fatalf("setup.status = %q, want %q", c.Setup.Status, "none")
	}
}

func TestNew_JSON_SetupFailureEmbedded(t *testing.T) {
	runner := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "view", "acme/web", "--json", "name,nameWithOwner,isPrivate"): {
				Out: []byte(`{"name":"web","nameWithOwner":"acme/web","isPrivate":false}`),
			},
		},
	}
	sel := &pickertest.Fake{}
	shell := &exectest.FakeShell{
		Err: errors.New("exit 1"),
		Out: []byte("npm warn deprecated\nERR_PNPM_LOCKFILE_BREAKING_CHANGE: bad lockfile\n"),
	}
	dir := newWithSetupWorkspace(t)

	out, err := runNewShell(t, runner, sel, shell, "new", "feature/x", "acme/web", "--json", "--dir", dir)
	// A setup failure is a warning: the worktree exists, the exit code is 0,
	// and the failure is reported inside the JSON document.
	if err != nil {
		t.Fatalf("setup failure should not fail --json, got: %v\n%s", err, out)
	}
	var res newJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(res.Created) != 1 {
		t.Fatalf("expected one created worktree:\n%s", out)
	}
	s := res.Created[0].Setup
	if s.Status != "failed" || s.Command != "pnpm install" {
		t.Fatalf("setup = %+v, want failed on pnpm install", s)
	}
	if !strings.Contains(s.Output, "ERR_PNPM_LOCKFILE_BREAKING_CHANGE") {
		t.Fatalf("setup.output should carry the captured tail, got %q", s.Output)
	}
}

func TestNew_PickerWithoutTerminal_InstructiveError(t *testing.T) {
	runner := &exectest.Fake{Responses: ghOK(oneRepoJSON)}
	sel := &pickertest.Fake{Err: picker.ErrNotATerminal}

	_, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"))
	if err == nil || !strings.Contains(err.Error(), "name repositories explicitly") {
		t.Fatalf("expected the non-TTY hint to name the explicit form, got: %v", err)
	}
	if !strings.Contains(err.Error(), "arb new feature/x") {
		t.Fatalf("expected a copy-pasteable example with the branch, got: %v", err)
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
