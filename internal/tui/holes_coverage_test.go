package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/model"
)

// Los dos callbacks del editor externo, ya sacados de sus tea.Cmd, tienen tres
// desenlaces cada uno. Lo que se comprueba aquí es que el mensaje que sale
// lleva el id de la tarea, porque sin él la interfaz no sabe a quién pertenece
// el resultado y el comentario se pierde.

func TestCommentCallback(t *testing.T) {
	t.Run("el editor falló", func(t *testing.T) {
		path := escribirTemporal(t, "contenido")
		msg := commentCallback(7, path)(errAccionFallida)

		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("un editor fallido no ha producido error")
		}
		if finished.taskID != 7 {
			t.Errorf("taskID = %d, want 7", finished.taskID)
		}
		if finished.body != "" {
			t.Errorf("body = %q, want vacío con el editor fallido", finished.body)
		}
	})

	t.Run("el editor escribió", func(t *testing.T) {
		path := escribirTemporal(t, "  un comentario  \n\n")
		msg := commentCallback(7, path)(nil)

		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentFinishedMsg", msg)
		}
		if finished.err != nil {
			t.Fatalf("hay error: %v", finished.err)
		}
		// Recortado: un comentario no empieza ni acaba en espacio en blanco.
		if finished.body != "un comentario" {
			t.Errorf("body = %q, want %q recortado", finished.body, "un comentario")
		}
		if finished.taskID != 7 {
			t.Errorf("taskID = %d, want 7", finished.taskID)
		}
	})

	t.Run("el temporal ya no está", func(t *testing.T) {
		msg := commentCallback(7, t.TempDir()+"/nunca-fue")(nil)
		if finished := msg.(commentFinishedMsg); finished.err == nil {
			t.Error("un temporal inexistente no ha producido error")
		}
	})
}

func TestEditorCallback(t *testing.T) {
	t.Run("el editor falló", func(t *testing.T) {
		path := escribirTemporal(t, "algo")
		msg := editorCallback(9, path)(errAccionFallida)

		finished, ok := msg.(editorFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want editorFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("un editor fallido no ha producido error")
		}
		if finished.taskID != 9 {
			t.Errorf("taskID = %d, want 9", finished.taskID)
		}
		if finished.file != "" {
			t.Errorf("file = %q, want vacío con el editor fallido", finished.file)
		}
	})

	t.Run("el editor escribió", func(t *testing.T) {
		// El editor externo NO recorta: el formato que genera lleva separadores
		// y saltos de línea que el parser necesita.
		path := escribirTemporal(t, "Title: nuevo\n---\nStatus: todo\n")
		msg := editorCallback(9, path)(nil)

		finished := msg.(editorFinishedMsg)
		if finished.err != nil {
			t.Fatalf("hay error: %v", finished.err)
		}
		if !strings.Contains(finished.file, "Title: nuevo") {
			t.Errorf("file = %q, want el contenido tal cual", finished.file)
		}
	})

	t.Run("el temporal ya no está", func(t *testing.T) {
		msg := editorCallback(9, t.TempDir()+"/nunca-fue")(nil)
		if finished := msg.(editorFinishedMsg); finished.err == nil {
			t.Error("un temporal inexistente no ha producido error")
		}
	})
}

func escribirTemporal(t *testing.T, contenido string) string {
	t.Helper()
	path := t.TempDir() + "/editado.md"
	if err := os.WriteFile(path, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// La carga de proyectos tiene dos consultas: la de activos y la de archivados.
// Si la segunda falla, el programa sigue con la primera: perder el panel de
// archivados es mucho mejor que no arrancar.
func TestLoadProjectsSurvivesABrokenArchivedQuery(t *testing.T) {
	m := newTestModel(t)

	sabotearProjects(t, m)

	msg := mustMsg(t, m.loadProjects())
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("el mensaje es %T, want projectsLoadedMsg", msg)
	}
	if loaded.archivedProjects != nil {
		t.Errorf("con la tabla de proyectos rota hay %d archivados, want nil",
			len(loaded.archivedProjects))
	}
}

// Poner y quitar una tag son dos escrituras; si la base falla, el comando
// devuelve nil en vez de un mensaje con datos a medias.
func TestToggleTagWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, tag := range []string{"nueva", ""} {
		if msg := m.toggleTagCmd(m.tasks[0].ID, tag)(); msg != nil {
			t.Errorf("toggleTag(%q) con la base cerrada ha devuelto %T, want nil", tag, msg)
		}
	}
}

