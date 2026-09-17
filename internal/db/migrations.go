package db

// schemaV1 es el schema inicial de la base de datos.
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

// schemaV2 añade los comentarios de tarea.
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

// schemaV3 añade el archivado (soft delete) de proyectos.
const schemaV3 = `
ALTER TABLE projects ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN archived_at TEXT;
CREATE INDEX IF NOT EXISTS idx_projects_archived ON projects(archived);
`

// schemaV4 elimina la columna position: el orden de las tareas ya no es manual
// sino que se deriva del workflow del proyecto.
const schemaV4 = `
ALTER TABLE tasks DROP COLUMN position;
`

// schemaV5 separa el orden de presentación de la vista List del workflow
// (que define la progresión de las acciones). list_order es un array JSON de
// estados; vacío ('[]') significa "usar el orden del workflow".
const schemaV5 = `
ALTER TABLE projects ADD COLUMN list_order TEXT NOT NULL DEFAULT '[]';
`

// schemaV6 elimina path: era metadata sin uso en la app.
const schemaV6 = `
ALTER TABLE projects DROP COLUMN path;
`

// migrations es la lista de migraciones en orden.
var migrations = []struct {
	version string
	query   string
}{
	{"001", schemaV1},
	{"002", schemaV2},
	{"003", schemaV3},
	{"004", schemaV4},
	{"005", schemaV5},
	{"006", schemaV6},
}
