package ai

import (
	"os"
	"os/exec"
)

// Focus moves the focus to the pane of the run that just finished, so a
// background task announces itself instead of waiting to be found. It knows the
// two multiplexers tsk runs under:
//
//   - herdr: HERDR_WORKSPACE_ID and HERDR_TAB_ID name the workspace and tab the
//     worker lives in; `herdr workspace focus` then `herdr tab focus` bring it
//     to the front.
//   - tmux: TMUX_PANE is this pane; select-window then select-pane focus it.
//
// It is best effort: no multiplexer means nothing to focus, and a failing
// command must not fail a task that already exists.
func Focus() error {
	if workspace, tab := os.Getenv("HERDR_WORKSPACE_ID"), os.Getenv("HERDR_TAB_ID"); workspace != "" && tab != "" {
		if err := runFocus("herdr", "workspace", "focus", workspace); err != nil {
			return err
		}
		return runFocus("herdr", "tab", "focus", tab)
	}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		if err := runFocus("tmux", "select-window", "-t", pane); err != nil {
			return err
		}
		return runFocus("tmux", "select-pane", "-t", pane)
	}
	return nil
}

// runFocus is the process boundary of Focus; a variable for tests.
var runFocus = func(name string, argv ...string) error {
	return exec.Command(name, argv...).Run()
}
