package model

import (
	"testing"
	"time"
)

// Mon es el lunes de referencia de varios tests (2026-09-14).
const monday = "2026-09-14"

func mustDate(t *testing.T, s string) (d time.Time) {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func task(assignee, status string, estimate float64) Task {
	return Task{Assignee: assignee, Status: status, Estimate: estimate}
}

func TestBuildSchedulePacksFractions(t *testing.T) {
	tasks := []Task{
		task("@a", "todo", 0.5),
		task("@a", "todo", 0.5),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)

	if len(s.Assignees) != 1 {
		t.Fatalf("assignees = %d, want 1", len(s.Assignees))
	}
	a := s.Assignees[0]
	if len(a.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(a.Entries))
	}
	for i, e := range a.Entries {
		if e.Start != monday || e.End != monday {
			t.Errorf("entry %d = %s→%s, want %s→%s", i, e.Start, e.End, monday, monday)
		}
	}
	if a.End != monday {
		t.Errorf("assignee end = %s, want %s", a.End, monday)
	}
}

func TestBuildScheduleSpillsToNextDay(t *testing.T) {
	tasks := []Task{
		task("@a", "todo", 0.5),
		task("@a", "todo", 0.75),
		task("@a", "todo", 0.25),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)
	a := s.Assignees[0]

	if a.Entries[0].Start != monday || a.Entries[0].End != monday {
		t.Errorf("t1 = %s→%s, want %s→%s", a.Entries[0].Start, a.Entries[0].End, monday, monday)
	}
	if a.Entries[1].Start != monday || a.Entries[1].End != "2026-09-15" {
		t.Errorf("t2 = %s→%s, want %s→2026-09-15", a.Entries[1].Start, a.Entries[1].End, monday)
	}
	if a.Entries[2].Start != "2026-09-15" || a.Entries[2].End != "2026-09-15" {
		t.Errorf("t3 = %s→%s, want 2026-09-15", a.Entries[2].Start, a.Entries[2].End)
	}
}

func TestBuildScheduleSkipsWeekend(t *testing.T) {
	// Viernes 2026-09-18.
	friday := mustDate(t, "2026-09-18")
	tasks := []Task{
		task("@a", "todo", 1),
		task("@a", "todo", 1),
	}
	s := BuildSchedule(tasks, nil, friday, 1)
	a := s.Assignees[0]

	if a.Entries[0].Start != "2026-09-18" || a.Entries[0].End != "2026-09-18" {
		t.Errorf("t1 = %s→%s, want Friday", a.Entries[0].Start, a.Entries[0].End)
	}
	// El segundo cae el lunes siguiente, saltando sáb/dom.
	if a.Entries[1].Start != "2026-09-21" || a.Entries[1].End != "2026-09-21" {
		t.Errorf("t2 = %s→%s, want Monday 2026-09-21", a.Entries[1].Start, a.Entries[1].End)
	}
}

func TestBuildScheduleOffDayPerAssignee(t *testing.T) {
	offdays := []OffDay{
		{Assignee: "@alice", StartDate: monday, EndDate: monday},
	}
	tasks := []Task{
		task("@alice", "todo", 1),
		task("@bob", "todo", 1),
	}
	s := BuildSchedule(tasks, offdays, mustDate(t, monday), 1)
	byName := map[string]AssigneeSchedule{}
	for _, a := range s.Assignees {
		byName[a.Assignee] = a
	}

	if got := byName["@alice"].Entries[0]; got.Start != "2026-09-15" || got.End != "2026-09-15" {
		t.Errorf("alice (off) = %s→%s, want 2026-09-15", got.Start, got.End)
	}
	if got := byName["@bob"].Entries[0]; got.Start != monday || got.End != monday {
		t.Errorf("bob (working) = %s→%s, want %s", got.Start, got.End, monday)
	}
}

func TestBuildScheduleOffDayRange(t *testing.T) {
	// Toda la semana laboral libre → arranca el lunes siguiente.
	offdays := []OffDay{
		{Assignee: "@a", StartDate: monday, EndDate: "2026-09-18"},
	}
	s := BuildSchedule([]Task{task("@a", "todo", 1)}, offdays, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries[0]
	if e.Start != "2026-09-21" || e.End != "2026-09-21" {
		t.Errorf("start/end = %s→%s, want 2026-09-21", e.Start, e.End)
	}
}

func TestBuildScheduleDefaultEstimate(t *testing.T) {
	tasks := []Task{task("@a", "todo", 0)}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 2)
	e := s.Assignees[0].Entries[0]

	if e.Estimate != 2 {
		t.Errorf("estimate = %v, want 2 (default)", e.Estimate)
	}
	if !e.EstimateDefaulted {
		t.Error("EstimateDefaulted should be true")
	}
	if e.Start != monday || e.End != "2026-09-15" {
		t.Errorf("range = %s→%s, want %s→2026-09-15", e.Start, e.End, monday)
	}
}

