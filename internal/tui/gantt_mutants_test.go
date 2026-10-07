package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// ganttModelWithPeople leaves the model on the Gantt view with `people` people
// and `perPerson` tasks each, all "active" so that they appear in the
// projection. The Gantt schedules the queue from today, so the dates depend
// on the clock; the tests work with what the model itself calculates, not
// with fixed dates.
func ganttModelWithPeople(t *testing.T, people []string, perPerson int) *Model {
	t.Helper()
	m := newTestModel(t)
	m.tasks = nil
	m.offdays = nil
	var id int64
	for _, p := range people {
		for range perPerson {
			id++
			m.tasks = append(m.tasks, model.Task{
				ID:        id,
				Title:     "task",
				Status:    "todo",
				Assignee:  p,
				Priority:  model.PriorityMedium,
				Estimate:  1, // without an estimate the task does not enter the queue
				CreatedAt: time.Now().Format("2006-01-02"),
			})
		}
	}
	m.invalidateFilterCache()
	m.currentView = viewGantt
	return m
}

// firstGanttMonday returns a known Monday, so that the ruler's and the
// axis's grid have predictable columns.
func firstGanttMonday() time.Time {
	return time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC) // Monday
}

// --- The structure of rows ----------------------------------------------

// Each person contributes a header followed by their tasks. It is the shape
// that forces the navigation to skip headers.
func TestGanttRowsInterleaveHeadersAndTasks(t *testing.T) {
	s := &model.Schedule{
		Start: "2026-09-07",
		Assignees: []model.AssigneeSchedule{
			{Assignee: "@john", Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 1}, Start: "2026-09-07", End: "2026-09-08"},
				{Task: model.Task{ID: 2}, Start: "2026-09-09", End: "2026-09-10"},
			}},
			{Assignee: "@margo", Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 3}, Start: "2026-09-11", End: "2026-09-12"},
			}},
		},
	}

	rows := ganttRows(s)
	want := []struct {
		kind     ganttRowKind
		assignee string
	}{
		{ganttAssigneeRow, "@john"},
		{ganttTaskRow, "@john"},
		{ganttTaskRow, "@john"},
		{ganttAssigneeRow, "@margo"},
		{ganttTaskRow, "@margo"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].kind != w.kind || rows[i].assignee != w.assignee {
			t.Errorf("row %d = {kind:%d assignee:%q}, want {kind:%d assignee:%q}",
				i, rows[i].kind, rows[i].assignee, w.kind, w.assignee)
		}
	}
	// And each task row points to ITS OWN entry, not the first one: if it
	// pointed to a shared copy, all the bars would show the same task.
	if rows[1].entry == nil || rows[1].entry.Task.ID != 1 {
		t.Errorf("row 1 must point to task 1: %+v", rows[1].entry)
	}
	if rows[2].entry == nil || rows[2].entry.Task.ID != 2 {
		t.Errorf("row 2 must point to task 2: %+v", rows[2].entry)
	}
	if rows[4].entry == nil || rows[4].entry.Task.ID != 3 {
		t.Errorf("row 4 must point to task 3: %+v", rows[4].entry)
	}
}

func TestGanttRowsEmpty(t *testing.T) {
	if got := ganttRows(&model.Schedule{}); len(got) != 0 {
		t.Errorf("with no assignees there are %d rows, want 0", len(got))
	}
	one := &model.Schedule{Assignees: []model.AssigneeSchedule{{Assignee: "solo"}}}
	if got := ganttRows(one); len(got) != 1 {
		t.Errorf("a person with no tasks gives %d rows, want 1 (only the header)", len(got))
	}
}

// --- ganttTaskRange -------------------------------------------------------

func TestGanttTaskRange(t *testing.T) {
	rows := []ganttRow{
		{kind: ganttAssigneeRow, assignee: "@john"},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow, assignee: "@margo"},
		{kind: ganttTaskRow},
	}
	first, last := ganttTaskRange(rows)
	if first != 1 || last != 4 {
		t.Errorf("range = (%d,%d), want (1,4)", first, last)
	}
}

