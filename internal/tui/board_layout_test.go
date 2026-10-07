package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// These tests measure the Kanban render in exact columns. The reason: the
// width split between columns is embedded in the render and comparing "it
// comes out" does not see it. A moved sign changes where the last column ends
// without removing a single word from the screen.
//
// All the indexes are by runes. The box corners and the horizontal
// dashes are multibyte, and strings.Index would give the position in bytes:
// with them in the way, the last column looked three times further to the
// right than where it was.

// boardRow returns the board line: the first one that has column
// corners, and the columns where their borders are.
func boardRow(t *testing.T, out string) []rune {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		runes := []rune(line)
		for i, r := range runes {
			if r == '╔' || (r == '╭' && i > 1) {
				return runes
			}
		}
	}
	t.Fatalf("the board row was not found:\n%s", out)
	return nil
}

// The last column reaches exactly the frame's inner border, with no gap
// behind it. The gap goes *between* columns: if it also went behind the last
// one, the board would be kanbanGap columns short of the border unnoticed.
func TestKanbanLastColumnReachesInnerEdge(t *testing.T) {
	tests := []struct {
		name  string
		width int
	}{
		// Only with slack: if the minimum widths do not fit, the board does not
		// reach the border, and this check says nothing about the split.
		{"very wide", 240},
		{"wide", 160},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newKanbanModel(t, 3)
			m.width = tt.width

			runes := boardRow(t, ansi.Strip(m.renderKanban(30)))
			// The frame closes with "╮│": the corner of the last column and its
			// own right border, and behind it the frame's.
			if len(runes) != tt.width {
				t.Fatalf("the row measures %d columns, want %d", len(runes), tt.width)
			}
			// The line ends in "...╮│": the corner of the last column right at
			// the last inner column, and behind it the frame's border.
			if runes[len(runes)-1] != '│' {
				t.Fatalf("the row does not end at the frame border: %q", string(runes[len(runes)-3:]))
			}
			if r := runes[len(runes)-2]; r != '╮' && r != '╝' && r != '│' {
				t.Errorf("the last column does not reach the inner border, it ends in %q: %q",
					string(r), string(runes[len(runes)-8:]))
			}
		})
	}
}

// Between columns there is kanbanGap of gap, neither more nor less. With
// three columns there are two gaps.
func TestKanbanGapBetweenColumns(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 1, []string{"todo", "doing", "done"})
	m.width = 240

	runes := boardRow(t, ansi.Strip(m.renderKanban(30)))
	// The columns that are not selected open with "╭" and close with "╮";
	// the selected one uses the double border. The indexes of the openings, in order.
	var openings []int
	for i, r := range runes {
		if r == '╭' || r == '╔' {
			openings = append(openings, i)
		}
	}
	want := len(m.kanbanColumns())
	if len(openings) != want {
		t.Fatalf("found %d column openings and the board has %d: %q", len(openings), want, string(runes[:60]))
	}
	openings = openings[1:] // the first one is the outer frame's

	// Each column takes its width plus kanbanGap of gap to the right. The gap
	// is measured between the close of one and the opening of the next.
	for i := 0; i+1 < len(openings); i++ {
		closing := closeOfColumn(runes, openings[i])
		if got := openings[i+1] - closing - 1; got != kanbanGap {
			t.Errorf("between columns %d and %d there are %d columns of gap, want %d",
				i, i+1, got, kanbanGap)
		}
	}
}

// closeOfColumn returns the index of the right border of the column that
// opens at opening, or -1 if it is not found.
func closeOfColumn(runes []rune, opening int) int {
	for i := opening + 1; i < len(runes); i++ {
		if runes[i] == '╮' || runes[i] == '╝' {
			return i
		}
	}
	return -1
}

