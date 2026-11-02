# Features

Concise feature inventory of tsk. Details live in `../README.md`.

## Feature inventory

| Feature | What it does | Trigger |
|---|---|---|
| Four TUI views | List, Kanban, Gantt, Dashboard | `1`/`2`/`3`/`4` |
| Per-project workflow | State sequence defined at project registration | `tsk project add --workflow` |
| Handoff path | Working directory per task or per project for Ask AI; a task path overrides the project path, and with neither set it falls back to `~/Tasks` | `tsk add|update --path`; `tsk project add|update --path`; `P` (detail) |
| Kanban by state | One column per workflow state; empty columns are hidden and a status strip marks the hidden ones | `2` |
| Capacity Gantt | Per-person queue on business days | `3`; `tsk gantt` |
| Priority | none/low/medium/high with visual marker | `Ctrl+p`; `--priority` |
| Assignee | Person assigned to a task | `--assignee`; `m` (Dashboard) |
| Estimate | Task duration in days (feeds the Gantt) | `--estimate` |
| Tags | Labels on tasks | `--tag`; `t` (detail) |
| Comments | Per-task discussion | `c` (detail); `tsk comment` |
| Status overlay | Move a task to any workflow status, backwards included | `m` (List/Kanban/Detail) |
| Cross-project move | Move a task to another project, then pick its status | `M` (List/Kanban/Detail) |
| Ask AI handoff | Hand a task to an external AI harness with per-task harness/model/thinking selection | `a` (List/Kanban/Detail); `tsk ask` |
| Off-days | Non-working days per person | `m` (Dashboard); `tsk offday` |
| Project archiving | Reversible soft delete | `d`/`r` (Dashboard); `tsk project archive` |
| JSON-first CLI | All commands output JSON | `tsk <cmd>` |
| Shell completion | bash/zsh/fish autocompletion | `tsk completion` |
| Database migrations | Schema version tracking | `tsk migrate` |
| TOML config | Optional configuration file | `~/.config/tsk/config.toml` |
| Remembered project | Restores the last selected project in List, Kanban and Gantt | `Tab` or Project filter; `$XDG_STATE_HOME/tsk/state.json` |
| Embedded SQLite | Pure Go, no CGO | default storage |

## TUI views

- **Dashboard** — project overview with status counts, team workload, active tasks; triggered by `4`.
- **List** — paginated task table with columns (`Prio`, status, assignee, title, tags); the title column stretches to the terminal width and tags drop first when space runs out. No description column (it lives in the detail). Two groups: `backlog` tasks always show last and every other status first, and inside each group tasks go by priority (high to low); triggered by `1`.
- **Kanban** — board with one column per workflow state; a state with no tasks is hidden (the default `all active` filter hides `done` and `cancelled`), and a status strip above the board lists every state with its **real** task count (the status filter does not zero it) and marks the hidden ones as `(hidden)`; triggered by `2`.
- **Gantt** — capacity projection per person on business days, respecting off-days; triggered by `3`.

## TUI keys

- **View switching** — `1`/`2`/`3`/`4` switch between List, Kanban, Gantt, Dashboard.
- **Navigation** — `hjkl` or arrows move the cursor; `Tab`/`Shift+Tab` cycle projects forwards/backwards.
- **Task actions** — `m` opens the status overlay (the next workflow status first, then the already-passed ones, `cancelled` last); `M` moves the task to another project and then asks for its status; `D` hard-deletes the opened task (with confirmation); `Ctrl+p` cycle priority.
- **Ask AI** — `a` on a focused task opens the harness picker and hands the task to the selected harness: the prompt tells the harness to read the task (`tsk show <id>`) and update it as it goes (`tsk move <id> <status>`), listing the project's valid statuses (List, Kanban, Detail). `↑↓/jk` move the harness or effort value, `h/l` or `←` move the focus (the right arrow is ignored), `Tab`/`Shift+Tab` cycle the effort, `/` is the only way to open and change the model picker (typing filters it, `↑↓` choose, `Enter`/`Esc` close it), `Enter` hands off and `Esc` closes.
- **Insert** — `i` creates a project (Dashboard) or task (List/Kanban).
- **Edit** — only from the task detail: `e` opens the inline description editor and `E` opens the external editor. In List and Kanban neither key does anything.
- **Filters** — `f` opens the filter modal; `Esc` clears active filter. With no field entered, `↑↓/jk` move the cursor and the right pane previews the field's options; the modal keeps a semi-fixed height that only shrinks when the terminal is too short; `Enter` enters the field, and inside it `↑↓/jk` move the value, `←→` cycle it and `Enter` applies and moves to the next field.
- **Search** — `/` searches tasks by title and description; `Enter` keeps the query, `Esc` restores or clears it.
- **Forms** — New task, Project and Off-day open with no field entered: `↑↓/jk` move the field cursor, `Enter` enters a field, `Enter` inside leaves it and moves to the next one, `Esc` leaves it backwards, and `Ctrl+S` saves or confirms. `Esc` with no field entered closes the overlay. The Ask AI picker is not a form: arrows move the harness or effort value; the model changes only through `/`.
- **Pagination** — `n`/`p` next/previous page, `N`/`P` first/last page (List).
- **Gantt scroll** — `h`/`l` scroll days, `g`/`G` jump to first/last task.
- **Dashboard extras** — `A` toggle archived, `d` archive, `r` restore, `e` edit project, `m` assignees. Creating a task in a project that is archived revives it, so the archived view drops it.
- **Help** — `?` toggles help overlay; `q` quits.

