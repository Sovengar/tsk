package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// ganttStart is a fixed date so that the Gantt render is reproducible.
var ganttStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Batch of edges that were left unchecked in files where the rest of the
// render was already covered: the selection point of the people modal's
// lists, the width of the Gantt ruler, the "nothing selected" resets of the
// description editor, and the tag deletion of the form.

// The roster's selected row carries the selection prefix and the rest do not.
// With two people in the roster and the cursor on the second one, only that one carries it.
func TestAssigneeRosterSelectionPrefix(t *testing.T) {
	m := newAssigneeModel(t, 2)
	m.archivedProjects = nil
	reloadOffDays(t, m)
	m.assigneeIdx = 1

	// The prefix is searched in the clean text; the bold, in the render with
	// its codes, because removing the codes makes the two marks indistinguishable.
	raw := m.renderAssigneeModal("")
	out := ansi.Strip(raw)
	if n := strings.Count(out, selectionPrefix(true)); n != 1 {
		t.Errorf("there are %d rows with the selection prefix, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, selectionPrefix(true)+"@user01") {
		t.Errorf("the mark is not on the second person:\n%s", out)
	}
	if n := countBoldLines(raw); n != 1 {
		t.Errorf("there are %d highlighted rows, want 1:\n%s", n, out)
	}
}

// The same in the off-days list: the cursor goes over a concrete off-day,
// not over "the list".
func TestAssigneeOffdaySelectionPrefix(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	for i := range 3 {
		day := "2026-0" + string(rune('1'+i)) + "-01"
		mustAddOffDay(t, m.database, "@user00", day, day, "")
	}
	reloadOffDays(t, m)
	m.assigneeDetail = true
	m.assigneeOffdayIdx = 2

	// The prefix is searched in the clean text; the bold, in the render with
	// its codes, because removing the codes makes the two marks indistinguishable.
	raw := m.renderAssigneeModal("")
	out := ansi.Strip(raw)
	if n := strings.Count(out, selectionPrefix(true)); n != 1 {
		t.Errorf("there are %d rows with the selection prefix, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, selectionPrefix(true)+"2026-03-01") {
		t.Errorf("the mark is not on the third off-day:\n%s", out)
	}
	// The detail also highlights the person's name in its header, so
	// the count is limited to the off-day rows: they are the ones carrying an arrow.
	if n := countBoldLinesWith(raw, "→"); n != 1 {
		t.Errorf("there are %d highlighted off-days, want 1:\n%s", n, out)
	}
}

// The Gantt axis has the label's width, a separator and the day columns,
// untruncated. The ruler is no use for this: it strips the right-hand
// spaces, so its visible width depends on where the last Monday falls.
func TestGanttAxisExactWidth(t *testing.T) {
	tests := []struct {
		name    string
		labelW  int
		dayCols int
	}{
		{"roomy", 30, 40},
		{"label at the minimum", 14, 7},
		{"a single day", 30, 1},
		{"with minimum label and days", 14, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ganttModelWithPeople(t, []string{"@john"}, 1)

			axis := ansi.Strip(m.renderGanttAxis(tt.labelW, tt.dayCols, ganttStart, 0))
			if got := len([]rune(axis)); got != tt.labelW+1+tt.dayCols {
				t.Errorf("the axis measures %d columns, want %d (label %d + 1 + days %d)",
					got, tt.labelW+1+tt.dayCols, tt.labelW, tt.dayCols)
			}
		})
	}
}

// Each Monday's caption falls in its column: the label on the left, the
// separator, and from there the day. January 1st of 2026 is a Thursday, so
// the first Monday is day 4, in column 35 with a label of 30.
func TestGanttRulerLabelLandsOnItsMonday(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 1)

	const labelW = 30
	ruler := []rune(ansi.Strip(m.renderGanttRuler(ganttStart, 0, labelW, 20)))
	const firstMonday = 4 // 2026-01-05

	want := model.WeekOfMonthLabel(ganttStart.AddDate(0, 0, firstMonday))
	at := labelW + 1 + firstMonday
	if at+len([]rune(want)) > len(ruler) {
		t.Fatalf("the caption %q does not fit in the %d-column ruler", want, len(ruler))
	}
	if got := string(ruler[at : at+len([]rune(want))]); got != want {
		t.Errorf("column %d has %q, want %q (ruler %q)", at, got, want, string(ruler))
	}
	// And the columns before the Monday are blank: the caption cannot
	// start earlier.
	for i := firstMonday; i < at; i++ {
		if ruler[i] != ' ' {
			t.Fatalf("column %d = %q, want a space before the caption", i, string(ruler[i]))
		}
	}
}

