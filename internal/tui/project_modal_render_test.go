package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The project modal has a three-field form, an archive confirmation and a
// save path that tells create from edit. The tests that covered it looked at
// whether the modal opened; what matters is which field is focused, what text
// is sent and what the model does with the response.

// projectModalRender returns the project modal without colors.
func projectModalRender(t *testing.T, m *Model) string {
	t.Helper()
	m.width = 120
	return ansi.Strip(m.renderProjectModal(""))
}

// newProjectModel opens the project modal in creation mode, with the given
// field focused.
func newProjectModel(t *testing.T, field int) *Model {
	t.Helper()
	m := newTestModel(t)
	m.projectModalOpen = true
	m.projectModalEdit = false
	m.projectModalField = field
	m.projectNameInput = "new"
	m.projectWorkflowInput = ""
	m.projectListOrderInput = ""
	return m
}

// The cursor goes in the focused field, and only in it. With three fields,
// moving the focus wraps all the way around.
func TestProjectModalCursorFollowsFocusedField(t *testing.T) {
	tests := []struct {
		name  string
		field int
		want  string
	}{
		{"name", 0, "new_"},
		{"workflow", 1, "new"},
		{"list order", 2, "new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newProjectModel(t, tt.field)
			m.projectWorkflowInput = "todo,done"
			m.projectListOrderInput = "todo"

			out := projectModalRender(t, m)
			if !strings.Contains(out, tt.want+" ") {
				t.Errorf("%q is not visible with the cursor on field %d:\n%s", tt.want, tt.field, out)
			}
			// The cursor is an underscore glued to the value, not a separate field.
			if strings.Count(out, "_") != 1 {
				t.Errorf("there are %d cursors, want exactly 1:\n%s", strings.Count(out, "_"), out)
			}
		})
	}
}

// Moving the focus with tab walks the three fields and returns to the first.
func TestProjectModalFieldCycles(t *testing.T) {
	from := []int{0, 1, 2}
	want := []int{1, 2, 0}
	for i, f := range from {
		m := newProjectModel(t, f)
		next, _ := press(m, "tab")
		if next.projectModalField != want[i] {
			t.Errorf("tab from field %d goes to %d, want %d", f, next.projectModalField, want[i])
		}
	}
}

// Creating with an empty workflow lets the database put the default one;
// sending it an invalid workflow, on the other hand, is an error that goes back to the modal.
func TestProjectSubmitEmptyWorkflowUsesDefault(t *testing.T) {
	m := newProjectModel(t, 0)
	m.projectWorkflowInput = ""
	m.projectListOrderInput = ""

	msg := m.saveProjectCmd(false, "", "new", m.projectWorkflowInput, m.projectListOrderInput)()
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("message %T, want projectSavedMsg", msg)
	}
	if saved.err != nil {
		t.Fatalf("create with an empty workflow failed: %v", saved.err)
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create", saved.action)
	}

	p, err := m.database.GetProject("new")
	if err != nil {
		t.Fatalf("the project was not created: %v", err)
	}
	if len(p.Workflow) == 0 {
		t.Error("the project ended up with no workflow, want the default one")
	}
}

func TestProjectSubmitInvalidWorkflowReportsError(t *testing.T) {
	m := newProjectModel(t, 0)
	m.projectWorkflowInput = "todo,roto,roto"

	msg := m.saveProjectCmd(false, "", "new", m.projectWorkflowInput, m.projectListOrderInput)()
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("message %T, want projectSavedMsg", msg)
	}
	if saved.err == nil {
		t.Fatal("a workflow with duplicates did not error")
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create so the modal reopens", saved.action)
	}
	if _, err := m.database.GetProject("new"); err == nil {
		t.Error("the project was created despite the error")
	}
}

// An invalid list order is also an error, and with the same action.
func TestProjectSubmitInvalidListOrderReportsError(t *testing.T) {
	m := newProjectModel(t, 0)
	m.projectWorkflowInput = "todo,done"
	m.projectListOrderInput = "a,b,a"

	saved := m.saveProjectCmd(false, "", "new", m.projectWorkflowInput, m.projectListOrderInput)().(projectSavedMsg)
	if saved.err == nil {
		t.Fatal("a list order with duplicates did not error")
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create", saved.action)
	}
}

// A project that already exists gives an error, and the name in the message is its own.
func TestProjectSubmitDuplicateReportsError(t *testing.T) {
	m := newTestModel(t)
	m.projectModalOpen = true
	m.projectModalEdit = false

	// "api" already exists in the fixture.
	saved := m.saveProjectCmd(false, "", "api", "todo,done", "")().(projectSavedMsg)
	if saved.err == nil {
		t.Fatal("creating a project that already exists did not error")
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create", saved.action)
	}
}

