// Package worktree implements Arborist's higher-level worktree workflows: given
// a branch and a set of repositories, it clones missing repos, refreshes refs,
// picks the right branch source, and creates worktrees safely, collecting a
// per-repository summary.
package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jjacoblee/arborist/internal/git"
	"github.com/jjacoblee/arborist/internal/github"
	"github.com/jjacoblee/arborist/internal/paths"
)

// Git is the subset of git operations the service needs. git.Client satisfies
// it; tests provide a fake.
type Git interface {
	IsRepo(ctx context.Context, path string) bool
	Fetch(ctx context.Context, repoPath string) error
	SetOriginHead(ctx context.Context, repoPath string) error
	DefaultBranch(ctx context.Context, repoPath string) (string, error)
	LocalBranchExists(ctx context.Context, repoPath, branch string) bool
	RemoteBranchExists(ctx context.Context, repoPath, branch string) bool
	AddWorktree(ctx context.Context, repoPath string, opts git.WorktreeAddOptions) error
	ListWorktrees(ctx context.Context, repoPath string) ([]git.Worktree, error)
	CurrentBranch(ctx context.Context, path string) (string, error)
	IsDirty(ctx context.Context, path string) (bool, error)
	MainRepoPath(ctx context.Context, path string) (string, error)
	RemoveWorktree(ctx context.Context, repoPath, worktreePath string, force bool) error
	PruneWorktrees(ctx context.Context, repoPath string) error
}

// Cloner clones a repository by its "owner/name" identity into a destination
// directory. github.Client satisfies it via `gh repo clone`, so cloning relies
// on the GitHub CLI's authentication rather than SSH keys.
type Cloner interface {
	CloneRepo(ctx context.Context, nameWithOwner, dest string) error
}

// Compile-time assertions that the production clients satisfy the interfaces.
var (
	_ Git    = git.Client{}
	_ Cloner = github.Client{}
)

// Service creates worktrees within a single owner workspace. WorkspaceRoot and
// WorktreeRoot must already be expanded (no leading "~"). Base clones live
// directly under WorkspaceRoot as <WorkspaceRoot>/<repo>.
type Service struct {
	Git    Git
	Cloner Cloner
	// Owner is the workspace's GitHub owner, used only for display.
	Owner string
	// WorkspaceRoot is the owner workspace root; it doubles as the repository
	// root, so base clones are <WorkspaceRoot>/<repo>.
	WorkspaceRoot string
	WorktreeRoot  string
	// CopyEnvFiles copies top-level .env/.env.* files from a repo's base clone
	// into each new worktree when true.
	CopyEnvFiles bool
	// CopyFiles lists additional repo-relative files to copy from the base clone
	// into each new worktree (for files .env matching doesn't cover).
	CopyFiles []string
	// Base, when non-empty, is the branch or ref new branches are created from
	// instead of the repository's default branch. It applies only when the
	// target branch does not already exist locally or remotely.
	Base string
}

// Create processes each repository for the given branch and returns a combined
// summary. name, when non-empty, is used as the worktree directory name instead
// of the branch (the branch itself is unchanged); it lets long branch names map
// to short, tidy folders. Repositories are handled sequentially; a failure on
// one does not stop the others (partial success is reported).
func (s Service) Create(ctx context.Context, branch, name string, repos []github.Repository) CreateResult {
	var result CreateResult
	for _, repo := range repos {
		s.createOne(ctx, branch, name, repo, &result)
	}
	return result
}

