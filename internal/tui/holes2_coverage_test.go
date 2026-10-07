package tui

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

// A second round on the interface: here live the paths where a write works
// and the read behind it fails, which is the pattern the read-only database
// cannot reach. To provoke them the table has to be left with the type that
// the Scan expects changed -- the tables are not STRICT, so the trick is not
// a weird value but removing the autoincrement: a row with a null id cannot
// be scanned into an int64.

// sabotageCommentsLeavesRowUnreadable turns the comments table into one without
// a primary key and inserts a row with a null id, so that any read fails.
// The writes keep working: that is why it serves for the pattern
// "I write and then cannot read".
func sabotageCommentsLeavesRowUnreadable(t *testing.T, m *Model, taskID int64) {
	t.Helper()
	conn := m.database.Conn()
	if _, err := conn.Exec(`DROP TABLE comments`); err != nil {
		t.Fatalf("DROP TABLE comments: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE comments (
		id TEXT, task_id INTEGER NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("CREATE TABLE comments: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO comments (id, task_id, body, created_at) VALUES (NULL, ?, 'unreadable', '2020-01-01')`,
		taskID,
	); err != nil {
		t.Fatalf("INSERT INTO comments: %v", err)
	}
	// And one with a valid id, so that deleting a comment still has something
	// to delete: what is meant to fail is the READ after, not the deletion.
	if _, err := conn.Exec(
		`INSERT INTO comments (id, task_id, body, created_at) VALUES ('7', ?, 'real', '2020-01-01')`,
		taskID,
	); err != nil {
		t.Fatalf("INSERT INTO comments: %v", err)
	}
}

// sabotageTasksLeavesRowUnreadable does the same with tasks, for the paths that
// write a task and reload the listing behind.
func sabotageTasksLeavesRowUnreadable(t *testing.T, m *Model, projectID int64) {
	t.Helper()
	conn := m.database.Conn()
	if _, err := conn.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("DROP TABLE tasks: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE tasks (
		id TEXT, project_id INTEGER NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'backlog', priority INTEGER NOT NULL DEFAULT 0,
		assignee TEXT NOT NULL DEFAULT '', position INTEGER NOT NULL DEFAULT 0,
		estimate REAL NOT NULL DEFAULT 0, tags TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')), completed_at TEXT)`); err != nil {
		t.Fatalf("CREATE TABLE tasks: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO tasks (id, project_id, title) VALUES (NULL, ?, 'unreadable')`, projectID,
	); err != nil {
		t.Fatalf("INSERT INTO tasks: %v", err)
	}
}

func projectIDOf(t *testing.T, m *Model) int64 {
	t.Helper()
	if len(m.projects) == 0 {
		t.Fatal("the model has no projects loaded")
	}
	return m.projects[0].ID
}

// Saving a comment and loading the list are two consecutive operations. If
// the write goes and the read does not, the message comes out with an empty
// list and no selection, which is what keeps the interface from selecting an impossible index.
func TestCommentCmdsWhenTheReadAfterTheWriteFails(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		m := newDetailModel(t, 0)
		taskID := m.detailTask.ID
		sabotageCommentsLeavesRowUnreadable(t, m, taskID)

		msg := mustMsg(t, m.addCommentCmd(taskID, "new"))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentsLoadedMsg", msg)
		}
		if len(loaded.comments) != 0 || loaded.selectIdx != -1 {
			t.Errorf("got %d comments and sel=%d, want 0 and -1",
				len(loaded.comments), loaded.selectIdx)
		}
	})

	t.Run("delete", func(t *testing.T) {
		m := newDetailModel(t, 2)
		taskID := m.detailTask.ID
		sabotageCommentsLeavesRowUnreadable(t, m, taskID)

		// Id 7 is the one the sabotage inserted: it is the deletable comment.
		msg := mustMsg(t, m.deleteCommentCmd(taskID, 7, 1))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentsLoadedMsg", msg)
		}
		// The deletion did work: the message must not keep the selection that
		// is kept only for the deletion failure case.
		if loaded.selectIdx != -1 {
			t.Errorf("selectIdx = %d, want -1", loaded.selectIdx)
		}
	})
}

