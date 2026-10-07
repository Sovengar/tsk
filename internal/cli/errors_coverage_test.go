package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/db"
)

// The CLI contract for an AI agent is that every failure comes out as JSON on
// stderr with code 1. That means every `if err != nil` of every command is
// a contract line, not internal handling: if one slips through, the agent is
// left without a diagnosis.
//
// What was not tested was any of them. They are all reached the same way:
// a database that reads fine and writes badly, or that does not
// respond at all.

// withBrokenDB points the config at a path that is not a database: the
// connection opens -- sql.Open with modernc never fails -- but any
// query fails. It is the earliest failure possible.
func withBrokenDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "no-es-una-base")
	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		t.Fatal(err)
	}
	setConfigPath(t, dbPath)
}

// withReadOnlyDB leaves a real, readable database that cannot be
// written to. It is the only way to reach the write errors of the
// commands that read first.
//
// It is done with triggers and not with permissions: SQLite writes to the WAL, which
// stays writable even when the main file is not, so chmod does
// prevent the write. An aborting trigger does, and it also pinpoints the table.
func withReadOnlyDB(t *testing.T) string {
	t.Helper()
	dbPath := withRealDB(t)

	writer, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening the database to sabotage it: %v", err)
	}
	defer func() { _ = writer.Close() }()

	for _, table := range []string{"tasks", "projects", "comments", "offdays"} {
		stmt := `CREATE TRIGGER read_only_` + table + ` BEFORE INSERT ON ` + table +
			` BEGIN SELECT RAISE(ABORT, 'read-only database'); END`
		if _, err := writer.Conn().Exec(stmt); err != nil {
			t.Fatalf("trigger for %s: %v", table, err)
		}
		stmt = `CREATE TRIGGER read_only_upd_` + table + ` BEFORE UPDATE ON ` + table +
			` BEGIN SELECT RAISE(ABORT, 'read-only database'); END`
		if _, err := writer.Conn().Exec(stmt); err != nil {
			t.Fatalf("trigger for %s: %v", table, err)
		}
		stmt = `CREATE TRIGGER read_only_del_` + table + ` BEFORE DELETE ON ` + table +
			` BEGIN SELECT RAISE(ABORT, 'read-only database'); END`
		if _, err := writer.Conn().Exec(stmt); err != nil {
			t.Fatalf("trigger for %s: %v", table, err)
		}
	}

	// Check that the sabotage works: a write has to fail.
	if _, err := writer.Conn().Exec(`UPDATE tasks SET title = 'x'`); err == nil {
		t.Fatal("the sabotage does not prevent writes")
	}
	// And a read still works.
	if _, err := writer.Conn().Exec(`SELECT 1`); err != nil {
		t.Fatalf("the sabotage broke reads: %v", err)
	}

	return dbPath
}

// withRealDB creates a database with a project and a task, and returns the
// path.
func withRealDB(t *testing.T) string {
	t.Helper()
	dbPath := withEmptyDB(t)
	output, code := run(t, "project", "add", "api", "--workflow", "backlog,done")
	if code != 0 {
		t.Fatalf("project add: %s", output)
	}
	if _, code := run(t, "add", "a task", "--project", "api", "--priority", "1"); code != 0 {
		t.Fatal("add failed")
	}
	return dbPath
}

func withEmptyDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tsk.db")
	setConfigPath(t, dbPath)
	return dbPath
}

// setConfigPath writes a config that points at the given path.
func setConfigPath(t *testing.T, dbPath string) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.toml")
	body := "[database]\npath = \"" + dbPath + "\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", cfg)
}

// When the database cannot even be opened, every command has to
// exit with 1 and an error JSON. openDB aborts before dispatching, so this
// does not exercise the error branches of each command: the sabotage below
// does. What it checks is that no command stays with a silent
// 0 because it could not open the database.
func TestEveryCommandFailsWhenTheDatabaseCannotBeOpened(t *testing.T) {
	for _, args := range [][]string{
		{"project", "list"},
		{"project", "list", "--archived"},
		{"project", "show", "api"},
		{"project", "update", "api", "--name", "other"},
		{"project", "remove", "api"},
		{"project", "archive", "api"},
		{"project", "unarchive", "api"},
		{"list"},
		{"list", "--project", "api"},
		{"show", "1"},
		{"comment", "add", "1", "hello"},
		{"comment", "list", "1"},
		{"comment", "remove", "1"},
		{"offday", "list"},
		{"offday", "add", "@john", "2026-01-01", "2026-01-02"},
		{"offday", "delete", "1"},
		{"gantt"},
		{"gantt", "--project", "api"},
		{"start", "1"},
		{"done", "1"},
		{"cancel", "1"},
		{"stats"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			withBrokenDB(t)
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("exit code is %d, want 1", code)
			}
		})
	}
}

