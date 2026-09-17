package db

import (
	"fmt"
	"strings"

	"tsk/internal/model"
)

// AddOffDay registra un día no laborable para una persona. endDate vacío
// significa un solo día (endDate = startDate).
func (db *DB) AddOffDay(assignee, startDate, endDate, note string) (*model.OffDay, error) {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" || model.IsUnassigned(assignee) {
		return nil, fmt.Errorf("assignee is required")
	}
	startDate = strings.TrimSpace(startDate)
	if startDate == "" {
		return nil, fmt.Errorf("start date is required")
	}
	if _, err := model.ParseDate(startDate); err != nil {
		return nil, fmt.Errorf("invalid start date %q (want YYYY-MM-DD)", startDate)
	}
	endDate = strings.TrimSpace(endDate)
	if endDate == "" {
		endDate = startDate
	}
	if _, err := model.ParseDate(endDate); err != nil {
		return nil, fmt.Errorf("invalid end date %q (want YYYY-MM-DD)", endDate)
	}
	if endDate < startDate {
		startDate, endDate = endDate, startDate
	}

	result, err := db.conn.Exec(
		`INSERT INTO offdays (assignee, start_date, end_date, note) VALUES (?, ?, ?, ?)`,
		assignee, startDate, endDate, strings.TrimSpace(note),
	)
	if err != nil {
		return nil, fmt.Errorf("create offday: %w", err)
	}

	id, _ := result.LastInsertId()
	return &model.OffDay{
		ID:        id,
		Assignee:  assignee,
		StartDate: startDate,
		EndDate:   endDate,
		Note:      strings.TrimSpace(note),
	}, nil
}

// ListOffDays devuelve los off-days de una persona, o todos si assignee está
// vacío, ordenados por fecha de inicio.
func (db *DB) ListOffDays(assignee string) ([]model.OffDay, error) {
	query := `SELECT id, assignee, start_date, end_date, note FROM offdays`
	args := []any{}
	if assignee != "" {
		query += ` WHERE assignee = ?`
		args = append(args, assignee)
	}
	query += ` ORDER BY start_date ASC, assignee ASC, id ASC`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offdays []model.OffDay
	for rows.Next() {
		var o model.OffDay
		if err := rows.Scan(&o.ID, &o.Assignee, &o.StartDate, &o.EndDate, &o.Note); err != nil {
			return nil, err
		}
		offdays = append(offdays, o)
	}
	return offdays, rows.Err()
}

// DeleteOffDay elimina un off-day por ID.
func (db *DB) DeleteOffDay(id int64) error {
	result, err := db.conn.Exec(`DELETE FROM offdays WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete offday: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("offday not found: %d", id)
	}
	return nil
}
