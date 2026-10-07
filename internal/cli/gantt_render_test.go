package cli

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

// These assertions are exact on purpose, not "contains". The Gantt render
// is a grid: if an offset moves one column, a label shifts or
// the axis loses a cell, the result still "looks like" a Gantt and that's why
// only a literal assert detects it. It covers the mutants of the offsets
// (labelW+1, +d), of the grid width and of the label truncation.

const (
	ganttLabelW  = 30
	ganttHeadOff = 2 // header and blank line before the ruler
)

func ganttLines(t *testing.T, s *model.Schedule, weeks int) []string {
	t.Helper()
	return strings.Split(renderGanttText(s, weeks), "\n")
}

func TestRenderGanttTextGrid(t *testing.T) {
	// Starts on a Monday: Mondays fall on d=0 and d=7, so the ruler ends up
	// exactly at 31+4+3+4 = 42 columns (trimmed by TrimRight) and the axis
	// at 31+14 = 45.
	lines := ganttLines(t, &model.Schedule{Start: "2026-09-14"}, 2)

	wantRuler := strings.Repeat(" ", ganttLabelW+1) + "3SEP" + "   " + "4SEP"
	if lines[ganttHeadOff] != wantRuler {
		t.Errorf("ruler = %q\nwant   %q", lines[ganttHeadOff], wantRuler)
	}

	wantAxis := strings.Repeat(" ", ganttLabelW+1) + "|------|------"
	if lines[ganttHeadOff+1] != wantAxis {
		t.Errorf("axis = %q\nwant  %q", lines[ganttHeadOff+1], wantAxis)
	}
	if got := len([]rune(lines[ganttHeadOff+1])); got != ganttLabelW+1+14 {
		t.Errorf("axis = %d cells, want %d", got, ganttLabelW+1+14)
	}
}

func TestRenderGanttTextAxisFollowsMonday(t *testing.T) {
	tests := []struct {
		start  string
		weeks  int
		pipes  int
		weekNb int
	}{
		{"2026-09-14", 2, 2, 14}, // Monday
		{"2026-09-15", 2, 2, 14}, // Tuesday: Mondays fall on d=5 and d=12
		{"2026-09-16", 1, 1, 7},  // Wednesday with a single week: Monday at d=4
		{"2026-09-14", 4, 4, 28},
	}
	for _, tt := range tests {
		t.Run(tt.start, func(t *testing.T) {
			axis := ganttLines(t, &model.Schedule{Start: tt.start}, tt.weeks)[ganttHeadOff+1]
			if got := strings.Count(axis, "|"); got != tt.pipes {
				t.Errorf("axis with %d Mondays, want %d (%q)", got, tt.pipes, axis)
			}
			if got := len([]rune(axis)) - ganttLabelW - 1; got != tt.weekNb {
				t.Errorf("axis = %d cells, want %d", got, tt.weekNb)
			}
		})
	}
}

// TestRenderGanttTextTruncatesLastLabel: only when the calendar does not
// start on a Monday does the last week label fall so far right that it does not
// fit in the ruler and gets truncated. It is the only case where `col+i < len(ruler)`
// decides something; without this assertion the guard is dead code from the test's point of view.
func TestRenderGanttTextTruncatesLastLabel(t *testing.T) {
	ruler := ganttLines(t, &model.Schedule{Start: "2026-09-15"}, 2)[ganttHeadOff]
	if !strings.HasSuffix(ruler, "1") {
		t.Errorf("expected the label truncated to 1OCT→1: %q", ruler)
	}
	if strings.Contains(ruler, "1OCT") {
		t.Errorf("the label should not fit whole: %q", ruler)
	}

	// With a Monday start the last one does fit: nothing is truncated.
	full := ganttLines(t, &model.Schedule{Start: "2026-09-14"}, 2)[ganttHeadOff]
	if !strings.HasSuffix(full, "4SEP") {
		t.Errorf("with a Monday start the label fits whole: %q", full)
	}
}

