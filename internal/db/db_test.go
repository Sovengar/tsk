package db

import (
	"testing"

	"tsk/internal/model"
)

// --- Projects ---

func TestCreateAndGetProject(t *testing.T) {
	db := newTestDB(t)
	p, err := db.CreateProject("api", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "api" {
		t.Errorf("name = %q, want api", p.Name)
	}
	if len(p.Workflow) != len(model.DefaultWorkflow) {
		t.Errorf("workflow len = %d, want %d", len(p.Workflow), len(model.DefaultWorkflow))
	}

	got, err := db.GetProject("api")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID {
		t.Errorf("id = %d, want %d", got.ID, p.ID)
	}
}

func TestCreateProjectDuplicate(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.CreateProject("api", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProject("api", nil); err == nil {
		t.Error("expected duplicate error")
	}
}

func TestCreateProjectCustomWorkflow(t *testing.T) {
	db := newTestDB(t)
	wf := []string{"todo", "doing", "done"}
	p, err := db.CreateProject("web", wf)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Workflow) != 3 || p.Workflow[1] != "doing" {
		t.Errorf("workflow = %v, want [todo doing done]", p.Workflow)
	}
}

func TestCreateProjectWithListOrder(t *testing.T) {
	db := newTestDB(t)
	p, err := db.CreateProjectWithListOrder("web",
		[]string{"todo", "doing", "reviewing", "done"},
		[]string{"reviewing", "doing", "todo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ListOrder) != 3 || p.ListOrder[0] != "reviewing" || p.ListOrder[2] != "todo" {
		t.Errorf("list_order = %v, want [reviewing doing todo]", p.ListOrder)
	}

	got, err := db.GetProject("web")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ListOrder) != 3 || got.ListOrder[1] != "doing" {
		t.Errorf("persisted list_order = %v, want [reviewing doing todo]", got.ListOrder)
	}
}

func TestCreateProjectInvalidListOrder(t *testing.T) {
	db := newTestDB(t)
	_, err := db.CreateProjectWithListOrder("web",
		[]string{"todo", "doing", "done"},
		[]string{"nope"})
	if err == nil {
		t.Error("expected error for status not in workflow")
	}
}

func TestUpdateProjectListOrder(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "web", nil)

	if err := db.UpdateProject("web", map[string]any{
		"list_order": []string{"reviewing", "backlog"},
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetProject("web")
	if len(got.ListOrder) != 2 || got.ListOrder[0] != "reviewing" || got.ListOrder[1] != "backlog" {
		t.Errorf("list_order = %v, want [reviewing backlog]", got.ListOrder)
	}

	// Limpiar con lista vacía.
	if err := db.UpdateProject("web", map[string]any{"list_order": []string{}}); err != nil {
		t.Fatal(err)
	}
	got, _ = db.GetProject("web")
	if len(got.ListOrder) != 0 {
		t.Errorf("list_order = %v, want empty", got.ListOrder)
	}
}

func TestUpdateProjectWorkflowFiltersListOrder(t *testing.T) {
	db := newTestDB(t)
	mustCreateProjectWithListOrder(t, db, "web",
		[]string{"todo", "doing", "reviewing", "done"},
		[]string{"reviewing", "doing", "todo"})

	// Quitar "doing" del workflow debe descartarlo también de list_order.
	if err := db.UpdateProject("web", map[string]any{
		"workflow": []string{"todo", "reviewing", "done"},
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetProject("web")
	if len(got.ListOrder) != 2 || got.ListOrder[0] != "reviewing" || got.ListOrder[1] != "todo" {
		t.Errorf("list_order = %v, want [reviewing todo]", got.ListOrder)
	}
}

func TestListProjects(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "a", nil)
	mustCreateProject(t, db, "b", nil)

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("count = %d, want 2", len(projects))
	}
	// Ordenado por nombre
	if projects[0].Name != "a" || projects[1].Name != "b" {
		t.Errorf("order = [%s %s], want [a b]", projects[0].Name, projects[1].Name)
	}
}

func TestUpdateProjectWorkflow(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	err := db.UpdateProject("api", map[string]any{
		"workflow": []string{"todo", "done"},
	})
	if err != nil {
		t.Fatal(err)
	}

	p, _ := db.GetProject("api")
	if len(p.Workflow) != 2 {
		t.Errorf("workflow = %v, want [todo done]", p.Workflow)
	}
}

func TestUpdateProjectRejectRemoveStatusWithTasks(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "task1", "", "", 0, "reviewing")

	err := db.UpdateProject("api", map[string]any{
		"workflow": []string{"todo", "doing", "done"},
	})
	if err == nil {
		t.Error("expected error removing status with tasks")
	}
}

func TestUpdateProjectForce(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "task1", "", "", 0, "reviewing")

	err := db.UpdateProject("api", map[string]any{
		"workflow": []string{"todo", "doing", "done"},
		"force":    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tarea debe haberse movido al primer estado
	tasks, _ := db.ListTasks("api", "", "")
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	if tasks[0].Status != "todo" {
		t.Errorf("status = %q, want todo (reassigned by force)", tasks[0].Status)
	}
}

func TestDeleteProject(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "task1", "", "", 0, "")

	err := db.DeleteProject("api")
	if err != nil {
		t.Fatal(err)
	}

	projects, _ := db.ListProjects()
	if len(projects) != 0 {
		t.Errorf("projects = %d, want 0", len(projects))
	}
	// CASCADE: tareas también deben eliminarse
	tasks, _ := db.ListTasks("api", "", "")
	if len(tasks) != 0 {
		t.Errorf("tasks = %d, want 0 (cascade)", len(tasks))
	}
}

func TestDeleteProjectNotFound(t *testing.T) {
	db := newTestDB(t)
	err := db.DeleteProject("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestGetProjectNotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := db.GetProject("nope")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetProjectByID(t *testing.T) {
	db := newTestDB(t)
	p, _ := db.CreateProject("api", nil)

	got, err := db.GetProjectByID(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "api" {
		t.Errorf("name = %q, want api", got.Name)
	}
}

func TestProjectTaskCount(t *testing.T) {
	db := newTestDB(t)
	p, _ := db.CreateProject("api", nil)
	mustCreateTask(t, db, "api", "t1", "", "", 0, "")
	mustCreateTask(t, db, "api", "t2", "", "", 0, "")

	count, err := db.ProjectTaskCount(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

// --- Tasks ---

func TestCreateTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	task, err := db.CreateTask("api", "Fix auth", "description here", "@juan", 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Fix auth" {
		t.Errorf("title = %q", task.Title)
	}
	if task.Status != "backlog" {
		t.Errorf("status = %q, want backlog", task.Status)
	}
	if task.Priority != 3 {
		t.Errorf("priority = %d, want 3", task.Priority)
	}
	if task.Assignee != "@juan" {
		t.Errorf("assignee = %q, want @juan", task.Assignee)
	}
	if task.ProjectName != "api" {
		t.Errorf("project = %q, want api", task.ProjectName)
	}
}

func TestCreateTaskCustomStatus(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	task, err := db.CreateTask("api", "task", "", "", 0, "doing")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "doing" {
		t.Errorf("status = %q, want doing", task.Status)
	}
}

func TestCreateTaskInvalidStatus(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	_, err := db.CreateTask("api", "task", "", "", 0, "invalid")
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestCreateTaskUnknownProject(t *testing.T) {
	db := newTestDB(t)
	_, err := db.CreateTask("nope", "task", "", "", 0, "")
	if err == nil {
		t.Error("expected error for unknown project")
	}
}

func TestGetTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	created, _ := db.CreateTask("api", "Fix N+1", "desc", "@juan", 3, "")

	got, err := db.GetTask(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Fix N+1" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Description != "desc" {
		t.Errorf("description = %q", got.Description)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := db.GetTask(999)
	if err == nil {
		t.Error("expected error")
	}
}

func TestListTasksFilters(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateProject(t, db, "web", nil)
	mustCreateTask(t, db, "api", "t1", "", "@juan", 3, "backlog")
	mustCreateTask(t, db, "api", "t2", "", "@maria", 2, "doing")
	mustCreateTask(t, db, "web", "t3", "", "@juan", 1, "todo")

	// Filter by project
	tasks, _ := db.ListTasks("api", "", "")
	if len(tasks) != 2 {
		t.Errorf("project filter: %d, want 2", len(tasks))
	}

	// Filter by status
	tasks, _ = db.ListTasks("", "doing", "")
	if len(tasks) != 1 || tasks[0].Title != "t2" {
		t.Errorf("status filter: %v", tasks)
	}

	// Filter by assignee
	tasks, _ = db.ListTasks("", "", "@juan")
	if len(tasks) != 2 {
		t.Errorf("assignee filter: %d, want 2", len(tasks))
	}

	// Combined
	tasks, _ = db.ListTasks("api", "", "@juan")
	if len(tasks) != 1 || tasks[0].Title != "t1" {
		t.Errorf("combined filter: %v", tasks)
	}
}

func TestListTasksOrderByPriority(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "low", "", "", 1, "")
	mustCreateTask(t, db, "api", "high", "", "", 3, "")
	mustCreateTask(t, db, "api", "med", "", "", 2, "")

	tasks, _ := db.ListTasks("", "", "")
	if len(tasks) != 3 {
		t.Fatalf("count = %d, want 3", len(tasks))
	}
	if tasks[0].Title != "high" || tasks[1].Title != "med" || tasks[2].Title != "low" {
		t.Errorf("order = [%s %s %s], want [high med low]",
			tasks[0].Title, tasks[1].Title, tasks[2].Title)
	}
}

func TestListTasksOrderByPriorityStatusAssignee(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "backlog-a", "", "@a", 2, "backlog")
	mustCreateTask(t, db, "api", "backlog-b", "", "@b", 2, "backlog")
	mustCreateTask(t, db, "api", "todo-a", "", "@a", 2, "todo")
	mustCreateTask(t, db, "api", "in-progress", "", "@a", 2, "doing")
	mustCreateTask(t, db, "api", "reviewing", "", "@a", 2, "reviewing")
	mustCreateTask(t, db, "api", "done", "", "@a", 2, "done")
	mustCreateTask(t, db, "api", "low", "", "@a", 1, "todo")

	// Prioridad desc, luego el orden literal del workflow (backlog > todo >
	// doing > reviewing > done), luego assignee asc.
	tasks, _ := db.ListTasks("", "", "")
	want := []string{"backlog-a", "backlog-b", "todo-a", "in-progress", "reviewing", "done", "low"}
	if len(tasks) != len(want) {
		t.Fatalf("count = %d, want %d", len(tasks), len(want))
	}
	for i, title := range want {
		if tasks[i].Title != title {
			t.Errorf("order[%d] = %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestListTasksOrderCustomWorkflow(t *testing.T) {
	db := newTestDB(t)
	// El array del proyecto define el orden del listado tal cual.
	mustCreateProject(t, db, "web", []string{"reviewing", "doing", "todo", "done"})
	mustCreateTask(t, db, "web", "todo", "", "@a", 2, "todo")
	mustCreateTask(t, db, "web", "doing", "", "@a", 2, "doing")
	mustCreateTask(t, db, "web", "reviewing", "", "@a", 2, "reviewing")
	mustCreateTask(t, db, "web", "done", "", "@a", 2, "done")

	tasks, _ := db.ListTasks("", "", "")
	want := []string{"reviewing", "doing", "todo", "done"}
	if len(tasks) != len(want) {
		t.Fatalf("count = %d, want %d", len(tasks), len(want))
	}
	for i, title := range want {
		if tasks[i].Title != title {
			t.Errorf("order[%d] = %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestListTasksOrderByListOrder(t *testing.T) {
	db := newTestDB(t)
	// workflow = progresión (acciones); list_order = orden de presentación.
	mustCreateProjectWithListOrder(t, db, "web",
		[]string{"todo", "doing", "reviewing", "done"},
		[]string{"reviewing", "doing", "todo"})
	mustCreateTask(t, db, "web", "todo", "", "@a", 2, "todo")
	mustCreateTask(t, db, "web", "doing", "", "@a", 2, "doing")
	mustCreateTask(t, db, "web", "reviewing", "", "@a", 2, "reviewing")
	mustCreateTask(t, db, "web", "done", "", "@a", 2, "done")
	cancelled, _ := db.CreateTask("web", "cancelled", "", "@a", 2, "todo")
	mustMoveTask(t, db, cancelled.ID, "cancelled")

	// reviewing > doing > todo (list_order), luego done (no listado pero en el
	// workflow) y al final cancelled (ni en list_order ni en workflow).
	tasks, _ := db.ListTasks("", "", "")
	want := []string{"reviewing", "doing", "todo", "done", "cancelled"}
	if len(tasks) != len(want) {
		t.Fatalf("count = %d, want %d", len(tasks), len(want))
	}
	for i, title := range want {
		if tasks[i].Title != title {
			t.Errorf("order[%d] = %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestListTasksEmptyListOrderFallsBackToWorkflow(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "web", []string{"reviewing", "doing", "todo", "done"})
	mustCreateTask(t, db, "web", "todo", "", "@a", 2, "todo")
	mustCreateTask(t, db, "web", "doing", "", "@a", 2, "doing")
	mustCreateTask(t, db, "web", "reviewing", "", "@a", 2, "reviewing")
	mustCreateTask(t, db, "web", "done", "", "@a", 2, "done")

	tasks, _ := db.ListTasks("", "", "")
	want := []string{"reviewing", "doing", "todo", "done"}
	if len(tasks) != len(want) {
		t.Fatalf("count = %d, want %d", len(tasks), len(want))
	}
	for i, title := range want {
		if tasks[i].Title != title {
			t.Errorf("order[%d] = %q, want %q", i, tasks[i].Title, title)
		}
	}
}

func TestMoveTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	moved, err := db.MoveTask(task.ID, "doing")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Status != "doing" {
		t.Errorf("status = %q, want doing", moved.Status)
	}
}

func TestMoveTaskInvalidStatus(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	_, err := db.MoveTask(task.ID, "invalid")
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestMoveTaskSetsCompletedAt(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	done, err := db.MoveTask(task.ID, "done")
	if err != nil {
		t.Fatal(err)
	}
	if done.CompletedAt == "" {
		t.Error("completed_at should be set for terminal status")
	}
}

func TestMoveTaskCancelledSetsCompletedAt(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	cancelled, err := db.MoveTask(task.ID, model.CancelledStatus)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.CompletedAt == "" {
		t.Error("completed_at should be set for cancelled")
	}
}

func TestStartTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil) // workflow: backlog,todo,doing,reviewing,done
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	started, err := db.StartTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != "todo" {
		t.Errorf("status = %q, want todo (2nd status after backlog)", started.Status)
	}
}

func TestStartTaskWithoutBacklog(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "simple", []string{"todo", "doing", "done"})
	task, _ := db.CreateTask("simple", "task", "", "", 0, "")

	started, err := db.StartTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != "todo" {
		t.Errorf("status = %q, want todo (1st status, no backlog)", started.Status)
	}
}

func TestDoneTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "doing")

	done, err := db.DoneTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "done" {
		t.Errorf("status = %q, want done", done.Status)
	}
	if done.CompletedAt == "" {
		t.Error("completed_at should be set")
	}
}

func TestCancelTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	cancelled, err := db.CancelTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != model.CancelledStatus {
		t.Errorf("status = %q, want cancelled", cancelled.Status)
	}
}

func TestReviewTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "doing")

	reviewed, err := db.ReviewTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Status != "reviewing" {
		t.Errorf("status = %q, want reviewing", reviewed.Status)
	}
}

func TestReviewTaskNoReviewStatus(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "simple", []string{"todo", "done"})
	task, _ := db.CreateTask("simple", "task", "", "", 0, "")

	_, err := db.ReviewTask(task.ID)
	if err == nil {
		t.Error("expected error: no reviewing status in workflow")
	}
}

func TestUpdateTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "old title", "old desc", "@juan", 1, "")

	updated, err := db.UpdateTask(task.ID, map[string]any{
		"title":    "new title",
		"priority": 3,
		"assignee": "@maria",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "new title" {
		t.Errorf("title = %q", updated.Title)
	}
	if updated.Priority != 3 {
		t.Errorf("priority = %d", updated.Priority)
	}
	if updated.Assignee != "@maria" {
		t.Errorf("assignee = %q", updated.Assignee)
	}
	// Description unchanged
	if updated.Description != "old desc" {
		t.Errorf("description changed: %q", updated.Description)
	}
}

// --- Stats ---

func TestStatsGlobal(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateProject(t, db, "web", nil)
	mustCreateTask(t, db, "api", "t1", "", "@juan", 3, "backlog")
	mustCreateTask(t, db, "api", "t2", "", "@maria", 2, "doing")
	mustCreateTask(t, db, "web", "t3", "", "@juan", 1, "done")

	stats, err := db.Stats("")
	if err != nil {
		t.Fatal(err)
	}
	if stats["total"] != 3 {
		t.Errorf("total = %v, want 3", stats["total"])
	}

	byStatus := stats["by_status"].(map[string]int)
	if byStatus["backlog"] != 1 || byStatus["doing"] != 1 || byStatus["done"] != 1 {
		t.Errorf("by_status = %v", byStatus)
	}

	byAssignee := stats["by_assignee"].(map[string]int)
	if byAssignee["@juan"] != 2 || byAssignee["@maria"] != 1 {
		t.Errorf("by_assignee = %v", byAssignee)
	}
}

func TestStatsByProject(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateProject(t, db, "web", nil)
	mustCreateTask(t, db, "api", "t1", "", "@juan", 0, "")
	mustCreateTask(t, db, "api", "t2", "", "@maria", 0, "")
	mustCreateTask(t, db, "web", "t3", "", "@juan", 0, "")

	stats, err := db.Stats("api")
	if err != nil {
		t.Fatal(err)
	}
	if stats["total"] != 2 {
		t.Errorf("total = %v, want 2", stats["total"])
	}
}

// --- Archive (soft delete) ---

func TestArchiveProjectHidesTasksAndStats(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateProject(t, db, "web", nil)
	mustCreateTask(t, db, "api", "t1", "", "@juan", 0, "")
	mustCreateTask(t, db, "web", "t2", "", "@maria", 0, "")

	if err := db.ArchiveProject("api"); err != nil {
		t.Fatal(err)
	}

	projects, _ := db.ListProjects()
	if len(projects) != 1 || projects[0].Name != "web" {
		t.Errorf("active projects = %v, want [web]", projects)
	}

	archived, _ := db.ListArchivedProjects()
	if len(archived) != 1 || archived[0].Name != "api" {
		t.Errorf("archived projects = %v, want [api]", archived)
	}
	if !archived[0].Archived {
		t.Error("archived project should have Archived=true")
	}

	tasks, _ := db.ListTasks("", "", "")
	if len(tasks) != 1 || tasks[0].ProjectName != "web" {
		t.Errorf("tasks = %v, want only web's", tasks)
	}

	stats, _ := db.Stats("")
	if stats["total"] != 1 {
		t.Errorf("stats total = %v, want 1", stats["total"])
	}
}

func TestUnarchiveProjectRestores(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "t1", "", "", 0, "")
	mustArchiveProject(t, db, "api")

	if err := db.UnarchiveProject("api"); err != nil {
		t.Fatal(err)
	}
	projects, _ := db.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(projects))
	}
	if projects[0].Archived {
		t.Error("project should not be archived")
	}
	tasks, _ := db.ListTasks("", "", "")
	if len(tasks) != 1 {
		t.Errorf("tasks = %d, want 1 restored", len(tasks))
	}
}

