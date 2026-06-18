package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/exectest"
	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
)

// clients builds git and github clients sharing one fake runner.
func clients(fake *exectest.Fake) (git.Client, github.Client) {
	return git.New(fake), github.New(fake)
}

func TestCheckPrerequisites_AllGood(t *testing.T) {
	fake := &exectest.Fake{} // every command succeeds
	g, h := clients(fake)
	if err := checkPrerequisites(context.Background(), g, h); err != nil {
		t.Fatalf("checkPrerequisites() = %v, want nil", err)
	}
}

func TestCheckPrerequisites_GitMissing(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "--version"): {Err: exec.ErrNotFound},
		},
	}
	g, h := clients(fake)
	err := checkPrerequisites(context.Background(), g, h)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "git-scm.com") {
		t.Fatalf("error should point to the Git download page, got: %q", err.Error())
	}
}

func TestCheckPrerequisites_GHMissing(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "--version"): {Err: exec.ErrNotFound},
		},
	}
	g, h := clients(fake)
	err := checkPrerequisites(context.Background(), g, h)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "cli.github.com") {
		t.Fatalf("error should point to the gh install page, got: %q", err.Error())
	}
}

func TestCheckPrerequisites_GHNotAuthenticated(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "auth", "status"): {Err: errors.New("exit status 1")},
		},
	}
	g, h := clients(fake)
	err := checkPrerequisites(context.Background(), g, h)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("error should tell the user to run 'gh auth login', got: %q", err.Error())
	}
}
