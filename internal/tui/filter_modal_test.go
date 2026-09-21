package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// ctrlR arma el keypress de reset del modal de filtros.
func ctrlR() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
}

// filterOptionIndex busca una opción dentro de la lista de un campo.
func filterOptionIndex(opts []string, want string) int {
	for i, o := range opts {
		if o == want {
			return i
		}
	}
	return -1
}

// TestFilterStatusOptionsScopedToProject verifica que con un proyecto
// seleccionado el selector de estado ofrezca sólo los estados de ese proyecto.
func TestFilterStatusOptionsScopedToProject(t *testing.T) {
	m := newTestModel(t)

	m.filterProject = "api"
	opts := m.filterFieldOptions(filterFieldStatus)
	// api usa el workflow por defecto.
	for _, want := range append([]string{"all"}, model.DefaultWorkflow...) {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("con api seleccionado falta %q en %v", want, opts)
		}
	}
	// Ningún estado que no pertenezca a api.
	if filterOptionIndex(opts, "reviewing") < 0 {
		t.Errorf("api debería ofrecer reviewing: %v", opts)
	}

	// web define un workflow propio más corto.
	m.filterProject = "web"
	opts = m.filterFieldOptions(filterFieldStatus)
	for _, want := range []string{"all", "todo", "doing", "done"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("con web seleccionado falta %q en %v", want, opts)
		}
	}
	for _, absent := range []string{"backlog", "reviewing", "cancelled"} {
		if filterOptionIndex(opts, absent) >= 0 {
			t.Errorf("con web seleccionado no debería ofrecerse %q: %v", absent, opts)
		}
	}
}

// TestFilterStatusClearedOnProjectSwitch verifica que al cambiar de proyecto
// se limpie el filtro de estado si ese estado no existe en el nuevo contexto.
func TestFilterStatusClearedOnProjectSwitch(t *testing.T) {
	m := newTestModel(t)

	// api tiene "reviewing"; web no.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "reviewing")
	if m.filterStatus != "reviewing" {
		t.Fatalf("precondición: filterStatus = %q, want reviewing", m.filterStatus)
	}

	m.filterApplySelection(filterFieldProject, "web")
	if m.filterStatus != "" {
		t.Errorf("al pasar a web, reviewing no existe: filterStatus = %q, want \"\"", m.filterStatus)
	}

	// Un estado común sobrevive el cambio.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "doing")
	m.filterApplySelection(filterFieldProject, "web")
	if m.filterStatus != "doing" {
		t.Errorf("doing existe en web: filterStatus = %q, want doing", m.filterStatus)
	}

	// Volver a "all projects": sólo sobrevive lo que está en la intersección.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "cancelled")
	m.filterApplySelection(filterFieldProject, "all")
	if m.filterStatus != "" {
		t.Errorf("cancelled no está en todos: filterStatus = %q, want \"\"", m.filterStatus)
	}
}

// TestFilterStatusOptionsIncludeAllActiveAndAll verifica que el filtro de
// estado ofrezca los dos modos agregados además de los estados del proyecto:
// "all active" (default, sin terminales) y "all" (sin restricción).
func TestFilterStatusOptionsIncludeAllActiveAndAll(t *testing.T) {
	m := newTestModel(t)

	opts := m.filterFieldOptions(filterFieldStatus)
	for _, want := range []string{"all active", "all"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("faltan opciones de estado: %q no está en %v", want, opts)
		}
	}

	if got := m.filterCurrentValue(filterFieldStatus); got != "all active" {
		t.Errorf("default del filtro de estado = %q, want all active", got)
	}
}

// TestStatusFilterAllActiveExcludesTerminal verifica que el default "all active"
// deje fuera done/cancelled, y que "all" las incluya de nuevo.
func TestStatusFilterAllActiveExcludesTerminal(t *testing.T) {
	m := newTestModel(t)
	markFirstDone(t, m)

	active := len(m.filteredTasks())

	m.filterApplySelection(filterFieldStatus, "all")
	all := len(m.filteredTasks())

	if all != active+1 {
		t.Errorf("all debe incluir la tarea done: active=%d all=%d", active, all)
	}
}

// TestFilterStatusOptionsAllProjectsIsIntersection verifica que sin proyecto
// seleccionado el selector sólo ofrezca estados comunes a todos los proyectos.
func TestFilterStatusOptionsAllProjectsIsIntersection(t *testing.T) {
	m := newTestModel(t)

	opts := m.filterFieldOptions(filterFieldStatus)
	// api (default) ∩ web ([todo,doing,done]) = [todo,doing,done].
	for _, want := range []string{"all", "todo", "doing", "done"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("intersección: falta %q en %v", want, opts)
		}
	}
	for _, absent := range []string{"backlog", "reviewing", "cancelled"} {
		if filterOptionIndex(opts, absent) >= 0 {
			t.Errorf("intersección no debería incluir %q (no está en todos): %v", absent, opts)
		}
	}
}

