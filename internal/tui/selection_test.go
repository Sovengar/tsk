package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// pasteMsg is bubbletea's paste message, which is what reaches the handler.
func pasteMsg(content string) tea.PasteMsg { return tea.PasteMsg{Content: content} }

// ---- editable task selection: single source for the external editor and
// the inline one. Without this, an index-limit mutant goes unnoticed. ----

func TestSelectedEditableTaskByView(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		want  string // expected title, "" = nil
	}{
		{
			name:  "list: the task under the cursor",
			setup: func(m *Model) { m.currentView = viewList; m.cursor = 0 },
			want:  "Fix checkout",
		},
		{
			name:  "list: cursor out of range",
			setup: func(m *Model) { m.currentView = viewList; m.cursor = 999 },
			want:  "",
		},
		{
			name:  "list: no tasks",
			setup: func(m *Model) { m.currentView = viewList; m.tasks = nil; m.cursor = 0 },
			want:  "",
		},
		{
			name:  "kanban: task of the current column and row",
			setup: func(m *Model) { m.currentView = viewKanban; m.kanbanCol = 0; m.kanbanRow = 0 },
			want:  "Add caching",
		},
		{
			name:  "kanban: column out of range",
			setup: func(m *Model) { m.currentView = viewKanban; m.kanbanCol = 99; m.kanbanRow = 0 },
			want:  "",
		},
		{
			name:  "kanban: row out of range",
			setup: func(m *Model) { m.currentView = viewKanban; m.kanbanCol = 0; m.kanbanRow = 99 },
			want:  "",
		},
		{
			name: "dashboard: only with the detail open",
			setup: func(m *Model) {
				m.currentView = viewDashboard
				m.detailOpen = true
				m.detailTask = &model.Task{Title: "in detail"}
			},
			want: "in detail",
		},
		{
			name: "dashboard: detail closed",
			setup: func(m *Model) {
				m.currentView = viewDashboard
				m.detailOpen = false
				m.detailTask = &model.Task{Title: "in detail"}
			},
			want: "",
		},
		{
			name:  "dashboard: no task in the detail",
			setup: func(m *Model) { m.currentView = viewDashboard; m.detailOpen = true; m.detailTask = nil },
			want:  "",
		},
		{
			name:  "view with no editable task of its own (gantt)",
			setup: func(m *Model) { m.currentView = viewGantt },
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			tt.setup(m)

			got := m.selectedEditableTask()
			if tt.want == "" {
				if got != nil {
					t.Fatalf("selectedEditableTask() = %q, want nil", got.Title)
				}
				return
			}
			if got == nil {
				t.Fatalf("selectedEditableTask() = nil, want %q", tt.want)
			}
			if got.Title != tt.want {
				t.Errorf("selectedEditableTask() = %q, want %q", got.Title, tt.want)
			}
		})
	}
}

// ---- paste in the text fields: line breaks are flattened. ----

func TestHandleNewTaskPasteFlattensNewlines(t *testing.T) {
	// Each field has its initial value and its suggestion index: pasting only
	// touches the active field, and drops the suggestion of THAT field.
	tests := []struct {
		name          string
		field         int
		initial       string
		content       string
		want          string
		suggReset     bool
		otherSuggHold bool
	}{
		{"title", newTaskFieldTitle, "pre-", "one\ntwo\rthree", "pre-one two three", true, true},
		{"assignee", newTaskFieldAssignee, "@pre ", "one\ntwo", "@pre one two", true, true},
		{"tags", newTaskFieldTags, "pre ", "one\ntwo", "pre one two", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.newTaskOpen = true
			m.newTaskFieldIdx = tt.field
			m.newTaskTitle = ""
			m.newTaskAssignee = ""
			m.newTaskTagInput = ""
			_ = m
			m.newTaskAssigneeSuggIdx = 3
			m.newTaskTagSuggIdx = 3
			m.newTaskErr = "previous error"

			set := func(mm *Model, s string) {
				switch tt.field {
				case newTaskFieldTitle:
					mm.newTaskTitle = s
				case newTaskFieldAssignee:
					mm.newTaskAssignee = s
				case newTaskFieldTags:
					mm.newTaskTagInput = s
				}
			}
			set(m, tt.initial)

			got, _ := m.handleNewTaskPaste(pasteMsg(tt.content))
			after := got.(Model)

			var value string
			switch tt.field {
			case newTaskFieldTitle:
				value = after.newTaskTitle
				if tt.suggReset && after.newTaskAssigneeSuggIdx == -1 {
					t.Error("the assignee suggestion should not reset when pasting in the title")
				}
			case newTaskFieldAssignee:
				value = after.newTaskAssignee
			case newTaskFieldTags:
				value = after.newTaskTagInput
			}
			if value != tt.want {
				t.Errorf("field = %q, want %q", value, tt.want)
			}

			// The pasted field's suggestion is dropped; the others are not.
			activeSugg := -1
			switch tt.field {
			case newTaskFieldAssignee:
				activeSugg = after.newTaskAssigneeSuggIdx
			case newTaskFieldTags:
				activeSugg = after.newTaskTagSuggIdx
			}
			if tt.suggReset && tt.field != newTaskFieldTitle && activeSugg != -1 {
				t.Errorf("the pasted field's suggestion should end up at -1, got %d", activeSugg)
			}
		})
	}
}

