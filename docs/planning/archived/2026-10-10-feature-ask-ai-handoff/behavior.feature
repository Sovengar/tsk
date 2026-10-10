Feature: Ask AI — hand a task off to an external AI harness

  Pressing "a" on a focused task opens a picker of the AI harnesses available on
  this machine (best-known harnesses actually installed, plus any declared in the
  config file) and hands the task over to the selected harness through a
  configurable, non-blocking handoff command. A CLI subcommand offers the same
  flow for scripts and agents.

  # ---------------------------------------------------------------------------
  # Opening the picker
  # ---------------------------------------------------------------------------

  Scenario: Open the Ask AI picker on a focused task
    Given a task is focused in the List, Kanban or Detail view
    When the user presses "a"
    Then the Ask AI overlay opens on top of the current view
    And it lists the installed known harnesses together with the config-declared ones
    And the list is sorted alphabetically by display name

  Scenario: Only installed known harnesses are listed
    Given the built-in registry knows "opencode" and "claude"
    And "opencode" is available on PATH but "claude" is not
    And the config declares no extra harness
    When the user presses "a" on a focused task
    Then "opencode" appears in the picker
    And "claude" does not appear

  Scenario: A config-declared harness is listed even when detection does not know it
    Given the config declares a harness named "my-tool" that is not in the built-in registry
    When the user presses "a" on a focused task
    Then "my-tool" appears in the picker

  Scenario: A config entry overrides a detected harness with the same name
    Given the config declares a harness named "opencode" with a custom binary
    And the built-in registry also detects "opencode" on PATH
    When the user presses "a" on a focused task
    Then exactly one "opencode" entry appears in the picker
    And it carries the custom binary declared in the config

  Scenario: No harnesses available
    Given no known harness is installed and the config declares none
    And a handoff command is configured
    When the user presses "a" on a focused task
    Then the overlay opens and shows exactly "No harnesses found"
    And it cannot launch any handoff

  Scenario: No task is focused
    Given no task is focused
    When the user presses "a"
    Then nothing happens: no overlay opens and no process is launched

  # ---------------------------------------------------------------------------
  # Handing the task over
  # ---------------------------------------------------------------------------

  Scenario: Hand a task off to the selected harness
    Given a handoff command is configured
    And at least one harness is available
    And a task "#7 Add dark mode" in project "tsk" whose workflow is the default one is focused
    When the user selects a harness in the picker
    Then the picker closes and the handoff command is launched detached, without blocking the TUI
    And the template placeholder "{{harness}}" is replaced by the selected harness's display name
    And the template placeholder "{{harness_binary}}" is replaced by the selected harness's binary
    And the template placeholder "{{cwd}}" is replaced by the handoff working directory
    And the template placeholder "{{prompt_file}}" is replaced by the path of a file containing:
      """
      I need to implement this task: Task #7: Add dark mode (tsk). Check it with `tsk show 7`. Update its state as you go with `tsk move 7 <status>`; valid statuses: backlog, todo, doing, delivered, reviewing, done, cancelled.
      """

  Scenario: A project with an empty workflow drops the status clause
    Given a task "#3 Tidy" in a project whose workflow is empty is focused
    When the handoff prompt is composed
    Then it reads "I need to implement this task: Task #3: Tidy (tsk). Check it with `tsk show 3`."
    And it does not contain "valid statuses"

  Scenario: The handoff runs in tsk's launch directory by default
    Given "handoff.cwd" is not set in the config
    When a handoff is launched
    Then the handoff command runs with the directory where tsk was launched as its working directory

  Scenario: The handoff working directory can be overridden
    Given "handoff.cwd" is set in the config
    When a handoff is launched
    Then the handoff command runs in the configured directory

  Scenario: Cancel the picker
    Given the Ask AI overlay is open
    When the user presses "Esc"
    Then the overlay closes and no process is launched

  # ---------------------------------------------------------------------------
  # Refusals: never a silent no-op
  # ---------------------------------------------------------------------------

  Scenario: No handoff command configured
    Given "handoff.command" is not set in the config
    When the user presses "a" on a focused task
    Then no overlay opens
    And the message "No handoff configured: set handoff.command in config.toml" is shown
    And no process is launched

  Scenario: Handoff command missing the prompt placeholder
    Given the configured "handoff.command" does not contain "{{prompt_file}}"
    When the user presses "a" on a focused task
    Then no overlay opens
    And a message naming "{{prompt_file}}" as the missing placeholder is shown
    And no process is launched

  # ---------------------------------------------------------------------------
  # CLI subcommand
  # ---------------------------------------------------------------------------

  Scenario: Hand a task off from the CLI
    Given a handoff command is configured
    When the user runs "tsk ask <task-id> --harness <name>"
    Then the selected harness is launched non-blocking
    And the command prints a JSON result stating that the handoff was launched
    And it exits with code 0 without waiting for the harness to finish

  Scenario: CLI auto-picks the only available harness
    Given exactly one harness is available
    When the user runs "tsk ask <task-id>" without "--harness"
    Then that harness is used for the handoff

  Scenario: CLI refuses to guess between several harnesses
    Given several harnesses are available
    When the user runs "tsk ask <task-id>" without "--harness"
    Then the command exits with a non-zero code
    And the error lists the available harness names
    And no process is launched

  Scenario: CLI reports unknown task or unknown harness
    Given "handoff.command" is configured
    When the user runs "tsk ask" with a task id that does not exist, or with a harness name that is not available
    Then the command exits with a non-zero code and a JSON error
    And no process is launched

  Scenario: CLI refuses when no handoff is configured
    Given "handoff.command" is not set in the config
    When the user runs "tsk ask <task-id> --harness <name>"
    Then the command exits with a non-zero code
    And the error names the config key "handoff.command"
    And no process is launched
