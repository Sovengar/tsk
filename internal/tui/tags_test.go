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

// TestFilterTagOptionsAndMatch verifica que las tags en uso se ofrezcan como
// opciones del filtro y que filtrar por tag recorte las tareas.
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
		t.Fatalf("opciones de tag no incluyen blocked: %v", opts)
	}

	m.filterApplySelection(filterFieldTag, "blocked")
	filtered := m.filteredTasks()
	if len(filtered) == 0 {
		t.Fatal("filtrar por blocked no devolvió tareas")
	}
	for _, tk := range filtered {
		if !model.HasTag(tk.Tags, "blocked") {
			t.Errorf("tarea sin la tag pasó el filtro: %+v", tk)
		}
	}

	m.filterApplySelection(filterFieldTag, "all")
	if m.filterTag != "" {
		t.Errorf("filterTag = %q, want vacío tras all", m.filterTag)
	}
}

// TestEditParsesTags verifica que el editor externo persista las tags.
func TestEditParsesTags(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	content := "# T\n\nD\n\n---\nassignee: @bob\npriority: 1\nestimate: 2\ntags: Blocked, bug\n"
	if cmd := m.updateTaskFromEdit(task.ID, content); cmd == nil {
		t.Fatal("updateTaskFromEdit devolvió nil")
	} else {
		cmd()
	}

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !model.HasTag(got.Tags, "blocked") || !model.HasTag(got.Tags, "bug") {
		t.Errorf("tags = %v, want blocked+bug", got.Tags)
	}
}

// TestKanbanColumnsScopedToProject verifica que al filtrar por un proyecto el
// board use su workflow, no la unión de todos.
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
			t.Errorf("con web falta la columna %q: %v", want, got)
		}
	}
	for _, absent := range []string{"backlog", "reviewing"} {
		if got[absent] {
			t.Errorf("con web no debería estar la columna %q: %v", absent, got)
		}
	}

	m.filterApplySelection(filterFieldProject, "all")
	if !statuses()["backlog"] {
		t.Errorf("en all projects debería volver la unión (backlog): %v", statuses())
	}
}

// TestRenderListShowsTags verifica que la columna Tags se vea en un ancho común.
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
		t.Errorf("con ancho 120 debería verse la cabecera Tags:\n%s", out)
	}
	if !strings.Contains(out, "blocked") {
		t.Errorf("con ancho 120 debería verse la tag blocked:\n%s", out)
	}
}