// The description field is the only one that delegates to the textarea: the
// text arrives there with its line breaks intact, not to newTaskTitle.
func TestHandleNewTaskPasteGoesToTextareaOnDescription(t *testing.T) {
	m := newTestModel(t)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldDescription
	m.newTaskTitle = "untouched"
	m.newTaskErr = "previous error"

	got, _ := m.handleNewTaskPaste(pasteMsg("with\nbreaks"))
	after := got.(Model)

	if after.newTaskTitle != "untouched" {
		t.Errorf("newTaskTitle = %q, want untouched: the description goes to the textarea", after.newTaskTitle)
	}
	if after.newTaskErr != "previous error" {
		t.Errorf("newTaskErr = %q: pasting in the description does not validate the field", after.newTaskErr)
	}
}

// ---- tag paste: same flattening, and it always resets the suggestion. ----

func TestHandleTagPaste(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		content string
		want    string
	}{
		{"simple", "", "bug", "bug"},
		{"flattens line breaks", "a ", "b\nc\rd", "a b c d"},
		{"empty", "a", "", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.tagOpen = true
			m.tagInput = tt.initial
			m.tagSuggestIdx = 2

			got, _ := m.handleTagPaste(tt.content)
			after := got.(Model)

			if after.tagInput != tt.want {
				t.Errorf("tagInput = %q, want %q", after.tagInput, tt.want)
			}
			if after.tagSuggestIdx != -1 {
				t.Errorf("tagSuggestIdx = %d, want -1", after.tagSuggestIdx)
			}
		})
	}
}

// ---- project selection: decides whether the view goes to archived. ----

func TestSelectProjectByNameSwitchesArchivedView(t *testing.T) {
	tests := []struct {
		name          string
		archived      []model.Project
		target        string
		wantIdx       int
		wantShowArch  bool
		wantUnchanged bool
	}{
		{"active", nil, "api", 0, false, false},
		{"active further down", nil, "web", 1, false, false},
		{"archived", []model.Project{{Name: "old"}}, "old", 0, true, false},
		{"missing", nil, "nope", 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.archivedProjects = tt.archived
			m.dashProjectIdx = -1
			m.showArchived = !tt.wantShowArch

			m.selectProjectByName(tt.target)

			if tt.wantUnchanged {
				if m.dashProjectIdx != -1 || m.showArchived == tt.wantShowArch {
					t.Errorf("a missing name must touch nothing: idx=%d showArchived=%v", m.dashProjectIdx, m.showArchived)
				}
				return
			}
			if m.dashProjectIdx != tt.wantIdx {
				t.Errorf("dashProjectIdx = %d, want %d", m.dashProjectIdx, tt.wantIdx)
			}
			if m.showArchived != tt.wantShowArch {
				t.Errorf("showArchived = %v, want %v", m.showArchived, tt.wantShowArch)
			}
		})
	}
}

// A project appears in both lists: the active one rules, because that is
// where it lives and the index belongs to that list.
func TestSelectProjectByNamePrefersActiveList(t *testing.T) {
	m := newTestModel(t)
	m.archivedProjects = []model.Project{{Name: "api"}}
	m.dashProjectIdx = -1
	m.showArchived = true

	m.selectProjectByName("api")

	if m.showArchived {
		t.Error("showArchived = true, want false (the active list wins)")
	}
	if m.dashProjectIdx != 0 {
		t.Errorf("dashProjectIdx = %d, want 0 (index in the active list)", m.dashProjectIdx)
	}
}

