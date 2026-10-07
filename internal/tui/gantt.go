package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

// ganttRulerRows are the fixed rows of the Gantt that are not data: the
// weeks' ruler and its axis.
const ganttRulerRows = 2

// ganttRowKind tells a person header from a task row.
type ganttRowKind int

const (
	ganttAssigneeRow ganttRowKind = iota
	ganttTaskRow
)

// ganttRow is a navigable row of the Gantt.
type ganttRow struct {
	kind     ganttRowKind
	assignee string
	entry    *model.ScheduleEntry
}

// ganttSchedule projects each person's queue from ALL the loaded tasks and
// off-days. The starting point is today. The queue is global: the
// capacity of a person is spread across all their projects, so the
// dates do not depend on which project you are looking at.
func (m *Model) ganttSchedule() *model.Schedule {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// The estimate sanitizing (0 or negative -> 1 day) is done by BuildSchedule.
	// There was an identical guard that could only produce the same value: its
	// mutant was equivalent by construction, not a coverage gap.
	return model.BuildSchedule(m.tasks, m.offdays, start, m.config.DefaultEstimateDays)
}

// ganttDisplay applies the active filters as a VIEW filter over the global
// projection: it only decides which rows are seen, without recalculating dates.
func (m *Model) ganttDisplay() *model.Schedule {
	return model.FilterSchedule(m.ganttSchedule(), func(t model.Task) bool {
		return m.taskMatchesFilter(t)
	})
}

// ganttRows flattens a schedule into navigable rows: a header per person and
// one row per task, in the queue's order.
func ganttRows(s *model.Schedule) []ganttRow {
	var rows []ganttRow
	for _, a := range s.Assignees {
		rows = append(rows, ganttRow{kind: ganttAssigneeRow, assignee: a.Assignee})
		for i := range a.Entries {
			rows = append(rows, ganttRow{kind: ganttTaskRow, assignee: a.Assignee, entry: &a.Entries[i]})
		}
	}
	return rows
}

// ganttRows returns the rows of the current schedule, already filtered for the view.
func (m *Model) ganttRows() []ganttRow {
	return ganttRows(m.ganttDisplay())
}

// snapGanttCursor reframes the cursor within the current rows and always
// rests it on a task row: person headers are not navigable.
//
// That "always" being true is structural: ganttRows only emits the header of
// a person that has entries, so the row after a header is always a task, and
// the forward search finds it. The callers do not need to check the type of
// the row.
func (m *Model) snapGanttCursor() {
	rows := m.ganttRows()
	if len(rows) == 0 {
		m.ganttCursor = 0
		return
	}
	m.ganttCursor = max(m.ganttCursor, 0)
	if m.ganttCursor >= len(rows) {
		m.ganttCursor = len(rows) - 1
	}
	if rows[m.ganttCursor].kind == ganttTaskRow {
		return
	}
	// Header: jump to the first task after it. There is no "if there is none, to
	// the previous one" case: ganttRows only emits the header of a person that
	// has entries, so the row after a header is always a task and this search
	// always terminates. The backward search that was here was not a safety net:
	// it was dead code, with its own comment saying that it could never
	// be reached.
	//
	// It is walked with range over subslices instead of a for with a manual
	// index: an inverted `i++` leaves the loop hanging and the mutant is
	// reported as TIMED OUT, not as dead.
	// The jump is made with an index counted from the cursor, and not with the
	// offset inside the subslice: `1 + i` with i being the subslice's index was
	// a sum of two numbers describing the same thing, and its mutant (`+ 2 * i`,
	// `+ i - i`) did not tell itself apart because i was always 0 -- the row
	// after a header is always a task.
	//
	// With nextGanttTaskRow the jump is a named operation and the index is the
	// absolute one from the start, which is what is kept. Less arithmetic that
	// can mutate without anyone noticing.
	m.ganttCursor = nextGanttTaskRow(rows, m.ganttCursor)
}

