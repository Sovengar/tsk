# Changelog

## [Unreleased]

### Added
- `tsk update <id> --project <name>`: move a task to another project. The
  target project must exist and not be archived, and the task's current status
  must already be in the target project's workflow (otherwise the move is
  rejected with a clear error).
- `tsk delete <id> [--force]`: hard-delete a single task (and its comments).
  `--force` skips the confirmation prompt; the TUI confirms with `D` on the
  opened task.
- `tsk ask <task-id> [--harness NAME]`: hand a task off to an external AI
  harness from the CLI (JSON output by default; auto-selects only when exactly
  one harness is available).
- Ask AI picker in the TUI: press `a` on a focused task to list the available
  harnesses and hand the task off without blocking the dashboard.
- `[[harness]]` config list (additive to PATH detection; `name` + optional
  `binary`) and `[handoff]` section (`command` shell template + optional `cwd`)
  to configure harness discovery and the launch mechanism.
- `model.IsOverdue(due, now)`: pure helper to detect overdue tasks. A zero
  due date (no date) or a future one never appears overdue.

### Changed
- Documented the Ask AI handoff contract in `docs/adr/0001-ask-ai-handoff-contract.md`,
  `docs/FEATURES.md` and the README config example.
