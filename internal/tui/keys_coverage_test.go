package tui

import (
	"strings"
	"testing"

	"tsk/internal/config"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// Los key handlers por vista son casi todos tablas de casos, y los tests
// existentes pulsaban cuatro o cinco teclas por vista. Lo que faltaba era la
// cola de la tabla: las teclas que sólo funcionan en un estado concreto
// (archivados visibles, comentario seleccionado, columna vacía) y las que no
// hacen nada. Estas son esas teclas.

// pulsar encadena una secuencia de teclas y devuelve el modelo resultante.
func pulsar(t *testing.T, m *Model, teclas ...string) (*Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range teclas {
		m, cmd = press(m, k)
	}
	return m, cmd
}

func TestDashboardKeys(t *testing.T) {
	t.Run("k y j van en direcciones distintas", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		// Con dos proyectos, +1 y -1 desde el índice 0 dan el mismo resultado y
		// no se distinguen: hace falta un tercero para que las dos direcciones
		// digamos cosas diferentes.
		mustCreateProject(t, m.database, "extra", model.DefaultWorkflow)
		reloadProjects(t, m)
		if len(m.dashProjectList()) < 3 {
			t.Fatalf("el fixture ha dejado %d proyectos visibles", len(m.dashProjectList()))
		}

		arriba, _ := pulsar(t, m, "k")
		if arriba.dashProjectIdx != len(m.dashProjectList())-1 {
			t.Errorf("k desde 0 ha dejado el índice en %d, want el último (%d)",
				arriba.dashProjectIdx, len(m.dashProjectList())-1)
		}

		abajo, _ := pulsar(t, arriba, "j")
		if abajo.dashProjectIdx != 0 {
			t.Errorf("j desde el último ha dejado el índice en %d, want 0 (dar la vuelta)", abajo.dashProjectIdx)
		}
	})

	t.Run("e abre el modal de edición del proyecto seleccionado", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		siguiente, cmd := pulsar(t, m, "e")
		if !siguiente.projectModalOpen {
			t.Fatal("e no ha abierto el modal de proyecto")
		}
		if !siguiente.projectModalEdit {
			t.Error("el modal se ha abierto en modo creación, want edición")
		}
		_ = cmd
	})

	t.Run("e sin proyecto seleccionado no hace nada", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard
		m.dashProjectIdx = 9999
		m.projects = nil

		siguiente, _ := pulsar(t, m, "e")
		if siguiente.projectModalOpen {
			t.Error("e ha abierto el modal sin ningún proyecto seleccionado")
		}
	})

	t.Run("d pide confirmar el archivado", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		siguiente, _ := pulsar(t, m, "d")
		if !siguiente.confirmOpen || siguiente.confirmAction != "archive" {
			t.Errorf("d no ha pedido confirmar el archivado: open=%v action=%q",
				siguiente.confirmOpen, siguiente.confirmAction)
		}
	})

	t.Run("d no hace nada con los archivados a la vista", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard
		m.showArchived = true
		m.archivedProjects = m.projects

		siguiente, _ := pulsar(t, m, "d")
		if siguiente.confirmOpen {
			t.Error("d ha archivado un proyecto que ya está en la lista de archivados")
		}
	})

	t.Run("r desarchiva, pero sólo cuando los archivados están a la vista", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		oculto, _ := pulsar(t, m, "r")
		if oculto.confirmOpen {
			t.Error("r ha pedido desarchivar con la lista de archivados oculta")
		}

		m.showArchived = true
		m.archivedProjects = m.projects
		visible, _ := pulsar(t, m, "r")
		if !visible.confirmOpen || visible.confirmAction != "unarchive" {
			t.Errorf("r no ha pedido confirmar el desarchivado: open=%v action=%q",
				visible.confirmOpen, visible.confirmAction)
		}
	})

	t.Run("m abre la gestión de off-days y carga las días libres", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard

		siguiente, cmd := pulsar(t, m, "m")
		if !siguiente.assigneeModalOpen {
			t.Fatal("m no ha abierto el modal de assignees")
		}
		if cmd == nil {
			t.Error("m no ha lanzado la carga de off-days")
		}
	})
}