// ---- help modal: it is the keybinds documentation inside the app. ----

func TestRenderHelpModalListsEveryKeybindSource(t *testing.T) {
	views := []viewKind{viewList, viewKanban, viewGantt, viewDashboard}
	for _, v := range views {
		t.Run(v.String(), func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = v

			out := m.renderHelpModal("")
			plain := ansi.Strip(out)

			// The modal's four sections.
			for _, section := range []string{
				"Task detail (modal)",
				"New task (modal)",
				"Filters (modal)",
			} {
				if !strings.Contains(plain, section) {
					t.Errorf("missing section %q", section)
				}
			}
			// And at least one key from each source of truth. If a keybinds
			// list is emptied or stops being rendered, this trips.
			for _, kb := range detailKeybinds() {
				if !strings.Contains(plain, kb.key) {
					t.Errorf("missing detail key %q", kb.key)
				}
			}
			for _, kb := range newTaskKeybinds() {
				if !strings.Contains(plain, kb.key) {
					t.Errorf("missing new task modal key %q", kb.key)
				}
			}
			for _, kb := range filterKeybinds() {
				if !strings.Contains(plain, kb.key) {
					t.Errorf("missing filter modal key %q", kb.key)
				}
			}
			if !strings.Contains(plain, "to close") {
				t.Error("missing the footer saying how to close the help")
			}
		})
	}
}

// ---- suggestions of the autocomplete fields: the logic of "create a new
// one" lives here, not in the render. ----

func TestRenderAssigneeSuggestions(t *testing.T) {
	// The fixtures' roster is Me, @john and @margo. With an empty input all
	// of them are listed; the exact match is EXCLUDED from the list (it is
	// already chosen) and the "create new" hint only appears when there is nobody with that name.
	tests := []struct {
		name     string
		typed    string
		wantRows int
		wantNew  bool
		wantHas  string
	}{
		{"empty input lists the whole roster", "", 3, false, "@john"},
		{"the exact match is not suggested", "@john", 0, false, ""},
		{"prefix: the partial ones remain", "@ma", 2, true, "@margo"},
		{"nobody with that name", "@new", 1, true, ""},
		{"only spaces counts as empty", "   ", 3, false, "@john"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.newTaskOpen = true
			m.newTaskFieldIdx = newTaskFieldAssignee
			m.newTaskAssignee = tt.typed

			lines := m.renderNewTaskAssigneeSuggestions()
			plain := ansi.Strip(strings.Join(lines, " | "))

			if len(lines) != tt.wantRows {
				t.Errorf("lines = %d, want %d (%q)", len(lines), tt.wantRows, plain)
			}
			if got := strings.Contains(plain, "new:"); got != tt.wantNew {
				t.Errorf("offers to create = %v, want %v (%q)", got, tt.wantNew, plain)
			}
			if tt.wantHas != "" && !strings.Contains(plain, tt.wantHas) {
				t.Errorf("missing %q in %q", tt.wantHas, plain)
			}
		})
	}
}

func TestRenderTagSuggestions(t *testing.T) {
	// The task already has "bug": it is neither suggested nor offered to create.
	tests := []struct {
		name     string
		typed    string
		wantRows int
		wantNew  bool
	}{
		{"empty input with no known tags", "", 0, false},
		{"already on the task", "bug", 0, false},
		{"does not exist", "new", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.newTaskOpen = true
			m.newTaskFieldIdx = newTaskFieldTags
			m.newTaskTagInput = tt.typed
			m.newTaskTags = []string{"bug"}

			lines := m.renderTagFieldSuggestions()
			plain := ansi.Strip(strings.Join(lines, " | "))

			if len(lines) != tt.wantRows {
				t.Errorf("lines = %d, want %d (%q)", len(lines), tt.wantRows, plain)
			}
			if got := strings.Contains(plain, "new:"); got != tt.wantNew {
				t.Errorf("offers to create = %v, want %v (%q)", got, tt.wantNew, plain)
			}
		})
	}
}

