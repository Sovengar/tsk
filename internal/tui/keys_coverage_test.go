package tui

import (
	"strings"
	"testing"

	"tsk/internal/config"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// The per-view key handlers are almost all case tables, and the existing
// tests pressed four or five keys per view. What was missing was the tail
// of the table: the keys that only work in a concrete state (archived
// visible, comment selected, empty column) and the ones that do nothing.
// These are those keys.

// pressKeys chains a sequence of keys and returns the resulting model.
func pressKeys(t *testing.T, m *Model, keys ...string) (*Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = press(m, k)
	}
	return m, cmd
}

func TestDashboardKeys(t *testing.T) {
	t.Run("k and j go in different directions", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		// With two projects, +1 and -1 from index 0 give the same result and
		// are told apart: a third one is needed so that the two directions
		// say different things.
		mustCreateProject(t, m.database, "extra", model.DefaultWorkflow)
		reloadProjects(t, m)
		if len(m.dashProjectList()) < 3 {
			t.Fatalf("the fixture left %d projects visible", len(m.dashProjectList()))
		}

		up, _ := pressKeys(t, m, "k")
		if up.dashProjectIdx != len(m.dashProjectList())-1 {
			t.Errorf("k from 0 left the index at %d, want the last (%d)",
				up.dashProjectIdx, len(m.dashProjectList())-1)
		}

		down, _ := pressKeys(t, up, "j")
		if down.dashProjectIdx != 0 {
			t.Errorf("j from the last left the index at %d, want 0 (wrap around)", down.dashProjectIdx)
		}
	})

	t.Run("e opens the edit modal of the selected project", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		next, cmd := pressKeys(t, m, "e")
		if !next.projectModalOpen {
			t.Fatal("e did not open the project modal")
		}
		if !next.projectModalEdit {
			t.Error("the modal opened in create mode, want edit")
		}
		_ = cmd
	})

	t.Run("e with no project selected does nothing", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard
		m.dashProjectIdx = 9999
		m.projects = nil

		next, _ := pressKeys(t, m, "e")
		if next.projectModalOpen {
			t.Error("e opened the modal with no project selected")
		}
	})

	t.Run("d asks for archive confirmation", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		next, _ := pressKeys(t, m, "d")
		if !next.confirmOpen || next.confirmAction != "archive" {
			t.Errorf("d did not ask for archive confirmation: open=%v action=%q",
				next.confirmOpen, next.confirmAction)
		}
	})

	t.Run("d does nothing with the archived visible", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard
		m.showArchived = true
		m.archivedProjects = m.projects

		next, _ := pressKeys(t, m, "d")
		if next.confirmOpen {
			t.Error("d archived a project that is already in the archived list")
		}
	})

	t.Run("r unarchives, but only when the archived are visible", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		hidden, _ := pressKeys(t, m, "r")
		if hidden.confirmOpen {
			t.Error("r asked to unarchive with the archived list hidden")
		}

		m.showArchived = true
		m.archivedProjects = m.projects
		visible, _ := pressKeys(t, m, "r")
		if !visible.confirmOpen || visible.confirmAction != "unarchive" {
			t.Errorf("r did not ask for unarchive confirmation: open=%v action=%q",
				visible.confirmOpen, visible.confirmAction)
		}
	})

	t.Run("m opens the off-day manager and loads the days off", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		next, cmd := pressKeys(t, m, "m")
		if !next.assigneeModalOpen {
			t.Fatal("m did not open the assignees modal")
		}
		if cmd == nil {
			t.Error("m did not launch the off-days load")
		}
	})
}