// The label to the left of the axis is spaces of exactly the label's
// width, not one more.
func TestGanttAxisLeadingSpaces(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 1)

	axis := []rune(ansi.Strip(m.renderGanttAxis(30, 10, ganttStart, 0)))
	for i, r := range axis {
		if i < 31 {
			if r != ' ' {
				t.Fatalf("column %d = %q, want a space (label + separator)", i, string(r))
			}
			continue
		}
		break
	}
	// From the separator on there is one character per day, and the first one
	// is Thursday, which is not Monday, so it is "-".
	if axis[31] != '-' {
		t.Errorf("the first day is %q, want \"-\"", string(axis[31]))
	}
}

// Opening the description editor clears the comment selection, just like
// opening the detail: the description has no comments, and leaving the
// selection pointing at a comment index that is not being shown would make
// the first key jump somewhere else.
func TestOpeningDescEditorClearsCommentSelection(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.detailComments = []model.Comment{{ID: 1, Body: "one", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.detailCommentSel = 1

	next, _ := press(m, "e")
	got := next

	if !got.descEditOpen {
		t.Fatal("the key e did not open the description editor")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1", got.detailCommentSel)
	}
}

// The description editor opens on the detail's task if the detail is open,
// and on the selected one in the view otherwise. With the detail open,
// editing is on the detail's task even if the list cursor is on another one.
func TestDescEditTargetPrefersOpenDetail(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	m.filteredT = nil

	if got := m.descEditTarget(); got == nil || got.ID != task.ID {
		t.Errorf("with the detail open the target is %v, want the detail's task", got)
	}

	m.detailOpen = false
	m.cursor = 1
	selected := m.filteredTasks()[1]
	if got := m.descEditTarget(); got == nil || got.ID != selected.ID {
		t.Errorf("without the detail the target is %v, want the selected task", got)
	}

	// With the detail "open" but with no task -- a state the interface does
	// not produce but a test can leave -- the target falls to the list, not
	// to nil: there is a selected task and it is as editable as the other.
	m.detailOpen = true
	m.detailTask = nil
	m.cursor = 1
	if got := m.descEditTarget(); got == nil {
		t.Error("with the detail open and no task the target is nil; want the view's selected one")
	} else if got.ID != m.filteredTasks()[1].ID {
		t.Errorf("the target is task %d, want the selected one (%d)", got.ID, m.filteredTasks()[1].ID)
	}

	// And with neither of the two, nil.
	m.detailOpen = false
	m.filteredT = nil
	m.cursor = 99
	if got := m.descEditTarget(); got != nil {
		t.Errorf("with no task selected the target is %v, want nil", got)
	}
}

// Deleting tags from the form field: with an empty input, backspace removes
// the last one added; with something typed, it removes a character from the
// input. And editing Tags clears the suggestion selection, because the list is going to change.
func TestNewTaskTagsBackspace(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		tags      []string
		wantInput string
		wantTags  []string
	}{
		{"with typed input it removes a character", "ab", nil, "a", nil},
		{"with empty input it removes the last tag", "", []string{"one", "two"}, "", []string{"one"}},
		{"without tags or input it does nothing", "", nil, "", nil},
		{"with one tag it removes it", "", []string{"one"}, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			next, _ := press(m, "i")
			m2 := next
			m2.newTaskFieldIdx = newTaskFieldTags
			m2.newTaskTagInput = tt.input
			m2.newTaskTags = append([]string(nil), tt.tags...)
			m2.newTaskTagSuggIdx = 2

			got, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
			gotM := got.(Model)

			if gotM.newTaskTagInput != tt.wantInput {
				t.Errorf("input = %q, want %q", gotM.newTaskTagInput, tt.wantInput)
			}
			if strings.Join(gotM.newTaskTags, ",") != strings.Join(tt.wantTags, ",") {
				t.Errorf("tags = %v, want %v", gotM.newTaskTags, tt.wantTags)
			}
			// Editing the input clears the suggestion selection; deleting an
			// added tag does not. That is what the code does, and it is not a
			// problem: the index is bounded before being used.
			wantSugg := -1
			if tt.input == "" && len(tt.tags) > 0 {
				wantSugg = 2
			}
			if gotM.newTaskTagSuggIdx != wantSugg {
				t.Errorf("suggIdx = %d, want %d", gotM.newTaskTagSuggIdx, wantSugg)
			}
		})
	}
}

