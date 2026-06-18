# Changelog

All notable changes to Arborist are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-11

First public release. Arborist is a guided CLI for managing Git worktrees across
multiple repositories, built around per-owner workspaces.

### Added

- **Owner workspaces.** `arb init --owner <owner>` creates a workspace by writing
  a hidden `.arborist.json` at the workspace root. Every other command finds the
  workspace by walking up from the current directory, like Git finds `.git`, and
  errors with guidance when run outside one.
- **`arb new <branch>`** — the flagship workflow: prerequisite checks (git / `gh`
  install + auth), GitHub repo discovery, a searchable multi-select picker,
  clone-if-missing, fetch, default-branch detection, and safe branch-source
  selection (existing local branch, remote-tracking branch, or a new branch from
  the default branch), with a created/skipped/failed summary. `--name` sets a
  short worktree folder name while keeping the full branch; `--base` creates the
  new branch from a chosen branch, tag, or commit instead of the default branch.
- **`arb list`** — managed worktrees with a short, stable **id** (derived from the
  worktree path, shown at the shortest unambiguous length) and paths relative to
  the worktree root; `--full` shows absolute paths.
- **`arb open <id-or-branch>`** — open a worktree in your editor (`--cursor`,
  `--code`, `--editor <cmd>`, the `editor` config value, or `$EDITOR`), or print
  its path with `--print` (handy for a `cd` shell helper).
- **`arb setup <id-or-branch>`** and per-repo `setup` config — shell commands
  (e.g. `pnpm install`, `uv sync`) run in each new worktree, automatically after
  `arb new` (`--no-setup` to skip) or on demand. Honored only from your own
  trust-checked config, never from a repository.
- **`arb remove <id-or-branch>`** — remove a single worktree by id, or every
  worktree on a branch, with confirmation. Never removes a worktree with
  uncommitted changes unless `--force` is given.
- **`arb prune`** — clear stale worktree references.
- **`arb repo list`** — list the workspace owner's repositories via `gh`.
- **`arb config`** — `list` / `get` / `set` / `path` for the workspace config
  (`owner`, `worktreeRoot`, `copyEnvFiles`, `editor`).
- **File seeding** — `copyEnvFiles` copies top-level `.env` / `.env.*` files into
  each new worktree; `copyFiles` copies additional listed repo-relative files
  (e.g. `secrets.env`) that the `.env` match misses. Copies are private (`0600`).
- **Worktree layout** — base clones live at `<workspace>/<repo>`; worktrees at
  `<worktreeRoot>/<repo>/<sanitized-branch>` (worktree root defaults to a sibling
  `worktrees/` folder).
- **Distribution** — a GoReleaser pipeline and `release` GitHub Actions workflow
  that publish cross-compiled macOS/Linux (amd64 + arm64) binaries and checksums
  on tagged releases, a `curl | sh` install script, and a ready-to-enable
  Homebrew tap. No Go toolchain required for end users.

### Security

- All Git and GitHub CLI calls use `os/exec` with argument arrays (never a shell
  string built from user input); branch names are validated and sanitized before
  use in paths; destructive actions confirm and stay inside configured
  directories. Authentication is delegated entirely to the GitHub CLI — no tokens
  are stored or logged.
- A discovered `.arborist.json` is loaded only if it is a regular file owned by
  the current user and not writable by group or others (a "dubious ownership"
  check), since the config's `editor` value is run by `arb open`. See
  SECURITY.md for the trust model.
- The install script verifies the downloaded archive's checksum against its
  exact filename in `checksums.txt`.

[Unreleased]: https://github.com/jjacoblee/arborist/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jjacoblee/arborist/releases/tag/v0.1.0
