# Contributing to Arborist

Thanks for your interest in contributing! Arborist aims to be a small, safe,
well-tested CLI. Contributions that keep it that way are very welcome.

By participating in this project you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## Development setup

Requirements:

- [Go](https://go.dev/dl/) 1.23 or newer
- [Git](https://git-scm.com/)
- The [GitHub CLI](https://cli.github.com/) (`gh`) for features that talk to
  GitHub (not required to build or run the unit tests)

Common tasks:

```bash
go build ./cmd/arb   # build the binary
go test ./...        # run all tests
gofmt -l .           # list files needing formatting (should be empty)
go vet ./...         # static checks
```

Or use the Makefile: `make check` runs gofmt, vet, and the tests in one go,
and `make help` lists all developer targets.

All Go code must be formatted with `gofmt`, and `go test ./...` must pass
before a change is considered complete.

The module path is `github.com/jjacoblee/arborist`; internal packages are
imported under that prefix (e.g. `github.com/jjacoblee/arborist/internal/cli`).

## How we work

- **Small, reviewable changes.** Prefer focused pull requests over large mixed
  ones. Each PR should have a clear, single purpose.
- **Tests with behavior.** Add or update tests alongside any meaningful change.
  Pure functions (path/branch sanitization, config defaults, JSON parsing,
  worktree-list parsing) should have table-driven tests. Mock command execution
  behind the `Runner` interface; do not require real GitHub access in unit
  tests. Integration tests may use temporary local Git repositories only.
- **Conservative dependencies.** Prefer the standard library. The approved MVP
  dependencies are `github.com/spf13/cobra` and `github.com/charmbracelet/huh`.
  Adding anything else needs justification in the PR description.

## Project structure and package boundaries

```
cmd/arb/      Program entry point (main).
internal/cli/      Cobra command definitions and wiring (kept thin).
internal/config/   Workspace config: discovery, read/write, validation.
internal/exec/     Command runner and editor launcher abstractions.
internal/git/      Low-level Git command execution and parsing.
internal/github/   GitHub CLI (gh) integration.
internal/picker/   Interactive repository selection.
internal/paths/    Path expansion, sanitization, worktree path generation.
internal/worktree/ Higher-level Arborist worktree workflows (incl. ids).
internal/exectest/, internal/pickertest/  Test fakes for the runner and picker.
```

Guidelines:

- `internal/cli` parses args, loads dependencies, calls services, and prints
  results — it is **not** the application logic layer.
- `internal/git` must not know about terminal prompts.
- `internal/picker` must not create worktrees.
- Prefer dependency injection through structs over global mutable state.

## Security

Arborist runs local commands and modifies the filesystem. Please follow the
security model in [SECURITY.md](SECURITY.md): use `os/exec` with argument
arrays, validate and sanitize input used in paths, confine destructive
operations to configured directories, and never store or log credentials.

Report vulnerabilities privately as described in
[SECURITY.md](SECURITY.md) rather than in a public issue or PR.

## Commit and PR checklist

Before opening a pull request:

- [ ] `gofmt -l .` reports no files
- [ ] `go vet ./...` passes
- [ ] `go build ./cmd/arb` succeeds
- [ ] `go test ./...` passes
- [ ] Tests added/updated for the change
- [ ] Docs updated if behavior changed
