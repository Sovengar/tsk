package tui

import (
	"errors"
	"strings"
	"testing"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// errActionFailed is the error that the simulated actions return when they
// are meant to fail: the command must not reload the list.
var errActionFailed = errors.New("action failed")

// The handlers always start with their clamp, so an out-of-range cursor
// cannot be provoked from the keyboard. What is real is the empty list:
// there the clamp leaves the cursor at 0 and the key guard has to prevent
// the action, because indexing tasks[0] on an empty list blows up.
//
// These tests pin that edge down: with tasks the key acts, with no tasks it
// does nothing.

// With the filtered list empty, no action key touches the database.
func TestListActionKeysNoopWithEmptyList(t *testing.T) {
	for _, key := range []string{"s", "d", "x"} {
		t.Run(key, func(t *testing.T) {
			m := newTestModel(t)
			m.tasks = nil
			m.filteredT = nil
			m.filterStatus = "nonexistent-status"

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("key %q with no tasks: %v wants to run an action", key, cmd)
			}
			if next.cursor != 0 {
				t.Errorf("the cursor ended at %d with no tasks", next.cursor)
			}
		})
	}
}

// The other side of the same guard: with a task under the cursor, the key acts.
// Without this half, the previous test would also pass with the guard removed.
func TestListActionKeysActWithValidCursor(t *testing.T) {
	for _, key := range []string{"s", "d", "x"} {
		t.Run(key, func(t *testing.T) {
			m := newTestModel(t)
			if len(m.filteredTasks()) == 0 {
				t.Skip("the fixture left no visible tasks")
			}
			m.cursor = 0

			if _, cmd := press(m, key); cmd == nil {
				t.Errorf("key %q with a task selected produced no command", key)
			}
		})
	}
}

// The height available for the content discounts the preview and the toast.
// The toast takes one line, so adding it instead of subtracting it changes how
// many task rows fit in the box.
//
// At a height of 18 the difference falls right on the edge: with toast 2 task
// rows are painted, without toast 3. The numbers are literals, not derived: if
// someone changes the layout's height split the test has to be updated by hand,
// which is exactly what is wanted.
func TestViewBudgetCountsToastLine(t *testing.T) {
	for _, tt := range []struct {
		toast string
		want  int
	}{
		{"saved", 2},
		{"", 3},
	} {
		t.Run("toast="+tt.toast, func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = viewList
			m.height = 18
			m.toast = tt.toast
			m.toastKind = "info"

			if got := countRenderedTasks(t, m); got != tt.want {
				t.Errorf("with toast %q %d task rows were painted, want %d", tt.toast, got, tt.want)
			}
		})
	}
}

// The rows are counted by the titles of the fixture's tasks: each one appears
// exactly once per painted row.
var fixtureTitles = []string{"Fix N+1 query", "Add caching", "Update README", "Fix checkout"}

func countRenderedTasks(t *testing.T, m *Model) int {
	t.Helper()
	out := m.View().Content
	n := 0
	for _, title := range fixtureTitles {
		n += strings.Count(out, title)
	}
	return n
}

// The detail's selection does not wrap upwards: -1 means "nothing
// selected", and from there "j" goes to the first comment, not the second.

// With the cursor on the last comment, "j" stays there.
func TestDetailCommentDownStopsAtLast(t *testing.T) {
	m := newDetailModel(t, 3)
	m.detailCommentSel = len(m.detailComments) - 1

	next, _ := press(m, "j")
	if next.detailCommentSel != 2 {
		t.Errorf("selection %d, want 2 (does not advance past the last)", next.detailCommentSel)
	}
}

// And from -1, "j" goes to the first one, not the second.
func TestDetailCommentDownFromNoneSelectsFirst(t *testing.T) {
	m := newDetailModel(t, 3)
	m.detailCommentSel = -1

	next, _ := press(m, "j")
	if next.detailCommentSel != 0 {
		t.Errorf("selection %d, want 0", next.detailCommentSel)
	}
}

// Up from the first one goes back to "nothing selected", and from there it
// does not go out to -2.
func TestDetailCommentUpFromFirstClearsSelection(t *testing.T) {
	m := newDetailModel(t, 2)
	m.detailCommentSel = 0

	next, _ := press(m, "k")
	if next.detailCommentSel != -1 {
		t.Errorf("selection %d, want -1", next.detailCommentSel)
	}

	next, _ = press(m, "k")
	if next.detailCommentSel != -1 {
		t.Errorf("selection %d, want -1 (does not go below -1)", next.detailCommentSel)
	}
}

// With no comments, neither "j" nor "k" move anything.
func TestDetailCommentKeysNoopWithoutComments(t *testing.T) {
	for _, key := range []string{"j", "k"} {
		t.Run(key, func(t *testing.T) {
			m := newDetailModel(t, 0)

			next, _ := press(m, key)
			if next.detailCommentSel != -1 {
				t.Errorf("selection %d, want -1 with no comments", next.detailCommentSel)
			}
		})
	}
}