func TestListKeys(t *testing.T) {
	t.Run("ctrl+p sube la prioridad de la tarea del cursor", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.cursor = 0

		id := m.filteredTasks()[0].ID
		antes := m.filteredTasks()[0].Priority
		if _, cmd := pulsar(t, m, "ctrl+p"); cmd == nil {
			t.Fatal("ctrl+p no ha lanzado ninguna acción")
		} else {
			mustRun(t, cmd)
		}

		despues, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if despues.Priority == antes {
			t.Errorf("ctrl+p no ha cambiado la prioridad: sigue en %d", antes)
		}
	})

	t.Run("ctrl+p con el cursor fuera de rango no hace nada", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.filteredT = nil
		m.tasks = nil

		siguiente, cmd := pulsar(t, m, "ctrl+p")
		if cmd != nil {
			t.Error("ctrl+p sin tareas ha lanzado una acción")
		}
		if siguiente.cursor != 0 {
			t.Error("ctrl+p sin tareas ha movido el cursor")
		}
	})
}

func TestKanbanKeys(t *testing.T) {
	t.Run("d marca done la tarjeta enfocada", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		id := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].ID
		_, cmd := pulsar(t, m, "d")
		if cmd == nil {
			t.Fatal("d no ha lanzado ninguna acción")
		}
		mustRun(t, cmd)

		tarea, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if tarea.Status != "done" {
			t.Errorf("la tarea quedó en %q, want done", tarea.Status)
		}
	})

	t.Run("x cancela la tarjeta enfocada", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		id := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].ID
		_, cmd := pulsar(t, m, "x")
		if cmd == nil {
			t.Fatal("x no ha lanzado ninguna acción")
		}
		mustRun(t, cmd)

		tarea, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if tarea.Status != "cancelled" {
			t.Errorf("la tarea quedó en %q, want cancelled", tarea.Status)
		}
	})

	t.Run("e abre el editor de descripción inline", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		siguiente, _ := pulsar(t, m, "e")
		if !siguiente.descEditOpen {
			t.Error("e no ha abierto el editor de descripción")
		}
	})

	t.Run("i abre el alta de tarea", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		siguiente, _ := pulsar(t, m, "i")
		if !siguiente.newTaskOpen {
			t.Error("i no ha abierto el alta de tarea")
		}
	})

	t.Run("E abre el editor externo", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		if _, cmd := pulsar(t, m, "E"); cmd == nil {
			t.Error("E no ha lanzado ningún comando")
		}
	})

	t.Run("ctrl+p cambia la prioridad de la tarjeta enfocada", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban

		antes := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].Priority
		_, cmd := pulsar(t, m, "ctrl+p")
		if cmd == nil {
			t.Fatal("ctrl+p no ha lanzado ninguna acción")
		}
		mustRun(t, cmd)
		reloadTasks(t, m)

		id := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow].ID
		tarea, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if tarea.Priority == antes {
			t.Error("ctrl+p no ha cambiado la prioridad de la tarjeta")
		}
	})

	t.Run("h en la primera columna no mueve el índice de columna", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban
		m.kanbanCol = 0
		m.kanbanRow = 1

		siguiente, _ := pulsar(t, m, "h")
		if siguiente.kanbanCol != 0 {
			t.Errorf("h ha movido la columna a %d, want 0", siguiente.kanbanCol)
		}
		// La fila no se toca al no cambiar de columna: el índice es por columna.
		if siguiente.kanbanRow != 1 {
			t.Errorf("h ha movido la fila a %d, want 1 (no cambia de columna)", siguiente.kanbanRow)
		}
	})

	t.Run("l cambia de columna y reinicia la fila", func(t *testing.T) {
		m := newKanbanModel(t, 3)
		m.currentView = viewKanban
		m.kanbanCol = 0
		m.kanbanRow = 2

		siguiente, _ := pulsar(t, m, "l")
		if siguiente.kanbanCol != 1 {
			t.Errorf("l ha dejado la columna en %d, want 1", siguiente.kanbanCol)
		}
		if siguiente.kanbanRow != 0 {
			t.Errorf("l no ha reiniciado la fila: %d, want 0", siguiente.kanbanRow)
		}
	})

	t.Run("sin columnas no hay nada que hacer", func(t *testing.T) {
		bare := newBareModel(t, func(*config.Config) {})
		m := &bare
		m.currentView = viewKanban
		m.filteredT = nil
		m.tasks = nil
		m.projects = []model.Project{{Name: "sin-estados"}}
		m.filterProject = "sin-estados"
		if cols := m.kanbanColumns(); len(cols) != 0 {
			t.Fatalf("el fixture no ha dejado el board vacío: %d columnas", len(cols))
		}

		for _, k := range []string{"h", "l", "j", "k", "d", "x", "s", "S", "enter", "ctrl+p"} {
			siguiente, cmd := pulsar(t, m, k)
			if cmd != nil {
				t.Errorf("%q con el board vacío ha lanzado una acción", k)
			}
			if siguiente.kanbanCol != 0 || siguiente.kanbanRow != 0 {
				t.Errorf("%q con el board vacío ha movido el cursor a [%d,%d]",
					k, siguiente.kanbanCol, siguiente.kanbanRow)
			}
		}
	})
}