// The selected suggestion is marked with ▸: it is what the user sees before
// pressing tab, so a mutant marking the wrong row shows up here.
func TestRenderSuggestionsMarkTheSelectedRow(t *testing.T) {
	m := newTestModel(t)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "@ma"

	for idx, wantMarked := range []bool{true, false} {
		m.newTaskAssigneeSuggIdx = -1
		if wantMarked {
			m.newTaskAssigneeSuggIdx = idx
		}
		lines := m.renderNewTaskAssigneeSuggestions()
		if len(lines) == 0 {
			t.Fatal("no suggestions")
		}
		marked := strings.Contains(ansi.Strip(lines[0]), "▸")
		if marked != wantMarked {
			t.Errorf("suggIdx %d: marked = %v, want %v", idx, marked, wantMarked)
		}
	}
}

// ---- off-day form: field navigation and validations. ----

func TestHandleOffdayFormKeyFieldNavigation(t *testing.T) {
	tests := []struct {
		name string
		from int
		key  string
		want int
	}{
		{"tab advances", 0, "tab", 1},
		{"tab wraps around", 2, "tab", 0},
		{"shift+tab goes back", 1, "shift+tab", 0},
		{"shift+tab wraps around", 0, "shift+tab", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.offdayFormOpen = true
			m.offdayFormField = tt.from

			got, _ := m.handleOffdayFormKey(tt.key)
			if after := got.(Model); after.offdayFormField != tt.want {
				t.Errorf("field = %d, want %d", after.offdayFormField, tt.want)
			}
		})
	}
}

func TestHandleOffdayFormKeyWritesIntoTheActiveField(t *testing.T) {
	tests := []struct {
		field int
		key   string
		check func(m Model) string
	}{
		{0, "a", func(m Model) string { return m.offdayStartInput }},
		{1, "a", func(m Model) string { return m.offdayEndInput }},
		{2, "a", func(m Model) string { return m.offdayNoteInput }},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.field), func(t *testing.T) {
			m := newTestModel(t)
			m.offdayFormOpen = true
			m.offdayFormField = tt.field

			got, _ := m.handleOffdayFormKey(tt.key)
			if v := tt.check(got.(Model)); v != "a" {
				t.Errorf("field %d = %q, want a", tt.field, v)
			}
		})
	}
}

func TestHandleOffdayFormKeyValidations(t *testing.T) {
	t.Run("esc closes", func(t *testing.T) {
		m := newTestModel(t)
		m.offdayFormOpen = true
		got, _ := m.handleOffdayFormKey("esc")
		if got.(Model).offdayFormOpen {
			t.Error("esc should close the form")
		}
	})

	t.Run("without assignee it does not save", func(t *testing.T) {
		m := newTestModel(t)
		m.offdayFormOpen = true
		m.assigneeIdx = -1
		m.offdayStartInput = "2026-07-28"

		got, cmd := m.handleOffdayFormKey("enter")
		if cmd == nil {
			t.Error("it should emit an error toast")
		}
		if !got.(Model).offdayFormOpen {
			t.Error("the form should not close without an assignee")
		}
	})

	t.Run("without a date it does not save", func(t *testing.T) {
		m := newTestModel(t)
		m.offdayFormOpen = true
		m.assigneeIdx = 0
		m.offdayStartInput = ""

		got, cmd := m.handleOffdayFormKey("enter")
		if cmd == nil {
			t.Error("it should emit an error toast")
		}
		if !got.(Model).offdayFormOpen {
			t.Error("the form should not close without a date")
		}
	})
}

// ---- project modal: navigation, typing and validations. ----

func TestHandleProjectModalKeyFieldNavigation(t *testing.T) {
	tests := []struct {
		name string
		from int
		key  string
		want int
	}{
		{"tab advances", 0, "tab", 1},
		{"tab wraps around", 2, "tab", 0},
		{"shift+tab goes back", 1, "shift+tab", 0},
		{"shift+tab wraps around", 0, "shift+tab", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.projectModalOpen = true
			m.projectModalField = tt.from

			got, _ := m.handleProjectModalKey(tt.key)
			if after := got.(Model); after.projectModalField != tt.want {
				t.Errorf("field = %d, want %d", after.projectModalField, tt.want)
			}
		})
	}
}

func TestHandleProjectModalKeyWritesIntoTheActiveField(t *testing.T) {
	tests := []struct {
		field int
		check func(m Model) string
	}{
		{0, func(m Model) string { return m.projectNameInput }},
		{1, func(m Model) string { return m.projectWorkflowInput }},
		{2, func(m Model) string { return m.projectListOrderInput }},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.field), func(t *testing.T) {
			m := newTestModel(t)
			m.projectModalOpen = true
			m.projectModalField = tt.field
			m.projectNameInput = ""
			m.projectWorkflowInput = ""
			m.projectListOrderInput = ""

			got, _ := m.handleProjectModalKey("x")
			if v := tt.check(got.(Model)); v != "x" {
				t.Errorf("field %d = %q, want x", tt.field, v)
			}
		})
	}
}

