package db

import (
	"database/sql"
	"fmt"
	"time"

	"tsk/internal/model"
)

// CreateTask creates a new task without an estimate.
func (db *DB) CreateTask(projectName, title, description, assignee string, priority int, status string) (*model.Task, error) {
	return db.CreateTaskWithEstimate(projectName, title, description, assignee, priority, status, 0)
}

// CreateTaskWithEstimate creates a new task with an estimate in days.
func (db *DB) CreateTaskWithEstimate(projectName, title, description, assignee string, priority int, status string, estimate float64) (*model.Task, error) {
	return db.CreateTaskFull(projectName, title, description, assignee, priority, status, estimate, nil)
}

// CreateTaskFull creates a new task with estimate and tags.
func (db *DB) CreateTaskFull(projectName, title, description, assignee string, priority int, status string, estimate float64, tags []string) (*model.Task, error) {
	p, err := db.GetProject(projectName)
	if err != nil {
		return nil, err
	}
	if p.Archived {
		return nil, fmt.Errorf("project %q is archived", projectName)
	}

	if status == "" {
		status = p.Workflow[0] // first status of the workflow
	}
	if !model.HasStatus(p.Workflow, status) {
		return nil, fmt.Errorf("status %q not in project workflow %v", status, p.Workflow)
	}
	if assignee == "" {
		assignee = "unassigned"
	}
	tags = model.NormalizeTags(tags)

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.conn.Exec(
		`INSERT INTO tasks (project_id, title, description, status, priority, assignee, estimate, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, title, description, status, priority, assignee, estimate, model.TagsJSON(tags), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	id, _ := result.LastInsertId()
	return &model.Task{
		ID:          id,
		ProjectID:   p.ID,
		ProjectName: p.Name,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		Assignee:    assignee,
		Estimate:    estimate,
		Tags:        tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetTask looks up a task by ID.
func (db *DB) GetTask(id int64) (*model.Task, error) {
	var t model.Task
	var completedAt sql.NullString
	var tagsJSON string
	err := db.conn.QueryRow(`
		SELECT t.id, t.project_id, p.name, t.title, t.description, t.status, t.priority, t.assignee,
		       t.estimate, t.tags, t.created_at, t.updated_at, t.completed_at
		FROM tasks t JOIN projects p ON t.project_id = p.id
		WHERE t.id = ?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.ProjectName, &t.Title, &t.Description, &t.Status,
		&t.Priority, &t.Assignee, &t.Estimate, &tagsJSON, &t.CreatedAt, &t.UpdatedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %d", id)
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		t.CompletedAt = completedAt.String
	}
	t.Tags = model.ParseTagsJSON(tagsJSON)
	return &t, nil
}