// "k" in Kanban wraps: from the first card it jumps to the last of the
// column, and on the last one it stays. It is a cycle, not a scroll, and
// the difference shows on the first keypress.
func TestKanbanUpWrapsWithinColumn(t *testing.T) {
	m := newKanbanModel(t, 3) // three cards in "todo", one column with tasks
	m.kanbanCol = 0
	m.kanbanRow = 0

	up, _ := press(m, "k")
	if up.kanbanRow == up.kanbanCol {
		t.Fatalf("the row did not change")
	}
	if up.kanbanRow != 2 {
		t.Errorf("from the first, \"k\" leaves the row at %d, want 2 (the last)", up.kanbanRow)
	}

	// And from the last one it goes back to the previous: it wraps upwards, it does not stay.
	up2, _ := press(up, "k")
	if up2.kanbanRow != 1 {
		t.Errorf("from the last, \"k\" leaves the row at %d, want 1", up2.kanbanRow)
	}
}

// The Dashboard's panels are capped by the height, and the cap is exact:
// with one more line of height one more team row comes out, and one less, one less.
func TestDashboardTeamPanelRespectsBudget(t *testing.T) {
	m := newDashModel(t, "")
	for i := range 8 {
		mustCreateTask(t, m.database, "api", "task of p"+string(rune('a'+i)), "", "@p"+string(rune('a'+i)), 1, "todo")
	}
	reloadTasks(t, m)
	m.filteredT = nil

	m.currentView = viewDashboard
	m.width = 140

	count := func(height int) int {
		out := ansi.Strip(m.renderDashboard(height))
		n := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, " tasks (") {
				n++
			}
		}
		return n
	}

	// The budget comes from the height minus the chrome, minus what the
	// Overview takes and minus the panel header, so a really small height is
	// needed. With ten people on the team the budget shows: a height of 60
	// shows all of them, 20 shows six, 16 shows two and at 14 none fits
	// anymore -- the Overview takes the whole column.
	cases := []struct{ height, want int }{{60, 10}, {30, 10}, {20, 6}, {16, 2}, {14, 0}}
	for _, c := range cases {
		if got := count(c.height); got != c.want {
			t.Errorf("at height %d there are %d team rows, want %d", c.height, got, c.want)
		}
	}
}

// The Active panel too, and with one more task of height one more comes out.
func TestDashboardActivePanelRespectsBudget(t *testing.T) {
	m := newDashModel(t, "")
	m.currentView = viewDashboard
	m.width = 140

	count := func(height int) int {
		out := ansi.Strip(m.renderDashboard(height))
		n := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "  ● ") || strings.Contains(line, "  ○ ") {
				n++
			}
		}
		return n
	}

	tall := count(60)
	short := count(8)
	if short >= tall {
		t.Errorf("at height 8 there are %d active rows and at 60 there are %d", short, tall)
	}
	if short == 0 {
		t.Error("at height 8 no active row comes out")
	}
}

// Closing the description editor with no detail behind leaves the detail
// closed and clean, comment selection included: otherwise the reopened
// detail would point at a comment that is no longer being shown.
func TestClosingDescEditorClearsDetailState(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.descEditOpen = true
	m.descEditTaskID = m.detailTask.ID
	m.descEditHadDetail = false // the detail was not open before editing
	m.detailCommentSel = 2
	m.detailComments = []model.Comment{{ID: 1, Body: "one"}}

	m.closeDescEditor()

	if m.descEditOpen {
		t.Error("the editor is still open")
	}
	if m.detailOpen {
		t.Error("the detail is still open, and it was not open before editing")
	}
	if m.detailTask != nil {
		t.Errorf("the detail's task ended up as %v, want nil", m.detailTask)
	}
	if m.detailComments != nil {
		t.Errorf("the comments ended up as %v, want nil", m.detailComments)
	}
	if m.detailCommentSel != -1 {
		t.Errorf("the selection ended up as %d, want -1", m.detailCommentSel)
	}
}

