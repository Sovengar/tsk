# Plan: Shared AI request contract (tsk pilot)

- Slug: `ai-request` · Type: refactor + feature foundation · Status: pilot
- Pilot repo: `tsk` · Shared doc: this file
- Related: `docs/adr/0001-ask-ai-handoff-contract.md`, `github.com/sovengar/ai`, `internal/harness`, `internal/tui/askai_modal.go`

## Goal

One definition of an AI invocation shared by every app: **agent + model + effort + prompt**.
The execution mode is separate and never changes the request shape.

```
Request { agent, model, effort, prompt }  ->  mode { assist, run, handoff, attach }
```

`handoff` is only one mode. It is not the umbrella concept.

## Modes

| Mode | Control | AI lives | Example | Output |
|---|---|---|---|---|
| **Assist** | User | In-process provider/API | dbx `Ask` | Inline result |
| **Run** (oneshot) | App | Headless subprocess | vroom, gitdash conflict | stdout/exit code in TUI |
| **Handoff** | Agent | External harness UI | tsk `Ask AI` | User continues there |
| **Attach** | User | External pane | prdash review agent | Adjacent pane |

## Concerns (keep them separate)

| Concern | Question | Shared abstraction | Owner |
|---|---|---|---|
| **Catalog** | Which agents exist on this machine? | `Detect(declared, look)` + built-in registry | `ai` |
| **Request** | What does the user want? | `Request{agent, model, effort, prompt, cwd}` | `ai` |
| **Invocation** | How is the agent called? | `Capabilities`, `ModelArgs`, `HeadlessArgs` | `ai` |
| **Model source** | Where do models/efforts come from? | `ModelSource` (local catalog, provider API, static) | `ai` + app |
| **Presentation** | Where does the delegation appear? | `Presenter` (overlay, detached, pane, direct API) | app adapter |
| **TUI overlay** | How does the user choose? | Picker component | each TUI |

Rules:

- Pane display (herdr, tmux, tuios) is **presentation**, not invocation.
- A direct API call (dbx) is an **invoker**, not a presenter.
- `run` / `-p` / `exec` argument logic is **invocation** and stays shared.
- The overlay stays in the TUI; it reads the catalog, asks the model source and calls the invoker.
- Model lists are never hardcoded as the only source.

## Shared vs adapter

The concerns are mostly the same across apps. Only two are genuinely app-specific.

| Concern | tsk | vroom | gitdash | dbx | Shared? |
|---|---|---|---|---|---|
| **Catalog** | config + PATH | config + PATH | config + PATH | provider registry | Same shape, different source |
| **Request** | task | service | repo | schema/table | 100% identical |
| **Invocation** | CLI | CLI | CLI | HTTP API | CLI shared; API is a separate invoker |
| **ModelSource** | local catalogs | local catalogs | local catalogs | provider `/models` | Same interface, different source |
| **Presenter** | detached | console | overlay + run | inline | App-specific |

Rule of thumb: four of the six concerns are shared or share an interface; only the presenter is always local. That is what justifies a shared Go package (and later a module) instead of copy-paste adapters.

`Agent` may later need a `Kind` (`cli` or `api`) so the invoker knows whether to build argv or call HTTP. The tsk pilot does not need it yet; add it when dbx joins.

## Direct API vs headless harness

Both eventually call a model API. The difference is who owns the agent loop and the tools.

| Criterion | Direct API (dbx) | Headless harness (`-p`, `exec`, `run`) |
|---|---|---|
| Agent loop | You build it | The harness owns it |
| Tools | You implement schema + execution | Built in (files, shell, MCP, skills) |
| Output | Structured, predictable | Text or JSON from a process |
| Control | Full | Limited to flags and prompt |
| Startup | Fast | Process spawn + harness boot |
| Cost | One call | Multi-turn, more tokens |
| Portability | One HTTP client | Depends on local install/version |
| Risk | App enforces permissions | Agent can edit/run commands |
| Model list | Provider `/models` | Local catalog + capability flags |
| Best for | Bounded transformation | Multi-step work with tools |

