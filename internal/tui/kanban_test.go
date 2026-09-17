package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRenderKanbanColumnsSpaced verifica que las columnas no queden pegadas:
// los bordes de una columna y la siguiente deben estar separados por un gap.
func TestRenderKanbanColumnsSpaced(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.width = 140

	plain := ansi.Strip(m.renderKanban(m.height))

	touching := []string{
		"╮╭", "╮╔", "╔╭", "╔╔",
		"╯╰", "╯╚", "╚╰", "╚╚",
	}
	for _, pair := range touching {
		if strings.Contains(plain, pair) {
			t.Errorf("los bordes de columnas se tocan: se encontró %q", pair)
		}
	}
}

// TestRenderKanbanFitsWidth verifica que el board no exceda el ancho de la
// terminal y que aproveche todo el ancho disponible.
func TestRenderKanbanFitsWidth(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.width = 140

	out := m.renderKanban(m.height)
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("línea %d mide %d, excede el ancho %d", i, w, m.width)
		}
	}
}

// TestKanbanColumnWidths verifica el reparto de ancho: se respeta el mínimo,
// no se deja espacio muerto y el sobrante se reparte parejo.
func TestKanbanColumnWidths(t *testing.T) {
	mins := []int{16, 12, 20, 15, 13}
	avail := 128

	widths := kanbanColumnWidths(mins, avail)
	total := 0
	for i, w := range widths {
		if w < mins[i] {
			t.Errorf("columna %d: ancho %d menor al mínimo %d", i, w, mins[i])
		}
		total += w
	}
	total += kanbanGap * (len(widths) - 1)
	if total != avail {
		t.Errorf("ancho total = %d, want %d (no aprovecha el espacio)", total, avail)
	}
}

// TestKanbanColumnWidthsTooNarrow verifica que con poco espacio se respeten los
// mínimos sin repartir sobrante inexistente.
func TestKanbanColumnWidthsTooNarrow(t *testing.T) {
	mins := []int{20, 20, 20, 20, 20, 20}

	widths := kanbanColumnWidths(mins, 60)
	for i, w := range widths {
		if w != mins[i] {
			t.Errorf("columna %d: ancho %d, want %d", i, w, mins[i])
		}
	}
}

// TestKanbanHiddenToggle verifica que H tenga efecto en el board: con tareas
// ocultas activas la columna done se vacía; con H apagado vuelven a mostrarse.
func TestKanbanHiddenToggle(t *testing.T) {
	m := newTestModel(t)
	markFirstDone(t, m)
	m, _ = press(m, "2")

	done := kanbanColumnIndex(m, "done")
	if done < 0 {
		t.Fatal("no hay columna done")
	}

	if n := len(m.kanbanColumns()[done].tasks); n != 0 {
		t.Errorf("con hidden activo la columna done debe estar vacía, tiene %d", n)
	}

	m, _ = press(m, "H")

	if n := len(m.kanbanColumns()[done].tasks); n != 1 {
		t.Errorf("con hidden apagado la columna done debe mostrar 1 tarea, tiene %d", n)
	}
}

// TestKanbanHiddenCardsAreNavigable verifica que las tarjetas ocultas, al
// mostrarse, sean seleccionables. Antes el render y la navegación usaban listas
// distintas, así que la columna done mostraba tarjetas inaccesibles.
func TestKanbanHiddenCardsAreNavigable(t *testing.T) {
	m := newTestModel(t)
	markFirstDone(t, m)
	m, _ = press(m, "2")
	m, _ = press(m, "H") // mostrar done

	done := kanbanColumnIndex(m, "done")
	if done < 0 {
		t.Fatal("no hay columna done")
	}
	tasks := m.kanbanColumns()[done].tasks
	if len(tasks) == 0 {
		t.Fatal("la columna done debería mostrar la tarea marcada")
	}

	m.kanbanCol = done
	m.kanbanRow = 0

	got := m.selectedTask()
	if got == nil {
		t.Fatal("la tarea de la columna done debería estar seleccionable")
	}
	if got.ID != tasks[0].ID {
		t.Errorf("selectedTask = %d, want %d", got.ID, tasks[0].ID)
	}

	m, _ = press(m, "enter")
	if !m.detailOpen || m.detailTask == nil {
		t.Fatal("Enter debería abrir el detalle de la tarea seleccionada")
	}
	if m.detailTask.ID != got.ID {
		t.Errorf("detalle = %d, want %d", m.detailTask.ID, got.ID)
	}
}

