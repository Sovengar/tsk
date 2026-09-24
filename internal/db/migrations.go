package db

import (
	"strings"
	"time"

	"tsk/internal/model"
)

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

// schemaV7 agrega la cuantificación de tareas (estimate, en días) y los
// off-days personales (vacaciones, feriados, ausencias). Un off-day cubre el
// rango inclusivo [start_date, end_date]; sin motivo, solo importa que la
// persona no estará disponible.
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

// schemaV8 agrega tags a las tareas. Se guardan como array JSON (mismo patrón
// que workflow/list_order) para poder consultarlas con json_each.
const schemaV8 = `
ALTER TABLE tasks ADD COLUMN tags TEXT NOT NULL DEFAULT '[]';
`

// migrations es la lista de migraciones en orden. `run` permite migraciones de
// datos en Go (p. ej. reescribir JSON); si está presente, `query` se ignora.
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
	{"009", "", migrateAddDelivered},
	{"010", "", migrateRepositionDelivered},
	{"011", "", migrateRenameReview},
}

// migrateAddDelivered inserta el estado "delivered" (entrega sin verificar) en
// el workflow de los proyectos que no lo tengan, antes de "reviewing" o, si no
// existe, antes de "done". También lo inserta en list_order cuando es explícito.
func migrateAddDelivered(db *DB) error {
	rows, err := db.conn.Query(`SELECT id, workflow, list_order FROM projects`)
	if err != nil {
		return err
	}
	type projectRow struct {
		id        int64
		workflow  string
		listOrder string
	}
	var projects []projectRow
	for rows.Next() {
		var p projectRow
		if err := rows.Scan(&p.id, &p.workflow, &p.listOrder); err != nil {
			_ = rows.Close()
			return err
		}
		projects = append(projects, p)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	const delivered = "delivered"
	for _, p := range projects {
		workflow, err := model.ParseWorkflowJSON(p.workflow)
		if err != nil {
			return err
		}
		listOrder, err := model.ParseWorkflowJSON(p.listOrder)
		if err != nil {
			return err
		}
		if model.HasStatus(workflow, delivered) {
			continue
		}
		workflow = injectStatus(workflow, delivered)
		if len(listOrder) > 0 {
			listOrder = injectStatus(listOrder, delivered)
		}
		if _, err := db.conn.Exec(
			`UPDATE projects SET workflow = ?, list_order = ? WHERE id = ?`,
			model.WorkflowJSON(workflow), model.WorkflowJSON(listOrder), p.id,
		); err != nil {
			return err
		}
	}
	return nil
}

// injectStatus inserta status antes del primer estado de revisión (cuyo nombre
// contenga "review", como "reviewing" o "review") o, si no hay, antes de
// "done". Si ninguna ancla existe, lo agrega al final. Así delivered queda
// siempre inmediatamente antes de la revisión.
func injectStatus(list []string, status string) []string {
	insertAt := reviewIndex(list)
	if insertAt < 0 {
		for i, s := range list {
			if s == "done" {
				insertAt = i
				break
			}
		}
	}
	if insertAt < 0 {
		return append(list, status)
	}
	out := make([]string, 0, len(list)+1)
	out = append(out, list[:insertAt]...)
	out = append(out, status)
	out = append(out, list[insertAt:]...)
	return out
}

// reviewIndex devuelve el índice del primer estado de revisión (contiene
// "review", case-insensitive) o -1.
func reviewIndex(list []string) int {
	for i, s := range list {
		if strings.Contains(strings.ToLower(s), "review") {
			return i
		}
	}
	return -1
}

// migrateRepositionDelivered corrige bases ya migradas por 009 donde delivered
// quedó después de un estado de revisión con otro nombre (p. ej. "review"). Lo
// mueve a la posición inmediatamente anterior a la revisión.
func migrateRepositionDelivered(db *DB) error {
	rows, err := db.conn.Query(`SELECT id, workflow FROM projects`)
	if err != nil {
		return err
	}
	type projectRow struct {
		id       int64
		workflow string
	}
	var projects []projectRow
	for rows.Next() {
		var p projectRow
		if err := rows.Scan(&p.id, &p.workflow); err != nil {
			_ = rows.Close()
			return err
		}
		projects = append(projects, p)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	const delivered = "delivered"
	for _, p := range projects {
		workflow, err := model.ParseWorkflowJSON(p.workflow)
		if err != nil {
			return err
		}
		fixed := moveBeforeReview(workflow, delivered)
		if sameStrings(workflow, fixed) {
			continue
		}
		if _, err := db.conn.Exec(
			`UPDATE projects SET workflow = ? WHERE id = ?`,
			model.WorkflowJSON(fixed), p.id,
		); err != nil {
			return err
		}
	}
	return nil
}

// moveBeforeReview mueve status para que quede antes del primer estado de
// revisión. Si no hay revisión, status no existe o ya está antes, devuelve la
// lista sin cambios.
func moveBeforeReview(list []string, status string) []string {
	statusIdx := -1
	for i, s := range list {
		if s == status {
			statusIdx = i
			break
		}
	}
	revIdx := reviewIndex(list)
	if statusIdx < 0 || revIdx < 0 || statusIdx < revIdx {
		return list
	}
	without := make([]string, 0, len(list))
	for _, s := range list {
		if s != status {
			without = append(without, s)
		}
	}
	revIdx = reviewIndex(without)
	if revIdx < 0 {
		return list
	}
	out := make([]string, 0, len(without)+1)
	out = append(out, without[:revIdx]...)
	out = append(out, status)
	out = append(out, without[revIdx:]...)
	return out
}

// migrateRenameReview renombra el estado exacto "review" a "reviewing" en el
// workflow y list_order de los proyectos existentes, y mueve sus tareas de
// "review" a "reviewing". Preserva la posición del estado (p. ej. "delivered"
// sigue quedando inmediatamente antes de la revisión). Es idempotente: correrla
// de nuevo no produce cambios.
func migrateRenameReview(db *DB) error {
	rows, err := db.conn.Query(`SELECT id, workflow, list_order FROM projects`)
	if err != nil {
		return err
	}
	type projectRow struct {
		id        int64
		workflow  string
		listOrder string
	}
	var projects []projectRow
	for rows.Next() {
		var p projectRow
		if err := rows.Scan(&p.id, &p.workflow, &p.listOrder); err != nil {
			_ = rows.Close()
			return err
		}
		projects = append(projects, p)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, p := range projects {
		workflow, err := model.ParseWorkflowJSON(p.workflow)
		if err != nil {
			return err
		}
		if !model.HasStatus(workflow, legacyReviewStatus) {
			continue
		}
		listOrder, err := model.ParseWorkflowJSON(p.listOrder)
		if err != nil {
			return err
		}

		workflow = renameReview(workflow)
		if _, err := db.conn.Exec(
			`UPDATE projects SET workflow = ?, list_order = ?, updated_at = ? WHERE id = ?`,
			model.WorkflowJSON(workflow), model.WorkflowJSON(renameReview(listOrder)), now, p.id,
		); err != nil {
			return err
		}

		if _, err := db.conn.Exec(
			`UPDATE tasks SET status = ?, updated_at = ? WHERE project_id = ? AND status = ?`,
			ReviewStatus, now, p.id, legacyReviewStatus,
		); err != nil {
			return err
		}
	}
	return nil
}

// ReviewStatus es el nombre canónico del estado de revisión.
const ReviewStatus = "reviewing"

// legacyReviewStatus es el nombre viejo del estado de revisión, renombrado por
// la migración 011.
const legacyReviewStatus = "review"

// renameReview reemplaza el estado exacto "review" por "reviewing". Si la lista
// ya contiene "reviewing", descarta "review" para no duplicar el estado; en
// cualquier caso conserva el orden del resto de los estados.
func renameReview(list []string) []string {
	hasReviewing := false
	for _, s := range list {
		if s == ReviewStatus {
			hasReviewing = true
			break
		}
	}
	out := make([]string, 0, len(list))
	for _, s := range list {
		if s == legacyReviewStatus {
			if hasReviewing {
				continue
			}
			s = ReviewStatus
		}
		out = append(out, s)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