// The write errors of the commands that read the task first: update with
// tags, start, done and cancel. The read works, the write does not, and the command
// has to say so.
func TestWriteCommandsFailWhenTheDatabaseIsReadOnly(t *testing.T) {
	withReadOnlyDB(t)

	for _, args := range [][]string{
		{"start", "1"},
		{"done", "1"},
		{"cancel", "1"},
		{"update", "1", "--tags", "new"},
		{"update", "1", "--untag", "new"},
		{"update", "1", "--tag", "new"},
		{"update", "1", "--title", "other"},
		{"comment", "add", "1", "hello"},
		{"comment", "remove", "1"},
		{"offday", "add", "@john", "2026-01-01", "2026-01-02"},
		{"offday", "delete", "1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("exit code is %d, want 1 with the read-only database", code)
			}
		})
	}
}

// The write errors of the project commands, which do not read before
// writing.
func TestProjectWritesFailOnAReadOnlyDatabase(t *testing.T) {
	withReadOnlyDB(t)

	for _, args := range [][]string{
		{"project", "add", "other"},
		{"project", "update", "api", "--name", "other"},
		{"project", "remove", "api"},
		{"project", "archive", "api"},
		{"project", "unarchive", "api"},
		{"add", "a new task", "--project", "api"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("exit code is %d, want 1", code)
			}
		})
	}
}

// The database path comes from the config; if the config does not bring it, it
// comes from XDG. With neither -- and without HOME -- there is nowhere to open anything.
func TestDBPathResolution(t *testing.T) {
	t.Run("without HOME or XDG", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(cfg, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", cfg)
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", "")

		_, code := run(t, "project", "list")
		if code != 1 {
			t.Errorf("without a database path the code is %d, want 1", code)
		}
	})

	t.Run("a path that cannot be created", func(t *testing.T) {
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		setConfigPath(t, filepath.Join(blocker, "sub", "tsk.db"))

		_, code := run(t, "project", "list")
		if code != 1 {
			t.Errorf("with an impossible path the code is %d, want 1", code)
		}
	})

	t.Run("with no path in the config the XDG one is used", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(cfg, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", cfg)
		t.Setenv("XDG_DATA_HOME", dir)

		output, code := run(t, "project", "add", "api")
		if code != 0 {
			t.Fatalf("project add: %s", output)
		}
		if _, err := os.Stat(filepath.Join(dir, "tsk", "tsk.db")); err != nil {
			t.Errorf("the XDG database was not created: %v", err)
		}
	})
}

// Usage messages are part of the contract: an agent that gets the
// arguments wrong has to read a text that tells it which ones are valid.
func TestUsageErrors(t *testing.T) {
	withEmptyDB(t)

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"project"}, "usage: tsk project"},
		{[]string{"project", "show"}, "usage: tsk project show"},
		{[]string{"project", "update"}, "usage: tsk project update"},
		{[]string{"project", "remove"}, "usage: tsk project remove"},
		{[]string{"project", "archive"}, "usage: tsk project archive"},
		{[]string{"project", "unarchive"}, "usage: tsk project unarchive"},
		{[]string{"project", "whatever"}, "unknown project subcommand: whatever"},
		{[]string{"completion", "tcsh"}, "usage: tsk completion"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			captureOutput(t)
			errs := &capture{}
			origErr := stderr
			stderr = errs
			t.Cleanup(func() { stderr = origErr })

			code := 0
			func() {
				defer func() {
					if r := recover(); r != nil {
						e, ok := r.(exitPanic)
						if !ok {
							panic(r)
						}
						code = e.code
					}
				}()
				Run(tc.args)
			}()

			if code != 1 {
				t.Errorf("exit code is %d, want 1", code)
			}
			if !strings.Contains(errs.String(), tc.want) {
				t.Errorf("the error does not say %q: %s", tc.want, errs.String())
			}
		})
	}
}

