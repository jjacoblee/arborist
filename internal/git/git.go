// Package git runs low-level Git commands through an exec.Runner.
//
// It deliberately knows nothing about terminal prompts, configuration, or
// higher-level Arborist workflows; it only constructs and runs git commands and
// parses their output.
package git

import (
	"context"
	"errors"
	"fmt"

	"github.com/jjacoblee/arborist/internal/exec"
)

// ErrNotInstalled indicates the git executable was not found on PATH.
var ErrNotInstalled = errors.New("git is not installed or not on PATH")

// Client runs Git commands via an exec.Runner.
type Client struct {
	runner exec.Runner
}

// New returns a Client backed by runner. Pass exec.OS{} in production.
func New(runner exec.Runner) Client {
	return Client{runner: runner}
}

// EnsureInstalled verifies that git is available by running "git --version".
// If git is not on PATH it returns ErrNotInstalled.
func (c Client) EnsureInstalled(ctx context.Context) error {
	if _, err := c.runner.Run(ctx, "git", "--version"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrNotInstalled
		}
		return fmt.Errorf("check git installation: %w", err)
	}
	return nil
}