// Redimensionar con el alta de tarea abierta tiene que reajustar el textarea.
// Sin eso, el textarea conserva el ancho con el que se construyó y se ve
// torcido en cuanto la ventana cambia de tamaño.
func TestWindowResizeAdjustsTheNewTaskTextarea(t *testing.T) {
	m := newTestModel(t)
	abierto, _ := pulsar(t, m, "i")

	abierto.width = 200
	redimensionado, _ := updateMsg(t, abierto, tea.WindowSizeMsg{Width: 200, Height: 40})

	if redimensionado.newTaskTextarea.Width() != redimensionado.newTaskTextareaWidth() {
		t.Errorf("el textarea mide %d, want el ancho recalculado %d",
			redimensionado.newTaskTextarea.Width(), redimensionado.newTaskTextareaWidth())
	}
	if redimensionado.statusbar.width != 200 {
		t.Errorf("la barra de estado mide %d, want 200", redimensionado.statusbar.width)
	}
	if redimensionado.preview.width != 200 {
		t.Errorf("la caja de descripción mide %d, want 200", redimensionado.preview.width)
	}
}

// El pegado va primero al editor de descripción, luego al alta, luego al modal
// de tags. Con dos de ellos abiertos a la vez, el orden decide cuál gana.
func TestPasteGoesToTheDescriptionEditorFirst(t *testing.T) {
	m := newTestModel(t)
	// El editor se abre de verdad y encima se abre el alta: con los dos
	// montados a la vez, el pegado tiene que decidir a cuál va.
	conEditor, _ := pulsar(t, m, "e")
	if !conEditor.descEditOpen {
		t.Fatal("e no ha abierto el editor de descripción")
	}
	conEditor.newTaskOpen = true

	siguiente, _ := updateMsg(t, conEditor, tea.PasteMsg{Content: "pegado"})
	if !strings.Contains(siguiente.descEditTextarea.Value(), "pegado") {
		t.Errorf("el pegado no ha llegado al editor de descripción: %q", siguiente.descEditTextarea.Value())
	}
	if strings.Contains(siguiente.newTaskTitle, "pegado") {
		t.Error("el pegado ha llegado también al alta de tarea")
	}
}

// Cualquier mensaje que no sea de Bubbletea va al editor de descripción si está
// abierto, o al textarea del alta si el cursor está en el campo de descripción.
// Es el mecanismo que deja que los textareas hablen por su cuenta.
func TestUnknownMessagesReachTheOpenEditor(t *testing.T) {
	t.Run("el editor de descripción", func(t *testing.T) {
		m := newTestModel(t)
		m.descEditOpen = true

		siguiente, _ := updateMsg(t, m, msgPrivado{})
		if !siguiente.descEditOpen {
			t.Error("un mensaje desconocido ha cerrado el editor de descripción")
		}
	})

	t.Run("el textarea del alta en el campo de descripción", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldDescription

		siguiente, cmd := updateMsg(t, m, msgPrivado{})
		if cmd != nil {
			t.Error("un mensaje desconocido ha lanzado un comando del textarea")
		}
		if !siguiente.newTaskOpen {
			t.Error("un mensaje desconocido ha cerrado el alta")
		}
	})

	t.Run("nada abierto", func(t *testing.T) {
		m := newTestModel(t)
		siguiente, cmd := updateMsg(t, m, msgPrivado{})
		if cmd != nil {
			t.Error("un mensaje desconocido sin nada abierto ha lanzado un comando")
		}
		if !strings.Contains(ansi.Strip(siguiente.View().Content), "Fix") {
			t.Error("un mensaje desconocido ha cambiado el render")
		}
	})
}

