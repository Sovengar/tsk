package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestViewSwitching4(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")
	if m.currentView != viewGantt {
		t.Errorf("after 4: view = %v, want gantt", m.currentView)
	}
}

func TestRenderGantt(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	out := m.renderGantt(m.height)
	if out == "" {
		t.Fatal("gantt should not be empty")
	}
	if !strings.Contains(out, "Gantt") {
		t.Errorf("gantt output missing title:\n%s", out)
	}
}

// TestRenderGanttShowsFilterHeader verifica que el Gantt muestre la cabecera
// de filtros sin Status: su proyección sólo incluye tareas activas, por lo que
// un filtro de estado siempre valdría "active".
func TestRenderGanttShowsFilterHeader(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")
	m.filterProject = "api"

	plain := ansi.Strip(m.renderGantt(m.height))
	for _, want := range []string{"Project:", "Assignee:", "Priority:", "api"} {
		if !strings.Contains(plain, want) {
			t.Errorf("la cabecera del Gantt no contiene %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Status:") {
		t.Errorf("la cabecera del Gantt no debe mostrar Status:\n%s", plain)
	}
}

// TestRenderGanttRespectsHeight verifica que la cabecera nueva no rompa el
// cálculo de alto del Gantt.
func TestRenderGanttRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	for _, budget := range []int{8, 10, 12, 16, 20} {
		if n := lineCount(m.renderGantt(budget)); n > budget {
			t.Errorf("budget %d: alto = %d, excede el presupuesto", budget, n)
		}
	}
}

// TestGanttHiddenKeyDisabled verifica que H (hidden) no tenga efecto en el
// Gantt: la proyección ya excluye done/cancelled, así que el toggle no debe
// cambiar el estado de filtros.
func TestGanttHiddenKeyDisabled(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	before := m.filterActiveOnly
	m, _ = press(m, "H")
	if m.filterActiveOnly != before {
		t.Errorf("H no debe alternar filterActiveOnly en Gantt: %v -> %v", before, m.filterActiveOnly)
	}
}

func TestGanttNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	rows := m.ganttRows()
	first, last := ganttTaskRange(rows)
	if first < 0 {
		t.Fatal("expected gantt task rows")
	}

	// El cursor arranca sobre la primera tarea, nunca sobre una cabecera.
	if m.ganttCursor != first {
		t.Fatalf("initial cursor = %d, want first task %d", m.ganttCursor, first)
	}

	// j/k se mueven entre tareas, ignorando cabeceras de persona.
	second := -1
	for i := first + 1; i < len(rows); i++ {
		if rows[i].kind == ganttTaskRow {
			second = i
			break
		}
	}
	if second < 0 {
		t.Fatal("expected at least two task rows")
	}
	m, _ = press(m, "j")
	if m.ganttCursor != second {
		t.Errorf("after j: cursor = %d, want %d", m.ganttCursor, second)
	}
	if m.ganttRows()[m.ganttCursor].kind != ganttTaskRow {
		t.Errorf("el cursor quedó sobre una cabecera tras j")
	}
	m, _ = press(m, "k")
	if m.ganttCursor != first {
		t.Errorf("after k: cursor = %d, want %d", m.ganttCursor, first)
	}

	// k sobre la primera tarea no debe saltar a la cabecera previa.
	m, _ = press(m, "k")
	if m.ganttCursor != first {
		t.Errorf("k at first task: cursor = %d, want %d", m.ganttCursor, first)
	}
	if m.ganttRows()[m.ganttCursor].kind != ganttTaskRow {
		t.Errorf("k no debe dejar el cursor sobre una cabecera")
	}

	// G va a la última tarea, g a la primera.
	m, _ = press(m, "G")
	if m.ganttCursor != last {
		t.Errorf("after G: cursor = %d, want last task %d", m.ganttCursor, last)
	}
	if m.ganttRows()[m.ganttCursor].kind != ganttTaskRow {
		t.Errorf("G no debe dejar el cursor sobre una cabecera")
	}
	m, _ = press(m, "g")
	if m.ganttCursor != first {
		t.Errorf("after g: cursor = %d, want first task %d", m.ganttCursor, first)
	}

	// h/l desplazan la ventana de días.
	m, _ = press(m, "l")
	if m.ganttOffsetDays != 1 {
		t.Errorf("after l: offset = %d, want 1", m.ganttOffsetDays)
	}
	m, _ = press(m, "h")
	if m.ganttOffsetDays != 0 {
		t.Errorf("after h: offset = %d, want 0", m.ganttOffsetDays)
	}
	m, _ = press(m, "h")
	if m.ganttOffsetDays != 0 {
		t.Errorf("h at 0: offset = %d, want 0", m.ganttOffsetDays)
	}
}

