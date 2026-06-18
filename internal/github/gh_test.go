package github

import (
	"context"
	"errors"
	"testing"

	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/exectest"
)

func TestEnsureInstalled_OK(t *testing.T) {
	fake := &exectest.Fake{}
	if err := New(fake).EnsureInstalled(context.Background()); err != nil {
		t.Fatalf("EnsureInstalled() = %v, want nil", err)
	}
}

func TestEnsureInstalled_NotFound(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "--version"): {Err: exec.ErrNotFound},
		},
	}
	if err := New(fake).EnsureInstalled(context.Background()); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("EnsureInstalled() = %v, want ErrNotInstalled", err)
	}
}

func TestEnsureAuthenticated_OK(t *testing.T) {
	fake := &exectest.Fake{} // gh auth status succeeds
	if err := New(fake).EnsureAuthenticated(context.Background()); err != nil {
		t.Fatalf("EnsureAuthenticated() = %v, want nil", err)
	}
	if len(fake.Calls) != 1 || fake.Calls[0].Name != "gh" {
		t.Fatalf("expected a single 'gh auth status' call, got %+v", fake.Calls)
	}
}

func TestEnsureAuthenticated_NotLoggedIn(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			// gh auth status exits non-zero when not authenticated.
			exectest.Key("gh", "auth", "status"): {Err: errors.New("exit status 1")},
		},
	}
	if err := New(fake).EnsureAuthenticated(context.Background()); !errors.Is(err, ErrNotAuthenticated) {
		t.Fatalf("EnsureAuthenticated() = %v, want ErrNotAuthenticated", err)
	}
}

func TestEnsureAuthenticated_GHMissing(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("gh", "auth", "status"): {Err: exec.ErrNotFound},
		},
	}
	if err := New(fake).EnsureAuthenticated(context.Background()); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("EnsureAuthenticated() = %v, want ErrNotInstalled", err)
	}
}