func TestDetailKeys(t *testing.T) {
	t.Run("s arranca la tarea y cierra el detalle", func(t *testing.T) {
		// StartStatus devuelve el segundo estado del workflow, así que sólo se
		// nota desde "backlog": en cualquier otro estado mover a "s" es un
		// no-op por diseño, no un fallo.
		m := newDetailModel(t, 0)
		m.detailTask = taskByStatus(t, m, "backlog")
		m.detailOpen = true

		id := m.detailTask.ID
		siguiente, cmd := pulsar(t, m, "s")
		if siguiente.detailOpen {
			t.Error("s no ha cerrado el detalle")
		}
		if siguiente.detailTask != nil {
			t.Error("s no ha vaciado la tarea del detalle")
		}
		if cmd == nil {
			t.Fatal("s no ha lanzado la acción de arranque")
		}
		_, _ = updateMsg(t, siguiente, mustMsg(t, cmd))

		tarea, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		// StartStatus devuelve el SEGUNDO estado del workflow, no el tercero: el
		// nombre "doing" era una suposición mía y no el contrato.
		if tarea.Status != "todo" {
			t.Errorf("la tarea quedó en %q, want el segundo estado del workflow (todo)", tarea.Status)
		}
	})

	t.Run("x cancela la tarea y cierra el detalle", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true

		id := m.detailTask.ID
		siguiente, cmd := pulsar(t, m, "x")
		if siguiente.detailOpen || siguiente.detailTask != nil {
			t.Error("x no ha cerrado el detalle")
		}
		if cmd == nil {
			t.Fatal("x no ha lanzado la acción de cancelación")
		}
		_, _ = updateMsg(t, siguiente, mustMsg(t, cmd))

		tarea, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if tarea.Status != "cancelled" {
			t.Errorf("la tarea quedó en %q, want cancelled", tarea.Status)
		}
	})

	t.Run("d sin comentario seleccionado marca la tarea como done", func(t *testing.T) {
		m := newDetailModel(t, 3)
		m.detailOpen = true
		m.detailCommentSel = -1

		id := m.detailTask.ID
		siguiente, cmd := pulsar(t, m, "d")
		if siguiente.detailOpen || siguiente.detailTask != nil {
			t.Error("d no ha cerrado el detalle")
		}
		if siguiente.detailComments != nil {
			t.Error("d no ha limpiado los comentarios cargados")
		}
		if cmd == nil {
			t.Fatal("d no ha lanzado la acción de done")
		}
		_, _ = updateMsg(t, siguiente, mustMsg(t, cmd))

		tarea, err := m.database.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d): %v", id, err)
		}
		if tarea.Status != "done" {
			t.Errorf("la tarea quedó en %q, want done", tarea.Status)
		}
	})

	t.Run("d con un comentario seleccionado borra el comentario, no la tarea", func(t *testing.T) {
		m := newDetailModel(t, 3)
		m.detailOpen = true

		m, _ = applyComments(t, m)
		m, _ = pulsar(t, m, "j")
		if m.detailCommentSel < 0 || m.detailCommentSel >= len(m.detailComments) {
			t.Fatalf("el fixture no ha dejado un comentario seleccionado (sel=%d, %d comentarios)",
				m.detailCommentSel, len(m.detailComments))
		}
		comment := m.detailComments[m.detailCommentSel]

		siguiente, cmd := pulsar(t, m, "d")
		if !siguiente.detailOpen {
			t.Error("d con un comentario seleccionado ha cerrado el detalle")
		}
		if siguiente.detailTask == nil {
			t.Error("d con un comentario seleccionado ha vaciado el detalle")
		}
		if cmd == nil {
			t.Fatal("d no ha lanzado el borrado del comentario")
		}
		_, _ = updateMsg(t, siguiente, mustMsg(t, cmd))

		restantes, err := m.database.ListComments(m.detailTask.ID)
		if err != nil {
			t.Fatalf("ListComments: %v", err)
		}
		for _, c := range restantes {
			if c.ID == comment.ID {
				t.Errorf("el comentario %d sigue ahí", comment.ID)
			}
		}
		// Y la tarea sigue viva: es un comentario, no la tarea.
		if _, err := m.database.GetTask(m.detailTask.ID); err != nil {
			t.Errorf("borrar un comentario ha borrado la tarea: %v", err)
		}
	})

	t.Run("d sin tarea en el detalle no hace nada", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true
		m.detailTask = nil

		siguiente, cmd := pulsar(t, m, "d")
		if cmd != nil {
			t.Error("d sin tarea ha lanzado una acción")
		}
		if !siguiente.detailOpen {
			t.Error("d sin tarea ha cerrado el detalle, want intacto")
		}
	})

	t.Run("t abre el modal de tags desde el detalle", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true

		siguiente, _ := pulsar(t, m, "t")
		if !siguiente.tagOpen {
			t.Error("t no ha abierto el modal de tags")
		}
		if siguiente.tagSuggestIdx != -1 {
			t.Errorf("tagSuggestIdx = %d, want -1 (sin sugerencias al abrir)", siguiente.tagSuggestIdx)
		}
	})

	t.Run("e y E necesitan una tarea", func(t *testing.T) {
		m := newDetailModel(t, 0)
		m.detailOpen = true
		m.detailTask = nil

		if siguiente, _ := pulsar(t, m, "e"); siguiente.descEditOpen {
			t.Error("e sin tarea ha abierto el editor de descripción")
		}
		if siguiente, cmd := pulsar(t, m, "E"); cmd != nil || siguiente.detailOpen == false {
			t.Error("E sin tarea ha cerrado el detalle")
		}
		if _, cmd := pulsar(t, m, "c"); cmd != nil {
			t.Error("c sin tarea ha lanzado un comando")
		}
		if siguiente, _ := pulsar(t, m, "t"); siguiente.tagOpen {
			t.Error("t sin tarea ha abierto el modal de tags")
		}
		if siguiente, _ := pulsar(t, m, "s"); siguiente.detailOpen == false {
			t.Error("s sin tarea ha cerrado el detalle")
		}
		if siguiente, _ := pulsar(t, m, "x"); siguiente.detailOpen == false {
			t.Error("x sin tarea ha cerrado el detalle")
		}
	})

	t.Run("j con comentarios avanza el seleccionado y sin comentarios no", func(t *testing.T) {
		m := newDetailModel(t, 2)
		m.detailOpen = true

		vacio := *m
		vacio.detailComments = nil
		vacio.detailCommentSel = 0
		if siguiente, cmd := pulsar(t, &vacio, "j"); cmd != nil || siguiente.detailCommentSel != 0 {
			t.Errorf("j sin comentarios ha movido la selección a %d", siguiente.detailCommentSel)
		}

		m.detailCommentSel = -1
		siguiente, _ := pulsar(t, m, "j")
		if siguiente.detailCommentSel != 0 {
			t.Errorf("j desde -1 ha dejado la selección en %d, want 0", siguiente.detailCommentSel)
		}
	})

	t.Run("k retrocede y en el primero deselecciona", func(t *testing.T) {
		m := newDetailModel(t, 3)
		m.detailOpen = true

		m.detailCommentSel = 2
		atras, _ := pulsar(t, m, "k")
		if atras.detailCommentSel != 1 {
			t.Errorf("k desde 2 ha dejado la selección en %d, want 1", atras.detailCommentSel)
		}

		primero, _ := pulsar(t, atras, "k")
		if primero.detailCommentSel != 0 {
			t.Errorf("k desde 1 ha dejado la selección en %d, want 0", primero.detailCommentSel)
		}

		nada, _ := pulsar(t, primero, "k")
		primero = nada
		if primero.detailCommentSel != -1 {
			t.Errorf("k en el primero ha dejado la selección en %d, want -1 (nada seleccionado)",
				primero.detailCommentSel)
		}
	})
}