Decision rule, per **action**, not per app:

- One bounded transformation with structured output → direct API. Example: dbx NL→SQL.
- Multi-step task that must read/write files or run commands → headless harness. Example: gitdash conflict, vroom "fix build".
- Hybrid is normal: an app may use API for classification and harness for actions.

Consequence: `Invoker` is a strategy, not an app identity. dbx keeps HTTP. gitdash/vroom use argv. tsk keeps handoff.

### Invokers (implemented in the pilot)

```go
type Invoker interface {
    Invoke(ctx context.Context, r Request) (Result, error)
}

type Result struct {
    Output string
    Model  string
    Usage  Usage // input/output tokens; a local process leaves it zero
}

type ProcessInvoker struct{ Run ProcessRunner }

type APIInvoker struct {
    Endpoint, APIKey, Model, System string
    Headers   map[string]string
    MaxTokens int
    HTTPClient *http.Client
}
```

- `ProcessInvoker` builds the argv with `HeadlessArgs`, runs it without a shell, captures stdout, treats stderr as diagnostics, and passes `Cwd`.
- `APIInvoker` is the OpenAI-compatible chat completions transport, ported from dbx `internal/ai/nl2sql/compatible.go`. The request model overrides the configured model; usage is reported when the endpoint returns it.
- Anthropic's messages transport is not ported yet; add it as a second API invoker when dbx needs it.
- Both are injectable seams: `ProcessRunner` and `HTTPClient` keep tests deterministic.

## Deployment models (why vendor apps are not a template)

Vendor AI apps and local apps answer the same concerns, but they own different parts.

| Type | Who owns the runtime | Catalog source | Model source | Invocation | Example |
|---|---|---|---|---|---|
| Vendor cloud agent | Vendor VM/sandbox | Account entitlements | Curated registry, brokered server-side | Internal API | Cursor Cloud, Copilot coding agent, Codex Cloud |
| Agent marketplace | Vendor orchestration | Installed agents | Each agent's own | Platform API | Linear agents |
| Local CLI orchestrator | User's machine | PATH + config | Local agent catalog | argv (`run`, `-p`, `exec`) | tsk, vroom, gitdash |
| Local API client | User's machine | Provider config | Provider `/models` + local cache | HTTP | dbx |
| Gateway (self-hosted or vendor) | Gateway host | Gateway model registry | Gateway `/models` | HTTP | Atuin AI |
| Editor + ACP | Local or remote agent | Agent registry | Agent's own | JSON-RPC | Zed-style editors |

### Atuin AI (gateway pattern)

Atuin is a better example of the API-gateway shape than the vendor cloud apps:

- The client (`atuin` shell UI) talks to one `[ai].endpoint`.
- The endpoint is either Atuin Hub or a self-hosted `atuin-ai-server` (open source, OpenAI-compatible).
- The server owns the model registry: `default_model`, `[[models]]` aliases, provider endpoint, API key, tools.
- The client selects a model with `[ai].model` or `/model`; the list comes from the server, not from local files.
- The transport is HTTP chat completions, not `exec`/`argv`.
- Self-hosting supports Ollama, vLLM, LM Studio, llama.cpp, LiteLLM, OpenRouter.

So Atuin is the "provider/model served by an API" pattern. tsk/vroom/gitdash are the opposite: local catalog plus local process invocation. dbx sits in between: local client, remote provider API.

Consequences for this plan:

- Our apps **do not provision a model server**. They are clients/orchestrators on the user's machine.
- The catalog must be environment-derived (PATH, config, local auth files), never an account entitlement list.
- The model source must be pluggable: local catalog, provider API, static fallback, free text.
- Invocation must support both surfaces: local process argv and HTTP API.
- Vendor apps can assume a curated model list because they broker it. We cannot.
- dbx is a local API client, not a hosted agent: same `Request`, different invoker and model source.

## Layers

