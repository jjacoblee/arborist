package github

import (
	"context"
	"errors"
	"testing"

	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/exectest"
)

const sampleRepoJSON = `[
  {"name":"web-app","nameWithOwner":"acme/web-app","isPrivate":true},
  {"name":"docs","nameWithOwner":"acme/docs","isPrivate":false}
]`

func TestListRepos_ParsesAndBuildsCommand(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "list", "--limit", "200", "--json", repoJSONFields): {Out: []byte(sampleRepoJSON)},
		},
	}

	repos, err := New(fake).ListRepos(context.Background(), "", 0) // personal, 0 -> default 200
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}

	want := Repository{
		Name:          "web-app",
		NameWithOwner: "acme/web-app",
		Owner:         "acme",
		IsPrivate:     true,
	}
	if repos[0] != want {
		t.Fatalf("repos[0] = %+v\nwant %+v", repos[0], want)
	}
	if repos[1].IsPrivate {
		t.Fatalf("repos[1] should be public")
	}

	// Confirm the exact gh invocation.
	if len(fake.Calls) != 1 {
		t.Fatalf("expected 1 call, got %+v", fake.Calls)
	}
	gotArgs := fake.Calls[0].Args
	wantArgs := []string{"repo", "list", "--limit", "200", "--json", repoJSONFields}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
}

func TestListRepos_OwnerScoped(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "list", "acme", "--limit", "200", "--json", repoJSONFields): {Out: []byte("[]")},
		},
	}
	if _, err := New(fake).ListRepos(context.Background(), "acme", 0); err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	want := []string{"repo", "list", "acme", "--limit", "200", "--json", repoJSONFields}
	got := fake.Calls[0].Args
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListRepos_LimitOverride(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "repo", "list", "--limit", "5", "--json", repoJSONFields): {Out: []byte("[]")},
		},
	}
	if _, err := New(fake).ListRepos(context.Background(), "", 5); err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if got := fake.Calls[0].Args[3]; got != "5" {
		t.Fatalf("limit arg = %q, want 5", got)
	}
}

func TestListRepos_Empty(t *testing.T) {
	fake := &exectest.Fake{Default: exectest.Result{Out: []byte("[]")}}
	repos, err := New(fake).ListRepos(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("got %d repos, want 0", len(repos))
	}
}

func TestListRepos_GHMissing(t *testing.T) {
	fake := &exectest.Fake{Default: exectest.Result{Err: exec.ErrNotFound}}
	if _, err := New(fake).ListRepos(context.Background(), "", 10); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("ListRepos error = %v, want ErrNotInstalled", err)
	}
}

func TestCloneRepo(t *testing.T) {
	fake := &exectest.Fake{}
	if err := New(fake).CloneRepo(context.Background(), "acme/web", "/repos/acme/web"); err != nil {
		t.Fatalf("CloneRepo: %v", err)
	}
	want := []string{"repo", "clone", "acme/web", "/repos/acme/web"}
	got := fake.Calls[0].Args
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	missing := &exectest.Fake{Default: exectest.Result{Err: exec.ErrNotFound}}
	if err := New(missing).CloneRepo(context.Background(), "acme/web", "/d"); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("CloneRepo error = %v, want ErrNotInstalled", err)
	}
}

func TestParseRepos_InvalidJSON(t *testing.T) {
	if _, err := parseRepos([]byte("{not an array")); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestParseRepos_BadNameWithOwner(t *testing.T) {
	cases := []string{
		`[{"nameWithOwner":"noslash"}]`,
		`[{"nameWithOwner":""}]`,
		`[{"nameWithOwner":"/name"}]`,
		`[{"nameWithOwner":"owner/"}]`,
	}
	for _, in := range cases {
		if _, err := parseRepos([]byte(in)); err == nil {
			t.Fatalf("expected an error for %s", in)
		}
	}
}

func TestParseRepos_NameFallback(t *testing.T) {
	// No explicit "name": derive it from nameWithOwner.
	repos, err := parseRepos([]byte(`[{"nameWithOwner":"acme/api","url":"https://github.com/acme/api"}]`))
	if err != nil {
		t.Fatalf("parseRepos: %v", err)
	}
	if repos[0].Name != "api" || repos[0].Owner != "acme" {
		t.Fatalf("got %+v, want name=api owner=acme", repos[0])
	}
}
