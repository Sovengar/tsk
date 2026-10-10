# Ask AI: hand a task off to an external coding harness

Status: implemented

## Why

Today a task in `tsk` is inert: you read it and act yourself. The value of `tsk`
is being the single place a developer keeps their work, so the natural next step
is to hand the task *as context* to an AI coding harness and let it act on it,
without leaving the TUI and without the TUI freezing while the harness runs.

The user works across 1–3 projects and drives several harnesses (opencode, pi,
codex, claude, …). Today that means: copy the task by hand, switch terminal,
paste, pick a harness. `Ask AI` collapses that into one keystroke on a task; the
CLI equivalent makes the same flow scriptable for other agents.

## What (scope IN)

- **New task action `a` ("Ask AI")** on a focused task, in the same places the
  detail modal can be opened (at minimum: List / Kanban cursor, task detail).
  A per-task action, disabled when no task is focused. *(Verified: `a` is free in
  every view — List, Kanban, Dashboard, Detail; the only existing binding is
  inside the assignee/off-day modal, a different context.)*
- **Harness picker overlay**: a modal listing the AI harnesses available on this
  machine — best-known harnesses actually installed (PATH-probed) **plus** any
  entries declared in the config file (additive). `Esc` cancels. When nothing is
  available, the overlay shows exactly `No harnesses found`.
- **Config-declared harness list (additive)**: entries may declare harnesses that
  detection missed; per-entry display name + binary/command.
- **Installed-harness detection**: built-in name→binary mapping probed with PATH
  lookup (`exec.LookPath`); the overlay lists detection results plus config
  additions (deduped).
- **Prompt pointing at the task**: the harness receives a prompt that names the
  concrete `tsk` CLI commands to read the task (`tsk show <id>`) and update it
  (`tsk move <id> <status>`) plus the task project's valid statuses; the
  description is not embedded. Exact composition is pinned in
  `behavior.feature`.
- **Non-blocking handoff**: launching the harness must NOT block the TUI or the
  CLI. The handoff is fired and `tsk` returns to normal interaction immediately;
  the harness runs independently (its own pane/window/process). No
  `tea.ExecProcess` suspension of the dashboard.
- **Config-driven handoff mechanism**: *how* the harness is launched is a
  command/template in the config file (with an optional cwd override; default
  cwd = where `tsk` was launched). With no handoff configured the action refuses
  with a clear message naming the config key to set. This user's instance will
  hand off by opening a **new herdr pane**, but herdr must never be a hardcoded
  dependency — it is just this instance's configuration value.
- **CLI subcommand equivalent**: the same Ask AI flow is reachable from the CLI
  for scripts/agents (task id + harness selection; JSON output per repo
  convention).
- **Docs**: a `docs/FEATURES.md` entry (what + trigger) in the same change.

## Scope OUT (explicitly not now)

- No harness-side session tracking, output capture, or result read-back into
  `tsk`. Fire-and-forget; `tsk` does not observe what the harness does.
- No per-task harness preference persistence, no harness history.
- No bundled/auto-installed harnesses and no harness-specific prompt formats
  (opencode vs. claude vs. pi) — one generic prompt template.
- No multi-harness fan-out (one pick, one handoff).
- No per-project path field: `projects` has no stored path (dropped in schema
  V6) and we do not add one.

## Acceptance criteria (high level)

1. With a task focused, pressing `a` opens an overlay listing available
   harnesses.
2. The list = PATH-detected known harnesses + config-declared additions
   (deduped); config entries work even for binaries detection does not know.
3. Selecting a harness hands the task off via the configured handoff command;
   the TUI stays responsive (no blocking) and the CLI returns without waiting
   for the harness.
4. The prompt names `tsk show <id>` and `tsk move <id> <status>` plus the task
   project's valid statuses (the description is not embedded).
5. Zero harnesses available → overlay shows exactly `No harnesses found`.
6. No handoff command configured → the action refuses with a clear message
   naming the config key to set; never a silent no-op.
7. Handoff runs with cwd = tsk's launch directory unless the config overrides it.
8. Handoff is generic: swapping the config command (herdr, tmux, plain detached
   exec) works with no code change; herdr is not referenced in code.
9. Config is additive and never fatal: malformed/absent new config falls back to
   defaults (silently), per the project's "config never fails" pattern.
10. The CLI subcommand offers the same handoff; output is JSON by default.
11. `docs/FEATURES.md` documents the feature, its `a` trigger and the CLI
    subcommand.
12. CI green: `Lint`, `Test` (100% diff coverage), and `Mutation`.

## Resolved decisions (checkpoint 1)

1. **Config harness list semantics** — additive to PATH detection; entries may
   declare `name` + binary/command for harnesses detection missed.
2. **Detection** — built-in name→binary table probed with PATH lookup.
3. **Prompt payload** — identifies the task and the concrete `tsk show`/`tsk
   move` commands plus the task project's valid statuses; the description is
   not embedded; exact composition pinned in `behavior.feature`.
4. **No handoff configured** — refuse with a clear message naming the config
   key to set (never a silent no-op).
5. **Zero-harness empty state** — overlay message exactly `No harnesses found`;
   config-declared entries are the fix path.
6. **Spawn cwd** — tsk's launch directory; optional config override; no
   per-project path.
7. **Key `a` availability** — verified free in every view; only the
   assignee/off-day modal binds it, a different context.
8. **CLI subcommand** — in scope (was scope OUT); designed alongside the TUI
   action sharing one core.
