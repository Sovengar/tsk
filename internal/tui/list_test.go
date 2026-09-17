package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRenderListRespectsHeight verifica que la lista no exceda el alto
// disponible y que la ventana mantenga el cursor visible.
func TestRenderListRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	addTasks(t, m, 40)
	m.cursor = 20

	const budget = 20
	out := m.renderList(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("alto = %d, excede el presupuesto %d", n, budget)
	}

	selected := selectedRow(out)
	if selected == "" {
		t.Fatal("no se encontró la fila seleccionada")
	}
	if want := m.filteredTasks()[m.cursor].Title; !strings.Contains(selected, want) {
		t.Errorf("fila seleccionada %q no contiene la tarea %q", selected, want)
	}
}

// TestRenderListPageLegend verifica que la leyenda de paginación quede
// incrustada en el borde inferior, alineada a la derecha.
func TestRenderListPageLegend(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	addTasks(t, m, 40) // 44 tareas activas
	m.pageSize = 10

	out := ansi.Strip(m.renderList(40))
	if !strings.Contains(out, "1-10 of 44 · Page 1/5") {
		t.Errorf("falta la leyenda de la primera página:\n%s", out)
	}

	m.cursor = 43
	out = ansi.Strip(m.renderList(40))
	if !strings.Contains(out, "41-44 of 44 · Page 5/5") {
		t.Errorf("leyenda de la última página incorrecta:\n%s", out)
	}

	lines := strings.Split(out, "\n")
	bottom := lines[len(lines)-1]
	if !strings.Contains(bottom, "41-44 of 44 · Page 5/5") {
		t.Errorf("la leyenda debe ir en la línea del borde inferior: %q", bottom)
	}
	if !strings.HasSuffix(bottom, "╯") {
		t.Errorf("la línea del borde inferior debe cerrar con la esquina: %q", bottom)
	}
}

func addTasks(t *testing.T, m *Model, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := m.database.CreateTask("api", fmt.Sprintf("T%02d", i), "", "@juan", 1, "todo"); err != nil {
			t.Fatal(err)
		}
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()
}

// selectedRow devuelve la fila marcada con "> " de un render.
func selectedRow(out string) string {
	for _, line := range strings.Split(ansi.Strip(out), "\n") {
		inner := strings.TrimPrefix(line, "│") // quita el borde izquierdo
		if strings.HasPrefix(inner, "> ") {
			return inner
		}
	}
	return ""
}

// TestRenderListFitsNarrowWidth verifica que con poco ancho las filas se
// recorten en vez de wrappear. Si wrappeasen, cada fila ocuparía dos líneas y
// la caja excedería el alto disponible.
func TestRenderListFitsNarrowWidth(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	addTasks(t, m, 40)
	m.width = 80

	const budget = 20
	out := m.renderList(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("alto = %d, excede el presupuesto %d", n, budget)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("línea %d mide %d, excede el ancho %d", i, w, m.width)
		}
	}
}

// TestVisibleListColumns verifica cuántas columnas entran según el ancho
// disponible: las últimas se descartan primero.
func TestVisibleListColumns(t *testing.T) {
	tests := []struct {
		name  string
		avail int
		want  int
	}{
		{name: "entran todas", avail: 126, want: 6},
		{name: "borde exacto con Description", avail: 113, want: 6},
		{name: "se cae Description", avail: 112, want: 5},
		{name: "borde exacto con Tags", avail: 72, want: 5},
		{name: "se cae Tags", avail: 71, want: 4},
		{name: "borde exacto sin Tags", avail: 61, want: 4},
		{name: "se cae Title", avail: 60, want: 3},
		{name: "muy angosto", avail: 10, want: 1},
		{name: "más angosto que la primera", avail: 5, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visibleListColumns(tt.avail); got != tt.want {
				t.Errorf("visibleListColumns(%d) = %d, want %d", tt.avail, got, tt.want)
			}
		})
	}
}

// TestRenderListHidesDescriptionWhenNarrow verifica que la columna Description
// se omita entera cuando no entra, en vez de cortarse a la mitad.
func TestRenderListHidesDescriptionWhenNarrow(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	wide := ansi.Strip(m.renderList(m.height))
	if !strings.Contains(wide, "Description") {
		t.Fatalf("con ancho %d debería verse Description:\n%s", m.width, wide)
	}

	m.width = 80
	narrow := ansi.Strip(m.renderList(m.height))
	if strings.Contains(narrow, "Descr") {
		t.Errorf("con ancho 80 Description debería estar oculta:\n%s", narrow)
	}
	if !strings.Contains(narrow, "Title") {
		t.Errorf("con ancho 80 Title debería seguir visible:\n%s", narrow)
	}
}

// TestRenderListColumnsAligned verifica que el valor de cada fila arranque en la
// misma columna de pantalla que su header. Acá se sumaban dos bugs: el padding
// de fmt cuenta bytes (y la celda de prioridad lleva ANSI), y el header no
// llevaba el prefijo de 2 columnas que sí llevan las filas.
func TestRenderListColumnsAligned(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m.width = 130 // entran todas las columnas

	lines := strings.Split(ansi.Strip(m.renderList(m.height)), "\n")

	headerIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "Description") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("no se encontró la cabecera de la tabla")
	}
	header := lines[headerIdx]

	// Columnas con valor no vacío en todas las tareas del fixture.
	columns := []string{"Priority", "Status", "Assignee", "Title"}

	rows := 0
	for _, l := range lines[headerIdx+1:] {
		if strings.HasPrefix(l, "╰") {
			break
		}
		if strings.Contains(l, "─") {
			continue // separador
		}
		rows++
		for _, col := range columns {
			off := displayColumn(header, col)
			if cell := ansi.Cut(l, off, off+1); cell == " " || cell == "" {
				t.Errorf("columna %s desalineada (vacía en la columna %d):\n%s", col, off, l)
			}
		}
	}
	if rows == 0 {
		t.Fatal("no se renderizó ninguna fila de tarea")
	}
}

// displayColumn devuelve la columna de pantalla donde arranca substr en line.
func displayColumn(line, substr string) int {
	idx := strings.Index(line, substr)
	if idx < 0 {
		return -1
	}
	return ansi.StringWidth(line[:idx])
}
