package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/db"
)

// The assignee modal has three lists with a row cap: the roster, the
// person's active tasks and their off-days. The three caps are independent
// and each one adds a line to the render, so the three are checked
// separately against the same fixture.

// With n assignees the roster fits whole and carries no pager footer; with one
// more than fit, it carries one. The footer appears exactly when the list does
// not fit, which is the edge that matters.
func TestAssigneeListFooterOnlyWhenRosterOverflows(t *testing.T) {
	// The sizes are of the roster, which besides the people put in also
	// includes "Me": the footer appears when the roster does not fit, not when
	// the names that make it up do not fit.
	tests := []struct {
		name      string
		roster    int
		wantPager bool
	}{
		{"all fit", assigneeModalMaxRows, false},
		{"one more than fit", assigneeModalMaxRows + 1, true},
		{"many more", assigneeModalMaxRows + 20, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAssigneeModel(t, tt.roster-1) // -1 for "Me"
			out := ansi.Strip(m.renderAssigneeModal(""))
			if got := hasPagerLine(out); got != tt.wantPager {
				t.Errorf("with a roster of %d the footer is %v, want %v\n%s",
					tt.roster, got, tt.wantPager, out)
			}
		})
	}
}

// The footer says what position the cursor is at out of how many there are,
// with the index plus one.
func TestAssigneeListPagerShowsSelectionPosition(t *testing.T) {
	m := newAssigneeModel(t, assigneeModalMaxRows+5)
	m.assigneeIdx = 4
	out := ansi.Strip(m.renderAssigneeModal(""))

	// The total is the roster's, which besides the names put in also includes
	// "Me": the footer does not count only the rows the test added.
	want := fmt.Sprintf("%d/%d", m.assigneeIdx+1, len(m.assigneeRoster()))
	if !strings.Contains(out, want) {
		t.Errorf("the footer does not say %q\n%s", want, out)
	}
}

// The roster's window follows the cursor: with the selection at the end you
// see the last person, not the first ones.
func TestAssigneeListWindowFollowsCursor(t *testing.T) {
	m := newAssigneeModel(t, assigneeModalMaxRows+5)
	last := m.assigneeRoster()[assigneeModalMaxRows+4].Name
	m.assigneeIdx = assigneeModalMaxRows + 4

	out := ansi.Strip(m.renderAssigneeModal(""))
	if !strings.Contains(out, last) {
		t.Errorf("with the cursor at the end %q is not visible\n%s", last, out)
	}
}

// The person's tasks are capped at assigneeModalTaskRows and the leftovers
// are summarized in "... N more", with the exact number of the ones that do not fit.
func TestAssigneeDetailTaskOverflow(t *testing.T) {
	tests := []struct {
		name     string
		tasks    int
		wantMore bool
		wantN    int
	}{
		{"all fit", assigneeModalTaskRows, false, 0},
		{"one too many", assigneeModalTaskRows + 1, true, 1},
		{"ten too many", assigneeModalTaskRows + 10, true, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAssigneeModel(t, 0)
			mustCreateTask(t, m.database, "api", "from @user00", "", "@user00", 1, "todo")
			for range tt.tasks - 1 {
				mustCreateTask(t, m.database, "api", "task", "", "@user00", 1, "todo")
			}
			reloadTasks(t, m)
			m.assigneeIdx = 0
			m.assigneeDetail = true

			out := ansi.Strip(m.renderAssigneeModal(""))
			more := strings.Contains(out, "... "+fmt.Sprint(tt.wantN)+" more")
			if more != tt.wantMore {
				t.Errorf("with %d tasks the summary is %v, want %v\n%s", tt.tasks, more, tt.wantMore, out)
			}
		})
	}
}

// A person with no active tasks and no off-days puts "(none)" in both
// sections. "Me" is who meets that in the fixture: the test tasks all
// have an assignee.
func TestAssigneeDetailNoTasksNoOffdays(t *testing.T) {
	m := newAssigneeModel(t, 2)
	m.assigneeIdx = m.meIndex(t)
	m.assigneeDetail = true

	out := ansi.Strip(m.renderAssigneeModal(""))
	if n := strings.Count(out, "(none)"); n != 2 {
		t.Errorf("the person with nothing has %d occurrences of (none), want 2\n%s", n, out)
	}
}

