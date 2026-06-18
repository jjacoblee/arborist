package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
)

// Launcher starts an interactive program (such as an editor) connected to the
// user's terminal. Unlike Runner, it does not capture output: the program owns
// stdin/stdout/stderr so terminal editors work and GUI editors behave normally.
//
// Implementations must invoke name directly with the given argument array and
// must not pass the arguments through a shell.
type Launcher interface {
	Launch(ctx context.Context, name string, args ...string) error
}

// OSLauncher is a Launcher backed by os/exec. The zero value is ready to use.
type OSLauncher struct{}

// Launch runs name with args, wiring the current process's stdin, stdout, and
// stderr to the child so an editor can take over the terminal. It waits for the
// program to exit. If name is not found on PATH the error wraps ErrNotFound.
func (OSLauncher) Launch(ctx context.Context, name string, args ...string) error {
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}
