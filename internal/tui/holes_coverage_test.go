package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/model"
)

// The two external editor callbacks, already taken out of their tea.Cmd, have
// three outcomes each. What is checked here is that the message that comes
// out carries the task id, because without it the interface does not know who
// the result belongs to and the comment is lost.

func TestCommentCallback(t *testing.T) {
	t.Run("the editor failed", func(t *testing.T) {
		path := writeTemp(t, "content")
		msg := commentCallback(7, path)(errActionFailed)

		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("a failed editor did not produce an error")
		}
		if finished.taskID != 7 {
			t.Errorf("taskID = %d, want 7", finished.taskID)
		}
		if finished.body != "" {
			t.Errorf("body = %q, want empty when the editor failed", finished.body)
		}
	})

	t.Run("the editor wrote", func(t *testing.T) {
		path := writeTemp(t, "  a comment  \n\n")
		msg := commentCallback(7, path)(nil)

		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentFinishedMsg", msg)
		}
		if finished.err != nil {
			t.Fatalf("there is an error: %v", finished.err)
		}
		// Truncated: a comment neither starts nor ends with whitespace.
		if finished.body != "a comment" {
			t.Errorf("body = %q, want %q trimmed", finished.body, "a comment")
		}
		if finished.taskID != 7 {
			t.Errorf("taskID = %d, want 7", finished.taskID)
		}
	})

	t.Run("the temp file is gone", func(t *testing.T) {
		msg := commentCallback(7, t.TempDir()+"/never-was")(nil)
		if finished := msg.(commentFinishedMsg); finished.err == nil {
			t.Error("a missing temp file did not produce an error")
		}
	})
}

func TestEditorCallback(t *testing.T) {
	t.Run("the editor failed", func(t *testing.T) {
		path := writeTemp(t, "something")
		msg := editorCallback(9, path)(errActionFailed)

		finished, ok := msg.(editorFinishedMsg)
		if !ok {
			t.Fatalf("the message is %T, want editorFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("a failed editor did not produce an error")
		}
		if finished.taskID != 9 {
			t.Errorf("taskID = %d, want 9", finished.taskID)
		}
		if finished.file != "" {
			t.Errorf("file = %q, want empty when the editor failed", finished.file)
		}
	})

	t.Run("the editor wrote", func(t *testing.T) {
		// The external editor does NOT truncate: the format it generates carries
		// separators and line breaks that the parser needs.
		path := writeTemp(t, "Title: new\n---\nStatus: todo\n")
		msg := editorCallback(9, path)(nil)

		finished := msg.(editorFinishedMsg)
		if finished.err != nil {
			t.Fatalf("there is an error: %v", finished.err)
		}
		if !strings.Contains(finished.file, "Title: new") {
			t.Errorf("file = %q, want the content as it is", finished.file)
		}
	})

	t.Run("the temp file is gone", func(t *testing.T) {
		msg := editorCallback(9, t.TempDir()+"/never-was")(nil)
		if finished := msg.(editorFinishedMsg); finished.err == nil {
			t.Error("a missing temp file did not produce an error")
		}
	})
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/edited.md"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The project loading has two queries: the active ones and the archived
// ones. If the second one fails, the program continues with the first:
// losing the archived panel is much better than not starting.
func TestLoadProjectsSurvivesABrokenArchivedQuery(t *testing.T) {
	m := newTestModel(t)

	sabotageProjects(t, m)

	msg := mustMsg(t, m.loadProjects())
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("the message is %T, want projectsLoadedMsg", msg)
	}
	if loaded.archivedProjects != nil {
		t.Errorf("with the projects table broken there are %d archived, want nil",
			len(loaded.archivedProjects))
	}
}

// Adding and removing a tag are two writes; if the DB fails, the command
// returns nil instead of a message with half-done data.
func TestToggleTagWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, tag := range []string{"new", ""} {
		if msg := m.toggleTagCmd(m.tasks[0].ID, tag)(); msg != nil {
			t.Errorf("toggleTag(%q) with the DB closed returned %T, want nil", tag, msg)
		}
	}
}

