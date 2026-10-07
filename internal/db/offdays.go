package db

import (
	"fmt"
	"strings"

	"tsk/internal/model"
)

// AddOffDay registers a non-working day for a person. An empty endDate
// means a single day (endDate = startDate).
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
	// Normalize the range with min/max instead of with an
	// `if endDate < startDate` + swap: when start == end the swap reassigned
	// the same value to both variables, so the branch was an equivalent
	// mutant no test could kill. The RHS is evaluated entirely before
	// assigning, so min and max read the original values.
	startDate, endDate = min(startDate, endDate), max(startDate, endDate)

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

// ListOffDays returns a person's off-days, or all of them if assignee is
// empty, ordered by start date.
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
	defer func() { _ = rows.Close() }()

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

// DeleteOffDay deletes an off-day by ID.
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