// rowIsTask says whether the index falls on a gantt task row, that is,
// whether there is a selected task.
//
// The range uses inRange, the same helper the Kanban uses for its cursor. The
// loose condition -- `>= 0 && < len(rows)` -- had a `>= 0` whose mutant
// (`> 0`) only told itself apart in row 0, and row 0 of a gantt with rows is
// always a person header: ganttRows emits the header before that person's
// entries. So the mutant was indistinguishable not for lack of a test but
// because the value separating it did not exist.
//
// inRange has no such edge: checking that the index is in the list and that
// the row at that index is a task are two questions and both are answered.
func rowIsTask(rows []ganttRow, i int) bool {
	return inRange(i, len(rows)) && rows[i].kind == ganttTaskRow
}

// nextGanttTaskRow returns the index of the first task row at or after
// `from`, or -1 if there is none.
//
// The search starts at `from` and not at `from+1` on purpose: the caller
// already knows the current row is not a task, so starting at it does not
// change the result, and starting at `from` makes the "there is no row" case
// a real -1 and not a hit with i == 0.
func nextGanttTaskRow(rows []ganttRow, from int) int {
	for i := max(from, 0); i < len(rows); i++ {
		if rows[i].kind == ganttTaskRow {
			return i
		}
	}
	return -1
}

// ganttNoTasks is the sentinel ganttTaskRange returns when the view has no
// task row at all. It is -1 and not 0 because row 0 is always a person
// header when there are tasks.
const ganttNoTasks = -1

// ganttTaskRange returns the index of the first and last task row of the
// Gantt, or ganttNoTasks if there is none.
func ganttTaskRange(rows []ganttRow) (first, last int) {
	first, last = ganttNoTasks, ganttNoTasks
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	return first, last
}

// moveGanttCursor jumps to the next/previous task row in the direction dir
// (+1 down, -1 up), skipping the person headers.
//
// The walk uses slices.IndexFunc on the row, and the range condition
// disappears with it. Before it was `for i := cursor + dir; i >= 0 && i < len(rows)`
// and its `i >= 0` only told itself apart from `i > 0` if row 0 were a task,
// which never happens because row 0 is always the first person's header. With
// IndexFunc the search has no lower bound to compare -- it is "from here to
// the end" or "from the start to here", and the two directions are told by
// the step.
func moveGanttCursor(m *Model, rows []ganttRow, dir int) {
	// The jump returns an ok instead of a sentinel because any comparison
	// against a -1 has the edge problem: "not -1" and "greater than -1" are the
	// same condition over integers, so no test kills its mutant; and "< -1" is
	// always false, which breaks the jumps to index 0.
	//
	// With the ok, "there is no jump" is a fact and not a number, and the only
	// question the caller asks is whether there is a jump. Its mutant --
	// negating the ok -- does show: the cursor would go to the sentinel.
	if target, ok := stepGanttCursor(rows, m.ganttCursor, dir); ok {
		m.ganttCursor = target
	}
}

// stepGanttCursor returns the index of the task row the cursor jumps to from
// `from` in the direction `dir`, and a false if there is none.
//
// The second value is a boolean and not a sentinel because of what the
// caller says: comparing an index against -1 has an edge no test reaches.
//
// All the arithmetic lives here and not in the caller, and on purpose. With
// the walk spread across moveGanttCursor and its two branches, each line
// carried its own `max(min(...))`: the floor at 0 did not tell itself apart
// from the floor at 1, and the `+ 1` of the offset did not tell itself apart
// from `+ 2`, because the two values separating them -- a negative cursor and
// a cursor beyond the end -- only happen in states the previous walk did not
// produce. Here, on the other hand, `from` is a parameter: a test can pass whatever it wants and check each branch from both sides.
//
// The two directions are solved with the same loop, over a subslice that
// goes from the target position to the end or to the start. The subslice is
// bounded with a clamp because an out-of-range cursor must not make the
// slice be cut the other way around.
func stepGanttCursor(rows []ganttRow, from, dir int) (int, bool) {
	if dir < 0 {
		until := clamp(from, 0, len(rows))
		for i, r := range slices.Backward(rows[:until]) {
			if r.kind == ganttTaskRow {
				return i, true
			}
		}
		return 0, false
	}

	start := clamp(from+1, 0, len(rows))
	for i, r := range rows[start:] {
		if r.kind == ganttTaskRow {
			return start + i, true
		}
	}
	return 0, false
}

