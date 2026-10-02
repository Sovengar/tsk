package tui

import (
	"path/filepath"
	"strings"
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

	msg := mustMsg(t, cmd)
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

// TestCreateTaskCmdApplied verifica que al crear una tarea desde el modal, el
// comando la persiste y devuelve un mensaje que refresca la lista.
func TestCreateTaskCmdApplied(t *testing.T) {
	m := newTestModel(t)

	cmd := m.createTaskCmd("api", "Brand New", "some desc", "@ana", 1, []string{"backend"})
	if cmd == nil {
		t.Fatal("createTaskCmd devolvió nil")
	}

	msg := mustMsg(t, cmd)
	loaded, ok := msg.(tasksLoadedMsg)
	if !ok {
		t.Fatalf("se esperaba tasksLoadedMsg, se obtuvo %T", msg)
	}

	found := false
	for _, task := range loaded.tasks {
		if task.Title == "Brand New" && task.Assignee == "@ana" && task.Priority == 1 &&
			task.Description == "some desc" && len(task.Tags) == 1 && task.Tags[0] == "backend" {
			found = true
		}
	}
	if !found {
		t.Error("la tarea creada no aparece con sus datos en las tareas recargadas")
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

	msg := mustMsg(t, cmd)
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

// Si no se puede crear el fichero temporal, el comando devuelve el error con el
// id de la tarea y no lanza el editor. TMPDIR es el punto de entrada: os.CreateTemp
// lo respeta, así que apuntarlo a un directorio inexistente hace fallar la
// creación sin necesidad de inyectar nada en producción.
func TestEditTaskCmdReportsTempFileFailure(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-existe"))

	task := model.Task{ID: 42, Title: "t"}
	msg := editTaskCmd(task, "true")()
	finished, ok := msg.(editorFinishedMsg)
	if !ok {
		t.Fatalf("mensaje %T, want editorFinishedMsg", msg)
	}
	if finished.err == nil {
		t.Fatal("no se reportó el fallo de crear el temporal")
	}
	if finished.taskID != task.ID {
		t.Errorf("taskID = %d, want %d", finished.taskID, task.ID)
	}
	if finished.file != "" {
		t.Errorf("file = %q, want vacío sin fichero", finished.file)
	}
}

// El editor recibe un fichero con el template de la tarea dentro, y el comando
// sólo devuelve el mensaje si el editor terminó sin error.
func TestEditTemplateCarriesTheTask(t *testing.T) {
	task := model.Task{
		ID:          7,
		Title:       "Arreglar el N+1",
		Description: "Cascade en tres tablas",
		Status:      "doing",
		Assignee:    "@juan",
		Priority:    2,
		Estimate:    2.5,
		Tags:        []string{"db", "perf"},
	}

	contenido := editTemplate(task)
	for _, want := range []string{
		task.Title,
		task.Description,
		task.Assignee,
		"2",       // prioridad
		"2.5",     // estimate con decimales, no redondeado
		"db,perf", // tags separadas por comas
	} {
		if !strings.Contains(contenido, want) {
			t.Errorf("el template no lleva %q:\n%s", want, contenido)
		}
	}

	// El estado NO va en el template, a propósito: editar el fichero no debe poder
	// mover la tarea de estado por accidente, porque el flujo de estados tiene su
	// propio camino (s/d/x) y sus propias reglas de proyecto.
	if strings.Contains(contenido, task.Status) {
		t.Errorf("el template lleva el estado %q, y no debería:\n%s", task.Status, contenido)
	}
}