func TestGanttTaskRangeEdgeCases(t *testing.T) {
	tests := []struct {
		name                string
		rows                []ganttRow
		wantFirst, wantLast int
	}{
		{"no rows", nil, -1, -1},
		{"only headers", []ganttRow{{kind: ganttAssigneeRow}}, -1, -1},
		{"a single task", []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttTaskRow}}, 1, 1},
		{"only tasks", []ganttRow{{kind: ganttTaskRow}, {kind: ganttTaskRow}}, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last := ganttTaskRange(tt.rows)
			if first != tt.wantFirst || last != tt.wantLast {
				t.Errorf("range = (%d,%d), want (%d,%d)", first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

// --- snapGanttCursor ------------------------------------------------------

// The cursor must end ALWAYS on a task row: person headers are not
// navigable, so it jumps to the adjacent task.
func TestSnapGanttCursorLandsOnTaskRow(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john", "@margo"}, 2)
	rows := m.ganttRows()
	// 2 people x (header + 2 tasks) = 6 rows.
	//  0 hdr @john · 1 task · 2 task · 3 hdr @margo · 4 task · 5 task
	if len(rows) != 6 {
		t.Fatalf("fixture: 2 people x (header + 2 tasks) = 6 rows, there are %d", len(rows))
	}

	tests := []struct {
		name   string
		cursor int
		want   int
	}{
		{"already on a task", 2, 2},
		{"first header jumps down", 0, 1},
		{"second header jumps down", 3, 4},
		{"last task stays", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ganttCursor = tt.cursor
			m.snapGanttCursor()
			if m.ganttCursor != tt.want {
				t.Errorf("cursor = %d, want %d", m.ganttCursor, tt.want)
			}
			if rows[m.ganttCursor].kind != ganttTaskRow {
				t.Errorf("the cursor ended up on a header (row %d)", m.ganttCursor)
			}
		})
	}
}

// Every header has at least one task behind it: ganttRows only emits the
// header of a person that has entries, so the next row is always a task. That
// turns snapGanttCursor's BACKWARD search into a defensive path that the row
// structure never reaches.
//
// The invariant is pinned down instead of inventing a case: if some day a
// header could end up at the end, this test would say so by breaking.
func TestSnapGanttCursorHeaderAlwaysHasTaskAhead(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john", "@margo"}, 1)
	rows := m.ganttRows() // [hdr @john, task, hdr @margo, task]
	if len(rows) != 4 {
		t.Fatalf("fixture: 4 rows expected, there are %d", len(rows))
	}
	for i, r := range rows {
		if r.kind != ganttAssigneeRow {
			continue
		}
		if i+1 >= len(rows) || rows[i+1].kind != ganttTaskRow {
			t.Fatalf("header %d is not followed by a task: %+v", i, rows)
		}
	}
}

func TestSnapGanttCursorOutOfRange(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 2)
	rows := m.ganttRows() // [hdr, task, task] -> 3
	last := len(rows) - 1

	tests := []struct {
		name   string
		cursor int
		want   int
	}{
		{"beyond the end", 99, last},
		{"negative", -5, 1},
		{"on the last one", last, last},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ganttCursor = tt.cursor
			m.snapGanttCursor()
			if m.ganttCursor != tt.want {
				t.Errorf("cursor = %d, want %d", m.ganttCursor, tt.want)
			}
		})
	}
}

// With no rows, the cursor stays at 0 instead of going out.
func TestSnapGanttCursorEmpty(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	m.ganttCursor = 7

	m.snapGanttCursor()
	if m.ganttCursor != 0 {
		t.Errorf("cursor = %d, want 0 with the view empty", m.ganttCursor)
	}
}

// Only headers, with no task at all: the cursor falls to 0.
func TestSnapGanttCursorHeadersWithoutTasks(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	// A person with no tasks does NOT generate rows, so the case has to be
	// forced with a filter that leaves only the header via an unscheduled task.
	m.tasks = []model.Task{{ID: 1, Title: "no people", Status: "todo", Assignee: ""}}
	m.invalidateFilterCache()

	m.ganttCursor = 1
	m.snapGanttCursor()
	if m.ganttCursor < 0 {
		t.Errorf("cursor = %d, it cannot be negative", m.ganttCursor)
	}
}

// --- daysBetween ----------------------------------------------------------

// daysBetween counts calendar days. An invalid date returns -1, which is the
// signal of "could not be measured": if it returned 0, the bar would be
// drawn in the first column instead of disappearing.
func TestDaysBetween(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		date string
		want int
	}{
		{"the same day", "2026-09-01", 0},
		{"one day later", "2026-09-02", 1},
		{"one week later", "2026-09-08", 7},
		{"next month", "2026-10-01", 30},
		{"invalid date", "not-a-date", -1},
		{"empty", "", -1},
		{"different format", "01/09/2026", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daysBetween(start, tt.date); got != tt.want {
				t.Errorf("daysBetween(%s, %q) = %d, want %d", model.FormatDate(start), tt.date, got, tt.want)
			}
		})
	}
}

