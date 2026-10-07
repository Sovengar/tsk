package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// ctrlS builds the save keypress of the inline editor.
func ctrlS() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
}

// send applies a raw message and normalizes the returned model to *Model.
func send(m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	switch v := next.(type) {
	case *Model:
		return v, cmd
	case Model:
		return &v, cmd
	default:
		panic("unexpected model type")
	}
}

// TestDescEditorOpensInList verifies that "e" opens the inline editor (not nvim)
// preloaded with the description of the selected task.
func TestDescEditorOpensInList(t *testing.T) {
	m := newTestModel(t)
	want := m.tasks[0]

	m, _ = press(m, "e")
	if !m.descEditOpen {
		t.Fatal("e must open the inline description editor")
	}
	if !m.detailOpen {
		t.Error("the editor integrates into the detail: the detail must open")
	}
	if m.descEditTaskID != want.ID {
		t.Errorf("taskID = %d, want %d", m.descEditTaskID, want.ID)
	}
	if got := m.descEditTextarea.Value(); got != want.Description {
		t.Errorf("textarea = %q, want %q", got, want.Description)
	}
	if m.overlayKind() != overlayDescEdit {
		t.Errorf("overlay = %v, want overlayDescEdit", m.overlayKind())
	}
}

// TestDescEditorTypingAndNewline verifies that Enter inserts a line break
// instead of saving.
func TestDescEditorTypingAndNewline(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "e")

	base := m.descEditTextarea.Value()
	m, _ = press(m, "x")
	if got := m.descEditTextarea.Value(); got != base+"x" {
		t.Fatalf("after typing: %q, want %q", got, base+"x")
	}

	m, _ = press(m, "enter")
	if got := m.descEditTextarea.Value(); !strings.Contains(got, "\n") {
		t.Errorf("Enter must insert a line break, got %q", got)
	}
	if !m.descEditOpen {
		t.Error("Enter must not close the editor")
	}
}

// TestDescEditorEscCancels verifies that Esc closes without persisting.
func TestDescEditorEscCancels(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	m, _ = press(m, "e")
	m, _ = press(m, "!")
	m, _ = press(m, "esc")

	if m.descEditOpen {
		t.Fatal("Esc must close the editor")
	}
	if m.detailOpen {
		t.Error("when editing from the list, Esc must go back to the list (close the detail)")
	}
	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != task.Description {
		t.Errorf("Esc must not persist: %q, want %q", got.Description, task.Description)
	}
}

// TestDescEditorCtrlSSaves verifies that Ctrl+S persists the description.
func TestDescEditorCtrlSSaves(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	m, _ = press(m, "e")
	m.descEditTextarea.SetValue("new description")
	m, cmd := send(m, ctrlS())

	if m.descEditOpen {
		t.Fatal("Ctrl+S must close the editor")
	}
	if cmd == nil {
		t.Fatal("Ctrl+S must return the save cmd")
	}
	if msg := mustMsg(t, cmd); msg == nil {
		t.Fatal("the save cmd returned nil")
	}

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "new description" {
		t.Errorf("description = %q, want %q", got.Description, "new description")
	}
}

// TestDescEditorFromDetailKeepsDetail verifies that editing from the detail
// keeps the modal open behind and reflects the change on save.
func TestDescEditorFromDetailKeepsDetail(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Fatal("enter must open the detail")
	}

	m, _ = press(m, "e")
	if !m.descEditOpen {
		t.Fatal("e in the detail must open the inline editor")
	}
	if !m.detailOpen {
		t.Fatal("the detail must stay open behind")
	}
	if m.overlayKind() != overlayDescEdit {
		t.Errorf("overlay = %v, want overlayDescEdit", m.overlayKind())
	}

	m.descEditTextarea.SetValue("edited from the detail")
	m, cmd := send(m, ctrlS())
	if cmd == nil {
		t.Fatal("Ctrl+S must return a cmd")
	}
	if !m.detailOpen || m.detailTask == nil {
		t.Fatal("the detail must stay open after saving")
	}
	if m.detailTask.Description != "edited from the detail" {
		t.Errorf("detailTask.Description = %q", m.detailTask.Description)
	}
}

// TestExternalEditorKeyUppercase verifies that "E" still opens the external
// editor (closing the detail in that case).
func TestExternalEditorKeyUppercase(t *testing.T) {
	m := newTestModel(t)

	m, cmd := press(m, "E")
	if cmd == nil {
		t.Fatal("E must return the external editor's cmd")
	}
	if m.descEditOpen {
		t.Error("E must not open the inline editor")
	}

	// From the detail: E closes the detail and launches the external editor.
	m, _ = press(m, "enter")
	m, cmd = press(m, "E")
	if cmd == nil {
		t.Fatal("E in the detail must return the external editor's cmd")
	}
	if m.detailOpen {
		t.Error("E must close the detail")
	}
	if m.descEditOpen {
		t.Error("E must not open the inline editor")
	}
}

// TestDescEditorCtrlCCopiesSelection verifies that Ctrl+C copies only when
// there is a selection (and returns no cmd if there is none).
func TestDescEditorCtrlCCopiesSelection(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "e")
	m.descEditTextarea.SetValue("hello world")

	m, cmd := send(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Error("Ctrl+C with no selection must not return a cmd")
	}

	m.descEditTextarea.SelectAll()
	_, cmd = send(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C with a selection must return the copy cmd")
	}
}

// TestDescEditorRendersInDetail verifies that the editor integrates into the
// box of the detail (title and metadata visible) without overflowing the width.
func TestDescEditorRendersInDetail(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	m, _ = press(m, "e")

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Description:") {
		t.Errorf("the edited detail does not show the Description section:\n%s", out)
	}
	if !strings.Contains(out, task.Title) {
		t.Errorf("the editor must not cover the detail's title:\n%s", out)
	}
	if !strings.Contains(out, "Status:") {
		t.Errorf("the editor must not cover the detail's metadata:\n%s", out)
	}
	// The embedded textarea must not force lines wider than the terminal.
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("line of width %d exceeds the width %d: %q", w, m.width, line)
		}
	}
}