func TestBuildScheduleExplicitEstimateNotDefaulted(t *testing.T) {
	s := BuildSchedule([]Task{task("@a", "todo", 0.25)}, nil, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries[0]
	if e.EstimateDefaulted {
		t.Error("explicit estimate should not be marked defaulted")
	}
	if e.Estimate != 0.25 {
		t.Errorf("estimate = %v, want 0.25", e.Estimate)
	}
}

func TestBuildScheduleIgnoresTerminalTasks(t *testing.T) {
	tasks := []Task{
		task("@a", "done", 1),
		task("@a", "cancelled", 1),
		task("@a", "todo", 1),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)
	if len(s.Assignees) != 1 || len(s.Assignees[0].Entries) != 1 {
		t.Fatalf("expected only 1 scheduled entry, got %+v", s.Assignees)
	}
}

func TestBuildScheduleUnassigned(t *testing.T) {
	tasks := []Task{
		task("unassigned", "todo", 1),
		task("", "todo", 1),
		task("@a", "todo", 1),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)
	if len(s.Unassigned) != 2 {
		t.Errorf("unassigned = %d, want 2", len(s.Unassigned))
	}
	if len(s.Assignees) != 1 || s.Assignees[0].Assignee != "@a" {
		t.Errorf("assignees = %+v, want only @a", s.Assignees)
	}
}

func TestFilterScheduleKeepsRealDates(t *testing.T) {
	tasks := []Task{
		{ID: 1, ProjectName: "api", Assignee: "@a", Status: "todo", Estimate: 1},
		{ID: 2, ProjectName: "web", Assignee: "@a", Status: "todo", Estimate: 1},
	}
	full := BuildSchedule(tasks, nil, mustDate(t, monday), 1)

	// Filtrar a "web" no debe recortar su fecha: sigue en el martes, aunque en
	// la vista no se vea la tarea de "api" del lunes.
	onlyWeb := FilterSchedule(full, func(t Task) bool { return t.ProjectName == "web" })
	if len(onlyWeb.Assignees) != 1 || len(onlyWeb.Assignees[0].Entries) != 1 {
		t.Fatalf("filtered assignees = %+v", onlyWeb.Assignees)
	}
	web := onlyWeb.Assignees[0].Entries[0]
	if web.Start != "2026-09-15" || web.End != "2026-09-15" {
		t.Errorf("web task = %s→%s, want 2026-09-15 (real slot, not recalculated)", web.Start, web.End)
	}
	if onlyWeb.Assignees[0].End != "2026-09-15" {
		t.Errorf("assignee end = %s, want 2026-09-15", onlyWeb.Assignees[0].End)
	}
}

func TestFilterScheduleDropsEmptyAndFiltersUnassigned(t *testing.T) {
	tasks := []Task{
		{ID: 1, ProjectName: "api", Assignee: "@a", Status: "todo", Estimate: 1},
		{ID: 2, ProjectName: "api", Assignee: "unassigned", Status: "todo", Estimate: 1},
	}
	full := BuildSchedule(tasks, nil, mustDate(t, monday), 1)

	onlyWeb := FilterSchedule(full, func(t Task) bool { return t.ProjectName == "web" })
	if len(onlyWeb.Assignees) != 0 {
		t.Errorf("assignees = %+v, want none", onlyWeb.Assignees)
	}
	if len(onlyWeb.Unassigned) != 0 {
		t.Errorf("unassigned = %+v, want none", onlyWeb.Unassigned)
	}
	if onlyWeb.Assignees == nil || onlyWeb.Unassigned == nil {
		t.Error("slices should be non-nil for clean JSON")
	}
}

func TestWeekOfMonthLabel(t *testing.T) {
	// t es el lunes que abre cada semana.
	tests := map[string]string{
		"2026-08-17": "3AUG",
		"2026-08-24": "4AUG",
		"2026-08-31": "1SEP",
		"2026-09-07": "2SEP",
		"2026-09-14": "3SEP",
		"2026-09-28": "1OCT",
		"2026-10-05": "2OCT",
	}
	for date, want := range tests {
		if got := WeekOfMonthLabel(mustDate(t, date)); got != want {
			t.Errorf("WeekOfMonthLabel(%s) = %q, want %q", date, got, want)
		}
	}
}

func TestFormatEstimate(t *testing.T) {
	tests := map[float64]string{1: "1d", 2: "2d", 0.5: "0.5d", 0.25: "0.25d", 1.5: "1.5d"}
	for in, want := range tests {
		if got := FormatEstimate(in); got != want {
			t.Errorf("FormatEstimate(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestIsOffDay(t *testing.T) {
	offdays := []OffDay{{Assignee: "@a", StartDate: monday, EndDate: "2026-09-15"}}
	// Fin de semana siempre.
	if !IsOffDay(offdays, "@a", mustDate(t, "2026-09-19")) {
		t.Error("Saturday should be off")
	}
	// Rango del assignee.
	if !IsOffDay(offdays, "@a", mustDate(t, "2026-09-15")) {
		t.Error("off-day in range should be off")
	}
	// Otra persona no se ve afectada.
	if IsOffDay(offdays, "@b", mustDate(t, "2026-09-15")) {
		t.Error("other assignee should work")
	}
	if IsOffDay(offdays, "@a", mustDate(t, "2026-09-16")) {
		t.Error("day after range should be working")
	}
}
