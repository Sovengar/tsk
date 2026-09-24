package db

import (
	"fmt"
	"strings"
	"time"

	"tsk/internal/model"
)

// AddComment crea un comentario nuevo en una tarea.
func (db *DB) AddComment(taskID int64, body string) (*model.Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("comment body cannot be empty")
	}
	if _, err := db.GetTask(taskID); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.conn.Exec(
		`INSERT INTO comments (task_id, body, created_at) VALUES (?, ?, ?)`,
		taskID, body, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	id, _ := result.LastInsertId()
	return &model.Comment{
		ID:        id,
		TaskID:    taskID,
		Body:      body,
		CreatedAt: now,
	}, nil
}

// ListComments devuelve los comentarios de una tarea en orden cronológico.
func (db *DB) ListComments(taskID int64) ([]model.Comment, error) {
	rows, err := db.conn.Query(
		`SELECT id, task_id, body, created_at FROM comments
		 WHERE task_id = ? ORDER BY created_at ASC, id ASC`, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// DeleteComment elimina un comentario por ID.
func (db *DB) DeleteComment(id int64) error {
	result, err := db.conn.Exec(`DELETE FROM comments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("comment not found: %d", id)
	}
	return nil
}
