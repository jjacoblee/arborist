package picker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// ErrNoWorktrees indicates there were no worktrees to choose from.
var ErrNoWorktrees = errors.New("no worktrees available to select")

// WorktreeChoice is one worktree offered in the removal picker, carrying the
// state a user needs before choosing it.
type WorktreeChoice struct {
	// ID is the worktree's stable id; it is what the picker returns.
	ID     string
	Repo   string
	Branch string
	// Group is the group folder the worktree sits in, or "" when ungrouped.
	Group string
	// Dirty marks uncommitted changes or untracked files.
	Dirty bool
	// Unpushed marks commits that exist on no origin ref.
	Unpushed bool
}

// WorktreeSelector presents worktrees for removal and returns the ids chosen.
type WorktreeSelector interface {
	SelectWorktrees(ctx context.Context, choices []WorktreeChoice) ([]string, error)
}

// HuhWorktrees is a WorktreeSelector backed by huh: a searchable, multi-select
// checkbox prompt. The zero value is ready to use.
type HuhWorktrees struct{}

// SelectWorktrees runs the interactive picker. It returns ErrNoWorktrees if
// choices is empty, and ErrCanceled if the user aborts the prompt.
func (HuhWorktrees) SelectWorktrees(ctx context.Context, choices []WorktreeChoice) ([]string, error) {
	if len(choices) == 0 {
		return nil, ErrNoWorktrees
	}

	var selected []string
	field := huh.NewMultiSelect[string]().
		Title("Select worktrees to remove").
		Description("Space to select · Enter to confirm · Esc to cancel").
		Options(buildWorktreeOptions(choices)...).
		Filterable(true).
		Value(&selected)

	if err := huh.NewForm(huh.NewGroup(field)).RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, ErrCanceled
		}
		return nil, fmt.Errorf("run worktree picker: %w", err)
	}
	return selected, nil
}

// buildWorktreeOptions converts choices into huh options keyed by worktree id.
// The repository column is padded so branches line up, and any risky state is
// spelled out at the end of the line where it reads as a warning.
func buildWorktreeOptions(choices []WorktreeChoice) []huh.Option[string] {
	// A group column is only worth its width once something is grouped, which
	// mirrors how "arb list" decides.
	var repoWidth, groupWidth int
	for _, c := range choices {
		if len(c.Repo) > repoWidth {
			repoWidth = len(c.Repo)
		}
		if len(c.Group) > groupWidth {
			groupWidth = len(c.Group)
		}
	}

	options := make([]huh.Option[string], 0, len(choices))
	for _, c := range choices {
		branch := c.Branch
		if branch == "" {
			branch = "detached"
		}
		label := fmt.Sprintf("%-*s  %s", repoWidth, c.Repo, branch)
		if groupWidth > 0 {
			group := c.Group
			if group == "" {
				group = "-"
			}
			label = fmt.Sprintf("%-*s  %-*s  %s", groupWidth, group, repoWidth, c.Repo, branch)
		}
		if markers := stateMarkers(c); markers != "" {
			label += "  " + markers
		}
		options = append(options, huh.NewOption(label, c.ID))
	}
	return options
}

// stateMarkers renders a choice's risky state, in order of how much it should
// give the user pause.
func stateMarkers(c WorktreeChoice) string {
	var markers []string
	if c.Dirty {
		markers = append(markers, "[dirty]")
	}
	if c.Unpushed {
		markers = append(markers, "[unpushed]")
	}
	return strings.Join(markers, " ")
}