// The three paths where something is written and then the listing is
// reloaded: the task form, the inline description save and the external
// editor save. With the read broken there is no reload, and the model keeps what it had.
func TestWriteThenReloadWhenTheReloadFails(t *testing.T) {
	t.Run("task creation", func(t *testing.T) {
		m := newTestModel(t)
		open, _ := pressKeys(t, m, "i")
		withTitle, _ := pressKeys(t, open, "n", "e", "w")

		sabotageTasksLeavesRowUnreadable(t, withTitle, projectIDOf(t, withTitle))

		cmd := withTitle.createTaskCmd(
			withTitle.currentProjectName(), "new", "", "Me", model.PriorityLow, nil)
		if cmd == nil {
			t.Fatal("creating the task did not launch any command")
		}
		if msg := cmd(); msg != nil {
			t.Errorf("with the listing unreadable got %T, want nil", msg)
		}
	})

	t.Run("inline description", func(t *testing.T) {
		m := newTestModel(t)
		id := m.tasks[0].ID
		sabotageTasksLeavesRowUnreadable(t, m, projectIDOf(t, m))

		if msg := m.saveDescriptionCmd(id, "new description")(); msg != nil {
			t.Errorf("with the listing unreadable got %T, want nil", msg)
		}
	})

	t.Run("external editor", func(t *testing.T) {
		m := newTestModel(t)
		id := m.tasks[0].ID
		sabotageTasksLeavesRowUnreadable(t, m, projectIDOf(t, m))

		if msg := m.updateTaskFromEdit(id, "Title: other\n")(); msg != nil {
			t.Errorf("with the listing unreadable got %T, want nil", msg)
		}
	})
}

// Adding a tag writes in two places: the tasks_tags table and, on some
// paths, the task itself. With the task's read broken, the whole command is
// left without a message.
func TestToggleTagWhenTheWriteFails(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_ = task

	if msg := m.toggleTagCmd(1, "new")(); msg != nil {
		t.Errorf("with the database closed got %T, want nil", msg)
	}
}

// The people modal has a second level with its own keys. With no free days
// the navigation has nothing to walk and "a" keeps working; with free days,
// the whole key set applies.
func TestAssigneeDetailKeys(t *testing.T) {
	m := newAssigneeModel(t, 3)
	m.assigneeModalOpen = true
	m.currentView = viewList
	m.assigneeDetail = true

	name := m.currentAssignee()
	if name == "" {
		t.Fatal("the fixture left no person selected")
	}
	for _, day := range []string{"2026-10-05", "2026-10-12", "2026-10-19"} {
		mustAddOffDay(t, m.database, name, day, day, "bridge")
	}
	reloadOffDays(t, m)

	offs := m.assigneeOffDays(name)
	if len(offs) < 3 {
		t.Fatalf("person %q has %d free days, want 3", name, len(offs))
	}

	down, _ := pressKeys(t, m, "j")
	if down.assigneeOffdayIdx != 1 {
		t.Errorf("j left the off-day at %d, want 1", down.assigneeOffdayIdx)
	}
	down2, _ := pressKeys(t, down, "j")
	if down2.assigneeOffdayIdx != 2 {
		t.Errorf("j from 1 left the off-day at %d, want 2", down2.assigneeOffdayIdx)
	}
	up, _ := pressKeys(t, down2, "k")
	if up.assigneeOffdayIdx != 1 {
		t.Errorf("k left the off-day at %d, want 1", up.assigneeOffdayIdx)
	}

	// "a" opens the creation of a free day for the focused person.
	opened, cmd := pressKeys(t, up, "a")
	if !opened.offdayFormOpen {
		t.Error("a did not open the free-day form")
	}
	_ = cmd

	// x asks to confirm the deletion, same as d.
	confirmed, _ := pressKeys(t, up, "x")
	if !confirmed.confirmOpen || confirmed.confirmAction != "delete-offday" {
		t.Errorf("x did not ask for confirmation: open=%v action=%q",
			confirmed.confirmOpen, confirmed.confirmAction)
	}

	// esc leaves the detail without closing the whole modal.
	back, _ := pressKeys(t, up, "esc")
	if back.assigneeDetail {
		t.Error("esc did not leave the person detail")
	}
	if !back.assigneeModalOpen {
		t.Error("esc in the detail closed the whole modal")
	}
}

