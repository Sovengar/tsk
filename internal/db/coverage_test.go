package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// closedDB returns a database with the connection already closed.
//
// It is the lever for the error paths of every method: SQLite answers
// "sql: database is closed" to any statement, so one test per method
// covers the `if err != nil` behind each query without having to
// invent a full disk or a corrupt table.
func closedDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return database
}

// Every DB method has an error check behind its query.
// With the connection closed all those `if`s run; what is checked is
// that the method returns the error instead of swallowing it or returning an empty
// value while being clever.
func TestEveryQueryFailsOnAClosedConnection(t *testing.T) {
	database := closedDB(t)

	t.Run("projects", func(t *testing.T) {
		if _, err := database.CreateProject("api", nil); err == nil {
			t.Error("CreateProject with no connection: want error")
		}
		if _, err := database.CreateProjectWithListOrder("api", nil, nil); err == nil {
			t.Error("CreateProjectWithListOrder with no connection: want error")
		}
		if _, err := database.GetProject("api"); err == nil {
			t.Error("GetProject with no connection: want error")
		}
		if _, err := database.GetProjectByID(1); err == nil {
			t.Error("GetProjectByID with no connection: want error")
		}
		if _, err := database.ListProjects(); err == nil {
			t.Error("ListProjects with no connection: want error")
		}
		if _, err := database.ListArchivedProjects(); err == nil {
			t.Error("ListArchivedProjects with no connection: want error")
		}
		if err := database.ArchiveProject("api"); err == nil {
			t.Error("ArchiveProject with no connection: want error")
		}
		if err := database.UnarchiveProject("api"); err == nil {
			t.Error("UnarchiveProject with no connection: want error")
		}
		if err := database.UpdateProject("api", map[string]any{"name": "api2"}); err == nil {
			t.Error("UpdateProject with no connection: want error")
		}
		if err := database.DeleteProject("api"); err == nil {
			t.Error("DeleteProject with no connection: want error")
		}
		if _, err := database.ProjectTaskCount(1); err == nil {
			t.Error("ProjectTaskCount with no connection: want error")
		}
	})

	t.Run("tasks", func(t *testing.T) {
		if _, err := database.CreateTask("api", "t", "", "", 2, "backlog"); err == nil {
			t.Error("CreateTask with no connection: want error")
		}
		if _, err := database.CreateTaskWithEstimate("api", "t", "", "", 2, "backlog", 1); err == nil {
			t.Error("CreateTaskWithEstimate with no connection: want error")
		}
		if _, err := database.CreateTaskFull("api", "t", "", "", 2, "backlog", 1, []string{"x"}); err == nil {
			t.Error("CreateTaskFull with no connection: want error")
		}
		if _, err := database.GetTask(1); err == nil {
			t.Error("GetTask with no connection: want error")
		}
		if _, err := database.ListTasks("", "", ""); err == nil {
			t.Error("ListTasks with no connection: want error")
		}
		if _, err := database.MoveTask(1, "doing"); err == nil {
			t.Error("MoveTask with no connection: want error")
		}
		if _, err := database.StartTask(1); err == nil {
			t.Error("StartTask with no connection: want error")
		}
		if _, err := database.ReviewTask(1); err == nil {
			t.Error("ReviewTask with no connection: want error")
		}
		if _, err := database.DoneTask(1); err == nil {
			t.Error("DoneTask with no connection: want error")
		}
		if _, err := database.CancelTask(1); err == nil {
			t.Error("CancelTask with no connection: want error")
		}
		if _, err := database.UpdateTask(1, map[string]any{"title": "x"}); err == nil {
			t.Error("UpdateTask with no connection: want error")
		}
		if _, err := database.SetTaskTags(1, []string{"x"}); err == nil {
			t.Error("SetTaskTags with no connection: want error")
		}
		if _, err := database.AddTaskTags(1, []string{"x"}); err == nil {
			t.Error("AddTaskTags with no connection: want error")
		}
		if _, err := database.RemoveTaskTags(1, []string{"x"}); err == nil {
			t.Error("RemoveTaskTags with no connection: want error")
		}
		if _, err := database.Stats(""); err == nil {
			t.Error("Stats with no connection: want error")
		}
	})

	t.Run("off-days", func(t *testing.T) {
		if _, err := database.AddOffDay("@john", "2026-03-01", "2026-03-02", ""); err == nil {
			t.Error("AddOffDay with no connection: want error")
		}
		if _, err := database.ListOffDays(""); err == nil {
			t.Error("ListOffDays with no connection: want error")
		}
		if err := database.DeleteOffDay(1); err == nil {
			t.Error("DeleteOffDay with no connection: want error")
		}
	})

	t.Run("comments", func(t *testing.T) {
		if _, err := database.AddComment(1, "hello"); err == nil {
			t.Error("AddComment with no connection: want error")
		}
		if _, err := database.ListComments(1); err == nil {
			t.Error("ListComments with no connection: want error")
		}
		if err := database.DeleteComment(1); err == nil {
			t.Error("DeleteComment with no connection: want error")
		}
	})
}