// msgPrivado es un tipo que ningún switch de Update conoce: representa los
// mensajes privados del paquete que emiten los textareas.
type msgPrivado struct{}

func (msgPrivado) String() string { return "privado" }

// Una vista que no existe no debe dejar la TUI sin pintar nada: cae al
// dashboard.
func TestUnknownViewFallsBack(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewKind(42)

	if _, cmd := pulsar(t, m, "j"); cmd != nil {
		t.Error("una tecla en una vista inexistente ha lanzado un comando")
	}

	m.width, m.height = 100, 30
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Total") {
		t.Errorf("una vista inexistente no ha salido por el dashboard:\n%s", out)
	}
}

// En el kanban, h y l no dan la vuelta: se quedan en el borde. Es lo que
// distingue shiftIndex de cycleIndex.
func TestKanbanColumnsDoNotWrap(t *testing.T) {
	m := newKanbanModel(t, 3)
	m.currentView = viewKanban
	cols := len(m.kanbanColumns())
	if cols < 2 {
		t.Skip("el fixture necesita dos columnas")
	}

	m.kanbanCol = 0
	izquierda, _ := pulsar(t, m, "h")
	if izquierda.kanbanCol != 0 {
		t.Errorf("h en la primera columna ha ido a la %d, want quedarse en 0", izquierda.kanbanCol)
	}

	m.kanbanCol = cols - 1
	derecha, _ := pulsar(t, m, "l")
	if derecha.kanbanCol != cols-1 {
		t.Errorf("l en la última columna ha ido a la %d, want quedarse en %d",
			derecha.kanbanCol, cols-1)
	}
}

// S mueve la tarjeta hacia atrás en el workflow de SU proyecto. Es el
// camino que hoy no se pulsaba y el que más se rompe cuando el workflow del
// proyecto cambia.
func TestKanbanMoveLeft(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 4, []string{"backlog", "todo", "doing", "done"})
	m.currentView = viewKanban

	// Sitúa el cursor en una tarjeta que no está en el primer estado.
	cols := m.kanbanColumns()
	col := -1
	for i, c := range cols {
		if len(c.tasks) > 0 {
			col = i
			break
		}
	}
	if col < 0 {
		t.Skip("el fixture no ha dejado tarjetas")
	}
	m.kanbanCol = col
	m.kanbanRow = 0

	id := cols[col].tasks[0].ID
	estado := cols[col].tasks[0].Status

	_, cmd := pulsar(t, m, "S")
	if cmd == nil {
		t.Fatal("S no ha lanzado ninguna acción")
	}
	mustRun(t, cmd)

	tarea, err := m.database.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", id, err)
	}
	if tarea.Status == estado {
		t.Errorf("la tarea sigue en %q tras moverla a la izquierda", estado)
	}
	if prev, _ := model.PrevStatus(m.projects[0].Workflow, estado); tarea.Status != prev {
		t.Errorf("la tarea ha quedado en %q, want el estado anterior %q", tarea.Status, prev)
	}
}

// El comentario desde el detalle abre el editor externo sobre la tarea
// abierta. Con la tarea abierta y sin comentarios, la tecla tiene que lanzar el
// comando.
func TestDetailOpensTheCommentEditor(t *testing.T) {
	m := newDetailModel(t, 0)

	if _, cmd := pulsar(t, m, "c"); cmd == nil {
		t.Error("c no ha lanzado el editor de comentarios")
	}
}

// selectedTask indexa el tablero sin mirar el rango del cursor de columna. Con
// el filtro puesto, el número de columnas cambia entre un render y el siguiente,
// así que el índice puede quedar fuera y hay que salir en vez de indexar un
// slice vacío.
func TestSelectedTaskWithACursorOutOfRange(t *testing.T) {
	m := newKanbanModel(t, 3)
	m.currentView = viewKanban

	if tsk := m.selectedTask(); tsk == nil {
		t.Fatal("el fixture no ha dejado ninguna tarea enfocada")
	}

	m.kanbanCol = 9999
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("con la columna fuera de rango hay tarea seleccionada: %q", tsk.Title)
	}

	// Y lo mismo en la lista, donde el cursor puede quedar pasado tras un filtro.
	m.currentView = viewList
	m.tasks = nil
	m.filteredT = nil
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("con la lista vacía hay tarea seleccionada: %q", tsk.Title)
	}
}