// The option cycle of the filter modal cannot report an impossible index
// when the field has none. The only such field is one that does not exist,
// which is exactly what happens if the number of fields and the switch get
// out of sync.
func TestFilterCycleWithAnUnknownField(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = 99

	if opts := m.filterVisibleOptions(); len(opts) != 0 {
		t.Fatalf("a non-existent field has %d options (%v), want 0", len(opts), opts)
	}

	before := m.filterOptionIdx
	for _, forward := range []bool{true, false} {
		m.filterCycle(forward)
		if m.filterOptionIdx != before {
			t.Errorf("with a field without options the index moved to %d, want %d",
				m.filterOptionIdx, before)
		}
	}
}

// A view's name is what appears in the bar and in the help. A view that
// does not exist has to say so, not print an empty integer.
func TestUnknownViewHasAName(t *testing.T) {
	if got := viewKind(42).String(); got != "?" {
		t.Errorf("a non-existent view is named %q, want %q", got, "?")
	}
}

// Editing a project with an empty list order leaves it as it is; with an
// order, it changes it. Two halves of the same command, and the second one
// was not running.
func TestProjectEditWithAListOrder(t *testing.T) {
	m := newDashModel(t, "api")
	original := m.projects[0].Workflow

	msg := mustMsg(t, m.saveProjectCmd(true, "api", "api", "", "done,backlog,todo"))
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("the message is %T, want projectSavedMsg", msg)
	}
	if saved.err != nil {
		t.Fatalf("editing with a valid order failed: %v", saved.err)
	}

	reloaded, err := m.database.GetProject("api")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if len(reloaded.ListOrder) != 3 {
		t.Errorf("the list order is %v, want three states", reloaded.ListOrder)
	}
	// The workflow comes empty in the call, so it must not have changed.
	if len(reloaded.Workflow) != len(original) {
		t.Errorf("the workflow went from %v to %v with an empty order", original, reloaded.Workflow)
	}
}

// h in the kanban changes column and resets the row, because the row index
// is per column. Staying on the previous column while leaving the index where
// it was would make the selection point at another task.
func TestKanbanColumnChangeResetsTheRow(t *testing.T) {
	m := newKanbanModel(t, 6)
	m.currentView = viewKanban

	cols := len(m.kanbanColumns())
	if cols < 2 {
		t.Skip("the fixture needs two columns")
	}

	m.kanbanCol = 1
	m.kanbanRow = 2

	left, _ := pressKeys(t, m, "h")
	if left.kanbanCol != 0 {
		t.Errorf("h left the column at %d, want 0", left.kanbanCol)
	}
	if left.kanbanRow != 0 {
		t.Errorf("h did not reset the row: %d, want 0", left.kanbanRow)
	}
}

// The gantt's task only exists if the cursor falls on a task row: a
// person's header is not a task, and a view with no rows either.
func TestSelectedTaskInTheGantt(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john", "@margo"}, 3)
	m.currentView = viewGantt
	m.width, m.height = 140, 40

	rows := m.ganttRows()
	if len(rows) == 0 {
		t.Fatal("the fixture left no rows")
	}

	taskIdx, headerIdx := -1, -1
	for i, f := range rows {
		switch {
		case f.kind == ganttTaskRow && taskIdx < 0:
			taskIdx = i
		case f.kind == ganttAssigneeRow && headerIdx < 0:
			headerIdx = i
		}
	}
	if taskIdx < 0 || headerIdx < 0 {
		t.Fatalf("the fixture left neither of the two row types: %d rows", len(rows))
	}

	m.ganttCursor = taskIdx
	tsk := m.selectedTask()
	if tsk == nil {
		t.Fatalf("with the cursor on row %d (a task) there is no selected task", taskIdx)
	}
	if tsk.ID != rows[taskIdx].entry.Task.ID {
		t.Errorf("the selected task is %d, want %d", tsk.ID, rows[taskIdx].entry.Task.ID)
	}

	m.ganttCursor = headerIdx
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("with the cursor on a header there is a task: %q", tsk.Title)
	}

	m.ganttCursor = len(rows) + 5
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("with the cursor out of range there is a task: %q", tsk.Title)
	}

	// The exact edge, which is what separates `< len(rows)` from `<= len(rows)`:
	// with the cursor on the row right after the last there is nothing, even
	// though it is "almost" a valid index.
	m.ganttCursor = len(rows)
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("with the cursor one row past the end there is a task: %q", tsk.Title)
	}

	// And the other edge, below.
	m.ganttCursor = -1
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("with the cursor at -1 there is a task: %q", tsk.Title)
	}
}

