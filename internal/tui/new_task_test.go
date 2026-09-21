package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// TestNewTaskOpenDefaults verifica el estado inicial del alta: foco en
// Priority, assignee "Me" y overlay activo.
func TestNewTaskOpenDefaults(t *testing.T) {
	m := newTestModel(t)

	m, _ = press(m, "i")
	if !m.newTaskOpen {
		t.Fatal("i debe abrir el modal de nueva tarea")
	}
	if m.newTaskFieldIdx != newTaskFieldPriority {
		t.Errorf("foco inicial = %d, want priority", m.newTaskFieldIdx)
	}
	if m.newTaskAssignee != "Me" {
		t.Errorf("assignee inicial = %q, want Me", m.newTaskAssignee)
	}
	if m.newTaskPriority != model.PriorityLow {
		t.Errorf("priority inicial = %d, want low", m.newTaskPriority)
	}
	if m.newTaskProject != "api" {
		t.Errorf("project = %q, want api", m.newTaskProject)
	}
	if m.overlayKind() != overlayNewTask {
		t.Errorf("overlay = %v, want overlayNewTask", m.overlayKind())
	}
	// "Me" coincide exacto: no debe sugerirse a sí mismo.
	if got := m.assigneeSuggestions(); len(got) != 0 {
		t.Errorf("con assignee Me no debe haber sugerencias, got %v", got)
	}
}

// TestNewTaskTabNavigation verifica que Tab/Shift+Tab recorren los campos en
// orden y dan la vuelta sin perderse.
func TestNewTaskTabNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	order := []int{
		newTaskFieldTitle,
		newTaskFieldDescription,
		newTaskFieldAssignee,
		newTaskFieldTags,
		newTaskFieldPriority,
	}
	for _, want := range order {
		m, _ = press(m, "tab")
		if m.newTaskFieldIdx != want {
			t.Fatalf("tras tab: campo = %d, want %d", m.newTaskFieldIdx, want)
		}
	}

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.newTaskFieldIdx != newTaskFieldTags {
		t.Errorf("shift+tab desde priority: campo = %d, want tags", m.newTaskFieldIdx)
	}
}

// TestNewTaskPriorityKeys verifica la selección de prioridad con 1-4 y flechas.
func TestNewTaskPriorityKeys(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	m, _ = press(m, "3")
	if m.newTaskPriority != 3 {
		t.Errorf("priority = %d, want 3", m.newTaskPriority)
	}
	m, _ = press(m, "left")
	if m.newTaskPriority != 2 {
		t.Errorf("tras left: priority = %d, want 2", m.newTaskPriority)
	}
	m, _ = press(m, "right")
	if m.newTaskPriority != 3 {
		t.Errorf("tras right: priority = %d, want 3", m.newTaskPriority)
	}
	m, _ = press(m, "1")
	if m.newTaskPriority != 1 {
		t.Errorf("tras 1: priority = %d, want 1", m.newTaskPriority)
	}
}

// TestNewTaskTypingOnPriorityJumpsToTitle verifica que tipear una letra sobre el
// selector de prioridad salta al título y la inserta.
func TestNewTaskTypingOnPriorityJumpsToTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	m, _ = press(m, "H")
	if m.newTaskFieldIdx != newTaskFieldTitle {
		t.Errorf("tapear sobre priority debe enfocar Title, got %d", m.newTaskFieldIdx)
	}
	if m.newTaskTitle != "H" {
		t.Errorf("title = %q, want H", m.newTaskTitle)
	}
}

// TestNewTaskRequiresTitle verifica la validación inline sin cerrar el modal.
func TestNewTaskRequiresTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	m, _ = press(m, "enter") // priority -> title
	if m.newTaskFieldIdx != newTaskFieldTitle {
		t.Fatalf("enter en priority debe avanzar a Title, got %d", m.newTaskFieldIdx)
	}
	m, _ = press(m, "enter") // submit sin título
	if !m.newTaskOpen {
		t.Fatal("el modal no debe cerrarse sin título")
	}
	if m.newTaskErr == "" {
		t.Error("se esperaba error inline de título requerido")
	}
}

