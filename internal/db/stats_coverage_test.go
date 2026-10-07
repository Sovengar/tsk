package db

import (
	"strings"
	"testing"
)

// Stats runs three queries against the same view: a COUNT, a GROUP BY status
// and a GROUP BY assignee. With a healthy database all three work, so their three
// error branches only show up if the SCHEMA is broken, not the data.
//
// The tool is to replace the tasks table with a VIEW. A view is SQL,
// not data, and that opens two distinct sabotages that must not be confused, because
// each fails at a different point of Stats:
//
//   - A column THAT DOES NOT EXIST. The COUNT does not mention it -- it counts rows, and
//     that is all the same-- so it passes; the GROUP BY does name it and fails while PREPARING the
//     query. It is a db.conn.Query failure: no row has come out yet.
//
//   - A column with a value that cannot be converted (a NULL). The query
//     is prepared, it runs, the row arrives... and it is the SCAN that does not know
//     what to do. database/sql has no NULL→string: there is nothing to fill it with.
//
// Confusing them would give a test that passes for the wrong reason, which is worse
// than having no test: the day the query changes, the test will stay green for a
// reason that no longer exists.

// tasksView swaps the tasks schema for the given view. It is called with the
// body of the view's SELECT, and the view has to bring the twelve columns
// the rest of the code expects -- it is not a black-box view over the
// schema, it is the same schema with one column changed.
func tasksView(t *testing.T, viewSelect string) *DB {
	t.Helper()
	database := newTestDB(t)
	populateAPI(t, database)

	if _, err := database.Conn().Exec(`ALTER TABLE tasks RENAME TO tasks_real`); err != nil {
		t.Fatalf("renaming tasks: %v", err)
	}
	if _, err := database.Conn().Exec(`CREATE VIEW tasks AS ` + viewSelect + ` FROM tasks_real t0`); err != nil {
		t.Fatalf("creating the view: %v", err)
	}
	return database
}

// breakColumn swaps a view column for the given expression, under the
// SAME name. It is the Scan sabotage: the query works, the row arrives, and
// what cannot be converted is the value.
//
// To remove a column entirely -- the query sabotage -- you have to pass
// a new name, and that is why it is a separate helper.
func breakColumn(t *testing.T, column, expr string) *DB {
	t.Helper()
	colStatus, colAssignee := "t0.status", "t0.assignee"
	switch column {
	case "status":
		colStatus = expr
	case "assignee":
		colAssignee = expr
	default:
		t.Fatalf("unknown column: %q", column)
	}

	cols := "t0.id, t0.project_id, t0.title, t0.description, " + colStatus + " AS status," +
		" t0.priority, " + colAssignee + " AS assignee," +
		" t0.created_at, t0.updated_at, t0.completed_at, t0.estimate, t0.tags"
	return tasksView(t, "SELECT "+cols)
}

// dropColumn leaves the view's column without the name the code
// queries. The COUNT ignores it because it does not name it; the GROUP BY names it and cannot
// be prepared.
func dropColumn(t *testing.T, column string) *DB {
	t.Helper()
	// The trick is the ALIAS, not the value: the column is still there with its content,
	// but the view no longer calls it status, so "GROUP BY t.status" does not
	// find what to group by.
	colStatus, colAssignee := "t0.status AS status", "t0.assignee AS assignee"
	switch column {
	case "status":
		colStatus = `t0.status AS estado`
	case "assignee":
		colAssignee = `t0.assignee AS responsable`
	default:
		t.Fatalf("unknown column: %q", column)
	}

	cols := "t0.id, t0.project_id, t0.title, t0.description, " + colStatus + "," +
		" t0.priority, " + colAssignee + "," +
		" t0.created_at, t0.updated_at, t0.completed_at, t0.estimate, t0.tags"
	return tasksView(t, "SELECT "+cols)
}

