# Summary: ask-ai-handoff

## Metadata
- **Completed:** 2026-10-10 14:33
- **Duration:** 1249 minutes
- **Plan Number:** 0001

## Scenarios
| Scenario (behavior.feature) | Status |
|-----------------------------|--------|
| Open the Ask AI picker on a focused task | ✅ Passed |
| Only installed known harnesses are listed | ✅ Passed |
| A config-declared harness is listed even when detection does not know it | ✅ Passed |
| A config entry overrides a detected harness with the same name | ✅ Passed |
| No harnesses available | ✅ Passed |
| No task is focused | ✅ Passed |
| Hand a task off to the selected harness | ✅ Passed |
| The handoff runs in tsk's launch directory by default | ✅ Passed |
| The handoff working directory can be overridden | ✅ Passed |
| Cancel the picker | ✅ Passed |
| No handoff command configured | ✅ Passed |
| Handoff command missing the prompt placeholder | ✅ Passed |
| Hand a task off from the CLI | ✅ Passed |
| CLI auto-picks the only available harness | ✅ Passed |
| CLI refuses to guess between several harnesses | ✅ Passed |
| CLI reports unknown task or unknown harness | ✅ Passed |
| CLI refuses when no handoff is configured | ✅ Passed |
| Hand a task off to the selected harness (tsk CLI commands + statuses) | ✅ Passed |
| A project with an empty workflow drops the status clause | ✅ Passed |

## Commits
- docs(planning): update progress tracking throughout the feature
- chore: add ask-ai-handoff plan
- feat(config): add additive [[harness]] list and [handoff] section
- feat(harness): detection, prompt composition and detached handoff core
- feat(cli): add tsk ask to hand a task off to a harness
- feat(tui): ask AI picker on a focused task with the a key
- docs(ask-ai): ADR-0001, features inventory and README config example
- fix(docs): correct the herdr handoff example
- docs(adr): note the jq prerequisite in the herdr example
- docs(reviews): add code review reports

## Files
- Created: `internal/harness/harness.go`, `internal/harness/harness_test.go`,
  `internal/tui/askai_modal.go`, `internal/tui/askai_modal_test.go`,
  `docs/adr/0001-ask-ai-handoff-contract.md`,
  `docs/planning/0001-feature-ask-ai-handoff/*`,
  `.code-reviews/review-*.md`
- Modified: `internal/cli/cli.go`, `internal/cli/cli_test.go`,
  `internal/config/config.go`, `internal/config/config_test.go`,
  `internal/tui/app.go`, `internal/tui/app_guards_test.go`, `internal/tui/help.go`,
  `internal/tui/keybindings.go`, `internal/tui/keybinds_test.go`,
  `internal/tui/keybindsbar.go`, `README.md`, `docs/FEATURES.md`

## Tests
- Added: 85 test functions (harness 29, cli 24, config 9, tui picker 23)
- System Tests: ✅ Passed (`make test`: vet + `go test -race -count=1 ./...`)

## Documentation
- Changelog: ✅ Updated
- Docs: `docs/FEATURES.md`, `README.md` config example, `docs/adr/0001-ask-ai-handoff-contract.md`
- ADR: ✅ Created

## Code Review Issues
- Critical Found: 0
- High Found: 0
- User Decision: Approved to continue (0 CRITICAL / 0 HIGH / 0 MEDIUM after re-review)

## Next Step
Integrate `feat/ask-ai-handoff` into `main` (merge approval pending), then
`make install` per the repo rule (the user runs the installed binary).
