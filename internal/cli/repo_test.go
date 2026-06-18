package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exectest"
)

// runRootWithRunner executes the command tree with a fake runner injected,
// capturing combined output.
func runRootWithRunner(t *testing.T, fake *exectest.Fake, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd("dev", deps{runner: fake})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

const repoListJSON = `[
  {"name":"web-app","nameWithOwner":"acme/web-app","isPrivate":true},
  {"name":"docs","nameWithOwner":"acme/docs","isPrivate":false}
]`

func TestRepoList_PrintsTable(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	fake := &exectest.Fake{
		// gh --version and gh auth status fall through to Default (success).
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "list", "acme", "--limit", "200", "--json",
				"name,nameWithOwner,isPrivate"): {Out: []byte(repoListJSON)},
		},
	}

	out, err := runRootWithRunner(t, fake, "repo", "list", "--dir", dir)
	if err != nil {
		t.Fatalf("repo list: %v", err)
	}
	for _, want := range []string{"REPOSITORY", "acme/web-app", "private", "acme/docs", "public"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRepoList_HonorsLimitFlag(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "list", "acme", "--limit", "3", "--json",
				"name,nameWithOwner,isPrivate"): {Out: []byte("[]")},
		},
	}

	out, err := runRootWithRunner(t, fake, "repo", "list", "--dir", dir, "--limit", "3")
	if err != nil {
		t.Fatalf("repo list --limit 3: %v", err)
	}
	if !strings.Contains(out, "No repositories found.") {
		t.Fatalf("expected empty-result message, got:\n%s", out)
	}

	// The gh repo list call should have used the overridden limit for acme.
	var found bool
	for _, c := range fake.Calls {
		if c.Name == "gh" && len(c.Args) >= 5 &&
			c.Args[0] == "repo" && c.Args[1] == "list" && c.Args[2] == "acme" && c.Args[4] == "3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a gh repo list acme --limit 3 call, got %+v", fake.Calls)
	}
}

func TestRepoList_NotAuthenticated(t *testing.T) {
	dir := writeWorkspace(t, "acme")
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "auth", "status"): {Err: errors.New("exit status 1")},
		},
	}

	_, err := runRootWithRunner(t, fake, "repo", "list", "--dir", dir)
	if err == nil {
		t.Fatal("expected an error when gh is not authenticated")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("error should tell the user to run gh auth login, got: %q", err.Error())
	}
	// It must not have attempted to list repos.
	for _, c := range fake.Calls {
		if c.Name == "gh" && len(c.Args) > 0 && c.Args[0] == "repo" {
			t.Fatalf("should not call gh repo list when unauthenticated; calls: %+v", fake.Calls)
		}
	}
}

func TestRepoList_NotInWorkspace(t *testing.T) {
	fake := &exectest.Fake{}
	_, err := runRootWithRunner(t, fake, "repo", "list", "--dir", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "arb init") {
		t.Fatalf("expected an actionable not-in-workspace error, got: %v", err)
	}
}

func TestRepoGroup_NoSubcommandShowsHelp(t *testing.T) {
	fake := &exectest.Fake{}
	out, err := runRootWithRunner(t, fake, "repo")
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	if !strings.Contains(out, "list") {
		t.Fatalf("repo help should mention the list subcommand:\n%s", out)
	}
}