func TestListKeys(t *testing.T) {
	t.Run("ctrl+p raises the priority of the task under the cursor", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.cursor = 0

		id := m.filteredTasks()[0].ID
		before := m.filteredTasks()[0].Priority
		if _, cmd := pressKeys(t, m, "ctrl+p"); cmd == nil {
			t.Fatal("ctrl+p did not launch any action")
		} else {
			mustRun(t, cmd)
		}

		after, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if after.Priority == before {
			t.Errorf("ctrl+p did not change the priority: still %d", before)
		}
	})

	t.Run("ctrl+p with the cursor out of range does nothing", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.filteredT = nil
		m.tasks = nil

		next, cmd := pressKeys(t, m, "ctrl+p")
		if cmd != nil {
			t.Error("ctrl+p with no tasks launched an action")
		}
		if next.cursor != 0 {
			t.Error("ctrl+p with no tasks moved the cursor")
		}
	})
}

func TestKanbanKeys(t *testing.T) {
	t.Run("d marks the focused card as done", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		id := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].ID
		_, cmd := pressKeys(t, m, "d")
		if cmd == nil {
			t.Fatal("d did not launch any action")
		}
		mustRun(t, cmd)

		task, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if task.Status != "done" {
			t.Errorf("the task ended up as %q, want done", task.Status)
		}
	})

	t.Run("x cancels the focused card", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		id := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].ID
		_, cmd := pressKeys(t, m, "x")
		if cmd == nil {
			t.Fatal("x did not launch any action")
		}
		mustRun(t, cmd)

		task, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if task.Status != "cancelled" {
			t.Errorf("the task ended up as %q, want cancelled", task.Status)
		}
	})

	t.Run("e opens the inline description editor", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		next, _ := pressKeys(t, m, "e")
		if !next.descEditOpen {
			t.Error("e did not open the description editor")
		}
	})

	t.Run("i opens the new task form", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		next, _ := pressKeys(t, m, "i")
		if !next.newTaskOpen {
			t.Error("i did not open the new task form")
		}
	})

	t.Run("E opens the external editor", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		if _, cmd := pressKeys(t, m, "E"); cmd == nil {
			t.Error("E did not launch any command")
		}
	})

	t.Run("ctrl+p changes the priority of the focused card", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		before := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].Priority
		_, cmd := pressKeys(t, m, "ctrl+p")
		if cmd == nil {
			t.Fatal("ctrl+p did not launch any action")
		}
		mustRun(t, cmd)
		reloadTasks(t, m)

		id := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].ID
		task, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if task.Priority == before {
			t.Error("ctrl+p did not change the card's priority")
		}
	})

	t.Run("h in the first column does not move the column index", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban
		m.kanbanCol = 0
		m.kanbanRow = 1

		next, _ := pressKeys(t, m, "h")
		if next.kanbanCol != 0 {
			t.Errorf("h moved the column to %d, want 0", next.kanbanCol)
		}
		// The row is not touched when the column does not change: the index is per column.
		if next.kanbanRow != 1 {
			t.Errorf("h moved the row to %d, want 1 (it does not change column)", next.kanbanRow)
		}
	})

	t.Run("l changes the column and resets the row", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban
		m.kanbanCol = 0
		m.kanbanRow = 2

		next, _ := pressKeys(t, m, "l")
		if next.kanbanCol != 1 {
			t.Errorf("l left the column at %d, want 1", next.kanbanCol)
		}
		if next.kanbanRow != 0 {
			t.Errorf("l did not reset the row: %d, want 0", next.kanbanRow)
		}
	})

	t.Run("with no columns there is nothing to do", func(t *testing.T) {
		bare := newBareModel(t, func(*config.Config) {})
		m := &bare
		m.currentView = viewKanban
		m.filteredT = nil
		m.tasks = nil
		m.projects = []model.Project{{Name: "no-states"}}
		m.filterProject = "no-states"
		if cols := m.kanbanColumns(); len(cols) != 0 {
			t.Fatalf("the fixture did not leave the board empty: %d columns", len(cols))
		}

		for _, k := range []string{"h", "l", "j", "k", "d", "x", "s", "S", "enter", "ctrl+p"} {
			next, cmd := pressKeys(t, m, k)
			if cmd != nil {
				t.Errorf("%q on the empty board launched an action", k)
			}
			if next.kanbanCol != 0 || next.kanbanRow != 0 {
				t.Errorf("%q on the empty board moved the cursor to [%d,%d]",
					k, next.kanbanCol, next.kanbanRow)
			}
		}
	})
}

