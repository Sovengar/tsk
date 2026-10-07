package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRenderListRespectsHeight verifies that the list does not exceed the
// available height and that the window keeps the cursor visible.
func TestRenderListRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	addTasks(t, m, 40)
	m.cursor = 20

	const budget = 20
	out := m.renderList(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("height = %d, exceeds the budget %d", n, budget)
	}

	selected := selectedRow(out)
	if selected == "" {
		t.Fatal("the selected row was not found")
	}
	if want := m.filteredTasks()[m.cursor].Title; !strings.Contains(selected, want) {
		t.Errorf("the selected row %q does not contain the task %q", selected, want)
	}
}

// TestRenderListPageLegend verifies that the pagination caption ends up
// embedded in the bottom border, aligned to the right.
func TestRenderListPageLegend(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	addTasks(t, m, 40) // 44 active tasks
	m.pageSize = 10

	out := ansi.Strip(m.renderList(40))
	if !strings.Contains(out, "1-10 of 44 · Page 1/5") {
		t.Errorf("the first page legend is missing:\n%s", out)
	}

	m.cursor = 43
	out = ansi.Strip(m.renderList(40))
	if !strings.Contains(out, "41-44 of 44 · Page 5/5") {
		t.Errorf("wrong last page legend:\n%s", out)
	}

	lines := strings.Split(out, "\n")
	bottom := lines[len(lines)-1]
	if !strings.Contains(bottom, "41-44 of 44 · Page 5/5") {
		t.Errorf("the legend must be on the bottom border line: %q", bottom)
	}
	if !strings.HasSuffix(bottom, "╯") {
		t.Errorf("the bottom border line must close with the corner: %q", bottom)
	}
}

func addTasks(t *testing.T, m *Model, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := m.database.CreateTask("api", fmt.Sprintf("T%02d", i), "", "@john", 1, "todo"); err != nil {
			t.Fatal(err)
		}
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()
}

// selectedRow returns the row marked with "> " of a render.
func selectedRow(out string) string {
	for _, line := range strings.Split(ansi.Strip(out), "\n") {
		inner := strings.TrimPrefix(line, "│") // strips the left border
		if strings.HasPrefix(inner, "> ") {
			return inner
		}
	}
	return ""
}

// TestRenderListFitsNarrowWidth verifies that at small width the rows are
// truncated instead of wrapped. If they wrapped, each row would take two lines
// and the box would exceed the available height.
func TestRenderListFitsNarrowWidth(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	addTasks(t, m, 40)
	m.width = 80

	const budget = 20
	out := m.renderList(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("height = %d, exceeds the budget %d", n, budget)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("line %d measures %d, exceeds the width %d", i, w, m.width)
		}
	}
}

// TestVisibleListColumns verifies how many columns fit according to the
// available width: the last ones are dropped first.
func TestVisibleListColumns(t *testing.T) {
	tests := []struct {
		name  string
		avail int
		want  int
	}{
		{name: "all fit", avail: 126, want: 6},
		{name: "exact edge with Description", avail: 113, want: 6},
		{name: "Description drops", avail: 112, want: 5},
		{name: "exact edge with Tags", avail: 72, want: 5},
		{name: "Tags drops", avail: 71, want: 4},
		{name: "exact edge without Tags", avail: 61, want: 4},
		{name: "Title drops", avail: 60, want: 3},
		{name: "very narrow", avail: 10, want: 1},
		{name: "narrower than the first", avail: 5, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visibleListColumns(tt.avail); got != tt.want {
				t.Errorf("visibleListColumns(%d) = %d, want %d", tt.avail, got, tt.want)
			}
		})
	}
}

// TestRenderListHidesDescriptionWhenNarrow verifies that the Description column
// is omitted entirely when it does not fit, instead of being cut in half.
func TestRenderListHidesDescriptionWhenNarrow(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	wide := ansi.Strip(m.renderList(m.height))
	if !strings.Contains(wide, "Description") {
		t.Fatalf("with width %d Description should be visible:\n%s", m.width, wide)
	}

	m.width = 80
	narrow := ansi.Strip(m.renderList(m.height))
	if strings.Contains(narrow, "Descr") {
		t.Errorf("with width 80 Description should be hidden:\n%s", narrow)
	}
	if !strings.Contains(narrow, "Title") {
		t.Errorf("with width 80 Title should still be visible:\n%s", narrow)
	}
}

// TestRenderListColumnsAligned verifies that the value of each row starts at
// the same screen column as its header. Two bugs added up here: fmt's padding
// counts bytes (and the priority cell carries ANSI), and the header did not
// carry the 2-column prefix that the rows do carry.
func TestRenderListColumnsAligned(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m.width = 130 // all columns fit

	lines := strings.Split(ansi.Strip(m.renderList(m.height)), "\n")

	headerIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "Description") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("the table header was not found")
	}
	header := lines[headerIdx]

	// Columns with a non-empty value in every task of the fixture.
	columns := []string{"Priority", "Status", "Assignee", "Title"}

	rows := 0
	for _, l := range lines[headerIdx+1:] {
		if strings.HasPrefix(l, "╰") {
			break
		}
		if strings.Contains(l, "─") {
			continue // separator
		}
		rows++
		for _, col := range columns {
			off := displayColumn(header, col)
			if cell := ansi.Cut(l, off, off+1); cell == " " || cell == "" {
				t.Errorf("column %s misaligned (empty at column %d):\n%s", col, off, l)
			}
		}
	}
	if rows == 0 {
		t.Fatal("no task row was rendered")
	}
}

// displayColumn returns the screen column where substr starts in line.
func displayColumn(line, substr string) int {
	idx := strings.Index(line, substr)
	if idx < 0 {
		return -1
	}
	return ansi.StringWidth(line[:idx])
}
