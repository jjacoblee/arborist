package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
)

// Actionable, user-facing messages for missing prerequisites. They live at the
// CLI boundary on purpose: the git and github packages report typed errors,
// and this layer turns them into guidance.
var (
	errGitNotInstalled = errors.New(
		"git is required but was not found.\n\n" +
			"Install Git from https://git-scm.com/downloads and try again.")
	errGHNotInstalled = errors.New(
		"the GitHub CLI (gh) is required but was not found.\n\n" +
			"Install it from https://cli.github.com/ and try again.")
	errGHNotAuthenticated = errors.New(
		"the GitHub CLI (gh) is not authenticated.\n\n" +
			"Run:\n  gh auth login\n\n" +
			"Then try your Arborist command again.")
)

// requireGit verifies git is installed, returning actionable guidance if not.
func requireGit(ctx context.Context, g git.Client) error {
	if err := g.EnsureInstalled(ctx); err != nil {
		if errors.Is(err, git.ErrNotInstalled) {
			return errGitNotInstalled
		}
		return fmt.Errorf("checking git: %w", err)
	}
	return nil
}

// requireGitHubCLI verifies gh is installed and authenticated, returning
// actionable guidance if not.
func requireGitHubCLI(ctx context.Context, h github.Client) error {
	if err := h.EnsureInstalled(ctx); err != nil {
		if errors.Is(err, github.ErrNotInstalled) {
			return errGHNotInstalled
		}
		return fmt.Errorf("checking gh: %w", err)
	}

	if err := h.EnsureAuthenticated(ctx); err != nil {
		switch {
		case errors.Is(err, github.ErrNotAuthenticated):
			return errGHNotAuthenticated
		case errors.Is(err, github.ErrNotInstalled):
			return errGHNotInstalled
		default:
			return fmt.Errorf("checking gh authentication: %w", err)
		}
	}
	return nil
}

// checkPrerequisites verifies every external tool Arborist needs for the full
// worktree workflow: git installed, then gh installed and authenticated. It is
// used by commands (such as "new") that touch both git and GitHub.
func checkPrerequisites(ctx context.Context, g git.Client, h github.Client) error {
	if err := requireGit(ctx, g); err != nil {
		return err
	}
	return requireGitHubCLI(ctx, h)
}
