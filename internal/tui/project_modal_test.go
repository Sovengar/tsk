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
		t.Fatal("i en Dashboard debe abrir el modal de proyecto")
	}
	if m.newTaskOpen {
		t.Error("i en Dashboard NO debe abrir el modal de tarea")
	}
}

func TestListIStillOpensTaskModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1") // list
	m, _ = press(m, "i")

	if !m.newTaskOpen {
		t.Fatal("i en List debe abrir el modal de tarea")
	}
	if m.projectModalOpen {
		t.Error("i en List no debe abrir el modal de proyecto")
	}
}

func TestDashboardArchiveConfirmCancel(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m, _ = press(m, "d")

	if !m.confirmOpen {
		t.Fatal("d debe abrir la confirmación de archivado")
	}
	if m.confirmAction != "archive" {
		t.Errorf("action = %q, want archive", m.confirmAction)
	}

	m, _ = press(m, "esc")
	if m.confirmOpen {
		t.Error("esc debe cancelar la confirmación")
	}
}

func TestDashboardToggleArchived(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	m, _ = press(m, "A")
	if !m.showArchived {
		t.Fatal("A debe activar la vista de archivados")
	}
	// Sin archivados, la lista visible está vacía.
	if len(m.dashProjectList()) != 0 {
		t.Errorf("archived list = %v, want empty", m.dashProjectList())
	}

	m, _ = press(m, "A")
	if m.showArchived {
		t.Error("A debe volver a la vista de activos")
	}
}

func TestProjectSavedErrorReopensModalAndToasts(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m, _ = press(m, "i")
	m.projectNameInput = "api" // ya existe

	next, cmd := m.handleProjectSaved(projectSavedMsg{
		err:    errors.New("project already exists: api"),
		action: "create",
	})
	mm := asModel(next)

	if !mm.projectModalOpen {
		t.Error("el modal debe reabrirse ante error de creación")
	}
	if mm.toast == "" || mm.toastKind != "error" {
		t.Errorf("toast = %q/%q, want error", mm.toast, mm.toastKind)
	}
	if cmd == nil {
		t.Error("se esperaba el comando de expiración del toast")
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
		t.Error("se esperaba el batch de recarga + toast")
	}
}

func TestToastExpiryRespectsSequence(t *testing.T) {
	m := newTestModel(t)
	m.setToast("hola", "info")
	seq := m.toastSeq

	next, _ := m.Update(toastExpiredMsg{seq: seq - 1})
	mm := asModel(next)
	if mm.toast == "" {
		t.Error("un tick viejo no debe limpiar el toast actual")
	}

	next, _ = mm.Update(toastExpiredMsg{seq: seq})
	mm = asModel(next)
	if mm.toast != "" {
		t.Error("el tick con la secuencia actual debe limpiar el toast")
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
		t.Error("se esperaba toast tras archivar")
	}

	// Recargar proyectos y verificar el efecto.
	next, _ = mm.Update(mm.loadProjects()())
	mm = asModel(next)

	if len(mm.projects) != 1 || mm.projects[0].Name != "web" {
		t.Errorf("activos tras archivar = %v, want [web]", mm.projects)
	}
	if len(mm.archivedProjects) != 1 || mm.archivedProjects[0].Name != "api" {
		t.Errorf("archivados = %v, want [api]", mm.archivedProjects)
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

	// El modal tiene 3 campos: name -> workflow -> list_order -> name.
	m, _ = press(m, "tab")
	if m.projectModalField != 1 {
		t.Errorf("field tras tab = %d, want 1", m.projectModalField)
	}
	m, _ = press(m, "tab")
	if m.projectModalField != 2 {
		t.Errorf("field tras segundo tab = %d, want 2", m.projectModalField)
	}
	m, _ = press(m, "tab")
	if m.projectModalField != 0 {
		t.Errorf("field tras tercer tab = %d, want 0 (wrap)", m.projectModalField)
	}
}

func TestRenderProjectModal(t *testing.T) {
	m := newTestModel(t)
	m.projectModalOpen = true
	out := m.renderProjectModal("base")
	if !strings.Contains(out, "New Project") {
		t.Errorf("modal no contiene título: %q", out)
	}
}

func TestRenderConfirmModal(t *testing.T) {
	m := newTestModel(t)
	m.confirmOpen = true
	m.confirmAction = "archive"
	m.confirmProject = "api"
	out := m.renderConfirmModal("base")
	if !strings.Contains(out, "api") {
		t.Errorf("confirmación no menciona el proyecto: %q", out)
	}
}
