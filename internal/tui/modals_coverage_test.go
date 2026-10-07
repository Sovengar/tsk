package tui

import (
	"strings"
	"testing"
)

// The filter modal has five fields and each one has its own axis of values.
// What was not exercised were the "all" values of the fields that are not
// the project one, and the edges of the index arithmetic with lists of 0
// and 1 elements.

func TestFilterFieldOptionsEdges(t *testing.T) {
	t.Run("the status with a project filter that no longer exists", func(t *testing.T) {
		m := newTestModel(t)
		m.filterProject = "deleted-project"

		// Without the project, the status can only offer the two global ones:
		// there is no workflow to show.
		opts := m.filterFieldOptions(filterFieldStatus)
		if len(opts) != 2 {
			t.Errorf("with a missing filter the status offers %d options (%v), want 2",
				len(opts), opts)
		}
	})

	t.Run("a field that does not exist", func(t *testing.T) {
		m := newTestModel(t)
		if opts := m.filterFieldOptions(99); opts != nil {
			t.Errorf("a missing field returns %v, want nil", opts)
		}
		if got := filterFieldLabel(99); got != "?" {
			t.Errorf("filterFieldLabel(99) = %q, want %q", got, "?")
		}
	})
}

func TestFilterApplySelectionPerField(t *testing.T) {
	t.Run("assignee", func(t *testing.T) {
		m := newTestModel(t)
		m.filterAssignee = "@john"

		m.filterApplySelection(filterFieldAssignee, "all")
		if m.filterAssignee != "" {
			t.Errorf("with \"all\" the assignee filter is %q, want empty", m.filterAssignee)
		}

		m.filterApplySelection(filterFieldAssignee, "@margo")
		if m.filterAssignee != "@margo" {
			t.Errorf("the assignee filter is %q, want @margo", m.filterAssignee)
		}
	})

	t.Run("status", func(t *testing.T) {
		m := newTestModel(t)
		m.filterStatus = "doing"

		m.filterApplySelection(filterFieldStatus, "all")
		if m.filterStatus != "" {
			t.Errorf("with \"all\" the status filter is %q, want empty", m.filterStatus)
		}

		m.filterApplySelection(filterFieldStatus, "todo")
		if m.filterStatus != "todo" {
			t.Errorf("the status filter is %q, want todo", m.filterStatus)
		}
	})

	t.Run("tag", func(t *testing.T) {
		m := newTestModel(t)
		m.filterTag = "blocked"

		m.filterApplySelection(filterFieldTag, "all")
		if m.filterTag != "" {
			t.Errorf("with \"all\" the tag filter is %q, want empty", m.filterTag)
		}

		m.filterApplySelection(filterFieldTag, "urgent")
		if m.filterTag != "urgent" {
			t.Errorf("the tag filter is %q, want urgent", m.filterTag)
		}
	})

	// The three values that are not "med"/"high", which is the only one pressed.
	t.Run("priority: all, none and low", func(t *testing.T) {
		for _, tc := range []struct {
			value string
			want  int
		}{
			{"all", -1},
			{"none", 0},
			{"low", 1},
			{"med", 2},
			{"high", 3},
		} {
			m := newTestModel(t)
			m.filterApplySelection(filterFieldPriority, tc.value)
			if m.filterPriority != tc.want {
				t.Errorf("priority %q left filterPriority=%d, want %d",
					tc.value, m.filterPriority, tc.want)
			}
			// And the readable value shown back in the modal.
			if got := m.filterCurrentValue(filterFieldPriority); got != tc.value {
				t.Errorf("filterCurrentValue after %q is %q", tc.value, got)
			}
		}
	})

	t.Run("project", func(t *testing.T) {
		m := newTestModel(t)
		m.filterProject = "api"

		m.filterApplySelection(filterFieldProject, "all")
		if m.filterProject != "" {
			t.Errorf("with \"all\" the project filter is %q, want empty", m.filterProject)
		}

		m.filterApplySelection(filterFieldProject, "web")
		if m.filterProject != "web" {
			t.Errorf("the project filter is %q, want web", m.filterProject)
		}
	})
}

// With a single project there is nowhere to cycle the project filter:
// "all" and the project itself already cover everything. What has to be
// checked is that it does not stay on an impossible index.
func TestCycleProjectFilterWithNoProjects(t *testing.T) {
	m := newTestModel(t)
	m.projects = nil

	if opts := m.filterFieldOptions(filterFieldProject); len(opts) != 1 {
		t.Fatalf("with no projects there are %d options (%v), want 1", len(opts), opts)
	}

	before := m.filterProject
	for range 3 {
		m.cycleProjectFilter(1)
		m.cycleProjectFilter(-1)
	}
	if m.filterProject != before {
		t.Errorf("the filter went from %q to %q with no projects", before, m.filterProject)
	}
}