## Modals and detail

- **Task detail** — metadata, description, comments; opened with `Enter`; `m` opens the status overlay, `M` moves it to another project, `P` edits the task working directory, `d` deletes the selected comment, `D` hard-deletes the task. The Path line shows the effective directory and its source (`task`, `project` or `default`).
- **Filter modal** — filter by project, status, assignee, priority, tag; the tag options are the tags in use in the selected project (the union when the project is `all`), and a tag filter that no longer exists in the new project is cleared on a project switch; fuzzy search; `Ctrl+R` resets.
- **New task modal** — priority, title, description, assignee with autocomplete, tags with suggestions; `Ctrl+S` creates.
- **Project modal** — name, workflow, list order; `Tab` cycles fields.
- **Assignee modal** — people roster with task counts; `Enter` opens detail with active tasks and off-days.
- **Off-day form** — start, end, note; `Tab` cycles fields.
- **Tag modal** — toggle tags on the opened task; `Tab` completes, `Enter` toggles.
- **Task path modal** — `P` in the detail; `Enter` saves, `Esc` cancels, an empty value clears the task override and inherits the project path.
- **Ask AI picker** — shows the selected harness, model and effort in one wide row with extra space between harness and model and `⟡` before the effort; it expands for long model IDs when the terminal has room. Its Keybinds section lists `↑↓`/`j`/`k` to move, `/` to search models, `Tab` to cycle effort, `Enter` to hand off and `Esc` to close. Model lists come from local harness registries: Pi `models-store.json`, Codex `models_cache.json`, OpenCode `model.json`, Hermes `provider_models_cache.json`/`ollama_cloud_models_cache.json`, and Jcode `~/.config/jcode`/`~/.jcode/cache`. With none available it shows `No harnesses found`.
- **Confirmation modal** — yes/no for archive/restore project, delete off-day, delete task; `y`/`n`.
- **Description editor** — inline textarea embedded in detail; `Ctrl+S` saves, `Esc` cancels.
- **Help modal** — keybindings for the current view plus the Task detail, New task and Filters modals.

## CLI commands

- **Project management** — `tsk project add|list|show|update|remove|archive|unarchive` (`--archived` lists archived ones). Adding a task to an archived project (`tsk add TITLE --project P`, or `tsk update ID --project P`) revives it: the project becomes active and its pre-existing tasks come back with it. Plain `project unarchive` is unchanged.
- **Handoff path** — a task can carry its own working directory with `tsk add|update --path <dir>` or `P` in the detail. It overrides the project path; the project path is set with `tsk project add|update --path`. With neither set, the handoff uses `~/Tasks`. A leading `~` expands to the home directory. Task JSON carries `path` only when the task has an override.
- **Task CRUD** — `tsk add`, `tsk list`, `tsk show`, `tsk update`, `tsk move`, `tsk delete`.
- **Workflow shortcuts** — `tsk start`, `tsk review`, `tsk done`, `tsk cancel`.
- **Ask AI** — `tsk ask <task-id> [--harness NAME] [--model MODEL] [--thinking LEVEL]` hands a task to a harness and exits without waiting.
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
- **`[handoff] command`** — one shell template for every harness. `{{harness}}`, `{{working_dir}}`, `{{container}}`, `{{model}}`, `{{effort}}`, `{{session_id}}` and `{{prompt_file}}` (required) are expanded by tsk. The shared `ai exec` binary builds the agent-specific argv; `mux` presents the process. `{{container}}` is the projectDir basename and becomes the herdr workspace (one per project).
- **`[handoff] working_dir`, `prompt`, `model`, `thinking`** — optional global defaults. The prompt template uses `{{id}}`, `{{title}}`, `{{project}}` and `{{statuses}}`.
- Malformed config falls back to defaults silently (the "config never fails" pattern). Unknown/legacy sections are ignored without discarding the handoff.

## Storage and migrations

- Embedded SQLite via `modernc.org/sqlite` (pure Go, no CGO).
- WAL mode with 5s busy timeout.
- 14 schema migrations (001-014): initial schema, comments, archiving, list order, estimate, off-days, tags, legacy-column drops (`tasks.position`, `projects.path`), the restored project `path` (012), the archived-project revive triggers (013) and the per-task `tasks.path` (014). Migration 014 also backfills a project path from `~/dev/projects/<name>` when that directory exists; projects without a match resolve to `~/Tasks`.
- 3 reserved migration slots (009-011) for removed data migrations.
- Atomic writes (tmp + rename) for persistence.