// The "cancelled" column is not in the project's workflow, and yet it is
// added at the end of the board as soon as there is a cancelled task.
//
// It is the status the database always accepts, whether it is in the workflow
// or not, so a task can end up there without any workflow column
// contemplating it. Without this column those tasks would be invisible on the board.
func TestKanbanCancelledColumnAppearsOnlyWithCancelledTasks(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 2, []string{"todo", "done"})

	if hasColumn(m, model.CancelledStatus) {
		t.Error("the cancelled column appeared with no cancelled task")
	}

	moveToCancelled(t, m, m.tasks[0].ID)
	reloadTasks(t, m)

	if !hasColumn(m, model.CancelledStatus) {
		t.Error("with a cancelled task its column did not appear")
	}
	if hasColumn(m, "never-exists") {
		t.Error("a column appeared for a status that does not exist")
	}
}

// And that column goes at the end, not interleaved: the workflow's order rules
// and "cancelled" is outside it.
func TestKanbanCancelledColumnGoesLast(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 2, []string{"todo", "done"})
	before := columnsOf(m)

	moveToCancelled(t, m, m.tasks[0].ID)
	reloadTasks(t, m)

	after := columnsOf(m)
	if len(after) != len(before)+1 {
		t.Fatalf("columns %v -> %v, want one more", before, after)
	}
	if after[len(after)-1] != model.CancelledStatus {
		t.Errorf("the cancelled column ended up as %q, want the last", after[len(after)-1])
	}
	for i, c := range before {
		if after[i] != c {
			t.Errorf("the workflow was reordered: %q moved from position %d", c, i)
		}
	}
}

// moveToCancelled moves a task to "cancelled", the only status that is
// accepted even when it is not in the project's workflow. That is why a
// column of cancelled ones can exist without any project declaring it.
func moveToCancelled(t *testing.T, m *Model, id int64) {
	t.Helper()
	if _, err := m.database.MoveTask(id, model.CancelledStatus); err != nil {
		t.Fatalf("MoveTask(%d, cancelled): %v", id, err)
	}
	m.filterStatus = "" // the default filter leaves the cancelled ones out
}

// columnsOf returns the statuses of the board's columns, in order.
func columnsOf(m *Model) []string {
	cols := m.kanbanColumns()
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.status
	}
	return out
}

// A column's header says how many are seen and how many there are, and it
// only carries the pair when they do not all fit.
func TestKanbanHeaderCountsShownOverTotal(t *testing.T) {
	tests := []struct {
		name  string
		shown int
		total int
		want  string
	}{
		{"all fit", 3, 3, "─ todo (3) "},
		{"do not fit", 3, 9, "─ todo (3/9) "},
		{"none visible", 0, 9, "─ todo (0/9) "},
		{"empty column", 0, 0, "─ todo (0) "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kanbanHeader("todo", tt.shown, tt.total); got != tt.want {
				t.Errorf("kanbanHeader(%d, %d) = %q, want %q", tt.shown, tt.total, got, tt.want)
			}
		})
	}
}

// With more cards than rows fit, that column's header says how many are seen
// out of how many there are. It is the same shown/total as the previous case,
// but measured on the real board: without it, a misplaced "end - start" would go unnoticed.
func TestKanbanHeaderShowsWindowWhenOverflowing(t *testing.T) {
	m := newKanbanModel(t, 8)
	m.width = 240

	out := ansi.Strip(m.renderKanban(14))
	if !strings.Contains(out, "(2/8)") && !strings.Contains(out, "(1/8)") {
		t.Errorf("the header does not say how many of eight are shown:\n%s", out)
	}
	if strings.Contains(out, "(8/8)") {
		t.Errorf("with eight cards and a short height the header cannot say 8/8:\n%s", out)
	}
}

// When the cursor is at the end of a column longer than the window, the
// window scrolls and the header says how many are seen from the one that is
// selected, not from the first. It is the other use of "end - start": with
// the cursor at the top, start is zero and the pair tells one count from another.
func TestKanbanHeaderFollowsScrolledWindow(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 20, []string{"todo", "done"})
	m.width = 240
	m.kanbanCol = 0
	m.kanbanRow = 19 // at the bottom of todo
	m.clampKanbanCursor()

	out := ansi.Strip(m.renderKanban(14))
	want := fmt.Sprintf("(%d/20)", kanbanMaxCards(14))
	if !strings.Contains(out, want) {
		t.Errorf("the header does not say %q with the cursor at the end:\n%s", want, out)
	}
	if strings.Contains(out, "(1/20)") {
		t.Errorf("with the cursor down the window cannot still be on the first card:\n%s", out)
	}
	// And the shift is visible in the cards: the first one is gone and the last
	// one is there. The header does not tell, because the number of visibles is
	// the same shifted or not.
	if !strings.Contains(out, "task 19") {
		t.Errorf("with the cursor down the last card is not visible:\n%s", out)
	}
	if strings.Contains(out, "task 0 ") {
		t.Errorf("with the cursor down the first card is still on screen:\n%s", out)
	}
}

