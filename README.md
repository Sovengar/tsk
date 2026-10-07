# tsk

[![CI](https://github.com/Sovengar/tsk/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Sovengar/tsk/actions/workflows/ci.yml)

TUI + CLI task manager designed for developers juggling **1-3 projects**.
Dashboard, list and Kanban in the terminal, with per-project configurable
workflow and a Gantt that projects each person's queue.
All commands speak **JSON**, so AI agents can create, start and close tasks
by shell (`tsk add`, `tsk start`, `tsk done`, …).

## Features

- **Four views** with key switching: Dashboard, List, Kanban and Gantt.
- **Per-project workflow**: the state sequence (e.g. `backlog → todo →
  doing → review → done`) is defined when registering the project and must include
  `done`; the list presentation order is configurable and independent.
- **Kanban by state**: one column per state of the project workflow.
- **Capacity Gantt**: projects each person's queue on business days,
  respects *off-days* and shows unassigned tasks. `--project` and
  `--assignee` filter the view, they do not recalculate dates: capacity is one
  and is distributed across projects.
- **Priority, assignee, estimate and tags** per task, with filtering by
  project, status, person and tag.
- **Due dates**: `IsOverdue` compares the due date with a given
  instant; without a due date (zero) a task never appears overdue.
- **Comments** per task and **off-days** per person (date range + note).
- **Reversible project archiving** (hides it, its tasks and comments without
  deleting them).
- **JSON-first CLI** for agent integration, with `completion` for
  bash/zsh/fish and `migrate` for database migrations.
- **Embedded SQLite** (pure Go, no CGO) and optional **TOML** config; without a
  config file it works with defaults.

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

| Key | Action |
|---|---|
| `1`/`2`/`3`/`4` | Switch to List / Kanban / Gantt / Dashboard |
| `hjkl`, arrows | Navigate |
| `Tab` | Cycle between projects |
| `Enter` | Open the detail of the task under the cursor |
| `i` | Insert (project in Dashboard, task in List/Kanban) |
| `s` | Start task (List) / change state (Kanban) |
| `d` / `x` | Mark as `done` / cancel |
| `e` / `E` | Edit / open in external editor |
| `/` | Filters |
| `Ctrl+p` | Priority |
| `n`/`p`, `N`/`P` | Pagination (List) |
| `m` | Assignees and off-days (Dashboard) |
| `?` | Help |
| `q` | Quit |

## Configuration

`~/.config/tsk/config.toml` (respects `$XDG_CONFIG_HOME`; override with
`$TSK_CONFIG`). All fields are optional:

```toml
[database]
path = ""                 # default: ~/.local/share/tsk/tsk.db (respects $XDG_DATA_HOME)

[editor]
command = "nvim"          # external editor (E key)

[list]
page_size        = 10     # tasks per page in List view
default_estimate_days = 1.0  # estimate for tasks without estimate (Gantt)
gantt_weeks      = 6      # default Gantt horizon
```

A malformed config does not break anything: defaults are applied.

## Development

`make check` is the local equivalent of the CI gate (Build/Lint/Test jobs):

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
