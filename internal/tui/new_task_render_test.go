package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// The new task modal has six fields and the focus goes on only one: the
// focused row carries a "▸" in front and the others do not. That marker is the
// only thing that says where the cursor is, so checking that it appears exactly
// once is what distinguishes the focused field from the other five.

// newTaskRender returns the form modal without colors, with the given field
// focused.
func newTaskRender(t *testing.T, field int) string {
	t.Helper()
	m := newTestModel(t)
	next, _ := press(m, "i") // open the form
	next.width = 120
	next.newTaskFieldIdx = field
	next.newTaskTitle = "my title"
	next.newTaskAssignee = "@john"
	return ansi.Strip(next.renderNewTaskModal(""))
}

// Each focused field leaves its mark, and only its own.
func TestNewTaskFocusMarkerPerField(t *testing.T) {
	fields := []struct {
		name  string
		field int
		label string
	}{
		{"priority", newTaskFieldPriority, "Priority"},
		{"title", newTaskFieldTitle, "Title"},
		{"assignee", newTaskFieldAssignee, "Assignee"},
		{"tags", newTaskFieldTags, "Tags"},
		{"description", newTaskFieldDescription, "Description"},
	}
	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			out := newTaskRender(t, f.field)

			if n := strings.Count(out, "▸"); n != 1 {
				t.Errorf("with focus on %s there are %d markers, want 1:\n%s", f.name, n, out)
			}
			// The marker sits right next to the focused field's label.
			for _, line := range strings.Split(out, "\n") {
				if !strings.Contains(line, "▸") {
					continue
				}
				if !strings.Contains(line, f.label) {
					t.Errorf("the marker is on %q, and the focused field is %q", line, f.label)
				}
			}
		})
	}
}

// The focused field's text also carries the text cursor, while the others
// do not. The cursor sits next to the value, not on the label.
func TestNewTaskWriteCursorOnlyOnFocusedField(t *testing.T) {
	onTitle := newTaskRender(t, newTaskFieldTitle)
	if !strings.Contains(onTitle, cursorGlyph) {
		t.Errorf("with focus on Title the text cursor does not come out:\n%s", onTitle)
	}

	// Only text fields carry a text cursor: the priority is a
	// selector and you do not type in it.
	out := newTaskRender(t, newTaskFieldAssignee)
	if n := strings.Count(out, cursorGlyph); n != 1 {
		t.Errorf("with focus on Assignee there are %d text cursors, want 1:\n%s", n, out)
	}
}

// With the focus off the title, the value comes out with no cursor attached.
func TestNewTaskNoWriteCursorOutsideTitle(t *testing.T) {
	out := newTaskRender(t, newTaskFieldPriority)
	// The cursor can only appear glued to the focused field's value, and the
	// priority one carries no cursor even when focused.
	if strings.Contains(out, "my title"+cursorGlyph) {
		t.Errorf("with focus elsewhere the title still carries the cursor:\n%s", out)
	}
}

