package progress

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestBarNoOpOffTerminal ensures the bar writes nothing when its destination is
// not a terminal. This keeps captured output (pipes, tests, CI logs) free of
// escape codes and bar text.
func TestBarNoOpOffTerminal(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, 3)
	if b.tty {
		t.Fatal("a bytes.Buffer must not be detected as a terminal")
	}

	b.Start()
	b.Advance("acme/web-app")
	b.Advance("acme/api")
	b.Stop()

	if buf.Len() != 0 {
		t.Fatalf("expected no output off-terminal, got %q", buf.String())
	}
}

// TestRenderProportions checks the bar fill and counter track progress.
func TestRenderProportions(t *testing.T) {
	var buf bytes.Buffer
	b := &Bar{w: &buf, tty: true, total: 4}

	b.Advance("acme/web-app")
	got := buf.String()
	if !strings.Contains(got, "1/4") {
		t.Fatalf("counter missing 1/4 in %q", got)
	}
	if !strings.Contains(got, "acme/web-app") {
		t.Fatalf("label missing in %q", got)
	}
	if fills := strings.Count(got, string(fillGlyph)); fills != barWidth/4 {
		t.Fatalf("fill = %d cells, want %d", fills, barWidth/4)
	}

	buf.Reset()
	b.Advance("acme/api")
	if !strings.Contains(buf.String(), "2/4") {
		t.Fatalf("counter did not advance to 2/4: %q", buf.String())
	}
}

// TestAdvanceCapsAtTotal guards against the counter running past total if
// Advance is somehow called too many times.
func TestAdvanceCapsAtTotal(t *testing.T) {
	var buf bytes.Buffer
	b := &Bar{w: &buf, tty: true, total: 2}
	b.Advance("one")
	b.Advance("two")
	buf.Reset()
	b.Advance("three")
	if !strings.Contains(buf.String(), "2/2") {
		t.Fatalf("counter exceeded total: %q", buf.String())
	}
}

// TestShowDoesNotAdvance verifies Show renders the label at the current fill
// without moving the counter, so a step can be named while it runs and the bar
// only fills once Advance marks it done. This is what keeps the bar from reading
// full before any work has completed.
func TestShowDoesNotAdvance(t *testing.T) {
	var buf bytes.Buffer
	b := &Bar{w: &buf, tty: true, total: 1}

	b.Show("Setting up acme/web")
	got := buf.String()
	if !strings.Contains(got, "0/1") {
		t.Fatalf("Show should not advance the counter, want 0/1 in %q", got)
	}
	if !strings.Contains(got, "Setting up acme/web") {
		t.Fatalf("Show should render the label, got %q", got)
	}
	if fills := strings.Count(got, string(fillGlyph)); fills != 0 {
		t.Fatalf("bar should be empty before any Advance, got %d filled cells", fills)
	}

	buf.Reset()
	b.Advance("Setting up acme/web")
	if !strings.Contains(buf.String(), "1/1") {
		t.Fatalf("Advance after Show should reach 1/1, got %q", buf.String())
	}
}

// TestShowNoOpOffTerminal ensures Show, like the other methods, writes nothing
// when the destination is not a terminal.
func TestShowNoOpOffTerminal(t *testing.T) {
	var buf bytes.Buffer
	b := New(&buf, 2)
	b.Show("acme/web")
	if buf.Len() != 0 {
		t.Fatalf("expected no output off-terminal, got %q", buf.String())
	}
}

// TestAnimatedRenderShowsSpinnerAndSweep checks that while animating, the line
// carries a spinner glyph and a moving highlight, on top of the real fill. The
// counter must still be accurate.
func TestAnimatedRenderShowsSpinnerAndSweep(t *testing.T) {
	var buf bytes.Buffer
	b := &Bar{w: &buf, tty: true, total: 4, current: 1, started: true, frame: 0, label: "acme/web"}
	b.mu.Lock()
	b.renderLocked()
	b.mu.Unlock()

	got := buf.String()
	if !strings.ContainsRune(got, spinnerFrames[0]) {
		t.Fatalf("animated frame should include a spinner glyph, got %q", got)
	}
	if !strings.ContainsRune(got, shineFill) && !strings.ContainsRune(got, shineEmpty) {
		t.Fatalf("animated frame should include the sweep highlight, got %q", got)
	}
	if !strings.Contains(got, "1/4") {
		t.Fatalf("animation must not lose the counter, got %q", got)
	}
}

// TestSpinnerAdvancesWithFrame confirms the spinner glyph changes as the frame
// counter advances, so the animation is actually visible rather than fixed.
func TestSpinnerAdvancesWithFrame(t *testing.T) {
	var buf bytes.Buffer
	b := &Bar{w: &buf, tty: true, total: 4, started: true, label: "x"}

	b.mu.Lock()
	b.frame = 0
	b.renderLocked()
	first := buf.String()
	buf.Reset()
	b.frame = 1
	b.renderLocked()
	second := buf.String()
	b.mu.Unlock()

	if first == second {
		t.Fatalf("expected the frame to change the rendered line, both were %q", first)
	}
	if !strings.ContainsRune(second, spinnerFrames[1%len(spinnerFrames)]) {
		t.Fatalf("frame 1 should show the second spinner glyph, got %q", second)
	}
}

// syncWriter is a goroutine-safe writer so the live-animation test can read what
// the background animator writes without racing on a bytes.Buffer.
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncWriter) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestStartAnimatesThenStopsCleanly drives the real background animator: Start
// must emit frames over time without an Advance, and Stop must halt it, restore
// the cursor, and return promptly. Run under -race, this also guards the
// goroutine's access to shared state.
func TestStartAnimatesThenStopsCleanly(t *testing.T) {
	w := &syncWriter{}
	b := &Bar{w: w, tty: true, total: 4} // tty forced; the writer is not a real terminal

	b.Start()
	b.Show("acme/web") // a step is running; the fill should not move on its own
	time.Sleep(4 * frameInterval)

	mid := w.String()
	if !strings.ContainsRune(mid, spinnerFrames[0]) && !strings.ContainsRune(mid, spinnerFrames[1]) {
		t.Fatalf("expected animation frames while running, got %q", mid)
	}
	if strings.Contains(mid, "1/4") {
		t.Fatalf("the bar should not advance without Advance, got %q", mid)
	}

	done := make(chan struct{})
	go func() { b.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return; the animator likely deadlocked")
	}

	if !strings.Contains(w.String(), showCursor) {
		t.Fatalf("Stop should restore the cursor, got %q", w.String())
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 40, "short"},
		{"exactly-ten", 11, "exactly-ten"},
		{"this-label-is-quite-long", 10, "this-labe…"},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.max); got != tt.want {
			t.Fatalf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}