// TestGanttSelectedTaskShowsCursor verifica que la fila seleccionada muestra el
// cursor ">" que la Lista ya usa, sin desalinear la grilla de días.
func TestGanttSelectedTaskShowsCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	rows := m.ganttRows()
	taskIdx := -1
	var id int64
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			taskIdx = i
			id = r.entry.Task.ID
			break
		}
	}
	if taskIdx < 0 {
		t.Fatal("expected a task row")
	}
	m.ganttCursor = taskIdx

	lines := strings.Split(ansi.Strip(m.renderGantt(m.height)), "\n")
	want := fmt.Sprintf("> #%d", id)
	found := false
	for _, line := range lines {
		if strings.Contains(line, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("la fila seleccionada debe mostrar el cursor %q:\n%s", want, strings.Join(lines, "\n"))
	}

	// Las tareas no seleccionadas conservan la indentación de dos espacios, para
	// que la cuadrícula de días no se desplace.
	for _, r := range rows {
		if r.kind == ganttTaskRow && r.entry.Task.ID != id {
			if !containsTaskLabel(lines, r.entry.Task.ID) {
				t.Errorf("fila no seleccionada #%d sin indentación esperada", r.entry.Task.ID)
			}
			break
		}
	}
}

// containsTaskLabel busca "  #<id>" (dos espacios) en alguna línea.
func containsTaskLabel(lines []string, id int64) bool {
	want := fmt.Sprintf("  #%d", id)
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func TestGanttProjectFilterIsViewOnly(t *testing.T) {
	m := newTestModel(t)
	full := m.ganttSchedule()

	m.filterProject = "api"
	filtered := m.ganttDisplay()

	if len(filtered.Assignees) == 0 {
		t.Fatal("expected filtered assignees for api")
	}
	for _, a := range filtered.Assignees {
		for _, e := range a.Entries {
			if e.Task.ProjectName != "api" {
				t.Errorf("filtered task project = %q, want api", e.Task.ProjectName)
			}
		}
	}

	// Las fechas deben ser idénticas al cálculo global: el filtro es de vista.
	dates := map[int64][2]string{}
	for _, a := range full.Assignees {
		for _, e := range a.Entries {
			dates[e.Task.ID] = [2]string{e.Start, e.End}
		}
	}
	seen := 0
	for _, a := range filtered.Assignees {
		for _, e := range a.Entries {
			seen++
			if dates[e.Task.ID] != [2]string{e.Start, e.End} {
				t.Errorf("task %d dates changed after filter: %v vs %v",
					e.Task.ID, [2]string{e.Start, e.End}, dates[e.Task.ID])
			}
		}
	}
	if seen == 0 {
		t.Error("no task entries survived the filter")
	}
}

func TestGanttSlashOpensFilters(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")
	m, _ = press(m, "/")
	if !m.filterOpen {
		t.Error("'/' en Gantt debería abrir el modal de filtros")
	}
	// Esc lo cierra.
	m, _ = press(m, "esc")
	if m.filterOpen {
		t.Error("Esc debería cerrar el modal de filtros")
	}
}

func TestKanbanSlashOpensFilters(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m, _ = press(m, "/")
	if !m.filterOpen {
		t.Error("'/' en Kanban debería abrir el modal de filtros")
	}
}

func TestGanttEnterOpensDetailOnTaskRow(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	// Posicionar el cursor en la primera fila de tarea.
	rows := m.ganttRows()
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			m.ganttCursor = i
			break
		}
	}

	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Error("enter on a task row should open the detail")
	}
	if m.detailTask == nil {
		t.Error("detail task should be set")
	}
}