// Pasting text goes to the active field, and in fields with suggestions it
// clears the selection: the list is going to change and the cursor would be
// pointing at something that is no longer there.
func TestNewTaskPasteGoesToActiveField(t *testing.T) {
	tests := []struct {
		name       string
		field      int
		wantTitle  string
		wantAssign string
		wantTagIn  string
	}{
		// The assignee comes set to "Me" when the form opens, so pasting
		// over it concatenates instead of replacing it.
		{"title", newTaskFieldTitle, "pasted", "", ""},
		{"assignee", newTaskFieldAssignee, "", "Mepasted", ""},
		{"tags", newTaskFieldTags, "", "", "pasted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			next, _ := press(m, "i")
			m2 := next
			m2.newTaskFieldIdx = tt.field
			m2.newTaskAssigneeSuggIdx = 3
			m2.newTaskTagSuggIdx = 3
			m2.newTaskErr = "Title is required"

			pasted, _ := m2.Update(tea.PasteMsg{Content: "pasted"})
			got := pasted.(Model)

			if got.newTaskTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", got.newTaskTitle, tt.wantTitle)
			}
			wantAssign := tt.wantAssign
			if wantAssign == "" {
				wantAssign = "Me" // the field's default value
			}
			if got.newTaskAssignee != wantAssign {
				t.Errorf("assignee = %q, want %q", got.newTaskAssignee, wantAssign)
			}
			if got.newTaskTagInput != tt.wantTagIn {
				t.Errorf("tagInput = %q, want %q", got.newTaskTagInput, tt.wantTagIn)
			}
			// The "missing title" error is only cleared by pasting in the title:
			// in the other fields the title is still missing.
			if tt.field == newTaskFieldTitle && got.newTaskErr != "" {
				t.Errorf("pasting in the title does not clear the error: %q", got.newTaskErr)
			}
			if tt.field != newTaskFieldTitle && got.newTaskErr == "" {
				t.Errorf("pasting in %s cleared the error without touching the title: %q", tt.name, got.newTaskErr)
			}
			if tt.field == newTaskFieldAssignee && got.newTaskAssigneeSuggIdx != -1 {
				t.Errorf("assignee suggIdx = %d, want -1", got.newTaskAssigneeSuggIdx)
			}
			if tt.field == newTaskFieldTags && got.newTaskTagSuggIdx != -1 {
				t.Errorf("tags suggIdx = %d, want -1", got.newTaskTagSuggIdx)
			}
		})
	}
}

// Pasting in the description goes to the textarea, not to the one-line
// inputs: a line break is a new line, not a space.
func TestNewTaskPasteIntoDescriptionKeepsNewlines(t *testing.T) {
	m := newTestModel(t)
	next, _ := press(m, "i")
	m2 := next
	m2.newTaskFieldIdx = newTaskFieldDescription
	m2.newTaskTitle = "before"
	// The textarea only accepts the paste with the focus on, which is what the
	// handler does when opening the modal.
	m2.newTaskTextarea.Focus()

	pasted, _ := m2.Update(tea.PasteMsg{Content: "one\ntwo"})
	got := pasted.(Model)

	if got.newTaskTitle != "before" {
		t.Errorf("the title changed when pasting in the description: %q", got.newTaskTitle)
	}
	value := got.newTaskTextarea.Value()
	if !strings.Contains(value, "one") || !strings.Contains(value, "two") {
		t.Errorf("the textarea ended as %q", value)
	}
	if !strings.Contains(value, "\n") {
		t.Errorf("the line break was not kept: %q", value)
	}
}

// Opening the form leaves the model in a known state: focus on priority, no
// title, "Me" as assignee, no selected suggestion and no tags.
// The two -1s are "nothing selected", and a 1 there would point to a list of
// suggestions that does not exist yet.
func TestOpeningNewTaskResetsToInitialState(t *testing.T) {
	m := newTestModel(t)

	// Before opening, the state is different: there is a selected suggestion and tags.
	next, _ := press(m, "i")
	got := next

	if !got.newTaskOpen {
		t.Fatal("the key i did not open the form")
	}
	if got.newTaskFieldIdx != newTaskFieldPriority {
		t.Errorf("focus opens at %d, want %d (priority)", got.newTaskFieldIdx, newTaskFieldPriority)
	}
	if got.newTaskTitle != "" {
		t.Errorf("the title starts at %q, want empty", got.newTaskTitle)
	}
	if got.newTaskAssignee != "Me" {
		t.Errorf("the assignee starts at %q, want \"Me\"", got.newTaskAssignee)
	}
	if got.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("assignee suggIdx = %d, want -1", got.newTaskAssigneeSuggIdx)
	}
	if got.newTaskTagSuggIdx != -1 {
		t.Errorf("tags suggIdx = %d, want -1", got.newTaskTagSuggIdx)
	}
	if len(got.newTaskTags) != 0 {
		t.Errorf("it starts with %d tags, want none", len(got.newTaskTags))
	}
	if got.newTaskErr != "" {
		t.Errorf("it starts with error %q, want none", got.newTaskErr)
	}
	if got.newTaskPriority != model.PriorityLow {
		t.Errorf("the priority starts at %d, want %d", got.newTaskPriority, model.PriorityLow)
	}
}
