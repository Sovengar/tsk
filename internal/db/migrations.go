package db

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
}

// noopMigration is the run of a reserved slot: it does not touch the schema, it only
// marks the version as applied so as not to break the number sequence.
func noopMigration(*DB) error { return nil }
