// Package picker provides interactive selection of repositories.
//
// It depends on an interactive prompt library (huh) but exposes a small
// Selector interface so callers can substitute a non-interactive implementation
// in tests. The picker only chooses repositories; it never clones or creates
// worktrees.
package picker

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/jjacoblee/arborist/internal/github"
)

var (
	// ErrCanceled indicates the user canceled the selection (Esc or Ctrl+C).
	ErrCanceled = errors.New("repository selection canceled")
	// ErrNoRepositories indicates there were no repositories to choose from.
	ErrNoRepositories = errors.New("no repositories available to select")
)

// Selector presents repositories for a branch and returns the chosen subset.
type Selector interface {
	Select(ctx context.Context, branch string, repos []github.Repository) ([]github.Repository, error)
}

// Huh is a Selector backed by the huh library: a searchable, multi-select
// checkbox prompt. The zero value is ready to use.
type Huh struct{}

// Select runs the interactive picker. It returns ErrNoRepositories if repos is
// empty, and ErrCanceled if the user aborts the prompt.
func (Huh) Select(ctx context.Context, branch string, repos []github.Repository) ([]github.Repository, error) {
	if len(repos) == 0 {
		return nil, ErrNoRepositories
	}

	options, index := buildOptions(repos)

	var selectedKeys []string
	field := huh.NewMultiSelect[string]().
		Title(fmt.Sprintf("Select repositories for branch: %s", branch)).
		Description("Space to select · Enter to confirm · Esc to cancel").
		Options(options...).
		Filterable(true).
		Value(&selectedKeys)

	if err := huh.NewForm(huh.NewGroup(field)).RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, ErrCanceled
		}
		return nil, fmt.Errorf("run repository picker: %w", err)
	}

	return resolve(selectedKeys, index), nil
}

// buildOptions converts repositories into huh options keyed by nameWithOwner and
// returns a lookup from that key back to the repository. The key is stable and
// unique, which makes the mapping back unambiguous.
func buildOptions(repos []github.Repository) ([]huh.Option[string], map[string]github.Repository) {
	options := make([]huh.Option[string], 0, len(repos))
	index := make(map[string]github.Repository, len(repos))
	for _, r := range repos {
		label := r.NameWithOwner
		if r.IsPrivate {
			label += " (private)"
		}
		options = append(options, huh.NewOption(label, r.NameWithOwner))
		index[r.NameWithOwner] = r
	}
	return options, index
}

// resolve maps the selected keys back to repositories, preserving selection
// order and skipping any key that is not recognized.
func resolve(keys []string, index map[string]github.Repository) []github.Repository {
	selected := make([]github.Repository, 0, len(keys))
	for _, k := range keys {
		if r, ok := index[k]; ok {
			selected = append(selected, r)
		}
	}
	return selected
}