// newDetailModel opens the detail of the first task with n comments.
func newDetailModel(t *testing.T, comments int) *Model {
	t.Helper()
	m := newTestModel(t)
	if len(m.tasks) == 0 {
		t.Fatal("the fixture left no tasks")
	}
	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	for i := range comments {
		mustAddComment(t, m.database, task.ID, "comment")
		_ = i
	}
	m.detailComments, _ = m.database.ListComments(task.ID)
	if len(m.detailComments) != comments {
		t.Fatalf("%d comments requested, got %d", comments, len(m.detailComments))
	}
	return m
}

// "d" in the detail deletes the selected comment; with no selection, or with
// a selection that does not apply to the list, it marks the task. The guard's
// edge is detailCommentSel == len(comments), which the interface never produces
// but the code checks: if the guard disappeared, it would index out of range.
func TestDetailDeleteFallsBackToTaskWhenSelectionOutOfRange(t *testing.T) {
	for _, sel := range []int{-1, -5, 2, 99} {
		m := newDetailModel(t, 2)
		m.detailCommentSel = sel

		next, _ := press(m, "d")
		// Marking the task closes the detail; deleting a comment does not.
		if next.detailOpen {
			t.Errorf("sel %d: neither deleted nor marked; the detail is still open", sel)
		}
	}
}

// With a valid selection, "d" deletes that comment and leaves the detail open.
// Without this half, the previous test would also pass with the guard removed.
func TestDetailDeleteRemovesSelectedComment(t *testing.T) {
	m := newDetailModel(t, 2)
	m.detailCommentSel = 1

	next, cmd := press(m, "d")
	if cmd == nil {
		t.Fatal("it produced no delete command")
	}
	if !next.detailOpen {
		t.Error("deleting a comment closed the detail")
	}
}

// The detail keys that need an open task open nothing without one. Each one
// has its own guard, and removing them would change the behavior with the
// detail closed.
func TestDetailKeysRequireOpenTask(t *testing.T) {
	for _, key := range []string{"t", "e", "c", "E"} {
		t.Run(key, func(t *testing.T) {
			m := newDetailModel(t, 1)
			m.detailOpen = true
			m.detailTask = nil

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("key %q with no open task produced command %T", key, cmd)
			}
			if next.tagOpen {
				t.Error("it opened the tag modal without a task")
			}
			if next.descEditOpen {
				t.Error("it opened the description editor without a task")
			}
		})
	}
}

// New starts with a usable pageSize even if the config brings none, and with
// the rest of the default values the views assume.
func TestNewDefaults(t *testing.T) {
	m := newBareModel(t, func(c *config.Config) { c.ListPageSize = 0 })

	if m.pageSize != config.DefaultPageSize {
		t.Errorf("pageSize %d, want %d with the config at 0", m.pageSize, config.DefaultPageSize)
	}
	if m.currentView != viewList {
		t.Errorf("initial view %v, want the list", m.currentView)
	}
	if m.cursor != 0 {
		t.Errorf("initial cursor %d, want 0", m.cursor)
	}
	// -1 means "no filter": it is told apart from 0, which would be only the highest priority.
	if m.filterPriority != -1 {
		t.Errorf("initial filterPriority %d, want -1", m.filterPriority)
	}
	if m.filterStatus != statusFilterAllActive {
		t.Errorf("initial filterStatus %q, want %q", m.filterStatus, statusFilterAllActive)
	}
	// Same as -1: "nothing selected" in both pickers.
	if m.detailCommentSel != -1 || m.tagSuggestIdx != -1 {
		t.Errorf("initial selectors (%d, %d), want (-1, -1)", m.detailCommentSel, m.tagSuggestIdx)
	}
	if m.width != 80 || m.height != 24 {
		t.Errorf("initial size %dx%d, want 80x24", m.width, m.height)
	}
}

// And a config with pageSize is respected as-is.
func TestNewKeepsConfiguredPageSize(t *testing.T) {
	for _, n := range []int{1, 7, 50} {
		m := newBareModel(t, func(c *config.Config) { c.ListPageSize = n })
		if m.pageSize != n {
			t.Errorf("pageSize %d, want %d", m.pageSize, n)
		}
	}
}

// newBareModel builds a model with an empty DB and the adjusted config.
func newBareModel(t *testing.T, tweak func(*config.Config)) Model {
	t.Helper()
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	cfg := config.Defaults()
	tweak(&cfg)
	return New(database, cfg)
}

// A failing action does not reload the list: the command returns nil instead
// of a tasksLoadedMsg with the old data, which was what was seen.
func TestTaskActionCmdErrorYieldsNoMsg(t *testing.T) {
	m := newTestModel(t)
	boom := func(int64) (*model.Task, error) { return nil, errActionFailed }

	cmd := m.taskActionCmd(1, boom)
	if cmd == nil {
		t.Fatal("it produced no command")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("a failed action returned %T, want nil (no reload)", msg)
	}
}

