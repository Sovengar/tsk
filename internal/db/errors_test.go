package db

import (
	"testing"

	"tsk/internal/model"
)

// populatedThenClosed returns a database with a real project and task,
// and then with the connection closed.
//
// The difference from closedDB is that here the methods pass all their
// previous validations -- the project exists, the status is valid, the task
// exists -- and fail on the real query. That is what covers the `if err` behind
// a GetTask or a GetProjectByID, which with a freshly
// closed database never get executed.
func populatedThenClosed(t *testing.T) *DB {
	t.Helper()
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "task", "", "", 2, "backlog")
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return database
}

// Status transitions validate the destination against the project's
// workflow BEFORE writing. With a freshly closed database the destination is rejected
// first and the write's `if err` is never seen; with a populated and
// closed database it is reached.
func TestStatusChangesFailAtTheWriteWithAClosedDB(t *testing.T) {
	closed := func(t *testing.T) *DB {
		t.Helper()
		return populatedThenClosed(t)
	}

	t.Run("MoveTask", func(t *testing.T) {
		if _, err := closed(t).MoveTask(1, "doing"); err == nil {
			t.Error("MoveTask: want error")
		}
	})
	t.Run("StartTask", func(t *testing.T) {
		if _, err := closed(t).StartTask(1); err == nil {
			t.Error("StartTask: want error")
		}
	})
	t.Run("ReviewTask", func(t *testing.T) {
		if _, err := closed(t).ReviewTask(1); err == nil {
			t.Error("ReviewTask: want error")
		}
	})
	t.Run("DoneTask", func(t *testing.T) {
		if _, err := closed(t).DoneTask(1); err == nil {
			t.Error("DoneTask: want error")
		}
	})
	t.Run("CancelTask", func(t *testing.T) {
		if _, err := closed(t).CancelTask(1); err == nil {
			t.Error("CancelTask: want error")
		}
	})
	t.Run("UpdateTask", func(t *testing.T) {
		if _, err := closed(t).UpdateTask(1, map[string]any{"title": "other"}); err == nil {
			t.Error("UpdateTask: want error")
		}
	})
	t.Run("SetTaskTags", func(t *testing.T) {
		if _, err := closed(t).SetTaskTags(1, []string{"x"}); err == nil {
			t.Error("SetTaskTags: want error")
		}
	})
	t.Run("AddTaskTags", func(t *testing.T) {
		if _, err := closed(t).AddTaskTags(1, []string{"x"}); err == nil {
			t.Error("AddTaskTags: want error")
		}
	})
	t.Run("RemoveTaskTags", func(t *testing.T) {
		if _, err := closed(t).RemoveTaskTags(1, []string{"x"}); err == nil {
			t.Error("RemoveTaskTags: want error")
		}
	})
}

// UpdateProject reads the project before building the UPDATE, so the
// write is only attempted when the read worked. With a populated and
// closed database the read fails first; what is covered here is the workflow and
// list_order validation, which happens in between and does not touch the database.
func TestUpdateProjectValidatesBeforeWriting(t *testing.T) {
	t.Run("invalid workflow", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		err := database.UpdateProject("api", map[string]any{"workflow": []string{"nope"}})
		if err == nil {
			t.Fatal("UpdateProject with an invalid workflow: want error")
		}
	})

	t.Run("invalid list_order", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		err := database.UpdateProject("api", map[string]any{"list_order": []string{"nope"}})
		if err == nil {
			t.Fatal("UpdateProject with an invalid list_order: want error")
		}
	})

	t.Run("invalid list_order against a new workflow", func(t *testing.T) {
		// Workflow and list_order in the same call: list_order is validated
		// against the workflow that just arrived, not the previous one.
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		err := database.UpdateProject("api", map[string]any{
			"workflow":   []string{"backlog", "done", "cancelled"},
			"list_order": []string{"done", "a-status-that-does-not-exist"},
		})
		if err == nil {
			t.Fatal("UpdateProject with a list_order that does not fit the new workflow: want error")
		}
	})

	t.Run("no changes", func(t *testing.T) {
		// An empty updates map touches nothing: no query, no
		// updated_at. It is the short branch that avoids a useless UPDATE.
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		if err := database.UpdateProject("api", map[string]any{}); err != nil {
			t.Errorf("UpdateProject with no changes: %v", err)
		}
		if err := database.UpdateProject("api", map[string]any{"force": true}); err != nil {
			t.Errorf("UpdateProject with only force: %v", err)
		}
	})
}