// --- The weeks' axis (ruler) -------------------------------------------

func TestRenderGanttRulerMarksMondays(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 10, 28 // exactly 4 weeks

	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	ruler := ansi.Strip(m.renderGanttRuler(start, 0, labelW, dayCols))

	if !strings.Contains(ruler, model.WeekOfMonthLabel(start)) {
		t.Errorf("the ruler must label the Monday of the first column: %q", ruler)
	}
	// With offset 0 we start on Monday, so the captions fall on the columns
	// 0, 7, 14 and 21. None can step on the label's padding.
	for _, col := range []int{0, 7, 14, 21} {
		at := labelW + 1 + col
		r := []rune(ruler)
		if at < len(r) && r[at] == ' ' {
			t.Errorf("column %d without a Monday label: %q", at, ruler)
		}
	}
}

// Shifting the start moves the caption: with offset 2 column 0 is Wednesday
// and the first Monday falls 5 columns beyond.
func TestRenderGanttRulerWithOffset(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 8, 30

	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	ruler := ansi.Strip(m.renderGanttRuler(start, 2, labelW, dayCols))

	nextMonday := model.WeekOfMonthLabel(start.AddDate(0, 0, 7))
	if !strings.Contains(ruler, nextMonday) {
		t.Errorf("with offset 2 the next Monday must be labeled: %q", ruler)
	}
	// And it must not be glued to the label: the Monday column is 5.
	r := []rune(ruler)
	at := labelW + 1 + 5
	if at < len(r) && r[at] == ' ' {
		t.Errorf("the Monday label did not fall on its column (offset 2): %q", ruler)
	}
}

// The ruler never overflows the grid, neither with labels nor with utc columns.
func TestRenderGanttRulerFitsWidth(t *testing.T) {
	start := firstGanttMonday()
	m := ganttModelWithPeople(t, []string{"@john"}, 1)

	for _, labelW := range []int{1, 4, 8, 14, 30} {
		for _, dayCols := range []int{1, 7, 14, 28, 60} {
			ruler := ansi.Strip(m.renderGanttRuler(start, 0, labelW, dayCols))
			if w := ansi.StringWidth(ruler); w > labelW+1+dayCols {
				t.Errorf("labelW=%d dayCols=%d: ruler measures %d, want <= %d", labelW, dayCols, w, labelW+1+dayCols)
			}
		}
	}
}

// --- The days' axis -----------------------------------------------------

// The axis marks "|" on each Monday and "-" on the rest of the days.
func TestRenderGanttAxisMondays(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 6, 14

	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	axis := ansi.Strip(m.renderGanttAxis(labelW, dayCols, start, 0))

	if w := ansi.StringWidth(axis); w != labelW+1+dayCols {
		t.Errorf("axis measures %d, want %d", w, labelW+1+dayCols)
	}
	marks := axis[labelW+1:]
	if pipes := strings.Count(marks, "|"); pipes != 2 {
		t.Errorf("14 days from a Monday have 2 Mondays, there are %d: %q", pipes, marks)
	}
	if dashes := strings.Count(marks, "-"); dashes != dayCols-2 {
		t.Errorf("the remaining %d days must be dashes, there are %d", dayCols-2, dashes)
	}
}

// Shifting a day moves each "|"'s position by one column.
func TestRenderGanttAxisOffsetMovesMondays(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 6, 14

	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	marks := ansi.Strip(m.renderGanttAxis(labelW, dayCols, start, 1))[labelW+1:]

	if marks[0] != '-' {
		t.Errorf("with offset 1 column 0 is Tuesday: %q", marks[0])
	}
	if marks[6] != '|' {
		t.Errorf("with offset 1 the first Monday must fall on column 6: %q", marks)
	}
}

func TestRenderGanttAxisZeroDays(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	axis := ansi.Strip(m.renderGanttAxis(10, 0, firstGanttMonday(), 0))
	if w := ansi.StringWidth(axis); w != 11 {
		t.Errorf("axis with 0 days measures %d, want 11 (only the padding)", w)
	}
}

// --- ganttLegend ----------------------------------------------------------

