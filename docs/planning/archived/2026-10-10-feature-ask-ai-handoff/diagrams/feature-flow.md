# Ask AI — Feature Flow

Derived from `behavior.feature` (18 scenarios). Two entry points (TUI key `a`,
CLI `tsk ask`) share one core: discovery → prompt composition → config-driven,
non-blocking handoff.

```mermaid
flowchart TD
    E1["TUI: press 'a' on a focused task<br/>(List / Kanban / Detail)"] --> NF{"no task<br/>focused?"}
    NF -->|yes| NOOP["no-op"]
    NF -->|no| GUARD
    E2["CLI: tsk ask &lt;id&gt; [--harness NAME]"] --> GUARD

    GUARD{"handoff.command set<br/>and contains {{prompt_file}}?"}
    GUARD -->|no| REFUSE["Refuse — never silent:<br/>message names handoff.command<br/>or the missing {{prompt_file}}<br/>(CLI: JSON error, exit 1)"]
    GUARD -->|yes| DISCOVER

    DISCOVER["Discovery: built-in registry &cap; PATH<br/>+ config [[harness]] (additive)<br/>dedupe by display name, declared wins<br/>sorted alphabetically"] --> ANY{"any<br/>harness?"}
    ANY -->|no| NONE["'No harnesses found'<br/>TUI: overlay message<br/>CLI: error, exit 1"]
    ANY -->|yes| SEL{"select"}

    SEL -->|"TUI: Esc"| CANCEL["close — nothing launched"]
    SEL -->|"TUI: Enter — or CLI: chosen / single auto-pick"| COMPOSE
    SEL -->|"CLI: several, no --harness"| AMBIG["error lists available names<br/>exit 1"]

    COMPOSE["Compose prompt &rarr; 0600 temp file:<br/>I need to implement this task: Task #id: title (project).<br/>Check it with `tsk show id`; update state with<br/>`tsk move id status`; valid statuses: project workflow."]
    COMPOSE --> EXEC["Expand template — {{harness}}, {{harness_binary}}, {{cwd}}, {{prompt_file}}<br/>substituted shell-quoted, run via /bin/sh -c<br/>Setsid &middot; /dev/null &middot; cwd = handoff.cwd or launch dir"]
    EXEC -->|"spawn error"| ERR["TUI: error toast<br/>CLI: JSON error, exit 1"]
    EXEC -->|"started"| DONE["Fire-and-forget:<br/>TUI stays responsive &middot; CLI prints JSON + exit 0 immediately"]
```