// clamp bounds v to the range [lo, hi]. With lo <= hi -- which is the case of
// all the uses here: 0 is the floor and hi is the length of a list -- the
// range always has a value inside, so there is no need to decide what happens if not.
func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// handleGanttKey navigates the Gantt: j/k over rows, h/l scrolls the day
// window, g/G goes to the start/end, Enter opens the task's detail.
func (m Model) handleGanttKey(key string) (tea.Model, tea.Cmd) {
	m.snapGanttCursor()
	rows := m.ganttRows()

	switch key {
	case "/":
		return m, m.openFilterModal()
	case "tab":
		// Switch project by cycling the Project filter, as in the Dashboard.
		m.cycleProjectFilter(1)
		m.ganttCursor = 0
		m.snapGanttCursor()
	case "j", "down":
		moveGanttCursor(&m, rows, 1)
	case "k", "up":
		moveGanttCursor(&m, rows, -1)
	case "h", "left":
		if m.ganttOffsetDays > 0 {
			m.ganttOffsetDays--
		}
	case "l", "right":
		m.ganttOffsetDays++
	case "g":
		// -1 is the "there are no tasks" sentinel. Comparing against it instead
		// of against 0 leaves the `>=` as what it is: the BOUNDARY of `>= 0` is
		// equivalent because every header occupies row 0.
		if first, _ := ganttTaskRange(rows); first != ganttNoTasks {
			m.ganttCursor = first
		}
	case "G":
		if _, last := ganttTaskRange(rows); last != ganttNoTasks {
			m.ganttCursor = last
		}
	case "enter":
		// The entry snap always leaves the cursor on a task row, so the `kind`
		// check that was here could not be false. What does need bounding is the
		// range: with no rows there is nothing to open.
		if inRange(m.ganttCursor, len(rows)) {
			t := rows[m.ganttCursor].entry.Task
			m.detailOpen = true
			m.detailTask = &t
			m.detailComments = nil
			m.detailCommentSel = -1
			return m, m.loadCommentsCmd(t.ID)
		}
	}
	return m, nil
}

// renderGantt draws the Gantt within the available height: one row per task,
// one column per day, grouped by person.
func (m *Model) renderGantt(maxHeight int) string {
	// The filters may have left fewer rows: reframe the cursor.
	m.snapGanttCursor()

	w := m.width
	innerW := w - 2

	s := m.ganttDisplay()
	rows := ganttRows(s)

	// Width split: label on the left, days on the right.
	labelW, dayCols := ganttLabelAndDays(innerW)

	start, _ := model.ParseDate(s.Start)
	offset := m.ganttOffsetDays
	offset = max(offset, 0)

	// Vertical window that follows the cursor.
	visible := maxHeight - listFixedRows - ganttRulerRows
	visible = max(visible, 1)
	// visibleRange already returns the whole window when everything fits, so the
	// previous `if len(rows) > visible` only had two branches with the same
	// result: its BOUNDARY was an equivalent mutant.
	vStart, vEnd := visibleRange(m.ganttCursor, len(rows), visible)

	lines := []string{
		m.renderFilterHeader(innerW),
		m.renderGanttRuler(start, offset, labelW, dayCols),
		m.renderGanttAxis(labelW, dayCols, start, offset),
	}

	if len(rows) == 0 {
		lines = append(lines, styleDim.Render("  No active tasks."))
	}

	for i, row := range rows[vStart:vEnd] {
		selected := i+vStart == m.ganttCursor
		lines = append(lines, m.renderGanttRow(row, start, offset, dayCols, labelW, selected))
	}

	content := strings.Join(lines, "\n")
	content = truncateLines(content, innerW)

	borderFg := lipgloss.Color("8")
	return bordered.RenderWithTitlesEx(
		lipgloss.RoundedBorder(),
		borderFg,
		" Gantt ",
		bordered.AlignLeft,
		" "+m.ganttLegend(offset, dayCols)+" ",
		bordered.AlignRight,
		content,
		w,
	)
}

