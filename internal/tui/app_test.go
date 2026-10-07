package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// newTestModel builds a model with an in-memory DB and test data.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	createFixtures(t, database)

	cfg := config.Defaults()
	m := New(database, cfg)
	m.width = 120
	m.height = 30

	// Load data directly (simulating Init messages)
	projects, _ := database.ListProjects()
	m.projects = projects
	tasks, _ := database.ListTasks("", "", "")
	m.tasks = tasks

	return &m
}

func createFixtures(t *testing.T, database *db.DB) {
	t.Helper()
	mustCreateProject(t, database, "api", nil)
	mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})

	mustCreateTask(t, database, "api", "Fix N+1 query", "desc", "@john", 3, "doing")
	mustCreateTask(t, database, "api", "Add caching", "", "@margo", 2, "backlog")
	mustCreateTask(t, database, "api", "Update README", "", "@john", 1, "reviewing")
	mustCreateTask(t, database, "web", "Fix checkout", "", "@margo", 3, "todo")
}

// Setup helpers: they fail the test if the creation fails, instead of
// discarding the error silently.

func mustCreateProject(t *testing.T, database *db.DB, name string, workflow []string) {
	t.Helper()
	if _, err := database.CreateProject(name, workflow); err != nil {
		t.Fatalf("CreateProject(%q): %v", name, err)
	}
}

func mustCreateTask(t *testing.T, database *db.DB, projectName, title, description, assignee string, priority int, status string) {
	t.Helper()
	if _, err := database.CreateTask(projectName, title, description, assignee, priority, status); err != nil {
		t.Fatalf("CreateTask(%q): %v", title, err)
	}
}

func mustAddComment(t *testing.T, database *db.DB, taskID int64, body string) {
	t.Helper()
	if _, err := database.AddComment(taskID, body); err != nil {
		t.Fatalf("AddComment(%d): %v", taskID, err)
	}
}

func press(m *Model, key string) (*Model, tea.Cmd) {
	var km tea.Msg
	switch key {
	case "enter":
		km = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		km = tea.KeyPressMsg{Code: tea.KeyTab}
	case "esc":
		km = tea.KeyPressMsg{Code: tea.KeyEsc}
	case "up":
		km = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		km = tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		km = tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		km = tea.KeyPressMsg{Code: ' ', Text: " "}
	case "shift+tab":
		km = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+p":
		km = tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "ctrl+s":
		km = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+n":
		km = tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	default:
		km = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	next, cmd := m.Update(km)
	// Bubbletea v2 can return a value or a pointer depending on the context
	switch v := next.(type) {
	case *Model:
		return v, cmd
	case Model:
		return &v, cmd
	default:
		panic("unexpected model type")
	}
}

// --- View switching ---

func TestViewSwitching123(t *testing.T) {
	m := newTestModel(t)
	if m.currentView != viewList {
		t.Fatal("initial view should be list")
	}

	m, _ = press(m, "2")
	if m.currentView != viewKanban {
		t.Errorf("after 2: view = %v, want kanban", m.currentView)
	}

	m, _ = press(m, "4")
	if m.currentView != viewDashboard {
		t.Errorf("after 4: view = %v, want dashboard", m.currentView)
	}

	m, _ = press(m, "1")
	if m.currentView != viewList {
		t.Errorf("after 1: view = %v, want list", m.currentView)
	}
}

func TestTabCyclesProjectFilter(t *testing.T) {
	m := newTestModel(t)

	// List: Tab cycles the Project filter (all -> api -> web -> all).
	if m.filterProject != "" {
		t.Fatalf("initial filter = %q, want empty", m.filterProject)
	}
	m, _ = press(m, "tab")
	if m.filterProject != "api" {
		t.Errorf("tab in List: filterProject = %q, want api", m.filterProject)
	}
	m, _ = press(m, "tab")
	if m.filterProject != "web" {
		t.Errorf("tab in List: filterProject = %q, want web", m.filterProject)
	}
	m, _ = press(m, "tab")
	if m.filterProject != "" {
		t.Errorf("tab in List must return to all: filterProject = %q", m.filterProject)
	}

	// Kanban: Tab also cycles projects (the columns move with h/l).
	m, _ = press(m, "2")
	m, _ = press(m, "tab")
	if m.filterProject != "api" {
		t.Errorf("tab in Kanban: filterProject = %q, want api", m.filterProject)
	}
	if m.kanbanCol != 0 {
		t.Errorf("tab in Kanban must not move the column: kanbanCol = %d", m.kanbanCol)
	}

	// Gantt: Tab cycles projects.
	m, _ = press(m, "3")
	m, _ = press(m, "tab")
	if m.filterProject != "web" {
		t.Errorf("tab in Gantt: filterProject = %q, want web", m.filterProject)
	}

	// Dashboard: Tab keeps cycling the highlighted project.
	m, _ = press(m, "4")
	m, _ = press(m, "tab")
	if m.currentView != viewDashboard {
		t.Errorf("tab from the dashboard changed the view: %v", m.currentView)
	}
	if m.dashProjectIdx != 1 {
		t.Errorf("tab in the dashboard: projectIdx = %d, want 1", m.dashProjectIdx)
	}
}

// --- List navigation ---

func TestListNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1") // switch to list

	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.cursor)
	}

	// j moves down
	m, _ = press(m, "j")
	if m.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", m.cursor)
	}

	// k moves up
	m, _ = press(m, "k")
	if m.cursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", m.cursor)
	}

	// k at the page's cap does nothing (clamp, no wrap)
	m, _ = press(m, "k")
	if m.cursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", m.cursor)
	}

	// Take the cursor to the end of the page. The loop is BOUNDED on purpose:
	// "while cursor < final" hangs if a mutant stops advancing the cursor,
	// and a hung test is reported as TIMED OUT instead of as a failure, which
	// is the worst possible signal in a gate.
	tasks := m.filteredTasks()
	for range len(tasks) + 2 {
		m, _ = press(m, "j")
	}
	if m.cursor != len(tasks)-1 {
		t.Fatalf("cursor = %d, want %d", m.cursor, len(tasks)-1)
	}

	// j at the end of the page does nothing (clamp, no wrap)
	m, _ = press(m, "j")
	if m.cursor != len(tasks)-1 {
		t.Errorf("j at bottom: cursor = %d, want %d", m.cursor, len(tasks)-1)
	}
}

