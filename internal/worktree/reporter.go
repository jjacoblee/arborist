package worktree

// Reporter receives progress events from the long-running Service operations
// (Create, Remove, Prune) so a caller can render them. It is deliberately
// abstract — plain-text events with no glyphs or terminal concerns — so this
// package stays free of any rendering decisions.
//
// The lifecycle is: Start once with the total number of items; for each item
// Step (naming it, just before the work) followed by Done (once the work
// finishes); and finally Stop. In between, the service narrates outcomes:
// Log for a completed action ("cloned acme/api"), Note for a neutral outcome
// ("skipped acme/api — worktree path already exists"), and Fail for a failed
// one. A renderer typically prints Log/Note/Fail as permanent lines and shows
// the most recent Step as a transient "in flight" indicator. A nil Reporter
// disables reporting.
type Reporter interface {
	Start(total int)
	// Step names the action about to run (e.g. "cloning acme/api"). It is
	// transient: a renderer should show it while the action runs, not record it.
	Step(label string)
	// Log records a completed action worth a permanent line, e.g.
	// "created worktree api/feature-x".
	Log(event string)
	// Note records a neutral, non-error outcome worth a permanent line, e.g. a
	// skip.
	Note(event string)
	// Fail records a failed action worth a permanent line.
	Fail(event string)
	// Done marks the current item (the most recent top-level Step) finished, so
	// a renderer advances only after the work actually completes.
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
func (noopReporter) Log(string)  {}
func (noopReporter) Note(string) {}
func (noopReporter) Fail(string) {}
func (noopReporter) Done()       {}
func (noopReporter) Stop()       {}
