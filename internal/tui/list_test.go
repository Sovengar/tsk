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
	m, _ = press(m, "2")
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
	m, _ = press(m, "2")
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
		if _, err := m.database.CreateTask("api", fmt.Sprintf("T%02d", i), "", "@juan", 1, 0, "todo"); err != nil {
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
	m, _ = press(m, "2")
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