// TestKanbanAdvanceUsesProjectWorkflow verifica que "s" avance según el
// workflow del proyecto de la tarea, no según el merge. El merge pondría
// "reviewing" como siguiente de "doing", pero web no tiene ese estado y el
// move sería rechazado en silencio.
func TestKanbanAdvanceUsesProjectWorkflow(t *testing.T) {
	m := newTestModel(t)
	if _, err := m.database.CreateTask("web", "Deploy", "", "@juan", 1, "doing"); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()

	m, _ = press(m, "2") // kanban view

	col := kanbanColumnIndex(m, "doing")
	if col < 0 {
		t.Fatal("no hay columna doing")
	}
	colTasks := m.tasksInColumn("doing")
	row, id := -1, int64(0)
	for j, task := range colTasks {
		if task.ProjectName == "web" {
			row, id = j, task.ID
			break
		}
	}
	if row < 0 {
		t.Fatal("no se encontró una tarea web en doing")
	}
	m.kanbanCol, m.kanbanRow = col, row

	_, cmd := press(m, "s")
	if cmd == nil {
		t.Fatal("s debería emitir un comando")
	}
	cmd()

	got, err := m.database.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	// web = [todo,doing,done]: el siguiente de doing es done, no reviewing.
	if got.Status != "done" {
		t.Errorf("status = %q, want done (workflow de web)", got.Status)
	}
}

// markFirstDone marca la primera tarea del fixture como done y recarga el modelo.
func markFirstDone(t *testing.T, m *Model) {
	t.Helper()
	if len(m.tasks) == 0 {
		t.Fatal("el fixture no tiene tareas")
	}
	if _, err := m.database.DoneTask(m.tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()
}

// kanbanColumnIndex devuelve el índice de la columna del estado dado, o -1.
func kanbanColumnIndex(m *Model, status string) int {
	for i, c := range m.kanbanColumns() {
		if c.status == status {
			return i
		}
	}
	return -1
}

// TestRenderKanbanShowsFilterHeader verifica que el board muestre la misma
// cabecera de filtros que la List, con el valor activo.
func TestRenderKanbanShowsFilterHeader(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.filterProject = "api"

	plain := ansi.Strip(m.renderKanban(m.height))
	for _, want := range []string{"Project:", "Status:", "Assignee:", "Priority:", "api"} {
		if !strings.Contains(plain, want) {
			t.Errorf("la cabecera del Kanban no contiene %q:\n%s", want, plain)
		}
	}
}

// TestKanbanRespectsProjectFilter verifica que la cabecera no mienta: el board
// sólo muestra tareas del proyecto filtrado.
func TestKanbanRespectsProjectFilter(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.filterProject = "api"

	for _, c := range m.kanbanColumns() {
		for _, task := range c.tasks {
			if task.ProjectName != "api" {
				t.Errorf("columna %s: tarea %d es de %q, want api", c.status, task.ID, task.ProjectName)
			}
		}
	}
}

// TestRenderKanbanRespectsHeight verifica que un título largo se recorte en vez
// de wrappear: si wrappease, la columna crecería y el board excedería el alto.
func TestRenderKanbanRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.width = 100
	for i := range m.tasks {
		m.tasks[i].Title = strings.Repeat("TITULO-LARGO ", 12)
	}

	const budget = 14
	out := m.renderKanban(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("alto = %d, excede el presupuesto %d", n, budget)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("línea %d mide %d, excede el ancho %d", i, w, m.width)
		}
	}
}