// Resizing with the task form open has to readjust the textarea. Without
// that, the textarea keeps the width it was built with and looks skewed as
// soon as the window changes size.
func TestWindowResizeAdjustsTheNewTaskTextarea(t *testing.T) {
	m := newTestModel(t)
	open, _ := pressKeys(t, m, "i")

	open.width = 200
	resized, _ := updateMsg(t, open, tea.WindowSizeMsg{Width: 200, Height: 40})

	if resized.newTaskTextarea.Width() != resized.newTaskTextareaWidth() {
		t.Errorf("the textarea measures %d, want the recalculated width %d",
			resized.newTaskTextarea.Width(), resized.newTaskTextareaWidth())
	}
	if resized.statusbar.width != 200 {
		t.Errorf("the status bar measures %d, want 200", resized.statusbar.width)
	}
	if resized.preview.width != 200 {
		t.Errorf("the description box measures %d, want 200", resized.preview.width)
	}
}

// The paste goes first to the description editor, then to the form, then to
// the tag modal. With two of them open at once, the order decides which wins.
func TestPasteGoesToTheDescriptionEditorFirst(t *testing.T) {
	m := newTestModel(t)
	// The editor really opens and on top of it the form opens: with both
	// mounted at once, the paste has to decide which one it goes to.
	withEditor, _ := pressKeys(t, m, "e")
	if !withEditor.descEditOpen {
		t.Fatal("e did not open the description editor")
	}
	withEditor.newTaskOpen = true

	next, _ := updateMsg(t, withEditor, tea.PasteMsg{Content: "pasted"})
	if !strings.Contains(next.descEditTextarea.Value(), "pasted") {
		t.Errorf("the paste did not reach the description editor: %q", next.descEditTextarea.Value())
	}
	if strings.Contains(next.newTaskTitle, "pasted") {
		t.Error("the paste also reached the new task form")
	}
}

// Any message that is not from Bubbletea goes to the description editor if it
// is open, or to the form's textarea if the cursor is on the description field.
// It is the mechanism that lets the textareas speak on their own.
func TestUnknownMessagesReachTheOpenEditor(t *testing.T) {
	t.Run("the description editor", func(t *testing.T) {
		m := newTestModel(t)
		m.descEditOpen = true

		next, _ := updateMsg(t, m, privateMsg{})
		if !next.descEditOpen {
			t.Error("an unknown message closed the description editor")
		}
	})

	t.Run("the form textarea on the description field", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldDescription

		next, cmd := updateMsg(t, m, privateMsg{})
		if cmd != nil {
			t.Error("an unknown message emitted a textarea command")
		}
		if !next.newTaskOpen {
			t.Error("an unknown message closed the form")
		}
	})

	t.Run("nothing open", func(t *testing.T) {
		m := newTestModel(t)
		next, cmd := updateMsg(t, m, privateMsg{})
		if cmd != nil {
			t.Error("an unknown message with nothing open emitted a command")
		}
		if !strings.Contains(ansi.Strip(next.View().Content), "Fix") {
			t.Error("an unknown message changed the render")
		}
	})
}

// privateMsg is a type no Update switch knows: it represents the private
// package messages the textareas emit.
type privateMsg struct{}

func (privateMsg) String() string { return "private" }

// A view that does not exist must not leave the TUI painting nothing: it
// falls to the dashboard.
func TestUnknownViewFallsBack(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewKind(42)

	if _, cmd := pressKeys(t, m, "j"); cmd != nil {
		t.Error("a key in a non-existent view emitted a command")
	}

	m.width, m.height = 100, 30
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Total") {
		t.Errorf("a non-existent view did not fall back to the dashboard:\n%s", out)
	}
}

// In the kanban, h and l do not wrap: they stay at the edge. That is what
// tells shiftIndex from cycleIndex.
func TestKanbanColumnsDoNotWrap(t *testing.T) {
	m := newKanbanModel(t, 3)
	m.currentView = viewKanban
	cols := len(m.kanbanColumns())
	if cols < 2 {
		t.Skip("the fixture needs two columns")
	}

	m.kanbanCol = 0
	left, _ := pressKeys(t, m, "h")
	if left.kanbanCol != 0 {
		t.Errorf("h in the first column went to %d, want to stay at 0", left.kanbanCol)
	}

	m.kanbanCol = cols - 1
	right, _ := pressKeys(t, m, "l")
	if right.kanbanCol != cols-1 {
		t.Errorf("l in the last column went to %d, want to stay at %d",
			right.kanbanCol, cols-1)
	}
}