// The two QUERY failures. The sabotage is real and not a harness trick:
// without it, Stats returns the whole map. And each case checks the message, which
// is what distinguishes "the query could not be prepared" from any other failure
// by which Stats might return an error.
func TestStatsFailsWhenTheGroupingColumnDoesNotExist(t *testing.T) {
	t.Run("the status query", func(t *testing.T) {
		database := dropColumn(t, "status")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats without a status column: want error")
		}
		// The name in the message says WHICH column was missing, and says it is the
		// status one: if the sabotage had slipped into the assignee one, the
		// message would be different and the test would not measure what it claims to.
		if !strings.Contains(err.Error(), "t.status") {
			t.Errorf("the failure is not the status column: %v", err)
		}
	})

	t.Run("the query by assignee", func(t *testing.T) {
		database := dropColumn(t, "assignee")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats without an assignee column: want error")
		}
		if !strings.Contains(err.Error(), "t.assignee") {
			t.Errorf("the failure is not the assignee column: %v", err)
		}
	})
}

// The two SCAN failures, which are different: the query worked and what
// cannot be converted is the value.
func TestStatsFailsWhenTheGroupingColumnIsNull(t *testing.T) {
	t.Run("the status arrives as NULL", func(t *testing.T) {
		database := breakColumn(t, "status", "NULL")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats with a NULL status: want error")
		}
		// "Scan error on column index 0" and not a query failure: index 0
		// is the status, the first column of the GROUP BY, and it is the one that was sabotaged.
		if !strings.Contains(err.Error(), "Scan error on column index 0") {
			t.Errorf("the failure is not the status Scan: %v", err)
		}
	})

	t.Run("the assignee arrives as NULL", func(t *testing.T) {
		database := breakColumn(t, "assignee", "NULL")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats with a NULL assignee: want error")
		}
		// It is still column 0 because this query returns the
		// assignee first.
		if !strings.Contains(err.Error(), "Scan error on column index 0") {
			t.Errorf("the failure is not the assignee Scan: %v", err)
		}
	})
}

// A Stats that fails halfway has to fail entirely: if it returned what it
// had, the command would show a total with no breakdown and nobody would notice a
// column is missing.
func TestStatsReturnsNothingWhenItFails(t *testing.T) {
	database := breakColumn(t, "assignee", "NULL")

	res, err := database.Stats("")
	if err == nil {
		t.Fatal("Stats with a NULL assignee: want error")
	}
	if res != nil {
		t.Errorf("Stats returned %v in addition to the error: an error here means the map is invented", res)
	}
}

// MoveTask reads the task and then its project. The task is read with a JOIN that
// only brings p.name, so it survives a project with a corrupt workflow;
// the second read, which brings all eight columns, does not.
func TestMoveTaskFailsWhenTheProjectCannotBeRead(t *testing.T) {
	database := newTestDB(t)
	populateAPI(t, database)

	// workflow is JSON and parsed on scan: an invalid workflow breaks
	// scanProject without touching the rest of the row.
	if _, err := database.Conn().Exec(`UPDATE projects SET workflow = 'no soy json'`); err != nil {
		t.Fatalf("corrupting the workflow: %v", err)
	}

	// The task's key lookup does work, or the test would not measure what it
	// says: if it failed there, the error would come from GetTask and not from GetProjectByID.
	if _, err := database.GetTask(1); err != nil {
		t.Fatalf("GetTask also fails, the test does not measure what it claims: %v", err)
	}

	if _, err := database.MoveTask(1, "done"); err == nil {
		t.Error("MoveTask with an unreadable project: want error")
	}

	// And the task has not moved: the failure is on the read, before the write.
	var status string
	if err := database.Conn().QueryRow(`SELECT status FROM tasks WHERE id = 1`).Scan(&status); err != nil {
		t.Fatalf("reading the status: %v", err)
	}
	if status == "done" {
		t.Error("the task moved even though its project could not be read")
	}
}

// The rule of the sabotage is that the database has to keep being a database: if the
// schema does not hold, the conclusion is that the test measures a broken database and not a
// code error path.
func TestBreakColumnLeavesADatabaseUsable(t *testing.T) {
	database := breakColumn(t, "status", "NULL")

	var count int
	if err := database.Conn().QueryRow(`SELECT COUNT(*) FROM tasks_real`).Scan(&count); err != nil {
		t.Fatalf("the original table has disappeared: %v", err)
	}
	if count == 0 {
		t.Error("the sabotage emptied the database, so it does not measure a Stats failure but a database without data")
	}
	// And the rest of the API keeps responding: only the read of
	// status broke, not the whole database.
	if _, err := database.GetProject("api"); err != nil {
		t.Errorf("the sabotage broke more than it claims: %v", err)
	}
}
