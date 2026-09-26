package model

import "time"

// IsOverdue (helper puro) indica si una tarea cuyo vencimiento es due esta vencida respecto
// de now. Un due cero (sin vencimiento) nunca esta vencido; tampoco una fecha
// futura.
func IsOverdue(due, now time.Time) bool {
	if due.IsZero() {
		return false
	}
	return due.Before(now)
}