func TestHandleProjectModalKeyEnter(t *testing.T) {
	t.Run("esc closes", func(t *testing.T) {
		m := newTestModel(t)
		m.projectModalOpen = true
		got, _ := m.handleProjectModalKey("esc")
		if got.(Model).projectModalOpen {
			t.Error("esc should close the modal")
		}
	})

	t.Run("empty name does not save", func(t *testing.T) {
		m := newTestModel(t)
		m.projectModalOpen = true
		m.projectNameInput = "   "

		got, cmd := m.handleProjectModalKey("enter")
		if cmd == nil {
			t.Error("it should emit an error toast")
		}
		if !got.(Model).projectModalOpen {
			t.Error("the modal should not close without a name")
		}
	})

	t.Run("valid name closes and persists", func(t *testing.T) {
		m := newTestModel(t)
		m.projectModalOpen = true
		m.projectModalField = 0
		m.projectNameInput = "new"
		m.projectWorkflowInput = "backlog,todo,done"
		m.projectListOrderInput = ""

		got, cmd := m.handleProjectModalKey("enter")
		if cmd == nil {
			t.Fatal("it should return the save cmd")
		}
		if got.(Model).projectModalOpen {
			t.Error("the modal should close on save")
		}

		msg, ok := mustMsg(t, cmd).(projectSavedMsg)
		if !ok {
			t.Fatalf("msg = %T, want projectSavedMsg", msg)
		}
		if msg.err != nil {
			t.Fatalf("save failed: %v", msg.err)
		}
		if _, err := m.database.GetProject("new"); err != nil {
			t.Errorf("the project was not created: %v", err)
		}
	})
}

// The modal opened to edit prefills the fields with the chosen project; the
// one to create leaves them with the defaults and empty.
func TestOpenProjectModalCreateAndEdit(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		m := newTestModel(t)
		m.projectNameInput = "trash"
		m.projectEditingName = "trash"

		m.openProjectModal(false)

		if !m.projectModalOpen || m.projectModalEdit {
			t.Errorf("modal = %v edit=%v, want open and in create mode", m.projectModalOpen, m.projectModalEdit)
		}
		if m.projectNameInput != "" || m.projectEditingName != "" {
			t.Errorf("create should clear the fields: %q/%q", m.projectNameInput, m.projectEditingName)
		}
		if m.projectWorkflowInput != "backlog,todo,doing,delivered,reviewing,done,cancelled" {
			t.Errorf("default workflow = %q", m.projectWorkflowInput)
		}
	})

	t.Run("edit", func(t *testing.T) {
		m := newTestModel(t)
		p, err := m.database.GetProject("web")
		if err != nil {
			t.Fatal(err)
		}

		m.openProjectModalForEdit(p)

		if !m.projectModalOpen || !m.projectModalEdit {
			t.Errorf("modal = %v edit=%v, want open and in edit mode", m.projectModalOpen, m.projectModalEdit)
		}
		if m.projectNameInput != "web" || m.projectEditingName != "web" {
			t.Errorf("name = %q, original = %q", m.projectNameInput, m.projectEditingName)
		}
		if !strings.Contains(m.projectWorkflowInput, "todo") {
			t.Errorf("prefilled workflow = %q", m.projectWorkflowInput)
		}
	})
}

// The highlighted project's index is truncated to the valid range: that is
// what keeps a filter from leaving the cursor pointing at nothing.
func TestClampDashProjectIdx(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		want  int
	}{
		{"within range", func(m *Model) { m.dashProjectIdx = 1 }, 1},
		{"too high", func(m *Model) { m.dashProjectIdx = 99 }, 1},
		{"negative", func(m *Model) { m.dashProjectIdx = -5 }, 0},
		{"no projects", func(m *Model) { m.projects = nil; m.dashProjectIdx = 3 }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			tt.setup(m)
			m.clampDashProjectIdx()
			if m.dashProjectIdx != tt.want {
				t.Errorf("dashProjectIdx = %d, want %d", m.dashProjectIdx, tt.want)
			}
		})
	}
}