// updateMsg es applyMsg pero devolviendo también el comando, porque varios de
// estos tests necesitan comprobar que NO se lanzó ninguno.
func updateMsg(t *testing.T, m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return asModel(next), cmd
}

// applyComments carga los comentarios del detalle como lo hace el Update al
// recibir commentsLoadedMsg, para partir de un estado realista.
func applyComments(t *testing.T, m *Model) (*Model, tea.Cmd) {
	t.Helper()
	cmd := m.loadCommentsCmd(m.detailTask.ID)
	if cmd == nil {
		t.Fatal("loadCommentsCmd ha devuelto nil")
	}
	return updateMsg(t, m, mustMsg(t, cmd))
}

func TestGlobalKeys(t *testing.T) {
	t.Run("esc cierra la ayuda y el filtro y el filtro activo", func(t *testing.T) {
		for _, tc := range []struct {
			nombre    string
			preparar  func(*Model)
			comprobar func(*testing.T, *Model)
		}{
			{"ayuda", func(m *Model) { m.helpOpen = true },
				func(t *testing.T, m *Model) {
					if m.helpOpen {
						t.Error("esc no ha cerrado la ayuda")
					}
				}},
			{"filtro abierto", func(m *Model) { m.filterOpen = true; m.filterActive = true },
				func(t *testing.T, m *Model) {
					if m.filterOpen {
						t.Error("esc no ha cerrado el filtro")
					}
				}},
			{"filtro activo", func(m *Model) { m.filterActive = true; m.filterText = "algo" },
				func(t *testing.T, m *Model) {
					if m.filterActive {
						t.Error("esc no ha desactivado el filtro")
					}
					if m.filterText != "" {
						t.Errorf("esc no ha limpiado el texto del filtro: %q", m.filterText)
					}
				}},
		} {
			t.Run(tc.nombre, func(t *testing.T) {
				m := newTestModel(t)
				tc.preparar(m)

				siguiente, _ := pulsar(t, m, "esc")
				tc.comprobar(t, siguiente)
			})
		}
	})

	t.Run("? alterna la ayuda", func(t *testing.T) {
		m := newTestModel(t)

		abierta, _ := pulsar(t, m, "?")
		if !abierta.helpOpen {
			t.Fatal("? no ha abierto la ayuda")
		}

		cerrada, _ := pulsar(t, abierta, "?")
		if cerrada.helpOpen {
			t.Error("? no ha cerrado la ayuda")
		}
	})

}