// An error on save reopens the modal so the written text is not lost; an
// error on archive or restore does not, because those have no written text behind them.
func TestProjectSavedErrorReopensModalOnlyForCreateAndEdit(t *testing.T) {
	tests := []struct {
		action   string
		wantOpen bool
	}{
		{"create", true},
		{"edit", true},
		{"archive", false},
		{"unarchive", false},
		{"other", false},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			m := newTestModel(t)
			m.projectModalOpen = false

			nextMsg, _ := m.handleProjectSaved(projectSavedMsg{err: errActionFailed, action: tt.action})
			next := nextMsg.(Model)
			if next.projectModalOpen != tt.wantOpen {
				t.Errorf("after an error on %q the modal ended %v, want %v", tt.action, next.projectModalOpen, tt.wantOpen)
			}
			if next.toast == "" {
				t.Error("the error is not reported with a toast")
			}
		})
	}
}

// Archiving and restoring are different operations, and the confirmation says so.
func TestProjectConfirmDistinguishesArchiveFromUnarchive(t *testing.T) {
	archive := newTestModel(t)
	archive.confirmOpen = true
	archive.confirmAction = "archive"
	archive.confirmProject = "api"

	out := ansi.Strip(archive.renderConfirmModal(""))
	if !strings.Contains(out, "Archive project") {
		t.Errorf("the archive confirmation does not say so:\n%s", out)
	}
	if strings.Contains(out, "Restore project") {
		t.Errorf("the archive confirmation says restore:\n%s", out)
	}

	restore := newTestModel(t)
	restore.confirmOpen = true
	restore.confirmAction = "unarchive"
	restore.confirmProject = "api"

	out2 := ansi.Strip(restore.renderConfirmModal(""))
	if !strings.Contains(out2, "Restore project") {
		t.Errorf("the restore confirmation does not say so:\n%s", out2)
	}
}

// selectedDashProject is nil with no selection and with an empty list, and the
// project as soon as there is one.
func TestSelectedDashProjectBounds(t *testing.T) {
	m := newTestModel(t)

	m.dashProjectIdx = -1
	if p := m.selectedDashProject(); p != nil {
		t.Errorf("with index -1 it returns %v, want nil", p.Name)
	}

	m.projects = nil
	m.dashProjectIdx = 0
	if p := m.selectedDashProject(); p != nil {
		t.Errorf("with no projects it returns %v, want nil", p.Name)
	}

	m.projects, _ = m.database.ListProjects()
	m.dashProjectIdx = len(m.projects) + 5
	if p := m.selectedDashProject(); p != nil {
		t.Errorf("out-of-range index returns %v, want nil", p.Name)
	}

	m.dashProjectIdx = 0
	p := m.selectedDashProject()
	if p == nil {
		t.Fatal("with a project at position 0 it returns nil")
	}
	if p.Name != m.projects[0].Name {
		t.Errorf("returns %q, want %q", p.Name, m.projects[0].Name)
	}
}

// Archiving or restoring a project that does not exist fails, and the message
// carries the error and the action: the modal closes but the toast warns, and
// a "could not" without saying which operation failed would be useless.
func TestProjectActionOnMissingProject(t *testing.T) {
	m := newTestModel(t)

	for _, action := range []string{"archive", "unarchive"} {
		t.Run(action, func(t *testing.T) {
			msg := m.projectActionCmd(action, "does-not-exist")()
			saved, ok := msg.(projectSavedMsg)
			if !ok {
				t.Fatalf("message %T, want projectSavedMsg", msg)
			}
			if saved.err == nil {
				t.Errorf("%s of a non-existent project did not error", action)
			}
			if saved.action != action {
				t.Errorf("action = %q, want %q", saved.action, action)
			}
		})
	}
}

// With the real project, archiving takes it out of the active list and
// restoring puts it back. The name travels in the success message, which is
// what the toast uses.
func TestProjectArchiveAndUnarchive(t *testing.T) {
	m := newTestModel(t)

	ok := m.projectActionCmd("archive", "api")()
	saved, isSaved := ok.(projectSavedMsg)
	if !isSaved {
		t.Fatalf("message %T, want projectSavedMsg", ok)
	}
	if saved.err != nil {
		t.Fatalf("archiving api failed: %v", saved.err)
	}
	if saved.name != "api" {
		t.Errorf("name = %q, want api", saved.name)
	}

	active, _ := m.database.ListProjects()
	for _, p := range active {
		if p.Name == "api" {
			t.Error("api is still in the active list after archiving")
		}
	}

	back := m.projectActionCmd("unarchive", "api")()
	if savedBack := back.(projectSavedMsg); savedBack.err != nil {
		t.Fatalf("restoring api failed: %v", savedBack.err)
	}
	active, _ = m.database.ListProjects()
	found := false
	for _, p := range active {
		if p.Name == "api" {
			found = true
		}
	}
	if !found {
		t.Error("api did not return to the active list after restoring")
	}
}
