package db

import (
	"os"
	"path/filepath"
)

// schemaV1 is the initial database schema.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS _meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    path       TEXT NOT NULL,
    workflow   TEXT NOT NULL DEFAULT '["backlog","todo","in_progress","review","done"]',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'backlog',
    priority     INTEGER NOT NULL DEFAULT 0,
    assignee     TEXT NOT NULL DEFAULT '',
    position     INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
`

// schemaV2 adds task comments.
const schemaV2 = `
CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_comments_task ON comments(task_id);
CREATE INDEX IF NOT EXISTS idx_comments_task_created ON comments(task_id, created_at);
`

// schemaV3 adds project archiving (soft delete).
const schemaV3 = `
ALTER TABLE projects ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN archived_at TEXT;
CREATE INDEX IF NOT EXISTS idx_projects_archived ON projects(archived);
`

// schemaV4 drops the position column: task order is no longer manual
// but derived from the project's workflow.
const schemaV4 = `
ALTER TABLE tasks DROP COLUMN position;
`

// schemaV5 separates the presentation order of the List view from the workflow
// (which defines the progression of actions). list_order is a JSON array of
// statuses; empty ('[]') means "use the workflow order".
const schemaV5 = `
ALTER TABLE projects ADD COLUMN list_order TEXT NOT NULL DEFAULT '[]';
`

// schemaV6 drops path: it was unused metadata in the app.
const schemaV6 = `
ALTER TABLE projects DROP COLUMN path;
`

// schemaV7 adds task quantification (estimate, in days) and personal
// off-days (vacations, holidays, absences). An off-day covers the
// inclusive range [start_date, end_date]; with no note, all that matters is that the
// person will not be available.
const schemaV7 = `
ALTER TABLE tasks ADD COLUMN estimate REAL NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS offdays (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    assignee   TEXT NOT NULL,
    start_date TEXT NOT NULL,
    end_date   TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_offdays_assignee ON offdays(assignee, start_date);
`

// schemaV8 adds tags to tasks. They are stored as a JSON array (same pattern
// as workflow/list_order) so they can be queried with json_each.
const schemaV8 = `
ALTER TABLE tasks ADD COLUMN tags TEXT NOT NULL DEFAULT '[]';
`

// schemaV12 restores project path as user-managed metadata for AI handoffs.
const schemaV12 = `
ALTER TABLE projects ADD COLUMN path TEXT NOT NULL DEFAULT '';
`

// schemaV13 revives an archived project when a task lands in it. It lives in
// the schema and not in Go so the revive and the write that motivated it are the
// same statement: a failed insert or move leaves the project archived, with no
// transaction to hand-roll in every caller. The project's own tasks are never
// touched, so they come back with it.
const schemaV13 = `
CREATE TRIGGER tasks_revive_project_on_insert AFTER INSERT ON tasks
WHEN (SELECT archived FROM projects WHERE id = NEW.project_id) = 1
BEGIN
	UPDATE projects SET archived = 0, archived_at = NULL, updated_at = NEW.created_at
	WHERE id = NEW.project_id AND archived = 1;
END;

CREATE TRIGGER tasks_revive_project_on_move AFTER UPDATE OF project_id ON tasks
WHEN NEW.project_id != OLD.project_id
 AND (SELECT archived FROM projects WHERE id = NEW.project_id) = 1
BEGIN
	UPDATE projects SET archived = 0, archived_at = NULL, updated_at = NEW.updated_at
	WHERE id = NEW.project_id AND archived = 1;
END;
`

// schemaV14 adds a per-task working directory. The task path overrides the
// project path; when both are empty the effective directory is ~/Tasks.
const schemaV14 = `
ALTER TABLE tasks ADD COLUMN path TEXT NOT NULL DEFAULT '';
`

// backfillProjectPaths gives every project with no path a real one when its
// conventional directory exists (~/dev/projects/<name>). Projects with no match
// keep the empty path and resolve to ~/Tasks at handoff time.
// migrateV14 adds the task path column and then backfills the project paths.
// The schema statement and the backfill share one run because a migration with
// a run function ignores its query field.
func migrateV14(db *DB) error {
	if err := ensureTaskPathColumn(db); err != nil {
		return err
	}
	return backfillProjectPaths(db)
}

// migrateV15 repairs databases that were stamped as 014 without the task path
// column. The repair is idempotent: the column is added only when missing, and
// the backfill runs again because the broken 014 never reached it.
func migrateV15(db *DB) error {
	if err := ensureTaskPathColumn(db); err != nil {
		return err
	}
	return backfillProjectPaths(db)
}

// ensureTaskPathColumn adds tasks.path when an interrupted or broken migration
// left it out. A healthy database keeps its column; the ALTER is skipped.
func ensureTaskPathColumn(db *DB) error {
	has, err := hasColumn(db, "tasks", "path")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = db.conn.Exec(schemaV14)
	return err
}

// hasColumn reports whether table already has the named column.
func hasColumn(db *DB, table, column string) (bool, error) {
	rows, err := db.conn.Query(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column,
	)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return false, rows.Err()
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, rows.Err()
}

func backfillProjectPaths(db *DB) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	base := filepath.Join(home, "dev", "projects")

	rows, err := db.conn.Query(`SELECT name FROM projects WHERE path = ''`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		// A row that cannot be read is skipped: the backfill is opportunistic
		// and must never block the schema migration.
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	// The rows are drained, not updated in place: with database/sql an UPDATE
	// during an open result set may take a second connection, and an in-memory
	// database gives that connection an empty schema of its own.
	_ = rows.Close()

	for _, name := range names {
		candidate := filepath.Join(base, name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			// Best effort: a failed update leaves the project with no path, which
			// resolves to ~/Tasks anyway.
			_, _ = db.conn.Exec(`UPDATE projects SET path = ? WHERE name = ? AND path = ''`, candidate, name)
		}
	}
	return nil
}

// migrations is the ordered list of migrations. `run` allows data
// migrations in Go (e.g. rewriting JSON); if present, `query` is ignored.
var migrations = []struct {
	version string
	query   string
	run     func(*DB) error
}{
	{"001", schemaV1, nil},
	{"002", schemaV2, nil},
	{"003", schemaV3, nil},
	{"004", schemaV4, nil},
	{"005", schemaV5, nil},
	{"006", schemaV6, nil},
	{"007", schemaV7, nil},
	{"008", schemaV8, nil},
	// 009-011 were DATA MIGRATIONS (inserting "delivered" into the
	// workflows, repositioning it before review and renaming "review" ->
	// "reviewing"). They have been removed: only this repository's database
	// ever ran them, and it is already at 011, so no other schema can ever run
	// them again. The slots are reserved because the version number is
	// PERSISTED in the DB: deleting them would make every database with schema_version
	// >= 009 silently skip future migrations (012+).
	{"009", "", noopMigration},
	{"010", "", noopMigration},
	{"011", "", noopMigration},
	{"012", schemaV12, nil},
	{"013", schemaV13, nil},
	{"014", "", migrateV14},
	{"015", "", migrateV15},
}

// noopMigration is the run of a reserved slot: it does not touch the schema, it only
// marks the version as applied so as not to break the number sequence.
func noopMigration(*DB) error { return nil }