// El pegado va a tres sitios según lo que esté abierto, y sólo a uno. El orden
// importa: con el editor de descripción y el alta abiertos a la vez, gana el
// editor porque se comprobó primero.
func TestPasteRouting(t *testing.T) {
	t.Run("nada abierto", func(t *testing.T) {
		m := newTestModel(t)
		siguiente, cmd := updateMsg(t, m, tea.PasteMsg{Content: "texto"})
		if cmd != nil {
			t.Error("un pegado sin nada abierto ha lanzado un comando")
		}
		if siguiente.tagInput != "" {
			t.Errorf("el pegado ha escrito en el input de tags: %q", siguiente.tagInput)
		}
	})

	t.Run("el modal de tags", func(t *testing.T) {
		m := newTestModel(t)
		m.tagOpen = true
		m.detailTask = &m.tasks[0]

		siguiente, _ := updateMsg(t, m, tea.PasteMsg{Content: "pegado"})
		if siguiente.tagInput != "pegado" {
			t.Errorf("tagInput = %q, want %q", siguiente.tagInput, "pegado")
		}
	})

	t.Run("el alta de tarea", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldTitle

		siguiente, _ := updateMsg(t, m, tea.PasteMsg{Content: "pegado"})
		if !strings.Contains(siguiente.newTaskTitle, "pegado") {
			t.Errorf("el pegado no ha llegado al alta: %q", siguiente.newTaskTitle)
		}
	})
}

