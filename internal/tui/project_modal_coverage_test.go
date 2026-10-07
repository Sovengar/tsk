package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// The edit mode of the project modal did not have a single test: the whole
// path -- preloading the fields, parsing the workflow, updating and
// notifying -- was never run. It is the path used when renaming a project, so
// it is not a minor gap.

func TestProjectModalEditRoundTrip(t *testing.T) {
	m := newDashModel(t, "api")
	m.currentView = viewDashboard

	p := m.selectedDashProject()
	if p == nil {
		t.Fatal("the fixture left no project selected")
	}
	original := p.Name

	opened, _ := pressKeys(t, m, "e")
	if !opened.projectModalEdit || opened.projectEditingName != original {
		t.Fatalf("the modal did not open in edit mode: edit=%v original=%q",
			opened.projectModalEdit, opened.projectEditingName)
	}
	if opened.projectNameInput != original {
		t.Errorf("the name was not preloaded: %q", opened.projectNameInput)
	}
	if opened.projectWorkflowInput != strings.Join(p.Workflow, ",") {
		t.Errorf("the workflow was not preloaded: %q", opened.projectWorkflowInput)
	}
	if opened.projectListOrderInput != strings.Join(p.ListOrder, ",") {
		t.Errorf("the list order was not preloaded: %q", opened.projectListOrderInput)
	}

	// Erase the whole name and write another one, which is what the user does.
	// Four backspaces erase the whole name (5 letters of the fixture), which is
	// the real path for renaming.
	renamed := opened
	renamed.projectNameInput = ""
	renamed, _ = pressKeys(t, opened, "backspace", "backspace", "backspace",
		"backspace", "backspace", "backspace")
	for _, k := range []string{"z", "e", "t", "a"} {
		renamed, _ = pressKeys(t, renamed, k)
	}
	if renamed.projectNameInput != "zeta" {
		t.Fatalf("typing by hand left %q, want zeta", renamed.projectNameInput)
	}

	saved, cmd := pressKeys(t, renamed, "enter")
	if saved.projectModalOpen {
		t.Error("enter did not close the modal")
	}
	if cmd == nil {
		t.Fatal("enter did not launch the save")
	}

	msg := mustMsg(t, cmd)
	savedMsg, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("the message is %T, want projectSavedMsg", msg)
	}
	if savedMsg.err != nil {
		t.Fatalf("the save failed: %v", savedMsg.err)
	}
	if savedMsg.action != "edit" {
		t.Errorf("the action is %q, want edit", savedMsg.action)
	}
	if savedMsg.name != "zeta" {
		t.Errorf("the saved name is %q, want zeta", savedMsg.name)
	}

	// And the project exists with the new name and the intact workflow, which is
	// what "empty = keep the current one" means.
	fetched, err := m.database.GetProject("zeta")
	if err != nil {
		t.Fatalf("GetProject(zeta): %v", err)
	}
	if len(fetched.Workflow) != len(p.Workflow) {
		t.Errorf("the workflow went from %v to %v", p.Workflow, fetched.Workflow)
	}

	applied, _ := updateMsg(t, saved, msg)
	if !strings.Contains(ansi.Strip(applied.View().Content), "updated") {
		t.Errorf("the update is not announced:\n%s", ansi.Strip(applied.View().Content))
	}
	if applied.pendingSelectName != "zeta" {
		t.Errorf("pendingSelectName = %q, want zeta so the dashboard selects it", applied.pendingSelectName)
	}
}

// Renaming to a name that already exists is the most likely error when editing,
// and the modal reopens so the written text is not lost.
func TestProjectModalEditRejectsADuplicateName(t *testing.T) {
	m := newDashModel(t, "api")
	m.currentView = viewDashboard

	other := ""
	for _, p := range m.projects {
		if p.Name != "api" {
			other = p.Name
			break
		}
	}
	if other == "" {
		t.Skip("the fixture needs a second project")
	}

	msg := mustMsg(t, m.saveProjectCmd(true, "api", other, "", ""))
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("the message is %T, want projectSavedMsg", msg)
	}
	if saved.err == nil {
		t.Fatal("renaming to a duplicate name did not fail")
	}
	if saved.action != "edit" {
		t.Errorf("the action is %q, want edit", saved.action)
	}

	reopened, _ := updateMsg(t, m, msg)
	if !reopened.projectModalOpen {
		t.Error("the modal did not reopen after the error, want reopen so the typed text is not lost")
	}
}