// TestRenderGanttTextBarPositions pins which cell each bar falls on: the
// first one starts at column 31 (30-wide label + space) and a task that
// starts on day 5 has 5 blank cells in front. A different day rounding
// (Hours()/24) moves the bar without changing anything visible at a glance.
func TestRenderGanttTextBarPositions(t *testing.T) {
	s := &model.Schedule{
		Start: "2026-09-14",
		Assignees: []model.AssigneeSchedule{{
			Assignee: "@a",
			Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 1, Title: "two days"}, Start: "2026-09-14", End: "2026-09-15", Estimate: 2},
				{Task: model.Task{ID: 2, Title: "day five"}, Start: "2026-09-19", End: "2026-09-19", Estimate: 1},
			},
		}},
	}
	lines := ganttLines(t, s, 2)

	var firstCells []int
	for _, line := range lines {
		if i := strings.Index(line, "█"); i >= 0 {
			firstCells = append(firstCells, i-ganttLabelW-1)
		}
	}
	if len(firstCells) != 2 {
		t.Fatalf("bars = %v, want 2 (lines: %q)", firstCells, lines)
	}
	if firstCells[0] != 0 {
		t.Errorf("first bar in cell %d, want 0", firstCells[0])
	}
	if firstCells[1] != 5 {
		t.Errorf("bar for Sep 19 in cell %d, want 5", firstCells[1])
	}
}

// TestRenderGanttTextClampsOutsideWindow: an entry outside the visible
// range paints no bar and does not overflow the row. It covers the right-hand
// and left-hand clamping (an unparseable date returns -1 in dayIndex).
func TestRenderGanttTextClampsOutsideWindow(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
	}{
		{"overflows to the right", "2026-10-01", "2026-10-05"},
		{"entirely before", "2020-01-01", "2020-01-02"},
		{"unparseable date", "not-a-date", "neither"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &model.Schedule{
				Start: "2026-09-14",
				Assignees: []model.AssigneeSchedule{{
					Assignee: "@a",
					Entries: []model.ScheduleEntry{
						{Task: model.Task{ID: 1, Title: "outside"}, Start: tt.start, End: tt.end, Estimate: 1},
					},
				}},
			}
			for _, line := range ganttLines(t, s, 2) {
				if strings.Contains(line, "█") {
					t.Errorf("an entry outside the window must not paint a bar: %q", line)
				}
			}
		})
	}
}

// TestRenderGanttTextMarksDefaultEstimate: the tilde "~" only appears when
// the estimate came from the default, not when the task brought it explicitly.
func TestRenderGanttTextMarksDefaultEstimate(t *testing.T) {
	entries := []model.ScheduleEntry{
		{Task: model.Task{ID: 1, Title: "default"}, Start: "2026-09-14", End: "2026-09-14", Estimate: 1, EstimateDefaulted: true},
		{Task: model.Task{ID: 2, Title: "explicit"}, Start: "2026-09-14", End: "2026-09-14", Estimate: 1},
	}
	out := renderGanttText(&model.Schedule{
		Start:     "2026-09-14",
		Assignees: []model.AssigneeSchedule{{Assignee: "@a", End: "2026-09-14", Entries: entries}},
	}, 1)

	if strings.Count(out, "~") != 1 {
		t.Errorf("expected 1 default mark, got %d:\n%s", strings.Count(out, "~"), out)
	}
}

func TestRenderGanttTextTasksWithoutOwner(t *testing.T) {
	s := &model.Schedule{
		Start:      "2026-09-14",
		Unassigned: []model.Task{{ID: 7, Title: "no owner"}, {ID: 8, Title: "other"}},
	}
	out := renderGanttText(s, 1)
	if !strings.Contains(out, "Unassigned (2):") {
		t.Errorf("the count is missing: %q", out)
	}
	// With no owner nothing is scheduled: there must be no bar and no per-person line.
	if strings.Contains(out, "█") || strings.Contains(out, "ends") {
		t.Errorf("an unassigned task should not be scheduled: %q", out)
	}
}

// TestRenderGanttTextEntrySpanningTheRightEdge: an entry that ends the day
// after the end of the window must be clamped to the last day, without writing
// outside the row or blowing up. It covers the `totalDays-1` of the clamp: if the
// edge were totalDays+1, the write would go outside the row.
func TestRenderGanttTextEntrySpanningTheRightEdge(t *testing.T) {
	const weeks = 2
	const totalDays = weeks * 7

	tests := []struct {
		name     string
		end      string
		wantBars int
	}{
		{"ends on the last visible day", "2026-09-27", totalDays},
		{"ends one day later", "2026-09-28", totalDays},
		{"ends two days later", "2026-09-29", totalDays},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &model.Schedule{
				Start: "2026-09-14",
				Assignees: []model.AssigneeSchedule{{
					Assignee: "@a",
					Entries: []model.ScheduleEntry{
						{Task: model.Task{ID: 1, Title: "at the edge"}, Start: "2026-09-14", End: tt.end, Estimate: 99},
					},
				}},
			}
			line := ganttLines(t, s, weeks)[ganttHeadOff+3]
			if got := strings.Count(line, "█"); got != tt.wantBars {
				t.Errorf("%d painted cells, want %d (%q)", got, tt.wantBars, line)
			}
		})
	}
}
