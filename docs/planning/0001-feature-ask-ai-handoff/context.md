---
feature: 0001-feature-ask-ai-handoff
freshness: 572496dd6d48c64bd425d9b2ef8f748c407ce14c
codegraph: ready
generated_by: codebase-researcher
---

# Context: Ask AI — hand a task off to an external AI harness

## Scope
- In: `internal/harness` core (detect/merge/prompt/template/launch), config `[[harness]]` + `[handoff]`, TUI picker + `a` key, CLI `tsk ask`, docs/ADR.
- Out: harness-side tracking, per-task harness history, per-project paths, multi-harness fan-out, `tea.ExecProcess` (forbidden — it freezes the dashboard).

## Files to Touch
| Symbol / Area | File | Lines | Why |
|---------------|------|-------|-----|
| `Config` struct + `Harnesses []HarnessConfig` + `Handoff HandoffConfig` | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/config/config.go` | 24-30 | Add `Harnesses []HarnessConfig \`toml:"harness"\`` (first list-typed key in the repo) and `Handoff HandoffConfig \`toml:"handoff"\``; sub-structs `HarnessConfig{Name, Binary string}` / `HandoffConfig{Command, CWD string}` with toml tags `name`/`binary` and `command`/`cwd` |
| `Defaults()` | same | 43-55 | Harness list and handoff default to empty (zero values); no code change needed beyond the struct, but keep the never-fails shape |
| `Load()` | same | 73-96 | Add tolerant `[[harness]]` decode (see TOML finding below); `handoff` needs no clamping (empty = disabled) |
| `internal/harness/harness.go` (NEW) | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/harness/` | — | Core package: `Harness{Name,Binary}`, built-in name→binary registry, `Detect(cfg)` (lookPath seam + merge + dedupe by name, declared wins, sorted by name), `ComposePrompt(task)`, `ExpandTemplate(cmd, harness, cwd, promptFile)` (pure; refuses when `{{prompt_file}}` missing; shell-quotes values), `Launch(...)` detached spawn. Imports only `config`/`model` — they stay below it |
| `Run()` dispatch | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/cli/cli.go` | 22-89 | Add `case "ask":` before `default` |
| `cmdAsk` (new) | same | near 619 (`cmdShow` is the model) | Parse `<task-id>` + `--harness NAME` via `flagScanner` (117-138); `openDB()` + `database.GetTask(id)` (pattern of `cmdShow` 619-638); on any failure `outputError(...)` (106-110, JSON to stderr + exit 1); success → `outputJSON(map[string]any{"ok": true, ...})` |
| `cmdHelp` commands map + bash/zsh/fish completion lists | same | 1333-1362, 1251, 1286, 1293-1331 | Add `tsk ask <id> [--harness NAME]` to the help map and to all three `completion` command lists |
| `Model` fields `askAIOpen`, `askAIIdx`, `askAIHarnesses` | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/tui/app.go` | 15-130 (modal state blocks: filter at 105-109 is the nearest model) | Picker state; cache the detected list at open (detection on demand, no long-lived cache) |
| `handleKey` priority chain | same | 604-668 | Insert `if m.askAIOpen { return m.handleAskAIKey(key) }` **between `tagOpen` (628) and `detailOpen` (633)** — must precede `detailOpen` so the picker captures keys while the detail sits behind it; the assignee modal's own `a` (add off-day) is checked later (646) and keeps priority |
| `handleListKey` / `handleKanbanKey` / `handleDetailKey` | same | 707-790 / 792-898 / 914-994 | Add `case "a":` in each. Detail handler opens the picker like `t` opens tags (941-947). Guard: `selectedTask() == nil` → return `m, nil` (no task focused = no-op, behavior.feature L47-50) |
| `overlayKind()` | same | 1061-1085 | Add `case m.askAIOpen: return overlayAskAI` **in the same position as handleKey (between tag and detail)** |
| `View()` overlay chain | same | 1123-1157 | Add `if m.askAIOpen { content = m.renderAskAIModal(content) }` **after the detail/tag blocks** (later in the chain = painted on top) |
| ask-failure msg + `Update` case | same | msg structs 153-182; Update switch 469-585 | e.g. `askFailedMsg{err}` → `m.setToast(msg.err.Error(), "error")` (pattern of `taskCreateFailedMsg` 535-539) |
| `overlayAskAI` constant | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/tui/keybindsbar.go` | 14-26 | New enum member |
| `title()` + `renderOverlay()` | same | 86-110, 145-201 | Both switch exhaustively on `overlayKind` — add `overlayAskAI` cases (`" Keybinds · Ask AI "`; keys: `{"↑↓","move"},{"enter","handoff"},{"esc","close"}`) |
| `keybindsForView` List + Kanban tables | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/tui/keybindings.go` | 64-75, 77-87 | Add `{"a", "ask AI"}`. (Gantt intentionally excluded — plan scopes the action to List/Kanban/Detail) |
| `detailKeybinds()` | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/tui/help.go` | 11-22 | Add `{"a", "ask AI"}`; help modal picks it up automatically (renderHelpModal 28-91) |
| `askai_modal.go` (NEW) | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/internal/tui/askai_modal.go` | — | `openAskAI()`, `handleAskAIKey`, `renderAskAIModal`, `askLaunchCmd`. Model after `filter_modal.go` (picker) + `assignee_modal.go` (roster render) |
| `docs/FEATURES.md` | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/docs/FEATURES.md` | inventory 7-24, TUI keys 33-45, CLI 60-69, Config 78-86 | Add: inventory row, `a` in TUI keys, `tsk ask` in CLI commands, `[[harness]]`/`[handoff]` in Config |
| `README.md` config example | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/README.md` | 100-113 | Add `[handoff]` + `[[harness]]` to the TOML sample + one sentence. Note: the sample documents `[list] page_size` but the real toml tag is flat `list_page_size` (config.go 27) — pre-existing doc drift, don't copy the `[list]` table shape |
| ADR (NEW) | `/home/buble/dev/projects/tsk/.worktrees/tsk.feat-ask-ai-handoff/docs/adr/0001-ask-ai-handoff-contract.md` | — | No `docs/adr/` exists yet; this is ADR-0001. Title `ask-ai-handoff-contract` (plan.md header). Content: the frozen config surface + the launch-mechanism tradeoff (shell template + prompt-file carriage + detached session) |

## Contracts
- **Config never fails** — `internal/config/config.go:73-96`: missing file → defaults; malformed → `Defaults()`. New `[[harness]]` decode must drop bad entries, never fail the load (see TOML finding).
- **CLI JSON-by-default + `outputError`** — `internal/cli/cli.go:100-110`: success = `outputJSON` (indented, stdout); failure = `model.ErrorResult{Error: msg}` on stderr + `exit(1)`. Tests inject `stdout/stderr/exit` (94-98).
- **Pinned strings** (behavior.feature): `No handoff configured: set handoff.command in config.toml` (L98); overlay shows exactly `No harnesses found` (L44); missing-placeholder message must name `{{prompt_file}}` (L105, exact wording free); prompt body is exactly `I need to implement this task: Task #<id>: <title> (<project>). You can check the task with the tsk CLI. After finishing, update its state with the same tsk CLI.` (single line; the description is not embedded — the harness reads the task via the tsk CLI).
- **Placeholder semantics**: `{{harness}}` → selected harness **display Name** (the herdr `--kind` needs the name; pin in tests); `{{cwd}}` → `handoff.cwd` or tsk's launch dir; `{{prompt_file}}` → 0600 temp file path. Substituted values are **shell-quoted by tsk** so templates use them bare.
- **Prompt-file lifetime**: tsk never deletes the file (harness may read it after tsk exits).
- **`internal/` only, snake_case files, PascalCase types, English only** (AGENTS.md).
- **Detached spawn**: `/bin/sh -c <expanded>`, `Setsid`, nil streams (= `/dev/null`), `Start()` + background `Wait()` for reaping. Only spawn-time failures surface. **Never `tea.ExecProcess`** (comment.go:53, editor.go:48 are the blocking pattern — do not imitate for the handoff).
- **New feature must land in `docs/FEATURES.md` in the same change** (AGENTS.md).