// The modal's workflow is free text separated by commas, so a comma-separated
// list is a parse error, not an empty workflow.
func TestProjectModalWorkflowParseErrors(t *testing.T) {
	m := newDashModel(t, "api")

	for _, tc := range []struct {
		name      string
		workflow  string
		listOrder string
	}{
		{"empty workflow", ", ,", ""},
		{"empty list order", "backlog,done", " , "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := mustMsg(t, m.saveProjectCmd(true, "api", "api", tc.workflow, tc.listOrder))
			saved, ok := msg.(projectSavedMsg)
			if !ok {
				t.Fatalf("the message is %T, want projectSavedMsg", msg)
			}
			if saved.err == nil {
				t.Fatalf("%q did not produce an error", tc.name)
			}
			if saved.action != "edit" {
				t.Errorf("the action is %q, want edit", saved.action)
			}
		})
	}

	t.Run("create", func(t *testing.T) {
		for _, tc := range []struct{ workflow, listOrder string }{
			{", ,", ""},
			{"", ", ,"},
		} {
			msg := mustMsg(t, m.saveProjectCmd(false, "", "new", tc.workflow, tc.listOrder))
			saved := msg.(projectSavedMsg)
			if saved.err == nil {
				t.Errorf("create with workflow=%q listOrder=%q did not fail", tc.workflow, tc.listOrder)
			}
			if saved.action != "create" {
				t.Errorf("the action is %q, want create", saved.action)
			}
		}
	})

	t.Run("create with valid workflow and order", func(t *testing.T) {
		msg := mustMsg(t, m.saveProjectCmd(false, "", "new", "backlog,done", "done,backlog"))
		saved := msg.(projectSavedMsg)
		if saved.err != nil {
			t.Fatalf("create with valid values failed: %v", saved.err)
		}

		created, err := m.database.GetProject("new")
		if err != nil {
			t.Fatalf("GetProjectByName: %v", err)
		}
		if len(created.Workflow) != 2 || len(created.ListOrder) != 2 {
			t.Errorf("the created project has workflow=%v listOrder=%v",
				created.Workflow, created.ListOrder)
		}
	})
}

// The modal's five text fields share the same one-line editor. The edges:
// empty buffer, the space that must not enter the list fields, and the nil
// pointer that the interface never produces but the function does
// have to tolerate.
func TestEditTextInput(t *testing.T) {
	t.Run("backspace on an empty buffer does nothing", func(t *testing.T) {
		s := ""
		editTextInput(&s, "backspace")
		if s != "" {
			t.Errorf("the buffer is %q, want empty", s)
		}
	})

	t.Run("backspace removes a character", func(t *testing.T) {
		s := "abc"
		editTextInput(&s, "backspace")
		if s != "ab" {
			t.Errorf("the buffer is %q, want ab", s)
		}
	})

	t.Run("the space enters with its name", func(t *testing.T) {
		s := ""
		editTextInput(&s, "space")
		if s != " " {
			t.Errorf("the buffer is %q, want a space", s)
		}
	})

	// Bubbletea delivers the space as a named key, so the lone character
	// branch is never exercised from the interface. It is tested here because it
	// is live code and its behavior (inserting a space) is part of the contract.
	t.Run("a lone space also enters", func(t *testing.T) {
		s := ""
		editTextInput(&s, " ")
		if s != " " {
			t.Errorf("the buffer is %q, want a space", s)
		}
	})

	t.Run("a nil pointer does not crash", func(t *testing.T) {
		editTextInput(nil, "a")
		editTextInput(nil, "backspace")
	})

	t.Run("the modal with an impossible field index does not crash", func(t *testing.T) {
		m := newProjectModel(t, 3)
		m.projectModalField = 7

		next, _ := press(m, "a")
		if next.projectNameInput != m.projectNameInput {
			t.Error("a key wrote into a field with an out-of-range index")
		}
	})
}