// ListTasks lists tasks with optional filters.
func (db *DB) ListTasks(projectName, status, assignee string) ([]model.Task, error) {
	// The status order of the List comes from list_order (presentation
	// order, independent of the workflow). When empty ('[]') it falls back to the
	// workflow order. Unlisted statuses come after the listed ones, and those
	// that do not exist in the workflow (e.g. cancelled) at the end.
	query := `
		WITH ranked AS (
			SELECT t.id, t.project_id, p.name, t.title, t.description, t.status, t.priority, t.assignee,
			       t.estimate, t.tags, t.created_at, t.updated_at, t.completed_at,
			       (SELECT je.key FROM json_each(p.list_order) AS je WHERE je.value = t.status) AS lo_rank,
			       (SELECT COUNT(*) FROM json_each(p.list_order)) AS lo_len,
			       (SELECT je.key FROM json_each(p.workflow) AS je WHERE je.value = t.status) AS wf_rank
			FROM tasks t JOIN projects p ON t.project_id = p.id
			WHERE p.archived = 0`
	args := []any{}

	if projectName != "" {
		query += " AND p.name = ?"
		args = append(args, projectName)
	}
	if status != "" {
		query += " AND t.status = ?"
		args = append(args, status)
	}
	if assignee != "" {
		query += " AND t.assignee = ?"
		args = append(args, assignee)
	}

	query += `
		)
		SELECT id, project_id, name, title, description, status, priority, assignee,
		       estimate, tags, created_at, updated_at, completed_at
		FROM ranked
		ORDER BY priority DESC,
		         CASE
		           WHEN lo_rank IS NOT NULL THEN lo_rank
		           WHEN lo_len > 0 THEN lo_len
		           ELSE COALESCE(wf_rank, 9999)
		         END ASC,
		         COALESCE(wf_rank, 9999) ASC,
		         assignee ASC,
		         id ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var completedAt sql.NullString
		var tagsJSON string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.ProjectName, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.Assignee, &t.Estimate, &tagsJSON, &t.CreatedAt, &t.UpdatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			t.CompletedAt = completedAt.String
		}
		t.Tags = model.ParseTagsJSON(tagsJSON)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// MoveTask moves a task to a specific status.
func (db *DB) MoveTask(id int64, newStatus string) (*model.Task, error) {
	t, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}

	p, err := db.GetProjectByID(t.ProjectID)
	if err != nil {
		return nil, err
	}

	if newStatus != model.CancelledStatus && !model.HasStatus(p.Workflow, newStatus) {
		return nil, fmt.Errorf("status %q not in project workflow %v", newStatus, p.Workflow)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	completedAt := "NULL"
	if newStatus == model.TerminalStatus(p.Workflow) || newStatus == model.CancelledStatus {
		completedAt = "datetime('now')"
	}

	query := fmt.Sprintf(`UPDATE tasks SET status = ?, updated_at = ?, completed_at = %s WHERE id = ?`, completedAt)
	if _, err := db.conn.Exec(query, newStatus, now, id); err != nil {
		return nil, err
	}

	return db.GetTask(id)
}

// StartTask moves to the second status of the workflow.
func (db *DB) StartTask(id int64) (*model.Task, error) {
	t, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}
	p, err := db.GetProjectByID(t.ProjectID)
	if err != nil {
		return nil, err
	}
	return db.MoveTask(id, model.StartStatus(p.Workflow))
}

// ReviewTask moves to the status containing "review".
func (db *DB) ReviewTask(id int64) (*model.Task, error) {
	t, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}
	p, err := db.GetProjectByID(t.ProjectID)
	if err != nil {
		return nil, err
	}
	status, ok := model.FindStatusContaining(p.Workflow, "review")
	if !ok {
		return nil, fmt.Errorf("no 'review' status in project %q workflow", p.Name)
	}
	return db.MoveTask(id, status)
}

// DoneTask moves to the last status of the workflow.
func (db *DB) DoneTask(id int64) (*model.Task, error) {
	t, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}
	p, err := db.GetProjectByID(t.ProjectID)
	if err != nil {
		return nil, err
	}
	return db.MoveTask(id, model.TerminalStatus(p.Workflow))
}

// CancelTask cancels a task.
func (db *DB) CancelTask(id int64) (*model.Task, error) {
	return db.MoveTask(id, model.CancelledStatus)
}

// UpdateTask updates task metadata.
func (db *DB) UpdateTask(id int64, updates map[string]any) (*model.Task, error) {
	_, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	setClauses := []string{}
	args := []any{}

	for k, v := range updates {
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now)
	args = append(args, id)

	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", joinStrings(setClauses, ", "))
	if _, err := db.conn.Exec(query, args...); err != nil {
		return nil, err
	}

	return db.GetTask(id)
}

// SetTaskTags replaces a task's tags.
func (db *DB) SetTaskTags(id int64, tags []string) (*model.Task, error) {
	if _, err := db.GetTask(id); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.conn.Exec(
		`UPDATE tasks SET tags = ?, updated_at = ? WHERE id = ?`,
		model.TagsJSON(model.NormalizeTags(tags)), now, id,
	); err != nil {
		return nil, err
	}
	return db.GetTask(id)
}

// AddTaskTags adds tags without duplicating existing ones.
func (db *DB) AddTaskTags(id int64, tags []string) (*model.Task, error) {
	t, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}
	return db.SetTaskTags(id, append(append([]string{}, t.Tags...), tags...))
}

// RemoveTaskTags removes the given tags from a task.
func (db *DB) RemoveTaskTags(id int64, tags []string) (*model.Task, error) {
	t, err := db.GetTask(id)
	if err != nil {
		return nil, err
	}
	remove := make(map[string]bool)
	for _, tag := range model.NormalizeTags(tags) {
		remove[tag] = true
	}
	var kept []string
	for _, tag := range t.Tags {
		if !remove[tag] {
			kept = append(kept, tag)
		}
	}
	return db.SetTaskTags(id, kept)
}

// Stats returns task statistics, excluding archived projects.
func (db *DB) Stats(projectName string) (map[string]any, error) {
	joinQuery := ` JOIN projects p ON t.project_id = p.id WHERE p.archived = 0`
	args := []any{}

	if projectName != "" {
		joinQuery += ` AND p.name = ?`
		args = append(args, projectName)
	}

	// Total
	var total int
	totalQuery := `SELECT COUNT(*) FROM tasks t` + joinQuery
	if err := db.conn.QueryRow(totalQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// By status
	statusQuery := `SELECT t.status, COUNT(*) FROM tasks t` + joinQuery + ` GROUP BY t.status`
	rows, err := db.conn.Query(statusQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	byStatus := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		byStatus[status] = count
	}

	// By assignee
	assigneeQuery := `SELECT t.assignee, COUNT(*) FROM tasks t` + joinQuery + ` GROUP BY t.assignee ORDER BY COUNT(*) DESC`
	rows2, err := db.conn.Query(assigneeQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows2.Close() }()

	byAssignee := map[string]int{}
	for rows2.Next() {
		var assignee string
		var count int
		if err := rows2.Scan(&assignee, &count); err != nil {
			return nil, err
		}
		byAssignee[assignee] = count
	}

	return map[string]any{
		"total":       total,
		"by_status":   byStatus,
		"by_assignee": byAssignee,
	}, nil
}