// ---- toast: it appears, is truncated to the width and changes style by type. ----

func TestRenderToast(t *testing.T) {
	tests := []struct {
		name      string
		toast     string
		kind      string
		width     int
		wantEmpty bool
		wantText  string
	}{
		{"no toast", "", "info", 80, true, ""},
		{"normal", "saved", "info", 80, false, "saved"},
		{"long is truncated", "0123456789012345678901234567890123456789", "info", 20, false, "…"},
		{"minimum width does not truncate", "ab", "info", 4, false, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.toast = tt.toast
			m.toastKind = tt.kind
			m.width = tt.width

			got := ansi.Strip(m.renderToast())
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("renderToast() = %q, want \"\"", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("renderToast() empty, want %q", tt.wantText)
			}
			if tt.wantText == "…" && !strings.Contains(got, "…") {
				t.Errorf("renderToast() = %q, wants the truncation ellipsis", got)
			}
			if tt.wantText != "…" && !strings.Contains(got, tt.wantText) {
				t.Errorf("renderToast() = %q, want %q", got, tt.wantText)
			}
		})
	}
}

// An error toast is painted with the error style, not the status one: if
// they are swapped, the user sees a failure in success color.
func TestRenderToastErrorUsesErrorStyle(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.toast = "boom"
	m.toastKind = "error"

	styled := m.renderToast()
	if !strings.Contains(styled, "\x1b[") {
		t.Fatalf("no ANSI styles: %q", styled)
	}
	if styleError.Render(" boom") != styled {
		t.Errorf("an error toast should use styleError: %q", styled)
	}

	m.toastKind = "info"
	if styled := m.renderToast(); styleError.Render(" boom") == styled {
		t.Error("an info toast should not use the error style")
	}
}

// ---- off-day form: the "_" cursor marks the active field. ----

func TestRenderOffdayFormMarksTheActiveField(t *testing.T) {
	// The "_" cursor goes behind the active field: if it is put somewhere
	// else, the user types in the wrong field without noticing.
	tests := []struct {
		field  int
		marker string
	}{
		{0, "2026-07-28_"},
		{1, "2026-08-01_"},
		{2, "note_"},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.field), func(t *testing.T) {
			m := newTestModel(t)
			m.assigneeIdx = 0
			m.offdayFormOpen = true
			m.offdayFormField = tt.field
			m.offdayStartInput = "2026-07-28"
			m.offdayEndInput = "2026-08-01"
			m.offdayNoteInput = "note"

			out := ansi.Strip(m.renderOffdayForm(""))
			if !strings.Contains(out, tt.marker) {
				t.Errorf("field %d: missing the cursor %q:\n%s", tt.field, tt.marker, out)
			}
		})
	}
}

