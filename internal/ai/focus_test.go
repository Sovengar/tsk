package ai

import (
	"errors"
	"strings"
	"testing"
)

func TestFocusUsesHerdr(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "w4C")
	t.Setenv("HERDR_TAB_ID", "w4C:t2")
	t.Setenv("TMUX_PANE", "")

	var calls []string
	restore := runFocus
	runFocus = func(name string, argv ...string) error {
		calls = append(calls, strings.Join(append([]string{name}, argv...), " "))
		return nil
	}
	t.Cleanup(func() { runFocus = restore })

	if err := Focus(); err != nil {
		t.Fatalf("focus: %v", err)
	}
	want := []string{"herdr workspace focus w4C", "herdr tab focus w4C:t2"}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestFocusUsesTmux(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "")
	t.Setenv("HERDR_TAB_ID", "")
	t.Setenv("TMUX_PANE", "%3")

	var calls []string
	restore := runFocus
	runFocus = func(name string, argv ...string) error {
		calls = append(calls, strings.Join(append([]string{name}, argv...), " "))
		return nil
	}
	t.Cleanup(func() { runFocus = restore })

	if err := Focus(); err != nil {
		t.Fatalf("focus: %v", err)
	}
	want := []string{"tmux select-window -t %3", "tmux select-pane -t %3"}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestFocusWithoutAMultiplexer(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "")
	t.Setenv("HERDR_TAB_ID", "")
	t.Setenv("TMUX_PANE", "")

	if err := Focus(); err != nil {
		t.Fatalf("focus: %v", err)
	}
}

func TestFocusReportsTheFirstFailure(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "w4C")
	t.Setenv("HERDR_TAB_ID", "w4C:t2")

	restore := runFocus
	runFocus = func(string, ...string) error { return errors.New("no socket") }
	t.Cleanup(func() { runFocus = restore })

	if err := Focus(); err == nil {
		t.Fatal("want the focus failure")
	}
}

func TestFocusReportsTheTabFailure(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "w4C")
	t.Setenv("HERDR_TAB_ID", "w4C:t2")

	calls := 0
	restore := runFocus
	runFocus = func(string, ...string) error {
		calls++
		if calls == 1 {
			return nil
		}
		return errors.New("no tab")
	}
	t.Cleanup(func() { runFocus = restore })

	if err := Focus(); err == nil {
		t.Fatal("want the tab failure")
	}
}

func TestFocusReportsTheWindowFailure(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "")
	t.Setenv("HERDR_TAB_ID", "")
	t.Setenv("TMUX_PANE", "%3")

	calls := 0
	restore := runFocus
	runFocus = func(string, ...string) error {
		calls++
		if calls == 1 {
			return nil
		}
		return errors.New("no pane")
	}
	t.Cleanup(func() { runFocus = restore })

	if err := Focus(); err == nil {
		t.Fatal("want the pane failure")
	}
}

func TestRunFocusRunsTheCommand(t *testing.T) {
	if err := runFocus("true"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if err := runFocus("false"); err == nil {
		t.Fatal("want an error from a failing command")
	}
}

func TestFocusReportsTheWindowSelectionFailure(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "")
	t.Setenv("HERDR_TAB_ID", "")
	t.Setenv("TMUX_PANE", "%3")

	restore := runFocus
	runFocus = func(string, ...string) error { return errors.New("no window") }
	t.Cleanup(func() { runFocus = restore })

	if err := Focus(); err == nil {
		t.Fatal("want the select-window failure")
	}
}