// The off-days too: capped, with their own footer.
func TestAssigneeDetailOffdayOverflow(t *testing.T) {
	tests := []struct {
		name      string
		offs      int
		wantPager bool
	}{
		{"all fit", assigneeModalOffRows, false},
		{"one more", assigneeModalOffRows + 1, true},
		{"many more", assigneeModalOffRows + 15, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAssigneeModel(t, 1)
			for i := range tt.offs {
				day := fmt.Sprintf("2026-01-%02d", (i%28)+1)
				mustAddOffDay(t, m.database, "@user00", day, day, "")
			}
			reloadOffDays(t, m)
			m.assigneeDetail = true

			out := ansi.Strip(m.renderAssigneeModal(""))
			if got := hasPagerLine(out); got != tt.wantPager {
				t.Errorf("with %d off-days the footer is %v, want %v\n%s", tt.offs, got, tt.wantPager, out)
			}
		})
	}
}

// With active tasks but no off-days, only the off-days section says
// "(none)". The tasks one has its own marker and must not appear.
func TestAssigneeDetailNoOffdays(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	m.assigneeDetail = true

	out := ansi.Strip(m.renderAssigneeModal(""))
	if n := strings.Count(out, "(none)"); n != 1 {
		t.Errorf("with tasks and no off-days there are %d occurrences of (none), want 1\n%s", n, out)
	}
	if strings.Contains(out, "Active tasks") && strings.Count(out, "#") == 0 {
		t.Errorf("the tasks section came out empty without saying (none):\n%s", out)
	}
}

// A long off-day note is the only thing in the modal that does not come
// already truncated by its field, so it is what puts the modal's truncation to
// work. The line has to end exactly at the inner width.
func TestAssigneeDetailTruncatesLongNoteToInnerWidth(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	mustAddOffDay(t, m.database, "@user00", "2026-05-01", "2026-05-02", strings.Repeat("n", 200))
	reloadOffDays(t, m)
	m.assigneeDetail = true

	out := ansi.Strip(m.renderAssigneeModal(""))
	width := modalInnerWidth(modalWidthFor(58, m.width))
	got := widestBoxLine(out, "nnn")
	if got == 0 {
		t.Fatalf("the line with the note was not found:\n%s", out)
	}
	if got != width {
		t.Errorf("the line with the note measures %d, want %d (the modal inner width)", got, width)
	}
}

// Deleting an off-day that fails travels in the message with the error, so
// that the form reopens instead of closing as if it had succeeded.
func TestDeleteOffDayCmdReportsError(t *testing.T) {
	m := newAssigneeModel(t, 1)
	msg, ok := m.deleteOffDayCmd(9999, "@user00")().(offdaySavedMsg)
	if !ok {
		t.Fatal("it did not return offdaySavedMsg")
	}
	if msg.err == nil {
		t.Error("deleting a non-existent id did not report an error")
	}
	if msg.action != "delete" || msg.name != "@user00" {
		t.Errorf("message %+v, want action=delete name=@user00", msg)
	}
}

// And the happy path carries no error, and the off-day disappears.
func TestDeleteOffDayCmdSuccess(t *testing.T) {
	m := newAssigneeModel(t, 1)
	off, err := m.database.AddOffDay("@user00", "2026-03-01", "2026-03-02", "vacation")
	if err != nil {
		t.Fatal(err)
	}

	msg, ok := m.deleteOffDayCmd(off.ID, "@user00")().(offdaySavedMsg)
	if !ok {
		t.Fatal("it did not return offdaySavedMsg")
	}
	if msg.err != nil {
		t.Errorf("deleting an existing off-day failed: %v", msg.err)
	}
	rest, _ := m.database.ListOffDays("@user00")
	if len(rest) != 0 {
		t.Errorf("%d off-days left, want 0", len(rest))
	}
}

