package db

import (
	"testing"
)

func TestMoveTaskToProject(t *testing.T) {
	t.Run("happy path keeps the task data and changes the project", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})

		task, err := database.CreateTaskFull("api", "task", "the description", "@john", 2, "backlog", 0, []string{"bug", "ui"})
		if err != nil {
			t.Fatalf("CreateTaskFull: %v", err)
		}
		web, err := database.GetProject("web")
		if err != nil {
			t.Fatalf("GetProject: %v", err)
		}

		moved, err := database.MoveTaskToProject(task.ID, web.ID, "doing")
		if err != nil {
			t.Fatalf("MoveTaskToProject: %v", err)
		}
		if moved.ProjectID != web.ID || moved.ProjectName != "web" {
			t.Errorf("project = %d/%q, want %d/web", moved.ProjectID, moved.ProjectName, web.ID)
		}
		if moved.Status != "doing" {
			t.Errorf("status = %q, want doing", moved.Status)
		}
		if moved.Description != "the description" {
			t.Errorf("description = %q, want it preserved", moved.Description)
		}
		if len(moved.Tags) != 2 || moved.Tags[0] != "bug" || moved.Tags[1] != "ui" {
			t.Errorf("tags = %v, want [bug ui] preserved", moved.Tags)
		}
	})

	t.Run("invalid status fails", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})
		task, _ := database.CreateTask("api", "task", "", "", 0, "backlog")
		web, _ := database.GetProject("web")

		if _, err := database.MoveTaskToProject(task.ID, web.ID, "nope"); err == nil {
			t.Error("MoveTaskToProject with a status outside the target workflow: want error")
		}
	})

	t.Run("terminal status sets completed_at", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})
		task, _ := database.CreateTask("api", "task", "", "", 0, "backlog")
		web, _ := database.GetProject("web")

		moved, err := database.MoveTaskToProject(task.ID, web.ID, "done")
		if err != nil {
			t.Fatalf("MoveTaskToProject: %v", err)
		}
		if moved.CompletedAt == "" {
			t.Error("completed_at should be set for the target's terminal status")
		}
	})

	t.Run("cancelled sets completed_at", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})
		task, _ := database.CreateTask("api", "task", "", "", 0, "backlog")
		web, _ := database.GetProject("web")

		moved, err := database.MoveTaskToProject(task.ID, web.ID, "cancelled")
		if err != nil {
			t.Fatalf("MoveTaskToProject: %v", err)
		}
		if moved.CompletedAt == "" {
			t.Error("completed_at should be set for cancelled")
		}
	})

	t.Run("leaving terminal clears completed_at", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})
		task, _ := database.CreateTask("api", "task", "", "", 0, "backlog")
		web, _ := database.GetProject("web")

		done, err := database.MoveTask(task.ID, "done")
		if err != nil {
			t.Fatalf("MoveTask: %v", err)
		}
		if done.CompletedAt == "" {
			t.Fatal("the setup did not set completed_at")
		}

		moved, err := database.MoveTaskToProject(task.ID, web.ID, "todo")
		if err != nil {
			t.Fatalf("MoveTaskToProject: %v", err)
		}
		if moved.CompletedAt != "" {
			t.Errorf("completed_at = %q, want it cleared when leaving terminal", moved.CompletedAt)
		}
	})

	t.Run("unknown task fails", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})
		web, _ := database.GetProject("web")

		if _, err := database.MoveTaskToProject(9999, web.ID, "todo"); err == nil {
			t.Error("MoveTaskToProject with an unknown task: want error")
		}
	})

	t.Run("unknown project fails", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		task, _ := database.CreateTask("api", "task", "", "", 0, "backlog")

		if _, err := database.MoveTaskToProject(task.ID, 9999, "done"); err == nil {
			t.Error("MoveTaskToProject with an unknown project: want error")
		}
	})
}

// Read-only mode: the reads succeed and the update fails at the end.
func TestMoveTaskToProjectReadOnlyFailsAtTheWrite(t *testing.T) {
	populate := func(t *testing.T, database *DB) {
		t.Helper()
		mustCreateProject(t, database, "api", nil)
		mustCreateProject(t, database, "web", []string{"todo", "doing", "done"})
		mustCreateTask(t, database, "api", "task", "", "@john", 2, "backlog")
	}

	ro := readonlyDB(t, populate)
	web, err := ro.GetProject("web")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if _, err := ro.MoveTaskToProject(1, web.ID, "doing"); err == nil {
		t.Error("MoveTaskToProject in read-only mode: want error")
	}
}
