// Package pickertest provides a fake picker.Selector for use in tests, so
// command logic that depends on interactive selection can be exercised without
// a terminal.
package pickertest

import (
	"context"

	"github.com/jjacoblee/arborist/internal/github"
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

// FakeConfirmer is a non-interactive picker.Confirmer for tests.
type FakeConfirmer struct {
	Result bool
	Err    error

	// Recorded prompt from the most recent Confirm call.
	Asked string
	Calls int
}

// Confirm implements picker.Confirmer.
func (f *FakeConfirmer) Confirm(_ context.Context, prompt string) (bool, error) {
	f.Calls++
	f.Asked = prompt
	return f.Result, f.Err
}
