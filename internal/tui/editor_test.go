package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/model"
)

// TestUpdateTaskFromEditApplied verifies that when editing a task:
//  1. the changes are persisted to the DB
//  2. the command returns a message that refreshes the list (tasksLoadedMsg)
func TestUpdateTaskFromEditApplied(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	content := "# New Title\n\nNew desc\n\n---\nassignee: @bob\npriority: 2\n"

	cmd := m.updateTaskFromEdit(task.ID, content)
	if cmd == nil {
		t.Fatal("updateTaskFromEdit returned nil")
	}

	msg := mustMsg(t, cmd)
	if _, ok := msg.(tasksLoadedMsg); !ok {
		t.Fatalf("tasksLoadedMsg expected, got %T", msg)
	}

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "New Title" {
		t.Errorf("title = %q, want %q", got.Title, "New Title")
	}
	if got.Description != "New desc" {
		t.Errorf("description = %q, want %q", got.Description, "New desc")
	}
	if got.Assignee != "@bob" {
		t.Errorf("assignee = %q, want %q", got.Assignee, "@bob")
	}
	if got.Priority != 2 {
		t.Errorf("priority = %d, want %d", got.Priority, 2)
	}
}

// TestCreateTaskCmdApplied verifies that when creating a task from the modal,
// the command persists it and returns a message that refreshes the list.
func TestCreateTaskCmdApplied(t *testing.T) {
	m := newTestModel(t)

	cmd := m.createTaskCmd("api", "Brand New", "some desc", "@ann", 1, []string{"backend"})
	if cmd == nil {
		t.Fatal("createTaskCmd returned nil")
	}

	msg := mustMsg(t, cmd)
	loaded, ok := msg.(tasksLoadedMsg)
	if !ok {
		t.Fatalf("tasksLoadedMsg expected, got %T", msg)
	}

	found := false
	for _, task := range loaded.tasks {
		if task.Title == "Brand New" && task.Assignee == "@ann" && task.Priority == 1 &&
			task.Description == "some desc" && len(task.Tags) == 1 && task.Tags[0] == "backend" {
			found = true
		}
	}
	if !found {
		t.Error("the created task does not appear with its data in the reloaded tasks")
	}
}

// TestEditorFinishedFlow refreshes the list upon receiving editorFinishedMsg,
// replicating the real flow: Update(msg) -> cmd -> tasksLoadedMsg.
func TestEditorFinishedFlow(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	content := "# Changed\n\nchanged desc\n\n---\nassignee: @bob\npriority: 3\n"

	next, cmd := m.Update(editorFinishedMsg{taskID: task.ID, file: content})
	if cmd == nil {
		t.Fatal("Update(editorFinishedMsg) did not return a cmd")
	}

	msg := mustMsg(t, cmd)
	loaded, ok := msg.(tasksLoadedMsg)
	if !ok {
		t.Fatalf("tasksLoadedMsg expected, got %T", msg)
	}

	var got *model.Task
	for i := range loaded.tasks {
		if loaded.tasks[i].ID == task.ID {
			got = &loaded.tasks[i]
		}
	}
	if got == nil {
		t.Fatal("the edited task does not appear in the reloaded tasks")
	}
	if got.Title != "Changed" || got.Description != "changed desc" {
		t.Errorf("task not updated: title=%q desc=%q", got.Title, got.Description)
	}

	_ = next
}

// If the temporary file cannot be created, the command returns the error with
// the task id and does not launch the editor. TMPDIR is the entry point:
// os.CreateTemp honors it, so pointing it at a non-existent directory makes
// the creation fail without having to inject anything in production.
func TestEditTaskCmdReportsTempFileFailure(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-existe"))

	task := model.Task{ID: 42, Title: "t"}
	msg := editTaskCmd(task, "true")()
	finished, ok := msg.(editorFinishedMsg)
	if !ok {
		t.Fatalf("message %T, want editorFinishedMsg", msg)
	}
	if finished.err == nil {
		t.Fatal("the temp file creation failure was not reported")
	}
	if finished.taskID != task.ID {
		t.Errorf("taskID = %d, want %d", finished.taskID, task.ID)
	}
	if finished.file != "" {
		t.Errorf("file = %q, want empty with no file", finished.file)
	}
}

// The editor receives a file with the task template inside, and the command
// only returns the message if the editor finished without error.
func TestEditTemplateCarriesTheTask(t *testing.T) {
	task := model.Task{
		ID:          7,
		Title:       "Fix the N+1",
		Description: "Cascade in three tables",
		Status:      "doing",
		Assignee:    "@john",
		Priority:    2,
		Estimate:    2.5,
		Tags:        []string{"db", "perf"},
	}

	content := editTemplate(task)
	for _, want := range []string{
		task.Title,
		task.Description,
		task.Assignee,
		"2",       // priority
		"2.5",     // estimate with decimals, not rounded
		"db,perf", // tags joined by commas
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the template does not carry %q:\n%s", want, content)
		}
	}

	// The state does NOT go into the template, on purpose: editing the file must not
	// be able to move the task's state by accident, because the state flow has its
	// own path (s/d/x) and its own project rules.
	if strings.Contains(content, task.Status) {
		t.Errorf("the template carries the status %q, and it should not:\n%s", task.Status, content)
	}
}