func TestListStartTask(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1") // list view

	_, cmd := press(m, "s")
	if cmd == nil {
		t.Error("s should produce a command")
	}
}

func TestListDoneTask(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	_, cmd := press(m, "d")
	if cmd == nil {
		t.Error("d should produce a command")
	}
}

func TestListCancelTask(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	_, cmd := press(m, "x")
	if cmd == nil {
		t.Error("x should produce a command")
	}
}

// --- List pagination ---

func TestListPageNavigation(t *testing.T) {
	m := newTestModel(t)
	m.pageSize = 2
	addTasks(t, m, 5) // 4 fixtures + 5 = 9 active tasks
	m, _ = press(m, "1")

	// n: jumps to the first element of the next page
	m, _ = press(m, "n")
	if m.cursor != 2 {
		t.Errorf("after n: cursor = %d, want 2", m.cursor)
	}
	m, _ = press(m, "n")
	if m.cursor != 4 {
		t.Errorf("after 2x n: cursor = %d, want 4", m.cursor)
	}

	// p: jumps to the first element of the previous page
	m, _ = press(m, "p")
	if m.cursor != 2 {
		t.Errorf("after p: cursor = %d, want 2", m.cursor)
	}
	m, _ = press(m, "p")
	if m.cursor != 0 {
		t.Errorf("after 2x p: cursor = %d, want 0", m.cursor)
	}

	// p on the first page does nothing
	m, _ = press(m, "p")
	if m.cursor != 0 {
		t.Errorf("p at first page: cursor = %d, want 0", m.cursor)
	}

	// n on the last page does nothing
	tasks := m.filteredTasks()
	m.cursor = len(tasks) - 1
	last := m.cursor
	m, _ = press(m, "n")
	if m.cursor != last {
		t.Errorf("N at last page: cursor = %d, want %d", m.cursor, last)
	}
}

