// Package progress renders step-by-step progress for Arborist's long-running
// commands: permanent one-line records of completed actions ("✓ cloned
// acme/api") plus a transient spinner line naming the action in flight, so slow
// steps (cloning, fetching, installing) never look like a hang.
//
// Output goes to stderr and only when it is an interactive terminal: when the
// destination is not a TTY (tests, pipes, CI logs, redirection) every method is
// a no-op, keeping captured output clean. The machine-readable record remains
// the command's final summary on stdout.
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
	clearLine  = "\x1b[K" // erase from cursor to end of line

	maxLabel = 60 // truncate the transient label so the line stays on one row

	// frameInterval is how often the spinner advances. ~12 fps is smooth
	// without flooding the terminal.
	frameInterval = 80 * time.Millisecond
)

// spinnerFrames is a braille spinner cycle shown next to the in-flight action.
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Steps renders progress as a stream of permanent lines with one transient
// spinner line underneath for the action currently running.
//
// Create one with NewSteps, Start it (which begins the spinner animation), call
// SetLabel to name the action in flight, Println to append a permanent line,
// and Stop when done (which erases the transient line, leaving only the
// permanent record).
//
// SetLabel/Println/Stop are called from the work goroutine while a background
// goroutine drives the spinner, so all mutable state and all writes to w are
// guarded by mu.
type Steps struct {
	w   io.Writer
	tty bool

	mu      sync.Mutex
	label   string // transient: the action currently in flight ("" for none)
	frame   int
	started bool

	stop chan struct{} // closed by Stop to tell the animator to exit
	done chan struct{} // closed by the animator when it has exited
}

// NewSteps returns a Steps that draws to w. If w is not a terminal the value is
// inert and all of its methods do nothing.
func NewSteps(w io.Writer) *Steps {
	return &Steps{w: w, tty: isTerminal(w)}
}

// Start hides the cursor and launches the spinner animation. A no-op on a
// non-terminal writer or if already started.
func (s *Steps) Start() {
	if !s.tty {
		return
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	io.WriteString(s.w, hideCursor)
	s.renderLocked()
	s.mu.Unlock()

	go s.animate()
}

// animate redraws the transient line on every tick so the spinner keeps moving
// during a single long action. It exits when Stop closes s.stop.
func (s *Steps) animate() {
	defer close(s.done)
	t := time.NewTicker(frameInterval)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			s.frame++
			s.renderLocked()
			s.mu.Unlock()
		}
	}
}

// SetLabel names the action currently in flight, shown on the transient spinner
// line (e.g. "cloning acme/api"). An empty label hides the line until the next
// SetLabel. A no-op when inert.
func (s *Steps) SetLabel(label string) {
	if !s.tty {
		return
	}
	s.mu.Lock()
	s.label = label
	s.renderLocked()
	s.mu.Unlock()
}

// Println appends line to the permanent record above the transient spinner
// line: the spinner line is erased, the line is printed with a trailing
// newline, and the spinner line is redrawn beneath it. A no-op when inert.
func (s *Steps) Println(line string) {
	if !s.tty {
		return
	}
	s.mu.Lock()
	fmt.Fprintf(s.w, "\r%s%s\n", clearLine, line)
	s.renderLocked()
	s.mu.Unlock()
}

// Stop halts the animation and erases the transient line, so following output
// starts on a clean line and only the permanent record remains. It waits for
// the animator to exit before clearing, so no stray frame can be drawn
// afterward. Safe to call more than once.
func (s *Steps) Stop() {
	if !s.tty {
		return
	}
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	stop, done := s.stop, s.done
	s.mu.Unlock()

	close(stop)
	<-done // ensure the animator has stopped touching w

	s.mu.Lock()
	io.WriteString(s.w, "\r"+clearLine+showCursor)
	s.mu.Unlock()
}

// renderLocked draws the transient spinner line. The caller must hold s.mu (the
// public methods and the animator all do).
func (s *Steps) renderLocked() {
	if s.label == "" {
		io.WriteString(s.w, "\r"+clearLine)
		return
	}
	prefix := ""
	if s.started {
		// Show the spinner only while the animation runs, so a one-shot render
		// (SetLabel without Start, as in tests) stays static.
		prefix = string(spinnerFrames[s.frame%len(spinnerFrames)]) + " "
	}
	fmt.Fprintf(s.w, "\r%s%s%s", clearLine, prefix, truncate(s.label, maxLabel))
}

// truncate shortens s to at most max runes, marking the cut with an ellipsis.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// isTerminal reports whether w is an interactive terminal. A terminal is a
// character device, whereas a pipe, regular file, or in-memory buffer is not;
// this stdlib-only check avoids pulling in an extra dependency.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