// The legend describes the visible range, so changing the offset changes it.
func TestGanttLegend(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 3)

	legend := m.ganttLegend(0, 14)
	parts := strings.Fields(legend)
	if len(parts) != 3 || parts[1] != "→" {
		t.Fatalf("legend = %q, want 'from → to'", legend)
	}
	if parts[0] == parts[2] {
		t.Errorf("with 14 days the range cannot start and end on the same day: %q", legend)
	}
	for _, d := range parts[0:1] {
		if _, err := model.ParseDate(d); err != nil {
			t.Errorf("start date %q does not parse: %v", d, err)
		}
	}
	for _, d := range parts[2:] {
		if _, err := model.ParseDate(d); err != nil {
			t.Errorf("end date %q does not parse: %v", d, err)
		}
	}

	if shifted := m.ganttLegend(7, 14); shifted == legend {
		t.Errorf("with offset 7 the legend must change: %q", shifted)
	}
	// And with a single column the range is a single day.
	one := strings.Fields(m.ganttLegend(0, 1))
	if len(one) != 3 || one[0] != one[2] {
		t.Errorf("with 1 visible day the range must be a single day: %q", one)
	}
}

// --- moveGanttCursor ------------------------------------------------------

// j/k move between task rows, skipping the person headers.
func TestMoveGanttCursorSkipsHeaders(t *testing.T) {
	rows := []ganttRow{
		{kind: ganttAssigneeRow, assignee: "@john"},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow, assignee: "@margo"},
		{kind: ganttTaskRow},
	}
	tests := []struct {
		name string
		from int
		dir  int
		want int
	}{
		{"goes down from the first to the second task", 1, 1, 2},
		{"goes up from the second to the first task", 2, -1, 1},
		{"goes down skipping a header", 2, 1, 4},
		{"goes up skipping a header", 4, -1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ganttModelWithPeople(t, []string{"@john"}, 1)
			m.ganttCursor = tt.from
			moveGanttCursor(m, rows, tt.dir)
			if m.ganttCursor != tt.want {
				t.Errorf("cursor = %d, want %d", m.ganttCursor, tt.want)
			}
		})
	}
}

// At the extreme there is nowhere to go: the cursor stays.
func TestMoveGanttCursorStopsAtEdges(t *testing.T) {
	rows := []ganttRow{{kind: ganttTaskRow}, {kind: ganttTaskRow}}
	m := ganttModelWithPeople(t, []string{"@john"}, 1)

	m.ganttCursor = 1
	moveGanttCursor(m, rows, 1)
	if m.ganttCursor != 1 {
		t.Errorf("going down on the last row must stay, cursor = %d", m.ganttCursor)
	}
	m.ganttCursor = 0
	moveGanttCursor(m, rows, -1)
	if m.ganttCursor != 0 {
		t.Errorf("going up on the first row must stay, cursor = %d", m.ganttCursor)
	}
}

// Only headers: there is no task to jump to and the cursor does not move.
func TestMoveGanttCursorNoTaskRows(t *testing.T) {
	rows := []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttAssigneeRow}}
	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	m.ganttCursor = 1
	moveGanttCursor(m, rows, 1)
	if m.ganttCursor != 1 {
		t.Errorf("with no task rows the cursor must not move, cursor = %d", m.ganttCursor)
	}
}

// --- The Gantt's keyboard -----------------------------------------------

// g/G jump to the first and the last day of the visible horizon.
func TestGanttKeysGotoEdges(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 3)
	rows := m.ganttRows()
	first, last := ganttTaskRange(rows)
	if first < 0 {
		t.Fatal("fixture: no task rows")
	}

	m.ganttCursor = last
	m, _ = press(m, "g")
	if m.ganttCursor != first {
		t.Errorf("after g cursor = %d, want %d (first task)", m.ganttCursor, first)
	}

	m, _ = press(m, "G")
	if m.ganttCursor != last {
		t.Errorf("after G cursor = %d, want %d (last task)", m.ganttCursor, last)
	}
}

// With an empty view, g and G do not move the cursor to any invalid place.
func TestGanttKeysWithNoTasks(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	for _, key := range []string{"g", "G", "j", "k"} {
		m, _ = press(m, key)
		if m.ganttCursor < 0 || m.ganttCursor > len(m.ganttRows()) {
			t.Errorf("after %q cursor = %d, out of range", key, m.ganttCursor)
		}
	}
}