// S moves the card backwards in ITS project's workflow. It is the path that
// is not pressed today and the one that breaks the most when the project's
// workflow changes.
func TestKanbanMoveLeft(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 4, []string{"backlog", "todo", "doing", "done"})
	m.currentView = viewKanban

	// Put the cursor on a card that is not in the first status.
	cols := m.kanbanColumns()
	col := -1
	for i, c := range cols {
		if len(c.tasks) > 0 {
			col = i
			break
		}
	}
	if col < 0 {
		t.Skip("the fixture left no cards")
	}
	m.kanbanCol = col
	m.kanbanRow = 0

	id := cols[col].tasks[0].ID
	status := cols[col].tasks[0].Status

	_, cmd := pressKeys(t, m, "S")
	if cmd == nil {
		t.Fatal("S did not emit any action")
	}
	mustRun(t, cmd)

	task, err := m.database.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", id, err)
	}
	if task.Status == status {
		t.Errorf("the task is still at %q after moving it left", status)
	}
	if prev, _ := model.PrevStatus(m.projects[0].Workflow, status); task.Status != prev {
		t.Errorf("the task ended at %q, want the previous status %q", task.Status, prev)
	}
}

// The comment from the detail opens the external editor on the open task.
// With the task open and with no comments, the key has to launch the
// command.
func TestDetailOpensTheCommentEditor(t *testing.T) {
	m := newDetailModel(t, 0)

	if _, cmd := pressKeys(t, m, "c"); cmd == nil {
		t.Error("c did not open the comment editor")
	}
}

// selectedTask indexes the board without looking at the column cursor's range.
// With the filter on, the number of columns changes between one render and
// the next, so the index may end up out and it has to return instead of
// indexing an empty slice.
func TestSelectedTaskWithACursorOutOfRange(t *testing.T) {
	m := newKanbanModel(t, 3)
	m.currentView = viewKanban

	if tsk := m.selectedTask(); tsk == nil {
		t.Fatal("the fixture left no focused task")
	}

	m.kanbanCol = 9999
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("with the column out of range there is a selected task: %q", tsk.Title)
	}

	// And the same in the list, where the cursor can end up past it after a filter.
	m.currentView = viewList
	m.tasks = nil
	m.filteredT = nil
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("with an empty list there is a selected task: %q", tsk.Title)
	}
}

// The save of the inline description editor is two writes: the task and the
// refresh of the listing. If either of the two fails, there is no reload, and
// the model keeps what it had.
func TestSaveDescriptionWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if msg := m.saveDescriptionCmd(id, "new description")(); msg != nil {
		t.Errorf("a failed save returned %T, want nil", msg)
	}
}

// The inline editor needs a task to copy the content from. Without it there
// is nothing to edit.
func TestOpenDescEditorWithoutATask(t *testing.T) {
	m := newTestModel(t)
	m.tasks = nil
	m.filteredT = nil
	m.filterActive = false

	if cmd := m.openDescEditor(); cmd != nil {
		t.Error("without a task the description editor emitted a command")
	}
	if m.descEditOpen {
		t.Error("without a task the description editor opened")
	}
}

// The task form writes to the database and then reloads the listing. With
// the DB closed, the first error is the one the user sees and the message carries the reason.
func TestCreateNewTaskWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	open, _ := pressKeys(t, m, "i")
	withTitle, _ := pressKeys(t, open, "n", "e", "w")
	if strings.TrimSpace(withTitle.newTaskTitle) != "new" {
		t.Fatalf("the title is %q, want new", withTitle.newTaskTitle)
	}

	if err := withTitle.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, cmd := pressKeys(t, withTitle, "enter")
	if cmd == nil {
		t.Fatal("enter did not try to create the task")
	}
	failed, ok := firstTaskFailure(t, cmd)
	if !ok {
		t.Fatal("the creation with the DB closed did not emit taskCreateFailedMsg")
	}
	if failed.err == nil {
		t.Error("creating with the DB closed did not produce an error")
	}
}

// The form's tag field carries no textarea: it is comma-separated text, and
// with the field focused and empty it has to show its hint next to the cursor.
func TestNewTaskTagsFieldShowsItsHint(t *testing.T) {
	m := newTestModel(t)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags

	out := ansi.Strip(m.renderNewTaskTags())
	if !strings.Contains(out, "type to add") {
		t.Errorf("the empty tags field does not show its hint: %q", out)
	}
	if !strings.Contains(out, cursorGlyph) {
		t.Errorf("the tags field hint does not carry the cursor: %q", out)
	}

	// Without focus the same empty field shows the long dash of the others.
	m.newTaskFieldIdx = newTaskFieldTitle
	if out := ansi.Strip(m.renderNewTaskTags()); !strings.Contains(out, "—") {
		t.Errorf("without focus and without tags the field comes out empty instead of \"—\": %q", out)
	}

	// And with something typed the hint disappears: it is a hint, not a value.
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "new"
	out = ansi.Strip(m.renderNewTaskTags())
	if strings.Contains(out, "type to add") {
		t.Errorf("with typed text the hint is still there: %q", out)
	}
	if !strings.Contains(out, "new") {
		t.Errorf("the typed text is not visible: %q", out)
	}

	// With tags already set the tags are shown, not the cursor.
	m.newTaskTagInput = ""
	m.newTaskTags = []string{"one", "two"}
	out = ansi.Strip(m.renderNewTaskTags())
	for _, want := range []string{"one", "two"} {
		if !strings.Contains(out, want) {
			t.Errorf("with tags set %q is not visible: %q", want, out)
		}
	}
	if strings.Contains(out, "type to add") {
		t.Errorf("with tags set the hint is still there: %q", out)
	}
}

