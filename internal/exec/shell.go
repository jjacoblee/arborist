package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
)

// ShellRunner runs a user-configured setup command in a directory, with the
// command connected to the user's terminal (inherited stdin/stdout/stderr).
//
// Unlike Runner and Launcher, ShellRunner intentionally invokes a shell
// ("sh -c <command>"), because setup commands are shell by nature (pipes, &&,
// environment expansion). This is safe only because the commands come from the
// user's own ownership-checked workspace config — never from a repository.
type ShellRunner interface {
	RunShell(ctx context.Context, dir, command string) error
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
