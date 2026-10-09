# Plan: Ask AI — hand a task off to an external AI harness

- Slug: `ask-ai-handoff` · Type: feature · Branch: `feat/ask-ai-handoff`
- Behavior source: `behavior.feature` (18 scenarios) — the exact user-visible strings, prompt shape and config-driven flows live there.
- `adr_required: true` — reason: this change freezes a new permanent public config surface (`[[harness]]`, `[handoff]`) and commits to a launch mechanism with a real security/genericity tradeoff (shell template + prompt-file carriage + detached session) that is hard to reverse. Proposed ADR title: **`ask-ai-handoff-contract`** (to be authored as part of this change).

## Intended outcome

Pressing `a` on a focused task opens a picker of the AI harnesses available on this machine (installed best-known harnesses + config-declared entries) and hands the task's description to the chosen harness through a configurable, non-blocking launch. The same flow is available as a CLI subcommand for scripts and agents. Fire-and-forget: `tsk` never waits on, observes or tracks the harness.

## Approach at high level

- **Shared core (new `internal/harness` package)**: detection (built-in name→binary registry probed via PATH lookup), merge with config-declared entries, prompt composition, and launch. The TUI and the CLI are thin adapters over this core; neither imports the other, and `config`/`model` stay below it.
- **Config — the feature's public API**:
  - `[[harness]]` entries: `name` (required; display name and default binary) + optional `binary`. Additive to detection; declared entries are listed even when not on PATH (the fix path when detection misses something); a declared entry with the same display name replaces the detected one. Malformed/nameless entries are dropped (never fatal).
  - `[handoff]`: `command` (shell template; empty = handoff disabled) + optional `cwd` (default: the directory where `tsk` was launched).
  - Template placeholders: `{{harness}}`, `{{cwd}}`, `{{prompt_file}}` (required — refuse otherwise). Substituted values are shell-quoted by tsk so templates use them bare (e.g. `--cwd {{cwd}}`).
- **Prompt carriage**: the prompt text never enters the command line — it is written to a 0600 temp file that `{{prompt_file}}` points at. The file is not deleted by tsk (the harness may read it after tsk exits; the OS temp reaper owns it). Content is pinned in `behavior.feature`: `Task #<id>: <title> (<project>)` + `I need to implement this: <description>` (title fallback when the description is empty).
- **Handoff execution**: `/bin/sh -c <expanded template>` with working directory `handoff.cwd` (or launch dir), inherited environment, std streams to `/dev/null`, started in its own session (`Setsid`) and reaped in the background. Only spawn-time failures are surfaced (TUI error toast / CLI JSON error); after a successful start it is fire-and-forget.
- **TUI**: a new picker modal following the existing overlay pattern (open flag + priority key dispatch + centered modal render + overlay kind in the bottom bar). `a` is added to the List/Kanban keybind hints and handled in List/Kanban/Detail; refusal and launch errors use the existing transient toast; zero harnesses shows exactly `No harnesses found`; no focus means no-op. Never `tea.ExecProcess` — that would freeze the dashboard.
- **CLI**: `tsk ask <task-id> [--harness NAME]` — JSON output by default; auto-selects only when exactly one harness is available; every failure (unknown task, unknown/ambiguous harness, unconfigured handoff, zero harnesses, spawn error) exits non-zero with a JSON error listing what is available.
- **Docs**: `docs/FEATURES.md` entry (what + trigger + CLI + config keys) in the same change; README config section gains the new keys with an example.

## Key decisions (from the brainstorm/architect consolidation)

1. **Prompt via temp file, not inline argv** — task text is untrusted user content; file carriage removes the escaping/injection class entirely. Tradeoff accepted: tsk never deletes the file.
2. **Shell template (`sh -c`) with pre-quoted substitution** — real wrappers (herdr's multi-step pane flow, tmux) need shell composition; the only dynamic values reaching the shell are tsk-controlled (temp path, cwd, config-declared binary).
3. **Detection on demand, no caching** — PATH lookups are cheap; caching adds invalidation complexity and test flakiness.
4. **Config dedupe by display name (declared wins)** — simplest rule supporting both "add what detection missed" and "override a detected harness".
5. **Refusal on `a` press when `handoff.command` is unset** — never open a picker that can do nothing.
6. **CLI auto-picks only when unambiguous (exactly one harness)** — determinism for agents; otherwise an error listing the options.
7. **ADR required** — durable config contract + launch-mechanism tradeoff (see header).

## Risks / attention points

- **Coverage + mutation gates are the main CI risk**: keep the real spawn/exec code tiny and injectable (lookpath, spawn, temp-file seams); push everything testable into pure functions (detection merge, dedupe, ordering, template substitution, prompt composition). Plan explicit tests for every refusal/error branch.
- **Prompt-file lifetime**: tsk cannot know when the harness reads it → never delete early; document the tradeoff.
- **herdr integration is environment-dependent** (its CLI expects a herdr session): the concrete config example must be validated against the installed binary during the context pass; failure mode must be a surfaced spawn/exit error, not a hang.
- **Detached spawn details**: own session (`Setsid`), `/dev/null` streams, background reaping — avoids terminal interference, zombies and Ctrl+C propagation.
- **First list-typed config key in the repo**: decoding must stay non-fatal; invalid entries are dropped with the config's warning behavior.
- **Definition of done includes docs and the repo gates**: `Lint`, `Test` (100% diff coverage), `Mutation`, protected `main` (PR-only), `docs/FEATURES.md` updated, `make install` after the change (repo rule: the user runs the installed binary).

## Order (coarse)

1. Config schema + decode (never-fatal semantics) →
2. `internal/harness` core (registry/detect/merge/prompt/template/launch) →
3. CLI `ask` →
4. TUI picker, keybindings, refusal/empty/error states →
5. ADR `ask-ai-handoff-contract` + `docs/FEATURES.md` + README config example →
6. Full gates (`make check`, mutation) and `make install`.

## Progress
Phase: execution
| Scenario (behavior.feature) | Status | Commit |
| --- | --- | --- |
| Open the Ask AI picker on a focused task | ⬜ | |
| Only installed known harnesses are listed | ⬜ | |
| A config-declared harness is listed even when detection does not know it | ⬜ | |
| A config entry overrides a detected harness with the same name | ⬜ | |
| No harnesses available | ⬜ | |
| No task is focused | ⬜ | |
| Hand a task off to the selected harness | ⬜ | |
| Empty description falls back to the task title | ⬜ | |
| The handoff runs in tsk's launch directory by default | ⬜ | |
| The handoff working directory can be overridden | ⬜ | |
| Cancel the picker | ⬜ | |
| No handoff command configured | ⬜ | |
| Handoff command missing the prompt placeholder | ⬜ | |
| Hand a task off from the CLI | ✅ | |
| CLI auto-picks the only available harness | ✅ | |
| CLI refuses to guess between several harnesses | ✅ | |
| CLI reports unknown task or unknown harness | ✅ | |
| CLI refuses when no handoff is configured | ✅ | |