// TestNewTaskSubmitsFromTitle verifica que Enter en Title crea y cierra,
// dejando el feedback del toast.
func TestNewTaskSubmitsFromTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")
	m, _ = press(m, "tab") // title

	for _, ch := range []string{"B", "u", "g"} {
		m, _ = press(m, ch)
	}
	if m.newTaskTitle != "Bug" {
		t.Fatalf("title = %q, want Bug", m.newTaskTitle)
	}

	m, cmd := press(m, "enter")
	if m.newTaskOpen {
		t.Error("Enter en Title debe cerrar el modal")
	}
	if cmd == nil {
		t.Error("se esperaba cmd de creación")
	}
	if m.toast != "Task created" {
		t.Errorf("toast = %q, want 'Task created'", m.toast)
	}
}

// TestNewTaskDescriptionUsesInlineEditor verifica que la descripción se edita
// con el textarea embebido y que Enter inserta salto en lugar de crear.
func TestNewTaskDescriptionUsesInlineEditor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")
	m, _ = press(m, "tab") // title
	m, _ = press(m, "tab") // description

	if m.newTaskFieldIdx != newTaskFieldDescription {
		t.Fatalf("campo = %d, want description", m.newTaskFieldIdx)
	}

	m, _ = press(m, "h")
	if got := m.newTaskTextarea.Value(); got != "h" {
		t.Fatalf("textarea = %q, want h", got)
	}

	m, _ = press(m, "enter")
	if !m.newTaskOpen {
		t.Fatal("Enter en Description no debe crear")
	}
	if !strings.Contains(m.newTaskTextarea.Value(), "\n") {
		t.Errorf("Enter debe insertar salto de línea, got %q", m.newTaskTextarea.Value())
	}
}

// TestNewTaskAssigneeAutocomplete verifica el filtrado fuzzy, la selección con
// ↓ y la completación con Enter sin crear la tarea.
func TestNewTaskAssigneeAutocomplete(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")
	m, _ = press(m, "tab") // title
	m, _ = press(m, "tab") // description
	m, _ = press(m, "tab") // assignee

	// Limpiar el "Me" por defecto.
	m, _ = press(m, "backspace")
	m, _ = press(m, "backspace")
	if m.newTaskAssignee != "" {
		t.Fatalf("assignee = %q, want vacío", m.newTaskAssignee)
	}

	m, _ = press(m, "@")
	m, _ = press(m, "m")
	suggs := m.assigneeSuggestions()
	if len(suggs) == 0 || suggs[0] != "@maria" {
		t.Fatalf("sugerencias = %v, want @maria primero", suggs)
	}

	m, _ = press(m, "down")
	if m.newTaskAssigneeSuggIdx != 0 {
		t.Fatalf("suggIdx = %d, want 0", m.newTaskAssigneeSuggIdx)
	}
	m, _ = press(m, "enter")
	if m.newTaskAssignee != "@maria" {
		t.Errorf("assignee = %q, want @maria", m.newTaskAssignee)
	}
	if !m.newTaskOpen {
		t.Error("Enter sobre una sugerencia debe completar, no crear")
	}
	// Al quedar el nombre completo no debe repetirse como sugerencia.
	if got := m.assigneeSuggestions(); len(got) != 0 {
		t.Errorf("match exacto no debe sugerirse: %v", got)
	}
}

// goNewTaskTags abre el alta y enfoca el campo Tags.
func goNewTaskTags(t *testing.T, m *Model) *Model {
	t.Helper()
	m, _ = press(m, "i")
	for i := 0; i < 4; i++ {
		m, _ = press(m, "tab")
	}
	if m.newTaskFieldIdx != newTaskFieldTags {
		t.Fatalf("campo = %d, want tags", m.newTaskFieldIdx)
	}
	return m
}

