# tsk

[![CI](https://github.com/Sovengar/tsk/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Sovengar/tsk/actions/workflows/ci.yml)

TUI + CLI task manager designed for developers juggling **1-3 projects**.
Dashboard, list and Kanban in the terminal, with per-project configurable
workflow and a Gantt that projects each person's queue.
All commands speak **JSON**, so AI agents can create, start and close tasks
by shell (`tsk add`, `tsk start`, `tsk done`, …).

## Features

The inventory of features — what each one does and how it is triggered (key,
flag, subcommand or config key) — lives in [`docs/FEATURES.md`](docs/FEATURES.md).
This README keeps the usage details: the CLI reference and the configuration.

In short: four views (Dashboard, List, Kanban, Gantt) over a per-project
configurable workflow, a capacity Gantt that projects each person's queue
respecting off-days, priority/assignee/estimate/tags/comments per task,
reversible project archiving, a JSON-first CLI for agent integration, and an
embedded SQLite database (pure Go, no CGO) with an optional TOML config.

## Installation

Requires **Go 1.26+**.

```bash
make build      # compiles to bin/tsk (local repo artifact)
make install    # installs to ~/.local/bin/tsk ($(PREFIX) by default)
```

Or directly:

```bash
go build -o ~/.local/bin/tsk ./cmd/tsk
```

The binary is called **`tsk`** (also the Go module name).

## Usage

```bash
tsk            # launches the TUI (no arguments)
tsk --help     # command list
```

### CLI

Full cycle example:

```bash
# Register a project with its workflow
tsk project add web --workflow backlog,todo,doing,review,done --list-order todo,doing,review,done

# Create a task
tsk add "Fix login redirect" --project web \
  --priority 3 --assignee @ana --estimate 0.5 --tag bug,auth

tsk list --project web --status todo   # filter
tsk start 1                            # move to "doing"
tsk review 1                           # move to "review"
tsk done 1                             # close
tsk gantt --assignee @ana --weeks 4    # project @ana's queue
```

Available commands:

| Command | Description |
|---|---|
| `tsk project add\|list\|show\|update\|remove\|archive\|unarchive` | Project and workflow management |
| `tsk add <title> --project X [--priority N] [--assignee @name] [--estimate N] [--tag T]` | Create task |
| `tsk list [--project X] [--status S] [--assignee A] [--tag T]` | List tasks |
| `tsk show <id>` | Task detail (with comments) |
| `tsk update <id> [--title] [--description] [--priority] [--assignee] [--estimate] [--tag] [--untag] [--tags]` | Edit metadata |
| `tsk move <id> <status>` | Move to a specific state |
| `tsk start\|review\|done\|cancel <id>` | Workflow shortcuts |
| `tsk comment add\|list\|remove` | Task comments |
| `tsk offday add\|list\|remove` | Non-working days per person |
| `tsk gantt [--project] [--assignee] [--from] [--weeks] [--json]` | Per-person projection |
| `tsk stats [--project X]` | Statistics by status and person |
| `tsk migrate` | Apply pending migrations |
| `tsk completion bash\|zsh\|fish` | Shell autocompletion |

Action commands (`add`, `update`, `move`, `start`, `done`, `cancel`,
`show`, `comment`, …) return **JSON to stdout**. Listing commands
(`list`, `project list`, `project show`, `offday list`, `gantt`, `stats`)
accept `--json`; without it they print an aligned table.

### TUI

The keys are always visible in the keybinds bar at the bottom of each view, and
`?` opens the help for the current view and the open modal. The inventory of
what each key does is in [`docs/FEATURES.md`](docs/FEATURES.md).

## Configuration

`~/.config/tsk/config.toml` (respects `$XDG_CONFIG_HOME`; override with
`$TSK_CONFIG`). All fields are optional:

```toml
[database]
path = ""                 # default: ~/.local/share/tsk/tsk.db (respects $XDG_DATA_HOME)

[editor]
command = "nvim"          # external editor (E key)

list_page_size        = 10    # tasks per page in List view
default_estimate_days = 1.0   # estimate for tasks without estimate (Gantt)
gantt_weeks           = 6     # default Gantt horizon

# Ask AI: hand a task to an external harness with `a` (TUI) or `tsk ask` (CLI).
[[harness]]
name   = "opencode"       # display name; also the default binary and {{harness}}
binary = "opencode"       # optional executable, exposed as {{harness_binary}}

[handoff]
command = "pane=$(herdr pane split --current --direction down --cwd {{cwd}} | jq -r '.result.pane.pane_id') && herdr agent start {{harness}}-$$ --kind {{harness}} --pane \"$pane\" -- --prompt \"$(cat {{prompt_file}})\""
cwd     = ""              # default: the directory where tsk was launched
```

The herdr example needs `jq` on `PATH` (it parses the `pane split` JSON).

The handoff is generic: `command` is a `/bin/sh` template with `{{harness}}`
(display name), `{{harness_binary}}` (the harness executable), `{{cwd}}` and
`{{prompt_file}}` (required) placeholders; swap it for tmux or a plain detached
command with no code change. The task text never reaches the
command line — it is written to a private temp file. The prompt does not embed
the description: it identifies the task, tells the harness to read it with
`tsk show <id>`, and to update it as it goes with `tsk move <id> <status>`,
listing the task project's valid statuses. The example opens the harness with
the prompt **pre-loaded but not
submitted** (herdr `agent start … -- --prompt`), so you can adjust the agent
before running it; omit `[handoff]` to disable Ask AI (the action then refuses,
naming the key to set).

A malformed config does not break anything: defaults are applied.

## Development

`make check` is the local equivalent of the `Lint`/`Test` gates (`Mutation` is
`make mutate-diff`, the coverage gate is `make coverage-check`):

```bash
make check   # golangci-lint + go vet + go test -race + go build ./... (never installs)
make test    # go vet + go test -race -count=1 ./...
make lint    # golangci-lint v2.13.2 (pinned, via go run)
make build   # compiles to bin/tsk (local repo artifact)
make install # installs to ~/.local/bin/tsk ($(PREFIX) by default)
```

Architecture: `cmd/tsk` (entry point), `internal/cli` (subcommands and JSON
output), `internal/config` (TOML + XDG paths), `internal/db` (SQLite, WAL,
migrations, CRUD), `internal/model` (Project, Task, workflow, Gantt) and
`internal/tui` (Bubbletea v2 dashboard).

## For AI agents

`AGENTS.md` in the root documents stack, commands, architecture, conventions and
the workflow (including binary deployment after each code
change). Read it before touching the repo.
