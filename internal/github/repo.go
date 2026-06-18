package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jjacoblee/arborist/internal/exec"
)

// DefaultRepoLimit is the default maximum number of repositories ListRepos
// requests from the GitHub CLI.
const DefaultRepoLimit = 200

// repoJSONFields are the gh fields Arborist requests. Kept as a single constant
// so the production code and tests agree on the exact command. Arborist clones
// through `gh repo clone`, so it needs no clone URLs — only identity and
// visibility.
const repoJSONFields = "name,nameWithOwner,isPrivate"

// Repository is a GitHub repository in the shape Arborist needs.
type Repository struct {
	Name          string // repository name, e.g. "web-app"
	NameWithOwner string // "owner/name", e.g. "acme/web-app"
	Owner         string // "acme"
	IsPrivate     bool
}

// ghRepo mirrors the JSON object emitted by `gh repo list --json ...`.
type ghRepo struct {
	Name          string `json:"name"`
	NameWithOwner string `json:"nameWithOwner"`
	IsPrivate     bool   `json:"isPrivate"`
}

// ListRepos returns repositories via the GitHub CLI.
//
// When owner is empty, it lists the authenticated user's repositories
// (`gh repo list`); when owner is set to a user or organization login, it lists
// that account's repositories (`gh repo list <owner>`).
//
// A limit <= 0 falls back to DefaultRepoLimit. The caller is responsible for
// ensuring gh is installed and authenticated first (see the CLI preflight); if
// gh is missing, ListRepos returns ErrNotInstalled.
func (c Client) ListRepos(ctx context.Context, owner string, limit int) ([]Repository, error) {
	if limit <= 0 {
		limit = DefaultRepoLimit
	}

	args := []string{"repo", "list"}
	if owner != "" {
		args = append(args, owner) // positional owner, before flags
	}
	args = append(args, "--limit", strconv.Itoa(limit), "--json", repoJSONFields)

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrNotInstalled
		}
		return nil, fmt.Errorf("list GitHub repositories: %w", err)
	}

	return parseRepos(out)
}

// CloneRepo clones nameWithOwner ("owner/repo") into dest using `gh repo clone`.
//
// Cloning through gh means Arborist relies entirely on the user's GitHub CLI
// authentication — no SSH keys or known_hosts setup are required. gh uses the
// protocol configured by `gh config get git_protocol` (HTTPS by default). The
// destination's parent directories must already exist; if gh is missing,
// CloneRepo returns ErrNotInstalled.
func (c Client) CloneRepo(ctx context.Context, nameWithOwner, dest string) error {
	if _, err := c.runner.Run(ctx, "gh", "repo", "clone", nameWithOwner, dest); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrNotInstalled
		}
		return fmt.Errorf("clone %s into %s with gh: %w", nameWithOwner, dest, err)
	}
	return nil
}

// parseRepos decodes the JSON array produced by `gh repo list --json ...` into
// Repository values, validating the fields Arborist relies on.
func parseRepos(data []byte) ([]Repository, error) {
	var raw []ghRepo
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse gh repo list output: %w", err)
	}

	repos := make([]Repository, 0, len(raw))
	for i, r := range raw {
		owner, name, ok := strings.Cut(r.NameWithOwner, "/")
		if !ok || owner == "" || name == "" {
			return nil, fmt.Errorf("repository %d: nameWithOwner %q is not in owner/name form", i, r.NameWithOwner)
		}
		// Prefer the explicit name field; fall back to the parsed segment.
		if r.Name != "" {
			name = r.Name
		}

		repos = append(repos, Repository{
			Name:          name,
			NameWithOwner: r.NameWithOwner,
			Owner:         owner,
			IsPrivate:     r.IsPrivate,
		})
	}
	return repos, nil
}
