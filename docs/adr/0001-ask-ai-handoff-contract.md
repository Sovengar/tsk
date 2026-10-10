# ADR-0001: Ask AI handoff contract

- Status: accepted
- Date: 2026-10-09
- Feature: `ask-ai-handoff` (`docs/planning/0001-feature-ask-ai-handoff`)

## Context

`tsk` is the single place a developer keeps their work. Handing a task to an
external AI coding harness (opencode, pi, codex, claude, …) still means copying
the task by hand and switching terminal. We added an `Ask AI` action (`a`) in the
TUI and a `tsk ask` subcommand that collapse that into one step.

The launch mechanism has to be configurable: the harnesses differ, and this
user's instance opens a new **herdr** pane, but herdr must never be a hardcoded
dependency. Two things are hard to reverse once shipped: the **public config
surface** and the **launch mechanism** (how user text reaches a shell). This ADR
freezes both.

## Decision

### Config surface (additive, never fatal)

```toml
[[harness]]
name = "opencode"     # display name and default binary
binary = "opencode"   # optional; the executable to look up

[handoff]
command = "… {{harness}} … {{harness_binary}} … {{cwd}} … {{prompt_file}} …"
cwd = "/optional/override"  # default: the directory where tsk was launched
```

- `[[harness]]` entries are **additive** to PATH detection: they are listed even
  when detection does not know them (the fix path when detection misses
  something). A declared entry with the same display name **replaces** the
  detected one.
- Detection is a built-in name→binary registry probed with `exec.LookPath`, on
  demand and uncached.
- `handoff.command` empty means the handoff is disabled; the action refuses
  with the pinned message `No handoff configured: set handoff.command in config.toml`.
- Config is **never fatal**: the `[[harness]]` list is decoded entry by entry
  from the raw TOML tree, so a single malformed entry is dropped instead of
  discarding the whole file (BurntSushi's typed slice decode returns an error on
  a bad entry, and the previous `Load()` fell back to `Defaults()`).

### Launch mechanism

- The command is a **POSIX shell template** executed as `/bin/sh -c <expanded>`.
  Wrappers like herdr need shell composition (split a pane, start an agent with
  its prompt prefilled), which a plain argv list cannot express.
- Placeholders: `{{harness}}` → the selected **display name** (wrappers key on
  it, e.g. herdr `--kind`), `{{harness_binary}}` → the executable to run (so a
  config `binary` override is usable), `{{cwd}}` → the resolved working
  directory, `{{prompt_file}}` → the prompt file path. `{{prompt_file}}` is
  **required**; a template without it is refused naming the missing placeholder.
- Substituted values are **shell-quoted by tsk** (single quotes, embedded single
  quotes escaped), so templates use them bare (`--cwd {{cwd}}`). The only
  dynamic values reaching the shell are tsk-controlled: the temp path, the
  working directory and the config-declared binary name.
- The task text **never enters the command line**. The prompt is written to a
  `0600` temp file that `{{prompt_file}}` points at, with content:

  ```
  I need to implement this task: Task #<id>: <title> (<project>). Check it with `tsk show <id>`. Update its state as you go with `tsk move <id> <status>`; valid statuses: <statuses>.
  ```

  where `<statuses>` is the task project's workflow (comma+space separated); an
  empty workflow drops the state clause rather than leaving a dangling "valid
  statuses:". This removes the escaping/injection class entirely. The description
  is **not** embedded: the harness reads the current task (and comments/status)
  from the `tsk` CLI and writes the state back through it, so the prompt stays a
  pointer rather than a snapshot. Tradeoff: after a successful launch **tsk never
  deletes the file** — the harness may read it after tsk exits; the OS temp
  reaper owns it. A failed launch (spawn error / bad cwd) removes it, since
  nothing will read it.
- The process is started in its own session (`Setsid`) with `/dev/null` streams
  and reaped in the background (`go cmd.Wait()`). It does not freeze the TUI
  (never `tea.ExecProcess`), does not receive Ctrl+C and leaves no zombie. Only
  spawn-time failures surface (toast / CLI JSON error); after a successful start
  it is fire-and-forget — `tsk` never observes the harness.

## Consequences

- Swapping the command (herdr, tmux, plain detached exec) works with **no code
  change**; herdr is referenced only in the user's config.
- The contract is POSIX-shell specific (`/bin/sh`); documented here.
- Because the prompt file outlives the process, secret material in a task
  description should be treated as written to the OS temp directory.
- CLI `tsk ask` auto-selects a harness only when exactly one is available; with
  several it errors listing the options, so an agent always knows which one ran.

## Example (this user's herdr instance)

```toml
[[harness]]
name = "opencode"
binary = "opencode"

[handoff]
command = "pane=$(herdr pane split --current --direction down --cwd {{cwd}} | jq -r '.result.pane.pane_id') && herdr agent start {{harness}}-$$ --kind {{harness}} --pane \"$pane\" -- --prompt \"$(cat {{prompt_file}})\""
```

Verified against `herdr 0.9.2-preview`: the pane id must be captured from the
`pane split` result (`| jq -r '.result.pane.pane_id'`) — `herdr pane current`
returns a JSON object, not a bare id, so passing it to `--pane` fails while the
split has already happened. The prompt rides on `agent start … -- --prompt
"<text>"`, which **pre-loads** it into the harness input without submitting, so
the user can adjust the agent before sending it; a separate `agent prompt` step
would auto-submit and is not used. Agent names must be unique (`{{harness}}-$$`,
`$$` = the handoff shell's PID). The split pane must land at an interactive shell
prompt ready for `agent start`; if it does not, the failure is a surfaced
spawn/exit error, never a hang.
