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

const twoRepoJSON = `[{"name":"web","nameWithOwner":"acme/web","isPrivate":false},
{"name":"api","nameWithOwner":"acme/api","isPrivate":false}]`

// runNewWith runs a command with a confirmer wired in, for flows that ask.
func runNewWith(t *testing.T, runner *exectest.Fake, sel *pickertest.Fake,
	conf *pickertest.FakeConfirmer, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: runner, selector: sel, confirmer: conf})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// worktreeAddPath returns the path git was asked to create a worktree at,
// covering both shapes Arborist emits: `worktree add <path> <branch>` when the
// branch already exists, and `worktree add -b <branch> <path> [base]` when it
// creates one.
func worktreeAddPath(runner *exectest.Fake) string {
	for _, c := range runner.Calls {
		if c.Name != "git" || len(c.Args) < 5 || c.Args[2] != "worktree" || c.Args[3] != "add" {
			continue
		}
		if c.Args[4] == "-b" {
			if len(c.Args) >= 7 {
				return c.Args[6]
			}
			continue
		}
		return c.Args[4]
	}
	return ""
}

func TestNew_GroupNestsWorktreeUnderGroupFolder(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{Result: true} // agrees to the new group

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "review")
	if err != nil {
		t.Fatalf("new --group: %v\n%s", err, out)
	}
	want := filepath.Join(workspaceWorktreeRoot(dir), "review", "api", "pr-1234")
	if got := worktreeAddPath(runner); got != want {
		t.Fatalf("worktree path = %q, want %q", got, want)
	}
}

// writeGroupWorkspace writes a workspace whose config declares groups and a
// default group.
func writeGroupWorkspace(t *testing.T, groups []string, defaultGroup string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Owner: "acme", Groups: groups, DefaultGroup: defaultGroup}
	if err := config.Save(config.ConfigPath(dir), cfg); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNew_NewGroupIsConfirmedFirst(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{Result: false} // declines the new group

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "reveiw")
	if err != nil {
		t.Fatalf("declining a group is not a failure: %v\n%s", err, out)
	}
	if conf.Calls != 1 {
		t.Fatalf("expected one prompt for the unfamiliar group, got %q", conf.Prompts)
	}
	if worktreeAddPath(runner) != "" {
		t.Fatalf("nothing may be created when the group is declined; got %+v", runner.Calls)
	}
}

func TestNew_ExistingGroupIsNotQueried(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	// A worktree already lives in "review", so the group is clearly deliberate.
	mkWorktree(t, filepath.Join(workspaceWorktreeRoot(dir), "review", "web", "pr-1"))
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "review")
	if err != nil {
		t.Fatalf("new --group: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("a group already in use must not be queried, got %q", conf.Prompts)
	}
}

func TestNew_DeclaredGroupIsNotQueried(t *testing.T) {
	dir := writeGroupWorkspace(t, []string{"review"}, "")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{Result: true}

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "review")
	if err != nil {
		t.Fatalf("new --group: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("a declared group must not be queried, got %q", conf.Prompts)
	}
}

func TestNew_YesSkipsTheGroupPrompt(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{}

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "review", "--yes")
	if err != nil {
		t.Fatalf("new --group --yes: %v\n%s", err, out)
	}
	if conf.Calls != 0 {
		t.Fatalf("--yes must keep the run unattended, got %q", conf.Prompts)
	}
	want := filepath.Join(workspaceWorktreeRoot(dir), "review", "api", "pr-1234")
	if got := worktreeAddPath(runner); got != want {
		t.Fatalf("worktree path = %q, want %q", got, want)
	}
}

func TestNew_DefaultGroupAppliesWithoutTheFlag(t *testing.T) {
	dir := writeGroupWorkspace(t, []string{"review"}, "review")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{}

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	want := filepath.Join(workspaceWorktreeRoot(dir), "review", "api", "pr-1234")
	if got := worktreeAddPath(runner); got != want {
		t.Fatalf("worktree path = %q, want %q", got, want)
	}
}

func TestNew_EmptyGroupFlagOptsOutOfDefaultGroup(t *testing.T) {
	dir := writeGroupWorkspace(t, []string{"review"}, "review")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	conf := &pickertest.FakeConfirmer{}

	out, err := runNewWith(t, runner, &pickertest.Fake{}, conf,
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "")
	if err != nil {
		t.Fatalf("new --group \"\": %v\n%s", err, out)
	}
	want := filepath.Join(workspaceWorktreeRoot(dir), "api", "pr-1234")
	if got := worktreeAddPath(runner); got != want {
		t.Fatalf("worktree path = %q, want %q (ungrouped)", got, want)
	}
}

func TestNew_UnusableGroupNameIsRejected(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}

	_, err := runNewWith(t, runner, &pickertest.Fake{}, &pickertest.FakeConfirmer{},
		"new", "pr/1234", "--dir", dir, "--repo", "api", "--group", "..")
	if !errors.Is(err, paths.ErrInvalidGroupName) {
		t.Fatalf("err = %v, want ErrInvalidGroupName", err)
	}
	if worktreeAddPath(runner) != "" {
		t.Fatalf("nothing may be created for an unusable group; got %+v", runner.Calls)
	}
}

func TestNew_RepoFlagSkipsPicker(t *testing.T) {
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	sel := &pickertest.Fake{}

	out, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"), "--repo", "api")
	if err != nil {
		t.Fatalf("new --repo: %v\n%s", err, out)
	}
	if sel.Calls != 0 {
		t.Fatal("naming repositories must skip the picker entirely")
	}
	if !strings.Contains(out, "acme/api") {
		t.Fatalf("expected a worktree for acme/api, got:\n%s", out)
	}
	if strings.Contains(out, "acme/web") {
		t.Fatalf("only the named repository should be used, got:\n%s", out)
	}
}

func TestNew_RepoFlagAcceptsListForms(t *testing.T) {
	for _, args := range [][]string{
		{"--repo", "api", "--repo", "web"},
		{"--repo", "api,web"},
		{"--repo", "api web"},
	} {
		runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
		sel := &pickertest.Fake{}

		cmdArgs := append([]string{"new", "feature/x", "--dir", writeWorkspace(t, "acme")}, args...)
		out, err := runNew(t, runner, sel, cmdArgs...)
		if err != nil {
			t.Fatalf("new %v: %v\n%s", args, err, out)
		}
		if !strings.Contains(out, "acme/api") || !strings.Contains(out, "acme/web") {
			t.Fatalf("new %v should use both repositories, got:\n%s", args, out)
		}
	}
}

func TestNew_UnknownRepoFailsWithoutCreatingAnything(t *testing.T) {
	runner := &exectest.Fake{Responses: ghOK(twoRepoJSON)}
	sel := &pickertest.Fake{}

	_, err := runNew(t, runner, sel, "new", "feature/x", "--dir", writeWorkspace(t, "acme"), "--repo", "api,wbe")
	if err == nil {
		t.Fatal("expected an error for an unknown repository")
	}
	if !strings.Contains(err.Error(), "wbe") {
		t.Fatalf("error should name the unknown repository, got: %v", err)
	}
	for _, c := range runner.Calls {
		if c.Name == "git" && len(c.Args) >= 4 && c.Args[2] == "worktree" && c.Args[3] == "add" {
			t.Fatalf("nothing may be created when a name is unknown; got %+v", c)
		}
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