// The live option cycle cannot do anything if there are no options: the
// index stays where it was instead of moving to -1.
func TestFilterCycleWithNoOptions(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = filterFieldTag
	m.filterSearch = "does-not-exist"

	before := m.filterOptionIdx
	m.filterCycle(true)
	if m.filterOptionIdx != before {
		t.Errorf("the index moved to %d with no options, want %d", m.filterOptionIdx, before)
	}
	if !m.filterActive && m.filterTag != "" {
		t.Errorf("cycling with no options left a hanging filter: %q", m.filterTag)
	}
}

func TestFilterModalKeys(t *testing.T) {
	t.Run("shift+tab goes back a field", func(t *testing.T) {
		m := newTestModel(t)
		m.filterOpen = true
		m.filterFieldIdx = 0

		next, _ := pressKeys(t, m, "shift+tab")
		// With five fields, from the first one shift+tab wraps to the last.
		want := filterFieldCount - 1
		if next.filterFieldIdx != want {
			t.Errorf("shift+tab from 0 left the field at %d, want %d",
				next.filterFieldIdx, want)
		}

		// And tab from the last one goes back to the first.
		next, _ = pressKeys(t, next, "tab")
		if next.filterFieldIdx != 0 {
			t.Errorf("tab from the last left the field at %d, want 0", next.filterFieldIdx)
		}
	})

	t.Run("backspace trims the search and does nothing when empty", func(t *testing.T) {
		m := newTestModel(t)
		m.filterOpen = true
		m.filterSearch = "api"
		m.filterOptionIdx = 2

		next, _ := pressKeys(t, m, "backspace")
		if next.filterSearch != "ap" {
			t.Errorf("the search is %q, want ap", next.filterSearch)
		}
		if next.filterOptionIdx != 0 {
			t.Errorf("backspace did not return to the start of the list: %d", next.filterOptionIdx)
		}

		next.filterSearch = ""
		other, _ := pressKeys(t, next, "backspace")
		if other.filterSearch != "" {
			t.Errorf("backspace with an empty search left %q", other.filterSearch)
		}
	})
}

// The people modal has two levels: the list and one person's detail. Both
// have their key set, and the detail one only acts if there is a selected
// person with free days.
func TestAssigneeModalKeys(t *testing.T) {
	t.Run("navigate the list and close it", func(t *testing.T) {
		m := newAssigneeModel(t, 3)
		m.assigneeModalOpen = true
		m.currentView = viewList

		if len(m.assigneeRoster()) < 2 {
			t.Skip("the fixture needs two people in the roster")
		}

		down, _ := pressKeys(t, m, "j")
		if down.assigneeIdx == m.assigneeIdx {
			t.Error("j did not move the person index")
		}
		up, _ := pressKeys(t, down, "k")
		if up.assigneeIdx != m.assigneeIdx {
			t.Errorf("k did not return to index %d: it is at %d", m.assigneeIdx, up.assigneeIdx)
		}

		closed, _ := pressKeys(t, up, "esc")
		if closed.assigneeModalOpen {
			t.Error("esc did not close the people modal")
		}
	})

	t.Run("one person's detail", func(t *testing.T) {
		m := newAssigneeModel(t, 3)
		m.assigneeModalOpen = true
		m.currentView = viewList
		m.assigneeDetail = true

		offs := m.assigneeOffDays(m.currentAssignee())
		if len(offs) < 2 {
			t.Skip("the fixture needs two free days for the selected person")
		}

		down, _ := pressKeys(t, m, "j")
		if down.assigneeOffdayIdx != 1 {
			t.Errorf("j left the off-day at %d, want 1", down.assigneeOffdayIdx)
		}
		up, _ := pressKeys(t, down, "k")
		if up.assigneeOffdayIdx != 0 {
			t.Errorf("k left the off-day at %d, want 0", up.assigneeOffdayIdx)
		}

		// d asks to confirm the deletion of the focused off-day.
		confirmed, _ := pressKeys(t, up, "d")
		if !confirmed.confirmOpen || confirmed.confirmAction != "delete-offday" {
			t.Errorf("d did not ask to confirm the deletion: open=%v action=%q",
				confirmed.confirmOpen, confirmed.confirmAction)
		}
		if confirmed.confirmOffday.ID == 0 {
			t.Error("the pending off-day was not copied into the confirmation")
		}

		// a opens the creation of a free day.
		formOpened, cmd := pressKeys(t, up, "a")
		if !formOpened.offdayFormOpen {
			t.Error("a did not open the free day form")
		}
		_ = cmd

		// esc goes back to the list level, it does not close the whole modal.
		back, _ := pressKeys(t, up, "esc")
		if back.assigneeDetail {
			t.Error("esc did not leave the person's detail")
		}
		if !back.assigneeModalOpen {
			t.Error("esc in the detail closed the whole modal")
		}
	})

}

