package model

import (
	"testing"
	"time"
)

func TestIsOverdue(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if !IsOverdue(now.Add(-time.Hour), now) {
		t.Fatal("past due should be overdue")
	}
	if IsOverdue(now.Add(time.Hour), now) {
		t.Fatal("future due should not be overdue")
	}
	if IsOverdue(time.Time{}, now) {
		t.Fatal("zero due should never be overdue")
	}
}
