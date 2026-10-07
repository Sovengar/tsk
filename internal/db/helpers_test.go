package db

import "testing"

// Setup helpers: they fail the test if creation fails, instead of discarding
// the error. They do not change the test's behavior — only how a setup
// failure is reported, which used to be silently ignored.

func mustCreateProject(t *testing.T, db *DB, name string, workflow []string) {
	t.Helper()
	if _, err := db.CreateProject(name, workflow); err != nil {
		t.Fatalf("CreateProject(%q): %v", name, err)
	}
}

func mustCreateProjectWithListOrder(t *testing.T, db *DB, name string, workflow, listOrder []string) {
	t.Helper()
	if _, err := db.CreateProjectWithListOrder(name, workflow, listOrder); err != nil {
		t.Fatalf("CreateProjectWithListOrder(%q): %v", name, err)
	}
}

func mustCreateTask(t *testing.T, db *DB, projectName, title, description, assignee string, priority int, status string) {
	t.Helper()
	if _, err := db.CreateTask(projectName, title, description, assignee, priority, status); err != nil {
		t.Fatalf("CreateTask(%q): %v", title, err)
	}
}

func mustCreateTaskWithEstimate(t *testing.T, db *DB, projectName, title, description, assignee string, priority int, status string, estimate float64) {
	t.Helper()
	if _, err := db.CreateTaskWithEstimate(projectName, title, description, assignee, priority, status, estimate); err != nil {
		t.Fatalf("CreateTaskWithEstimate(%q): %v", title, err)
	}
}

func mustArchiveProject(t *testing.T, db *DB, name string) {
	t.Helper()
	if err := db.ArchiveProject(name); err != nil {
		t.Fatalf("ArchiveProject(%q): %v", name, err)
	}
}

func mustMoveTask(t *testing.T, db *DB, id int64, status string) {
	t.Helper()
	if _, err := db.MoveTask(id, status); err != nil {
		t.Fatalf("MoveTask(%d): %v", id, err)
	}
}

func mustAddComment(t *testing.T, db *DB, taskID int64, body string) {
	t.Helper()
	if _, err := db.AddComment(taskID, body); err != nil {
		t.Fatalf("AddComment(%d): %v", taskID, err)
	}
}