// Los tres mensajes que sólo existen para propagar errores: el editor externo
// que falla, el alta que falla y un comentario vacío. Ninguno debe dejar el
// modelo en un estado a medias.
func TestSilentMessagesLeaveTheModelIntact(t *testing.T) {
	t.Run("el editor externo falla", func(t *testing.T) {
		m := newTestModel(t)
		antes := ansi.Strip(m.View().Content)

		siguiente, cmd := updateMsg(t, m, editorFinishedMsg{err: errAccionFallida, taskID: 1})
		if cmd != nil {
			t.Error("un editor fallido ha lanzado un comando")
		}
		if despues := ansi.Strip(siguiente.View().Content); despues != antes {
			t.Error("un editor fallido ha cambiado el render")
		}
	})

	t.Run("el editor externo no escribe nada", func(t *testing.T) {
		m := newTestModel(t)

		siguiente, cmd := updateMsg(t, m, editorFinishedMsg{taskID: m.tasks[0].ID})
		if cmd != nil {
			t.Error("un editor que no escribe nada ha lanzado un comando")
		}
		if len(siguiente.tasks) != len(m.tasks) {
			t.Error("un editor vacío ha modificado las tareas")
		}
	})

	t.Run("el alta falla con toast", func(t *testing.T) {
		m := newTestModel(t)

		siguiente, _ := updateMsg(t, m, taskCreateFailedMsg{err: errAccionFallida})
		if !strings.Contains(ansi.Strip(siguiente.View().Content), errAccionFallida.Error()) {
			t.Errorf("el error del alta no sale por la interfaz:\n%s",
				ansi.Strip(siguiente.View().Content))
		}
	})

	t.Run("el alta falla sin error", func(t *testing.T) {
		m := newTestModel(t)
		antes := ansi.Strip(m.View().Content)

		siguiente, _ := updateMsg(t, m, taskCreateFailedMsg{})
		if despues := ansi.Strip(siguiente.View().Content); despues != antes {
			t.Error("un alta fallida sin error ha cambiado el render")
		}
	})

	t.Run("un comentario vacío no se guarda", func(t *testing.T) {
		m := newTestModel(t)

		siguiente, cmd := updateMsg(t, m, commentFinishedMsg{taskID: m.tasks[0].ID, body: ""})
		if cmd != nil {
			t.Error("un comentario vacío ha lanzado un guardado")
		}
		if len(siguiente.detailComments) != 0 {
			t.Error("un comentario vacío ha cargado comentarios")
		}
	})

	t.Run("un comentario con error no se guarda", func(t *testing.T) {
		m := newTestModel(t)

		if _, cmd := updateMsg(t, m, commentFinishedMsg{err: errAccionFallida, taskID: m.tasks[0].ID}); cmd != nil {
			t.Error("un comentario fallido ha lanzado un guardado")
		}
	})
}

