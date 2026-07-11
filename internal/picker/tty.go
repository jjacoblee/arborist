package picker

import (
	"errors"
	"os"
)

// ErrNotATerminal indicates an interactive prompt was needed but the session
// has no terminal to render it on — for example when Arborist is run by a
// script, a CI job, or a coding agent. Prompts fail fast with this error
// instead of handing huh a stream it cannot draw on; callers translate it into
// a command-specific hint about the non-interactive alternative (naming
// repositories, passing --yes).
var ErrNotATerminal = errors.New("interactive prompt requires a terminal")

// interactiveSession reports whether both stdin and stdout are terminals, i.e.
// whether huh can actually render a prompt and receive keystrokes. It is a
// package variable so tests can force the non-interactive path
// deterministically regardless of how the test binary is run.
var interactiveSession = func() bool {
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

// isTerminal reports whether f is a character device (a terminal). It uses
// only the standard library, so the check behaves the same on every platform.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
