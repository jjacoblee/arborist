# Security Policy

Arborist executes local Git and GitHub CLI commands and modifies the
filesystem, so we take security seriously and treat all input as untrusted.

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report privately using GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
on this repository (Security tab → "Report a vulnerability"), or contact the
maintainers at the address listed on the repository profile.

When reporting, please include:

- A description of the issue and its impact.
- Steps to reproduce, if possible.
- The Arborist version (`arb --version`) and your OS.

We will acknowledge your report, investigate, and keep you informed of progress.

## Supported versions

Arborist is pre-1.0 and under active development. Security fixes are applied to
the latest release. Until a stable release exists, please track the default
branch.

## Security model

Arborist is designed to be safe by default:

- **Command execution** uses `os/exec` with argument arrays for all Git and
  GitHub CLI calls — never a shell string built from user input. The one
  deliberate exception is the `setup` commands you configure, which are run
  through a shell (`sh -c`) because they are shell by nature; those come only
  from your own ownership-checked workspace config (see "Workspace config
  trust"), never from a repository.
- **Path safety**: branch names are validated and sanitized before being used as
  filesystem path segments, and destructive operations are confined to the
  configured Arborist directories.
- **Workspace config trust**: Arborist discovers `.arborist.json` by walking up
  from the current directory, so it treats that file like a shell rc or
  `.git/config`. A discovered config is loaded only if it is a regular file,
  owned by the current user, and not writable by group or others; otherwise
  Arborist refuses it (a "dubious ownership" check in the spirit of git's
  `safe.directory`). The config's `editor` value is run as a command by
  `arb open`, and `worktreeRoot` directs where files are written — so, as with
  any per-directory config tool, **only run `arb` in directories you trust**
  (for example, don't run `arb open` inside a repository you just cloned from an
  untrusted source without reviewing its `.arborist.json`). Arborist also runs
  `git` inside the repositories it manages, which inherits git's own repository
  trust model (`safe.directory`).
- **Confirmation before deletion**: destructive actions require explicit
  confirmation and print the exact paths that will be removed. Dirty worktrees
  are never removed silently. `--force` is only honored when explicitly passed.
- **No credential handling**: Arborist never stores or logs GitHub tokens. It
  relies on the GitHub CLI (`gh`) for authentication.
- **No surprising side effects**: Arborist never runs anything from a repository
  on its own initiative — it does not look inside a clone for scripts to run or
  install dependencies by itself. It runs only the `setup` commands from your
  own ownership-checked config (see "Command execution" above). Those commands
  are yours to choose, and typical ones (`pnpm install`, `uv sync`) do install
  the repo's dependencies and can trigger its `postinstall`/lifecycle scripts —
  so configure setup only for repositories you trust.
- **No telemetry**: Arborist runs entirely locally except for the Git and
  GitHub CLI operations you explicitly trigger.

If you believe any of these properties is violated, that is a security issue —
please report it as described above.
