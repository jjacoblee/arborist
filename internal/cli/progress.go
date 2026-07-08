package cli

import (
	"io"

	"github.com/jjacoblee/arborist/internal/progress"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// stepsReporter adapts progress.Steps to worktree.Reporter, rendering the
// service's events as permanent step lines — "✓ cloned acme/api",
// "• skipped acme/web — worktree path already exists", "✗ acme/api: …" — with a
// transient spinner naming the action in flight. It writes to w — the command's
// stderr — and is inert when w is not a terminal, keeping piped and test output
// clean; the final summary on stdout remains the machine-readable record.
type stepsReporter struct {
	steps *progress.Steps
}

// newStepsReporter returns a worktree.Reporter that draws to w.
func newStepsReporter(w io.Writer) *stepsReporter {
	return &stepsReporter{steps: progress.NewSteps(w)}
}

func (s *stepsReporter) Start(int) { s.steps.Start() }

// Step shows the action in flight on the transient spinner line.
func (s *stepsReporter) Step(label string) { s.steps.SetLabel(label) }

// Log, Note, and Fail append permanent lines; the glyph is chosen here so the
// worktree package stays free of rendering concerns.
func (s *stepsReporter) Log(event string)  { s.steps.Println("✓ " + event) }
func (s *stepsReporter) Note(event string) { s.steps.Println("• " + event) }
func (s *stepsReporter) Fail(event string) { s.steps.Println("✗ " + event) }

// Done clears the transient line between items so a finished item's label never
// lingers while the next one starts.
func (s *stepsReporter) Done() { s.steps.SetLabel("") }

func (s *stepsReporter) Stop() { s.steps.Stop() }

// compile-time check that stepsReporter satisfies the worktree.Reporter contract.
var _ worktree.Reporter = (*stepsReporter)(nil)
