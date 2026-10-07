package model

import (
	"testing"
	"time"
)

// Mon is the reference Monday of several tests (2026-09-14).
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
	// Friday 2026-09-18.
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
	// The second falls on the next Monday, skipping Sat/Sun.
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
	// The whole working week is free -> starts the next Monday.
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

// A non-positive defaultEstimate is normalized to 1 day. The previous tests only
// passed 1 and 2, so the exact edge (0 and negative) was left uncovered.
func TestBuildScheduleDefaultEstimateNonPositive(t *testing.T) {
	tests := []struct {
		name       string
		defaultEst float64
	}{
		{"zero", 0},
		{"negative", -2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := BuildSchedule([]Task{task("@a", "todo", 0)}, nil, mustDate(t, monday), tt.defaultEst)
			e := s.Assignees[0].Entries[0]
			if e.Estimate != 1 {
				t.Errorf("estimate = %v, want 1 (normalized)", e.Estimate)
			}
			if !e.EstimateDefaulted {
				t.Error("EstimateDefaulted should be true")
			}
			if e.Start != monday || e.End != monday {
				t.Errorf("range = %s→%s, want %s→%s (1 day = the whole Monday)", e.Start, e.End, monday, monday)
			}
		})
	}
}

// An estimate below half a minute consumes NO day at all: nothing is
// reserved, so the entry keeps no start day (FormatDate of the zero value)
// and the calendar does not advance. It pins that floor, which is what separates `remaining > 0`
// from `remaining >= 0`.
//
// Before, the floor was 1e-9 DAYS and the comparison was floating point, so the
// edge was unreachable: no estimate lands exactly on 1e-9. Now the
// arithmetic is in whole minutes, and 0 is a real value.
func TestBuildScheduleEstimateAtTheFloor(t *testing.T) {
	s := BuildSchedule([]Task{task("@a", "todo", 1e-9)}, nil, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries[0]

	if e.Estimate != 1e-9 {
		t.Errorf("estimate = %v, want 1e-9 (an explicit estimate is not sanitized)", e.Estimate)
	}
	if e.EstimateDefaulted {
		t.Error("1e-9 is not 0: EstimateDefaulted should not be set")
	}
	if e.End != monday {
		t.Errorf("end = %s, want %s (consumes no day)", e.End, monday)
	}
	if e.Start == monday {
		t.Error("start = Monday: an epsilon estimate should not assign a start day")
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

	// Filtering to "web" must not trim its date: it stays on Tuesday, even though
	// the "api" task from Monday is not visible in the view.
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
	// t is the Monday that opens each week.
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
	// Weekend, always.
	if !IsOffDay(offdays, "@a", mustDate(t, "2026-09-19")) {
		t.Error("Saturday should be off")
	}
	// The assignee's range.
	if !IsOffDay(offdays, "@a", mustDate(t, "2026-09-15")) {
		t.Error("off-day in range should be off")
	}
	// Another person is unaffected.
	if IsOffDay(offdays, "@b", mustDate(t, "2026-09-15")) {
		t.Error("other assignee should work")
	}
	if IsOffDay(offdays, "@a", mustDate(t, "2026-09-16")) {
		t.Error("day after range should be working")
	}
}

// The full-day edge.
//
// The split packs tasks: two half-day tasks fit in the same day, and once
// it is full the next one jumps to the next working day. That jump is a `free == 0`
// -- an equality comparison, not an epsilon -- and it is hit by any
// estimate that fills the whole day.
//
// These two cases are the opposite sides of the same edge, and they are what
// separate `free == 0` from `free != 0`: if the jump did not happen, both one-day
// tasks would start on the same day.
func TestBuildSchedulePacksThenJumps(t *testing.T) {
	t.Run("two halves fit in the same day", func(t *testing.T) {
		s := BuildSchedule([]Task{
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
		}, nil, mustDate(t, monday), 1)

		e := s.Assignees[0].Entries
		if len(e) != 2 {
			t.Fatalf("entries = %d, want 2", len(e))
		}
		if e[0].Start != monday || e[1].Start != monday {
			t.Errorf("starts = %s, %s; want %s both: half day + half day fills the day",
				e[0].Start, e[1].Start, monday)
		}
	})

	t.Run("when the day is full, the next one jumps", func(t *testing.T) {
		s := BuildSchedule([]Task{
			task("@a", "todo", 1.0),
			task("@a", "todo", 1.0),
		}, nil, mustDate(t, monday), 1)

		e := s.Assignees[0].Entries
		if len(e) != 2 {
			t.Fatalf("entries = %d, want 2", len(e))
		}
		if e[0].Start != monday {
			t.Errorf("the first starts on %s, want %s", e[0].Start, monday)
		}
		if e[1].Start == monday {
			t.Errorf("the second also starts on %s: the day was already full and had to jump",
				e[1].Start)
		}
		if e[1].Start != "2026-09-15" {
			t.Errorf("the second starts on %s, want 2026-09-15 (Tuesday)", e[1].Start)
		}
	})

	t.Run("three halves do not fit in one day", func(t *testing.T) {
		s := BuildSchedule([]Task{
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
		}, nil, mustDate(t, monday), 1)

		e := s.Assignees[0].Entries
		// Two halves fill the day; the third no longer fits and jumps. It is the
		// same edge from the other side of the fill.
		if e[0].Start != monday || e[1].Start != monday {
			t.Errorf("starts = %s, %s; want %s both", e[0].Start, e[1].Start, monday)
		}
		for i := 2; i < len(e); i++ {
			if e[i].Start == monday {
				t.Errorf("task %d starts on %s: the day was already full", i, e[i].Start)
			}
		}
		if e[2].Start != "2026-09-15" {
			t.Errorf("the third starts on %s, want 2026-09-15", e[2].Start)
		}
	})
}

// The zero floor of the estimate: below half a minute no bar is painted,
// but the task STILL is on the calendar and the rest of the queue does not go out of sync.
func TestBuildScheduleRoundedFloorDoesNotShiftTheQueue(t *testing.T) {
	s := BuildSchedule([]Task{
		task("@a", "todo", 0.0001), // rounds to 0 minutes
		task("@a", "todo", 1.0),
	}, nil, mustDate(t, monday), 1)

	e := s.Assignees[0].Entries
	if len(e) != 2 {
		t.Fatalf("entries = %d, want 2", len(e))
	}
	if e[0].Start == monday {
		t.Error("the 0-minute task has a start day: it should not paint a bar")
	}
	// And the next one stays on Monday: the 0-minute task did not consume a day.
	if e[1].Start != monday {
		t.Errorf("the second starts on %s, want %s: a 0-minute task consumes no day",
			e[1].Start, monday)
	}
}

// estimateMinutes is the pure function of the refactor, and its rounding is a decision
// with visible consequences: below half a minute there is no bar.
func TestEstimateMinutesRoundsToTheMinute(t *testing.T) {
	cases := []struct {
		est  float64
		want int
	}{
		{0, 0},
		{0.0001, 0},
		{1.0 / (3 * minutesPerDay), 0}, // a third of a minute, rounds to 0
		{1.0 / (2 * minutesPerDay), 1}, // half a minute: round() goes up
		{0.5, 720},
		{1, minutesPerDay},
		{1.5, 2160},
		{-1, -minutesPerDay},
	}
	for _, c := range cases {
		if got := estimateMinutes(c.est); got != c.want {
			t.Errorf("estimateMinutes(%v) = %d, want %d", c.est, got, c.want)
		}
	}
}

// divRound rounds to the nearest integer, and the rounding has an edge: the
// exact half.
//
// It is the number of FULL DAYS of an estimate. The count decides how many
// full days are consumed by the loop and what the remainder is, so a
// different rounding gives a different split -- and it is a split visible in the
// calendar dates, not in an internal detail.
//
// The three edge cases: below the half (rounds to 0), exactly at the
// half (rounds to 1, which is what `+ b/2` does), and above (1).
func TestDivRoundAtTheHalfBoundary(t *testing.T) {
	const day = minutesPerDay

	cases := []struct {
		name string
		a, b int
		want int
	}{
		{"zero", 0, day, 0},
		{"well below half a workday", 1, day, 0},
		{"almost half a workday", day/2 - 1, day, 0},
		{"EXACTLY half a workday", day / 2, day, 1},
		{"almost one workday", day - 1, day, 1},
		{"one workday", day, day, 1},
		{"one workday and one minute", day + 1, day, 1},
		{"exactly one and a half", day + day/2, day, 2},
		{"two workdays", 2 * day, day, 2},

		{"exact half workday, in days", 0, 1, 0},
		{"three and a half days", 3*day + day/2, day, 4},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := divRound(c.a, c.b); got != c.want {
				t.Errorf("divRound(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}

// The same edge, seen through the calendar -- and in the NEXT task,
// which is where the rounding really shows.
//
// The remainder a split leaves is the capacity the next task inherits. A
// rounding that leaves too much behind gives the next task more room than it
// deserves, and then it starts a day earlier. It is the difference between
// correct rounding and one that truncates, and it shows in both dates.
func TestRoundingShowsInTheNextTask(t *testing.T) {
	// Two and a half workdays, and then a whole workday.
	//
	// With rounding to the nearest integer: 2.5 workdays are 2 full days plus
	// half a workday left over. The next task inherits that half workday, it does not
	// fit as a whole, and has to move to the next day.
	//
	// With a truncating rounding: the 2.5 workdays are 2 full days and the
	// leftover half day is not discounted. The next task inherits a whole free
	// day, it fits, and starts the SAME day.
	cases := []struct {
		name            string
		first           float64
		second          float64
		wantSecondStart string
		wantSecondEnd   string
	}{
		{"2.5 workdays then 1", 2.5, 1.0, "2026-09-16", "2026-09-17"},
		{"3.5 workdays then 1", 3.5, 1.0, "2026-09-17", "2026-09-18"},
		{"2.5 workdays then half", 2.5, 0.5, "2026-09-16", "2026-09-16"},
		{"1.5 workdays then 1", 1.5, 1.0, "2026-09-15", "2026-09-16"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := BuildSchedule([]Task{
				task("@a", "todo", c.first),
				task("@a", "todo", c.second),
			}, nil, mustDate(t, monday), 1)

			e := s.Assignees[0].Entries
			if len(e) != 2 {
				t.Fatalf("entries = %d, want 2", len(e))
			}
			if e[1].Start != c.wantSecondStart {
				t.Errorf("the second starts on %s, want %s (the first ends on %s)",
					e[1].Start, c.wantSecondStart, e[0].End)
			}
			if e[1].End != c.wantSecondEnd {
				t.Errorf("the second ends on %s, want %s", e[1].End, c.wantSecondEnd)
			}
		})
	}

	// The exact-half case, which is what separates "rounding" from "truncating".
	// 1.5 workdays are 1 full day plus half a workday left over, so the
	// next one inherits half a workday and fits on the same day.
	//
	// If the rounding truncated, the 1.5 would be a whole day with no remainder,
	// the next would inherit a whole day, and both would end on the same day instead
	// of splitting into Monday and Tuesday. It is the case where rounding shows.
	s := BuildSchedule([]Task{
		task("@a", "todo", 1.5),
		task("@a", "todo", 0.5),
	}, nil, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries
	if len(e) != 2 {
		t.Fatalf("entries = %d, want 2", len(e))
	}
	if e[0].Start != "2026-09-14" || e[0].End != "2026-09-15" {
		t.Errorf("the first is %s→%s, want Monday→Tuesday", e[0].Start, e[0].End)
	}
	// The second one, half a workday, fits in the rest of Tuesday: same day as the end
	// of the first.
	if e[1].Start != "2026-09-15" || e[1].End != "2026-09-15" {
		t.Errorf("the second is %s→%s, want Tuesday→Tuesday: the free half workday "+
			"has to fit in the same day the first ends", e[1].Start, e[1].End)
	}
}
