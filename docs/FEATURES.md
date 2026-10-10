# Features

Concise feature inventory of tsk. Details live in `../README.md`.

## Feature inventory

| Feature | What it does | Trigger |
|---|---|---|
| Four TUI views | List, Kanban, Gantt, Dashboard | `1`/`2`/`3`/`4` |
| Per-project workflow | State sequence defined at project registration | `tsk project add --workflow` |
| Kanban by state | One column per workflow state | `2` |
| Capacity Gantt | Per-person queue on business days | `3`; `tsk gantt` |
| Priority | none/low/medium/high with visual marker | `Ctrl+p`; `--priority` |
| Assignee | Person assigned to a task | `--assignee`; `m` (Dashboard) |
| Estimate | Task duration in days (feeds the Gantt) | `--estimate` |
| Tags | Labels on tasks | `--tag`; `t` (detail) |
| Comments | Per-task discussion | `c` (detail); `tsk comment` |
| Ask AI handoff | Hand a task to an external AI harness (detected + config) | `a` (List/Kanban/Detail); `tsk ask` |
| Off-days | Non-working days per person | `m` (Dashboard); `tsk offday` |
| Project archiving | Reversible soft delete | `d`/`r` (Dashboard); `tsk project archive` |
| JSON-first CLI | All commands output JSON | `tsk <cmd>` |
| Shell completion | bash/zsh/fish autocompletion | `tsk completion` |
| Database migrations | Schema version tracking | `tsk migrate` |
| TOML config | Optional configuration file | `~/.config/tsk/config.toml` |
| Embedded SQLite | Pure Go, no CGO | default storage |

## TUI views

- **Dashboard** — project overview with status counts, team workload, active tasks; triggered by `4`.
- **List** — paginated task table with columns (priority, status, assignee, title, tags, description); triggered by `1`.
- **Kanban** — board with one column per workflow state; triggered by `2`.
- **Gantt** — capacity projection per person on business days, respecting off-days; triggered by `3`.

## TUI keys

- **View switching** — `1`/`2`/`3`/`4` switch between List, Kanban, Gantt, Dashboard.
- **Navigation** — `hjkl` or arrows move the cursor; `Tab` cycles projects.
- **Task actions** — `s` start, `d` done, `x` cancel, `Ctrl+p` cycle priority.
- **Ask AI** — `a` on a focused task opens the harness picker and hands the task to the selected harness: the prompt tells the harness to read the task (`tsk show <id>`) and update it as it goes (`tsk move <id> <status>`), listing the project's valid statuses (List, Kanban, Detail).
- **Insert** — `i` creates a project (Dashboard) or task (List/Kanban).
- **Edit** — `e` opens inline description editor; `E` opens external editor.
- **Filters** — `/` opens the filter modal; `Esc` clears active filter.
- **Pagination** — `n`/`p` next/previous page, `N`/`P` first/last page (List).
- **Gantt scroll** — `h`/`l` scroll days, `g`/`G` jump to first/last task.
- **Kanban move** — `s` advance status, `S` retreat status.
- **Dashboard extras** — `A` toggle archived, `d` archive, `r` restore, `e` edit project, `m` assignees.
- **Help** — `?` toggles help overlay; `q` quits.

## Modals and detail

- **Task detail** — metadata, description, comments; opened with `Enter`.
- **Filter modal** — filter by project, status, assignee, priority, tag; fuzzy search; `Ctrl+R` resets.
- **New task modal** — priority, title, description, assignee with autocomplete, tags with suggestions; `Ctrl+S` creates.
- **Project modal** — name, workflow, list order; `Tab` cycles fields.
- **Assignee modal** — people roster with task counts; `Enter` opens detail with active tasks and off-days.
- **Off-day form** — start, end, note; `Tab` cycles fields.
- **Tag modal** — toggle tags on the opened task; `Tab` completes, `Enter` toggles.
- **Ask AI picker** — the harnesses available on this machine (built-in detection: `aider`, `claude`, `codex`, `gemini`, `opencode`, `pi`, each listed when its binary is on PATH; plus config-declared); `↑↓` move, `Enter` hands the task off detached, `Esc` closes. With none available it shows `No harnesses found`.
- **Confirmation modal** — yes/no for archive/restore project, delete off-day; `y`/`n`.
- **Description editor** — inline textarea embedded in detail; `Ctrl+S` saves, `Esc` cancels.
- **Help modal** — keybindings for the current view plus the Task detail, New task and Filters modals.

