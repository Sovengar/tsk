package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

func hasOption(opts []string, want string) bool {
	for _, o := range opts {
		if o == want {
			return true
		}
	}
	return false
}

// TestFilterTagOptionsAndMatch verifies that the tags in use are offered as
// filter options and that filtering by tag trims the tasks.
func TestFilterTagOptionsAndMatch(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	if _, err := m.database.SetTaskTags(task.ID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()

	if opts := m.filterFieldOptions(filterFieldTag); !hasOption(opts, "blocked") {
		t.Fatalf("tag options do not include blocked: %v", opts)
	}

	m.filterApplySelection(filterFieldTag, "blocked")
	filtered := m.filteredTasks()
	if len(filtered) == 0 {
		t.Fatal("filtering by blocked returned no tasks")
	}
	for _, tk := range filtered {
		if !model.HasTag(tk.Tags, "blocked") {
			t.Errorf("a task without the tag passed the filter: %+v", tk)
		}
	}

	m.filterApplySelection(filterFieldTag, "all")
	if m.filterTag != "" {
		t.Errorf("filterTag = %q, want empty after all", m.filterTag)
	}
}

// TestEditParsesTags verifies that the external editor persists the tags.
func TestEditParsesTags(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	content := "# T\n\nD\n\n---\nassignee: @bob\npriority: 1\nestimate: 2\ntags: Blocked, bug\n"
	if cmd := m.updateTaskFromEdit(task.ID, content); cmd == nil {
		t.Fatal("updateTaskFromEdit returned nil")
	} else {
		mustMsg(t, cmd)
	}

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !model.HasTag(got.Tags, "blocked") || !model.HasTag(got.Tags, "bug") {
		t.Errorf("tags = %v, want blocked+bug", got.Tags)
	}
}

// TestKanbanColumnsScopedToProject verifies that when filtering by a project the
// board uses its workflow, not the union of all of them. The statuses come from
// kanbanStatuses, because the empty columns are hidden from the board.
func TestKanbanColumnsScopedToProject(t *testing.T) {
	m := newTestModel(t)

	statuses := func() map[string]bool {
		set := map[string]bool{}
		for _, c := range m.kanbanStatuses() {
			set[c.status] = true
		}
		return set
	}

	m.filterApplySelection(filterFieldProject, "web") // web = [todo, doing, done]
	got := statuses()
	for _, want := range []string{"todo", "doing", "done"} {
		if !got[want] {
			t.Errorf("with web the column %q is missing: %v", want, got)
		}
	}
	for _, absent := range []string{"backlog", "reviewing"} {
		if got[absent] {
			t.Errorf("with web the column %q should not be there: %v", absent, got)
		}
	}

	m.filterApplySelection(filterFieldProject, "all")
	if !statuses()["backlog"] {
		t.Errorf("in all projects the union (backlog) should return: %v", statuses())
	}
}

// TestRenderListShowsTags verifies that the Tags column is visible at a common width.
func TestRenderListShowsTags(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	if _, err := m.database.SetTaskTags(task.ID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks
	m.invalidateFilterCache()
	m.width = 120

	out := ansi.Strip(m.renderList(m.height))
	if !strings.Contains(out, "Tags") {
		t.Errorf("with width 120 the Tags header should be visible:\n%s", out)
	}
	if !strings.Contains(out, "blocked") {
		t.Errorf("with width 120 the tag blocked should be visible:\n%s", out)
	}
}

// setTaskTagByTitle writes tags on the task whose title matches, in the given
// project, and reloads the model's task list.
func setTaskTagByTitle(t *testing.T, m *Model, projectName, title string, tags []string) {
	t.Helper()
	for _, tk := range m.tasks {
		if tk.ProjectName == projectName && tk.Title == title {
			if _, err := m.database.SetTaskTags(tk.ID, tags); err != nil {
				t.Fatalf("SetTaskTags(%q): %v", title, err)
			}
			tasks, _ := m.database.ListTasks("", "", "")
			m.tasks = tasks
			m.invalidateFilterCache()
			return
		}
	}
	t.Fatalf("task %q of project %q not found", title, projectName)
}

// TestFilterTagOptionsScopedToProject verifies that with a project selected
// the tag picker offers only the tags in use in that project, and that in
// "all projects" it offers the union.
func TestFilterTagOptionsScopedToProject(t *testing.T) {
	m := newTestModel(t)
	setTaskTagByTitle(t, m, "api", "Fix N+1 query", []string{"backend"})
	setTaskTagByTitle(t, m, "web", "Fix checkout", []string{"frontend"})

	// In "all projects" both are offered.
	opts := m.filterFieldOptions(filterFieldTag)
	for _, want := range []string{"all", "backend", "frontend"} {
		if !hasOption(opts, want) {
			t.Errorf("all projects: %q is missing in %v", want, opts)
		}
	}

	// With api selected, only api's tags.
	m.filterProject = "api"
	opts = m.filterFieldOptions(filterFieldTag)
	if !hasOption(opts, "backend") {
		t.Errorf("with api selected backend is missing in %v", opts)
	}
	if hasOption(opts, "frontend") {
		t.Errorf("with api selected frontend should not be offered: %v", opts)
	}

	// With web selected, only web's tags.
	m.filterProject = "web"
	opts = m.filterFieldOptions(filterFieldTag)
	for _, absent := range []string{"backend"} {
		if hasOption(opts, absent) {
			t.Errorf("with web selected %q should not be offered: %v", absent, opts)
		}
	}
	if !hasOption(opts, "frontend") {
		t.Errorf("with web selected frontend is missing in %v", opts)
	}
}

// TestFilterTagClearedOnProjectSwitch verifies that switching project clears a
// tag filter that is not in use in the new project, and keeps it when it is.
func TestFilterTagClearedOnProjectSwitch(t *testing.T) {
	m := newTestModel(t)
	setTaskTagByTitle(t, m, "api", "Fix N+1 query", []string{"backend"})
	setTaskTagByTitle(t, m, "web", "Fix checkout", []string{"backend"})

	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldTag, "backend")
	if m.filterTag != "backend" {
		t.Fatalf("precondition: filterTag = %q, want backend", m.filterTag)
	}

	// A tag used by both projects survives the switch.
	m.filterApplySelection(filterFieldProject, "web")
	if m.filterTag != "backend" {
		t.Errorf("backend exists in web: filterTag = %q, want backend", m.filterTag)
	}

	// A tag of web only does not survive a switch back to api.
	setTaskTagByTitle(t, m, "web", "Fix checkout", []string{"frontend"})
	m.filterApplySelection(filterFieldTag, "frontend")
	m.filterApplySelection(filterFieldProject, "api")
	if m.filterTag != "" {
		t.Errorf("frontend is not in api: filterTag = %q, want empty", m.filterTag)
	}

	// Back to "all projects": the union keeps a web-only tag alive.
	m.filterApplySelection(filterFieldProject, "web")
	m.filterApplySelection(filterFieldTag, "frontend")
	m.filterApplySelection(filterFieldProject, "all")
	if m.filterTag != "frontend" {
		t.Errorf("frontend is in the union: filterTag = %q, want frontend", m.filterTag)
	}
}
