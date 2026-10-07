package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// ctrlR builds the reset keypress of the filter modal.
func ctrlR() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
}

// filterOptionIndex looks up an option inside a field's list.
func filterOptionIndex(opts []string, want string) int {
	for i, o := range opts {
		if o == want {
			return i
		}
	}
	return -1
}

// TestFilterStatusOptionsScopedToProject verifies that with a project
// selected the status picker offers only the statuses of that project.
func TestFilterStatusOptionsScopedToProject(t *testing.T) {
	m := newTestModel(t)

	m.filterProject = "api"
	opts := m.filterFieldOptions(filterFieldStatus)
	// api uses the default workflow.
	for _, want := range append([]string{"all"}, model.DefaultWorkflow...) {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("with api selected %q is missing in %v", want, opts)
		}
	}
	// No status that does not belong to api.
	if filterOptionIndex(opts, "reviewing") < 0 {
		t.Errorf("api should offer reviewing: %v", opts)
	}

	// web defines its own shorter workflow.
	m.filterProject = "web"
	opts = m.filterFieldOptions(filterFieldStatus)
	for _, want := range []string{"all", "todo", "doing", "done"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("with web selected %q is missing in %v", want, opts)
		}
	}
	for _, absent := range []string{"backlog", "reviewing", "cancelled"} {
		if filterOptionIndex(opts, absent) >= 0 {
			t.Errorf("with web selected %q should not be offered: %v", absent, opts)
		}
	}
}

// TestFilterStatusClearedOnProjectSwitch verifies that when switching project
// the status filter is cleared if that status does not exist in the new context.
func TestFilterStatusClearedOnProjectSwitch(t *testing.T) {
	m := newTestModel(t)

	// api has "reviewing"; web does not.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "reviewing")
	if m.filterStatus != "reviewing" {
		t.Fatalf("precondition: filterStatus = %q, want reviewing", m.filterStatus)
	}

	m.filterApplySelection(filterFieldProject, "web")
	if m.filterStatus != "" {
		t.Errorf("when switching to web, reviewing does not exist: filterStatus = %q, want \"\"", m.filterStatus)
	}

	// A common status survives the switch.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "doing")
	m.filterApplySelection(filterFieldProject, "web")
	if m.filterStatus != "doing" {
		t.Errorf("doing exists in web: filterStatus = %q, want doing", m.filterStatus)
	}

	// Back to "all projects": only what is in the intersection survives.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "cancelled")
	m.filterApplySelection(filterFieldProject, "all")
	if m.filterStatus != "" {
		t.Errorf("cancelled is not in all: filterStatus = %q, want \"\"", m.filterStatus)
	}
}

// TestFilterStatusOptionsIncludeAllActiveAndAll verifies that the status filter
// offers the two aggregate modes in addition to the project's statuses:
// "all active" (default, no terminals) and "all" (no restriction).
func TestFilterStatusOptionsIncludeAllActiveAndAll(t *testing.T) {
	m := newTestModel(t)

	opts := m.filterFieldOptions(filterFieldStatus)
	for _, want := range []string{"all active", "all"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("status options missing: %q is not in %v", want, opts)
		}
	}

	if got := m.filterCurrentValue(filterFieldStatus); got != "all active" {
		t.Errorf("status filter default = %q, want all active", got)
	}
}

// TestStatusFilterAllActiveExcludesTerminal verifies that the "all active"
// default leaves out done/cancelled, and that "all" includes them again.
func TestStatusFilterAllActiveExcludesTerminal(t *testing.T) {
	m := newTestModel(t)
	markFirstDone(t, m)

	active := len(m.filteredTasks())

	m.filterApplySelection(filterFieldStatus, "all")
	all := len(m.filteredTasks())

	if all != active+1 {
		t.Errorf("all must include the done task: active=%d all=%d", active, all)
	}
}

// TestFilterStatusOptionsAllProjectsIsIntersection verifies that with no
// project selected the picker only offers statuses common to all projects.
func TestFilterStatusOptionsAllProjectsIsIntersection(t *testing.T) {
	m := newTestModel(t)

	opts := m.filterFieldOptions(filterFieldStatus)
	// api (default) ∩ web ([todo,doing,done]) = [todo,doing,done].
	for _, want := range []string{"all", "todo", "doing", "done"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("intersection: %q is missing in %v", want, opts)
		}
	}
	for _, absent := range []string{"backlog", "reviewing", "cancelled"} {
		if filterOptionIndex(opts, absent) >= 0 {
			t.Errorf("the intersection should not include %q (it is not in all): %v", absent, opts)
		}
	}
}

