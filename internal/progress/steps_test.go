package progress

import (
	"bytes"
	"strings"
	"testing"
)

// TestStepsNoOpOffTerminal ensures Steps writes nothing when its destination is
// not a terminal. This keeps captured output (pipes, tests, CI logs) free of
// escape codes and progress text.
func TestStepsNoOpOffTerminal(t *testing.T) {
	var buf bytes.Buffer
	s := NewSteps(&buf)
	if s.tty {
		t.Fatal("a bytes.Buffer must not be detected as a terminal")
	}

	s.Start()
	s.SetLabel("cloning acme/api")
	s.Println("✓ cloned acme/api")
	s.Stop()

	if buf.Len() != 0 {
		t.Fatalf("expected no output off-terminal, got %q", buf.String())
	}
}

// TestPrintlnKeepsPermanentRecord checks that Println emits the line followed
// by a redraw of the transient label, so completed actions stack above the
// in-flight one.
func TestPrintlnKeepsPermanentRecord(t *testing.T) {
	var buf bytes.Buffer
	s := &Steps{w: &buf, tty: true}

	s.SetLabel("cloning acme/api")
	s.Println("✓ acme/web-app already cloned")

	got := buf.String()
	if !strings.Contains(got, "✓ acme/web-app already cloned\n") {
		t.Fatalf("permanent line missing from %q", got)
	}
	// The transient label must be redrawn after the permanent line.
	idxLine := strings.Index(got, "✓ acme/web-app already cloned\n")
	if !strings.Contains(got[idxLine:], "cloning acme/api") {
		t.Fatalf("transient label not redrawn after Println: %q", got)
	}
}

// TestSetLabelReplacesTransientLine checks the transient line is overwritten in
// place (carriage return + clear), not appended.
func TestSetLabelReplacesTransientLine(t *testing.T) {
	var buf bytes.Buffer
	s := &Steps{w: &buf, tty: true}

	s.SetLabel("cloning acme/api")
	s.SetLabel("fetching acme/api")

	got := buf.String()
	if strings.Contains(got, "cloning acme/api\n") {
		t.Fatalf("transient label must not end with a newline: %q", got)
	}
	if !strings.Contains(got, "\r"+clearLine+"fetching acme/api") {
		t.Fatalf("second label must overwrite the first: %q", got)
	}
}

// TestEmptyLabelClearsLine checks that clearing the label erases the transient
// line entirely.
func TestEmptyLabelClearsLine(t *testing.T) {
	var buf bytes.Buffer
	s := &Steps{w: &buf, tty: true}

	s.SetLabel("cloning acme/api")
	buf.Reset()
	s.SetLabel("")

	if got := buf.String(); got != "\r"+clearLine {
		t.Fatalf("empty label should just clear the line, got %q", got)
	}
}

// TestTruncate guards the label truncation used to keep the transient line on
// one row.
func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{name: "shorter than max", input: "abc", max: 5, want: "abc"},
		{name: "exactly max", input: "abcde", max: 5, want: "abcde"},
		{name: "longer than max", input: "abcdef", max: 5, want: "abcd…"},
		{name: "max of one", input: "abc", max: 1, want: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.input, tt.max); got != tt.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}
