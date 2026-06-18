package git

import (
	"context"
	"errors"
	"testing"

	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/exectest"
)

func TestEnsureInstalled_OK(t *testing.T) {
	fake := &exectest.Fake{} // default: success
	if err := New(fake).EnsureInstalled(context.Background()); err != nil {
		t.Fatalf("EnsureInstalled() = %v, want nil", err)
	}
	// It should have actually probed git.
	if len(fake.Calls) != 1 || fake.Calls[0].Name != "git" {
		t.Fatalf("expected one 'git' call, got %+v", fake.Calls)
	}
}

func TestEnsureInstalled_NotFound(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "--version"): {Err: exec.ErrNotFound},
		},
	}
	err := New(fake).EnsureInstalled(context.Background())
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("EnsureInstalled() = %v, want ErrNotInstalled", err)
	}
}

func TestEnsureInstalled_OtherErrorIsNotNotInstalled(t *testing.T) {
	fake := &exectest.Fake{
		Responses: map[string]exectest.Result{
			exectest.Key("git", "--version"): {Err: errors.New("permission denied")},
		},
	}
	err := New(fake).EnsureInstalled(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrNotInstalled) {
		t.Fatalf("a non-PATH failure should not be reported as ErrNotInstalled: %v", err)
	}
}
