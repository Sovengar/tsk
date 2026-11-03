package db

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
)

// openStaleDB writes a database stamped as schema 014 that never got the task
// path column, the shape a broken migration leaves behind.
func openStaleDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tsk.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer func() { _ = conn.Close() }()

	for _, m := range migrations {
		version, _ := strconv.Atoi(m.version)
		if version > 13 {
			break
		}
		if m.run != nil {
			continue
		}
		if _, err := conn.Exec(m.query); err != nil {
			t.Fatalf("seed schema %s: %v", m.version, err)
		}
	}

	for _, stmt := range []string{
		`INSERT INTO _meta (key, value) VALUES ('schema_version', '014')`,
		`INSERT INTO projects (name, path) VALUES ('p', '')`,
		`INSERT INTO tasks (project_id, title) VALUES (1, 't')`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("seed stale db: %v", err)
		}
	}
	return path
}

func TestRepairMissingTaskPathColumn(t *testing.T) {
	path := openStaleDB(t)

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen raw db: %v", err)
	}
	has, err := hasColumn(&DB{conn: raw}, "tasks", "path")
	_ = raw.Close()
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if has {
		t.Fatal("the seeded database should not have a path column")
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = database.Close() }()

	// The repair migration runs on open: the column is back and GetTask works.
	if _, err := database.GetTask(1); err != nil {
		t.Fatalf("GetTask after repair: %v", err)
	}
	if _, err := database.AddComment(1, "hello"); err != nil {
		t.Fatalf("AddComment after repair: %v", err)
	}
}

func TestEnsureTaskPathColumnIsIdempotent(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer func() { _ = database.Close() }()

	for i := range 2 {
		if err := ensureTaskPathColumn(database); err != nil {
			t.Fatalf("ensureTaskPathColumn run %d: %v", i+1, err)
		}
	}
}
