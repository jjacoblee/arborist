package cli

import (
	"github.com/jjacoblee/arborist/internal/exec"
	"github.com/jjacoblee/arborist/internal/picker"
)

// deps holds the external collaborators that commands need. Bundling them lets
// commands be constructed with fakes in tests instead of touching the real
// system (git, gh, the filesystem, the terminal).
type deps struct {
	// runner executes git and gh commands.
	runner exec.Runner
	// selector presents the interactive repository picker.
	selector picker.Selector
	// confirmer asks yes/no questions (e.g. before removing worktrees).
	confirmer picker.Confirmer
	// launcher starts an editor for "arb open", wired to the terminal.
	launcher exec.Launcher
	// shell runs configured setup commands in a worktree, wired to the terminal.
	shell exec.ShellRunner
}

// defaultDeps returns the production dependencies: the real OS command runner,
// the huh-backed interactive picker and confirmer, the OS editor launcher, and
// the OS shell for setup commands.
func defaultDeps() deps {
	return deps{
		runner:    exec.OS{},
		selector:  picker.Huh{},
		confirmer: picker.HuhConfirmer{},
		launcher:  exec.OSLauncher{},
		shell:     exec.OSShell{},
	}
}
