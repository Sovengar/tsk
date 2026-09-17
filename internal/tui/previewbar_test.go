package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

func TestPreviewBarRendersDescription(t *testing.T) {
	p := NewPreviewBar(60)
	p.SetTask(&model.Task{ID: 1, Title: "t", Description: "hola que tal"})

	out := ansi.Strip(p.View())
	if !strings.Contains(out, "hola que tal") {
		t.Errorf("el preview debería mostrar la descripción:\n%s", out)
	}
	if !strings.Contains(out, "Description") {
		t.Errorf("el preview debería llevar título Description:\n%s", out)
	}
}

func TestPreviewBarEmptyDescriptionShowsPlaceholder(t *testing.T) {
	p := NewPreviewBar(60)
	p.SetTask(&model.Task{ID: 1, Title: "t"})

	if out := ansi.Strip(p.View()); !strings.Contains(out, "(no description)") {
		t.Errorf("sin descripción debería mostrar el placeholder:\n%s", out)
	}
}

func TestPreviewBarWithoutTaskIsEmpty(t *testing.T) {
	p := NewPreviewBar(60)
	if out := p.View(); out != "" {
		t.Errorf("sin tarea seleccionada el preview debe ser vacío, got %q", out)
	}
}

// TestPreviewBarTruncatesToMaxLines verifica el tope de alto y que la línea
// truncada nunca exceda el ancho (si lo excediera, bordered la re-wrapéaría y
// agregaría una fila de más).
func TestPreviewBarTruncatesToMaxLines(t *testing.T) {
	const width = 40
	p := NewPreviewBar(width)
	p.SetMaxLines(3)
	p.SetTask(&model.Task{Description: strings.Repeat("palabra ", 60)})

	out := p.View()
	lines := strings.Split(out, "\n")
	if len(lines) != 3+2 { // maxLines + bordes superior e inferior
		t.Fatalf("alto = %d filas, want %d:\n%s", len(lines), 3+2, out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("una descripción larga debería truncarse con …:\n%s", out)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("línea %d mide %d, excede el ancho %d", i, w, width)
		}
	}
}

func TestPreviewBarRespectsWidth(t *testing.T) {
	const width = 30
	p := NewPreviewBar(width)
	p.SetTask(&model.Task{Description: strings.Repeat("una palabra bastante larga ", 20)})

	for i, line := range strings.Split(p.View(), "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("línea %d mide %d, excede el ancho %d", i, w, width)
		}
	}
}

func TestSelectedTaskNilOnDashboard(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	if got := m.selectedTask(); got != nil {
		t.Errorf("en dashboard no hay tarea seleccionada, got %+v", got)
	}
}

func TestSelectedTaskFollowsListCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m.cursor = 1

	got := m.selectedTask()
	if got == nil {
		t.Fatal("debería haber tarea seleccionada en la lista")
	}
	if want := m.filteredTasks()[1].ID; got.ID != want {
		t.Errorf("selectedTask = %d, want %d", got.ID, want)
	}
}

func TestSelectedTaskFollowsKanbanCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")

	cols := m.kanbanColumns()
	idx := -1
	for i, c := range cols {
		if len(c.tasks) > 0 {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("el fixture debería tener alguna columna con tareas")
	}
	m.kanbanCol = idx
	m.kanbanRow = 0

	got := m.selectedTask()
	if got == nil {
		t.Fatal("debería haber tarea seleccionada en kanban")
	}
	if want := cols[idx].tasks[0].ID; got.ID != want {
		t.Errorf("selectedTask = %d, want %d", got.ID, want)
	}
}