// The confirmation modal is a switch over the pending action, and each branch
// has its own degenerate case: deleting an off-day that does not exist,
// archiving with no project, and an action that is none of the known ones.
func TestConfirmModalEdges(t *testing.T) {
	t.Run("an off-day that does not exist", func(t *testing.T) {
		m := newTestModel(t)
		m.confirmOpen = true
		m.confirmAction = "delete-offday"
		m.confirmOffday = model.OffDay{}

		next, cmd := pressKeys(t, m, "y")
		if cmd != nil {
			t.Error("deleting a non-existent off-day launched a command")
		}
		if next.confirmOpen {
			t.Error("the modal is still open")
		}
	})

	t.Run("a project action without a name", func(t *testing.T) {
		for _, action := range []string{"archive", "unarchive"} {
			m := newTestModel(t)
			m.confirmOpen = true
			m.confirmAction = action
			m.confirmProject = ""

			if next, cmd := pressKeys(t, m, "y"); cmd != nil {
				t.Errorf("%s without a project name launched a command", action)
			} else if next.confirmOpen {
				t.Errorf("%s without a project name left the modal open", action)
			}
		}
	})

	t.Run("an unknown action", func(t *testing.T) {
		m := newTestModel(t)
		m.confirmOpen = true
		m.confirmAction = "anything"

		next, cmd := pressKeys(t, m, "y")
		if cmd != nil {
			t.Error("an unknown action launched a command")
		}
		if next.confirmOpen {
			t.Error("an unknown action left the modal open")
		}
	})

	t.Run("every way of saying no clears the state", func(t *testing.T) {
		for _, k := range []string{"n", "N", "esc", "q"} {
			m := newTestModel(t)
			m.confirmOpen = true
			m.confirmAction = "archive"
			m.confirmProject = "api"
			m.confirmOffday = model.OffDay{ID: 7, Assignee: "@john"}

			next, cmd := pressKeys(t, m, k)
			if cmd != nil {
				t.Errorf("%q launched an action command", k)
			}
			if next.confirmOpen || next.confirmAction != "" || next.confirmProject != "" {
				t.Errorf("%q did not clear the confirmation state", k)
			}
			if next.confirmOffday.ID != 0 {
				t.Errorf("%q did not clear the pending off-day", k)
			}
		}
	})

	t.Run("and it also accepts enter", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard
		m.confirmOpen = true
		m.confirmAction = "archive"
		m.confirmProject = "api"

		_, cmd := pressKeys(t, m, "enter")
		if cmd == nil {
			t.Fatal("enter did not confirm the action")
		}
		msg := mustMsg(t, cmd)
		saved, ok := msg.(projectSavedMsg)
		if !ok {
			t.Fatalf("the message is %T, want projectSavedMsg", msg)
		}
		if saved.err != nil {
			t.Fatalf("archiving failed: %v", saved.err)
		}
		// GetProject does not filter by archived: you have to look at the active list.
		active, err := m.database.ListProjects()
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		for _, p := range active {
			if p.Name == "api" {
				t.Error("the project is still in the active list after archiving")
			}
		}
	})
}

// The result of a project operation decides the notice text and whether the
// modal reopens. The four texts and the default "Done" are different branches
// of the same switch.
func TestProjectSavedMessages(t *testing.T) {
	for _, tc := range []struct {
		action   string
		name     string
		want     string
		selected bool
	}{
		{"create", "new", `Project "new" created`, true},
		{"edit", "new", `Project "new" updated`, true},
		{"archive", "api", `Project "api" archived`, false},
		{"unarchive", "api", `Project "api" restored`, true},
		{"any-other", "not-relevant", "Done", false},
	} {
		t.Run(tc.action, func(t *testing.T) {
			m := newDashModel(t, "api")
			m.width, m.height = 100, 30

			next, _ := updateMsg(t, m, projectSavedMsg{name: tc.name, action: tc.action})
			out := ansi.Strip(next.View().Content)
			if !strings.Contains(out, tc.want) {
				t.Errorf("the notice does not say %q:\n%s", tc.want, out)
			}
			if (next.pendingSelectName == tc.name) != tc.selected {
				t.Errorf("pendingSelectName = %q, the selection %v", next.pendingSelectName, tc.selected)
			}
		})
	}

	t.Run("an archive error does not reopen the modal", func(t *testing.T) {
		m := newDashModel(t, "api")
		next, _ := updateMsg(t, m, projectSavedMsg{err: errActionFailed, action: "archive"})
		if next.projectModalOpen {
			t.Error("a failed archive reopened the project modal")
		}
	})
}

// The modal title tells creation from editing, and the hint text
// changes with it: when creating, empty means "use the default"; when editing,
// empty means "do not touch what is there".
func TestProjectModalTitleFollowsTheMode(t *testing.T) {
	creating := newProjectModel(t, 0)
	creating.projectModalEdit = false
	if out := ansi.Strip(creating.renderProjectModal("")); !strings.Contains(out, "New Project") {
		t.Errorf("the creation modal does not say New Project:\n%s", out)
	} else if !strings.Contains(out, "empty = default") {
		t.Errorf("the creation modal does not warn that empty = default:\n%s", out)
	}

	editing := *creating
	editing.projectModalEdit = true
	out := ansi.Strip(editing.renderProjectModal(""))
	if !strings.Contains(out, "Edit Project") {
		t.Errorf("the edit modal does not say Edit Project:\n%s", out)
	}
	if !strings.Contains(out, "keep current") {
		t.Errorf("the edit modal does not warn that empty = keep:\n%s", out)
	}
}