// And when they all fit, the header carries no pair.
func TestKanbanHeaderOmitsPairWhenAllVisible(t *testing.T) {
	m := newKanbanModel(t, 2)
	m.width = 240

	out := ansi.Strip(m.renderKanban(40))
	if strings.Contains(out, "(2/2)") {
		t.Errorf("with both cards visible the header should not carry the pair:\n%s", out)
	}
	if !strings.Contains(out, "todo (2)") {
		t.Errorf("the header does not say how many there are:\n%s", out)
	}
}

func hasColumn(m *Model, status string) bool {
	for _, c := range m.kanbanColumns() {
		if c.status == status {
			return true
		}
	}
	return false
}

// reloadProjects re-reads the projects, as a projectsLoadedMsg would.
func reloadProjects(t *testing.T, m *Model) {
	t.Helper()
	var err error
	if m.projects, err = m.database.ListProjects(); err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// newKanbanModel starts from a DB with a single project, the given workflow
// and n cards in "todo".
//
// Loading the projects and filtering by "only" matters: without the filter the
// board uses the union of all projects' workflows, and the auxiliary "api"
// project of newEmptyDBModel brings the default workflow, with seven statuses
// and "cancelled" inside. The test's columns would be the default's.
func newKanbanModel(t *testing.T, tasks int) *Model {
	t.Helper()
	return newKanbanModelWithWorkflow(t, tasks, []string{"todo", "done"})
}

func newKanbanModelWithWorkflow(t *testing.T, tasks int, workflow []string) *Model {
	t.Helper()
	m := newEmptyDBModel(t)
	mustCreateProject(t, m.database, "only", workflow)
	for i := range tasks {
		mustCreateTask(t, m.database, "only", fmt.Sprintf("task %d", i), "", "@john", 1, "todo")
	}
	reloadTasks(t, m)
	reloadProjects(t, m)
	// The project filter is what makes the board use the workflow of
	// "only" and not the union of all of them. Without it, the auxiliary "api"
	// project of newEmptyDBModel puts in its default workflow, which brings
	// "cancelled", and the column would always appear.
	// newEmptyDBModel leaves the people modal open, and that one keeps the keys
	// before the view receives any. Without closing it, "h" and "l" would never
	// reach the board.
	m.assigneeModalOpen = false
	m.filterProject = "only"
	m.currentView = viewKanban
	m.width = 240
	m.clampKanbanCursor()
	return m
}

// A column's card carries the priority if the column shows it, and only the
// title if not. And the assignee goes with its tags behind when it has them.
func TestKanbanCardContent(t *testing.T) {
	tags := []string{"bug", "urgent"}
	withTags := model.Task{ID: 1, Title: "fix", Assignee: "@john", Priority: 3, Tags: tags}

	tests := []struct {
		name         string
		task         model.Task
		showPriority bool
		wantPriority bool
		wantTags     bool
	}{
		{"with priority and no tags", model.Task{ID: 1, Title: "t", Assignee: "@john", Priority: 2}, true, true, false},
		{"without priority and no tags", model.Task{ID: 1, Title: "t", Assignee: "@john", Priority: 2}, false, false, false},
		{"with tags and with priority", withTags, true, true, true},
		{"with tags and without priority", withTags, false, false, true},
		{"without assignee", model.Task{ID: 1, Title: "t", Priority: 1}, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newKanbanModel(t, 0)
			col := kanbanColumn{status: "todo", tasks: []model.Task{tt.task}, showPriority: tt.showPriority}

			out := ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", false, columnWindow{end: 1}))
			// The priority character comes with color, so it is compared without it:
			// the render is also measured without it.
			bar := ansi.Strip(priorityChar(tt.task.Priority))
			if has := strings.Contains(out, bar); has != tt.wantPriority {
				t.Errorf("the priority bar %q shows=%v, want %v:\n%s",
					bar, has, tt.wantPriority, out)
			}
			// The tags go two columns behind the assignee, not glued. Without that
			// gap the names read as one block and the "> 0 tags" of the counter
			// stops starting at a fixed column.
			withGap := "@john  " + strings.Join(tags, ",")
			if has := strings.Contains(out, withGap); has != tt.wantTags {
				t.Errorf("the gap with the tags shows=%v, want %v:\n%s", has, tt.wantTags, out)
			}
			if has := strings.Contains(out, strings.Join(tags, ",")); has != tt.wantTags {
				t.Errorf("the tags show=%v, want %v:\n%s", has, tt.wantTags, out)
			}
			if !strings.Contains(out, "t") && !strings.Contains(out, "fix") {
				t.Errorf("the title does not show:\n%s", out)
			}
		})
	}
}