// The project loading is two queries. If the archived one fails, the
// program starts anyway with the active one: losing the archived panel is
// better than not starting.
func TestLoadProjectsWithAnUnreadableArchive(t *testing.T) {
	m := newTestModel(t)

	conn := m.database.Conn()
	if _, err := conn.Exec(`DROP TABLE projects`); err != nil {
		t.Fatalf("DROP TABLE projects: %v", err)
	}
	// A view with types that cannot be scanned: the active query will select
	// rows, the archived one will fail converting them.
	if _, err := conn.Exec(`CREATE TABLE projects (
		id BLOB, name BLOB, workflow BLOB, list_order BLOB, archived BLOB)`); err != nil {
		t.Fatalf("CREATE TABLE projects: %v", err)
	}
	for _, archived := range []int{0, 1} {
		if _, err := conn.Exec(
			`INSERT INTO projects VALUES (x'00ff', x'00ff', x'00ff', x'00ff', ?1)`, archived,
		); err != nil {
			t.Fatalf("INSERT INTO projects: %v", err)
		}
	}

	msg := mustMsg(t, m.loadProjects())
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("the message is %T, want projectsLoadedMsg", msg)
	}
	if loaded.archivedProjects != nil {
		t.Errorf("with the archived table unreadable there are %d archived, want nil",
			len(loaded.archivedProjects))
	}
}

// Row 0 of a gantt with rows is always a person HEADER, never a task:
// ganttRows emits the header before that person's entries, and it does not
// emit the header of anyone who has no entries.
//
// That is what lets selectedTask settle for the rest of the range without
// the comparison: with the cursor at 0, row 0 is not a task, so the
// comparison with the lower bound's zero gives the same as a "> 0". The test
// pins the invariant so that, if some day ganttRows stops meeting it, it
// shows here and not in a mutant that survives without explanation.
func TestGanttRowZeroIsAHeader(t *testing.T) {
	for _, people := range [][]string{
		{"@john"},
		{"@john", "@margo"},
		nil, // with no people there is neither header nor task
	} {
		t.Run(strings.Join(people, "+"), func(t *testing.T) {
			m := ganttModelWithPeople(t, people, 3)
			m.currentView = viewGantt
			m.width, m.height = 140, 40

			rows := m.ganttRows()
			if len(rows) == 0 {
				if len(people) > 0 {
					t.Fatal("there are people but no rows")
				}
				return
			}
			if rows[0].kind != ganttAssigneeRow {
				t.Errorf("row 0 is %v, want a person header: selectedTask's rejection of cursor 0 "+
					"without checking the lower bound depends on it", rows[0].kind)
			}

			// And with the cursor there there is no selected task, which is the
			// same as a "> 0" would say in the code.
			m.ganttCursor = 0
			if tsk := m.selectedTask(); tsk != nil {
				t.Errorf("with the cursor on row 0 there is a task: %q", tsk.Title)
			}
		})
	}
}