// If the action goes well, the command reloads and brings the list.
func TestTaskActionCmdSuccessReloads(t *testing.T) {
	m := newTestModel(t)
	ok := func(id int64) (*model.Task, error) { return m.database.DoneTask(id) }

	msgs := mustRun(t, m.taskActionCmd(m.tasks[0].ID, ok))
	if len(msgs) != 1 {
		t.Fatalf("messages %d, want 1", len(msgs))
	}
	loaded, isLoaded := msgs[0].(tasksLoadedMsg)
	if !isLoaded {
		t.Fatalf("message %T, want tasksLoadedMsg", msgs[0])
	}
	if len(loaded.tasks) == 0 {
		t.Error("the reload did not bring tasks")
	}
}

// projectsLoadedMsg with a pending project selects it and clears the pending
// one; with no pending one, it leaves the cursor where it was.
func TestProjectsLoadedSelectsPendingName(t *testing.T) {
	m := newTestModel(t)
	m.pendingSelectName = "web"
	idxBefore := m.dashProjectIdx

	next, _ := m.Update(projectsLoadedMsg{projects: m.projects})
	got := next.(Model)
	if got.dashProjectIdx == idxBefore {
		t.Errorf("it did not select the pending project (dashProjectIdx %d)", got.dashProjectIdx)
	}
	if got.pendingSelectName != "" {
		t.Errorf("the pending one stayed at %q, want empty after being consumed", got.pendingSelectName)
	}
}

func TestProjectsLoadedWithoutPendingKeepsSelection(t *testing.T) {
	m := newTestModel(t)
	m.pendingSelectName = ""
	m.dashProjectIdx = 0

	next, _ := m.Update(projectsLoadedMsg{projects: m.projects})
	got := next.(Model)
	if got.dashProjectIdx != 0 {
		t.Errorf("without a pending one the selection changed to %d", got.dashProjectIdx)
	}
	if got.pendingSelectName != "" {
		t.Errorf("a pending one %q appeared without being requested", got.pendingSelectName)
	}
}

// With the detail already open and comments loaded, reopening it with Enter
// goes back to leaving the selection at -1.
func TestReopeningDetailClearsCommentSelection(t *testing.T) {
	m := newTestModel(t)
	m.cursor = 0
	m.detailOpen = false
	m.detailTask = nil
	m.detailCommentSel = 3

	next, _ := press(m, "enter")
	got := next

	if !got.detailOpen {
		t.Fatal("Enter did not open the detail")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1 (nothing selected)", got.detailCommentSel)
	}
	if got.detailComments != nil {
		t.Error("comments start unloaded, not as an empty list")
	}
}

// Opening the tag modal leaves the suggestion index at -1, for the same reason
// as the detail with the comments.
func TestOpeningTagModalClearsSuggestionIndex(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	m.tagInput = "something"
	m.tagSuggestIdx = 4

	next, _ := press(m, "t")
	got := next

	if !got.tagOpen {
		t.Fatal("the t key did not open the tag modal")
	}
	if got.tagSuggestIdx != -1 {
		t.Errorf("tagSuggestIdx = %d, want -1", got.tagSuggestIdx)
	}
	if got.tagInput != "" {
		t.Errorf("the input stayed at %q, want empty", got.tagInput)
	}
}

// In Kanban, opening the detail on the selected card also clears the
// comment selection.
func TestKanbanOpeningDetailClearsCommentSelection(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewKanban
	cols := m.kanbanColumns()
	m.kanbanCol = clampTo(m.kanbanCol, len(cols))
	m.kanbanRow = clampTo(m.kanbanRow, len(cols[m.kanbanCol].tasks))
	m.detailCommentSel = 2

	next, _ := press(m, "enter")
	got := next

	if !got.detailOpen {
		t.Fatal("Enter in Kanban did not open the detail")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1", got.detailCommentSel)
	}
}

// With a single column there is nowhere to move with the horizontal arrows,
// and the row is not reset: the reset is a consequence of changing column,
// not of pressing the key. With more than one column, moving does reset.
func TestKanbanColumnArrowsWithASingleColumn(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 2, []string{"todo", "done"})
	if len(m.kanbanColumns()) != 2 {
		t.Skipf("the fixture has %d columns", len(m.kanbanColumns()))
	}
	m.kanbanCol = 0
	m.kanbanRow = 1

	next, _ := press(m, "h")
	if next.kanbanRow != 1 {
		t.Errorf("with a single column, moving left changed the row to %d, want 1", next.kanbanRow)
	}

	// And with two columns, moving left from the first one does not move and the
	// row stays: there was no column change.
	next2, _ := press(m, "h")
	if next2.kanbanCol != 0 {
		t.Errorf("the column moved to %d from the edge", next2.kanbanCol)
	}
	if next2.kanbanRow != 1 {
		t.Errorf("the row was reset to %d without changing column", next2.kanbanRow)
	}
}
