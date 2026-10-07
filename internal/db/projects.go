package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tsk/internal/model"
)

// CreateProject registers a new project with the default list order
// (empty = follow the workflow order).
func (db *DB) CreateProject(name string, workflow []string) (*model.Project, error) {
	return db.CreateProjectWithListOrder(name, workflow, nil)
}

// CreateProjectWithListOrder registers a new project with explicit
// workflow and list_order.
func (db *DB) CreateProjectWithListOrder(name string, workflow, listOrder []string) (*model.Project, error) {
	if workflow == nil {
		workflow = model.DefaultWorkflow
	}
	if err := model.ValidateWorkflow(workflow); err != nil {
		return nil, err
	}
	if err := model.ValidateListOrder(workflow, listOrder); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.conn.Exec(
		`INSERT INTO projects (name, workflow, list_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, model.WorkflowJSON(workflow), model.WorkflowJSON(listOrder), now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("project already exists: %s", name)
		}
		return nil, fmt.Errorf("create project: %w", err)
	}

	id, _ := result.LastInsertId()
	return &model.Project{
		ID:        id,
		Name:      name,
		Workflow:  workflow,
		ListOrder: listOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// projectColumns is the list of projects columns in scan order.
const projectColumns = `id, name, workflow, list_order, archived, archived_at, created_at, updated_at`

// scanProject scans a projects row using projectColumns.
func scanProject(scan func(dest ...any) error) (model.Project, error) {
	var p model.Project
	var workflowJSON, listOrderJSON string
	var archived int
	var archivedAt sql.NullString
	if err := scan(&p.ID, &p.Name, &workflowJSON, &listOrderJSON, &archived, &archivedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return p, err
	}
	wf, err := model.ParseWorkflowJSON(workflowJSON)
	if err != nil {
		return p, err
	}
	p.Workflow = wf
	lo, err := model.ParseWorkflowJSON(listOrderJSON)
	if err != nil {
		return p, err
	}
	p.ListOrder = lo
	p.Archived = archived != 0
	if archivedAt.Valid {
		p.ArchivedAt = archivedAt.String
	}
	return p, nil
}

// GetProject looks up a project by name, including archived ones.
func (db *DB) GetProject(name string) (*model.Project, error) {
	p, err := scanProject(db.conn.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE name = ?`, name,
	).Scan)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProjectByID looks up a project by ID, including archived ones.
