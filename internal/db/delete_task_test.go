package db

import "testing"

func TestDeleteTask(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, err := db.CreateTask("api", "task", "desc", "@john", 2, "backlog")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := db.DeleteTask(task.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := db.GetTask(task.ID); err == nil {
		t.Error("task still present after DeleteTask")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.DeleteTask(9999); err == nil {
		t.Error("DeleteTask with unknown id: want error")
	}
}

func TestDeleteTaskCascadesComments(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, err := db.CreateTask("api", "task", "", "", 0, "backlog")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	mustAddComment(t, db, task.ID, "hello")

	if err := db.DeleteTask(task.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	comments, err := db.ListComments(task.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("comments left after delete: %d", len(comments))
	}
}
