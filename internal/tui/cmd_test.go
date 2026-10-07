package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// mustMsg runs a cmd and returns its message.
//
// It runs in a goroutine with a time ceiling on purpose. A cmd can be
// a tea.Tick, which sleeps for several seconds, and a test that runs commands
// blindly hangs entirely when a mutant changes the branch it returns. A hung
// test is the worst possible signal in mutation testing: the mutant gets
// reported as TIMED OUT instead of dead, and the gate counts it as
// an "incomplete measurement" instead of a real failure.
func mustMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd is nil")
	}

	type result struct{ msg tea.Msg }
	ch := make(chan result, 1)
	go func() { ch <- result{cmd()} }()

	select {
	case r := <-ch:
		return r.msg
	case <-time.After(250 * time.Millisecond):
		t.Fatal("the command did not return a message in time: it looks like a tea.Tick (it sleeps), not an I/O cmd")
		return nil
	}
}

// mustRun runs a cmd and returns the messages it produces, flattening the
// tea.Batch. The commands that only sleep (a tea.Tick of a toast) are
// skipped: sleeping is their job, not a failure, and the time ceiling removes them
// from the path instead of killing the test.
//
// tea.Batch returns a single BatchMsg: it does not run what it wraps, it only
// groups it so that the bubbletea runtime dispatches it. A test that uses mustMsg
// on a batch gets the BatchMsg and the work never happens, so the
// assertions on the database fail with no bug behind them.
func mustRun(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}

	msg := tryMsg(cmd)
	if msg == nil {
		return nil // it was sleeping: a toast tick, not work
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, inner := range batch {
			out = append(out, mustRun(t, inner)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// tryMsg runs a cmd with the same ceiling as mustMsg but returns nil
// instead of failing when the command does not finish in time.
func tryMsg(cmd tea.Cmd) tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()

	select {
	case msg := <-ch:
		return msg
	case <-time.After(250 * time.Millisecond):
		return nil
	}
}