## Pattern to Follow
- **Picker modal** → `internal/tui/filter_modal.go` (whole file): `filterMaxVisibleOptions` cap (L24), `visibleRange` windowing (L436), `handleFilterModalKey` (L343-401), `renderFilterModal` with `modalWidthFor`/`overlayModal` (L458-464). Render via `renderModalBox` (`modal.go:13`) + `overlayModal` (`modal.go:35`).
- **Roster render with selection prefix + `styleSelected`** → `internal/tui/assignee_modal.go:79-84, 218-255`.
- **Keybinding registration** → three places must agree: `keybindsForView` table (`keybindings.go:41`), `overlayKind()` mapping (`app.go:1061`), `keybindsbar` title+render (`keybindsbar.go:86,145`). Help modal derives from the first two.
- **Config decode** → `internal/config/config.go:73-96` + tests `internal/config/config_test.go` (`t.Setenv("TSK_CONFIG", path)` pattern, L40).
- **CLI subcommand** → `cmdShow` (`cli.go:619-638`): `parseID` → `openDB`/`defer closeDB` → `database.GetTask` → `outputError` or `outputJSON`. Flag parsing via `flagScanner` (117-138). Test harness: `captureOutput`/`run`/`runErr`/`withTempDB` (`internal/cli/cli_test.go:19-62, 268-290`).
- **Process spawn / test seam** → `internal/tui/comment.go:31-33` (`var createTemp = func(dir, pattern string) (tempFile, error)`), `tempFile` interface (61-65), `writeAndClose` (74-85). Mirror these seams in `internal/harness`: `var lookPath = exec.LookPath`, `var createTemp = os.CreateTemp`, injectable spawn — every error branch must be reachable in tests without launching a real process.
- **Async result in Update** → msg struct + `tea.Cmd` closure + `Update` case (app.go:535-539 `taskCreateFailedMsg` → `setToast(..., "error")`).
- **TUI white-box tests** → `newTestModel` (`app_test.go:14-35`), `press` (`app_test.go:72-110`, sends `tea.KeyPressMsg`), `pressKeys` (`keys_coverage_test.go:22-29`), `mustMsg`/`mustRun` with 250 ms ceiling (`cmd_test.go:18-78`), `ansi.Strip` for render assertions.

