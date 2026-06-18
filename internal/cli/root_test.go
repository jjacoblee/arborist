package cli

import (
	"bytes"
	"strings"
	"testing"
)

// runRoot executes the root command with the given args, capturing combined
// stdout/stderr so tests do not depend on the global process streams.
func runRoot(t *testing.T, version string, args ...string) (string, error) {
	t.Helper()

	cmd := NewRootCmd(version)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

func TestRootCmd_Version(t *testing.T) {
	const want = "9.9.9"

	out, err := runRoot(t, want, "--version")
	if err != nil {
		t.Fatalf("execute --version: %v", err)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("version output = %q, want it to contain %q", out, want)
	}
}

func TestRootCmd_NoArgsShowsHelp(t *testing.T) {
	out, err := runRoot(t, "dev")
	if err != nil {
		t.Fatalf("execute with no args: %v", err)
	}
	if !strings.Contains(out, "arb") {
		t.Fatalf("help output = %q, want it to mention %q", out, "arb")
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("help output = %q, want it to contain a usage section", out)
	}
}

func TestRootCmd_UnknownCommandErrors(t *testing.T) {
	_, err := runRoot(t, "dev", "definitely-not-a-command")
	if err == nil {
		t.Fatal("expected an error for an unknown command, got nil")
	}
}