| Layer | Responsibility | Location |
|---|---|---|
| **Core** | catalog, request, capabilities, model sources, args | Go package `ai` |
| **Invoker** | API call, headless argv, detached command, pane command | app adapter + `ai` args |
| **Presenter** | overlay, detached process, pane, inline result | app adapter |
| **UI** | overlay: prompt, agent, model, effort | each TUI |

The core must not spawn processes, read app config, render UI or know about herdr/tmux/tuios.

## Core contract

```go
package ai

type Agent struct {
    Name   string
    Binary string
}

type Request struct {
    Agent  Agent
    Model  string
    Effort string
    Prompt string
    Cwd    string
}

type Options struct {
    Models  []string
    Efforts []string
}

type Capabilities struct {
    ModelFlag  []string // flag template; {value} is replaced
    EffortFlag []string // flag template; {value} is replaced
    Headless   []string // argv prefix; prompt is appended last
}

func Detect(declared []Agent, look func(string) (string, error)) []Agent
func OptionsFor(agent string) Options
func ModelArgs(agent Agent, model, effort string) string   // shell-quoted
func HeadlessArgs(r Request) ([]string, error)             // raw argv
```

`ModelArgs` serves handoff templates. `HeadlessArgs` serves run mode.
Both derive from the same capability table.

Planned seams, added when the second app needs them (not in the pilot):

```go
// ModelSource answers where the picker reads models and efforts.
type ModelSource interface {
    Options(ctx context.Context, agent Agent) (Options, error)
}

// Presenter answers where the delegation appears.
type Presenter interface {
    Present(ctx context.Context, r Request, invocation Invocation) error
}
```

Implementations of `ModelSource`: local agent catalog (tsk, vroom, gitdash), provider `/models` API (dbx), static/config fallback, composite with cache TTL.
Implementations of `Presenter`: TUI overlay, detached process, herdr/tmux/tuios pane, direct inline result.

## Capability table

| Agent | Model | Effort | Headless |
|---|---|---|---|
| `pi` | `--model` | `--thinking` | `pi -p` |
| `codex` | `--model` | `-c model_reasoning_effort=` | `codex exec` |
| `opencode` | `--model` | `--variant` | `opencode run` |
| `hermes` | `-m` | `--reasoning` | `hermes chat -q` |
| `jcode` | `-m` | unsupported | `jcode run` |

Unsupported effort omits the flag. Never invent one.

## Prompt delivery

The prompt is the same text in every mode. Delivery is a harness capability:

- `argv`: prompt appended as one argument (vroom, headless).
- `prompt_file`: 0600 temp file, `{{prompt_file}}` placeholder (tsk handoff).
- `stdin`: reserved; not needed by the pilot.

The app quotes values. The prompt never enters a shell unquoted.

## Config contract

> Superseded: a single handoff command is now the target. See
> [`remaining.md`](remaining.md). The per-harness `command` below is the old
> shape and must not be extended.

Per-harness config stays the same shape as tsk today:

```toml
[[harness]]
name = "pi"
binary = "pi"

[harness.handoff]
command = "… {{prompt_file}} … {{model_args}} …"
model = "free"          # optional default
thinking = "medium"     # optional default; "effort" is the shared term
```

- `command` empty disables the mode.
- `{{prompt_file}}` is required for handoff.
- `{{model_args}}` is produced by the core.
- Config decode stays per app and never fatal.

## Presentation

- Headless `run`: app builds `HeadlessArgs`, captures stdout/stderr, shows output in the TUI.
- Handoff: app expands the command template and starts it detached or attached.
- Attach: pane/wrapper integration (herdr, tmux, tuios) — presentation only.
- Direct API: dbx calls the provider in-process; that is an invoker, not a presenter.
- The shared module ships `scripts/mux`, a presenter dispatcher for tmux, herdr, tuios, kitty and wezterm. It receives already-expanded, quoted values and only decides where the command appears. Apps consume it from their config command (for tsk, `[harness.handoff].command`), so a user can replace it with any script.
- The script supports `pane`, `tab`, `popup`/`overlay` and `--dry-run`. Herdr overlay is a CLI split + zoom; a true herdr popup is config-file-only (`[[keys.command]] type = "popup"`). Wezterm has no floating popup and degrades to a pane.