// NewTestDB is the database of all the TUI tests, so it was
// uncovered without anything noticing: if it broke, the other tests would stay
// green because they would never even get to open it.
func TestNewTestDBAndConnAreUsable(t *testing.T) {
	database, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	conn := database.Conn()
	if conn == nil {
		t.Fatal("Conn returned nil")
	}
	// Really alive: one write statement and one read statement.
	if _, err := conn.Exec(`CREATE TABLE probe (n INTEGER)`); err != nil {
		t.Fatalf("Exec on Conn: %v", err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&n); err != nil {
		t.Fatalf("QueryRow on Conn: %v", err)
	}
	if n != 0 {
		t.Errorf("the newly created table has %d rows, want 0", n)
	}
}

// DefaultPath has two branches -- XDG_DATA_HOME set and not set -- and the second
// needs the home directory, which may not exist in a CI environment.
func TestDefaultPathFollowsXDGDataHome(t *testing.T) {
	t.Run("xdg", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/xdg/data")
		got, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath: %v", err)
		}
		if want := filepath.Join("/xdg/data", "tsk", "tsk.db"); got != want {
			t.Errorf("DefaultPath = %q, want %q", got, want)
		}
	})

	t.Run("home", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath: %v", err)
		}
		if want := filepath.Join(home, ".local", "share", "tsk", "tsk.db"); got != want {
			t.Errorf("DefaultPath = %q, want %q", got, want)
		}
	})

	t.Run("without home", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		// On Linux os.UserHomeDir only looks at $HOME, so emptying it is a
		// clean way to provoke the error without misconfiguring the machine.
		t.Setenv("HOME", "")
		if _, err := DefaultPath(); err == nil {
			t.Error("DefaultPath without HOME: want error")
		}
	})
}

// Open fails in three different ways and each one deserves its own message,
// because they are failures the user sees in the terminal.
func TestOpenFailureModes(t *testing.T) {
	t.Run("the parent directory cannot be created", func(t *testing.T) {
		// A regular file where the directory should go: mkdir
		// on top of a file fails with ENOTDIR.
		parentFile := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Open(filepath.Join(parentFile, "sub", "tsk.db"))
		if err == nil {
			t.Fatal("Open with a parent that is a file: want error")
		}
		if !strings.Contains(err.Error(), "create db dir") {
			t.Errorf("the error does not mention creating the directory: %v", err)
		}
	})

	t.Run("the path is not a database", func(t *testing.T) {
		// A text file where SQLite expects a database file:
		// sql.Open does not fail, but the migration does.
		path := filepath.Join(t.TempDir(), "no-es-db")
		if err := os.WriteFile(path, []byte("this is not sqlite"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Open(path)
		if err == nil {
			t.Fatal("Open on a file that is not a DB: want error")
		}
		if !strings.Contains(err.Error(), "migrate") {
			t.Errorf("the error does not mention the migration: %v", err)
		}
	})

	t.Run("a migration that returns an error", func(t *testing.T) {
		restore := migrations
		t.Cleanup(func() { migrations = restore })
		migrations = []struct {
			version string
			query   string
			run     func(*DB) error
		}{
			{version: "9999", run: func(*DB) error { return os.ErrPermission }},
		}

		_, err := Open(":memory:")
		if err == nil {
			t.Fatal("Open with a migration that fails: want error")
		}
		if !strings.Contains(err.Error(), "migration 9999") {
			t.Errorf("the error does not name the migration: %v", err)
		}
	})

	t.Run("a migration with invalid SQL", func(t *testing.T) {
		restore := migrations
		t.Cleanup(func() { migrations = restore })
		migrations = []struct {
			version string
			query   string
			run     func(*DB) error
		}{
			{version: "9999", query: "THIS IS NOT SQL"},
		}

		if _, err := Open(":memory:"); err == nil {
			t.Fatal("Open with a migration of invalid SQL: want error")
		}
	})

	t.Run("a migration that drops _meta", func(t *testing.T) {
		// If the migration drops the table where the version is later recorded, the
		// INSERT fails. It is the only path to reach that error without touching the
		// driver.
		restore := migrations
		t.Cleanup(func() { migrations = restore })
		migrations = []struct {
			version string
			query   string
			run     func(*DB) error
		}{
			{version: "9999", query: "DROP TABLE _meta"},
		}

		_, err := Open(":memory:")
		if err == nil {
			t.Fatal("Open with a migration that drops _meta: want error")
		}
		if !strings.Contains(err.Error(), "update version") {
			t.Errorf("the error does not mention the version update: %v", err)
		}
	})
}

// CreateProject validates the workflow before touching the database, so an
// invalid workflow is detected with the DB closed: proof that the validation does not
// depend on the connection.
func TestProjectValidationHappensBeforeTheQuery(t *testing.T) {
	database := closedDB(t)

	_, err := database.CreateProject("api", []string{"nope"})
	if err == nil {
		t.Fatal("CreateProject with an invalid workflow: want error")
	}
	// The message comes from the validator, not from SQLite: that is exactly what
	// we want to check, because with the DB closed any error would be the
	// connection's and the test would pass without proving anything.
	if strings.Contains(err.Error(), "closed") {
		t.Errorf("the error is from the connection, not the validator: %v", err)
	}
}