// renderGanttRuler is the top line with each week's label ("1SEP") aligned
// to each visible Monday.
func (m *Model) renderGanttRuler(start time.Time, offset, labelW, dayCols int) string {
	ruler := make([]rune, labelW+1+dayCols)
	for i := range ruler {
		ruler[i] = ' '
	}
	for col := range dayCols {
		day := start.AddDate(0, 0, offset+col)
		if day.Weekday() != time.Monday {
			continue
		}
		label := model.WeekOfMonthLabel(day)
		at := labelW + 1 + col
		for i, r := range label {
			if at+i < len(ruler) {
				ruler[at+i] = r
			}
		}
	}
	return styleDim.Render(strings.TrimRight(string(ruler), " "))
}

// renderGanttAxis draws a "|" on each Monday and "-" on the rest of the days.
func (m *Model) renderGanttAxis(labelW, dayCols int, start time.Time, offset int) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", labelW+1))
	for col := range dayCols {
		if start.AddDate(0, 0, offset+col).Weekday() == time.Monday {
			b.WriteString("|")
		} else {
			b.WriteString("-")
		}
	}
	return styleSep.Render(b.String())
}

// renderGanttRow draws a row: a person header or a task bar.
func (m *Model) renderGanttRow(row ganttRow, start time.Time, offset, dayCols, labelW int, selected bool) string {
	if row.kind == ganttAssigneeRow {
		return styleColumnHeader.Render(cellWidth(row.assignee, labelW+1+dayCols))
	}

	e := row.entry
	d0 := daysBetween(start, e.Start)
	d1 := daysBetween(start, e.End)

	cells := make([]rune, dayCols)
	for col := range dayCols {
		d := offset + col
		day := start.AddDate(0, 0, d)
		switch {
		case d >= d0 && d <= d1:
			cells[col] = '█'
		case model.IsOffDay(m.offdays, row.assignee, day):
			cells[col] = '·'
		default:
			cells[col] = ' '
		}
	}

	mark := ""
	if e.EstimateDefaulted {
		mark = "~"
	}
	prefix := "  "
	if selected {
		prefix = "> "
	}
	label := fmt.Sprintf("%s#%d %s %s%s", prefix, e.Task.ID, e.Task.Title, model.FormatEstimate(e.Estimate), mark)
	line := cellWidth(label, labelW) + " " + string(cells)
	if selected {
		return styleSelected.Render(line)
	}
	return line
}

// ganttLegend describes the visible range and the total horizon.
func (m *Model) ganttLegend(offset, dayCols int) string {
	s := m.ganttSchedule()
	// The ParseDate error is not checked: s.Start is set by ganttSchedule
	// formatting a time.Time with the same layout ParseDate reads, so the
	// parsing cannot fail. Before there was a backup return "Gantt" that was
	// unreachable and that, if it could ever have fired, would have hidden a
	// real failure behind a label with no dates.
	start, _ := model.ParseDate(s.Start)
	from := start.AddDate(0, 0, offset).Format("2006-01-02")
	to := start.AddDate(0, 0, offset+dayCols-1).Format("2006-01-02")
	return fmt.Sprintf("%s → %s ", from, to)
}

// daysBetween counts the calendar days between two YYYY-MM-DD dates.
func daysBetween(start time.Time, date string) int {
	d, err := model.ParseDate(date)
	if err != nil {
		return -1
	}
	return int(d.Sub(start).Hours() / 24)
}