## CLI commands

- **Project management** — `tsk project add|list|show|update|remove|archive|unarchive` (`--archived` lists archived ones).
- **Task CRUD** — `tsk add`, `tsk list`, `tsk show`, `tsk update`, `tsk move`.
- **Workflow shortcuts** — `tsk start`, `tsk review`, `tsk done`, `tsk cancel`.
- **Ask AI** — `tsk ask <task-id> [--harness NAME]` hands a task to a harness and exits without waiting.
- **Comments** — `tsk comment add|list|remove`.
- **Off-days** — `tsk offday add|list|remove`.
- **Gantt** — `tsk gantt [--project] [--assignee] [--from] [--weeks]`.
- **Stats** — `tsk stats [--project]`.
- **Maintenance** — `tsk migrate`, `tsk completion bash|zsh|fish`.

## JSON contract

- All commands return JSON to stdout; errors are JSON on stderr with exit code 1.
- Action commands (`add`, `update`, `move`, `start`, `done`, `cancel`, `show`, `comment`) always output JSON.
- Listing commands (`list`, `project list`, `project show`, `offday list`, `gantt`, `stats`) accept `--json`; without it they print aligned tables.
- `tsk start` includes `next` field with the following workflow status.

## Config

- **Path** — `~/.config/tsk/config.toml`; respects `$XDG_CONFIG_HOME`, override with `$TSK_CONFIG`.
- **`[database] path`** — SQLite file location; default `~/.local/share/tsk/tsk.db` (respects `$XDG_DATA_HOME`).
- **`[editor] command`** — external editor for `E` key; default `nvim`.
- **`list_page_size`** — tasks per page in List view; default 10.
- **`default_estimate_days`** — estimate for tasks without one in the Gantt; default 1.0.
- **`gantt_weeks`** — default Gantt horizon; default 6.
- **`[[harness]] name` / `binary`** — additive harnesses: listed in the Ask AI picker even when detection misses them; `binary` defaults to `name` and is exposed to the command as `{{harness_binary}}`. A declared entry replaces a detected one with the same name.
- **`[ai.ask.handoff] command`** — shell template that launches a harness, with `{{harness}}` (display name), `{{harness_binary}}`, `{{cwd}}` and `{{prompt_file}}` (required) placeholders; values are shell-quoted by tsk. Empty = Ask AI disabled.
- **`[ai.ask.handoff] cwd`** — working directory for the handoff; default: where `tsk` was launched.
- **`[ai.ask.handoff] prompt`** — prompt template handed to the harness, with `{{id}}`, `{{title}}`, `{{project}}` and `{{statuses}}` (the task project's workflow; an empty workflow drops the state clause). Empty = built-in default ("Run this task: …').
- Malformed config falls back to defaults silently (the "config never fails" pattern). A single malformed `[[harness]]` entry is dropped without discarding the rest of the file.

## Storage and migrations

- Embedded SQLite via `modernc.org/sqlite` (pure Go, no CGO).
- WAL mode with 5s busy timeout.
- 8 schema migrations (001-008) tracking: initial schema, comments, archiving, list order, estimate, off-days, tags, and two legacy-column drops (`tasks.position`, `projects.path`).
- 3 reserved migration slots (009-011) for removed data migrations.
- Atomic writes (tmp + rename) for persistence.
