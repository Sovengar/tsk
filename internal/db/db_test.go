package db

import (
	"testing"

	"tsk/internal/model"
)

// --- Projects ---

func TestCreateAndGetProject(t *testing.T) {
	db := newTestDB(t)
	p, err := db.CreateProject("api", "/dev/api", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "api" {
		t.Errorf("name = %q, want api", p.Name)
	}
	if len(p.Workflow) != 5 {
		t.Errorf("workflow len = %d, want 5", len(p.Workflow))
	}

	got, err := db.GetProject("api")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID {
		t.Errorf("id = %d, want %d", got.ID, p.ID)
	}
	if got.Path != "/dev/api" {
		t.Errorf("path = %q, want /dev/api", got.Path)
	}
}

func TestCreateProjectDuplicate(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.CreateProject("api", "/a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProject("api", "/b", nil); err == nil {
		t.Error("expected duplicate error")
	}
}

func TestCreateProjectCustomWorkflow(t *testing.T) {
	db := newTestDB(t)
	wf := []string{"todo", "doing", "done"}
	p, err := db.CreateProject("web", "/dev/web", wf)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Workflow) != 3 || p.Workflow[1] != "doing" {
		t.Errorf("workflow = %v, want [todo doing done]", p.Workflow)
	}
}

func TestListProjects(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("a", "/a", nil)
	db.CreateProject("b", "/b", nil)

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
	db.CreateProject("api", "/dev/api", nil)

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
	db.CreateProject("api", "/dev/api", nil)
	db.CreateTask("api", "task1", "", "", 0, 0, "review")

	err := db.UpdateProject("api", map[string]any{
		"workflow": []string{"todo", "in_progress", "done"},
	})
	if err == nil {
		t.Error("expected error removing status with tasks")
	}
}

func TestUpdateProjectForce(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	db.CreateTask("api", "task1", "", "", 0, 0, "review")

	err := db.UpdateProject("api", map[string]any{
		"workflow": []string{"todo", "in_progress", "done"},
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
	db.CreateProject("api", "/dev/api", nil)
	db.CreateTask("api", "task1", "", "", 0, 0, "")

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
	p, _ := db.CreateProject("api", "/dev/api", nil)

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
	p, _ := db.CreateProject("api", "/dev/api", nil)
	db.CreateTask("api", "t1", "", "", 0, 0, "")
	db.CreateTask("api", "t2", "", "", 0, 0, "")

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
	db.CreateProject("api", "/dev/api", nil)

	task, err := db.CreateTask("api", "Fix auth", "description here", "@juan", 3, 0, "")
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
	db.CreateProject("api", "/dev/api", nil)

	task, err := db.CreateTask("api", "task", "", "", 0, 0, "in_progress")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", task.Status)
	}
}

func TestCreateTaskInvalidStatus(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)

	_, err := db.CreateTask("api", "task", "", "", 0, 0, "invalid")
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestCreateTaskUnknownProject(t *testing.T) {
	db := newTestDB(t)
	_, err := db.CreateTask("nope", "task", "", "", 0, 0, "")
	if err == nil {
		t.Error("expected error for unknown project")
	}
}

func TestGetTask(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	created, _ := db.CreateTask("api", "Fix N+1", "desc", "@juan", 3, 0, "")

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
	db.CreateProject("api", "/dev/api", nil)
	db.CreateProject("web", "/dev/web", nil)
	db.CreateTask("api", "t1", "", "@juan", 3, 0, "backlog")
	db.CreateTask("api", "t2", "", "@maria", 2, 0, "in_progress")
	db.CreateTask("web", "t3", "", "@juan", 1, 0, "todo")

	// Filter by project
	tasks, _ := db.ListTasks("api", "", "")
	if len(tasks) != 2 {
		t.Errorf("project filter: %d, want 2", len(tasks))
	}

	// Filter by status
	tasks, _ = db.ListTasks("", "in_progress", "")
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
	db.CreateProject("api", "/dev/api", nil)
	db.CreateTask("api", "low", "", "", 1, 0, "")
	db.CreateTask("api", "high", "", "", 3, 0, "")
	db.CreateTask("api", "med", "", "", 2, 0, "")

	tasks, _ := db.ListTasks("", "", "")
	if len(tasks) != 3 {
		t.Fatalf("count = %d, want 3", len(tasks))
	}
	if tasks[0].Title != "high" || tasks[1].Title != "med" || tasks[2].Title != "low" {
		t.Errorf("order = [%s %s %s], want [high med low]",
			tasks[0].Title, tasks[1].Title, tasks[2].Title)
	}
}

func TestMoveTask(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

	moved, err := db.MoveTask(task.ID, "in_progress")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", moved.Status)
	}
}

func TestMoveTaskInvalidStatus(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

	_, err := db.MoveTask(task.ID, "invalid")
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestMoveTaskSetsCompletedAt(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

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
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

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
	db.CreateProject("api", "/dev/api", nil) // workflow: backlog,todo,in_progress,review,done
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

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
	db.CreateProject("simple", "/dev/simple", []string{"todo", "in_progress", "done"})
	task, _ := db.CreateTask("simple", "task", "", "", 0, 0, "")

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
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "in_progress")

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
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

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
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "in_progress")

	reviewed, err := db.ReviewTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Status != "review" {
		t.Errorf("status = %q, want review", reviewed.Status)
	}
}

func TestReviewTaskNoReviewStatus(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("simple", "/dev/simple", []string{"todo", "done"})
	task, _ := db.CreateTask("simple", "task", "", "", 0, 0, "")

	_, err := db.ReviewTask(task.ID)
	if err == nil {
		t.Error("expected error: no review status in workflow")
	}
}

func TestUpdateTask(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "old title", "old desc", "@juan", 1, 0, "")

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

func TestReorderTask(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, 0, "")

	err := db.ReorderTask(task.ID, 5)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := db.GetTask(task.ID)
	if got.Position != 5 {
		t.Errorf("position = %d, want 5", got.Position)
	}
}

// --- Stats ---

func TestStatsGlobal(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	db.CreateProject("web", "/dev/web", nil)
	db.CreateTask("api", "t1", "", "@juan", 3, 0, "backlog")
	db.CreateTask("api", "t2", "", "@maria", 2, 0, "in_progress")
	db.CreateTask("web", "t3", "", "@juan", 1, 0, "done")

	stats, err := db.Stats("")
	if err != nil {
		t.Fatal(err)
	}
	if stats["total"] != 3 {
		t.Errorf("total = %v, want 3", stats["total"])
	}

	byStatus := stats["by_status"].(map[string]int)
	if byStatus["backlog"] != 1 || byStatus["in_progress"] != 1 || byStatus["done"] != 1 {
		t.Errorf("by_status = %v", byStatus)
	}

	byAssignee := stats["by_assignee"].(map[string]int)
	if byAssignee["@juan"] != 2 || byAssignee["@maria"] != 1 {
		t.Errorf("by_assignee = %v", byAssignee)
	}
}

func TestStatsByProject(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", "/dev/api", nil)
	db.CreateProject("web", "/dev/web", nil)
	db.CreateTask("api", "t1", "", "@juan", 0, 0, "")
	db.CreateTask("api", "t2", "", "@maria", 0, 0, "")
	db.CreateTask("web", "t3", "", "@juan", 0, 0, "")

	stats, err := db.Stats("api")
	if err != nil {
		t.Fatal(err)
	}
	if stats["total"] != 2 {
		t.Errorf("total = %v, want 2", stats["total"])
	}
}

// --- helpers ---

func newTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
