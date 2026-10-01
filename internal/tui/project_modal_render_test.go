package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// El modal de proyectos tiene un formulario de tres campos, una confirmación de
// archivo y un camino de guardado que distingue crear de editar. Los tests que
// lo cubrían miraban si el modal se abría; lo que importa es qué campo está
// enfocado, qué texto se envía y qué hace el modelo con la respuesta.

// projectModalRender devuelve el modal de proyectos sin colores.
func projectModalRender(t *testing.T, m *Model) string {
	t.Helper()
	m.width = 120
	return ansi.Strip(m.renderProjectModal(""))
}

// newProjectModel abre el modal de proyectos en modo creación, con el campo
// indicado enfocado.
func newProjectModel(t *testing.T, field int) *Model {
	t.Helper()
	m := newTestModel(t)
	m.projectModalOpen = true
	m.projectModalEdit = false
	m.projectModalField = field
	m.projectNameInput = "nuevo"
	m.projectWorkflowInput = ""
	m.projectListOrderInput = ""
	return m
}

// El cursor va en el campo enfocado, y sólo en él. Con tres campos, mover el foco
// da la vuelta completa.
func TestProjectModalCursorFollowsFocusedField(t *testing.T) {
	tests := []struct {
		name  string
		field int
		want  string
	}{
		{"nombre", 0, "nuevo_"},
		{"workflow", 1, "nuevo"},
		{"list order", 2, "nuevo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newProjectModel(t, tt.field)
			m.projectWorkflowInput = "todo,done"
			m.projectListOrderInput = "todo"

			out := projectModalRender(t, m)
			if !strings.Contains(out, tt.want+" ") {
				t.Errorf("no se ve %q con el cursor en el campo %d:\n%s", tt.want, tt.field, out)
			}
			// El cursor es un guion bajo pegado al valor, no un campo aparte.
			if strings.Count(out, "_") != 1 {
				t.Errorf("hay %d cursores, want exactamente 1:\n%s", strings.Count(out, "_"), out)
			}
		})
	}
}

// Mover el foco con tab recorre los tres campos y vuelve al primero.
func TestProjectModalFieldCycles(t *testing.T) {
	from := []int{0, 1, 2}
	want := []int{1, 2, 0}
	for i, f := range from {
		m := newProjectModel(t, f)
		next, _ := press(m, "tab")
		if next.projectModalField != want[i] {
			t.Errorf("tab desde el campo %d lleva al %d, want %d", f, next.projectModalField, want[i])
		}
	}
}

// Crear con workflow vacío deja que la base de datos ponga el de por defecto;
// mandarle un workflow inválido, en cambio, es un error que vuelve al modal.
func TestProjectSubmitEmptyWorkflowUsesDefault(t *testing.T) {
	m := newProjectModel(t, 0)
	m.projectWorkflowInput = ""
	m.projectListOrderInput = ""

	msg := m.saveProjectCmd(false, "", "nuevo", m.projectWorkflowInput, m.projectListOrderInput)()
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("mensaje %T, want projectSavedMsg", msg)
	}
	if saved.err != nil {
		t.Fatalf("crear con workflow vacío falló: %v", saved.err)
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create", saved.action)
	}

	p, err := m.database.GetProject("nuevo")
	if err != nil {
		t.Fatalf("el proyecto no se creó: %v", err)
	}
	if len(p.Workflow) == 0 {
		t.Error("el proyecto quedó sin workflow, want el de por defecto")
	}
}

func TestProjectSubmitInvalidWorkflowReportsError(t *testing.T) {
	m := newProjectModel(t, 0)
	m.projectWorkflowInput = "todo,roto,roto"

	msg := m.saveProjectCmd(false, "", "nuevo", m.projectWorkflowInput, m.projectListOrderInput)()
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("mensaje %T, want projectSavedMsg", msg)
	}
	if saved.err == nil {
		t.Fatal("un workflow con duplicados no dio error")
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create para que el modal se reabra", saved.action)
	}
	if _, err := m.database.GetProject("nuevo"); err == nil {
		t.Error("el proyecto se creó pese al error")
	}
}