// El guardado del editor inline de descripción son dos escrituras: la tarea y
// el refresco del listado. Si cualquiera de las dos falla, no hay recarga, y el
// modelo se queda con lo que tenía.
func TestSaveDescriptionWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if msg := m.saveDescriptionCmd(id, "nueva descripción")(); msg != nil {
		t.Errorf("un guardado fallido ha devuelto %T, want nil", msg)
	}
}

// El editor inline necesita una tarea de la que copiar el contenido. Sin ella
// no hay nada que editar.
func TestOpenDescEditorWithoutATask(t *testing.T) {
	m := newTestModel(t)
	m.tasks = nil
	m.filteredT = nil
	m.filterActive = false

	if cmd := m.openDescEditor(); cmd != nil {
		t.Error("sin tarea el editor de descripción ha lanzado un comando")
	}
	if m.descEditOpen {
		t.Error("sin tarea se ha abierto el editor de descripción")
	}
}

// El alta de tarea escribe en la base y luego recarga el listado. Con la base
// cerrada, el primer error es el que ve el usuario y el mensaje lleva el motivo.
func TestCreateNewTaskWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	abierto, _ := pulsar(t, m, "i")
	conTitulo, _ := pulsar(t, abierto, "n", "u", "e", "v", "a")
	if strings.TrimSpace(conTitulo.newTaskTitle) != "nueva" {
		t.Fatalf("el título es %q, want nueva", conTitulo.newTaskTitle)
	}

	if err := conTitulo.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, cmd := pulsar(t, conTitulo, "enter")
	if cmd == nil {
		t.Fatal("enter no ha intentado crear la tarea")
	}
	failed, ok := firstTaskFailure(t, cmd)
	if !ok {
		t.Fatal("el alta con la base cerrada no ha emitido taskCreateFailedMsg")
	}
	if failed.err == nil {
		t.Error("crear con la base cerrada no ha producido error")
	}
}

// El campo de tags del alta no lleva textarea: es texto separado por comas, y
// con el campo enfocado y vacío tiene que enseñar su pista junto al cursor.
func TestNewTaskTagsFieldShowsItsHint(t *testing.T) {
	m := newTestModel(t)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags

	out := ansi.Strip(m.renderNewTaskTags())
	if !strings.Contains(out, "type to add") {
		t.Errorf("el campo de tags vacío no muestra su pista: %q", out)
	}
	if !strings.Contains(out, cursorGlyph) {
		t.Errorf("la pista del campo de tags no trae el cursor: %q", out)
	}

	// Sin foco el mismo campo vacío enseña el guion largo de los otros.
	m.newTaskFieldIdx = newTaskFieldTitle
	if out := ansi.Strip(m.renderNewTaskTags()); !strings.Contains(out, "—") {
		t.Errorf("sin foco y sin tags el campo sale vacío en vez de \"—\": %q", out)
	}

	// Y con algo escrito la pista desaparece: es una pista, no un valor.
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "nue"
	out = ansi.Strip(m.renderNewTaskTags())
	if strings.Contains(out, "type to add") {
		t.Errorf("con texto escrito la pista sigue ahí: %q", out)
	}
	if !strings.Contains(out, "nue") {
		t.Errorf("lo escrito no se ve: %q", out)
	}

	// Con tags ya puestos se ven los tags, no el cursor.
	m.newTaskTagInput = ""
	m.newTaskTags = []string{"uno", "dos"}
	out = ansi.Strip(m.renderNewTaskTags())
	for _, want := range []string{"uno", "dos"} {
		if !strings.Contains(out, want) {
			t.Errorf("con tags puestos no se ve %q: %q", want, out)
		}
	}
	if strings.Contains(out, "type to add") {
		t.Errorf("con tags puestos la pista sigue ahí: %q", out)
	}
}

