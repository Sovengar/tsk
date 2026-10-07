package db

import "testing"

func TestAddAndListComments(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	if _, err := db.AddComment(task.ID, "first comment"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddComment(task.ID, "second comment"); err != nil {
		t.Fatal(err)
	}

	comments, err := db.ListComments(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 {
		t.Fatalf("count = %d, want 2", len(comments))
	}
	// Ascending chronological order
	if comments[0].Body != "first comment" || comments[1].Body != "second comment" {
		t.Errorf("order = [%q %q]", comments[0].Body, comments[1].Body)
	}
	if comments[0].TaskID != task.ID {
		t.Errorf("task_id = %d, want %d", comments[0].TaskID, task.ID)
	}
	if comments[0].CreatedAt == "" {
		t.Error("created_at should be set")
	}
}

func TestAddCommentTrimsBody(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	c, err := db.AddComment(task.ID, "  with spaces  ")
	if err != nil {
		t.Fatal(err)
	}
	if c.Body != "with spaces" {
		t.Errorf("body = %q, want %q", c.Body, "with spaces")
	}
}

func TestAddCommentEmptyBody(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	if _, err := db.AddComment(task.ID, "   "); err == nil {
		t.Error("expected error for empty body")
	}
}

func TestAddCommentUnknownTask(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.AddComment(999, "hello"); err == nil {
		t.Error("expected error for unknown task")
	}
}

func TestDeleteComment(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")

	c, _ := db.AddComment(task.ID, "to delete")
	if err := db.DeleteComment(c.ID); err != nil {
		t.Fatal(err)
	}

	comments, _ := db.ListComments(task.ID)
	if len(comments) != 0 {
		t.Errorf("count = %d, want 0", len(comments))
	}
}

func TestDeleteCommentNotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.DeleteComment(999); err == nil {
		t.Error("expected error for nonexistent comment")
	}
}

func TestCommentsCascadeOnProjectDelete(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "", 0, "")
	mustAddComment(t, db, task.ID, "note")

	if err := db.DeleteProject("api"); err != nil {
		t.Fatal(err)
	}

	comments, err := db.ListComments(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 0 {
		t.Errorf("comments = %d, want 0 (cascade)", len(comments))
	}
}
