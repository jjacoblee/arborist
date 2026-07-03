// Package progress renders a small progress bar while `arb new` works through
// the selected repositories, so the slow steps (cloning and fetching) don't
// look like a hang.
//
// The bar is determinate — discrete Advance calls fill it as steps complete —
// but it also animates: once Start launches it, a background ticker redraws a
// cycling spinner and a highlight that sweeps across the bar, so the line keeps
// moving during a single long step (e.g. an install) instead of freezing. The
// animation rides on top of the real fill; it never misrepresents progress.
//
// The bar draws only to an interactive terminal: when the destination is not a
// TTY (tests, pipes, CI logs, output redirection) every method is a no-op so
// captured output stays clean and free of escape codes.
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

	barWidth = 24 // characters in the bar itself
	maxLabel = 40 // truncate the trailing label so the line stays on one row

	// Bar cell glyphs. The fill is solid/empty; the sweeping highlight uses an
	// intermediate shade so it reads as a shimmer over the fill and a moving dot
	// over the empty track.
	fillGlyph  = '█'
	emptyGlyph = '░'
	shineFill  = '▓' // highlight while passing over a filled cell
	shineEmpty = '▒' // highlight while passing over an empty cell

	// frameInterval is how often the animation advances. ~12 fps is smooth
	// without flooding the terminal.
	frameInterval = 80 * time.Millisecond
)

// spinnerFrames is a braille spinner cycle — the little glyph that spins to show
// the command is still working even when the fill hasn't moved.
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Bar is a single-line, animated progress bar over a known number of steps.
// Create one with New, Start it (which begins the animation), call Show to name
// the step about to run, Advance once per completed step, and Stop when done.
//
// Show/Advance/Stop are called from the work goroutine while a background
// goroutine drives the animation, so all access to the mutable fields and all
// writes to w are guarded by mu.
type Bar struct {
	w   io.Writer
	tty bool

	mu      sync.Mutex
	total   int
	current int
	label   string
	frame   int
	started bool

	stop chan struct{} // closed by Stop to tell the animator to exit
	done chan struct{} // closed by the animator when it has exited
}

// New returns a Bar that draws to w over total steps. If w is not a terminal the
// Bar is inert and all of its methods do nothing.
func New(w io.Writer, total int) *Bar {
	return &Bar{w: w, tty: isTerminal(w), total: total}
}

// Start hides the cursor, draws the bar, and launches the animation. It is a
// no-op on a non-terminal writer or if already started.
func (b *Bar) Start() {
	if !b.tty {
		return
	}
	b.mu.Lock()
	if b.started {
		b.mu.Unlock()
		return
	}
	b.started = true
	b.stop = make(chan struct{})
	b.done = make(chan struct{})
	io.WriteString(b.w, hideCursor)
	b.renderLocked()
	b.mu.Unlock()

	go b.animate()
}

// animate redraws the bar on every tick so the spinner and sweep keep moving
// between Advance calls. It exits when Stop closes b.stop.
func (b *Bar) animate() {
	defer close(b.done)
	t := time.NewTicker(frameInterval)
	defer t.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			b.mu.Lock()
			b.frame++
			b.renderLocked()
			b.mu.Unlock()
		}
	}
}

// Show renders label at the current position without advancing, naming the step
// that is about to run while the bar still reflects only completed work. Pair it
// with Advance (call Show before the step, Advance after it finishes) so the bar
// never reads ahead of the work. A no-op when inert.
func (b *Bar) Show(label string) {
	if !b.tty {
		return
	}
	b.mu.Lock()
	b.label = label
	b.renderLocked()
	b.mu.Unlock()
}

// Advance moves the bar forward one step and shows label (typically the
// repository being processed). Call it after a step completes so the fill
// reflects finished work, not work in progress. A no-op when inert.
func (b *Bar) Advance(label string) {
	if !b.tty {
		return
	}
	b.mu.Lock()
	if b.current < b.total {
		b.current++
	}
	b.label = label
	b.renderLocked()
	b.mu.Unlock()
}

// Stop halts the animation, erases the bar, and restores the cursor so following
// output starts on a clean line. It waits for the animator to exit before
// clearing, so no stray frame can be drawn afterward. Safe to call more than
// once.
func (b *Bar) Stop() {
	if !b.tty {
		return
	}
	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()
		return
	}
	b.started = false
	stop, done := b.stop, b.done
	b.mu.Unlock()

	close(stop)
	<-done // ensure the animator has stopped touching w

	b.mu.Lock()
	io.WriteString(b.w, "\r"+clearLine+showCursor)
	b.mu.Unlock()
}

// renderLocked draws the current frame. The caller must hold b.mu (the public
// methods and the animator all do).
func (b *Bar) renderLocked() {
	filled := 0
	if b.total > 0 {
		filled = b.current * barWidth / b.total
	}

	cells := make([]rune, barWidth)
	for i := range cells {
		if i < filled {
			cells[i] = fillGlyph
		} else {
			cells[i] = emptyGlyph
		}
	}

	prefix := ""
	if b.started {
		// Overlay the sweeping highlight and prepend the spinner only while the
		// animation is running, so a one-shot render (Advance without Start, as in
		// tests) stays a clean, static bar.
		pos := b.frame % barWidth
		if cells[pos] == fillGlyph {
			cells[pos] = shineFill
		} else {
			cells[pos] = shineEmpty
		}
		prefix = string(spinnerFrames[b.frame%len(spinnerFrames)]) + " "
	}

	line := fmt.Sprintf("\r%s%s[%s] %d/%d", clearLine, prefix, string(cells), b.current, b.total)
	if b.label != "" {
		line += " · " + truncate(b.label, maxLabel)
	}
	io.WriteString(b.w, line)
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
