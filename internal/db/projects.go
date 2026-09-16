package db

import (
	"database/sql"
	"fmt"
	"time"

	"taskd/internal/model"
)

// CreateProject registra un proyecto nuevo.
func (db *DB) CreateProject(name, path string, workflow []string) (*model.Project, error) {
	if workflow == nil {
		workflow = model.DefaultWorkflow
	}
	if err := model.ValidateWorkflow(workflow); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.conn.Exec(
		`INSERT INTO projects (name, path, workflow, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, path, model.WorkflowJSON(workflow), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	id, _ := result.LastInsertId()
	return &model.Project{
		ID:        id,
		Name:      name,
		Path:      path,
		Workflow:  workflow,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetProject busca un proyecto por nombre.
func (db *DB) GetProject(name string) (*model.Project, error) {
	var p model.Project
	var workflowJSON string
	err := db.conn.QueryRow(
		`SELECT id, name, path, workflow, created_at, updated_at FROM projects WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Path, &workflowJSON, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	if err != nil {
		return nil, err
	}
	p.Workflow, err = model.ParseWorkflowJSON(workflowJSON)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProjectByID busca un proyecto por ID.
func (db *DB) GetProjectByID(id int64) (*model.Project, error) {
	var p model.Project
	var workflowJSON string
	err := db.conn.QueryRow(
		`SELECT id, name, path, workflow, created_at, updated_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Path, &workflowJSON, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found: id %d", id)
	}
	if err != nil {
		return nil, err
	}
	p.Workflow, err = model.ParseWorkflowJSON(workflowJSON)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListProjects devuelve todos los proyectos.
func (db *DB) ListProjects() ([]model.Project, error) {
	rows, err := db.conn.Query(`SELECT id, name, path, workflow, created_at, updated_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		var workflowJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &workflowJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Workflow, err = model.ParseWorkflowJSON(workflowJSON)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProject actualiza un proyecto.
func (db *DB) UpdateProject(name string, updates map[string]any) error {
	p, err := db.GetProject(name)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if wf, ok := updates["workflow"]; ok {
		workflow := wf.([]string)
		if err := model.ValidateWorkflow(workflow); err != nil {
			return err
		}
		// Verificar que no se eliminen estados con tareas
		for _, oldStatus := range p.Workflow {
			if oldStatus == model.TerminalStatus(p.Workflow) {
				continue // done no se puede eliminar nunca
			}
			if !model.HasStatus(workflow, oldStatus) {
				// Verificar si hay tareas en ese estado
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
					// --force: reasignar tareas al primer estado del nuevo workflow
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
	}

	if path, ok := updates["path"]; ok {
		updates["path"] = path.(string)
	}

	// Remove non-DB keys
	delete(updates, "force")

	if len(updates) == 0 {
		return nil
	}

	// Construir UPDATE dinámico
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
	_, err = db.conn.Exec(query, args...)
	return err
}

// DeleteProject elimina un proyecto y sus tareas (CASCADE).
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

// ProjectTaskCount devuelve el número de tareas de un proyecto.
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