// gantt takes a start date and a number of weeks. A date that is
// not a date has to produce an error saying so, not a gantt with zero dates;
// and a --weeks that is not a number is ignored instead of taking down the command.
func TestGanttRejectsABadDate(t *testing.T) {
	withRealDB(t)

	t.Run("--from invalid", func(t *testing.T) {
		_, code := run(t, "gantt", "--from", "yesterday")
		if code != 1 {
			t.Errorf("with an invalid --from the code is %d, want 1", code)
		}
	})

	t.Run("--from without a value", func(t *testing.T) {
		// A flag without what follows is ignored instead of eating the next
		// argument: it is the contract of the CLI's flags.
		if _, code := run(t, "gantt", "--from"); code != 0 {
			t.Errorf("with only --from the code is %d, want 0", code)
		}
	})

	t.Run("without a shell", func(t *testing.T) {
		// Without an argument there is no shell and the command does nothing, instead of
		// printing the wrong completion.
		if _, code := run(t, "completion"); code != 0 {
			t.Errorf("completion without a shell has code %d, want 0", code)
		}
	})

	t.Run("--weeks that is not a number", func(t *testing.T) {
		// An invalid --weeks is not an error: it is ignored and the config's wins.
		// What it cannot be is a negative weeks range.
		if _, code := run(t, "gantt", "--weeks", "many"); code != 0 {
			t.Errorf("with an invalid --weeks the code is %d, want 0", code)
		}
		if _, code := run(t, "gantt", "--weeks", "-3"); code != 0 {
			t.Errorf("with a negative --weeks the code is %d, want 0", code)
		}
	})

	t.Run("a good --from", func(t *testing.T) {
		if _, code := run(t, "gantt", "--from", "2026-01-01", "--json"); code != 0 {
			t.Errorf("with a valid date the code is %d, want 0", code)
		}
	})
}

// project add with a workflow that cannot be parsed: the list is only
// commas, which is the case the parser rejects.
func TestProjectAddWithUnparseableLists(t *testing.T) {
	withEmptyDB(t)

	for _, args := range [][]string{
		{"project", "add", "api", "--workflow", " , "},
		{"project", "add", "api", "--list-order", " , "},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("exit code is %d, want 1", code)
			}
		})
	}
}

// The per-command error branches need a database that opens and migrates
// well -- otherwise openDB aborts before reaching the command -- but whose reads
// fail. The trick is the same as in internal/db: recreate the table without a primary
// key and insert a row whose id is not a number, so that any
// Scan over it blows up.
//
// Every row carries all its values because the columns are recreated without DEFAULT:
// a NULL column matches neither a WHERE nor an equality, and the poison would
// slip through without triggering the error we want to provoke.

const (
	ddlProjects = `CREATE TABLE %s (
		id TEXT, name TEXT, workflow TEXT, list_order TEXT, archived INTEGER,
		archived_at TEXT, created_at TEXT, updated_at TEXT)`
	ddlTasks = `CREATE TABLE %s (
		id TEXT, project_id INTEGER, title TEXT, description TEXT, status TEXT,
		priority INTEGER, assignee TEXT, estimate REAL, tags TEXT,
		created_at TEXT, updated_at TEXT, completed_at TEXT)`
	ddlComments = `CREATE TABLE %s (id TEXT, task_id INTEGER, body TEXT, created_at TEXT)`
	ddlOffDays  = `CREATE TABLE %s (id TEXT, assignee TEXT, start_date TEXT, end_date TEXT, note TEXT)`

	// Two poison rows in projects: one active and one archived. With a single one,
	// the query filtering by the other value does not see it and returns without error.
	rowProjects = `INSERT INTO projects VALUES
		('unreadable', 'poison', '[]', '[]', 0, NULL, '2020-01-01', '2020-01-01'),
		('unreadable', 'poison-archived', '[]', '[]', 1, '2020-01-01', '2020-01-01', '2020-01-01')`
	rowTasks = `INSERT INTO tasks VALUES
		('unreadable', 1, 'poison', '', 'backlog', 0, '', 0, '[]', '2020-01-01', '2020-01-01', NULL)`
	rowComments = `INSERT INTO comments VALUES
		('unreadable', 1, 'poison', '2020-01-01')`
	rowOffDays = `INSERT INTO offdays VALUES
		('unreadable', '@john', '2020-01-01', '2020-01-02', 'poison')`
)

