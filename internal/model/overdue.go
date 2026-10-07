package model

import "time"

// IsOverdue (pure helper) reports whether a task whose due date is due is overdue
// relative to now. A zero due (no due date) is never overdue; neither is a
// future date.
func IsOverdue(due, now time.Time) bool {
	if due.IsZero() {
		return false
	}
	return due.Before(now)
}
