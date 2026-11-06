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
tsk project add web --workflow backlog,todo,doing,review,done --list-order todo,doing,review,done --path ~/src/web

# Create a task (its own --path overrides the project's for the AI handoff)
tsk add "Fix login redirect" --project web \
  --priority 3 --assignee @ana --estimate 0.5 --tag bug,auth --path ~/src/web-login

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
| `tsk add --ai [--wait] ... "<request>"` | Create a task from a free-form request (background + notification) |
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

# Ask AI: one handoff for every harness with `a` (TUI) or `tsk ask` (CLI).
# `ai` builds the agent-specific argv; `mux` only decides where it appears.
[handoff]
command = "mux --container {{container}} --layout tab --working-dir {{working_dir}} --session-id {{session_id}} -- agentbridge exec --harness {{harness}} --model {{model}} --effort {{effort}} --prompt-file {{prompt_file}}"
working_dir = ""          # default: task path, then project path, then ~/Tasks
prompt  = "Run this task: Task #{{id}}: {{title}} ({{project}}). Check it with `tsk show {{id}}`. Update its state as you go with `tsk move {{id}} <status>`; valid statuses: {{statuses}}."
model   = ""              # optional global default for the picker
thinking = ""             # optional global effort default
```

### AI task creation

`tsk add --ai` turns a free-form request into a task. The harness investigates
the working directory, answers with a JSON spec, and tsk validates it
(project, assignee, title) before inserting the row itself.

```bash
tsk add --ai "revisar el recovery de voice-gate"     # background + notification
tsk add --ai --wait --project voice-gate "revisar"   # foreground, prints spec + task
```

Its settings live in the `[ai]` section of `~/.config/tsk/config.toml`:

```toml
[ai]
harness  = "pi"     # pi, codex, opencode, hermes, jcode
model    = ""       # provider/model, or a category from the Pi catalog (dumb, free, smart)
thinking = ""       # optional effort override (off, low, medium, high, ...)
notify   = true     # desktop notification when a background run finishes
prompt   = ""       # default: ~/.config/tsk/ai.md
container = "Background"  # mux workspace that hosts every AI run
focus     = true    # bring that run's pane to the front when it finishes
```

`model` accepts a Pi model category (`dumb`, `free`, `smart`, ...) from
`~/.pi/agent/extensions-extras/models-by-category.json`: tsk resolves it to the
physical model (and its thinking level) before the harness runs, because Pi
itself rejects a category name passed to `--model`.

The prompt itself is a plain markdown file, `~/.config/tsk/ai.md` (override with
`$TSK_AI_PROMPT` or `ai.prompt`):

```markdown
Create exactly one task and reply with JSON. Request: {{text}}
Projects: {{projects}}. Assignees: {{assignees}}. Tags: {{tags}}.
```

The body places `{{text}}`, `{{projects}}`, `{{assignees}}`, `{{tags}}`,
`{{workdir}}`, `{{project_hint}}` and `{{default_assignee}}`; tsk fills them
with live state, so the harness never runs `tsk` to read it. Missing or blank
`ai.md` falls back to the embedded prompt.

Background runs open a new tab in the shared mux workspace (`ai.container`,
`Background` by default) without stealing focus, so closing the shell they were
started from loses nothing. When a run finishes, the focus moves to its tab
(`ai.focus`, on by default), the pane keeps the worker's JSON result, the job and
log stay under `$XDG_STATE_HOME/tsk/ai`, and `notify-send` fires as well. Without
a multiplexer (no herdr/tmux/kitty/wezterm) the worker falls back to a detached
process with that log.

The command is a `/bin/sh` template. Placeholders: `{{harness}}`, `{{working_dir}}`,
`{{container}}`, `{{model}}`, `{{effort}}`, `{{session_id}}` and `{{prompt_file}}` (required).
`{{session_id}}` is `tsk-<project>-<task>` (e.g. `tsk-projects-92`); `mux` uses it only
to label a tab. `{{container}}` is the projectDir basename (e.g. `tsk`, `gitdash`);
`mux` uses it as the herdr workspace, so one project keeps one workspace. The task text never reaches the command line: it is written to a
private temp file pointed at by `{{prompt_file}}`.

The working directory resolves in this order: the task's own `path`
(`tsk add|update --path`, or `P` in the detail), the project's `path`
(`tsk project add|update --path`), the configured `handoff.working_dir`, and
finally `~/Tasks` (created on demand). A leading `~` expands to the home
directory. This lets a task from a catch-all project such as "Projects" point
at the real repository.

No per-harness command is needed. `agentbridge exec` knows each agent's flags (`pi`
positional prompt, `opencode --prompt`, `codex` flags, …). Swapping the
presenter (pane, tab, tmux, herdr, kitty) is a config/environment change.

A malformed config does not break anything: defaults are applied.

## Development

`make check` is the local equivalent of the `Lint`/`Test` gates (`Mutation` is
`make mutate-diff`, the coverage gate is `make coverage-check`):

```bash
make check   # golangci-lint + go vet + go test -race + go build ./... (never installs)
make test    # go vet + go test -race -count=1 ./...
make lint    # golangci-lint latest (via go run)
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