// GetProjectByID distinguishes "does not exist" from "the query failed", and they are
// two different messages because the first tells the user the name is
// misspelled and the second that something broke.
func TestGetProjectByIDReportsMissingSeparately(t *testing.T) {
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)

	_, err := database.GetProjectByID(9999)
	if err == nil {
		t.Fatal("GetProjectByID with a nonexistent id: want error")
	}
	// And with the database closed the message is the connection's, not "not found".
	closed := populatedThenClosed(t)
	if _, err := closed.GetProjectByID(1); err == nil {
		t.Fatal("GetProjectByID with no connection: want error")
	}
}

// Writing directly into the table is the only way to make a Scan fail:
// the types are set by the code, so only a corrupt row can contradict them.
// It is a white-box test over the schema, and that's why it uses Conn().
func TestScanFailsOnACorruptRow(t *testing.T) {
	t.Run("task with non-numeric priority", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateTask(t, database, "api", "task", "", "", 2, "backlog")

		// The column is INTEGER and SQLite is dynamically typed: text fits
		// in it without any constraint preventing it.
		if _, err := database.Conn().Exec(`UPDATE tasks SET priority = 'high'`); err != nil {
			t.Fatalf("corrupting the row: %v", err)
		}

		if _, err := database.ListTasks("", "", ""); err == nil {
			t.Error("ListTasks with a corrupted priority: want error")
		}
		if _, err := database.GetTask(1); err == nil {
			t.Error("GetTask with a corrupted priority: want error")
		}
	})

	t.Run("project with workflow that is not JSON", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		if _, err := database.Conn().Exec(`UPDATE projects SET workflow = 'no soy json'`); err != nil {
			t.Fatalf("corrupting the row: %v", err)
		}

		if _, err := database.GetProject("api"); err == nil {
			t.Error("GetProject with a corrupted workflow: want error")
		}
		if _, err := database.GetProjectByID(1); err == nil {
			t.Error("GetProjectByID with a corrupted workflow: want error")
		}
		if _, err := database.ListProjects(); err == nil {
			t.Error("ListProjects with a corrupted workflow: want error")
		}
	})

	t.Run("project with list_order that is not JSON", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		if _, err := database.Conn().Exec(`UPDATE projects SET list_order = '{'`); err != nil {
			t.Fatalf("corrupting the row: %v", err)
		}

		if _, err := database.GetProject("api"); err == nil {
			t.Error("GetProject with a corrupted list_order: want error")
		}
	})

	t.Run("task with tags that is not JSON", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateTask(t, database, "api", "task", "", "", 2, "backlog")

		if _, err := database.Conn().Exec(`UPDATE tasks SET tags = '['`); err != nil {
			t.Fatalf("corrupting the row: %v", err)
		}

		// An unreadable tags value is not a Scan error but a deserialize one, and the
		// list still has to come out: losing every task because of a row
		// with weird tags would be worse than losing the tags.
		tasks, err := database.ListTasks("", "", "")
		if err != nil {
			t.Fatalf("ListTasks with corrupted tags: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("%d tasks come out, want 1", len(tasks))
		}
		if len(tasks[0].Tags) != 0 {
			t.Errorf("the corrupted tags produced %v, want none", tasks[0].Tags)
		}
	})
}

// Stats counts by status and by assignee in two separate queries. With a
// populated and closed database it fails on the first one; reaching the second
// requires the first to work, so a stats over a live database
// with data in several statuses is used.
func TestStatsCountsEveryStateAndAssignee(t *testing.T) {
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "one", "", "@john", 2, "backlog")
	mustCreateTask(t, database, "api", "two", "", "@john", 2, "done")
	mustCreateTask(t, database, "api", "three", "", "@margo", 2, "done")

	stats, err := database.Stats("")
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	byStatus, ok := stats["by_status"].(map[string]int)
	if !ok {
		t.Fatalf("by_status is %T, want map[string]int", stats["by_status"])
	}
	if byStatus["backlog"] != 1 || byStatus["done"] != 2 {
		t.Errorf("by_status = %v, want backlog 1 and done 2", byStatus)
	}

	byAssignee, ok := stats["by_assignee"].(map[string]int)
	if !ok {
		t.Fatalf("by_assignee is %T, want map[string]int", stats["by_assignee"])
	}
	if byAssignee["@john"] != 2 || byAssignee["@margo"] != 1 {
		t.Errorf("by_assignee = %v, want @john 2 and @margo 1", byAssignee)
	}

	t.Run("with the database closed", func(t *testing.T) {
		if _, err := populatedThenClosed(t).Stats(""); err == nil {
			t.Error("Stats with no connection: want error")
		}
	})
}

// ProjectTaskCount counts with COUNT(*) over a NOT NULL column, so the
// only way it can fail is the connection.
func TestProjectTaskCountFailsOnAClosedDB(t *testing.T) {
	if _, err := populatedThenClosed(t).ProjectTaskCount(1); err == nil {
		t.Error("ProjectTaskCount with no connection: want error")
	}
}

