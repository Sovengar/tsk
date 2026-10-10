# Ask AI — Planning Process Flow (run `ask-ai-handoff`)

Feature-aware record of this planning run: real checkpoints, the decisions taken
for this change, and the gates waiting at the end.

```mermaid
flowchart TD
    O["Orchestrator: worktree + branch feat/ask-ai-handoff<br/>planning_dir = docs/planning/0001-feature-ask-ai-handoff"] --> IDX
    IDX["Index: Engram codebase-index/tsk built (obs 2998)<br/>worktree codegraph synced — codegraph: ready"] --> ISS
    ISS["idea-refiner &rarr; issue.md"] --> C1
    C1["CHECKPOINT 1 — approved<br/>CLI subcommand moved INTO scope<br/>8 decisions: additive config list, PATH probe,<br/>prompt framing, refuse w/o handoff, 'No harnesses found',<br/>cwd = launch dir, key 'a' free, CLI designed"] --> BR
    BR["brainstormer + architect (parallel)"] --> BEH
    BEH["behavior.feature — 18 scenarios"] --> C2
    C2["CHECKPOINT 2 — approved, no changes"] --> PLAN
    PLAN["plan.md — internal/harness core, frozen config API<br/>sh -c template + prompt-file carriage, detached spawn<br/>adr_required: true"] --> CTX
    CTX["codebase-researcher &rarr; context.md<br/>verified: TOML partial-decode hazard, handleKey ordering,<br/>herdr 0.9.2 syntax, empty mutation allowlist"] --> DIA
    DIA["diagrams (this file + feature-flow.md)"] --> C3
    C3["CHECKPOINT 3 — plan approval"] --> EXEC
    EXEC["swe-executor: TDD from behavior.feature"] --> GATES
    GATES["Gates: make check (lint + test + 100% diff coverage)<br/>mutation (empty allowlist) &middot; docs/FEATURES.md + README<br/>ADR ask-ai-handoff-contract &middot; PR to protected main"] --> INST
    INST["make install — user runs ~/.local/bin/tsk"]

    PLAN -.->|flagged risks| RISK["Coverage/mutation hotspots in exec paths<br/>prompt-file lifetime (never deleted)<br/>herdr needs a herdr session (env-dependent)<br/>never tea.ExecProcess — no TUI freeze"]
    PLAN -.->|scope cuts| CUT["No harness-side tracking/history<br/>No per-project path &middot; Gantt excluded<br/>No CLI --list (errors list harnesses)"]
```

Note: `RISK` and `CUT` are annotations, not pipeline stages — they were flagged
during planning and must survive into implementation.
