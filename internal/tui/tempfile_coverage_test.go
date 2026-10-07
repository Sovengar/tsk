package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// errInjected is the failure the doubles inject: one that no disk produces.
var errInjected = errors.New("injected failure")

// failingFile is a fake temp file. Its two failures --write and close--
// are the ones a real tempfile never produces on a
// working machine: they would need a full disk or closing the same file twice.
// The double exists for that, and to check that the file is deleted anyway,
// which is the part that really matters: a half-written temp file in /tmp does
// not clean itself up.
type failingFile struct {
	name     string
	written  []byte
	errWrite error
	errClose error
	closes   int
	writes   int
}

func (a *failingFile) Write(p []byte) (int, error) {
	a.writes++
	if a.errWrite != nil {
		return 0, a.errWrite
	}
	a.written = append(a.written, p...)
	return len(p), nil
}

func (a *failingFile) Close() error {
	a.closes++
	return a.errClose
}

func (a *failingFile) Name() string { return a.name }

// livePath returns a real path inside t.TempDir so that the code's
// os.Remove has something to actually delete.
func livePath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tsk-test.md")
	if err := os.WriteFile(path, []byte("old content"), 0o600); err != nil {
		t.Fatalf("preparing the temp file: %v", err)
	}
	return path
}

func TestWriteAndCloseLeavesTheTempReady(t *testing.T) {
	path := livePath(t)
	f := &failingFile{name: path}

	if err := writeAndClose(f, "new content"); err != nil {
		t.Fatalf("writeAndClose: %v", err)
	}
	if string(f.written) != "new content" {
		t.Errorf("it wrote %q, want %q", f.written, "new content")
	}
	if f.closes != 1 {
		t.Errorf("it closed %d times, want 1", f.closes)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a temp file written correctly must not be deleted: %v", err)
	}
}

// The comment carries no template: it passes the empty string and still
// writes, which is what distinguishes it from "I wrote nothing" when the editor reads the file.
func TestWriteAndCloseAcceptsEmptyContent(t *testing.T) {
	f := &failingFile{name: livePath(t)}

	if err := writeAndClose(f, ""); err != nil {
		t.Fatalf("writeAndClose with empty content: %v", err)
	}
	if f.writes != 1 {
		t.Errorf("it wrote %d times, want 1: the temp file must exist even if empty", f.writes)
	}
	if f.closes != 1 {
		t.Errorf("it closed %d times, want 1", f.closes)
	}
}

// The two failures. In both cases the file is deleted: that is half the reason
// they exist, because a half-written temp file stays in /tmp forever.
func TestWriteAndCloseDeletesTheTempOnFailure(t *testing.T) {
	t.Run("write fails", func(t *testing.T) {
		path := livePath(t)
		f := &failingFile{name: path, errWrite: errInjected}

		err := writeAndClose(f, "new content")
		if !errors.Is(err, errInjected) {
			t.Fatalf("error = %v, want %v", err, errInjected)
		}
		if f.written != nil {
			t.Errorf("it wrote something despite the write failing: %q", f.written)
		}
		if f.closes != 1 {
			t.Errorf("it closed %d times, want 1: an open file is closed even if the write fails", f.closes)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the temp file is still there after the write failed: %v", err)
		}
	})

	t.Run("close fails", func(t *testing.T) {
		path := livePath(t)
		f := &failingFile{name: path, errClose: errInjected}

		err := writeAndClose(f, "new content")
		if !errors.Is(err, errInjected) {
			t.Fatalf("error = %v, want %v", err, errInjected)
		}
		// The content did get written: the failure comes later, and the editor
		// is not launched, so it does not matter what it contains.
		if string(f.written) != "new content" {
			t.Errorf("it wrote %q, want %q", f.written, "new content")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the temp file is still there after the close failed: %v", err)
		}
	})
}

// If the name is not on disk, os.Remove fails and it is ignored. It is not a
// real production path -- os.CreateTemp just created the temp file -- but it
// documents that a delete failure does not cover up the error that does matter.
func TestWriteAndCloseKeepsTheErrorWhenItCannotDelete(t *testing.T) {
	f := &failingFile{
		name:     filepath.Join(t.TempDir(), "never-existed.md"),
		errWrite: errInjected,
	}

	if err := writeAndClose(f, "x"); !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want the write one and not the delete one", err)
	}
}

// The two callers of writeAndClose. Without the indirection of createTemp
// their error branches could not be tested: a real temp file does not fail on
// write on a working machine, so that if would never be reached.
//
// And what is checked is what really matters to the person who is
// in front: the error comes out with the task id, so the TUI knows which task
// the failure belongs to and does not lose it.
func TestBothEditorsReturnTheTaskIDWhenTheTempFails(t *testing.T) {
	t.Run("task editor", func(t *testing.T) {
		restore := replaceTemp(t, func(string, string) (tempFile, error) {
			return &failingFile{name: livePath(t), errWrite: errInjected}, nil
		})
		defer restore()

		msg := editTaskCmd(model.Task{ID: 42, Title: "with a title"}, "vi /tmp/x")()
		finished, ok := msg.(editorFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want editorFinishedMsg", msg)
		}
		if !errors.Is(finished.err, errInjected) {
			t.Errorf("err = %v, want the injected one", finished.err)
		}
		if finished.taskID != 42 {
			t.Errorf("taskID = %d, want 42: without it the TUI does not know which task to return to", finished.taskID)
		}
	})

	t.Run("comment editor", func(t *testing.T) {
		restore := replaceTemp(t, func(string, string) (tempFile, error) {
			return &failingFile{name: livePath(t), errClose: errInjected}, nil
		})
		defer restore()

		msg := commentCmd(7, "vi /tmp/x")()
		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentFinishedMsg", msg)
		}
		if !errors.Is(finished.err, errInjected) {
			t.Errorf("err = %v, want the injected one", finished.err)
		}
		if finished.taskID != 7 {
			t.Errorf("taskID = %d, want 7", finished.taskID)
		}
	})
}

// The temp file creation failure is the other path, and it is the only one that
// was tested before: TMPDIR points at a directory that does not exist.
func TestBothEditorsReturnTheIDWhenTheTempCannotBeCreated(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))

	for _, tc := range []struct {
		name   string
		call   func() tea.Msg
		taskID func(tea.Msg) (int64, error)
	}{
		{"task", func() tea.Msg { return editTaskCmd(model.Task{ID: 5}, "vi /tmp/x")() },
			func(msg tea.Msg) (int64, error) {
				finished := msg.(editorFinishedMsg)
				return finished.taskID, finished.err
			}},
		{"comment", func() tea.Msg { return commentCmd(9, "vi /tmp/x")() },
			func(msg tea.Msg) (int64, error) {
				finished := msg.(commentFinishedMsg)
				return finished.taskID, finished.err
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			taskID, err := tc.taskID(tc.call())
			if err == nil {
				t.Fatal("want error: TMPDIR does not exist")
			}
			if taskID == 0 {
				t.Error("the error came out without a task id")
			}
		})
	}
}

// replaceTemp swaps createTemp during the test and returns the
// function that puts it back as it was. Tests running in parallel could not use
// this, and that is why none of them does: it is a global.
func replaceTemp(t *testing.T, f func(string, string) (tempFile, error)) func() {
	t.Helper()
	previous := createTemp
	createTemp = f
	return func() { createTemp = previous }
}
