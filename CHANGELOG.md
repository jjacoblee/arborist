# Changelog

All notable changes to Arborist are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Worktree groups.** `arb new <branch> --group review` nests worktrees under a
  named folder inside the worktree root, laid out as
  `<worktreeRoot>/<group>/<repo>/<branch>`. The group comes first so everything
  in it is a single directory to inspect or delete — the point being bulk
  cleanup by intent: throw away everything in `review` without looking at any of
  it. Worktrees created without a group keep their existing path, so nothing
  moves.

  Groups are free-form and created on demand, so the first use of an unfamiliar
  name is confirmed (listing the groups that already exist) rather than silently
  creating a folder for a typo. Declaring a name in the new `groups` config
  field, or passing the new `arb new --yes`, skips that question. The new
  `defaultGroup` config field sets the group used when `--group` is absent;
  `--group ""` places a worktree outside it.

  `arb list` grows a GROUP column once anything is grouped — and only then, so a
  workspace that doesn't use groups sees the table it always saw. Both
  `arb list` and `arb remove` take `--group <name>` to narrow to one group, with
  `--group ""` selecting the ungrouped worktrees.

### Fixed

- **New branches no longer adopt the default branch as their upstream.** Basing
  a new branch on `origin/<default>` (see the stale-default fix below) meant it
  started at a *remote-tracking* ref, and git's `branch.autoSetupMerge` default
  records an upstream whenever a branch starts there. Every new branch therefore
  came out tracking `origin/main`: `git push` targeted main, and `git status`
  reported the work as behind it. `arb new` now states the intent — it tracks
  the base ref only when that ref is the branch's own counterpart on origin, and
  passes `--no-track` otherwise, so the result no longer depends on each user's
  `branch.autoSetupMerge` setting. A branch that isn't on origin yet is left
  with no upstream, which the first `git push -u` sets correctly.

  Worktrees created before this fix keep the wrong upstream. Run
  `git branch --unset-upstream` inside each one to clear it.

- **Removing a worktree now cleans up the folders it emptied.** git removes the
  checkout but leaves the `<repo>` (and, for a grouped worktree, `<group>`)
  directories above it, so a fully cleaned-out group still looked like it held
  something. Emptied parents are now removed up to — never including — the
  worktree root, and a directory still holding work is never touched.

- **`arb new` no longer branches off a stale default branch.** It fetched before
  choosing a branch source, but then created new branches from the *local*
  default branch — and `git fetch` updates `origin/main`, never local `main`. A
  workspace whose base clone hadn't been pulled by hand therefore started every
  new branch from whatever tip it was last left at. New branches now come from
  `origin/<default>`, falling back to the local branch for a repository whose
  default isn't on origin yet. `--base <ref>` is unchanged: a ref you name
  explicitly still resolves to your local copy when you have one.

### Added

- **`arb new --repo`.** Name the repositories up front and skip the picker
  entirely, so `arb new` works from a script and repeat workflows stop
  re-selecting the same set: `arb new my-branch --repo api,web`. The flag is
  repeatable and also accepts space-separated names, so `--repo api --repo web`,
  `--repo api,web`, and `--repo "api web"` are equivalent. Names may be bare
  (`api`) or owner-qualified (`acme/api`), and match case-insensitively. A name
  that matches no repository fails the command before anything is created,
  reporting every unknown name at once.

- **Interactive bulk removal.** `arb remove` with no argument now opens a
  searchable multi-select picker of the worktrees that are safe to remove, so
  cleaning up a backlog no longer means one `arb remove <branch>` at a time.
  `arb remove <id-or-branch>` is unchanged.

  By default the picker lists only clean worktrees, so nothing risky is even
  selectable. `--force` lists every worktree instead, marking each with
  `[dirty]` (uncommitted changes or untracked files) and `[unpushed]` (commits
  that exist on no origin ref). The selection is then summarized with full paths
  and confirmed before anything is deleted, and any branch the removal orphans
  goes through the same deletion offer as above.

- **Local branch cleanup.** Removing a worktree used to leave its branch ref
  behind, so re-adding the worktree checked out the same stale branch again.
  `arb remove` now notices when a removal takes a branch's last worktree and
  offers to delete the local branch too; `--delete-branch` opts in without the
  prompt. `arb prune` likewise lists every local branch left with no worktree and
  offers to delete them (`--delete-branches` to skip the prompt, `--yes` to
  report only).

  Deletion uses `git branch -d`, so a branch holding commits git can't see
  anywhere else is reported as skipped rather than deleted — `--force` (`-D`)
  deletes it deliberately. A repository's default branch is never a candidate,
  even when nothing has it checked out.

## [0.1.0] - 2026-07-07

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
- **Step-line progress output** — long-running commands narrate completed
  actions as permanent lines (`✓ cloned acme/api`, `• skipped …`, `✗ …`,
  `▸ setup api: pnpm install ✓`) with a spinner naming the action in flight.
  Drawn only on an interactive terminal; piped output stays clean.
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
