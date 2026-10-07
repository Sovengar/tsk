# AGENTS.md — tsk

Guide for agents with no prior context about this project.

## What it is

TUI + CLI task management, designed for developers working on 1-3 projects
simultaneously. Full AI integration via CLI: AI agents run `tsk add`, `tsk start`,
`tsk done` etc. by shell.

## Stack

- Go 1.26+, module `tsk`
- `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2` (import paths `charm.land`, NOT github.com/charmbracelet)
- `modernc.org/sqlite` (pure Go, no CGO)
- `github.com/BurntSushi/toml`
- Binary name: **`tsk`** (not `taskd`)

## Commands

```bash
make build          # compiles to bin/tsk (local repo artifact)
make install        # installs to $(PREFIX)/bin/tsk (default ~/.local/bin/tsk)
make uninstall      # uninstalls from $(PREFIX)/bin/tsk
make test           # go vet + go test
make all            # test + build
make check          # build + lint + test (local gate = CI, never installs)
```

```bash
./scripts/gen-fixtures.sh   # (if exists) regenerates test data
```

## Crucial step after any code change

**Deploy the binary** (tests/smoke with `go run` do not update the installed
binary; the user runs the bin from `~/.local/bin`, not the repo):

```bash
make install
```

Without this step, any verification the user does on the TUI uses the old
version. Run it ALWAYS after finishing a code task, after verification
(`make test`).

## CI and `main` protection

The `.github/workflows/ci.yml` workflow runs on every PR, every push to `main`
and manually (`workflow_dispatch`). It has three jobs, all on `ubuntu-24.04`:

- **Build** — `go build ./...` + `go vet ./...`
- **Lint** — `make lint` (golangci-lint v2.13.2 pinned in the `Makefile`)
- **Test** — `go test -race -count=1 -coverprofile=coverage.out ./...` + coverage summary

`main` is protected by the **`protect-main`** ruleset: requires a PR with those
three checks green, and blocks force-push and branch deletion. The admin role
bypass is deliberate (owner escape hatch): an admin can merge a red PR or
force-push, so the gate is absolute only for non-admins.

To (re)apply the protection (idempotent, requires `gh` admin + `jq`):

```bash
scripts/setup-repo-protection.sh            # applies
scripts/setup-repo-protection.sh --dry-run  # shows without mutating
```

The script derives the default branch and the real check contexts; it does not
hardcode `main` or check names.

## Architecture

```
cmd/tsk/main.go            → entry point: CLI dispatch → TUI
internal/cli/cli.go        → Run(args) bool — all CLI subcommands
internal/config/config.go  → TOML config, XDG paths, defaults
internal/db/               → SQLite connection, WAL, migrations, CRUD
internal/model/            → Project, Task, workflow helpers
internal/tui/              → Bubbletea v2: Dashboard, List, Kanban, Detail modal
```

## Conventions

- **Files**: `snake_case.go`
- **Types**: `PascalCase`
- **Comments**: English
- **`internal/`** only — no exported packages
- **Config never fails**: defaults + warning pattern
- **Atomic writes**: tmp + rename for persistence
- **Bubbletea v2**: imports with `charm.land/*` (NOT `github.com/charmbracelet`)
- **Tests**: directly on the model (like vroom/gitdash), white-box testing
- **CLI entry**: `internal/cli/cli.go` with `Run(args []string) bool`

## CLI — output

All commands return JSON to stdout by default for AI integration.
Explicit `--json` flags on `project list`/`project show`/`stats` for JSON output.
Without `--json`, they use tabwriter for aligned human output.

## Binary name

The binary, the Go module and the project directory are all called **`tsk`**.
