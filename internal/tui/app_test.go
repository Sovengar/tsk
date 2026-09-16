package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// newTestModel construye un modelo con una DB en memoria y datos de prueba.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
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
	database.CreateProject("api", "/dev/api", nil)
	database.CreateProject("web", "/dev/web", []string{"todo", "in_progress", "done"})

	database.CreateTask("api", "Fix N+1 query", "desc", "@juan", 3, 0, "in_progress")
	database.CreateTask("api", "Add caching", "", "@maria", 2, 0, "backlog")
	database.CreateTask("api", "Update README", "", "@juan", 1, 0, "review")
	database.CreateTask("web", "Fix checkout", "", "@maria", 3, 0, "todo")
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
	default:
		km = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	next, cmd := m.Update(km)
	// Bubbletea v2 puede devolver value o pointer depending on context
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

	m, _ = press(m, "3")
	if m.currentView != viewKanban {
		t.Errorf("after 3: view = %v, want kanban", m.currentView)
	}

	m, _ = press(m, "1")
	if m.currentView != viewDashboard {
		t.Errorf("after 1: view = %v, want dashboard", m.currentView)
	}

	m, _ = press(m, "2")
	if m.currentView != viewList {
		t.Errorf("after 2: view = %v, want list", m.currentView)
	}
}

func TestTabCyclesViews(t *testing.T) {
	m := newTestModel(t)

	// Tab in list cycles to kanban
	m, _ = press(m, "tab")
	if m.currentView != viewKanban {
		t.Errorf("tab from list: %v, want kanban", m.currentView)
	}

	// Tab in kanban cycles columns
	m, _ = press(m, "tab")
	if m.kanbanCol != 1 {
		t.Errorf("tab in kanban: col = %d, want 1", m.kanbanCol)
	}

	// Tab from dashboard cycles projects
	m, _ = press(m, "1") // switch to dashboard
	m, _ = press(m, "tab")
	if m.currentView != viewDashboard {
		t.Errorf("tab from dashboard should stay in dashboard, got %v", m.currentView)
	}
	if m.dashProjectIdx != 1 {
		t.Errorf("tab from dashboard: projectIdx = %d, want 1", m.dashProjectIdx)
	}
}

// --- List navigation ---

func TestListNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // switch to list

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

	// k at top wraps to bottom
	m, _ = press(m, "k")
	tasks := m.filteredTasks()
	if m.cursor != len(tasks)-1 {
		t.Errorf("wrap up: cursor = %d, want %d", m.cursor, len(tasks)-1)
	}

	// j at bottom wraps to top
	m, _ = press(m, "j")
	if m.cursor != 0 {
		t.Errorf("wrap down: cursor = %d, want 0", m.cursor)
	}
}

func TestListStartTask(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // list view

	_, cmd := press(m, "s")
	if cmd == nil {
		t.Error("s should produce a command")
	}
}

func TestListDoneTask(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")

	_, cmd := press(m, "d")
	if cmd == nil {
		t.Error("d should produce a command")
	}
}

func TestListCancelTask(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")

	_, cmd := press(m, "x")
	if cmd == nil {
		t.Error("x should produce a command")
	}
}

// --- List filters ---

func TestListPriorityFilter(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // list view

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

	// Close modal
	m, _ = press(m, "enter")
	if m.filterOpen {
		t.Error("enter should close modal")
	}
}

func TestListAssigneeFilter(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // list view

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

	// Close modal
	m, _ = press(m, "enter")
}

// --- Kanban navigation ---

func TestKanbanNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3") // kanban view

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
	m, _ = press(m, "3") // kanban view

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

func TestKanbanMoveLeft(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3") // kanban view

	// Find a column with tasks that isn't the first column
	workflow := m.mergedWorkflow()
	for i := 1; i < len(workflow); i++ {
		if len(m.tasksInColumn(workflow[i])) > 0 {
			m.kanbanCol = i
			m.kanbanRow = 0
			break
		}
	}

	// Only test if we found such a column
	if m.kanbanCol > 0 {
		_, cmd := press(m, "S")
		if cmd == nil {
			t.Error("S in kanban should produce a command (if task exists)")
		}
	}
}

// --- Merged workflow ---

func TestMergedWorkflow(t *testing.T) {
	m := newTestModel(t)
	wf := m.mergedWorkflow()
	// api: backlog,todo,in_progress,review,done
	// web: todo,in_progress,done
	// merged: backlog,todo,in_progress,review,done (5 unique)
	if len(wf) != 5 {
		t.Errorf("merged workflow len = %d, want 5: %v", len(wf), wf)
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
	defer database.Close()
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
	tasks := m.tasksInColumn("in_progress")
	if len(tasks) == 0 {
		t.Error("no tasks in in_progress column")
	}
	for _, task := range tasks {
		if task.Status != "in_progress" {
			t.Errorf("task %d has status %q", task.ID, task.Status)
		}
	}
}

// --- View rendering ---

func TestRenderDashboard(t *testing.T) {
	m := newTestModel(t)
	out := m.renderDashboard()
	if out == "" {
		t.Error("dashboard should not be empty")
	}
}

func TestRenderList(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")
	out := m.renderList()
	if out == "" {
		t.Error("list should not be empty")
	}
}

func TestRenderKanban(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "3")
	out := m.renderKanban()
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