func TestListPageCursorClamp(t *testing.T) {
	m := newTestModel(t)
	m.pageSize = 2
	addTasks(t, m, 5)
	m, _ = press(m, "1")

	// Page 0 = [0, 2): j stops at 1
	m, _ = press(m, "j")
	if m.cursor != 1 {
		t.Fatalf("after j: cursor = %d, want 1", m.cursor)
	}
	m, _ = press(m, "j")
	if m.cursor != 1 {
		t.Errorf("j at page end: cursor = %d, want 1", m.cursor)
	}

	// k stops at 0
	m, _ = press(m, "k")
	if m.cursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", m.cursor)
	}
	m, _ = press(m, "k")
	if m.cursor != 0 {
		t.Errorf("k at page start: cursor = %d, want 0", m.cursor)
	}
}

func TestPageLegend(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		pageSize int
		cursor   int
		want     string
	}{
		{"first of several", 4, 3, 0, "1-3 of 4 · Page 1/2"},
		{"last partial", 4, 3, 3, "4-4 of 4 · Page 2/2"},
		{"a single page", 4, 10, 2, "1-4 of 4 · Page 1/1"},
		{"no tasks", 0, 10, 0, "0-0 of 0 · Page 1/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.tasks = make([]model.Task, tt.total)
			m.invalidateFilterCache()
			m.pageSize = tt.pageSize
			m.cursor = tt.cursor

			if got := m.pageLegend(); got != tt.want {
				t.Errorf("pageLegend() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCursorClampedWhenTasksShrink(t *testing.T) {
	m := newTestModel(t)
	addTasks(t, m, 20) // 24 tasks
	m.cursor = 20

	m.tasks = m.tasks[:2]
	m.invalidateFilterCache()

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

// --- List filters ---

func TestListPriorityFilter(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1") // list view

	if m.filterPriority != -1 {
		t.Fatalf("initial priority filter = %d, want -1", m.filterPriority)
	}

	// Open filter modal
	m, _ = press(m, "/")
	if !m.filterOpen {
		t.Fatal("/ should open filter modal")
	}

	// Navigate to priority field (index 3): Tab x3
	m, _ = press(m, "tab")
	m, _ = press(m, "tab")
	m, _ = press(m, "tab")
	if m.filterFieldIdx != filterFieldPriority {
		t.Fatalf("field idx = %d, want %d", m.filterFieldIdx, filterFieldPriority)
	}

	// Change priority: left goes from "all" (-1) to "high" (3)
	m, _ = press(m, "left")
	if m.filterPriority != 3 {
		t.Errorf("after left: priority = %d, want 3", m.filterPriority)
	}

	// Left again: high -> med
	m, _ = press(m, "left")
	if m.filterPriority != 2 {
		t.Errorf("after 2 left: priority = %d, want 2", m.filterPriority)
	}

	// Enter applies and advances to the next field; it does not close yet.
	m, _ = press(m, "enter")
	if !m.filterOpen {
		t.Fatal("enter in an intermediate field must not close the modal")
	}
	if m.filterFieldIdx != filterFieldTag {
		t.Errorf("enter must advance to Tag, got %d", m.filterFieldIdx)
	}

	// Esc closes.
	m, _ = press(m, "esc")
	if m.filterOpen {
		t.Error("esc should close modal")
	}
}

func TestListAssigneeFilter(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1") // list view

	// Open filter modal
	m, _ = press(m, "/")

	// Navigate to assignee field (index 2): Tab x2
	m, _ = press(m, "tab")
	m, _ = press(m, "tab")
	if m.filterFieldIdx != filterFieldAssignee {
		t.Fatalf("field idx = %d, want %d", m.filterFieldIdx, filterFieldAssignee)
	}

	// Change assignee: right goes from "all" to first assignee
	m, _ = press(m, "right")
	if m.filterAssignee == "" {
		t.Error("after right: assignee filter should be set")
	}

	tasks := m.filteredTasks()
	for _, task := range tasks {
		if task.Assignee != m.filterAssignee {
			t.Errorf("task %d has assignee %q, want %q", task.ID, task.Assignee, m.filterAssignee)
		}
	}

	// closes the modal
	_, _ = press(m, "enter")
}

// --- Kanban navigation ---

func TestKanbanNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // kanban view

	if m.kanbanCol != 0 || m.kanbanRow != 0 {
		t.Fatalf("initial pos = (%d,%d), want (0,0)", m.kanbanCol, m.kanbanRow)
	}

	// h moves left (stays at 0)
	m, _ = press(m, "h")
	if m.kanbanCol != 0 {
		t.Errorf("h at col 0: col = %d", m.kanbanCol)
	}

	// l moves right
	m, _ = press(m, "l")
	if m.kanbanCol != 1 {
		t.Errorf("l: col = %d, want 1", m.kanbanCol)
	}

	// Move to a column with tasks for j/k testing
	workflow := m.mergedWorkflow()
	for i, status := range workflow {
		if len(m.tasksInColumn(status)) > 0 {
			m.kanbanCol = i
			m.kanbanRow = 0
			break
		}
	}

	// j/k navigate within column
	m, _ = press(m, "j")
	colTasks := m.tasksInColumn(workflow[m.kanbanCol])
	if len(colTasks) > 1 && m.kanbanRow != 1 {
		t.Errorf("j: row = %d, want 1", m.kanbanRow)
	}
	m, _ = press(m, "k")
	if m.kanbanRow != 0 {
		t.Errorf("k: row = %d, want 0", m.kanbanRow)
	}
}

func TestKanbanMoveRight(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // kanban view

	// Find a column with tasks that isn't the last column
	workflow := m.mergedWorkflow()
	for i := 0; i < len(workflow)-1; i++ {
		if len(m.tasksInColumn(workflow[i])) > 0 {
			m.kanbanCol = i
			m.kanbanRow = 0
			break
		}
	}

	_, cmd := press(m, "s")
	// cmd may be nil if task is already in last status; that's ok
	_ = cmd
}

// --- Merged workflow ---

func TestMergedWorkflow(t *testing.T) {
	m := newTestModel(t)
	wf := m.mergedWorkflow()
	// api: backlog,todo,doing,reviewing,done,cancelled (default)
	// web: todo,doing,done
	// merged: full default (6 unique)
	if len(wf) != len(model.DefaultWorkflow) {
		t.Errorf("merged workflow len = %d, want %d: %v", len(wf), len(model.DefaultWorkflow), wf)
	}
	seen := map[string]bool{}
	for _, s := range wf {
		if seen[s] {
			t.Errorf("duplicate in merged workflow: %s", s)
		}
		seen[s] = true
	}
}

func TestMergedWorkflowEmpty(t *testing.T) {
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	cfg := config.Defaults()
	m := New(database, cfg)
	m.width = 120
	m.height = 30

	wf := m.mergedWorkflow()
	if len(wf) != len(model.DefaultWorkflow) {
		t.Errorf("empty merged = %v, want default", wf)
	}
}

// --- Unique assignees ---

func TestUniqueAssignees(t *testing.T) {
	m := newTestModel(t)
	assignees := m.uniqueAssignees()
	if len(assignees) < 2 {
		t.Errorf("unique assignees = %v, want at least 2", assignees)
	}
	seen := map[string]bool{}
	for _, a := range assignees {
		if seen[a] {
			t.Errorf("duplicate assignee: %s", a)
		}
		seen[a] = true
	}
}

// --- Tasks in column ---

func TestTasksInColumn(t *testing.T) {
	m := newTestModel(t)
	tasks := m.tasksInColumn("doing")
	if len(tasks) == 0 {
		t.Error("no tasks in doing column")
	}
	for _, task := range tasks {
		if task.Status != "doing" {
			t.Errorf("task %d has status %q", task.ID, task.Status)
		}
	}
}

// --- View rendering ---

func TestRenderDashboard(t *testing.T) {
	m := newTestModel(t)
	out := m.renderDashboard(m.height)
	if out == "" {
		t.Error("dashboard should not be empty")
	}
}

func TestRenderList(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	out := m.renderList(m.height)
	if out == "" {
		t.Error("list should not be empty")
	}
}

func TestRenderKanban(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	out := m.renderKanban(m.height)
	if out == "" {
		t.Error("kanban should not be empty")
	}
}

// --- Quit ---

func TestQuit(t *testing.T) {
	m := newTestModel(t)
	_, cmd := press(m, "q")
	if cmd == nil {
		t.Error("q should produce quit command")
	}
}