// The form always shows its four fields with a label and the format hint:
// if a field disappears from the render, nobody notices except here.
func TestRenderOffdayFormShowsEveryField(t *testing.T) {
	m := newTestModel(t)
	m.assigneeIdx = 0
	m.offdayFormOpen = true
	m.offdayFormField = 0

	out := ansi.Strip(m.renderOffdayForm(""))
	for _, want := range []string{"Person:", "Start:", "End:", "Note:", "YYYY-MM-DD"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

// The modal is drawn even with no content behind: it is a real case when
// opening from an empty state, and if the overlay does not tolerate a short background it blows up.
func TestRenderOffdayFormOverEmptyBackground(t *testing.T) {
	m := newTestModel(t)
	m.assigneeIdx = 0
	m.offdayFormOpen = true
	m.width = 100

	out := ansi.Strip(m.renderOffdayForm(""))
	if !strings.Contains(out, "New off-day") {
		t.Errorf("the modal must not be missing even without a background:\n%s", out)
	}
}

// ---- PreviewBar and the description editor: size adjustments. ----

func TestPreviewBarSetWidth(t *testing.T) {
	var p PreviewBar
	p.SetWidth(42)
	if p.width != 42 {
		t.Errorf("width = %d, want 42", p.width)
	}
	p.SetMaxLines(7)
	if p.maxLines != 7 {
		t.Errorf("maxLines = %d, want 7", p.maxLines)
	}
}

// The description editor sizes itself to the width of the detail box, not
// to the terminal's. It is tested through the real path (opening the editor
// and then resizing) because the textarea only exists while the editor is
// open: calling resizeDescEditor() on a freshly created model is not a
// reachable state.
func TestResizeDescEditorFollowsTerminal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "e")
	if !m.descEditOpen {
		t.Fatal("could not open the inline editor")
	}

	m.width = 100
	m, _ = send(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	wide := m.descEditTextarea.Width()
	if want := m.descEditorWidth(); wide != want {
		t.Errorf("textarea width = %d, want %d", wide, want)
	}

	m, _ = send(m, tea.WindowSizeMsg{Width: 40, Height: 40})
	narrow := m.descEditTextarea.Width()
	if want := m.descEditorWidth(); narrow != want {
		t.Errorf("after narrowing: width = %d, want %d", narrow, want)
	}
	if narrow >= wide {
		t.Errorf("narrowing the terminal did not narrow the editor: %d -> %d", wide, narrow)
	}

	// A very narrow terminal never leaves the width at an invalid value.
	m, _ = send(m, tea.WindowSizeMsg{Width: 2, Height: 10})
	if got := m.descEditorWidth(); got < 1 {
		t.Errorf("descEditorWidth() = %d, want >= 1", got)
	}

	// With the editor closed, a resize must not touch anything.
	m.descEditOpen = false
	before := m.descEditTextarea.Width()
	m, _ = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if got := m.descEditTextarea.Width(); got != before {
		t.Errorf("with the editor closed the width changed: %d -> %d", before, got)
	}
}

// The sequence counter exists so that an old tick does not erase a newer
// toast: each setToast increments and stores the value, and the tick only
// clears if its number is still the current one.
func TestToastSeqIgnoresStaleTick(t *testing.T) {
	m := newTestModel(t)
	firstToast := m.setToast("first", "info")
	firstToastSeq := m.toastSeq

	secondToast := m.setToast("second", "error")
	if m.toastSeq == firstToastSeq {
		t.Fatalf("the counter did not advance: %d", m.toastSeq)
	}

	// The first toast's tick arrives, which is no longer the current one: the toast stays.
	if msg := firstToast(); msg != nil {
		if exp, ok := msg.(toastExpiredMsg); ok && exp.seq == m.toastSeq {
			t.Errorf("the stale tick carries the current sequence %d", exp.seq)
		}
	}

	next, _ := m.Update(toastExpiredMsg{seq: m.toastSeq})
	if next.(Model).toast != "" {
		t.Errorf("the tick of the current sequence did not clear the toast: %q", next.(Model).toast)
	}

	// And the second tick either, on what is already clean.
	if cmd := secondToast(); cmd == nil {
		t.Error("the second toast did not return a command")
	}
}

// The toast is truncated to the width minus the margin, and the margin is
// what keeps the text from touching the borders. With no truncation there is nothing to truncate.
func TestToastTruncationWidth(t *testing.T) {
	tests := []struct {
		name     string
		toast    string
		width    int
		wantTrim bool
	}{
		{"short", "hello", 40, false},
		{"exactly the width", strings.Repeat("x", 38), 40, false},
		{"one too many", strings.Repeat("x", 39), 40, true},
		{"way too long", strings.Repeat("x", 100), 40, true},
		// With fewer than five columns nothing is truncated, even if the text
		// does not fit: the two-column margin would eat the whole text. That
		// is why the threshold is "> 4" and not "> 2".
		{"width of five, fits", "abc", 5, false},
		{"width of five, does not fit", "abcd", 5, true},
		{"width of four", "abcd", 4, false},
		{"width of three", "abcd", 3, false},
		{"width of two", "abcd", 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.toast = tt.toast
			m.toastKind = "info"
			m.width = tt.width

			out := ansi.Strip(m.renderToast())
			// The toast is drawn with a margin space in front, so the
			// comparison is against the text with that space.
			if got := out != " "+tt.toast; got != tt.wantTrim {
				t.Errorf("truncated=%v, want %v (output %q)", got, tt.wantTrim, out)
			}
		})
	}
}

// The toast's style depends on the kind, and "error" is not the same as "info".
func TestToastErrorStyle(t *testing.T) {
	m := newTestModel(t)
	m.toast = "failed"
	m.width = 40

	m.toastKind = "error"
	withError := m.renderToast()
	m.toastKind = "info"
	withInfo := m.renderToast()

	if withError == withInfo {
		t.Error("an error toast looks the same as an info one")
	}
	if !strings.Contains(ansi.Strip(withError), "failed") {
		t.Errorf("the error toast does not show:\n%q", withError)
	}
}