// AddOffDay validates the dates before writing, and those messages are what the
// user sees when they mistype an off-day.
func TestAddOffDayRejectsBadDates(t *testing.T) {
	database := newTestDB(t)

	if _, err := database.AddOffDay("@john", "", "2026-03-02", ""); err == nil {
		t.Error("AddOffDay without a start date: want error")
	}
	if _, err := database.AddOffDay("@john", "2026-03-02", "not-a-date", ""); err == nil {
		t.Error("AddOffDay with an invalid end date: want error")
	}
}

// The terminal status is the one that cannot be removed from the workflow, and the
// filter that protects it has its own path inside UpdateProject.
func TestUpdateProjectKeepsTheTerminalStatus(t *testing.T) {
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "task", "", "", 2, model.DoneStatus)

	// Removing "done" is forbidden even when there are no tasks in it, and there
	// are tasks: the two checks exist separately.
	if err := database.UpdateProject("api", map[string]any{
		"workflow": []string{"backlog", "doing", "cancelled"},
	}); err == nil {
		t.Error("removing the terminal status from the workflow: want error")
	}
}

// GetTask and GetProjectByID read different things from the same project row:
// GetTask only wants the name, and GetProjectByID additionally deserializes the
// workflow. Breaking the workflow -- and only the workflow -- makes the first pass and the
// second fail, which is the only way to reach the intermediate error of
// StartTask, ReviewTask and DoneTask.
func TestStatusChangesFailWhenTheProjectWorkflowIsUnreadable(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*DB) error
	}{
		{"StartTask", func(d *DB) error { _, err := d.StartTask(1); return err }},
		{"ReviewTask", func(d *DB) error { _, err := d.ReviewTask(1); return err }},
		{"DoneTask", func(d *DB) error { _, err := d.DoneTask(1); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database := newTestDB(t)
			populateAPI(t, database)
			if _, err := database.Conn().Exec(`UPDATE projects SET workflow = 'no soy json'`); err != nil {
				t.Fatalf("corrupting the row: %v", err)
			}

			// GetTask still works: it only reads the name.
			if _, err := database.GetTask(1); err != nil {
				t.Fatalf("GetTask also fails, the test does not measure what it claims: %v", err)
			}
			if err := tc.call(database); err == nil {
				t.Error("want error")
			}
		})
	}
}

// Columns have type affinity, so SQLite does not let an incompatible
// value in: an integer in a TEXT is converted to text and a text in an
// INTEGER is rejected on the UPDATE. The only way for a Scan to receive a type
// it does not know how to convert is replacing the table with a view with the types
// changed, which is a white-box view over the schema.
func TestScanFailsWhenTheTableIsReplacedByAWronglyTypedView(t *testing.T) {
	t.Run("comments", func(t *testing.T) {
		database := newTestDB(t)
		populateAPI(t, database)
		mustAddComment(t, database, 1, "hello")

		if _, err := database.Conn().Exec(`DROP TABLE comments;
			CREATE VIEW comments AS SELECT x'00ff' AS id, 1 AS task_id, 'x' AS body, 'y' AS created_at`); err != nil {
			t.Fatalf("replacing the table with a view: %v", err)
		}

		if _, err := database.ListComments(1); err == nil {
			t.Error("ListComments with an id that is not an integer: want error")
		}
	})

	t.Run("off-days", func(t *testing.T) {
		database := newTestDB(t)
		if _, err := database.AddOffDay("@john", "2026-03-01", "2026-03-02", ""); err != nil {
			t.Fatalf("AddOffDay: %v", err)
		}

		if _, err := database.Conn().Exec(`DROP TABLE offdays;
			CREATE VIEW offdays AS SELECT x'00ff' AS id, 'j' AS assignee, 'd' AS start_date, 'd' AS end_date, '' AS note`); err != nil {
			t.Fatalf("replacing the table with a view: %v", err)
		}

		if _, err := database.ListOffDays(""); err == nil {
			t.Error("ListOffDays with an id that is not an integer: want error")
		}
	})

	t.Run("stats without tasks table", func(t *testing.T) {
		// The three Stats queries count over tasks; without the table, the
		// first -- the total -- already fails.
		database := newTestDB(t)
		populateAPI(t, database)
		if _, err := database.Conn().Exec(`DROP TABLE tasks`); err != nil {
			t.Fatalf("dropping tasks: %v", err)
		}

		if _, err := database.Stats(""); err == nil {
			t.Error("Stats without the tasks table: want error")
		}
	})
}
