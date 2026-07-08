// Package exectest provides a fake exec.Runner for use in tests. It records
// invocations and returns scripted output and errors, so command construction
// and error handling can be tested without running real commands.
package exectest

import (
	"context"
	"strings"
)

// Call records a single command invocation.
type Call struct {
	Name string
	Args []string
}

// Result is the scripted outcome of a command.
type Result struct {
	Out []byte
	Err error
}

// Fake is a scriptable exec.Runner.
//
// A command is matched by its full key: the program name followed by each
// argument, space-joined (for example "gh auth status"). If no key in Responses
// matches, Default is returned. Every invocation is appended to Calls in order.
type Fake struct {
	Responses map[string]Result
	Default   Result
	Calls     []Call
}

// Key builds the lookup key for a command and its arguments.
func Key(name string, args ...string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

// Run implements exec.Runner.
func (f *Fake) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: append([]string(nil), args...)})
	if r, ok := f.Responses[Key(name, args...)]; ok {
		return r.Out, r.Err
	}
	return f.Default.Out, f.Default.Err
}

// FakeLauncher is a scriptable exec.Launcher. It records each Launch call and
// returns Err (for example exec.ErrNotFound to simulate a missing editor).
type FakeLauncher struct {
	Err   error
	Calls []Call
}

// Launch implements exec.Launcher.
func (f *FakeLauncher) Launch(_ context.Context, name string, args ...string) error {
	f.Calls = append(f.Calls, Call{Name: name, Args: append([]string(nil), args...)})
	return f.Err
}

// ShellCall records a single RunShell invocation.
type ShellCall struct {
	Dir     string
	Command string
}

// FakeShell is a scriptable exec.ShellRunner. It records each command and, when
// Err is set, fails: for every command, or only those containing FailOn. Out is
// the captured output returned by RunShellCapture (for example to simulate an
// install log on failure).
type FakeShell struct {
	Err    error
	FailOn string
	Out    []byte
	Calls  []ShellCall
}

func (f *FakeShell) fail(command string) bool {
	return f.Err != nil && (f.FailOn == "" || strings.Contains(command, f.FailOn))
}

// RunShell implements exec.ShellRunner.
func (f *FakeShell) RunShell(_ context.Context, dir, command string) error {
	f.Calls = append(f.Calls, ShellCall{Dir: dir, Command: command})
	if f.fail(command) {
		return f.Err
	}
	return nil
}

// RunShellCapture implements exec.ShellRunner.
func (f *FakeShell) RunShellCapture(_ context.Context, dir, command string) ([]byte, error) {
	f.Calls = append(f.Calls, ShellCall{Dir: dir, Command: command})
	if f.fail(command) {
		return f.Out, f.Err
	}
	return f.Out, nil
}