No new binary: the core is the `github.com/sovengar/ai` Go module, and the presenter is a script in that module.

## Model sources

| Source | Used by | Rule |
|---|---|---|
| Local agent catalogs | tsk, vroom, gitdash | `pi`, `codex`, `opencode` JSON files; missing/malformed returns empty |
| Provider `/models` API | dbx (anthropic, openai, deepseek, qwen) | cache with TTL; manual refresh |
| Gateway `/v1/models` | optional for any app | one endpoint, server-owned registry (Atuin, LiteLLM) |
| Config/static list | all | fallback and free-text entry |

The picker must always allow a value outside the fetched list, so a stale catalog never blocks a handoff.

## App roadmap

1. **tsk (pilot)**: extract `github.com/sovengar/ai`; `internal/harness` delegates. No behavior change.
2. **vroom**: add `run` mode per agent; reuse `logrun.Run` + `consoleTailCmd`.
3. **gitdash**: conflict overlay; `outcomeConflict` hook; `run` in background; diff + confirm `add`/`--continue`.
4. **dbx**: keep `assist`; add model/effort selector to the existing `Ask` overlay; options from local catalogs or `/models`; persist last choice.
5. **prdash**: attach mode only.
6. **Shared script** in the module: `scripts/mux`.

## tsk pilot scope

In scope:

- New `github.com/sovengar/ai` module, split by concern: `request.go` (types), `catalog.go` (agent discovery), `invoke.go` (capabilities/argv), `models.go` (model sources), `invoker.go` + `invoker_process.go` + `invoker_api.go` (execution strategies).
- `scripts/mux` (presenter) and its fake-backend test; no herdr plugin.
- App config example consuming `mux` (tsk `[harness.handoff].command`).
- Move model/effort catalogs from `internal/harness/models.go`.
- `internal/harness` keeps its public API and delegates: `Harness = ai.Agent`, `ModelOptions = ai.Options`, `Options`, `ModelArgs`, `Detect`.
- Tests for every new branch.
- tsk consumes the module with `require github.com/sovengar/ai` + a local `replace` until the repo is published.
- No config change, no keybinding change, no user-visible behavior change.

Out of scope for the pilot:

- New execution modes in tsk.
- Config migration.
- Docs/FEATURES change (no user-visible change).
- Publishing the module or switching tsk to a versioned `require` (the `replace` stays for now).

## Decisions

1. **Go package, not a script for the core.** All apps are Go; the request/capability logic needs types and tests.
2. **Script only for launch wrapping.** The herdr command is configuration, not core.
3. **No new binary.** A binary cannot render an in-TUI overlay and adds a dependency.
4. **Mode is orthogonal to request.** `Request` has no `mode` field.
5. **Effort is optional per harness.** Capabilities are data, not assumptions.
6. **tsk first.** It already has the reference implementation; extracting it proves the contract without a rewrite.
7. **Module now.** The core lives at `github.com/sovengar/ai`; apps consume it with a versioned require (local `replace` during development).

## Risks

- **Coverage gate**: moved code must keep 100% diff coverage. Add table tests for each capability and catalog branch.
- **Alias drift**: `type Harness = ai.Agent` keeps callers source-compatible; do not fork the struct.
- **Catalog formats**: local JSON files are undocumented and can change; a missing or malformed file returns empty options.
- **Rename**: `thinking` remains the tsk config/CLI word; `effort` is the shared term. Document the mapping.
- **No behavior change**: the pilot must not alter prompt text, launch order or refusal messages.

## Order

1. Write this plan.
2. Add the `github.com/sovengar/ai` module with tests.
3. Refactor `internal/harness` to delegate to the module.
4. Run `make test` (and `make lint` if quick).
5. `make install` (repo rule: the user runs the installed binary).
6. Update task #92.

## Verification

- `go test ./...`
- `make lint`
- `make install`
- Manual: `tsk ask <id>` still launches the configured handoff; picker still lists the same harnesses and options.
