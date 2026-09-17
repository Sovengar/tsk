package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRenderDashboardRespectsHeight verifica que el dashboard no exceda el alto
// disponible aunque haya muchas tareas y assignees: las filas sobrantes de las
// columnas se descartan.
func TestRenderDashboardRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m.width = 100

	for i := 0; i < 30; i++ {
		if _, err := m.database.CreateTask("api", fmt.Sprintf("T%02d", i), "", fmt.Sprintf("@dev%02d", i), 1, "todo"); err != nil {
			t.Fatal(err)
		}
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()

	const budget = 20
	out := m.renderDashboard(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("alto = %d, excede el presupuesto %d", n, budget)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("línea %d mide %d, excede el ancho %d", i, w, m.width)
		}
	}
}