// TestNewTaskTagsAddAndRemove verifica agregar con Enter y coma, y borrar la
// última tag con backspace sobre el input vacío.
func TestNewTaskTagsAddAndRemove(t *testing.T) {
	m := newTestModel(t)
	m = goNewTaskTags(t, m)

	for _, ch := range []string{"b", "a", "c", "k"} {
		m, _ = press(m, ch)
	}
	m, _ = press(m, "enter")
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "back" {
		t.Fatalf("tags = %v, want [back]", m.newTaskTags)
	}
	if m.newTaskTagInput != "" {
		t.Errorf("input = %q, want vacío tras commit", m.newTaskTagInput)
	}

	m, _ = press(m, "f")
	m, _ = press(m, ",")
	if len(m.newTaskTags) != 2 || m.newTaskTags[1] != "f" {
		t.Fatalf("tags = %v, want [back f]", m.newTaskTags)
	}

	m, _ = press(m, "backspace")
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "back" {
		t.Errorf("tras backspace: tags = %v, want [back]", m.newTaskTags)
	}
}

// TestNewTaskTagsCommitOnTab verifica que salir del campo no pierde la tag a
// medio tipear.
func TestNewTaskTagsCommitOnTab(t *testing.T) {
	m := newTestModel(t)
	m = goNewTaskTags(t, m)

	m, _ = press(m, "x")
	m, _ = press(m, "tab") // tags -> priority
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "x" {
		t.Errorf("tags = %v, want [x] tras salir del campo", m.newTaskTags)
	}
}

// TestNewTaskTagsAutocomplete verifica el filtrado fuzzy y la completación con
// Enter sin crear la tarea.
func TestNewTaskTagsAutocomplete(t *testing.T) {
	m := newTestModel(t)
	m.tasks[0].Tags = []string{"backend", "frontend"}
	m = goNewTaskTags(t, m)

	for _, ch := range []string{"b", "a", "c", "k"} {
		m, _ = press(m, ch)
	}
	suggs := m.tagFieldSuggestions()
	if len(suggs) == 0 || suggs[0] != "backend" {
		t.Fatalf("sugerencias = %v, want backend primero", suggs)
	}

	m, _ = press(m, "down")
	m, _ = press(m, "enter")
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "backend" {
		t.Errorf("tags = %v, want [backend]", m.newTaskTags)
	}
	if !m.newTaskOpen {
		t.Error("Enter sobre una sugerencia de tag no debe crear la tarea")
	}
	// La tag ya elegida no vuelve a sugerirse (aunque el dropdown liste el resto).
	if got := m.tagFieldSuggestions(); containsFold(got, "backend") {
		t.Errorf("tags ya agregadas no deben sugerirse: %v", got)
	}
}

// TestNewTaskModalRenders verifica que el modal muestre todos los campos y no
// desborde el ancho de la terminal.
func TestNewTaskModalRenders(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"New Task", "Priority", "Title", "Description", "Assignee", "Me", "Tags", "optional"} {
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

// TestFuzzyFilter cubre el matcher compartido del autocompletado.
func TestFuzzyFilter(t *testing.T) {
	items := []string{"@maria", "@juan", "Me"}

	if got := fuzzyFilter(items, "mar", 5); len(got) != 1 || got[0] != "@maria" {
		t.Errorf("fuzzyFilter(mar) = %v, want [@maria]", got)
	}

	// Substring con match más temprano gana.
	if got := fuzzyFilter([]string{"@juan", "@maria"}, "a", 5); got[0] != "@maria" {
		t.Errorf("orden por score = %v, want @maria primero", got)
	}

	// Query vacío respeta el orden original y el tope.
	if got := fuzzyFilter(items, "", 2); len(got) != 2 || got[0] != "@maria" {
		t.Errorf("fuzzyFilter('') = %v, want primeros dos en orden", got)
	}

	// Subsecuencia y descarte.
	if _, ok := fuzzyScore("jn", "@juan"); !ok {
		t.Error("jn debería matchear @juan como subsecuencia")
	}
	if _, ok := fuzzyScore("zz", "@juan"); ok {
		t.Error("zz no debería matchear @juan")
	}
}