func TestDetailKeys(t *testing.T) {
	t.Run("s starts the task and closes the detail", func(t *testing.T) {
		// StartStatus returns the second state of the workflow, so it is only
		// noticed from "backlog": in any other status moving to "s" is a
		// no-op by design, not a failure.
		m := newDetailModel(t, 0)
		m.detailTask = taskByStatus(t, m, "backlog")
		m.detailOpen = true

		id := m.detailTask.ID
		next, cmd := pressKeys(t, m, "s")
		if next.detailOpen {
			t.Error("s did not close the detail")
		}
		if next.detailTask != nil {
			t.Error("s did not clear the task from the detail")
		}
		if cmd == nil {
			t.Fatal("s did not launch the start action")
		}
		_, _ = updateMsg(t, next, mustMsg(t, cmd))

		task, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		// StartStatus returns the SECOND state of the workflow, not the third:
		// the name "doing" was my own assumption and not the contract.
		if task.Status != "todo" {
			t.Errorf("the task ended up as %q, want the second workflow state (todo)", task.Status)
		}
	})

	t.Run("x cancels the task and closes the detail", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true

		id := m.detailTask.ID
		next, cmd := pressKeys(t, m, "x")
		if next.detailOpen || next.detailTask != nil {
			t.Error("x did not close the detail")
		}
		if cmd == nil {
			t.Fatal("x did not launch the cancel action")
		}
		_, _ = updateMsg(t, next, mustMsg(t, cmd))

		task, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if task.Status != "cancelled" {
			t.Errorf("the task ended up as %q, want cancelled", task.Status)
		}
	})

	t.Run("d with no comment selected marks the task as done", func(t *testing.T) {
		m := newDetailModel(t, 3)
		m.detailOpen = true
		m.detailCommentSel = -1

		id := m.detailTask.ID
		next, cmd := pressKeys(t, m, "d")
		if next.detailOpen || next.detailTask != nil {
			t.Error("d did not close the detail")
		}
		if next.detailComments != nil {
			t.Error("d did not clear the loaded comments")
		}
		if cmd == nil {
			t.Fatal("d did not launch the done action")
		}
		_, _ = updateMsg(t, next, mustMsg(t, cmd))

		task, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if task.Status != "done" {
			t.Errorf("the task ended up as %q, want done", task.Status)
		}
	})

	t.Run("d with a comment selected deletes the comment, not the task", func(t *testing.T) {
		m := newDetailModel(t, 3)
		m.detailOpen = true

		m, _ = applyComments(t, m)
		m, _ = pressKeys(t, m, "j")
		if m.detailCommentSel < 0 || m.detailCommentSel >= len(m.detailComments) {
			t.Fatalf("the fixture did not leave a comment selected (sel=%d, %d comments)",
				m.detailCommentSel, len(m.detailComments))
		}
		comment := m.detailComments[m.detailCommentSel]

		next, cmd := pressKeys(t, m, "d")
		if !next.detailOpen {
			t.Error("d with a comment selected closed the detail")
		}
		if next.detailTask == nil {
			t.Error("d with a comment selected cleared the detail")
		}
		if cmd == nil {
			t.Fatal("d did not launch the comment deletion")
		}
		_, _ = updateMsg(t, next, mustMsg(t, cmd))

		remaining, err := m.database.ListComments(m.detailTask.ID)
		if err != nil {
			t.Fatalf("ListComments: %v", err)
		}
		for _, c := range remaining {
			if c.ID == comment.ID {
				t.Errorf("comment %d is still there", comment.ID)
			}
		}
		// And the task is still alive: it is a comment, not the task.
		if _, err := m.database.GetTask(m.detailTask.ID); err != nil {
			t.Errorf("deleting a comment deleted the task: %v", err)
		}
	})

	t.Run("d with no task in the detail does nothing", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true
		m.detailTask = nil

		next, cmd := pressKeys(t, m, "d")
		if cmd != nil {
			t.Error("d with no task launched an action")
		}
		if !next.detailOpen {
			t.Error("d with no task closed the detail, want intact")
		}
	})

	t.Run("t opens the tags modal from the detail", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true

		next, _ := pressKeys(t, m, "t")
		if !next.tagOpen {
			t.Error("t did not open the tags modal")
		}
		if next.tagSuggestIdx != -1 {
			t.Errorf("tagSuggestIdx = %d, want -1 (no suggestions on open)", next.tagSuggestIdx)
		}
	})

	t.Run("e and E need a task", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true
		m.detailTask = nil

		if next, _ := pressKeys(t, m, "e"); next.descEditOpen {
			t.Error("e with no task opened the description editor")
		}
		if next, cmd := pressKeys(t, m, "E"); cmd != nil || next.detailOpen == false {
			t.Error("E with no task closed the detail")
		}
		if _, cmd := pressKeys(t, m, "c"); cmd != nil {
			t.Error("c with no task launched a command")
		}
		if next, _ := pressKeys(t, m, "t"); next.tagOpen {
			t.Error("t with no task opened the tags modal")
		}
		if next, _ := pressKeys(t, m, "s"); next.detailOpen == false {
			t.Error("s with no task closed the detail")
		}
		if next, _ := pressKeys(t, m, "x"); next.detailOpen == false {
			t.Error("x with no task closed the detail")
		}
	})

	t.Run("j with comments advances the selection and without comments it does not", func(t *testing.T) {
		m := newDetailModel(t, 2)
		m.detailOpen = true

		empty := *m
		empty.detailComments = nil
		empty.detailCommentSel = 0
		if next, cmd := pressKeys(t, &empty, "j"); cmd != nil || next.detailCommentSel != 0 {
			t.Errorf("j with no comments moved the selection to %d", next.detailCommentSel)
		}

		m.detailCommentSel = -1
		next, _ := pressKeys(t, m, "j")
		if next.detailCommentSel != 0 {
			t.Errorf("j from -1 left the selection at %d, want 0", next.detailCommentSel)
		}
	})

	t.Run("k goes back and deselects on the first one", func(t *testing.T) {
		m := newDetailModel(t, 3)
		m.detailOpen = true

		m.detailCommentSel = 2
		back, _ := pressKeys(t, m, "k")
		if back.detailCommentSel != 1 {
			t.Errorf("k from 2 left the selection at %d, want 1", back.detailCommentSel)
		}

		first, _ := pressKeys(t, back, "k")
		if first.detailCommentSel != 0 {
			t.Errorf("k from 1 left the selection at %d, want 0", first.detailCommentSel)
		}

		none, _ := pressKeys(t, first, "k")
		first = none
		if first.detailCommentSel != -1 {
			t.Errorf("k on the first left the selection at %d, want -1 (nothing selected)",
				first.detailCommentSel)
		}
	})
}

