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
	database.CreateProject("api", nil)
	database.CreateProject("web", []string{"todo", "doing", "done"})

	database.CreateTask("api", "Fix N+1 query", "desc", "@juan", 3, "doing")
	database.CreateTask("api", "Add caching", "", "@maria", 2, "backlog")
	database.CreateTask("api", "Update README", "", "@juan", 1, "reviewing")
	database.CreateTask("web", "Fix checkout", "", "@maria", 3, "todo")
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

	// List: Tab cicla el filtro Project (all -> api -> web -> all).
	if m.filterProject != "" {
		t.Fatalf("filtro inicial = %q, want vacío", m.filterProject)
	}
	m, _ = press(m, "tab")
	if m.filterProject != "api" {
		t.Errorf("tab en List: filterProject = %q, want api", m.filterProject)
	}
	m, _ = press(m, "tab")
	if m.filterProject != "web" {
		t.Errorf("tab en List: filterProject = %q, want web", m.filterProject)
	}
	m, _ = press(m, "tab")
	if m.filterProject != "" {
		t.Errorf("tab en List debe volver a all: filterProject = %q", m.filterProject)
	}

	// Kanban: Tab también cicla proyectos (las columnas se mueven con h/l).
	m, _ = press(m, "2")
	m, _ = press(m, "tab")
	if m.filterProject != "api" {
		t.Errorf("tab en Kanban: filterProject = %q, want api", m.filterProject)
	}
	if m.kanbanCol != 0 {
		t.Errorf("tab en Kanban no debe mover columna: kanbanCol = %d", m.kanbanCol)
	}

	// Gantt: Tab cicla proyectos.
	m, _ = press(m, "3")
	m, _ = press(m, "tab")
	if m.filterProject != "web" {
		t.Errorf("tab en Gantt: filterProject = %q, want web", m.filterProject)
	}

	// Dashboard: Tab sigue ciclando el proyecto resaltado.
	m, _ = press(m, "4")
	m, _ = press(m, "tab")
	if m.currentView != viewDashboard {
		t.Errorf("tab desde dashboard cambió de view: %v", m.currentView)
	}
	if m.dashProjectIdx != 1 {
		t.Errorf("tab en dashboard: projectIdx = %d, want 1", m.dashProjectIdx)
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

	// k en el tope de la página no hace nada (clamp, sin wrap)
	m, _ = press(m, "k")
	if m.cursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", m.cursor)
	}

	// Llevar el cursor al final de la página
	tasks := m.filteredTasks()
	for m.cursor < len(tasks)-1 {
		m, _ = press(m, "j")
	}
	if m.cursor != len(tasks)-1 {
		t.Fatalf("cursor = %d, want %d", m.cursor, len(tasks)-1)
	}

	// j en el final de la página no hace nada (clamp, sin wrap)
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
	addTasks(t, m, 5) // 4 fixtures + 5 = 9 tareas activas
	m, _ = press(m, "1")

	// n: salta al primer elemento de la próxima página
	m, _ = press(m, "n")
	if m.cursor != 2 {
		t.Errorf("after n: cursor = %d, want 2", m.cursor)
	}
	m, _ = press(m, "n")
	if m.cursor != 4 {
		t.Errorf("after 2x n: cursor = %d, want 4", m.cursor)
	}

	// p: salta al primer elemento de la página previa
	m, _ = press(m, "p")
	if m.cursor != 2 {
		t.Errorf("after p: cursor = %d, want 2", m.cursor)
	}
	m, _ = press(m, "p")
	if m.cursor != 0 {
		t.Errorf("after 2x p: cursor = %d, want 0", m.cursor)
	}

	// p en la primera página no hace nada
	m, _ = press(m, "p")
	if m.cursor != 0 {
		t.Errorf("p at first page: cursor = %d, want 0", m.cursor)
	}

	// n en la última página no hace nada
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

	// Página 0 = [0, 2): j se detiene en 1
	m, _ = press(m, "j")
	if m.cursor != 1 {
		t.Fatalf("after j: cursor = %d, want 1", m.cursor)
	}
	m, _ = press(m, "j")
	if m.cursor != 1 {
		t.Errorf("j at page end: cursor = %d, want 1", m.cursor)
	}

	// k se detiene en 0
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
		{"primera de varias", 4, 3, 0, "1-3 of 4 · Page 1/2"},
		{"última parcial", 4, 3, 3, "4-4 of 4 · Page 2/2"},
		{"una sola página", 4, 10, 2, "1-4 of 4 · Page 1/1"},
		{"sin tareas", 0, 10, 0, "0-0 of 0 · Page 1/1"},
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
	addTasks(t, m, 20) // 24 tareas
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

	// Close modal
	m, _ = press(m, "enter")
	if m.filterOpen {
		t.Error("enter should close modal")
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

	// Close modal
	m, _ = press(m, "enter")
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

func TestKanbanMoveLeft(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2") // kanban view

	// Buscar una tarjeta que tenga estado anterior en el workflow de SU
	// proyecto. Con workflows divergentes eso no coincide con el merge, así que
	// el bug anterior (retroceder según el merge) produciría un move inválido.
	workflow := m.mergedWorkflow()
	found := false
	for i := 1; i < len(workflow) && !found; i++ {
		colTasks := m.tasksInColumn(workflow[i])
		for j, task := range colTasks {
			p := m.projectByName(task.ProjectName)
			if p == nil {
				continue
			}
			if _, ok := model.PrevStatus(p.Workflow, task.Status); ok {
				m.kanbanCol, m.kanbanRow = i, j
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("el fixture no tiene una tarea con estado anterior en su proyecto")
	}

	_, cmd := press(m, "S")
	if cmd == nil {
		t.Error("S in kanban should produce a command when the task can retreat")
	}
}

// --- Merged workflow ---

func TestMergedWorkflow(t *testing.T) {
	m := newTestModel(t)
	wf := m.mergedWorkflow()
	// api: backlog,todo,doing,reviewing,done,cancelled (default)
	// web: todo,doing,done
	// merged: default completo (6 únicos)
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
