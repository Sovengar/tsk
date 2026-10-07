package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// readonlyDB opens a database in read-only mode.
//
// It is the second lever, and it covers what the closed connection does not: here
// reads work and writes fail. The methods that first read the
// task or project and then write -- MoveTask, SetTaskTags,
// UpdateProject -- only reach their write error check with
// this lever, because with the database closed they stop at the read.
func readonlyDB(t *testing.T, populate func(*testing.T, *DB)) *DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ro.db")
	writeDB, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	populate(t, writeDB)
	if err := writeDB.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	conn, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("opening in read-only mode: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ro := &DB{conn: conn}

	// Check that the lever is the one we think: it reads, and it does not write.
	if _, err := ro.ListProjects(); err != nil {
		t.Fatalf("a read-only database should be readable: %v", err)
	}
	if _, err := ro.CreateProject("write", nil); err == nil {
		t.Fatal("a read-only database should not be writable")
	}
	return ro
}

func populateAPI(t *testing.T, database *DB) {
	t.Helper()
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "task", "", "@john", 2, "backlog")
}

// The methods that validate, read and then write fail at the end, on the
// write. With a closed connection they would fail earlier, on the read, and their
// error check would go uncovered.
func TestWritesFailAfterTheReadsSucceed(t *testing.T) {
	t.Run("CreateTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).CreateTask("api", "new", "", "", 2, "backlog"); err == nil {
			t.Error("CreateTask in read-only mode: want error")
		}
	})
	t.Run("MoveTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).MoveTask(1, "doing"); err == nil {
			t.Error("MoveTask in read-only mode: want error")
		}
	})
	t.Run("StartTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).StartTask(1); err == nil {
			t.Error("StartTask in read-only mode: want error")
		}
	})
	t.Run("ReviewTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).ReviewTask(1); err == nil {
			t.Error("ReviewTask in read-only mode: want error")
		}
	})
	t.Run("DoneTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).DoneTask(1); err == nil {
			t.Error("DoneTask in read-only mode: want error")
		}
	})
	t.Run("CancelTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).CancelTask(1); err == nil {
			t.Error("CancelTask in read-only mode: want error")
		}
	})
	t.Run("UpdateTask", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).UpdateTask(1, map[string]any{"title": "other"}); err == nil {
			t.Error("UpdateTask in read-only mode: want error")
		}
	})
	t.Run("SetTaskTags", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).SetTaskTags(1, []string{"x"}); err == nil {
			t.Error("SetTaskTags in read-only mode: want error")
		}
	})
	t.Run("AddTaskTags", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).AddTaskTags(1, []string{"x"}); err == nil {
			t.Error("AddTaskTags in read-only mode: want error")
		}
	})
	t.Run("RemoveTaskTags", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).RemoveTaskTags(1, []string{"x"}); err == nil {
			t.Error("RemoveTaskTags in read-only mode: want error")
		}
	})
	t.Run("ArchiveProject", func(t *testing.T) {
		if err := readonlyDB(t, populateAPI).ArchiveProject("api"); err == nil {
			t.Error("ArchiveProject in read-only mode: want error")
		}
	})
	t.Run("UnarchiveProject", func(t *testing.T) {
		if err := readonlyDB(t, populateAPI).UnarchiveProject("api"); err == nil {
			t.Error("UnarchiveProject in read-only mode: want error")
		}
	})
	t.Run("DeleteProject", func(t *testing.T) {
		if err := readonlyDB(t, populateAPI).DeleteProject("api"); err == nil {
			t.Error("DeleteProject in read-only mode: want error")
		}
	})
	t.Run("AddComment", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).AddComment(1, "hello"); err == nil {
			t.Error("AddComment in read-only mode: want error")
		}
	})
	t.Run("AddOffDay", func(t *testing.T) {
		if _, err := readonlyDB(t, populateAPI).AddOffDay("@john", "2026-03-01", "2026-03-02", ""); err == nil {
			t.Error("AddOffDay in read-only mode: want error")
		}
	})
	t.Run("DeleteOffDay", func(t *testing.T) {
		if err := readonlyDB(t, populateAPI).DeleteOffDay(1); err == nil {
			t.Error("DeleteOffDay in read-only mode: want error")
		}
	})
	t.Run("UpdateProject renaming", func(t *testing.T) {
		if err := readonlyDB(t, populateAPI).UpdateProject("api", map[string]any{"name": "api2"}); err == nil {
			t.Error("UpdateProject in read-only mode: want error")
		}
	})
}

// UpdateProject counts the tasks of each status that is going to be removed from
// the workflow. That count is a read, so neither the closed connection nor the
// read-only one reaches it: the query has to exist at validation
// time and not exist an instant later.
func TestUpdateProjectCountsTasksOfRemovedStates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sabotage.db")

	live, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	populateAPI(t, live)

	// The table is renamed in between: the project read works, the
	// task count does not.
	if _, err := live.Conn().Exec(`ALTER TABLE tasks RENAME TO tasks_bak`); err != nil {
		t.Fatalf("renaming tasks: %v", err)
	}

	err = live.UpdateProject("api", map[string]any{
		"workflow": []string{"doing", "done", "cancelled"},
	})
	if err == nil {
		t.Fatal("UpdateProject counting over a table that does not exist: want error")
	}

	_ = live.Close()
}

// CreateProject also validates the workflow before inserting, and with a
// closed database the validation trips first: that is why the database error never
// appears with an invalid workflow.
func TestCreateProjectFailsAtTheInsert(t *testing.T) {
	if _, err := readonlyDB(t, func(*testing.T, *DB) {}).CreateProject("api", nil); err == nil {
		t.Error("CreateProject in read-only mode: want error")
	}
}