// --- Modal UX ---

// TestFilterModalOpenResetsSearchAndCursor verifica el estado inicial al abrir.
func TestFilterModalOpenResetsSearchAndCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	if !m.filterOpen {
		t.Fatal("/ debe abrir el modal de filtros")
	}
	if m.filterFieldIdx != filterFieldProject {
		t.Errorf("foco inicial = %d, want project", m.filterFieldIdx)
	}
	if m.filterSearch != "" {
		t.Errorf("búsqueda inicial = %q, want vacía", m.filterSearch)
	}
	if m.filterOptionIdx != 0 {
		t.Errorf("cursor inicial = %d, want 0 (all)", m.filterOptionIdx)
	}
}

// TestFilterModalCursorNavigation verifica ↑↓ sobre el listado de opciones.
func TestFilterModalCursorNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	m, _ = press(m, "down")
	if m.filterOptionIdx != 1 {
		t.Errorf("tras down: cursor = %d, want 1", m.filterOptionIdx)
	}
	m, _ = press(m, "up")
	if m.filterOptionIdx != 0 {
		t.Errorf("tras up: cursor = %d, want 0", m.filterOptionIdx)
	}
	// up desde 0 vuelve al final (wrap).
	m, _ = press(m, "up")
	if m.filterOptionIdx != len(m.filterFieldOptions(filterFieldProject))-1 {
		t.Errorf("up desde 0 debe envolver, cursor = %d", m.filterOptionIdx)
	}
}

// TestFilterModalFuzzySearchApplies verifica que escribir filtre las opciones y
// Enter aplique el primer match y avance de campo.
func TestFilterModalFuzzySearchApplies(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	m, _ = press(m, "w") // filtra Project a [web, all]
	opts := m.filterVisibleOptions()
	if len(opts) == 0 || opts[0] != "web" {
		t.Fatalf("opciones = %v, want web primero", opts)
	}
	m, _ = press(m, "enter")
	if m.filterProject != "web" {
		t.Errorf("project = %q, want web", m.filterProject)
	}
	if m.filterFieldIdx != filterFieldStatus {
		t.Errorf("enter debe avanzar a status, got %d", m.filterFieldIdx)
	}
}

// TestFilterModalEnterLastFieldCloses verifica que Enter en Tag cierre el modal.
func TestFilterModalEnterLastFieldCloses(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")
	for i := 0; i < 4; i++ {
		m, _ = press(m, "tab")
	}
	if m.filterFieldIdx != filterFieldTag {
		t.Fatalf("campo = %d, want tag", m.filterFieldIdx)
	}
	m, _ = press(m, "enter")
	if m.filterOpen {
		t.Error("Enter en el último campo debe cerrar el modal")
	}
}

// TestFilterModalReset verifica que Ctrl+R vuelva todo a su default, incluido
// el estado "all active".
func TestFilterModalReset(t *testing.T) {
	m := newTestModel(t)
	m.filterProject = "api"
	m.filterStatus = "doing"
	m.filterAssignee = "@juan"
	m.filterPriority = 3
	m.filterTag = "bug"

	m, _ = press(m, "/")
	m, _ = send(m, ctrlR())

	if m.filterProject != "" || m.filterAssignee != "" || m.filterTag != "" {
		t.Errorf("reset dejó filtros: project=%q assignee=%q tag=%q",
			m.filterProject, m.filterAssignee, m.filterTag)
	}
	if m.filterPriority != -1 {
		t.Errorf("priority = %d, want -1", m.filterPriority)
	}
	if m.filterStatus != statusFilterAllActive {
		t.Errorf("status = %q, want %q", m.filterStatus, statusFilterAllActive)
	}
}

// TestFilterModalCycleRemainsLive verifica que ←→ siga aplicando en vivo.
func TestFilterModalCycleRemainsLive(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")
	m, _ = press(m, "right") // all -> api
	if m.filterProject != "api" {
		t.Errorf("project = %q, want api", m.filterProject)
	}
}

// TestFilterModalRenders verifica que el modal muestre todos los campos, el
// conteo en vivo y no desborde el ancho de la terminal.
func TestFilterModalRenders(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Filters", "Project", "Status", "Assignee", "Priority", "Tag"} {
		if !strings.Contains(out, want) {
			t.Errorf("el modal no muestra %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("línea de ancho %d supera el ancho %d: %q", w, m.width, line)
		}
	}
}