// El toggle de una tag que ya no está en la base de datos falla al leer y
// devuelve nil, no un mensaje: no hay nada que actualizar.
func TestToggleTagOnAMissingTask(t *testing.T) {
	m := newTestModel(t)

	if msg := m.toggleTagCmd(9999, "nada")(); msg != nil {
		t.Errorf("toggleTag sobre una tarea inexistente ha devuelto %T, want nil", msg)
	}
}

// commonWorkflow cae al workflow por defecto cuando los proyectos no comparten
// ningún estado, que es justo lo que pasa si cada proyecto define el suyo.
func TestCommonWorkflowFallsBackWhenNothingIsShared(t *testing.T) {
	m := newTestModel(t)
	m.projects = []model.Project{
		{Name: "a", Workflow: []string{"uno"}},
		{Name: "b", Workflow: []string{"otro"}},
	}

	got := m.commonWorkflow()
	if len(got) != len(model.DefaultWorkflow) {
		t.Errorf("sin estados comunes el workflow es %v, want el default %v",
			got, model.DefaultWorkflow)
	}
}

// taskByStatus devuelve la primera tarea del modelo en ese estado, o falla el
// test. Los estados importan: mover una tarea depende del estado en el que está,
// no sólo de la tecla.
func taskByStatus(t *testing.T, m *Model, status string) *model.Task {
	t.Helper()
	for i := range m.tasks {
		if m.tasks[i].Status == status {
			return &m.tasks[i]
		}
	}
	t.Fatalf("el fixture no tiene ninguna tarea en %q", status)
	return nil
}

// El número de secuencia del toast tiene que cambiar en CADA toast, y cada tick
// de expiración tiene que llevar el número del suyo.
//
// No es un detalle interno: el tick compara su número con el actual antes de
// borrar, y por eso es un tick VIEJO el que no debe tocar un toast nuevo. Si dos
// toasts compartieran número, el tick del primero borraría el segundo; y si el
// número no avanzara, todos los ticks viejos borrarían el toast vivo.
func TestElNumeroDeSecuenciaDelToastAvanza(t *testing.T) {
	m := newTestModel(t)

	// Tres toasts seguidos: tres números distintos.
	var vistos []int
	for i := 0; i < 3; i++ {
		m.setToast("toast "+itoa(i), "info")
		vistos = append(vistos, m.toastSeq)
	}
	for i := 1; i < len(vistos); i++ {
		if vistos[i] == vistos[i-1] {
			t.Fatalf("los toasts %d y %d comparten secuencia %d", i-1, i, vistos[i])
		}
	}

	// El tick del PRIMERO llega tarde, con un toast más nuevo en pantalla: no lo
	// borra. Éste es el caso para el que existe el contador. Update tiene receptor
	// por valor, así que el modelo devuelto es el que hay que mirar.
	modelo, _ := m.Update(toastExpiredMsg{seq: vistos[0]})
	conViejo := modelo.(Model)
	if conViejo.toast == "" {
		t.Error("el tick del primer toast ha borrado el tercer toast: un tick viejo no puede limpiar")
	}

	// El tick del ÚLTIMO sí es el suyo: lo borra.
	modelo, _ = conViejo.Update(toastExpiredMsg{seq: vistos[2]})
	if modelo.(Model).toast != "" {
		t.Errorf("el tick de su propio toast no lo ha borrado: %q", modelo.(Model).toast)
	}
}

// jumps en sí: el salto es de uno en uno, y eso es lo que un test puede mirar.
func TestJumps(t *testing.T) {
	for _, c := range []struct{ in, want int }{{0, 1}, {1, 2}, {41, 42}} {
		if got := jumps(c.in); got != c.want {
			t.Errorf("jumps(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
