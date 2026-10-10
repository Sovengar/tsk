# Changelog

## [Unreleased]

### Added
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
