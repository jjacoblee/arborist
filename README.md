<p align="center">
  <img src="docs/brand/assets/logo-primary.png" alt="Arborist — Git worktree management" width="300">
</p>

<p align="center"><strong>Guided Git worktree management across multiple repositories.</strong></p>

Arborist turns the multi-step ritual of cloning repos and hand-typing
`git worktree add` into a single guided command:

```bash
arb new feature/my-change
```

It helps you pick the repositories you want, clones any that are missing, and
creates predictable, isolated worktrees for the branch — so you can work on the
same branch across several repos without clobbering your main checkouts.

> **Project status: pre-1.0.** All commands documented below are implemented
> and tested. Behavior is stable, but flags and output may still change before
> 1.0. See the [Roadmap](#roadmap) for what's next.

## Why Arborist

Git worktrees are great for working on multiple branches at once without
stashing or re-cloning, but the ergonomics are rough: you have to remember long
commands, manage repo paths by hand, and repeat the dance for every repository
in a multi-repo project. Arborist is a small, safe, single-binary tool that
makes the common worktree workflow a guided experience.

## Prerequisites

- [Git](https://git-scm.com/)
- The [GitHub CLI](https://cli.github.com/) (`gh`), authenticated:

  ```bash
  gh auth login
  ```

Arborist uses your existing `gh` authentication for GitHub access. It never asks
for, stores, or logs GitHub tokens.

## Installation

Arborist ships as a single prebuilt binary — **no Go toolchain required**.

### From source (contributors)

Building from source requires Go 1.23+:

```bash
git clone https://github.com/jjacoblee/arborist.git
cd arborist
make build      # compiles to ./bin/arb
make install    # builds, then links arb into /usr/local/bin (on your PATH)
```

`make install` puts `arb` on your `PATH` directly (no Go-bin setup needed); it
uses `sudo` only if the target directory isn't writable. Override the location
with `make install PREFIX=~/.local`, and remove it with `make uninstall`. Run
`make help` to see all developer targets.

## Quick start

```bash
gh auth login                          # one-time GitHub CLI authentication

mkdir -p ~/work/acme && cd ~/work/acme # a folder for one GitHub owner
arb init --owner acme                  # set up the workspace (.arborist.json)

arb new feature/example-change         # pick repos and create worktrees
arb list                               # see your worktrees
arb remove feature/example-change
```

New here? The **[Getting Started guide](docs/getting-started.md)** walks through
prerequisites, installation, and your first `arb new` step by step.

## Commands

| Command | Description |
| --- | --- |
| `arb init --owner <owner>` | Set up an owner workspace in the current directory (writes `.arborist.json`). |
| `arb new <branch-name>` | The flagship workflow: pick repositories, clone any that are missing, and create worktrees for the branch, then run your configured setup commands (`--no-setup` to skip). `--name <short>` gives the worktree folder a short name; `--base <ref>` branches off a chosen ref instead of the default branch. |
| `arb list` | List managed worktrees, each with a short **id**; paths are shown relative to the worktree root (use `--full` for absolute). |
| `arb open <id-or-branch>` | Open a worktree in your editor (`--cursor`, `--code`, `--editor <cmd>`, or your configured default), or print its path with `--print`. |
| `arb setup <id-or-branch>` | Run this workspace's configured setup commands in a worktree (e.g. `pnpm install`, `uv sync`). Runs automatically after `arb new`. |
| `arb remove <id-or-branch>` | Safely remove a single worktree by its short id, or every worktree on a branch (with confirmation; `--yes` to skip it, `--force` for dirty worktrees). Alias: `arb rm`. |
| `arb prune` | Clean up stale worktree references. |
| `arb repo list` | List the workspace owner's GitHub repositories (via `gh`). |
| `arb config` | View and edit the workspace configuration (`list`/`get`/`set`/`path`). |

Every command except `arb init` runs inside a workspace; they locate it by
walking up from the current directory.

`arb worktree add/list/remove` will exist later as explicit aliases, but
`arb new` is the primary, documented workflow.

## Configuration

Arborist is workspace-rooted, like Git. Each GitHub owner (user or organization)
gets its own folder anywhere on disk, with a hidden `.arborist.json` config at
its root. Commands find it by walking up from the current directory, so you work
in an owner by `cd`-ing into its folder. Run outside any workspace and Arborist
tells you to `cd` in or run `arb init`.

Create a workspace with `arb init --owner <github-owner>` inside the folder you
want to use (it won't overwrite an existing config unless you pass `--force`):

```json
{
  "owner": "acme",
  "copyEnvFiles": false
}
```

- **owner** (required) — the GitHub user or organization this workspace
  discovers repositories from.
- **worktreeRoot** (optional) — where worktrees live. Defaults to a `worktrees/`
  folder inside the workspace (a sibling of the cloned repos). A relative value
  is resolved against the workspace root; a leading `~` is expanded to your home
  directory.
- **copyEnvFiles** — when `true`, copies top-level `.env` / `.env.*` files from
  a repo's base clone into each new worktree. Off by default (these files
  usually hold secrets).
- **copyFiles** (optional) — additional repo-relative files to copy from the base
  clone into each new worktree, for files `copyEnvFiles` doesn't match (e.g.
  `["secrets.env"]`). Paths can't escape the repo; copies are written `0600`.
- **editor** (optional) — the command `arb open` uses by default, e.g. `cursor`
  or `code --wait`. Falls back to `$EDITOR` when unset.
- **setup** (optional) — per-repo shell commands run in each new worktree, e.g.
  `{"setup": {"admin": ["pnpm install"], "*": ["pnpm install"]}}`. The key `*`
  applies to any repo without an exact entry. These run through a shell, so they
  are honored only from this trust-checked config — never from a repository.

Because the config is hidden, edit it with `arb config` rather than by hand:

```bash
arb config                       # print the resolved configuration
arb config get worktreeRoot      # read one value
arb config set copyEnvFiles true # change a value
arb config path                  # print the config file location
```

`arb config get`/`set` covers `owner`, `worktreeRoot`, `copyEnvFiles`, and
`editor`. The structured fields (`copyFiles`, `setup`) are edited in the file
itself — open it with `$EDITOR "$(arb config path)"` and keep its permissions
at `0600` (see [Trust](docs/config.md#trust)).

### Layout

A workspace is laid out as:

```
~/work/acme/                 # workspace root (= repo root), holds .arborist.json
  admin/                     # base clone   <workspace>/<repo>
  api/
  worktrees/                 # worktree root (default)
    admin/
      feature-x/             # a worktree   <worktreeRoot>/<repo>/<branch>
```

To work across several owners, create one workspace folder per owner and `cd`
between them. See [docs/config.md](docs/config.md) for full details.

## Safety model

Arborist runs Git and `gh` and touches the filesystem, so it is built to be
conservative by default:

- Commands are executed with argument arrays (`os/exec`), never by building
  shell strings from user input.
- Branch names are validated and sanitized before being used as path segments.
- Destructive actions require explicit confirmation and print the exact paths
  that will be removed.
- A dirty worktree is never removed silently.
- `--force` is only used when you explicitly pass a force flag.
- Filesystem changes stay inside the configured Arborist directories.
- GitHub tokens are never stored or logged; authentication is delegated to the
  GitHub CLI. Repositories are cloned with `gh repo clone`, so cloning uses your
  `gh` authentication — no SSH keys or `known_hosts` setup required.
- Cloned repositories are never run automatically: Arborist executes only the
  `setup` commands from your own trust-checked config, never code it finds in a
  repo. Those commands are yours to choose, though — ones like `pnpm install`
  install dependencies and can run a repo's lifecycle scripts, so configure
  setup only for repositories you trust.


## Roadmap

Implemented today:

- Git-like **owner workspaces** (`arb init --owner`): a hidden `.arborist.json`
  per owner, discovered by walking up from the current directory.
- `arb new <branch>`: the flagship workflow — searchable multi-select repo
  picker, clone-if-missing, fetch, default-branch detection, safe branch-source
  selection, and a created/skipped/failed summary. `--name` for short folders.
- `arb list`: managed worktrees with a short, stable **id** and relative paths
  (`--full` for absolute).
- `arb open <id-or-branch>`: open a worktree in your editor (`--cursor`,
  `--code`, `--editor`, or a configured default) or print its path (`--print`).
- `arb remove <id-or-branch>`: remove one worktree by id or all on a branch,
  with confirmation; never deletes a dirty worktree without `--force`.
- `arb prune`, `arb repo list`, and `arb config` (`get`/`set`/`path`).
- Actionable prerequisite checks (git / `gh` install + auth), all behind a
  mockable command runner with table-driven tests.
- Release packaging: a GoReleaser pipeline publishing no-Go binaries, checksums,
  an install script, and a ready-to-enable Homebrew tap.

Next up: enabling the Homebrew tap on a tagged release, more integration tests,
and richer examples.

## Current limitations

- Pre-1.0: the commands are stable in behavior, but flags and output may still
  change before 1.0.
- Prebuilt binaries cover macOS and Linux (amd64 + arm64); Windows users build
  from source.
- GitHub is the only supported provider, via the GitHub CLI.
- No telemetry or analytics — Arborist runs entirely locally except for the Git
  and GitHub CLI operations you trigger.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues: see
[SECURITY.md](SECURITY.md). By participating you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
