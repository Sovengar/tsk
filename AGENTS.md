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

## New feature → docs/FEATURES.md + skill

Any **new feature** — and any user-visible change to an existing one — must be
documented in `docs/FEATURES.md` **in the same change** (create the file if it does
not exist yet): add or update its entry with what it does and how it is
triggered (key, flag, CLI subcommand or config key). A feature that is not in
`docs/FEATURES.md` does not exist for the next reader. Keep it a concise inventory,
not a tutorial: the details live in `README.md`.

The same change must also update the global skill
**`~/.agents/skills/tsk/SKILL.md`** — trigger, subcommands/flags, JSON shapes
and keybinds that changed. The skill is the runtime contract agents load before
working on tsk; a feature absent from it does not exist for them.

## CI and `main` protection

Two workflows, three required checks: `Lint`, `Test` and `Mutation`. Neither
workflow uses `paths` filters (a skipped workflow leaves its required checks
stuck pending and blocks every PR that skips it).

### `CI` (`.github/workflows/ci.yml`) — every PR, and pushes to `main`

The gate. `Mutation` is a job of this file: two jobs, `Lint` ∥ `Test`, plus the
mutation job on PRs. All on `ubuntu-24.04`.

- **`Lint`** — `make lint` (golangci-lint **v2.13.2** pinned in the `Makefile`).
- **`Test`** — `go test -race -count=1 -covermode=atomic -coverprofile=coverage.out
  ./...` (the whole suite, no `-short`), then **`Coverage gate`** →
  `scripts/diff-coverage.sh`: the PR's **diff at 100%** and the **total against
  `scripts/coverage-floor`** (100.00, can only go up). The base is the explicit
  merge-base, hence `fetch-depth: 0`.

### `CI fast` (`.github/workflows/ci-fast.yml`) — every commit on a branch

Build + unit tests, no lint, no mutation. **Not required**: a red never blocks a
merge and a green never authorises one. Its 10m ceiling is for a cold runner, not
for the ~2m30s the suite takes warm.

### `Mutation` — a job of `ci.yml`, non-draft PRs only

Runs on non-draft PRs. The job always reports — no `needs:`, no
`continue-on-error`, the only `if:` is the event gate — which is what makes it a
required check.

- **Where the decision lives**: `scripts/mutate.sh` measures AND decides in one
  step (`scripts/mutate.sh --diff --ci --summary …`); the workflow only brings
  paths, refs and budget. *"Could not measure"* is a red, never a green, and a
  red says how to fix it in the step summary.
- **Budget** (mandatory under `--ci`, asserted at startup):
  `2*CAP < STALL < CEILING`, `CEILING + SETUP_RESERVE < JOB_CEILING` →
  `180s · 4 workers · 8m · 13m · +600s · 25m`.
- **Where it runs**: the gate is CI (the `Mutation` job above). Locally it is
  never automatic: run it manually, only when needed, with `make mutate`
  (whole module) and `make mutate-diff` (the diff against `MUTATE_BASE`), same
  wiring as CI. `make coverage-check` is the local equivalent of the coverage
  gate. The coefficient is derived from `ceil(cap / coverage pass)`, so there
  is no pinned `MUTATE_TIMEOUT` to tune.
- **Scope**: `MUTATE_EXCLUDE` in the `Makefile` (`cmd/`), read by
  `scripts/mutate.sh` so local and CI gate the same set.
- **Two counts, and they are different numbers**: `WATCH_LINES` (every mutant
  considered = the supervisor's denominator) and `EXPECTED_MEASURED` (in-scope
  `RUNNABLE` only = what the verdict compares against the report's
  `mutants_total`, which is `killed + lived + notViable`). `SKIPPED` leaves the
  scope and `NOT COVERED` sits in no cover block (Go cover starts a case
  clause after the colon), so neither can be measured and neither belongs in
  the denominator; the exclusion and its reason are published by the verdict.
- The gitconfig is neutralised (`GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` →
  `/dev/null`, `GIT_CONFIG_NOSYSTEM=1`): gremlins computes its `--diff` ranges
  with a plain `git diff` run with the AMBIENT config, so a dev's
  `diff.algorithm`/`diff.interhunkcontext` would make the local loop measure a
  different mutant set than CI's defaults. Hermetic by construction.
- **Files that make the gate possible** (all committed):
  `.mutation-allowlist` (survivors accepted **by line**; missing = red with the
  command to seed it), `.mutation-timeouts` (`<file> <ceiling>` for mutants that
  expired and were never tested), `scripts/coverage-floor` (the total, ratchet),
  `scripts/watchdog.sh` + `scripts/watchdog_test.sh` (vendored frozen copy) and
  `scripts/mutate_test.sh` (the gate's red paths, ~1s, run as the `Shell suites`
  CI step).

`main` is protected by the **`protect-main`** ruleset: requires a PR with
`Lint`, `Test` and `Mutation` green, and blocks force-push and branch deletion.
`Build` is not a check anymore: it moved to the advisory `CI fast`, so a required
`Build` context would deadlock every PR. The admin role bypass is deliberate
(owner escape hatch): an admin can merge a red PR or force-push, so the gate is
absolute only for non-admins.

To (re)apply the protection (idempotent, requires `gh` admin + `jq`):

```bash
scripts/setup-repo-protection.sh            # applies
scripts/setup-repo-protection.sh --dry-run  # shows without mutating
```

The script derives the default branch and the real check contexts; it does not
hardcode `main` or check names.

### Waiting for CI

To follow a PR's checks, wait with `gh run watch <run-id> --exit-status`
(or `gh pr checks <n> --watch`). Never `sleep` + `gh pr checks`: runs go
stale after a force-push and you have to ask for the id again.

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