// The selected card carries "> " in front and the others "  ". It is the only
// difference between them, so what is compared is the prefix of the first
// line of the card, not its text: the text carries the priority bar with
// color and comparing on the colorless render does not make it clear.
func TestKanbanSelectedCardIsMarked(t *testing.T) {
	m := newKanbanModel(t, 0)
	task := model.Task{ID: 1, Title: "task", Assignee: "@john", Priority: 2}
	// With showPriority the card already brings its own two-column prefix, so
	// the selection prefix is told apart from the card's. Without the bar,
	// the two prefixes overlap and removing them would be invisible.
	col := kanbanColumn{status: "todo", tasks: []model.Task{task}, showPriority: true}

	selected := firstCard(t, ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", true, columnWindow{end: 1})), "task")
	if !strings.HasPrefix(selected, "> ") {
		t.Errorf("the selected card starts with %q, want \"> \"", selected)
	}

	notSelected := firstCard(t, ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", false, columnWindow{end: 1})), "task")
	if strings.HasPrefix(notSelected, "> ") {
		t.Errorf("an unselected card starts with %q", notSelected)
	}
	if !strings.HasPrefix(notSelected, "    ") {
		t.Errorf("an unselected card does not carry the two-column prefix: %q", notSelected)
	}
}

// With two cards and the cursor on the second one, only that one carries the mark.
func TestKanbanOnlyCursorRowIsMarked(t *testing.T) {
	m := newKanbanModel(t, 0)
	m.kanbanRow = 1
	col := kanbanColumn{status: "todo", tasks: []model.Task{
		{ID: 1, Title: "first", Assignee: "@john", Priority: 2},
		{ID: 2, Title: "second", Assignee: "@john", Priority: 2},
	}}

	out := ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", true, columnWindow{end: 2}))
	if n := strings.Count(out, "> "); n != 1 {
		t.Errorf("there are %d marks, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "second") {
		t.Errorf("the second card does not show:\n%s", out)
	}
}

// firstCard returns the first line containing the text of a card,
// with the column's left border removed. The rest of the box -- the box,
// the header -- is skipped by looking for the line carrying the text.
//
// All by runes: the borders and the dashes are multibyte, and a byte cut
// splits the character and returns garbage.
func firstCard(t *testing.T, rendered, title string) string {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		i := strings.Index(line, title)
		if i < 0 {
			continue
		}
		runes := []rune(line[:i])
		// Outside the left border: "║", "│", "╔" or "╭".
		for len(runes) > 0 && strings.ContainsRune("║│╔╭", runes[0]) {
			runes = runes[1:]
		}
		return string(runes)
	}
	t.Fatalf("the column does not have the card %q:\n%s", title, rendered)
	return ""
}

// A column with no cards says "(empty)" instead of overflowing with an empty box.
func TestKanbanEmptyColumnSaysSo(t *testing.T) {
	m := newKanbanModel(t, 0)
	col := kanbanColumn{status: "todo", tasks: nil}

	out := ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", false, columnWindow{}))
	if !strings.Contains(out, "(empty)") {
		t.Errorf("an empty column does not say so:\n%s", out)
	}
}
