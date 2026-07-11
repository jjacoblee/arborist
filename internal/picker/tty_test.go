package picker

import (
	"context"
	"errors"
	"testing"
)

// forceNonInteractive makes the session look like it has no terminal for the
// duration of a test, so the fail-fast path is exercised deterministically no
// matter how the test binary is run.
func forceNonInteractive(t *testing.T) {
	t.Helper()
	orig := interactiveSession
	interactiveSession = func() bool { return false }
	t.Cleanup(func() { interactiveSession = orig })
}

func TestSelect_NoTerminal_FailsFast(t *testing.T) {
	forceNonInteractive(t)

	_, err := Huh{}.Select(context.Background(), "feature/x", sampleRepos())
	if !errors.Is(err, ErrNotATerminal) {
		t.Fatalf("Select without a terminal = %v, want ErrNotATerminal", err)
	}
}

// TestSelect_EmptyBeatsTerminalCheck pins the check order: an empty repo list
// is reported as ErrNoRepositories even in a non-interactive session, because
// "nothing to pick" is the more specific problem.
func TestSelect_EmptyBeatsTerminalCheck(t *testing.T) {
	forceNonInteractive(t)

	_, err := Huh{}.Select(context.Background(), "feature/x", nil)
	if !errors.Is(err, ErrNoRepositories) {
		t.Fatalf("Select(empty) = %v, want ErrNoRepositories", err)
	}
}

func TestConfirm_NoTerminal_FailsFast(t *testing.T) {
	forceNonInteractive(t)

	_, err := HuhConfirmer{}.Confirm(context.Background(), "Remove 1 worktree?")
	if !errors.Is(err, ErrNotATerminal) {
		t.Fatalf("Confirm without a terminal = %v, want ErrNotATerminal", err)
	}
}
