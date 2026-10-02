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

// El preview envuelve la descripción al ancho y, si no cabe en el alto dado,
// recorta las líneas que sobran. La última que se ve lleva una elipsis y sigue
// midiendo el ancho entero: por eso el recorte es de limit-1 y no de limit, y por
// eso la elipsis entra justo.
func TestPreviewTruncatedLastLineKeepsWidth(t *testing.T) {
	p := NewPreviewBar(40)
	p.SetTask(&model.Task{Description: strings.Repeat("palabra ", 40)})
	p.SetMaxLines(2)

	lineas := strings.Split(ansi.Strip(p.View()), "\n")
	if len(lineas) < 2 {
		t.Fatalf("el preview tiene %d líneas:\n%q", len(lineas), p.View())
	}
	// Lo que se comprueba es que la última línea lleva elipsis: es lo que dice
	// que la descripción sigue más allá de las líneas que caben. El ancho de la
	// línea no sirve aquí -- la caja rellena hasta su borde y todas miden lo
	// mismo.
	if !strings.Contains(strings.Join(lineas, "\n"), "…") {
		t.Errorf("una descripción recortada no lleva elipsis:\n%q", p.View())
	}
}

// Si la descripción cabe entera, no hay ni recorte ni elipsis.
func TestPreviewNoEllipsisWhenItFits(t *testing.T) {
	p := NewPreviewBar(80)
	p.SetTask(&model.Task{Description: "corta"})
	p.SetMaxLines(20)

	if out := ansi.Strip(p.View()); strings.Contains(out, "…") {
		t.Errorf("una descripción corta lleva elipsis:\n%q", out)
	}
}

// maxLines nunca baja de uno: un preview de alto cero no puede no mostrar nada,
// porque entonces la fila de la tarea desaparece.
func TestPreviewMaxLinesFloor(t *testing.T) {
	p := NewPreviewBar(40)
	p.SetTask(&model.Task{Description: strings.Repeat("palabra ", 50)})
	p.SetMaxLines(0)

	if lineCount(ansi.Strip(p.View())) < 1 {
		t.Errorf("con maxLines 0 el preview no muestra nada:\n%q", p.View())
	}
}

// Con la descripción justo al límite de líneas no hay elipsis: cabe entera. El
// borde es donde `len(wrapped) > maxLines` deja de recortar.
func TestPreviewEllipsisOnlyWhenClipped(t *testing.T) {
	desc := strings.Repeat("palabra ", 40)
	for _, maxLines := range []int{1, 2, 5, 20} {
		p := NewPreviewBar(40)
		p.SetTask(&model.Task{Description: desc})
		p.SetMaxLines(maxLines)

		// La caja añade bordes y cabecera, así que se cuentan sólo las líneas de
		// la descripción.
		desc := 0
		for _, linea := range strings.Split(ansi.Strip(p.View()), "\n") {
			if strings.Contains(linea, "palabra") {
				desc++
			}
		}
		if desc > maxLines {
			t.Errorf("con maxLines %d salen %d líneas de descripción", maxLines, desc)
		}
		if desc == 0 {
			t.Errorf("con maxLines %d no sale ninguna línea de descripción", maxLines)
		}
	}
}