// breakTable leaves the given table unreadable and returns the path of the
// database, already seeded.
func breakTable(t *testing.T, table, ddl, row string) string {
	t.Helper()
	dbPath := withRealDB(t)

	writer, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer func() { _ = writer.Close() }()

	conn := writer.Conn()
	if _, err := conn.Exec(`PRAGMA foreign_keys(OFF)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`ALTER TABLE ` + table + ` RENAME TO ` + table + `_healthy`); err != nil {
		t.Fatalf("renaming %s: %v", table, err)
	}
	if _, err := conn.Exec(fmt.Sprintf(ddl, table)); err != nil {
		t.Fatalf("recreating %s: %v", table, err)
	}
	if _, err := conn.Exec(row); err != nil {
		t.Fatalf("inserting the poison row into %s: %v", table, err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys(ON)`); err != nil {
		t.Fatal(err)
	}

	return dbPath
}

// The listings that run with the corresponding table broken.
func TestReadsFailWhenTheirTableIsUnreadable(t *testing.T) {
	t.Run("projects", func(t *testing.T) {
		setConfigPath(t, breakTable(t, "projects", ddlProjects, rowProjects))
		expectFailure(t, "project", "list")
		expectFailure(t, "project", "list", "--archived")
		expectFailure(t, "project", "show", "api")
		expectFailure(t, "project", "update", "api", "--name", "other")
	})

	t.Run("tasks", func(t *testing.T) {
		setConfigPath(t, breakTable(t, "tasks", ddlTasks, rowTasks))
		expectFailure(t, "list")
		expectFailure(t, "show", "1")
		expectFailure(t, "update", "1", "--tag", "new")
		expectFailure(t, "update", "1")
		expectFailure(t, "gantt")
	})

	t.Run("comments", func(t *testing.T) {
		setConfigPath(t, breakTable(t, "comments", ddlComments, rowComments))
		expectFailure(t, "show", "1")
		expectFailure(t, "comment", "list", "1")
		expectFailure(t, "comment", "delete", "1")
	})

	t.Run("off-days", func(t *testing.T) {
		setConfigPath(t, breakTable(t, "offdays", ddlOffDays, rowOffDays))
		expectFailure(t, "offday", "list")
		expectFailure(t, "gantt")
	})

	// A table that does not exist is the other failure mode: the query prepares
	// fine but does not find the table. It affects everything that uses it in a JOIN,
	// which is the case of stats: its COUNT only counts tasks of non-archived
	// projects, so with no projects there is nothing to count and the command has
	// to say so instead of producing a zero total.
	t.Run("vanished projects", func(t *testing.T) {
		setConfigPath(t, dropTable(t, "projects"))
		expectFailure(t, "stats")
	})
}

// dropTable drops the table leaving nothing in its place: the query
// prepares but fails when executed.
func dropTable(t *testing.T, table string) string {
	t.Helper()
	dbPath := withRealDB(t)

	writer, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer func() { _ = writer.Close() }()

	conn := writer.Conn()
	if _, err := conn.Exec(`PRAGMA foreign_keys(OFF)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`DROP TABLE ` + table); err != nil {
		t.Fatalf("DROP TABLE %s: %v", table, err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys(ON)`); err != nil {
		t.Fatal(err)
	}

	return dbPath
}

func expectFailure(t *testing.T, args ...string) {
	t.Helper()
	t.Run(strings.Join(args, " "), func(t *testing.T) {
		if _, code := run(t, args...); code != 1 {
			t.Errorf("the code is %d, want 1", code)
		}
	})
}

// Editing a project with a list that cannot be parsed fails before
// touching the database: the error is the parser's, not the write's.
func TestProjectUpdateRejectsUnparseableLists(t *testing.T) {
	withRealDB(t)

	for _, args := range [][]string{
		{"project", "update", "api", "--workflow", " , "},
		{"project", "update", "api", "--list-order", " , "},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, code := run(t, args...); code != 1 {
				t.Errorf("the code is %d, want 1", code)
			}
		})
	}
}