func (s Service) createOne(ctx context.Context, branch, name string, repo github.Repository, res *CreateResult) {
	// The worktree directory is named after the branch unless a short name is
	// given; the branch checked out inside it is always the full branch.
	dirName := branch
	if name != "" {
		dirName = name
	}
	repoPath := paths.RepoPath(s.WorkspaceRoot, repo.Name)
	worktreePath := paths.WorktreePath(s.WorktreeRoot, repo.Name, dirName)

	fail := func(err error) {
		res.Failed = append(res.Failed, FailedWorktree{Repository: repo, Branch: branch, Err: err})
	}
	skip := func(path, reason string) {
		res.Skipped = append(res.Skipped, SkippedWorktree{Repository: repo, Branch: branch, Path: path, Reason: reason})
	}

	// Safety: never operate outside the configured Arborist directories.
	if !isWithin(s.WorkspaceRoot, repoPath) || !isWithin(s.WorktreeRoot, worktreePath) {
		fail(fmt.Errorf("computed path escapes the configured Arborist directories"))
		return
	}

	// Never overwrite an existing worktree directory.
	if pathExists(worktreePath) {
		skip(worktreePath, "worktree path already exists")
		return
	}

	// Ensure the base repository is available locally.
	if pathExists(repoPath) {
		if !s.Git.IsRepo(ctx, repoPath) {
			fail(fmt.Errorf("%s exists but is not a git repository", repoPath))
			return
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
			fail(fmt.Errorf("create repository directory: %w", err))
			return
		}
		if err := s.Cloner.CloneRepo(ctx, repo.NameWithOwner, repoPath); err != nil {
			fail(err)
			return
		}
	}

	// Refresh remote refs so branch detection is accurate.
	if err := s.Git.Fetch(ctx, repoPath); err != nil {
		fail(err)
		return
	}

	// If this branch is already checked out somewhere, point the user there.
	worktrees, err := s.Git.ListWorktrees(ctx, repoPath)
	if err != nil {
		fail(err)
		return
	}
	for _, wt := range worktrees {
		if wt.Branch == branch {
			skip(wt.Path, "branch already has a worktree")
			return
		}
	}

	// Choose the branch source.
	opts := git.WorktreeAddOptions{Path: worktreePath, Branch: branch}
	source := SourceLocalBranch
	switch {
	case s.Git.LocalBranchExists(ctx, repoPath, branch):
		// Use the existing local branch as-is.
	case s.Git.RemoteBranchExists(ctx, repoPath, branch):
		opts.CreateNew = true
		opts.BaseRef = "origin/" + branch
		source = SourceRemoteTracking
	default:
		opts.CreateNew = true
		if s.Base != "" {
			opts.BaseRef = s.resolveBaseRef(ctx, repoPath, s.Base)
			source = BranchSource("new branch from " + s.Base)
		} else {
			base, err := s.defaultBranch(ctx, repoPath)
			if err != nil {
				fail(fmt.Errorf("detect default branch: %w", err))
				return
			}
			opts.BaseRef = base
			source = SourceNewBranch
		}
	}

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		fail(fmt.Errorf("create worktree directory: %w", err))
		return
	}
	if err := s.Git.AddWorktree(ctx, repoPath, opts); err != nil {
		fail(err)
		return
	}

	created := CreatedWorktree{
		Repository: repo,
		Branch:     branch,
		Path:       worktreePath,
		Source:     source,
	}
	if s.CopyEnvFiles {
		// Best effort: a worktree is still "created" even if env copying fails.
		created.CopiedEnv, _ = copyEnvFiles(repoPath, worktreePath)
	}
	if len(s.CopyFiles) > 0 {
		created.CopiedFiles, _ = copyExtraFiles(repoPath, worktreePath, s.CopyFiles)
	}
	res.Created = append(res.Created, created)
}

// resolveBaseRef turns a user-supplied base branch into a ref git can use:
// a local branch is used directly, a remote-only branch becomes origin/<base>,
// and anything else (a tag or commit SHA) is passed through unchanged.
func (s Service) resolveBaseRef(ctx context.Context, repoPath, base string) string {
	if s.Git.LocalBranchExists(ctx, repoPath, base) {
		return base
	}
	if s.Git.RemoteBranchExists(ctx, repoPath, base) {
		return "origin/" + base
	}
	return base
}

// defaultBranch detects the repository's default branch, attempting to set
// origin/HEAD once if it is not yet recorded.
func (s Service) defaultBranch(ctx context.Context, repoPath string) (string, error) {
	if b, err := s.Git.DefaultBranch(ctx, repoPath); err == nil {
		return b, nil
	}
	if err := s.Git.SetOriginHead(ctx, repoPath); err != nil {
		return "", err
	}
	return s.Git.DefaultBranch(ctx, repoPath)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// isWithin reports whether target is the same as, or nested under, root.
func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
