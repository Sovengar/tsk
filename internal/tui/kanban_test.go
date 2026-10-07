package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRenderKanbanColumnsSpaced verifies that the columns are not stuck
// together: the borders of one column and the next must be separated by a gap.
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
			t.Errorf("column borders touch: found %q", pair)
		}
	}
}

// TestRenderKanbanFitsWidth verifies that the board does not exceed the
// terminal's width and that it uses up all the available width.
func TestRenderKanbanFitsWidth(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.width = 140

	out := m.renderKanban(m.height)
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("line %d measures %d, exceeds the width %d", i, w, m.width)
		}
	}
}

// TestKanbanColumnWidths verifies the width split: the minimum is respected,
// no dead space is left and the remainder is spread evenly.
func TestKanbanColumnWidths(t *testing.T) {
	mins := []int{16, 12, 20, 15, 13}
	avail := 128

	widths := kanbanColumnWidths(mins, avail)
	total := 0
	for i, w := range widths {
		if w < mins[i] {
			t.Errorf("column %d: width %d below the minimum %d", i, w, mins[i])
		}
		total += w
	}
	total += kanbanGap * (len(widths) - 1)
	if total != avail {
		t.Errorf("total width = %d, want %d (it does not use the space)", total, avail)
	}
}

// TestKanbanColumnWidthsTooNarrow verifies that with little space the
// minimums are respected without spreading a non-existent remainder.
func TestKanbanColumnWidthsTooNarrow(t *testing.T) {
	mins := []int{20, 20, 20, 20, 20, 20}

	widths := kanbanColumnWidths(mins, 60)
	for i, w := range widths {
		if w != mins[i] {
			t.Errorf("column %d: width %d, want %d", i, w, mins[i])
		}
	}
}

// TestKanbanTerminalColumnFilteredByStatus verifies that the status filter
// controls the done column: with the default "all active" it stays empty; with
// "all" the terminal ones show up again.
func TestKanbanTerminalColumnFilteredByStatus(t *testing.T) {
	m := newTestModel(t)
	markFirstDone(t, m)
	m, _ = press(m, "2")

	done := kanbanColumnIndex(m, "done")
	if done < 0 {
		t.Fatal("there is no done column")
	}

	if n := len(m.kanbanColumns()[done].tasks); n != 0 {
		t.Errorf("with all active the done column must be empty, it has %d", n)
	}

	m.filterApplySelection(filterFieldStatus, "all")

	if n := len(m.kanbanColumns()[done].tasks); n != 1 {
		t.Errorf("with all the done column must show 1 task, it has %d", n)
	}
}

// TestKanbanAllStatusCardsAreNavigable verifies that the terminal cards, once
// shown, are selectable. Before, the render and the navigation used different
// lists, so the done column showed unreachable cards.
func TestKanbanAllStatusCardsAreNavigable(t *testing.T) {
	m := newTestModel(t)
	markFirstDone(t, m)
	m, _ = press(m, "2")
	m.filterApplySelection(filterFieldStatus, "all") // show done

	done := kanbanColumnIndex(m, "done")
	if done < 0 {
		t.Fatal("there is no done column")
	}
	tasks := m.kanbanColumns()[done].tasks
	if len(tasks) == 0 {
		t.Fatal("the done column should show the marked task")
	}

	m.kanbanCol = done
	m.kanbanRow = 0

	got := m.selectedTask()
	if got == nil {
		t.Fatal("the task in the done column should be selectable")
	}
	if got.ID != tasks[0].ID {
		t.Errorf("selectedTask = %d, want %d", got.ID, tasks[0].ID)
	}

	m, _ = press(m, "enter")
	if !m.detailOpen || m.detailTask == nil {
		t.Fatal("Enter should open the detail of the selected task")
	}
	if m.detailTask.ID != got.ID {
		t.Errorf("detail = %d, want %d", m.detailTask.ID, got.ID)
	}
}

// TestKanbanAdvanceUsesProjectWorkflow verifies that "s" advances according
// to the task's project workflow, not the merge. The merge would put
// "reviewing" as the next of "doing", but web does not have that status and
// the move would be silently rejected.
func TestKanbanAdvanceUsesProjectWorkflow(t *testing.T) {
	m := newTestModel(t)
	if _, err := m.database.CreateTask("web", "Deploy", "", "@john", 1, "doing"); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()

	m, _ = press(m, "2") // kanban view

	col := kanbanColumnIndex(m, "doing")
	if col < 0 {
		t.Fatal("there is no doing column")
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
		t.Fatal("no web task was found in doing")
	}
	m.kanbanCol, m.kanbanRow = col, row

	_, cmd := press(m, "s")
	if cmd == nil {
		t.Fatal("s should emit a command")
	}
	mustMsg(t, cmd)

	got, err := m.database.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	// web = [todo,doing,done]: the next of doing is done, not reviewing.
	if got.Status != "done" {
		t.Errorf("status = %q, want done (web's workflow)", got.Status)
	}
}

// markFirstDone marks the first task of the fixture as done and reloads the model.
func markFirstDone(t *testing.T, m *Model) {
	t.Helper()
	if len(m.tasks) == 0 {
		t.Fatal("the fixture has no tasks")
	}
	if _, err := m.database.DoneTask(m.tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()
}

// kanbanColumnIndex returns the index of the given status's column, or -1.
func kanbanColumnIndex(m *Model, status string) int {
	for i, c := range m.kanbanColumns() {
		if c.status == status {
			return i
		}
	}
	return -1
}

// TestRenderKanbanShowsFilterHeader verifies that the board shows the same
// filter header as the List, with the active value.
func TestRenderKanbanShowsFilterHeader(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.filterProject = "api"

	plain := ansi.Strip(m.renderKanban(m.height))
	for _, want := range []string{"Project:", "Status:", "Assignee:", "Priority:", "api"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the Kanban header does not contain %q:\n%s", want, plain)
		}
	}
}

// TestKanbanRespectsProjectFilter verifies that the header does not lie: the
// board only shows tasks of the filtered project.
func TestKanbanRespectsProjectFilter(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.filterProject = "api"

	for _, c := range m.kanbanColumns() {
		for _, task := range c.tasks {
			if task.ProjectName != "api" {
				t.Errorf("column %s: task %d belongs to %q, want api", c.status, task.ID, task.ProjectName)
			}
		}
	}
}

// TestRenderKanbanRespectsHeight verifies that a long title is truncated
// instead of wrapping: if it wrapped, the column would grow and the board would exceed the height.
func TestRenderKanbanRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	m.width = 100
	for i := range m.tasks {
		m.tasks[i].Title = strings.Repeat("LONG-TITLE ", 12)
	}

	const budget = 14
	out := m.renderKanban(budget)

	if n := lineCount(out); n > budget {
		t.Errorf("height = %d, exceeds the budget %d", n, budget)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("line %d measures %d, exceeds the width %d", i, w, m.width)
		}
	}
}
