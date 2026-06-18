// Package github integrates with GitHub through the GitHub CLI (gh).
//
// Arborist never manages GitHub credentials itself: authentication is delegated
// entirely to gh, and this package only runs gh commands and parses their
// output.
package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/jjacoblee/arborist/internal/exec"
)

var (
	// ErrNotInstalled indicates the gh executable was not found on PATH.
	ErrNotInstalled = errors.New("the GitHub CLI (gh) is not installed or not on PATH")
	// ErrNotAuthenticated indicates gh is installed but not logged in.
	ErrNotAuthenticated = errors.New("the GitHub CLI (gh) is not authenticated")
)

// Client runs GitHub CLI commands via an exec.Runner.
type Client struct {
	runner exec.Runner
}

// New returns a Client backed by runner. Pass exec.OS{} in production.
func New(runner exec.Runner) Client {
	return Client{runner: runner}
}

// EnsureInstalled verifies that gh is available by running "gh --version".
// If gh is not on PATH it returns ErrNotInstalled.
func (c Client) EnsureInstalled(ctx context.Context) error {
	if _, err := c.runner.Run(ctx, "gh", "--version"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrNotInstalled
		}
		return fmt.Errorf("check gh installation: %w", err)
	}
	return nil
}

// EnsureAuthenticated verifies that gh is logged in by running "gh auth status",
// which exits non-zero when no account is authenticated. If gh is missing it
// returns ErrNotInstalled; if it is installed but not logged in it returns
// ErrNotAuthenticated.
func (c Client) EnsureAuthenticated(ctx context.Context) error {
	if _, err := c.runner.Run(ctx, "gh", "auth", "status"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrNotInstalled
		}
		return ErrNotAuthenticated
	}
	return nil
}