// The result of deleting an off-day has its own text, and the default
// "Done" is the branch that ran when the message brought no known
// action.
func TestOffDaySavedMessage(t *testing.T) {
	m := newAssigneeModel(t, 2)

	withName, _ := updateMsg(t, m, offdaySavedMsg{action: "delete", name: "@john"})
	if !strings.Contains(withName.toast, "@john") {
		t.Errorf("the notice does not name the person: %q", withName.toast)
	}
	if withName.toastKind != "info" {
		t.Errorf("the notice is of kind %q, want info", withName.toastKind)
	}

	fallback, _ := updateMsg(t, m, offdaySavedMsg{action: "anything"})
	if fallback.toast != "Done" {
		t.Errorf("the default notice is %q, want Done", fallback.toast)
	}

	// A failed create reopens the form so the written text is not lost; a
	// failed delete does not, because there was nothing written.
	afterAdd, _ := updateMsg(t, m, offdaySavedMsg{err: errActionFailed, action: "add"})
	if !afterAdd.offdayFormOpen {
		t.Error("a failed creation did not reopen the form")
	}
	afterDelete, _ := updateMsg(t, m, offdaySavedMsg{err: errActionFailed, action: "delete"})
	if afterDelete.offdayFormOpen {
		t.Error("a failed deletion reopened the creation form")
	}
	if afterDelete.toastKind != "error" {
		t.Errorf("the error notice is of kind %q, want error", afterDelete.toastKind)
	}
}

// The tag modal offers suggestions and navigates them with the arrows. With
// no suggestions the index does not move: it is the difference between
// "nothing to choose" and "choosing the first one".
func TestTagModalSuggestionNavigation(t *testing.T) {
	t.Run("with suggestions", func(t *testing.T) {
		m := newTestModel(t)
		m.tasks[0].Tags = []string{"alpha", "beta", "gamma"}
		m.tagOpen = true
		m.detailTask = &m.tasks[0]

		sug := m.tagSuggestions()
		if len(sug) < 2 {
			t.Fatalf("the fixture needs two suggestions, it has %v", sug)
		}

		down, _ := pressKeys(t, m, "down")
		if down.tagSuggestIdx != 0 {
			t.Errorf("down left the suggestion at %d, want 0", down.tagSuggestIdx)
		}
		up, _ := pressKeys(t, down, "up")
		// up from 0 cycles to the last, it does not stay at -1.
		if up.tagSuggestIdx != len(sug)-1 {
			t.Errorf("up from 0 left the suggestion at %d, want %d",
				up.tagSuggestIdx, len(sug)-1)
		}
	})

	t.Run("without suggestions", func(t *testing.T) {
		m := newTestModel(t)
		m.tagOpen = true
		m.detailTask = &m.tasks[0]

		next, _ := pressKeys(t, m, "down")
		if next.tagSuggestIdx != -1 {
			t.Errorf("with no suggestions down left the index at %d, want -1", next.tagSuggestIdx)
		}
	})
}

// Enter with an empty input picks the suggestion; if there is no suggestion
// nor task, it does nothing and the modal stays open so the written text is not lost.
func TestTagModalEnterWithoutSuggestions(t *testing.T) {
	m := newTestModel(t)
	m.tagOpen = true
	m.detailTask = &m.tasks[0]
	m.tagInput = "   "
	m.tagSuggestIdx = -1

	next, cmd := pressKeys(t, m, "enter")
	if cmd != nil {
		t.Error("enter with no suggestion emitted a toggle")
	}
	if !next.tagOpen {
		t.Error("enter with no suggestion closed the modal")
	}
}

// The gantt's navigation jumps from task row to task row,
// skipping the person headers. To check that this always works,
// one has to verify the invariant it depends on: every header is
// immediately followed by a task row, so searching forward from a
// header always finds something.
//
// This test is what sustains the claim of snapGanttCursor's comment. If it
// stopped being true, the backward search would stop being unreachable
// and this would fail before anyone noticed it on the screen.
func TestEveryGanttAssigneeHeaderIsFollowedByATask(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john", "@margo", "@nick"}, 3)
	m.currentView = viewGantt
	m.width, m.height = 140, 40

	rows := m.ganttRows()
	headers := 0
	for i, f := range rows {
		if f.kind != ganttAssigneeRow {
			continue
		}
		headers++
		if i+1 >= len(rows) || rows[i+1].kind != ganttTaskRow {
			t.Fatalf("header %d (%s) is not followed by a task: %+v",
				i, f.assignee, rows[min(i+1, len(rows)-1)])
		}
	}
	if headers == 0 {
		t.Fatal("the fixture left no person header")
	}

	// And with that invariant, resting the cursor on any header ends up on a
	// task, never on an impossible index.
	for i, f := range rows {
		if f.kind != ganttAssigneeRow {
			continue
		}
		m.ganttCursor = i
		m.snapGanttCursor()
		if rows[m.ganttCursor].kind != ganttTaskRow {
			t.Errorf("with the cursor on header %d the gantt stayed on a row %v, want a task",
				i, rows[m.ganttCursor].kind)
		}
	}
}