// And with a detail behind, closing it leaves it open: you go back to the
// detail, not to the list.
func TestClosingDescEditorKeepsExistingDetail(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.descEditOpen = true
	m.descEditHadDetail = true
	m.detailCommentSel = 1

	m.closeDescEditor()

	if m.descEditOpen {
		t.Error("the editor is still open")
	}
	if !m.detailOpen {
		t.Error("the detail closed, and it was open before editing")
	}
	if m.detailTask == nil {
		t.Error("the detail's task was lost")
	}
	// The comment selection stays: the user is going to keep seeing them.
	if m.detailCommentSel != 1 {
		t.Errorf("the selection changed to %d, want 1 (untouched)", m.detailCommentSel)
	}
}

// On leaving the assignee field with a selected suggestion, the input
// completes with that suggestion. That is what tells "I chose one" from
// "I typed one": with no selected suggestion the input stays as it was.
func TestNewTaskLeavingAssigneeCompletesSelection(t *testing.T) {
	t.Run("with a suggestion", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := press(m, "i")
		m2 := next
		m2.newTaskFieldIdx = newTaskFieldAssignee
		m2.newTaskAssignee = "@j"
		m2.newTaskAssigneeSuggIdx = 0 // "@john", the first suggestion

		got, _ := m2.newTaskMoveField(1)
		gotM := got.(Model)

		if gotM.newTaskAssignee != "@john" {
			t.Errorf("the assignee ends at %q, want @john (the chosen suggestion)", gotM.newTaskAssignee)
		}
		if gotM.newTaskAssigneeSuggIdx != -1 {
			t.Errorf("suggIdx = %d, want -1 after completing", gotM.newTaskAssigneeSuggIdx)
		}
	})

	t.Run("without a suggestion", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := press(m, "i")
		m2 := next
		m2.newTaskFieldIdx = newTaskFieldAssignee
		m2.newTaskAssignee = "@free"
		m2.newTaskAssigneeSuggIdx = -1

		got, _ := m2.newTaskMoveField(1)
		gotM := got.(Model)

		if gotM.newTaskAssignee != "@free" {
			t.Errorf("the assignee changed to %q with no suggestion selected", gotM.newTaskAssignee)
		}
	})
}

// The highlighted row is the cursor's, and not "the first of the window":
// with the window scrolled, the index of the drawn row and the cursor's
// stop matching, and it is the sum of the two -- neither the subtraction
// nor the first -- that points at the right one.
func TestGanttSelectedRowFollowsScrolledWindow(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john", "@margo"}, 6)
	m.currentView = viewGantt
	m.width = 120

	rows := m.ganttRows()
	first, last := -1, -1
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if last <= first {
		t.Fatalf("the fixture needs several task rows, there are %d..%d", first, last)
	}

	// A height that does not show all the rows, with the cursor at the end:
	// the window scrolls and the drawn index stops being zero.
	m.ganttCursor = last
	out := ansi.Strip(m.renderGantt(listFixedRows + ganttRulerRows + 3))
	if !strings.Contains(out, "> #") {
		t.Fatalf("there is no highlighted row:\n%s", out)
	}
	if want := fmt.Sprintf("> #%d", rows[last].entry.Task.ID); !strings.Contains(out, want) {
		t.Errorf("the highlighted row is not the cursor's (%s):\n%s", want, out)
	}
	// The window does not show more than the three rows that fit. It is
	// counted by the task prefix "#": task rows carry it, person headers do
	// not, and each drawn row is counted once regardless of its id ("#1" is
	// also inside "#11").
	drawn := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "#") {
			drawn++
		}
	}
	if drawn != 3 {
		t.Errorf("%d task rows are drawn, want 3 (the ones that fit)", drawn)
	}
	// And there is exactly one highlighted row: the cursor's.
	if n := strings.Count(out, "> #"); n != 1 {
		t.Errorf("there are %d highlighted rows, want 1:\n%s", n, out)
	}
}