// updateMsg is applyMsg but also returning the command, because several of
// these tests need to check that NONE was launched.
func updateMsg(t *testing.T, m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return asModel(next), cmd
}

// applyComments loads the detail's comments the way Update does it upon
// receiving commentsLoadedMsg, to start from a realistic state.
func applyComments(t *testing.T, m *Model) (*Model, tea.Cmd) {
	t.Helper()
	cmd := m.loadCommentsCmd(m.detailTask.ID)
	if cmd == nil {
		t.Fatal("loadCommentsCmd returned nil")
	}
	return updateMsg(t, m, mustMsg(t, cmd))
}

func TestGlobalKeys(t *testing.T) {
	t.Run("esc closes help, the open filter and the active filter", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			setup func(*Model)
			check func(*testing.T, *Model)
		}{
			{"help", func(m *Model) { m.helpOpen = true },
				func(t *testing.T, m *Model) {
					if m.helpOpen {
						t.Error("esc did not close the help")
					}
				}},
			{"open filter", func(m *Model) { m.filterOpen = true; m.filterActive = true },
				func(t *testing.T, m *Model) {
					if m.filterOpen {
						t.Error("esc did not close the filter")
					}
				}},
			{"active filter", func(m *Model) { m.filterActive = true; m.filterText = "something" },
				func(t *testing.T, m *Model) {
					if m.filterActive {
						t.Error("esc did not deactivate the filter")
					}
					if m.filterText != "" {
						t.Errorf("esc did not clear the filter text: %q", m.filterText)
					}
				}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m := newTestModel(t)
				tc.setup(m)

				next, _ := pressKeys(t, m, "esc")
				tc.check(t, next)
			})
		}
	})

	t.Run("? toggles the help", func(t *testing.T) {
		m := newTestModel(t)

		open, _ := pressKeys(t, m, "?")
		if !open.helpOpen {
			t.Fatal("? did not open the help")
		}

		closed, _ := pressKeys(t, open, "?")
		if closed.helpOpen {
			t.Error("? did not close the help")
		}
	})

}