// nextGanttTaskRow returns -1 when there is no task row from the point on.
// From the cursor it is impracticable -- there would be a broken invariant --
// but as a pure function the case is reachable, and it is the one that
// returns the value snapGanttCursor keeps when it finds nothing.
func TestNextGanttTaskRowWithoutTasks(t *testing.T) {
	cases := []struct {
		name string
		rows []ganttRow
		from int
	}{
		{"empty list", nil, 0},
		{"empty list with negative cursor", nil, -1},
		{"only headers", []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttAssigneeRow}}, 0},
		{"cursor beyond the end", []ganttRow{{kind: ganttTaskRow}}, 5},
		{"from the last row", []ganttRow{{kind: ganttTaskRow}}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nextGanttTaskRow(c.rows, c.from); got != -1 {
				t.Errorf("nextGanttTaskRow(%d rows, from=%d) = %d, want -1", len(c.rows), c.from, got)
			}
		})
	}

	// And the normal case: it finds the next task, skipping the headers.
	rows := []ganttRow{
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
	}
	for from, want := range map[int]int{0: 1, 1: 1, 2: 2, 3: 4} {
		if got := nextGanttTaskRow(rows, from); got != want {
			t.Errorf("nextGanttTaskRow(from=%d) = %d, want %d", from, got, want)
		}
	}

	// And with a negative cursor it searches from the start, which is what
	// keeps a slice with a negative index.
	if got := nextGanttTaskRow(rows, -3); got != 1 {
		t.Errorf("nextGanttTaskRow(from=-3) = %d, want 1: it must start at the first row", got)
	}
}

// stepGanttCursor is the gantt's jump arithmetic, taken out of the walk so
// that it can be checked with any cursor, including the ones the real walk
// never produces (negative, beyond the end).
//
// The edges of that arithmetic are exactly what the spread walk did not let
// you see: the clamp's floor at 0, the +1 of the forward offset, and the
// count from the end backwards.
func TestStepGanttCursor(t *testing.T) {
	// Header, task, task, header, task.
	rows := []ganttRow{
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
	}

	t.Run("forwards", func(t *testing.T) {
		cases := []struct{ from, want int }{
			{0, 1}, // from a header
			{1, 2}, // from a task
			{3, 4}, // from a header with a task behind it
			{2, 4}, // skips the intermediate header
		}
		for _, c := range cases {
			got, ok := stepGanttCursor(rows, c.from, 1)
			if !ok {
				t.Errorf("stepGanttCursor(from=%d, +1) says there is no jump", c.from)
				continue
			}
			if got != c.want {
				t.Errorf("stepGanttCursor(from=%d, +1) = %d, want %d", c.from, got, c.want)
			}
		}
	})

	t.Run("backwards", func(t *testing.T) {
		cases := []struct{ from, want int }{
			{4, 2},
			{3, 2},
			{2, 1},
		}
		for _, c := range cases {
			got, ok := stepGanttCursor(rows, c.from, -1)
			if !ok {
				t.Errorf("stepGanttCursor(from=%d, -1) says there is no jump", c.from)
				continue
			}
			if got != c.want {
				t.Errorf("stepGanttCursor(from=%d, -1) = %d, want %d", c.from, got, c.want)
			}
		}
	})

	t.Run("out-of-range cursors", func(t *testing.T) {
		// The clamp is what keeps the slice from being cut the other way around.
		// With a negative cursor, the floor at 0 and not at -1 is what decides.
		for _, from := range []int{-1, -5, -100} {
			if got, ok := stepGanttCursor(rows, from, 1); !ok || got != 1 {
				t.Errorf("stepGanttCursor(from=%d, +1) = (%d, %v), want (1, true): the clamp must start at 0",
					from, got, ok)
			}
			if _, ok := stepGanttCursor(rows, from, -1); ok {
				t.Errorf("stepGanttCursor(from=%d, -1) says there is a jump: there is nothing above 0",
					from)
			}
		}
		// And with the cursor beyond the end: the ceiling is len(rows), not
		// len(rows)-1, because the destination of "forward" is cursor+1.
		for _, from := range []int{5, 6, 100} {
			if got, ok := stepGanttCursor(rows, from, 1); ok {
				t.Errorf("stepGanttCursor(from=%d, +1) = (%d, true), want no jump", from, got)
			}
			if got, ok := stepGanttCursor(rows, from, -1); !ok || got != 4 {
				t.Errorf("stepGanttCursor(from=%d, -1) = (%d, %v), want (4, true): the ceiling is the length",
					from, got, ok)
			}
		}
	})

	t.Run("nowhere to jump to", func(t *testing.T) {
		headersOnly := []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttAssigneeRow}}
		if got, ok := stepGanttCursor(headersOnly, 0, 1); ok {
			t.Errorf("no tasks, +1 = (%d, true), want no jump", got)
		}
		if got, ok := stepGanttCursor(headersOnly, 1, -1); ok {
			t.Errorf("no tasks, -1 = (%d, true), want no jump", got)
		}
		if got, ok := stepGanttCursor(nil, 0, 1); ok {
			t.Errorf("empty list, +1 = (%d, true), want no jump", got)
		}
	})

	t.Run("dir zero goes forward", func(t *testing.T) {
		// dir == 0 is not "do not move": it enters the "forward" branch because
		// it is not negative, and the destination is from+1. That is what
		// separates `dir < 0` from `dir <= 0`: with the `<=`, zero dir would
		// enter the back branch and return -1 instead of the next row's index.
		for _, from := range []int{0, 1, 2, 3} {
			got, gotOK := stepGanttCursor(rows, from, 0)
			want, wantOK := stepGanttCursor(rows, from, 1)
			if got != want || gotOK != wantOK {
				t.Errorf("stepGanttCursor(from=%d, 0) = (%d, %v), want (%d, %v) (same as +1)",
					from, got, gotOK, want, wantOK)
			}
		}
		// And the cases where "forward" finds nothing: zero dir has to say the
		// same thing: there is no jump.
		if got, ok := stepGanttCursor(rows, 4, 0); ok {
			t.Errorf("stepGanttCursor(from=4, 0) = (%d, true), want no jump", got)
		}
		if got, ok := stepGanttCursor(rows, 99, 0); ok {
			t.Errorf("stepGanttCursor(from=99, 0) = (%d, true), want no jump", got)
		}
	})
}