// A priority out of the known range is rendered as "no priority", not as a
// garbage character. The range is imposed by the database, but the render
// should not trust that.
func TestRenderPriorityOutOfRange(t *testing.T) {
	m := newTestModel(t)
	m.tasks[0].Priority = 7
	m.width, m.height = 100, 30

	out := ansi.Strip(m.renderList(20))
	if !strings.Contains(out, "Fix checkout") {
		t.Fatalf("the task is not visible in the list:\n%s", out)
	}
	// No priority character should appear in its row.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Fix checkout") && strings.ContainsAny(line, "HLMN") {
			t.Errorf("a priority 7 was rendered as a known character: %q", line)
		}
	}
}

// The keybinds bar changes the border color depending on whether it has focus.
// It is the only signal of "this is what you are reading", so the two branches
// have to produce truly different renders.
func TestKeybindsBarBorderFollowsFocus(t *testing.T) {
	base := KeybindsBar{width: 60, view: viewList}

	withFocus := base
	withFocus.focused = true
	withoutFocus := base

	if withFocus.View() == withoutFocus.View() {
		t.Error("the border is identical with and without focus, want two different colors")
	}

	// And with focus on, the border's color code is the blue the comment talks
	// about, not the default gray.
	if !strings.Contains(withFocus.View(), "\x1b[94m") {
		t.Errorf("with focus the border is not blue (94): %q", withFocus.View())
	}
	if strings.Contains(withoutFocus.View(), "\x1b[94m") {
		t.Errorf("without focus the border comes out blue: %q", withoutFocus.View())
	}
}

// A list with no tasks says so, instead of leaving an empty gap that looks
// like a load failure.
func TestEmptyListSaysSo(t *testing.T) {
	m := newBareModel(t, func(*config.Config) {})
	m.width, m.height = 100, 20
	m.tasks = nil
	m.filteredT = nil

	out := ansi.Strip(m.renderList(18))
	if !strings.Contains(out, "No tasks found") {
		t.Errorf("an empty list does not say so:\n%s", out)
	}
}

// With the priority filter on, the readable value shown in the modal has to
// be the same one the list was filtered by. If not, the filter acts on a
// value the interface does not show.
func TestFilterPriorityAndItsLabel(t *testing.T) {
	m := newTestModel(t)

	for _, value := range []string{"none", "low", "med", "high"} {
		m.filterApplySelection(filterFieldPriority, value)
		if got := m.filterCurrentValue(filterFieldPriority); got != value {
			t.Errorf("set %q shows %q", value, got)
		}
	}
}

// sabotageProjects leaves the projects table with types that cannot be read,
// so that the archived query fails while the active one does not.
// It is the way to prove that a broken secondary query does not sink the start.
func sabotageProjects(t *testing.T, m *Model) {
	t.Helper()
	if _, err := m.database.Conn().Exec(`DROP TABLE projects`); err != nil {
		t.Fatalf("DROP TABLE projects: %v", err)
	}
	if _, err := m.database.Conn().Exec(
		`CREATE TABLE projects (id BLOB, name BLOB, workflow BLOB, list_order BLOB, archived BLOB)`,
	); err != nil {
		t.Fatalf("CREATE TABLE projects: %v", err)
	}
}

// firstTaskFailure unwraps the batch the form returns and returns the first
// taskCreateFailedMsg it finds.
func firstTaskFailure(t *testing.T, cmd tea.Cmd) (taskCreateFailedMsg, bool) {
	t.Helper()
	for _, msg := range mustRun(t, cmd) {
		if failed, ok := msg.(taskCreateFailedMsg); ok {
			return failed, true
		}
	}
	return taskCreateFailedMsg{}, false
}