// Una prioridad fuera del rango conocido se renderiza como "sin prioridad", no
// como un carácter basura. El rango lo impone la base, pero el render no
// debería confiar en eso.
func TestRenderPriorityOutOfRange(t *testing.T) {
	m := newTestModel(t)
	m.tasks[0].Priority = 7
	m.width, m.height = 100, 30

	out := ansi.Strip(m.renderList(20))
	if !strings.Contains(out, "Fix checkout") {
		t.Fatalf("la tarea no se ve en la lista:\n%s", out)
	}
	// No debe aparecer ningún carácter de prioridad en su fila.
	for _, linea := range strings.Split(out, "\n") {
		if strings.Contains(linea, "Fix checkout") && strings.ContainsAny(linea, "HLMN") {
			t.Errorf("una prioridad 7 se ha renderizado como un carácter conocido: %q", linea)
		}
	}
}

// La barra de keybinds cambia el color del borde según tenga el foco. Es la
// única señal de "esto es lo que estás leyendo", así que las dos ramas tienen que
// producir renders distintos de verdad.
func TestKeybindsBarBorderFollowsFocus(t *testing.T) {
	base := KeybindsBar{width: 60, view: viewList}

	conFoco := base
	conFoco.focused = true
	sinFoco := base

	if conFoco.View() == sinFoco.View() {
		t.Error("el borde es idéntico con y sin foco, want dos colores distintos")
	}

	// Y con el foco puesto, el código de color del borde es el azul del que
	// habla el comentario, no el gris por defecto.
	if !strings.Contains(conFoco.View(), "\x1b[94m") {
		t.Errorf("con el foco el borde no es azul (94): %q", conFoco.View())
	}
	if strings.Contains(sinFoco.View(), "\x1b[94m") {
		t.Errorf("sin foco el borde sale azul: %q", sinFoco.View())
	}
}

// Una lista sin tareas lo dice, en vez de dejar un hueco vacío que parece un
// fallo de carga.
func TestEmptyListSaysSo(t *testing.T) {
	m := newBareModel(t, func(*config.Config) {})
	m.width, m.height = 100, 20
	m.tasks = nil
	m.filteredT = nil

	out := ansi.Strip(m.renderList(18))
	if !strings.Contains(out, "No tasks found") {
		t.Errorf("una lista vacía no lo dice:\n%s", out)
	}
}

// Con el filtro de prioridad puesto, el valor legible que se muestra en el
// modal tiene que ser el mismo con el que se filtró la lista. Si no, el filtro
// actúa sobre un valor que la interfaz no enseña.
func TestFilterPriorityAndItsLabel(t *testing.T) {
	m := newTestModel(t)

	for _, valor := range []string{"none", "low", "med", "high"} {
		m.filterApplySelection(filterFieldPriority, valor)
		if got := m.filterCurrentValue(filterFieldPriority); got != valor {
			t.Errorf("puesto %q se muestra %q", valor, got)
		}
	}
}

// sabotearProjects deja la tabla de proyectos con tipos que no se pueden leer,
// de modo que la consulta de archivados falle sin que la de activos lo haga.
// Es la forma de probar que una consulta secundaria rota no tira el arranque.
func sabotearProjects(t *testing.T, m *Model) {
	t.Helper()
	if _, err := m.database.Conn().Exec(`DROP TABLE projects`); err != nil {
		t.Fatalf("DROP TABLE projects: %v", err)
	}
	if _, err := m.database.Conn().Exec(
		`CREATE TABLE projects (id BLOB, name BLOB, workflow BLOB, list_order BLOB, archived BLOB)`,
	); err != nil {
		t.Fatalf("CREATE TABLE projects: %v", err)
	}
}

// firstTaskFailure desenvuelve el lote que devuelve el alta y devuelve el primer
// taskCreateFailedMsg que encuentre.
func firstTaskFailure(t *testing.T, cmd tea.Cmd) (taskCreateFailedMsg, bool) {
	t.Helper()
	for _, msg := range mustRun(t, cmd) {
		if failed, ok := msg.(taskCreateFailedMsg); ok {
			return failed, true
		}
	}
	return taskCreateFailedMsg{}, false
}