// clamp on the three sides: the floor, the ceiling and the middle.
func TestClamp(t *testing.T) {
	cases := []struct{ v, lo, hi, want int }{
		{-5, 0, 10, 0},  // below the floor
		{0, 0, 10, 0},   // exactly the floor
		{3, 0, 10, 3},   // in the middle
		{10, 0, 10, 10}, // exactly the ceiling
		{15, 0, 10, 10}, // above the ceiling
		{-1, -5, 5, -1}, // negative floor
		{9, 5, 5, 5},    // lo == hi
		{1, 3, 3, 3},    // lo > v, hi == lo
	}
	for _, c := range cases {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}

// moveGanttCursor with a jump that does not exist: the sentinel has to leave
// the cursor where it is.
//
// stepGanttCursor returns ganttNoTasks when there is no task row to jump to,
// and moveGanttCursor has to tell that -1 apart from a real index.
// With `>= 0` instead of `!= ganttNoTasks` the two forms are the same
// condition over integers, so its mutant could not be killed. With the named
// sentinel the comparison says what it compares.
func TestMoveGanttCursorWithoutJumpDoesNotMoveTheCursor(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john"}, 2)
	m.currentView = viewGantt
	m.width, m.height = 140, 40
	rows := m.ganttRows()

	// Two cases with no destination: the cursor on the last row going forward,
	// and the cursor on the first going backwards.
	cases := []struct {
		name     string
		position int
		dir      int
	}{
		{"first row backwards", 0, -1},
		{"last row forwards", len(rows) - 1, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			local := *m
			local.ganttCursor = c.position
			// That there really is no jump.
			if got, ok := stepGanttCursor(rows, c.position, c.dir); ok {
				t.Fatalf("the fixture is useless: stepGanttCursor(%d, %d) = (%d, true), want no jump",
					c.position, c.dir, got)
			}

			moveGanttCursor(&local, rows, c.dir)

			if local.ganttCursor != c.position {
				t.Errorf("the cursor moved to %d with no destination; it was at %d",
					local.ganttCursor, c.position)
			}
		})
	}

	// And the normal case, so that the test does not pass by doing nothing:
	// with a destination it does move, and to the exact place stepGanttCursor says.
	local := *m
	local.ganttCursor = len(rows) - 2
	want, ok := stepGanttCursor(rows, local.ganttCursor, 1)
	if !ok {
		t.Fatal("the fixture is useless: there is a destination")
	}
	moveGanttCursor(&local, rows, 1)
	if local.ganttCursor != want {
		t.Errorf("the cursor went to %d, want %d", local.ganttCursor, want)
	}
}