// The paste goes to three places depending on what is open, and only to one.
// The order matters: with the description editor and the form open at once,
// the editor wins because it is checked first.
func TestPasteRouting(t *testing.T) {
	t.Run("nothing open", func(t *testing.T) {
		m := newTestModel(t)
		next, cmd := updateMsg(t, m, tea.PasteMsg{Content: "text"})
		if cmd != nil {
			t.Error("a paste with nothing open launched a command")
		}
		if next.tagInput != "" {
			t.Errorf("the paste wrote into the tags input: %q", next.tagInput)
		}
	})

	t.Run("the tags modal", func(t *testing.T) {
		m := newTestModel(t)
		m.tagOpen = true
		m.detailTask = &m.tasks[0]

		next, _ := updateMsg(t, m, tea.PasteMsg{Content: "pasted"})
		if next.tagInput != "pasted" {
			t.Errorf("tagInput = %q, want %q", next.tagInput, "pasted")
		}
	})

	t.Run("the new task form", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldTitle

		next, _ := updateMsg(t, m, tea.PasteMsg{Content: "pasted"})
		if !strings.Contains(next.newTaskTitle, "pasted") {
			t.Errorf("the paste did not reach the form: %q", next.newTaskTitle)
		}
	})
}

// The three messages that exist only to propagate errors: the external
// editor that fails, the form that fails and an empty comment. None must
// leave the model in a half-done state.
func TestSilentMessagesLeaveTheModelIntact(t *testing.T) {
	t.Run("the external editor fails", func(t *testing.T) {
		m := newTestModel(t)
		before := ansi.Strip(m.View().Content)

		next, cmd := updateMsg(t, m, editorFinishedMsg{err: errActionFailed, taskID: 1})
		if cmd != nil {
			t.Error("a failed editor launched a command")
		}
		if after := ansi.Strip(next.View().Content); after != before {
			t.Error("a failed editor changed the render")
		}
	})

	t.Run("the external editor writes nothing", func(t *testing.T) {
		m := newTestModel(t)

		next, cmd := updateMsg(t, m, editorFinishedMsg{taskID: m.tasks[0].ID})
		if cmd != nil {
			t.Error("an editor that writes nothing launched a command")
		}
		if len(next.tasks) != len(m.tasks) {
			t.Error("an empty editor modified the tasks")
		}
	})

	t.Run("the creation fails with a toast", func(t *testing.T) {
		m := newTestModel(t)

		next, _ := updateMsg(t, m, taskCreateFailedMsg{err: errActionFailed})
		if !strings.Contains(ansi.Strip(next.View().Content), errActionFailed.Error()) {
			t.Errorf("the creation error does not show in the UI:\n%s",
				ansi.Strip(next.View().Content))
		}
	})

	t.Run("the creation fails without an error", func(t *testing.T) {
		m := newTestModel(t)
		before := ansi.Strip(m.View().Content)

		next, _ := updateMsg(t, m, taskCreateFailedMsg{})
		if after := ansi.Strip(next.View().Content); after != before {
			t.Error("a failed creation without an error changed the render")
		}
	})

	t.Run("an empty comment is not saved", func(t *testing.T) {
		m := newTestModel(t)

		next, cmd := updateMsg(t, m, commentFinishedMsg{taskID: m.tasks[0].ID, body: ""})
		if cmd != nil {
			t.Error("an empty comment launched a save")
		}
		if len(next.detailComments) != 0 {
			t.Error("an empty comment loaded comments")
		}
	})

	t.Run("a comment with an error is not saved", func(t *testing.T) {
		m := newTestModel(t)

		if _, cmd := updateMsg(t, m, commentFinishedMsg{err: errActionFailed, taskID: m.tasks[0].ID}); cmd != nil {
			t.Error("a failed comment launched a save")
		}
	})
}

