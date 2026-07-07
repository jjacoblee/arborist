package exec

import (
	"context"
	"strings"
	"testing"
)

// TestRunDisablesGitPrompts verifies the captured-output runner forces git's
// interactive credential prompt off. Without this, a git/gh command that needs
// credentials it can't obtain silently blocks forever on a /dev/tty prompt
// (the multi-repo `arb new` hang).
func TestRunDisablesGitPrompts(t *testing.T) {
	out, err := OS{}.Run(context.Background(), "sh", "-c", "printf %s \"$GIT_TERMINAL_PROMPT\"")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(out); got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want %q", got, "0")
	}
}

func TestNonInteractiveEnv(t *testing.T) {
	t.Run("disables credential and SSH prompts by default", func(t *testing.T) {
		t.Setenv("GIT_SSH_COMMAND", "")
		env := nonInteractiveEnv()
		assertEnv(t, env, "GIT_TERMINAL_PROMPT", "0")
		assertEnv(t, env, "GCM_INTERACTIVE", "never")
		assertEnv(t, env, "GIT_SSH_COMMAND", "ssh -o BatchMode=yes")
	})

	t.Run("respects a user-provided GIT_SSH_COMMAND", func(t *testing.T) {
		t.Setenv("GIT_SSH_COMMAND", "ssh -i /custom/key")
		env := nonInteractiveEnv()
		// Arborist must not clobber a custom SSH command.
		assertEnv(t, env, "GIT_SSH_COMMAND", "ssh -i /custom/key")
		// Prompt disabling still applies.
		assertEnv(t, env, "GIT_TERMINAL_PROMPT", "0")
	})

	t.Run("forces prompts off even if inherited as enabled", func(t *testing.T) {
		t.Setenv("GIT_TERMINAL_PROMPT", "1")
		// Go's exec keeps the last value for a duplicate key, so the appended
		// "0" must win over the inherited "1".
		assertEnv(t, nonInteractiveEnv(), "GIT_TERMINAL_PROMPT", "0")
	})
}

// assertEnv checks the effective value of key in a KEY=VALUE slice, honoring
// last-wins semantics (matching os/exec's dedup behavior).
func assertEnv(t *testing.T, env []string, key, want string) {
	t.Helper()
	got, found := "", false
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == key {
			got, found = v, true
		}
	}
	if !found {
		t.Fatalf("%s not set in environment", key)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
