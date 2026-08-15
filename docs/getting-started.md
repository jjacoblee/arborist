# Getting started with Arborist

This guide takes you from nothing to creating your first worktrees with
`arb new`.

## 1. Prerequisites

You need these tools installed:

| Tool | Check | Where to get it |
| --- | --- | --- |
| Git | `git --version` | https://git-scm.com/downloads |
| GitHub CLI (`gh`) | `gh --version` | https://cli.github.com/ |
| Go 1.23+ | `go version` | https://go.dev/dl/ — only needed to build from source |

Arborist uses the GitHub CLI for all GitHub access and **never stores or logs
GitHub tokens**.

## 2. Authenticate the GitHub CLI

If you haven't already, log in once:

```bash
gh auth login
```

Verify it worked:

```bash
gh auth status
```

Arborist clones repositories with `gh repo clone`, so this is the only
authentication you need — no SSH keys or `known_hosts` setup.

## 3. Install Arborist

The quickest way is the install script — it downloads the right prebuilt binary
for your platform (macOS/Linux, amd64/arm64) from the latest release, verifies
its checksum, and puts `arb` on your `PATH`. No Go toolchain required:

```bash
curl -fsSL https://raw.githubusercontent.com/jjacoblee/arborist/main/install.sh | sh
```

Pin a version with `ARBORIST_VERSION=v0.1.0`, or change the destination with
`ARBORIST_INSTALL_DIR` (default `/usr/local/bin`, falling back to
`~/.local/bin`). You can also download an archive yourself from the
[releases page](https://github.com/jjacoblee/arborist/releases).

Building from source instead (contributors, or Windows) requires Go 1.23+:

```bash
git clone https://github.com/jjacoblee/arborist.git
cd arborist
make install        # builds, then links arb into /usr/local/bin (on your PATH)
```

`make install` uses `sudo` only if the target directory isn't writable. Override
the location with `make install PREFIX=~/.local`, and remove it with
`make uninstall`.

Confirm the install:

```bash
arb --version
```

## 4. Create a workspace

Arborist is workspace-rooted, like Git: each GitHub owner gets its own folder
with a hidden `.arborist.json` config. Make a folder for the owner you want to
work with and initialize it:

```bash
mkdir -p ~/work/acme && cd ~/work/acme
arb init --owner acme
```

This writes `.arborist.json` at the workspace root (it won't overwrite an
existing one without `--force`):

```
Created Arborist workspace at ~/work/acme

  config:        ~/work/acme/.arborist.json
  owner:         acme
  worktreeRoot:  ~/work/acme/worktrees
  copyEnvFiles:  false
```

- **owner** (required, set by `--owner`) — the GitHub user or organization this
  workspace discovers repositories from.
- **worktreeRoot** — where worktrees are created, as
  `<worktreeRoot>/<repo>/<sanitized-branch>`. Defaults to `<workspace>/worktrees`
  (a sibling of your clones); override with `--worktree-root`.
- **copyEnvFiles** — set to `true` to copy top-level `.env` / `.env.*` files
  from each repo's base clone into new worktrees. Off by default.

Every other command finds this workspace by walking up from your current
directory, so just `cd` into the folder (or any subdirectory) to use it. To work
in a different owner, make another folder and `arb init --owner <other>` there.

Because the config is hidden, manage it with `arb config` rather than editing by
hand:

```bash
arb config                       # show the resolved configuration
arb config set copyEnvFiles true # change a value
```

See [config.md](config.md) for the full reference.

## 5. Create your first worktrees

```bash
arb new feature/my-change
```

Arborist will:

1. Validate the branch name.
2. Check that `git` and `gh` are ready.
3. Discover your repositories and open an interactive picker:

   ```
   Select repositories for branch: feature/my-change
   Space to select · Enter to confirm · Esc to cancel

   > [ ] acme/web-app (private)
     [ ] acme/api
     [x] acme/admin
   ```

   Type to filter, **Space** to toggle a repo, **Enter** to confirm, **Esc** (or
   Ctrl+C) to cancel without making any changes.

4. For each selected repository, clone it if it isn't already local, fetch the
   latest refs, and create the worktree — reusing an existing local branch,
   tracking a remote branch, or creating a new branch from `origin/<default>`
   (the fetched default branch, not a stale local copy).

5. Print a summary:

   ```
   Created worktrees (1)

     acme/admin
       Branch: feature/my-change
       Path:   ~/work/acme/worktrees/admin/feature-my-change
       Source: new branch from default branch
   ```

Your new worktree is a normal working directory — `cd` into the path shown and
start working.

Long branch names make for long folder names. To keep the folder short, pass
`--name`; the branch is still created in full:

```bash
arb new full-feature-branch-name --name short-name
# folder: <worktreeRoot>/<repo>/short-name   branch: full-feature-branch-name
```

By default a new branch is created from the repo's default branch on origin
(for example `origin/main`), not from a local copy that may be stale. To branch
off something else — say another feature branch — pass `--base`:

```bash
arb new follow-up --base feature-x
```

`--base` accepts a local branch, a remote branch (resolved to `origin/<name>`),
or a tag/commit. It applies only when the branch is newly created — if the
branch already exists locally or on the remote, Arborist uses it as-is.

### Name the repositories and skip the picker

`arb new` opens the picker by default. When you already know the set — or you
run the command from a script — pass `--repo` and Arborist goes straight to
clone-if-missing plus worktree creation:

```bash
arb new feature/my-change --repo api,web
arb new feature/my-change --repo api --repo web
arb new feature/my-change --repo "api web"
```

Names may be bare (`api`) or owner-qualified (`acme/api`). They match
case-insensitively. If any name matches no repository, the command fails before
it creates a worktree, and it reports every unknown name at once. `--repo`
always skips the picker, at a terminal and in a script.

### Group worktrees for cleanup by intent

By default every worktree lands in one flat tree
(`<worktreeRoot>/<repo>/<branch>`). Pass `--group` to nest related work one
level deeper, so a whole category is one folder to inspect or delete:

```bash
arb new pr/1234 --group review --repo admin
# folder: <worktreeRoot>/review/admin/pr-1234
```

Worktrees created without a group keep their original path, so nothing moves
when you start using groups. The first use of an unfamiliar group name is
confirmed (Arborist lists the groups that already exist) so a typo does not
silently create a near-duplicate folder. Name the group in the `groups` config
field, or pass `--yes`, to skip that question.

Set a default group if most new worktrees should land in one folder:

```bash
arb config set defaultGroup review
```

Pass `--group ""` to place a single worktree outside it.

`arb list` grows a GROUP column once anything is grouped. Both `arb list` and
`arb remove` take `--group <name>` to narrow to one (`--group ""` selects the
ungrouped worktrees). See [config.md](config.md#groups) for the full reference.

## Other commands available today

```bash
arb list               # managed worktrees, each with a short id (paths relative; --full for absolute)
arb list --group review
arb open a3f9 --cursor # open a worktree in your editor (by id or branch)
arb remove             # pick worktrees to remove (searchable multi-select)
arb remove a3f9        # remove one worktree by its short id (from `arb list`)
arb remove feature/my-change   # or remove every worktree on a branch
arb prune              # clean up stale worktree references and leftover branches
arb repo list          # list the workspace owner's repositories
arb config             # view/edit the workspace configuration
arb --help             # full command list
```

### Opening a worktree

`arb open <id-or-branch>` launches a worktree in your editor. Pick the editor
with `--cursor`, `--code`, or `--editor <command>`, or set a default once:

```bash
arb config set editor cursor   # then a bare `arb open <id>` uses it
arb open a3f9                  # opens that worktree in Cursor
```

Without a flag or config, `arb open` falls back to your `$EDITOR`.

To jump into a worktree in your shell, use `--print` (a program can't change
your shell's directory, so add a tiny helper to your `~/.zshrc`):

```bash
acd() { cd "$(arb open "$1" --print)"; }
# then:  acd a3f9
```

### Getting a worktree ready to run (setup commands)

A fresh worktree is a clean checkout, so it usually needs setup before you can
run it — installing dependencies, syncing tools, and so on. Configure those
commands per repository under `setup` in your workspace config, and Arborist
runs them in each new worktree right after `arb new`:

```json
{
  "owner": "acme",
  "setup": {
    "admin": ["pnpm install", "uv sync", "pnpm run init:ruff"],
    "*": ["pnpm install"]
  }
}
```

The `*` entry applies to any repo without an exact match. `setup` (like
`copyFiles`) is a structured field, so edit it in the file itself — open it
with `$EDITOR "$(arb config path)"`. Skip the commands for one run with
`arb new <branch> --no-setup`, or run them on demand against an existing
worktree:

```bash
arb setup a3f9   # re-run setup for that worktree (by id or branch)
```

Setup commands run through a shell in the worktree directory. Because they come
only from your own trust-checked config (never from a cloned repository),
Arborist runs them without prompting — see [SECURITY.md](../SECURITY.md).

Worktrees also often need local, gitignored files. Set `copyEnvFiles: true` to
copy top-level `.env` / `.env.*` files from the base clone into each new
worktree, and list anything else under `copyFiles` (e.g. `["secrets.env"]`) —
useful for files the `.env` match doesn't cover.

`arb list` shows a short, stable **id** for each worktree:

```
ID    REPOSITORY  BRANCH                STATUS  PATH
7351  acme/admin  feature/my-change     clean   admin/feature-my-change
```

Use that id with `arb remove <id>` to remove exactly that worktree, or a branch
name to remove every worktree on it. Either way `arb remove` shows the exact
paths and asks before deleting, and never removes a worktree with uncommitted
changes unless you pass `--force`.

### Remove worktrees in bulk

`arb remove` with no argument opens a searchable multi-select picker of the
worktrees that are safe to remove, so cleanup does not require one
`arb remove <branch>` at a time:

```bash
arb remove                 # pick from the clean worktrees
arb remove --group review  # only that group
arb remove --force         # include dirty and unpushed worktrees
```

By default the picker lists only clean worktrees, so nothing risky is
selectable. `--force` lists every worktree instead, and marks each with
`[dirty]` (uncommitted changes or untracked files) or `[unpushed]` (commits
that exist on no origin ref). A selection is not an agreement to delete: the
chosen worktrees are listed with full paths and confirmed first.

When a removal takes a branch's last worktree, Arborist offers to delete the
local branch too. Pass `--delete-branch` to opt in without the prompt. A
`--yes` run without that flag never deletes a branch nobody asked for.

`arb prune` does the same cleanup for branches that are already orphaned: it
lists every local branch left with no worktree and offers to delete them
(`--delete-branches` to skip the prompt, `--yes` to report only).

Deletion uses `git branch -d`, so a branch holding commits git cannot see
elsewhere is reported as skipped rather than deleted. `--force` maps to `-D`
and deletes it deliberately. A repository's default branch is never a
candidate.

When a removal empties the folders above a worktree, Arborist removes those
empty parents up to — never including — the worktree root. An emptied group
folder does not stay behind.

## Troubleshooting

Arborist aims to tell you exactly what to do. Common messages:

| Message | Fix |
| --- | --- |
| `git is required but was not found` | Install Git and retry. |
| `the GitHub CLI (gh) is required but was not found` | Install `gh` from https://cli.github.com/. |
| `the GitHub CLI (gh) is not authenticated` | Run `gh auth login`. |
| `not inside an Arborist workspace` | `cd` into a workspace folder, or run `arb init --owner <owner>` to create one. |
| `worktree path already exists` / `branch already has a worktree` | Not an error — Arborist safely skipped it and shows the existing path. |
| `unknown repository` (from `--repo`) | Check the name against `arb repo list`. Names may be bare (`api`) or owner-qualified (`acme/api`). |
| `git status` says the branch is behind `main` (or `master`) | Worktrees created before the upstream fix track the default branch. Run `git branch --unset-upstream` inside that worktree. New worktrees do not set that upstream. |

## Safety notes

- Arborist runs `git`/`gh` with explicit argument arrays — never a shell string
  built from your input.
- Creating worktrees never uses Git's `--force`, so Arborist won't overwrite an
  existing worktree or reuse a branch already checked out elsewhere. Removal
  only touches a dirty worktree if you explicitly pass `--force`. A branch
  holding unmerged commits is never deleted without `--force`, and a
  repository's default branch is never offered for deletion.
- All file changes stay inside the workspace and its worktree root.
- Arborist never runs anything from a repository on its own — it doesn't look
  inside a clone for a script to run or auto-install dependencies. It runs only
  the `setup` commands from your own trust-checked config. Those commands are
  yours to choose, though: ones like `pnpm install` install dependencies and can
  trigger a repo's own `postinstall`/lifecycle scripts, so configure setup only
  for repositories you trust — just as when you run `pnpm install` by hand.

## What's next

The full command set (`init`, `new`, `open`, `setup`, `list`, `remove`,
`prune`, `repo list`, `config`) is implemented, including `--repo`, worktree
groups, bulk removal, and leftover-branch cleanup. Releases ship as prebuilt
binaries. Upcoming work: enabling the Homebrew tap, integration tests against
real temporary git repositories, and richer examples.