## Tests
- **Existing affected**:
  - `internal/tui/keybinds_test.go:15-53` (`TestKeybindsBarRenderOverlay`), `:57-72` (`TestEveryOverlayHasKeybinds`), `:111-137` (`overlayName`) — explicit overlay lists; add `overlayAskAI` or the new overlay's keybinds go untested.
  - `internal/tui/keys_coverage_test.go` — per-view key tables; add `a` coverage for List/Kanban/Detail + no-focus no-op.
  - `internal/tui/app_guards_test.go` — model the no-task guard after `TestListActionKeysNoopWithEmptyList` (L26-43).
  - `internal/config/config_test.go` — add `[[harness]]`/`[handoff]` decode tests (valid, bad entry dropped, nameless dropped, absent file).
  - `internal/cli/cli_test.go` — `TestRunHandlesKnownCommands` (80-103) for the happy path (needs a config with `[handoff]` — extend `withTempDB` or write a custom `TSK_CONFIG`); `runErr` for every refusal (unknown task, unknown/ambiguous harness, no handoff, missing placeholder).
  - `internal/tui/help_layout_test.go` — asserts box position/width only, **not** key lists → likely unaffected.
- **Framework / runner**: `make test` (go vet + `go test -race -count=1 ./...`); gates `make check`, `make coverage-check` (diff-coverage), `make mutate-diff` (mutation). TUI tests are direct model tests, no teatest/golden infra. Integration infra: in-memory SQLite via `db.NewTestDB()`; config via `TSK_CONFIG` env; **unit only** — no real processes (spawn is injected).
- **Suggested new test files**: `internal/harness/harness_test.go` (detect/merge/dedupe/sort, prompt composition incl. title fallback, template expansion incl. missing-placeholder refusal, shell quoting, launch via fake spawn seam asserting `Setsid`/dir/command), `internal/tui/askai_modal_test.go` (open/list/sort/empty-state `No harnesses found`/esc/enter-launch/no-handoff toast/missing-placeholder toast/no-focus no-op).
- **Mutation hotspots** (allowlist is EMPTY — every mutant must die): `ExpandTemplate` substitution + quoting, `Detect` merge/dedupe/sort, prompt fallback branch, every refusal branch, picker index clamp, `cmdAsk` flag parsing. Follow the repo's mutation-proof style: `min`/`max` arithmetic instead of if-chains (see `modalWidthFor` `modal.go:28`, `listWindow` `app.go:388-406`), named operations instead of `++` (`jumps` `toast.go:56`), extracted pure functions (`goesToNewTaskTextarea` `app.go:600`), no hand-advanced loops (`flagScanner` comment `cli.go:112-117`).

