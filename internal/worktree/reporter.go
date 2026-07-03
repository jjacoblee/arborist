package worktree

// Reporter receives progress updates from the long-running Service operations
// (Create, Remove, Prune) so a caller can render an indicator. It is deliberately
// abstract — Start/Step/Done/Stop with a count and labels — so this package stays
// free of any terminal or rendering concerns.
//
// The lifecycle is: Start once with the total number of items, then for each item
// Step (naming it, just before the work) followed by Done (once the work
// finishes), and finally Stop. Splitting "starting" (Step) from "finished" (Done)
// lets a renderer show the current item without counting it complete, so an
// indicator never reads ahead of the actual work. A nil Reporter disables
// reporting.
type Reporter interface {
	Start(total int)
	// Step names the item about to be processed. It does not mark progress; a
	// renderer should show the label but not yet count the item as done.
	Step(label string)
	// Done marks the current item (the most recent Step) finished, so a renderer
	// advances only after the work actually completes.
	Done()
	Stop()
}

// report returns the Service's Reporter, or a no-op if none is set, so callers
// can report unconditionally.
func (s Service) report() Reporter {
	if s.Progress == nil {
		return noopReporter{}
	}
	return s.Progress
}

type noopReporter struct{}

func (noopReporter) Start(int)   {}
func (noopReporter) Step(string) {}
func (noopReporter) Done()       {}
func (noopReporter) Stop()       {}
