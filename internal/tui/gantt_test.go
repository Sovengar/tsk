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

// TestRenderGanttShowsFilterHeader verifies that the Gantt shows the filter
// header without Status: its projection only includes active tasks, so a
// status filter would always be "active".
func TestRenderGanttShowsFilterHeader(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")
	m.filterProject = "api"

	plain := ansi.Strip(m.renderGantt(m.height))
	for _, want := range []string{"Project:", "Assignee:", "Priority:", "api"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the Gantt header does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Status:") {
		t.Errorf("the Gantt header must not show Status:\n%s", plain)
	}
}

// TestRenderGanttRespectsHeight verifies that the new header does not break the
// Gantt's height calculation.
func TestRenderGanttRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	for _, budget := range []int{8, 10, 12, 16, 20} {
		if n := lineCount(m.renderGantt(budget)); n > budget {
			t.Errorf("budget %d: height = %d, exceeds the budget", budget, n)
		}
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

	// The cursor starts on the first task, never on a header.
	if m.ganttCursor != first {
		t.Fatalf("initial cursor = %d, want first task %d", m.ganttCursor, first)
	}

	// j/k move between tasks, ignoring person headers.
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
		t.Errorf("the cursor ended on a header after j")
	}
	m, _ = press(m, "k")
	if m.ganttCursor != first {
		t.Errorf("after k: cursor = %d, want %d", m.ganttCursor, first)
	}

	// k on the first task must not jump to the previous header.
	m, _ = press(m, "k")
	if m.ganttCursor != first {
		t.Errorf("k at first task: cursor = %d, want %d", m.ganttCursor, first)
	}
	if m.ganttRows()[m.ganttCursor].kind != ganttTaskRow {
		t.Errorf("k must not leave the cursor on a header")
	}

	// G goes to the last task, g to the first.
	m, _ = press(m, "G")
	if m.ganttCursor != last {
		t.Errorf("after G: cursor = %d, want last task %d", m.ganttCursor, last)
	}
	if m.ganttRows()[m.ganttCursor].kind != ganttTaskRow {
		t.Errorf("G must not leave the cursor on a header")
	}
	m, _ = press(m, "g")
	if m.ganttCursor != first {
		t.Errorf("after g: cursor = %d, want first task %d", m.ganttCursor, first)
	}

	// h/l scroll the day window.
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

// TestGanttSelectedTaskShowsCursor verifies that the selected row shows the
// ">" cursor the List already uses, without misaligning the day grid.
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
		t.Errorf("the selected row must show the cursor %q:\n%s", want, strings.Join(lines, "\n"))
	}

	// The unselected tasks keep the two-space indentation, so that the
	// day grid does not shift.
	for _, r := range rows {
		if r.kind == ganttTaskRow && r.entry.Task.ID != id {
			if !containsTaskLabel(lines, r.entry.Task.ID) {
				t.Errorf("unselected row #%d without the expected indentation", r.entry.Task.ID)
			}
			break
		}
	}
}

// containsTaskLabel looks for "  #<id>" (two spaces) in some line.
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

	// The dates must be identical to the global calculation: the filter is a view one.
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
		t.Error("'/' in the Gantt should open the filter modal")
	}
	// Esc closes it.
	m, _ = press(m, "esc")
	if m.filterOpen {
		t.Error("Esc should close the filter modal")
	}
}

func TestKanbanSlashOpensFilters(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m, _ = press(m, "/")
	if !m.filterOpen {
		t.Error("'/' in the Kanban should open the filter modal")
	}
}

func TestGanttEnterOpensDetailOnTaskRow(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")

	// Position the cursor on the first task row.
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