## Integration Points (non-obvious)
- **TOML v1.6.0 decode behavior (verified with a scratch program against the module cache)**: a typed `[]HarnessConfig` decode with one bad entry (e.g. `name = 5`) returns an error **and** still partially populates the slice — including the bad entry with an empty `Name`. So (a) the existing `Load()` pattern (`err != nil → return Defaults()`) would discard the *entire* config over one bad harness entry, and (b) nameless entries survive even successful decodes. **Pattern**: decode the file a second time into `map[string]any` (raw decode succeeds even when the typed decode fails — verified), take `harness` as `[]map[string]any`, per-entry `toml.Marshal` + `toml.Decode` into the typed struct (verified working), drop entries that fail or have `name == ""`. Keep the existing scalar decode untouched.
- **handleKey priority vs detail modal**: the picker opens *from* the detail, so `askAIOpen` must be checked **before** `detailOpen` in `handleKey` (app.go:604-668) — otherwise the detail captures the picker's keys. The assignee modal's `a` (assignee_modal.go:101,127) stays safe because `assigneeModalOpen` is checked earlier in the chain and the Dashboard has no focused task.
- **Launch dir**: "tsk's launch directory" = the process's `os.Getwd()` at startup. The core `Launch` must take it as an explicit parameter (testable); TUI and CLI pass their cwd. `config.Path()`/XDG (config.go:58-70) is unrelated to the handoff cwd — `handoff.cwd` is a plain directory string, no XDG resolution.
- **Who reaps the child**: tsk itself, via `go cmd.Wait()` after `Start()` (background reap, no zombies, no terminal interference, no Ctrl+C propagation thanks to `Setsid`).
- **`{{harness}}` value**: use the harness display **Name**, not the Binary — the picker lists names and wrapper tools (herdr `--kind`) key on names. Pin in tests.
- **Gantt**: `a` intentionally not bound there (plan scopes List/Kanban/Detail); `selectedTask()` supports Gantt but no handler is required.

## Example handoff command (herdr, TO-VERIFY-BY-USER)
herdr 0.9.2-preview is installed. Verified CLI surface: `herdr pane split --current --direction down --cwd <PATH>` (options: `--direction right|down`, `--ratio`, `--cwd`), `herdr agent start <NAME> --kind <KIND> --pane <ID>` (kinds include `pi, claude, codex, opencode, …`; "the pane must be at its interactive shell prompt"), `herdr agent prompt <TARGET> <TEXT>`, `herdr pane current|list|run|send-text`. Plausible config for this user:

```toml
[[harness]]
name = "opencode"
binary = "opencode"

[handoff]
command = "herdr pane split --current --direction down --cwd {{cwd}} && herdr agent start {{harness}} --kind {{harness}} --pane $(herdr pane current) && herdr agent prompt {{harness}} \"$(cat {{prompt_file}})\""
```

Unverified: whether `herdr pane current` prints a bare pane ID, and whether the split pane lands at an interactive shell prompt ready for `agent start`. Failure mode must be a surfaced spawn/exit error (toast / JSON error), never a hang. herdr is never referenced in tsk code — only in the user's config. Built-in registry names should overlap herdr's kind list (`opencode`, `pi`, `claude`, `codex`, …) since both are just name→binary pairs.

## Risks / Assumptions
- **Coverage + mutation gates are the main risk**: 100% diff coverage (`scripts/diff-coverage.sh`, floor 100.00) and zero survivors (`.mutation-allowlist` is empty; `MUTATE_EXCLUDE=cmd/` so `internal/harness` **is** mutated). Keep the real spawn path tiny and injected; push everything into pure functions with explicit tests per branch.
- **Spawn testability**: no test may reach the real `exec.Command` — inject lookPath/createTemp/spawn seams or every error branch is uncovered (and a surviving mutant is a red CI).
- **`/bin/sh` dependency**: the template contract is a POSIX shell command; document it in the ADR.
- **UTF-8/locale**: prompt content is user text (non-ASCII possible); shell quoting must be byte-safe; render code elsewhere truncates by runes (`truncateLabel` cli.go:983, `padRight` 998) — reuse that care in the picker.
- **Config double-decode cost**: decoding the file twice (typed + raw) is trivial (config files are small) and only happens at load.
- **Docs drift**: README's `[list]` table vs flat `list_page_size` toml tags — pre-existing; don't propagate the wrong shape for the new keys.
- **Definition of done**: `make check` green, `make mutate-diff` green, `docs/FEATURES.md` + README + ADR in the same change, then `make install` (repo rule: the user runs `~/.local/bin/tsk`).
