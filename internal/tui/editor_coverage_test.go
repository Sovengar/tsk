package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tsk/internal/model"
)

// The external editor is the only part of the program that depends on another
// binary, and everything it does with it is behind tea.ExecProcess: a Cmd that
// returns an execMsg that only the Bubbletea loop knows how to run. What can be
// checked without a terminal is the half that decides what to do with what the
// editor left behind, and that half is readEditedFile.

func TestReadEditedFile(t *testing.T) {
	t.Run("the editor failed", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "deleted.md")
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := readEditedFile(path, errActionFailed)
		if !errors.Is(err, errActionFailed) {
			t.Errorf("the error is %v, want the process's", err)
		}
		if got != "" {
			t.Errorf("the content is %q, want empty when the editor failed", got)
		}
		// The temp file is deleted even on failure: otherwise every failed edit
		// would leave a file in /tmp.
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("the temp file was not deleted after a failed editor")
		}
	})

	t.Run("the file is gone", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "never-existed.md")

		if _, err := readEditedFile(path, nil); err == nil {
			t.Error("reading a missing temp file did not fail")
		}
	})

	t.Run("the editor wrote nothing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.md")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := readEditedFile(path, nil)
		if err != nil {
			t.Fatalf("readEditedFile: %v", err)
		}
		if got != "" {
			t.Errorf("the content is %q, want empty", got)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("the temp file was not deleted")
		}
	})

	t.Run("the editor wrote something", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "full.md")
		if err := os.WriteFile(path, []byte("hello\n\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		// The truncation is not this place's business: who decides whether the
		// comment is truncated is commentCmd, and the external editor keeps the breaks.
		got, err := readEditedFile(path, nil)
		if err != nil {
			t.Fatalf("readEditedFile: %v", err)
		}
		if got != "hello\n\n" {
			t.Errorf("the content is %q, want %q untrimmed", got, "hello\n\n")
		}
	})
}

// If the temp directory does not exist, there is no file to edit: the
// command returns the error with the task id instead of breaking. That is
// what happens with a misconfigured TMPDIR, and before these tests nobody had
// ever seen it happen.
func TestExternalEditorWithoutATempDir(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))

	t.Run("comment", func(t *testing.T) {
		msg := mustMsg(t, commentCmd(7, "nvim"))
		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("without a temp directory there was no error")
		}
		if finished.taskID != 7 {
			t.Errorf("the message carries taskID %d, want 7 so the UI knows which task it is", finished.taskID)
		}
	})

	t.Run("task editor", func(t *testing.T) {
		msg := mustMsg(t, editTaskCmd(model.Task{ID: 9, Title: "t"}, "nvim"))
		finished, ok := msg.(editorFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want editorFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("without a temp directory there was no error")
		}
		if finished.taskID != 9 {
			t.Errorf("the message carries taskID %d, want 9", finished.taskID)
		}
	})
}

// With the DB closed, the three comment operations fail and the message that
// comes out is the "reloaded without comments" one, not a panic. The important
// thing is that the detail stays usable and not with an impossible selection index.
func TestCommentCommandsWithAClosedDB(t *testing.T) {
	m := newDetailModel(t, 2)
	taskID := m.detailTask.ID
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	t.Run("load", func(t *testing.T) {
		msg := mustMsg(t, m.loadCommentsCmd(taskID))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentsLoadedMsg", msg)
		}
		if len(loaded.comments) != 0 || loaded.selectIdx != -1 {
			t.Errorf("with the DB closed there were %d comments and sel=%d, want 0 and -1",
				len(loaded.comments), loaded.selectIdx)
		}
	})

	t.Run("add", func(t *testing.T) {
		msg := mustMsg(t, m.addCommentCmd(taskID, "new"))
		loaded := msg.(commentsLoadedMsg)
		if len(loaded.comments) != 0 || loaded.selectIdx != -1 {
			t.Errorf("with the DB closed there were %d comments and sel=%d, want 0 and -1",
				len(loaded.comments), loaded.selectIdx)
		}
	})

	t.Run("delete", func(t *testing.T) {
		msg := mustMsg(t, m.deleteCommentCmd(taskID, 1, 2))
		loaded := msg.(commentsLoadedMsg)
		if len(loaded.comments) != 0 {
			t.Errorf("with the DB closed there were %d comments, want 0", len(loaded.comments))
		}
		// A failed delete keeps the selection so the user can see which
		// comment is still there; success leaves it at -1.
		if loaded.selectIdx != 2 {
			t.Errorf("selectIdx = %d, want 2 (keep the selection when the delete fails)",
				loaded.selectIdx)
		}
	})
}

// The external editor's save rewrites five fields at once. If the DB
// fails halfway, the model has to stay as it was instead of showing a
// half-saved task.
func TestUpdateTaskFromEditWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if msg := m.updateTaskFromEdit(id, "Title: other\nStatus: todo\n")(); msg != nil {
		t.Errorf("a failed save returned %T, want nil", msg)
	}
}

// Neither the selected task nor the current project exist when the filter
// leaves nothing: opening the editor or the form then cannot do anything.
func TestExternalEditorNeedsASelectedTask(t *testing.T) {
	m := newTestModel(t)
	m.filteredT = nil
	m.tasks = nil

	if cmd := m.editSelectedTask(); cmd != nil {
		t.Error("with no task selected the external editor was launched anyway")
	}
}

func TestNewTaskNeedsACurrentProject(t *testing.T) {
	// The project filter only counts in List and Kanban; in the Dashboard, with
	// no projects, there is no current project and the form cannot be opened.
	m := newTestModel(t)
	m.currentView = viewDashboard
	m.filterProject = ""
	m.projects = nil

	if cmd := m.newTask(); cmd != nil {
		t.Error("with no current project the form was launched anyway")
	}
	if m.newTaskOpen {
		t.Error("the form opened without a project")
	}
}

// The external editor's command is what gets indexed when composing the
// exec.Command, so an empty command would panic. The default config brings
// "nvim"; a config with the field empty is possible and that is the case this covers.
func TestEditorCommandFallsBackWhenUnset(t *testing.T) {
	if got := editorCommand(""); got != "nvim" {
		t.Errorf("editorCommand(%q) = %q, want nvim", "", got)
	}
	if got := editorCommand("hx"); got != "hx" {
		t.Errorf("editorCommand(%q) = %q, want the config command", "hx", got)
	}
}

// An external editor that returns an unreadable file does not leave the task
// in an intermediate state: the content is discarded and the model stays as it was.
func TestEditorFinishedWithUnparseableContent(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	before := m.tasks[0].Title

	next, _ := updateMsg(t, m, editorFinishedMsg{taskID: id, file: "this is not the format"})
	task := taskByTitle(t, next, before)
	if task == nil {
		t.Fatalf("the task %q disappeared after an editor with garbage", before)
	}
	if task.Title != before {
		t.Errorf("the title is %q, want %q", task.Title, before)
	}
}