func (db *DB) GetProjectByID(id int64) (*model.Project, error) {
	p, err := scanProject(db.conn.QueryRow(
		`SELECT `+projectColumns+` FROM projects WHERE id = ?`, id,
	).Scan)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found: id %d", id)
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListProjects returns the active (non-archived) projects, ordered by name.
func (db *DB) ListProjects() ([]model.Project, error) {
	return db.listProjectsWhere(`archived = 0`)
}

// ListArchivedProjects returns the archived projects, ordered by name.
func (db *DB) ListArchivedProjects() ([]model.Project, error) {
	return db.listProjectsWhere(`archived = 1`)
}

func (db *DB) listProjectsWhere(where string) ([]model.Project, error) {
	rows, err := db.conn.Query(`SELECT ` + projectColumns + ` FROM projects WHERE ` + where + ` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var projects []model.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ArchiveProject marks a project as archived (soft delete). Tasks are not
// touched: they are hidden transitively by excluding the archived project.
func (db *DB) ArchiveProject(name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.conn.Exec(
		`UPDATE projects SET archived = 1, archived_at = ?, updated_at = ? WHERE name = ? AND archived = 0`,
		now, now, name,
	)
	if err != nil {
		return fmt.Errorf("archive project: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("project not found or already archived: %s", name)
	}
	return nil
}

// UnarchiveProject restores an archived project.
func (db *DB) UnarchiveProject(name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.conn.Exec(
		`UPDATE projects SET archived = 0, archived_at = NULL, updated_at = ? WHERE name = ? AND archived = 1`,
		now, name,
	)
	if err != nil {
		return fmt.Errorf("unarchive project: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("project not found or not archived: %s", name)
	}
	return nil
}

// UpdateProject updates a project.
func (db *DB) UpdateProject(name string, updates map[string]any) error {
	p, err := db.GetProject(name)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if newName, ok := updates["name"]; ok {
		s, _ := newName.(string)
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("project name cannot be empty")
		}
		updates["name"] = strings.TrimSpace(s)
	}

	// effectiveWorkflow is the workflow resulting from applying the updates
	// (or the current one if untouched). It is used to validate list_order even when both
	// change in the same call.
	effectiveWorkflow := p.Workflow
	workflowChanged := false

	if wf, ok := updates["workflow"]; ok {
		workflow := wf.([]string)
		if err := model.ValidateWorkflow(workflow); err != nil {
			return err
		}
		// Check that statuses with tasks are not removed
		for _, oldStatus := range p.Workflow {
			if oldStatus == model.TerminalStatus(p.Workflow) {
				continue // done can never be removed
			}
			if !model.HasStatus(workflow, oldStatus) {
				// Check whether there are tasks in that status
				var count int
				err := db.conn.QueryRow(
					`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND status = ? AND status != ?`,
					p.ID, oldStatus, model.CancelledStatus,
				).Scan(&count)
				if err != nil {
					return err
				}
				if count > 0 {
					force, _ := updates["force"].(bool)
					if !force {
						return fmt.Errorf("cannot remove status %q — %d tasks still in this status", oldStatus, count)
					}
					// --force: reassign tasks to the first status of the new workflow
					firstStatus := workflow[0]
					if _, err := db.conn.Exec(
						`UPDATE tasks SET status = ?, updated_at = ? WHERE project_id = ? AND status = ?`,
						firstStatus, now, p.ID, oldStatus,
					); err != nil {
						return err
					}
				}
			}
		}
		updates["workflow"] = model.WorkflowJSON(workflow)
		effectiveWorkflow = workflow
		workflowChanged = true
	}

	if lo, ok := updates["list_order"]; ok {
		listOrder := lo.([]string)
		if err := model.ValidateListOrder(effectiveWorkflow, listOrder); err != nil {
			return err
		}
		updates["list_order"] = model.WorkflowJSON(listOrder)
	} else if workflowChanged {
		// The workflow changed without touching list_order: drop entries that no longer
		// exist to avoid desynchronization.
		var kept []string
		for _, s := range p.ListOrder {
			if s == model.CancelledStatus || model.HasStatus(effectiveWorkflow, s) {
				kept = append(kept, s)
			}
		}
		if len(kept) != len(p.ListOrder) {
			updates["list_order"] = model.WorkflowJSON(kept)
		}
	}

	delete(updates, "force")

	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE
	setClauses := []string{}
	args := []any{}
	for k, v := range updates {
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now)
	args = append(args, p.ID)

	query := fmt.Sprintf("UPDATE projects SET %s WHERE id = ?", joinStrings(setClauses, ", "))
	if _, err = db.conn.Exec(query, args...); err != nil {
		if isUniqueViolation(err) {
			// The only UNIQUE column of projects is name, so a
			// violation can only come from updates["name"], which is always
			// present when the name is updated. The generic message there was for the
			// case where it was never present, which never runs.
			return fmt.Errorf("project already exists: %s", updates["name"])
		}
		return err
	}
	return nil
}

// DeleteProject deletes a project and its tasks (CASCADE).
func (db *DB) DeleteProject(name string) error {
	result, err := db.conn.Exec(`DELETE FROM projects WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project not found: %s", name)
	}
	return nil
}

// ProjectTaskCount returns the number of tasks of a project.
func (db *DB) ProjectTaskCount(projectID int64) (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ?`, projectID).Scan(&count)
	return count, err
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// isUniqueViolation detects violations of SQLite's UNIQUE index.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
