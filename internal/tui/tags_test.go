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
// board uses its workflow, not the union of all of them.
func TestKanbanColumnsScopedToProject(t *testing.T) {
	m := newTestModel(t)

	statuses := func() map[string]bool {
		set := map[string]bool{}
		for _, c := range m.kanbanColumns() {
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
