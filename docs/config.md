# Arborist configuration

Arborist is **workspace-rooted**, like Git. Each GitHub owner (user or
organization) gets its own folder on disk, with a hidden `.arborist.json` config
at its root. This page documents how workspaces are discovered, what each field
means, and how to manage the config.

## Workspaces and discovery

A workspace is just a folder containing a `.arborist.json` file. That folder is
also the repository root: base clones live directly under it, and worktrees go in
a sibling `worktrees/` folder by default.

Every command except `arb init` finds the workspace by walking **up** from the
current directory until it sees a `.arborist.json`, exactly like Git locates
`.git`. So you operate on an owner by `cd`-ing into its folder (or any
subdirectory of it). Run a command outside any workspace and Arborist stops with
an actionable message:

```text
not inside an Arborist workspace.

cd into an owner workspace folder, or create one here with:
  arb init --owner <github-owner>
```

Use `--dir <path>` on any command to start the search from a different directory
instead of the current one.

To work across several owners, create one workspace folder per owner — anywhere
you like — and `cd` between them. There is no global, machine-wide config.

## Creating a workspace

Run this inside the folder you want to use as the owner's workspace:

```bash
mkdir -p ~/work/acme && cd ~/work/acme
arb init --owner acme
```

`--owner` is required. If a workspace config already exists, `init` makes no
changes and tells you so; pass `--force` to overwrite it. By default the worktree
root is `<workspace>/worktrees`; override it with `--worktree-root`.

## Fields

```json
{
  "owner": "acme",
  "copyEnvFiles": false
}
```

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `owner` | string | — (required) | The GitHub user or organization this workspace discovers repositories from. |
| `worktreeRoot` | string | `<workspace>/worktrees` | Where worktrees live, laid out as `<worktreeRoot>/<repo>/<sanitized-branch-name>`. Optional. |
| `copyEnvFiles` | bool | `false` | When `true`, copy top-level `.env` / `.env.*` files from a repo's base clone into each new worktree. |
| `copyFiles` | array | `[]` | Extra repo-relative files to copy from the base clone into each new worktree, for files `copyEnvFiles` misses (e.g. `["secrets.env"]`). Confined to the repo; copies are `0600`. |
| `editor` | string | `$EDITOR` | Command `arb open` uses by default, e.g. `cursor` or `code --wait`. Optional; falls back to the `$EDITOR` environment variable. |
| `groups` | array | `[]` | Group names this workspace expects, e.g. `["review", "spike"]`. Advisory only: groups are created on demand, and listing one here just tells Arborist the name is deliberate so `arb new --group` doesn't query it. |
| `defaultGroup` | string | — | Group `arb new` uses when `--group` is not passed. Optional; when unset, new worktrees keep the ungrouped layout. Pass `--group ""` to place one outside the default. |
| `setup` | object | `{}` | Per-repo shell commands run in each new worktree (e.g. `{"admin": ["pnpm install"], "*": ["uv sync"]}`). Key `*` is the fallback. Run via a shell from this trusted config only. |

The repository root is **not** stored — it is implicitly the directory that
contains the config. Arborist clones with `gh repo clone`, so there is no
clone-protocol setting; cloning uses your GitHub CLI authentication (no SSH keys
required).

### Managing the config with `arb config`

Because `.arborist.json` is hidden, prefer the `arb config` command over editing
it by hand:

```bash
arb config                       # print the resolved configuration
arb config get worktreeRoot      # read one value (owner, worktreeRoot, copyEnvFiles, editor, defaultGroup)
arb config set copyEnvFiles true # change a value (re-validated before saving)
arb config path                  # print the config file location
```

`get` and `set` cover `owner`, `worktreeRoot`, `copyEnvFiles`, `editor`, and
`defaultGroup`. Edit `copyFiles`, `setup`, and `groups` in the file itself —
open it with `$EDITOR "$(arb config path)"` and keep its permissions at `0600`
(see [Trust](#trust)). An empty `set defaultGroup` clears the default.

### The worktree root

When `worktreeRoot` is unset, worktrees go in `<workspace>/worktrees`. You can
set it to:

- a **relative** path, resolved against the workspace root
  (`arb config set worktreeRoot trees` → `<workspace>/trees`);
- an **absolute** path (`/srv/worktrees`);
- a path beginning with `~`, expanded to your home directory at use time.

### Copying .env files into worktrees

A fresh worktree is a clean checkout, so gitignored files like `.env` aren't
present. With `copyEnvFiles: true`, Arborist copies the **top-level** `.env` and
`.env.*` files from the repository's base clone (`<workspace>/<repo>`) into each
new worktree it creates.

It is deliberately narrow and safe: only the repo root is scanned (no recursion),
only regular files are copied (symlinks are skipped), and copies are written with
private (`0600`) permissions. It's off by default because these files commonly
contain secrets. The created-worktree summary lists what was copied.

### Example layout

In the `~/work/acme` workspace, cloning `acme/admin` and creating a worktree
for `feature/my-change` produces:

```text
~/work/acme/.arborist.json
~/work/acme/admin
~/work/acme/worktrees/admin/feature-my-change
~/work/acme/worktrees/review/admin/pr-1234
```

### Groups

A group is a named folder inside the worktree root that separates work by
intent — long-lived feature work in one place, disposable review checkouts in
another — so a whole category can be cleaned up without inspecting any of it.
Pass `--group` to `arb new`, and the worktree nests one level deeper:

```text
~/work/acme/worktrees/admin/feature-my-change      # ungrouped
~/work/acme/worktrees/review/admin/pr-1234         # --group review
```

The group comes first so everything in it is a single directory to inspect or
delete. Worktrees created without a group keep their original path, so nothing
moves when you start using groups.

Groups are created on demand, so nothing stops a typo becoming a near-duplicate
folder. The first use of an unfamiliar name is therefore confirmed, listing the
groups that already exist; naming it in `groups` (or passing `--yes`) skips that
question. Group names go through the same sanitization as branch names, so
`--group "code review"` becomes the folder `code-review`.

Set `defaultGroup` if most new worktrees should land in one group:

```bash
arb config set defaultGroup review
```

Declare expected names in `groups` in the file itself so first use does not
ask. Pass `--group ""` to place a single worktree outside the default.

`arb list` grows a GROUP column once anything is grouped, and both `arb list`
and `arb remove` take `--group <name>` to narrow to one (`--group ""` selects
the ungrouped worktrees). When a removal empties a group's folder, the folder
goes with it.

## Validation

Arborist validates the config when it loads and when `arb config set` writes it:

- `owner` must be present and a single account login (no spaces or `/`).
- `defaultGroup` and every entry in `groups` must be usable as a folder name.

If validation fails, Arborist reports which field is wrong rather than guessing.

## Trust

Because Arborist discovers `.arborist.json` automatically (and its `editor`
value is run by `arb open`), it refuses to load a config that is a symbolic
link, not owned by you, or writable by group or others — similar to git's
`safe.directory` check. `arb config set` and `arb init` always write it
`0600` (owner read/write only); if you create or copy one by hand, keep it that
way (`chmod 600 .arborist.json`). See [SECURITY.md](../SECURITY.md) for the full
trust model.

## What is **not** stored here

Arborist never stores credentials or GitHub tokens. Authentication is handled by
the GitHub CLI (`gh auth login`). The config file contains no secrets.