func TestCreateTaskOnArchivedProjectFails(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustArchiveProject(t, db, "api")

	if _, err := db.CreateTask("api", "t", "", "", 0, ""); err == nil {
		t.Error("expected error creating task on archived project")
	}
}

func TestArchiveProjectErrors(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	if err := db.ArchiveProject("api"); err != nil {
		t.Fatal(err)
	}
	if err := db.ArchiveProject("api"); err == nil {
		t.Error("expected error archiving twice")
	}
	if err := db.UnarchiveProject("nope"); err == nil {
		t.Error("expected error unarchiving nonexistent project")
	}
}

func TestUpdateProjectRename(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateTask(t, db, "api", "t1", "", "", 0, "")

	if err := db.UpdateProject("api", map[string]any{"name": "backend"}); err != nil {
		t.Fatal(err)
	}
	p, err := db.GetProject("backend")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "backend" {
		t.Errorf("name = %q, want backend", p.Name)
	}
	// La tarea referencia project_id, así que sobrevive al rename.
	tasks, _ := db.ListTasks("", "", "")
	if len(tasks) != 1 || tasks[0].ProjectName != "backend" {
		t.Errorf("tasks after rename = %v", tasks)
	}
}

func TestUpdateProjectDuplicateName(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateProject(t, db, "web", nil)

	if err := db.UpdateProject("api", map[string]any{"name": "web"}); err == nil {
		t.Error("expected duplicate name error")
	}
}

func TestUpdateProjectEmptyNameRejected(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	if err := db.UpdateProject("api", map[string]any{"name": "  "}); err == nil {
		t.Error("expected empty name error")
	}
}

// --- helpers ---

func newTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