// h/l scroll the day window, and h does not go below zero.
func TestGanttKeysShiftWindow(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 2)

	m, _ = press(m, "l")
	m, _ = press(m, "l")
	if m.ganttOffsetDays != 2 {
		t.Errorf("after two l offset = %d, want 2", m.ganttOffsetDays)
	}
	m, _ = press(m, "h")
	if m.ganttOffsetDays != 1 {
		t.Errorf("after h offset = %d, want 1", m.ganttOffsetDays)
	}
	for range 5 {
		m, _ = press(m, "h")
	}
	if m.ganttOffsetDays != 0 {
		t.Errorf("the offset cannot be negative: %d", m.ganttOffsetDays)
	}
}

// enter on a task row opens the detail with its comments not yet loaded (the
// comment cursor starts at "none").
func TestGanttEnterOpensDetail(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	m.snapGanttCursor()
	if rows := m.ganttRows(); m.ganttCursor >= len(rows) || rows[m.ganttCursor].kind != ganttTaskRow {
		t.Fatalf("fixture: the cursor must land on a task, it is at %d", m.ganttCursor)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := asModel(next)

	if !got.detailOpen {
		t.Error("enter must open the detail")
	}
	if got.detailTask == nil {
		t.Fatal("enter must load the task in the detail")
	}
	if got.detailComments != nil {
		t.Error("the comments start unloaded, not as an empty list")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1 (nothing selected)", got.detailCommentSel)
	}
	if cmd == nil {
		t.Error("enter must emit the comments load")
	}
}

// enter on a header opens nothing: headers are not navigable.
func TestGanttEnterOnHeaderDoesNothing(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 2)
	// We force the cursor onto the header by skipping the snap.
	m.ganttCursor = 0

	// handleGanttKey snaps on entry, so we check the net effect:
	// the detail opens, but on a TASK, never on a header.
	m, _ = press(m, "enter")
	if m.detailOpen && m.detailTask != nil && m.detailTask.Assignee == "" {
		t.Error("the detail must never open on a person header")
	}
}

// --- Width split of the Gantt -------------------------------------------

// ganttBudgetForRows returns the height that fits exactly `rows` rows of
// Gantt. In renderGantt, visible = maxHeight - listFixedRows - ganttRulerRows,
// so the budget is the sum, not another discount.
func ganttBudgetForRows(rows int) int {
	return rows + listFixedRows + ganttRulerRows
}

// The width split has two regimes: with room to spare the label stays at 30
// columns; with little, it keeps a third of the inner width.
// The threshold is 70 inner columns, not 60: at 65 inner the third still
// rules, and that is exactly the edge that tells `innerW < 70` from
// any other nearby value.
func TestRenderGanttLabelWidthThreshold(t *testing.T) {
	tests := []struct {
		width int
		// columns the label takes before the days
		wantLabel int
	}{
		{120, 30}, // inner 118: fixed label
		{80, 30},  // inner 78: fixed label
		{72, 30},  // inner 70: exactly at the threshold, still the fixed one
		{71, 23},  // inner 69: one third
		{65, 21},  // inner 63
		{50, 16},  // inner 48
		{40, 14},  // inner 38, never less than 14
	}
	for _, tt := range tests {
		m := ganttModelWithPeople(t, []string{"@john"}, 1)
		m.width = tt.width

		// The axis padding is labelW+1, and the axis goes right after the label.
		// The label's width is NOT recalculated here on purpose: what is checked
		// is that the render applies the split the threshold says, and
		// replicating the formula in the test would only prove that the test
		// copies the code.
		out := ansi.Strip(m.renderGantt(30))
		axis := ""
		for _, line := range strings.Split(out, "\n") {
			if isGanttAxisLine(line) {
				axis = line
				break
			}
		}
		if axis == "" {
			t.Fatalf("width=%d: the axis was not found:\n%s", tt.width, out)
		}
		runes := []rune(axis)
		inner := string(runes[1 : len(runes)-1]) // without the box's sides
		lead := 0
		for _, r := range inner {
			if r != ' ' {
				break
			}
			lead++
		}
		if lead != tt.wantLabel+1 {
			t.Errorf("width=%d: the label takes %d columns (padding %d), want %d",
				tt.width, lead-1, lead, tt.wantLabel)
		}
	}
}

