// Package exec provides a small abstraction over running external commands so
// that Git and GitHub CLI calls can be mocked in tests.
//
// Commands are always invoked with an explicit program name and argument array
// (never a shell string), which keeps untrusted input from being interpreted by
// a shell.
package exec

import (
	"bytes"
	"context"
	"fmt"
	osexec "os/exec"
	"strings"
)

// ErrNotFound is returned (wrapped) when the requested executable cannot be
// found on PATH. It aliases os/exec.ErrNotFound so callers can match it with
// errors.Is without importing os/exec directly.
var ErrNotFound = osexec.ErrNotFound

// Runner runs an external command and returns its standard output.
//
// Implementations must invoke name directly with the given argument array and
// must not pass the arguments through a shell.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// OS is a Runner backed by os/exec. The zero value is ready to use.
type OS struct{}

// Run executes name with args and returns its standard output.
//
// On failure the returned error wraps the underlying error (so errors.Is works,
// including for ErrNotFound) and includes a trimmed snippet of the command's
// standard error, which is where git and gh put their actual "fatal: …"
// messages. This is safe because Arborist never passes credential URLs to git
// (cloning goes through gh), so stderr does not carry secrets.
func (OS) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := osexec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := trimStderr(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("run %s: %w: %s", name, err, msg)
		}
		return stdout.Bytes(), fmt.Errorf("run %s: %w", name, err)
	}
	return stdout.Bytes(), nil
}

// trimStderr tidies command stderr for inclusion in an error: it trims
// surrounding whitespace and caps the length so a runaway message can't flood
// the output.
func trimStderr(s string) string {
	s = strings.TrimSpace(s)
	const max = 500
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