// --- Modal UX ---

// TestFilterModalOpenResetsSearchAndCursor verifies the initial state on open.
func TestFilterModalOpenResetsSearchAndCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	if !m.filterOpen {
		t.Fatal("/ must open the filter modal")
	}
	if m.filterFieldIdx != filterFieldProject {
		t.Errorf("initial focus = %d, want project", m.filterFieldIdx)
	}
	if m.filterSearch != "" {
		t.Errorf("initial search = %q, want empty", m.filterSearch)
	}
	if m.filterOptionIdx != 0 {
		t.Errorf("initial cursor = %d, want 0 (all)", m.filterOptionIdx)
	}
}

// TestFilterModalCursorNavigation verifies ↑↓ over the list of options.
func TestFilterModalCursorNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	m, _ = press(m, "down")
	if m.filterOptionIdx != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.filterOptionIdx)
	}
	m, _ = press(m, "up")
	if m.filterOptionIdx != 0 {
		t.Errorf("after up: cursor = %d, want 0", m.filterOptionIdx)
	}
	// up from 0 goes back to the last one (wrap).
	m, _ = press(m, "up")
	if m.filterOptionIdx != len(m.filterFieldOptions(filterFieldProject))-1 {
		t.Errorf("up from 0 must wrap, cursor = %d", m.filterOptionIdx)
	}
}

// TestFilterModalFuzzySearchApplies verifies that typing filters the options
// and Enter applies the first match and advances the field.
func TestFilterModalFuzzySearchApplies(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	m, _ = press(m, "w") // filters Project to [web, all]
	opts := m.filterVisibleOptions()
	if len(opts) == 0 || opts[0] != "web" {
		t.Fatalf("options = %v, want web first", opts)
	}
	m, _ = press(m, "enter")
	if m.filterProject != "web" {
		t.Errorf("project = %q, want web", m.filterProject)
	}
	if m.filterFieldIdx != filterFieldStatus {
		t.Errorf("enter must advance to status, got %d", m.filterFieldIdx)
	}
}

// TestFilterModalEnterLastFieldCloses verifies that Enter on Tag closes the modal.
func TestFilterModalEnterLastFieldCloses(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")
	for i := 0; i < 4; i++ {
		m, _ = press(m, "tab")
	}
	if m.filterFieldIdx != filterFieldTag {
		t.Fatalf("field = %d, want tag", m.filterFieldIdx)
	}
	m, _ = press(m, "enter")
	if m.filterOpen {
		t.Error("Enter on the last field must close the modal")
	}
}

// TestFilterModalReset verifies that Ctrl+R returns everything to its default,
// including the "all active" status.
func TestFilterModalReset(t *testing.T) {
	m := newTestModel(t)
	m.filterProject = "api"
	m.filterStatus = "doing"
	m.filterAssignee = "@john"
	m.filterPriority = 3
	m.filterTag = "bug"

	m, _ = press(m, "/")
	m, _ = send(m, ctrlR())

	if m.filterProject != "" || m.filterAssignee != "" || m.filterTag != "" {
		t.Errorf("reset left filters: project=%q assignee=%q tag=%q",
			m.filterProject, m.filterAssignee, m.filterTag)
	}
	if m.filterPriority != -1 {
		t.Errorf("priority = %d, want -1", m.filterPriority)
	}
	if m.filterStatus != statusFilterAllActive {
		t.Errorf("status = %q, want %q", m.filterStatus, statusFilterAllActive)
	}
}

// TestFilterModalCycleRemainsLive verifies that ←→ keeps applying live.
func TestFilterModalCycleRemainsLive(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")
	m, _ = press(m, "right") // all -> api
	if m.filterProject != "api" {
		t.Errorf("project = %q, want api", m.filterProject)
	}
}

// TestFilterModalRenders verifies that the modal shows all the fields, the
// live count and does not overflow the terminal width.
func TestFilterModalRenders(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "/")

	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Filters", "Project", "Status", "Assignee", "Priority", "Tag"} {
		if !strings.Contains(out, want) {
			t.Errorf("the modal does not show %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("line of width %d exceeds the width %d: %q", w, m.width, line)
		}
	}
}
