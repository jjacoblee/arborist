package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
)

// ShellRunner runs a user-configured setup command in a directory.
//
// RunShell streams the command to the user's terminal (for `arb setup`, where
// the user is watching). RunShellCapture instead captures combined output and
// returns it, so `arb new` can run setup quietly behind a progress bar and show
// output only when a command fails.
//
// Unlike Runner and Launcher, ShellRunner intentionally invokes a shell
// ("sh -c <command>"), because setup commands are shell by nature (pipes, &&,
// environment expansion). This is safe only because the commands come from the
// user's own ownership-checked workspace config — never from a repository.
type ShellRunner interface {
	RunShell(ctx context.Context, dir, command string) error
	RunShellCapture(ctx context.Context, dir, command string) ([]byte, error)
}

// OSShell is a ShellRunner backed by os/exec. The zero value is ready to use.
type OSShell struct{}

// RunShell runs command via "sh -c" with the working directory set to dir,
// streaming the command's output to the terminal. It waits for completion.
func (OSShell) RunShell(ctx context.Context, dir, command string) error {
	cmd := osexec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %q in %s: %w", command, dir, err)
	}
	return nil
}

// RunShellCapture runs command via "sh -c" in dir, capturing combined stdout and
// stderr and returning it (even on failure, so the caller can show the log). The
// command's stdin is left disconnected so a setup step can't block waiting for
// terminal input.
func (OSShell) RunShellCapture(ctx context.Context, dir, command string) ([]byte, error) {
	cmd := osexec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.Bytes(), fmt.Errorf("run %q in %s: %w", command, dir, err)
	}
	return buf.Bytes(), nil
}
