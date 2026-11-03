package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTaskWithPath(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)

	task, err := db.CreateTaskWithPath("api", "T", "d", "", 0, "", 0, []string{"x"}, "/work/dir")
	if err != nil {
		t.Fatal(err)
	}
	if task.Path != "/work/dir" {
		t.Errorf("created path = %q, want /work/dir", task.Path)
	}

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/work/dir" {
		t.Errorf("GetTask path = %q, want /work/dir", got.Path)
	}

	tasks, err := db.ListTasks("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Path != "/work/dir" {
		t.Errorf("ListTasks paths = %v, want [/work/dir]", tasks)
	}
}

func TestUpdateTaskPath(t *testing.T) {
	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	task, err := db.CreateTask("api", "T", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if task.Path != "" {
		t.Errorf("a task without an explicit path = %q, want empty", task.Path)
	}

	updated, err := db.UpdateTask(task.ID, map[string]any{"path": "/work/dir"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Path != "/work/dir" {
		t.Errorf("updated path = %q, want /work/dir", updated.Path)
	}

	cleared, err := db.UpdateTask(task.ID, map[string]any{"path": ""})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Path != "" {
		t.Errorf("cleared path = %q, want empty", cleared.Path)
	}
}

func TestMigrationV14AddsTheTaskPathColumn(t *testing.T) {
	db := newTestDB(t)
	rows, err := db.conn.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "path" {
			found = true
		}
	}
	if !found {
		t.Fatal("tasks.path was not created by migration 014")
	}
	var version string
	if err := db.conn.QueryRow(`SELECT value FROM _meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "015" {
		t.Errorf("schema_version = %q, want 015", version)
	}
}

// TestBackfillProjectPathsUsesTheConventionalDir checks the one-shot backfill:
// a project with no path takes ~/dev/projects/<name> when that directory
// exists. A file with the project's name, or no directory at all, leaves it empty.
func TestBackfillProjectPathsUsesTheConventionalDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	base := filepath.Join(home, "dev", "projects")
	if err := os.MkdirAll(filepath.Join(base, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "docs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	db := newTestDB(t)
	mustCreateProject(t, db, "api", nil)
	mustCreateProject(t, db, "docs", nil)
	mustCreateProject(t, db, "missing", nil)

	if err := backfillProjectPaths(db); err != nil {
		t.Fatal(err)
	}

	api, err := db.GetProject("api")
	if err != nil {
		t.Fatal(err)
	}
	if api.Path != filepath.Join(base, "api") {
		t.Errorf("api path = %q, want %q", api.Path, filepath.Join(base, "api"))
	}
	for _, name := range []string{"docs", "missing"} {
		p, err := db.GetProject(name)
		if err != nil {
			t.Fatal(err)
		}
		if p.Path != "" {
			t.Errorf("%s path = %q, want empty", name, p.Path)
		}
	}
}

func TestBackfillProjectPathsWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	db := newTestDB(t)
	if err := backfillProjectPaths(db); err != nil {
		t.Fatalf("backfill without HOME = %v, want nil", err)
	}
}

func TestBackfillProjectPathsReportsQueryErrors(t *testing.T) {
	db := newTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := backfillProjectPaths(db); err == nil {
		t.Fatal("backfill on a closed DB = nil, want an error")
	}
}

func TestMigrateV14ReportsSchemaErrors(t *testing.T) {
	db := newTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := migrateV14(db); err == nil {
		t.Fatal("migrateV14 on a closed DB = nil, want an error")
	}
}