// Toggling a tag that is no longer in the database fails on read and
// returns nil, not a message: there is nothing to update.
func TestToggleTagOnAMissingTask(t *testing.T) {
	m := newTestModel(t)

	if msg := m.toggleTagCmd(9999, "nothing")(); msg != nil {
		t.Errorf("toggleTag on a missing task returned %T, want nil", msg)
	}
}

// commonWorkflow falls back to the default workflow when the projects share
// no status at all, which is exactly what happens if each project defines its own.
func TestCommonWorkflowFallsBackWhenNothingIsShared(t *testing.T) {
	m := newTestModel(t)
	m.projects = []model.Project{
		{Name: "a", Workflow: []string{"one"}},
		{Name: "b", Workflow: []string{"other"}},
	}

	got := m.commonWorkflow()
	if len(got) != len(model.DefaultWorkflow) {
		t.Errorf("with no common states the workflow is %v, want the default %v",
			got, model.DefaultWorkflow)
	}
}

// taskByStatus returns the model's first task in that status, or fails the
// test. The statuses matter: moving a task depends on the status it is in,
// not only on the key.
func taskByStatus(t *testing.T, m *Model, status string) *model.Task {
	t.Helper()
	for i := range m.tasks {
		if m.tasks[i].Status == status {
			return &m.tasks[i]
		}
	}
	t.Fatalf("the fixture has no task in %q", status)
	return nil
}

// The toast's sequence number has to change on EACH toast, and each
// expiration tick has to carry its own number.
//
// It is not an internal detail: the tick compares its number with the current
// one before deleting, and that is why an OLD tick is the one that must not
// touch a new toast. If two toasts shared a number, the first one's tick
// would erase the second; and if the number did not advance, all old ticks would erase the live toast.
func TestToastSequenceNumberAdvances(t *testing.T) {
	m := newTestModel(t)

	// Three toasts in a row: three different numbers.
	var seen []int
	for i := 0; i < 3; i++ {
		m.setToast("toast "+itoa(i), "info")
		seen = append(seen, m.toastSeq)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] == seen[i-1] {
			t.Fatalf("toasts %d and %d share sequence %d", i-1, i, seen[i])
		}
	}

	// The FIRST one's tick arrives late, with a newer toast on screen: it does
	// not erase it. This is the case the counter exists for. Update has the
	// receiver by value, so the returned model is the one to look at.
	model, _ := m.Update(toastExpiredMsg{seq: seen[0]})
	withOld := model.(Model)
	if withOld.toast == "" {
		t.Error("the first toast's tick erased the third toast: an old tick cannot clear")
	}

	// The LAST one's tick is its own: it erases it.
	model, _ = withOld.Update(toastExpiredMsg{seq: seen[2]})
	if model.(Model).toast != "" {
		t.Errorf("its own toast's tick did not erase it: %q", model.(Model).toast)
	}
}

// jumps itself: the jump is one by one, and that is what a test can look at.
func TestJumps(t *testing.T) {
	for _, c := range []struct{ in, want int }{{0, 1}, {1, 2}, {41, 42}} {
		if got := jumps(c.in); got != c.want {
			t.Errorf("jumps(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