// newAssigneeModel opens the modal with n assignees, each with an active task.
func newAssigneeModel(t *testing.T, assignees int) *Model {
	t.Helper()
	if assignees == 0 {
		return newEmptyDBModel(t)
	}
	names := make([]string, assignees)
	for i := range assignees {
		names[i] = fmt.Sprintf("@user%02d", i)
	}
	m := newEmptyDBModel(t)
	for _, name := range names {
		mustCreateTask(t, m.database, "api", "task of "+name, "", name, 1, "todo")
	}
	reloadTasks(t, m)
	m.assigneeModalOpen = true
	m.clampAssigneeIdx()
	return m
}

// newEmptyDBModel starts from a database with nothing, which is what the
// empty roster requires: the normal fixture brings tasks with assignees.
func newEmptyDBModel(t *testing.T) *Model {
	t.Helper()
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	mustCreateProject(t, database, "api", nil)

	m := New(database, config.Defaults())
	m.width = 120
	m.height = 30
	m.assigneeModalOpen = true
	m.clampAssigneeIdx()
	return &m
}

// reloadTasks re-reads the task list, as a tasksLoadedMsg would.
func reloadTasks(t *testing.T, m *Model) {
	t.Helper()
	var err error
	if m.tasks, err = m.database.ListTasks("", "", ""); err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	m.filteredT = nil
}

// meIndex returns the position of "Me" in the model's roster.
func (m *Model) meIndex(t *testing.T) int {
	t.Helper()
	for i, s := range m.assigneeRoster() {
		if s.Name == "Me" {
			return i
		}
	}
	t.Fatal("the roster does not have \"Me\"")
	return 0
}

// reloadOffDays re-reads the off-days, as an offDaysLoadedMsg would.
func reloadOffDays(t *testing.T, m *Model) {
	t.Helper()
	var err error
	if m.offdays, err = m.database.ListOffDays(""); err != nil {
		t.Fatalf("ListOffDays: %v", err)
	}
}

func mustAddOffDay(t *testing.T, database *db.DB, assignee, start, end, note string) {
	t.Helper()
	if _, err := database.AddOffDay(assignee, start, end, note); err != nil {
		t.Fatalf("AddOffDay(%s, %s): %v", assignee, start, err)
	}
}

// hasPagerLine looks for the "n/total" footer of any of the capped lists.
//
// It looks inside the box, not in the whole line: the rendered line carries
// the border, the centering and the right padding, so splitting it into
// fields by whitespace finds nothing. Only an "n/m" with both sides numeric
// and nothing else inside the border is accepted, so as not to confuse it
// with a date ("2026-01-01 → 2026-01-02", which also carries a slash).
func hasPagerLine(rendered string) bool {
	for _, line := range strings.Split(rendered, "\n") {
		start := strings.Index(line, "│")
		if start < 0 {
			continue
		}
		rest := line[start+len("│"):]
		if end := strings.Index(rest, "│"); end >= 0 {
			rest = rest[:end]
		}
		if isPager(rest) {
			return true
		}
	}
	return false
}

// isPager tells whether a line fragment is the "n/total" footer and nothing else.
func isPager(fragment string) bool {
	fields := strings.Fields(fragment)
	if len(fields) != 1 || !strings.Contains(fields[0], "/") {
		return false
	}
	slash := strings.Index(fields[0], "/")
	return isAllDigits(fields[0][:slash]) && isAllDigits(fields[0][slash+1:])
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// widestBoxLine returns the width of the widest line of the modal's box that
// contains marker, measuring what is between the two vertical borders.
func widestBoxLine(rendered, marker string) int {
	width := 0
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		start := strings.Index(line, "│")
		if start < 0 {
			continue
		}
		rest := line[start+len("│"):]
		if end := strings.Index(rest, "│"); end >= 0 {
			rest = rest[:end]
		}
		if w := len([]rune(rest)); w > width {
			width = w
		}
	}
	return width
}
