package tui

import (
	"errors"
	"strings"
	"testing"

	"tsk/internal/model"
)

func TestDashboardIOpensProjectModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4") // dashboard
	m, _ = press(m, "i")

	if !m.projectModalOpen {
		t.Fatal("i in Dashboard must open the project modal")
	}
	if m.newTaskOpen {
		t.Error("i in Dashboard must NOT open the task modal")
	}
}

func TestListIStillOpensTaskModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1") // list
	m, _ = press(m, "i")

	if !m.newTaskOpen {
		t.Fatal("i in List must open the task modal")
	}
	if m.projectModalOpen {
		t.Error("i in List must not open the project modal")
	}
}

func TestDashboardArchiveConfirmCancel(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m, _ = press(m, "d")

	if !m.confirmOpen {
		t.Fatal("d must open the archive confirmation")
	}
	if m.confirmAction != "archive" {
		t.Errorf("action = %q, want archive", m.confirmAction)
	}

	m, _ = press(m, "esc")
	if m.confirmOpen {
		t.Error("esc must cancel the confirmation")
	}
}

func TestDashboardToggleArchived(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	m, _ = press(m, "A")
	if !m.showArchived {
		t.Fatal("A must enable the archived view")
	}
	// Without archived ones, the visible list is empty.
	if len(m.dashProjectList()) != 0 {
		t.Errorf("archived list = %v, want empty", m.dashProjectList())
	}

	m, _ = press(m, "A")
	if m.showArchived {
		t.Error("A must go back to the active view")
	}
}

func TestProjectSavedErrorReopensModalAndToasts(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m, _ = press(m, "i")
	m.projectNameInput = "api" // already exists

	next, cmd := m.handleProjectSaved(projectSavedMsg{
		err:    errors.New("project already exists: api"),
		action: "create",
	})
	mm := asModel(next)

	if !mm.projectModalOpen {
		t.Error("the modal must reopen on a creation error")
	}
	if mm.toast == "" || mm.toastKind != "error" {
		t.Errorf("toast = %q/%q, want error", mm.toast, mm.toastKind)
	}
	if cmd == nil {
		t.Error("the toast expiry command was expected")
	}
}

func TestProjectSavedSuccessToastsAndSelects(t *testing.T) {
	m := newTestModel(t)

	next, cmd := m.handleProjectSaved(projectSavedMsg{name: "newproj", action: "create"})
	mm := asModel(next)

	if mm.toast == "" || mm.toastKind != "info" {
		t.Errorf("toast = %q/%q, want info", mm.toast, mm.toastKind)
	}
	if mm.pendingSelectName != "newproj" {
		t.Errorf("pendingSelectName = %q, want newproj", mm.pendingSelectName)
	}
	if cmd == nil {
		t.Error("the reload + toast batch was expected")
	}
}

func TestToastExpiryRespectsSequence(t *testing.T) {
	m := newTestModel(t)
	m.setToast("hello", "info")
	seq := m.toastSeq

	next, _ := m.Update(toastExpiredMsg{seq: seq - 1})
	mm := asModel(next)
	if mm.toast == "" {
		t.Error("an old tick must not clear the current toast")
	}

	next, _ = mm.Update(toastExpiredMsg{seq: seq})
	mm = asModel(next)
	if mm.toast != "" {
		t.Error("the tick with the current sequence must clear the toast")
	}
}

func TestArchiveProjectCommandReloads(t *testing.T) {
	m := newTestModel(t)

	msg := m.projectActionCmd("archive", "api")()
	saved, ok := msg.(projectSavedMsg)
	if !ok || saved.err != nil {
		t.Fatalf("archive msg = %#v", msg)
	}

	next, _ := m.Update(saved)
	mm := asModel(next)
	if mm.toast == "" {
		t.Error("a toast was expected after archiving")
	}

	// Reload the projects and check the effect.
	next, _ = mm.Update(mm.loadProjects()())
	mm = asModel(next)

	if len(mm.projects) != 1 || mm.projects[0].Name != "web" {
		t.Errorf("active after archiving = %v, want [web]", mm.projects)
	}
	if len(mm.archivedProjects) != 1 || mm.archivedProjects[0].Name != "api" {
		t.Errorf("archived = %v, want [api]", mm.archivedProjects)
	}
}

func TestOpenProjectModalPrefillsDefaults(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4") // dashboard
	m, _ = press(m, "i")

	if got := m.projectWorkflowInput; got != strings.Join(model.DefaultWorkflow, ",") {
		t.Errorf("workflow input = %q, want default", got)
	}
	if got := m.projectListOrderInput; got != strings.Join(model.DefaultListOrder, ",") {
		t.Errorf("list_order input = %q, want default", got)
	}

	// The modal has 3 fields: name -> workflow -> list_order -> name.
	m, _ = press(m, "tab")
	if m.projectModalField != 1 {
		t.Errorf("field after tab = %d, want 1", m.projectModalField)
	}
	m, _ = press(m, "tab")
	if m.projectModalField != 2 {
		t.Errorf("field after second tab = %d, want 2", m.projectModalField)
	}
	m, _ = press(m, "tab")
	if m.projectModalField != 0 {
		t.Errorf("field after third tab = %d, want 0 (wrap)", m.projectModalField)
	}
}

func TestRenderProjectModal(t *testing.T) {
	m := newTestModel(t)
	m.projectModalOpen = true
	out := m.renderProjectModal("base")
	if !strings.Contains(out, "New Project") {
		t.Errorf("modal does not contain the title: %q", out)
	}
}

func TestRenderConfirmModal(t *testing.T) {
	m := newTestModel(t)
	m.confirmOpen = true
	m.confirmAction = "archive"
	m.confirmProject = "api"
	out := m.renderConfirmModal("base")
	if !strings.Contains(out, "api") {
		t.Errorf("confirmation does not mention the project: %q", out)
	}
}