// Un list order inválido también es un error, y con la misma acción.
func TestProjectSubmitInvalidListOrderReportsError(t *testing.T) {
	m := newProjectModel(t, 0)
	m.projectWorkflowInput = "todo,done"
	m.projectListOrderInput = "a,b,a"

	saved := m.saveProjectCmd(false, "", "nuevo", m.projectWorkflowInput, m.projectListOrderInput)().(projectSavedMsg)
	if saved.err == nil {
		t.Fatal("un list order con duplicados no dio error")
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create", saved.action)
	}
}

// Un proyecto que ya existe da error, y el nombre en el mensaje es el suyo.
func TestProjectSubmitDuplicateReportsError(t *testing.T) {
	m := newTestModel(t)
	m.projectModalOpen = true
	m.projectModalEdit = false

	// "api" ya existe en el fixture.
	saved := m.saveProjectCmd(false, "", "api", "todo,done", "")().(projectSavedMsg)
	if saved.err == nil {
		t.Fatal("crear un proyecto que ya existe no dio error")
	}
	if saved.action != "create" {
		t.Errorf("action = %q, want create", saved.action)
	}
}

// Un error al guardar reabre el modal para no perder lo escrito; un error al
// archivar o restaurar no, porque esos no tienen nada escrito detrás.
func TestProjectSavedErrorReopensModalOnlyForCreateAndEdit(t *testing.T) {
	tests := []struct {
		action   string
		wantOpen bool
	}{
		{"create", true},
		{"edit", true},
		{"archive", false},
		{"unarchive", false},
		{"otro", false},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			m := newTestModel(t)
			m.projectModalOpen = false

			nextMsg, _ := m.handleProjectSaved(projectSavedMsg{err: errAccionFallida, action: tt.action})
			next := nextMsg.(Model)
			if next.projectModalOpen != tt.wantOpen {
				t.Errorf("tras un error en %q el modal quedó %v, want %v", tt.action, next.projectModalOpen, tt.wantOpen)
			}
			if next.toast == "" {
				t.Error("no se avisa del error con un toast")
			}
		})
	}
}

// Archivar y restaurar son operaciones distintas, y la confirmación lo dice.
func TestProjectConfirmDistinguishesArchiveFromUnarchive(t *testing.T) {
	archivo := newTestModel(t)
	archivo.confirmOpen = true
	archivo.confirmAction = "archive"
	archivo.confirmProject = "api"

	out := ansi.Strip(archivo.renderConfirmModal(""))
	if !strings.Contains(out, "Archive project") {
		t.Errorf("la confirmación de archivo no lo dice:\n%s", out)
	}
	if strings.Contains(out, "Restore project") {
		t.Errorf("la confirmación de archivo dice restaurar:\n%s", out)
	}

	restaurar := newTestModel(t)
	restaurar.confirmOpen = true
	restaurar.confirmAction = "unarchive"
	restaurar.confirmProject = "api"

	out2 := ansi.Strip(restaurar.renderConfirmModal(""))
	if !strings.Contains(out2, "Restore project") {
		t.Errorf("la confirmación de restauración no lo dice:\n%s", out2)
	}
}

// selectedDashProject es nil sin selección y con la lista vacía, y el proyecto
// en cuanto la hay.
func TestSelectedDashProjectBounds(t *testing.T) {
	m := newTestModel(t)

	m.dashProjectIdx = -1
	if p := m.selectedDashProject(); p != nil {
		t.Errorf("con índice -1 devuelve %v, want nil", p.Name)
	}

	m.projects = nil
	m.dashProjectIdx = 0
	if p := m.selectedDashProject(); p != nil {
		t.Errorf("sin proyectos devuelve %v, want nil", p.Name)
	}

	m.projects, _ = m.database.ListProjects()
	m.dashProjectIdx = len(m.projects) + 5
	if p := m.selectedDashProject(); p != nil {
		t.Errorf("índice fuera de rango devuelve %v, want nil", p.Name)
	}

	m.dashProjectIdx = 0
	p := m.selectedDashProject()
	if p == nil {
		t.Fatal("con un proyecto en la posición 0 devuelve nil")
	}
	if p.Name != m.projects[0].Name {
		t.Errorf("devuelve %q, want %q", p.Name, m.projects[0].Name)
	}
}