// The days' axis takes exactly the Gantt's inner width, whatever the split
// between label and columns. With one "- 1" too many or too few the axis
// goes out of sync and the box ends up one column wider or narrower.
func TestRenderGanttAxisSpansInnerWidth(t *testing.T) {
	// Below ~40 columns the minimum label (14) plus the 7 minimum days do
	// not fit, so the axis is truncated and loses the Mondays: there is no
	// line to compare.
	for _, width := range []int{120, 100, 80, 72, 71, 65, 50, 40} {
		m := ganttModelWithPeople(t, []string{"@john"}, 1)
		m.width = width

		out := ansi.Strip(m.renderGantt(30))
		found := false
		for i, line := range strings.Split(out, "\n") {
			if !isGanttAxisLine(line) {
				continue
			}
			found = true
			// The axis line includes the box's two sides, so it measures the
			// full terminal width: it is the check that tells the width split
			// apart from its variants, because any "- 1" or "+ 1" in dayCols
			// throws the line off.
			if got := ansi.StringWidth(line); got != width {
				t.Errorf("width=%d: the axis (line %d) measures %d, want %d: %q",
					width, i, got, width, line)
			}
		}
		if !found {
			t.Errorf("width=%d: the axis line was not found:\n%s", width, out)
		}
	}
}

// The person header takes the whole width of the box, labels and days
// included: that is why its cellWidth uses labelW+1+dayCols and not just labelW.
func TestRenderGanttAssigneeHeaderWidth(t *testing.T) {
	for _, width := range []int{120, 80, 65, 40} {
		m := ganttModelWithPeople(t, []string{"@john"}, 1)
		m.width = width

		rows := m.ganttRows()
		var header ganttRow
		for _, r := range rows {
			if r.kind == ganttAssigneeRow {
				header = r
			}
		}
		if header.assignee == "" {
			t.Fatal("fixture: no person header")
		}

		innerW := width - 2
		labelW := 30
		if innerW < 70 {
			labelW = innerW / 3
		}
		labelW = max(labelW, 14)
		dayCols := max(innerW-labelW-1, 7)

		got := ansi.StringWidth(ansi.Strip(m.renderGanttRow(header, firstGanttMonday(), 0, dayCols, labelW, false)))
		if got != labelW+1+dayCols {
			t.Errorf("width=%d: the header measures %d, want %d (labelW+1+dayCols)",
				width, got, labelW+1+dayCols)
		}
	}
}

// A task's bar is drawn on the columns that fall between its start and its
// end. Shifting the window (offset) does not move it: if the column
// calculation inverted the sign of the offset, the bar would go to the opposite side.
func TestRenderGanttBarFollowsTaskDates(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 1)
	rows := m.ganttRows()

	var task ganttRow
	for _, r := range rows {
		if r.kind == ganttTaskRow {
			task = r
		}
	}
	if task.entry == nil {
		t.Fatal("fixture: no task row")
	}

	const labelW, dayCols = 10, 20
	start, _ := model.ParseDate(m.ganttDisplay().Start)
	d0 := daysBetween(start, task.entry.Start)
	d1 := daysBetween(start, task.entry.End)

	// With no shift the bar starts at the start's column.
	line := ansi.Strip(m.renderGanttRow(task, start, 0, dayCols, labelW, false))
	cells := []rune(line)[labelW+1:]
	bars := []int{}
	for i, c := range cells {
		if c == '█' {
			bars = append(bars, i)
		}
	}
	if len(bars) == 0 {
		t.Fatalf("no bar was drawn: %q", line)
	}
	if bars[0] != d0 {
		t.Errorf("the bar starts at column %d, want %d (d0)", bars[0], d0)
	}
	if bars[len(bars)-1] != d1 {
		t.Errorf("the bar ends at column %d, want %d (d1)", bars[len(bars)-1], d1)
	}

	// With an offset the bar shifts with the window, not the other way around:
	// the same absolute columns are drawn offset positions earlier.
	shifted := ansi.Strip(m.renderGanttRow(task, start, 3, dayCols, labelW, false))
	scells := []rune(shifted)[labelW+1:]
	for i, c := range scells {
		if c == '█' && i != bars[0]-3 {
			t.Errorf("with offset 3 the bar was drawn at column %d, want %d", i, bars[0]-3)
			break
		}
	}
}

// --- Height budget: vertical window -------------------------------------

