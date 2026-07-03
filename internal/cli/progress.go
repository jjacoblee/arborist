package cli

import (
	"io"

	"github.com/jjacoblee/arborist/internal/progress"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// barReporter adapts a progress.Bar to worktree.Reporter. The bar is created
// once the total is known (in Start), so the service can drive a determinate
// progress bar for new, remove, and prune. It writes to w — the command's
// stderr — and is inert when w is not a terminal, keeping piped and test output
// clean.
//
// It maps the Reporter's Step (name the current item) to the bar's Show and Done
// (item finished) to Advance, so the bar fills only as work completes and never
// reads full while the last, often slowest, item is still running.
type barReporter struct {
	w     io.Writer
	bar   *progress.Bar
	label string
}

// newBarReporter returns a worktree.Reporter that draws to w.
func newBarReporter(w io.Writer) *barReporter {
	return &barReporter{w: w}
}

func (b *barReporter) Start(total int) {
	if total <= 0 {
		return
	}
	b.bar = progress.New(b.w, total)
	b.bar.Start()
}

// Step names the item about to be processed. It shows the label on the bar
// without advancing it, so the fill keeps reflecting only completed work (see
// Done). The label is remembered so Done can keep it on screen as the bar moves.
func (b *barReporter) Step(label string) {
	b.label = label
	if b.bar != nil {
		b.bar.Show(label)
	}
}

// Done marks the current item finished, advancing the bar one step while keeping
// the item's label on screen. Advancing here rather than in Step is what keeps
// the bar from reading full while the last, often slowest, item is still running.
func (b *barReporter) Done() {
	if b.bar != nil {
		b.bar.Advance(b.label)
	}
}

func (b *barReporter) Stop() {
	if b.bar != nil {
		b.bar.Stop()
		b.bar = nil
	}
}

// compile-time check that barReporter satisfies the worktree.Reporter contract.
var _ worktree.Reporter = (*barReporter)(nil)
