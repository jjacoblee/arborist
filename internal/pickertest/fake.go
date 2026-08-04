// Package pickertest provides a fake picker.Selector for use in tests, so
// command logic that depends on interactive selection can be exercised without
// a terminal.
package pickertest

import (
	"context"

	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/picker"
)

// Fake is a non-interactive picker.Selector. It returns the configured Result
// and Err, and records what it was asked to present.
type Fake struct {
	Result []github.Repository
	Err    error

	// Recorded inputs from the most recent Select call.
	GotBranch string
	GotRepos  []github.Repository
	Calls     int
}

// Select implements picker.Selector.
func (f *Fake) Select(_ context.Context, branch string, repos []github.Repository) ([]github.Repository, error) {
	f.Calls++
	f.GotBranch = branch
	f.GotRepos = repos
	return f.Result, f.Err
}

// FakeWorktreeSelector is a non-interactive picker.WorktreeSelector for tests.
// It returns the configured Result and Err, and records what it was offered.
type FakeWorktreeSelector struct {
	Result []string
	Err    error

	// GotChoices records the worktrees offered in the most recent call.
	GotChoices []picker.WorktreeChoice
	Calls      int
}

// SelectWorktrees implements picker.WorktreeSelector.
func (f *FakeWorktreeSelector) SelectWorktrees(_ context.Context, choices []picker.WorktreeChoice) ([]string, error) {
	f.Calls++
	f.GotChoices = choices
	return f.Result, f.Err
}

// FakeConfirmer is a non-interactive picker.Confirmer for tests.
type FakeConfirmer struct {
	// Result answers every prompt not covered by Results.
	Result bool
	// Results answers prompts in order, one entry per Confirm call, so a flow
	// that asks more than once (for example "remove worktrees?" then "delete the
	// branch too?") can answer them differently. Once exhausted, Result applies.
	Results []bool
	Err     error

	// Recorded prompt from the most recent Confirm call.
	Asked string
	// Prompts records every prompt in order.
	Prompts []string
	Calls   int
}

// Confirm implements picker.Confirmer.
func (f *FakeConfirmer) Confirm(_ context.Context, prompt string) (bool, error) {
	f.Asked = prompt
	f.Prompts = append(f.Prompts, prompt)
	f.Calls++

	if f.Calls <= len(f.Results) {
		return f.Results[f.Calls-1], f.Err
	}
	return f.Result, f.Err
}