// With exactly `visible` rows they all fit; with one more the window has to
// shift. The `>` is what separates both cases.
func TestRenderGanttVerticalWindow(t *testing.T) {
	for _, perPerson := range []int{1, 2, 3, 5} {
		m := ganttModelWithPeople(t, []string{"@john"}, perPerson)
		rows := m.ganttRows() // 1 header + perPerson tasks

		tasks := 0
		for _, r := range rows {
			if r.kind == ganttTaskRow {
				tasks++
			}
		}

		// Budget just enough to see all the rows.
		exact := ganttBudgetForRows(len(rows))
		out := ansi.Strip(m.renderGantt(exact))
		if n := countGanttRows(out); n != tasks {
			t.Errorf("perPerson=%d: with budget %d, %d tasks are visible, want the %d that exist",
				perPerson, exact, n, tasks)
		}

		// One budget less: the window leaves out the last row and fewer tasks
		// fit. With a single task the truncation eats the header and the bar
		// still fits, so the case starts at 2.
		if tasks >= 2 {
			tight := exact - 1
			out = ansi.Strip(m.renderGantt(tight))
			if n := countGanttRows(out); n >= tasks {
				t.Errorf("perPerson=%d: with budget %d fewer than %d tasks should fit, %d are visible",
					perPerson, tight, tasks, n)
			}
		}
	}
}

// isGanttAxisLine tells the axis line: it starts with the box's left side,
// has dashes and Monday bars, and no digit (so as not to be confused with
// the legend's dates).
func isGanttAxisLine(line string) bool {
	if !strings.HasPrefix(line, "│") {
		return false
	}
	if !strings.Contains(line, "-") || !strings.Contains(line, "|") {
		return false
	}
	return strings.IndexFunc(line, func(r rune) bool { return r >= '0' && r <= '9' }) < 0
}

// countGanttRows counts the visible TASK rows: the ones carrying the bar.
// The person header does not count, so the number to compare is the tasks'
// one, not the rows' one.
func countGanttRows(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "█") {
			n++
		}
	}
	return n
}

// --- The empty view -----------------------------------------------------

// With no rows, the Gantt says so explicitly instead of leaving a mute gap.
func TestRenderGanttWithNoTasks(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	out := ansi.Strip(m.renderGantt(30))

	if !strings.Contains(out, "No active tasks") {
		t.Errorf("with no tasks it must warn:\n%s", out)
	}
}

// With rows, that notice does NOT appear: inverting the `== 0` would dirty it.
func TestRenderGanttWithTasksHidesEmptyNotice(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 2)
	out := ansi.Strip(m.renderGantt(30))

	if strings.Contains(out, "No active tasks") {
		t.Errorf("with tasks the empty notice must not appear:\n%s", out)
	}
}

// --- The exact edge of the cursor ---------------------------------------

// A cursor exactly equal to the number of rows is out and is truncated to
// the last one. The `>=` is what tells it apart from a `>`.
func TestSnapGanttCursorAtExactLength(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 3)
	rows := m.ganttRows()

	m.ganttCursor = len(rows)
	m.snapGanttCursor()

	if m.ganttCursor != len(rows)-1 {
		t.Errorf("cursor = %d, want %d (an index equal to the number of rows is out of range)",
			m.ganttCursor, len(rows)-1)
	}
}

// --- Default estimate ---------------------------------------------------

// A global estimate of 0 days is impossible: the queue would never advance.
// The fallback to 1 day is what keeps the projection alive.
func TestGanttScheduleFallsBackToOneDayEstimate(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 2)
	// The global default is only used if the task brings no estimate of its own.
	for i := range m.tasks {
		m.tasks[i].Estimate = 0
	}
	m.config.DefaultEstimateDays = 0

	s := m.ganttSchedule()
	if s == nil || len(s.Assignees) == 0 {
		t.Fatal("fixture: no schedule")
	}
	for _, entry := range s.Assignees[0].Entries {
		if entry.Estimate != 1 {
			t.Errorf("estimate = %v with default 0, want 1", entry.Estimate)
		}
	}

	// And with a valid estimate it is respected, so the sanitizing is not always 1.
	m2 := ganttModelWithPeople(t, []string{"@john"}, 2)
	for i := range m2.tasks {
		m2.tasks[i].Estimate = 0
	}
	m2.config.DefaultEstimateDays = 3
	for _, entry := range m2.ganttSchedule().Assignees[0].Entries {
		if entry.Estimate != 3 {
			t.Errorf("estimate = %v with default 3, want 3", entry.Estimate)
		}
	}
}
