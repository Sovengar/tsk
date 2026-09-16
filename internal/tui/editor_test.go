package tui

import (
	"testing"

	"tsk/internal/model"
)

// TestUpdateTaskFromEditApplied verifica que al editar una tarea:
//  1. los cambios se persisten en la DB
//  2. el comando devuelve un mensaje que refresca la lista (tasksLoadedMsg)
func TestUpdateTaskFromEditApplied(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	content := "# New Title\n\nNew desc\n\n---\nassignee: @bob\npriority: 2\n"

	cmd := m.updateTaskFromEdit(task.ID, content)
	if cmd == nil {
		t.Fatal("updateTaskFromEdit devolvió nil")
	}

	msg := cmd()
	if _, ok := msg.(tasksLoadedMsg); !ok {
		t.Fatalf("se esperaba tasksLoadedMsg, se obtuvo %T", msg)
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

// TestCreateTaskFromEditApplied verifica que al crear una tarea desde el
// editor, el comando devuelve un mensaje que refresca la lista.
func TestCreateTaskFromEditApplied(t *testing.T) {
	m := newTestModel(t)

	content := "# Brand New\n\nsome desc\n\n---\nassignee: @ana\npriority: 1\n"

	cmd := m.createTaskFromEdit("api", content)
	if cmd == nil {
		t.Fatal("createTaskFromEdit devolvió nil")
	}

	msg := cmd()
	loaded, ok := msg.(tasksLoadedMsg)
	if !ok {
		t.Fatalf("se esperaba tasksLoadedMsg, se obtuvo %T", msg)
	}

	found := false
	for _, task := range loaded.tasks {
		if task.Title == "Brand New" && task.Assignee == "@ana" && task.Priority == 1 {
			found = true
		}
	}
	if !found {
		t.Error("la tarea creada no aparece en las tareas recargadas")
	}
}

// TestEditorFinishedFlow refresca la lista al recibir editorFinishedMsg,
// replicando el flujo real: Update(msg) -> cmd -> tasksLoadedMsg.
func TestEditorFinishedFlow(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	content := "# Changed\n\ndesc cambiada\n\n---\nassignee: @bob\npriority: 3\n"

	next, cmd := m.Update(editorFinishedMsg{taskID: task.ID, file: content})
	if cmd == nil {
		t.Fatal("Update(editorFinishedMsg) no devolvió cmd")
	}

	msg := cmd()
	loaded, ok := msg.(tasksLoadedMsg)
	if !ok {
		t.Fatalf("se esperaba tasksLoadedMsg, se obtuvo %T", msg)
	}

	var got *model.Task
	for i := range loaded.tasks {
		if loaded.tasks[i].ID == task.ID {
			got = &loaded.tasks[i]
		}
	}
	if got == nil {
		t.Fatal("la tarea editada no aparece en las tareas recargadas")
	}
	if got.Title != "Changed" || got.Description != "desc cambiada" {
		t.Errorf("tarea sin actualizar: title=%q desc=%q", got.Title, got.Description)
	}

	_ = next
}
