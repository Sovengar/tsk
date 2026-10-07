package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRenderDashboardRespectsHeight verifies that the dashboard does not
// exceed the available height even with many tasks and assignees: the extra
// rows of the columns are dropped.
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
		t.Errorf("height = %d, exceeds the budget %d", n, budget)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("line %d measures %d, exceeds the width %d", i, w, m.width)
		}
	}
}
