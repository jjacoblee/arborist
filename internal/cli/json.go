package cli

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/jjacoblee/arborist/internal/config"
	"github.com/jjacoblee/arborist/internal/worktree"
)

// The --json output shapes in this file are a stable, machine-readable
// contract for scripts and coding agents. Treat them as public API: fields may
// be added, but never renamed, removed, or repurposed. Slices are always
// emitted as arrays (never null) so consumers can range over them without nil
// checks.

// worktreeJSON is one managed worktree. It is shared by `arb list --json` and
// `arb remove --json` so consumers parse a single shape.
type worktreeJSON struct {
	ID         string `json:"id"`              // full id; prefixes work with "arb remove"/"arb open"
	Repository string `json:"repository"`      // owner/name
	Branch     string `json:"branch"`          // "" when detached or unknown
	Status     string `json:"status"`          // "clean", "dirty", or "broken"
	Path       string `json:"path"`            // absolute worktree path
	Error      string `json:"error,omitempty"` // git's error when status is "broken"
}

// toWorktreeJSON converts a managed worktree, deriving the same status label
// the human table shows.
func toWorktreeJSON(wt worktree.ManagedWorktree) worktreeJSON {
	repo := wt.Repo
	if wt.Owner != "" {
		repo = wt.Owner + "/" + wt.Repo
	}
	j := worktreeJSON{
		ID:         wt.ID,
		Repository: repo,
		Branch:     wt.Branch,
		Status:     "clean",
		Path:       wt.Path,
	}
	if wt.Dirty {
		j.Status = "dirty"
	}
	if wt.Err != nil {
		j.Status = "broken"
		j.Error = wt.Err.Error()
	}
	return j
}

// writeListJSON emits the `arb list --json` output: a JSON array of worktrees.
func writeListJSON(w io.Writer, worktrees []worktree.ManagedWorktree) error {
	out := make([]worktreeJSON, 0, len(worktrees))
	for _, wt := range worktrees {
		out = append(out, toWorktreeJSON(wt))
	}
	return encodeJSON(w, out)
}

// setupJSON reports what happened to a created worktree's configured setup
// commands.
type setupJSON struct {
	// Status is "ok" (all commands succeeded), "failed" (a command failed; see
	// Command and Output), "none" (no setup commands are configured for the
	// repository), or "skipped" (--no-setup was passed).
	Status  string `json:"status"`
	Command string `json:"command,omitempty"` // the failing command
	Output  string `json:"output,omitempty"`  // tail of the failing command's output
}

type createdWorktreeJSON struct {
	Repository  string    `json:"repository"` // owner/name
	Branch      string    `json:"branch"`
	Path        string    `json:"path"`   // absolute worktree path
	Source      string    `json:"source"` // where the branch came from
	CopiedEnv   []string  `json:"copiedEnv,omitempty"`
	CopiedFiles []string  `json:"copiedFiles,omitempty"`
	Setup       setupJSON `json:"setup"`
}

type skippedWorktreeJSON struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Path       string `json:"path,omitempty"` // existing path, when relevant
	Reason     string `json:"reason"`
}

type failedWorktreeJSON struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Error      string `json:"error"`
}

type newResultJSON struct {
	Created []createdWorktreeJSON `json:"created"`
	Skipped []skippedWorktreeJSON `json:"skipped"`
	Failed  []failedWorktreeJSON  `json:"failed"`
}

// writeNewJSON emits the `arb new --json` output: every created worktree with
// its absolute path and setup outcome, plus skipped and failed repositories.
// A non-empty "failed" array is also reflected in the exit code, so consumers
// may rely on either.
func writeNewJSON(w io.Writer, result worktree.CreateResult, cfg config.Config, noSetup bool, failures []setupFailure) error {
	failureFor := func(repo, branch string) *setupFailure {
		for i := range failures {
			if failures[i].repo == repo && failures[i].branch == branch {
				return &failures[i]
			}
		}
		return nil
	}

	out := newResultJSON{
		Created: make([]createdWorktreeJSON, 0, len(result.Created)),
		Skipped: make([]skippedWorktreeJSON, 0, len(result.Skipped)),
		Failed:  make([]failedWorktreeJSON, 0, len(result.Failed)),
	}
	for _, c := range result.Created {
		setup := setupJSON{Status: "ok"}
		switch {
		case noSetup:
			setup.Status = "skipped"
		case len(cfg.SetupCommands(c.Repository.Name)) == 0:
			setup.Status = "none"
		default:
			if f := failureFor(c.Repository.NameWithOwner, c.Branch); f != nil {
				setup = setupJSON{
					Status:  "failed",
					Command: f.command,
					Output:  strings.Join(tailLines(f.output, 20), "\n"),
				}
			}
		}
		out.Created = append(out.Created, createdWorktreeJSON{
			Repository:  c.Repository.NameWithOwner,
			Branch:      c.Branch,
			Path:        c.Path,
			Source:      string(c.Source),
			CopiedEnv:   c.CopiedEnv,
			CopiedFiles: c.CopiedFiles,
			Setup:       setup,
		})
	}
	for _, s := range result.Skipped {
		out.Skipped = append(out.Skipped, skippedWorktreeJSON{
			Repository: s.Repository.NameWithOwner,
			Branch:     s.Branch,
			Path:       s.Path,
			Reason:     s.Reason,
		})
	}
	for _, f := range result.Failed {
		out.Failed = append(out.Failed, failedWorktreeJSON{
			Repository: f.Repository.NameWithOwner,
			Branch:     f.Branch,
			Error:      f.Err.Error(),
		})
	}
	return encodeJSON(w, out)
}

type removeSkippedJSON struct {
	Worktree worktreeJSON `json:"worktree"`
	Reason   string       `json:"reason"`
}

type removeFailedJSON struct {
	Worktree worktreeJSON `json:"worktree"`
	Error    string       `json:"error"`
}

type removeResultJSON struct {
	Removed []worktreeJSON      `json:"removed"`
	Skipped []removeSkippedJSON `json:"skipped"`
	Failed  []removeFailedJSON  `json:"failed"`
}

// writeRemoveJSON emits the `arb remove --json` output. Dirty worktrees that
// were not removed appear under "skipped" with a reason; a non-empty "failed"
// array is also reflected in the exit code.
func writeRemoveJSON(w io.Writer, result worktree.RemoveResult) error {
	out := removeResultJSON{
		Removed: make([]worktreeJSON, 0, len(result.Removed)),
		Skipped: make([]removeSkippedJSON, 0, len(result.Skipped)),
		Failed:  make([]removeFailedJSON, 0, len(result.Failed)),
	}
	for _, wt := range result.Removed {
		out.Removed = append(out.Removed, toWorktreeJSON(wt))
	}
	for _, s := range result.Skipped {
		out.Skipped = append(out.Skipped, removeSkippedJSON{Worktree: toWorktreeJSON(s.Worktree), Reason: s.Reason})
	}
	for _, f := range result.Failed {
		out.Failed = append(out.Failed, removeFailedJSON{Worktree: toWorktreeJSON(f.Worktree), Error: f.Err.Error()})
	}
	return encodeJSON(w, out)
}

// encodeJSON writes v as indented JSON with a trailing newline.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
