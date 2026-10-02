package tui

import (
	"errors"
	"strings"
	"testing"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// errAccionFallida es el error que devuelven las acciones simuladas que tienen
// que fallar: el comando no debe recargar la lista.
var errAccionFallida = errors.New("accion fallida")

// Los handlers empiezan siempre con su clamp, así que un cursor fuera de rango
// no se puede provocar desde el teclado. Lo que sí es real es la lista vacía:
// ahí el clamp deja el cursor en 0 y el guarda de la tecla tiene que impedir la
// acción, porque indexar tasks[0] sobre una lista vacía revienta.
//
// Estos tests fijan ese borde: con tareas la tecla actúa, sin tareas no hace
// nada.

// Con la lista filtrada vacía, ninguna tecla de acción toca la base de datos.
func TestListActionKeysNoopWithEmptyList(t *testing.T) {
	for _, key := range []string{"s", "d", "x"} {
		t.Run(key, func(t *testing.T) {
			m := newTestModel(t)
			m.tasks = nil
			m.filteredT = nil
			m.filterStatus = "nonexistent-status"

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("tecla %q sin tareas: %v quiere ejecutar una acción", key, cmd)
			}
			if next.cursor != 0 {
				t.Errorf("el cursor quedó en %d sin tareas", next.cursor)
			}
		})
	}
}

// El otro lado del mismo guarda: con una tarea bajo el cursor, la tecla actúa.
// Sin esta mitad, el test anterior pasaría también con el guarda eliminado.
func TestListActionKeysActWithValidCursor(t *testing.T) {
	for _, key := range []string{"s", "d", "x"} {
		t.Run(key, func(t *testing.T) {
			m := newTestModel(t)
			if len(m.filteredTasks()) == 0 {
				t.Skip("el fixture no dejó tareas visibles")
			}
			m.cursor = 0

			if _, cmd := press(m, key); cmd == nil {
				t.Errorf("tecla %q con una tarea seleccionada no produjo comando", key)
			}
		})
	}
}

// El alto disponible para el contenido descuenta el preview y el toast. El
// toast ocupa una línea, así que sumarlo en vez de restarlo cambia cuántas filas
// de tareas caben en la caja.
//
// A 18 de alto la diferencia cae justo en el borde: con toast se pintan 2 filas
// de tarea, sin toast 3. Los números son literales, no derivados: si alguien
// cambia el reparto de alto del layout hay que actualizar el test a mano, que es
// justo lo que se quiere.
func TestViewBudgetCountsToastLine(t *testing.T) {
	for _, tt := range []struct {
		toast string
		want  int
	}{
		{"guardado", 2},
		{"", 3},
	} {
		t.Run("toast="+tt.toast, func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = viewList
			m.height = 18
			m.toast = tt.toast
			m.toastKind = "info"

			if got := countRenderedTasks(t, m); got != tt.want {
				t.Errorf("con toast %q se pintaron %d filas de tarea, want %d", tt.toast, got, tt.want)
			}
		})
	}
}

// Las filas se cuentan por los títulos de las tareas del fixture: cada uno
// aparece exactamente una vez por fila pintada.
var fixtureTitles = []string{"Fix N+1 query", "Add caching", "Update README", "Fix checkout"}

func countRenderedTasks(t *testing.T, m *Model) int {
	t.Helper()
	out := m.View().Content
	n := 0
	for _, title := range fixtureTitles {
		n += strings.Count(out, title)
	}
	return n
}

// La selección del detalle no envuelve por arriba: -1 significa "nada
// seleccionado", y desde ahí "j" va al primer comentario, no al segundo.

// Con el cursor en el último comentario, "j" se queda ahí.
func TestDetailCommentDownStopsAtLast(t *testing.T) {
	m := newDetailModel(t, 3)
	m.detailCommentSel = len(m.detailComments) - 1

	next, _ := press(m, "j")
	if next.detailCommentSel != 2 {
		t.Errorf("selección %d, want 2 (no avanza más allá del último)", next.detailCommentSel)
	}
}

// Y desde -1, "j" va al primero, no al segundo.
func TestDetailCommentDownFromNoneSelectsFirst(t *testing.T) {
	m := newDetailModel(t, 3)
	m.detailCommentSel = -1

	next, _ := press(m, "j")
	if next.detailCommentSel != 0 {
		t.Errorf("selección %d, want 0", next.detailCommentSel)
	}
}

// Hacia arriba desde el primero se vuelve a "nada seleccionado", y de ahí no se
// sale hacia -2.
func TestDetailCommentUpFromFirstClearsSelection(t *testing.T) {
	m := newDetailModel(t, 2)
	m.detailCommentSel = 0

	next, _ := press(m, "k")
	if next.detailCommentSel != -1 {
		t.Errorf("selección %d, want -1", next.detailCommentSel)
	}

	next, _ = press(m, "k")
	if next.detailCommentSel != -1 {
		t.Errorf("selección %d, want -1 (no baja de -1)", next.detailCommentSel)
	}
}

// Sin comentarios, ni "j" ni "k" mueven nada.
func TestDetailCommentKeysNoopWithoutComments(t *testing.T) {
	for _, key := range []string{"j", "k"} {
		t.Run(key, func(t *testing.T) {
			m := newDetailModel(t, 0)

			next, _ := press(m, key)
			if next.detailCommentSel != -1 {
				t.Errorf("selección %d, want -1 sin comentarios", next.detailCommentSel)
			}
		})
	}
}

// newDetailModel abre el detalle de la primera tarea con n comentarios.
func newDetailModel(t *testing.T, comments int) *Model {
	t.Helper()
	m := newTestModel(t)
	if len(m.tasks) == 0 {
		t.Fatal("el fixture no dejó tareas")
	}
	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	for i := range comments {
		mustAddComment(t, m.database, task.ID, "comentario")
		_ = i
	}
	m.detailComments, _ = m.database.ListComments(task.ID)
	if len(m.detailComments) != comments {
		t.Fatalf("pedidos %d comentarios, hay %d", comments, len(m.detailComments))
	}
	return m
}

// "d" en el detalle borra el comentario seleccionado; sin selección, o con una
// selección que no vale para la lista, marca la tarea. El borde del guarda es
// detailCommentSel == len(comments), que la interfaz no produce pero el código
// comprueba: si el guarda desapareciera, indexaría fuera de rango.
func TestDetailDeleteFallsBackToTaskWhenSelectionOutOfRange(t *testing.T) {
	for _, sel := range []int{-1, -5, 2, 99} {
		m := newDetailModel(t, 2)
		m.detailCommentSel = sel

		next, _ := press(m, "d")
		// Marcar la tarea cierra el detalle; borrar un comentario no.
		if next.detailOpen {
			t.Errorf("sel %d: no se borró ni se marcó; el detalle sigue abierto", sel)
		}
	}
}

// Con una selección válida, "d" borra ese comentario y deja el detalle abierto.
// Sin esta mitad, el test anterior pasaría también con el guarda eliminado.
func TestDetailDeleteRemovesSelectedComment(t *testing.T) {
	m := newDetailModel(t, 2)
	m.detailCommentSel = 1

	next, cmd := press(m, "d")
	if cmd == nil {
		t.Fatal("no produjo comando de borrado")
	}
	if !next.detailOpen {
		t.Error("borrar un comentario cerró el detalle")
	}
}

// Las teclas del detalle que necesitan una tarea abierta no abren nada sin ella.
// Cada una tiene su propio guarda, y quitarlos sería cambiar el comportamiento
// con el detalle cerrado.
func TestDetailKeysRequireOpenTask(t *testing.T) {
	for _, key := range []string{"t", "e", "c", "E"} {
		t.Run(key, func(t *testing.T) {
			m := newDetailModel(t, 1)
			m.detailOpen = true
			m.detailTask = nil

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("tecla %q sin tarea abierta produjo comando %T", key, cmd)
			}
			if next.tagOpen {
				t.Error("abrió el modal de tags sin tarea")
			}
			if next.descEditOpen {
				t.Error("abrió el editor de descripción sin tarea")
			}
		})
	}
}

// New arranca con un pageSize utilizable aunque la configuración no traiga uno,
// y con el resto de valores por defecto que las vistas asumen.
func TestNewDefaults(t *testing.T) {
	m := newBareModel(t, func(c *config.Config) { c.ListPageSize = 0 })

	if m.pageSize != config.DefaultPageSize {
		t.Errorf("pageSize %d, want %d con la config a 0", m.pageSize, config.DefaultPageSize)
	}
	if m.currentView != viewList {
		t.Errorf("vista inicial %v, want la lista", m.currentView)
	}
	if m.cursor != 0 {
		t.Errorf("cursor inicial %d, want 0", m.cursor)
	}
	// -1 es "sin filtro": se distingue del 0, que sería sólo prioridad máxima.
	if m.filterPriority != -1 {
		t.Errorf("filterPriority inicial %d, want -1", m.filterPriority)
	}
	if m.filterStatus != statusFilterAllActive {
		t.Errorf("filterStatus inicial %q, want %q", m.filterStatus, statusFilterAllActive)
	}
	// Igual que -1: "nada seleccionado" en los dos selectores.
	if m.detailCommentSel != -1 || m.tagSuggestIdx != -1 {
		t.Errorf("selectores iniciales (%d, %d), want (-1, -1)", m.detailCommentSel, m.tagSuggestIdx)
	}
	if m.width != 80 || m.height != 24 {
		t.Errorf("tamaño inicial %dx%d, want 80x24", m.width, m.height)
	}
}

// Y una configuración con pageSize se respeta tal cual.
func TestNewKeepsConfiguredPageSize(t *testing.T) {
	for _, n := range []int{1, 7, 50} {
		m := newBareModel(t, func(c *config.Config) { c.ListPageSize = n })
		if m.pageSize != n {
			t.Errorf("pageSize %d, want %d", m.pageSize, n)
		}
	}
}

// newBareModel construye un modelo con DB vacía y la config ajustada.
func newBareModel(t *testing.T, tweak func(*config.Config)) Model {
	t.Helper()
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	cfg := config.Defaults()
	tweak(&cfg)
	return New(database, cfg)
}

// Una acción que falla no recarga la lista: el comando devuelve nil en vez de
// un tasksLoadedMsg con los datos de antes, que era lo que se veía.
func TestTaskActionCmdErrorYieldsNoMsg(t *testing.T) {
	m := newTestModel(t)
	boom := func(int64) (*model.Task, error) { return nil, errAccionFallida }

	cmd := m.taskActionCmd(1, boom)
	if cmd == nil {
		t.Fatal("no produjo comando")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("una acción fallida devolvió %T, want nil (no recarga)", msg)
	}
}

// Si la acción va bien, el comando recarga y trae la lista.
func TestTaskActionCmdSuccessReloads(t *testing.T) {
	m := newTestModel(t)
	ok := func(id int64) (*model.Task, error) { return m.database.DoneTask(id) }

	msgs := mustRun(t, m.taskActionCmd(m.tasks[0].ID, ok))
	if len(msgs) != 1 {
		t.Fatalf("mensajes %d, want 1", len(msgs))
	}
	loaded, isLoaded := msgs[0].(tasksLoadedMsg)
	if !isLoaded {
		t.Fatalf("mensaje %T, want tasksLoadedMsg", msgs[0])
	}
	if len(loaded.tasks) == 0 {
		t.Error("la recarga no trajo tareas")
	}
}

// projectsLoadedMsg con un proyecto pendiente lo selecciona y limpia el
// pendiente; sin pendiente, deja el cursor donde estaba.
func TestProjectsLoadedSelectsPendingName(t *testing.T) {
	m := newTestModel(t)
	m.pendingSelectName = "web"
	idxAntes := m.dashProjectIdx

	next, _ := m.Update(projectsLoadedMsg{projects: m.projects})
	got := next.(Model)
	if got.dashProjectIdx == idxAntes {
		t.Errorf("no seleccionó el proyecto pendiente (dashProjectIdx %d)", got.dashProjectIdx)
	}
	if got.pendingSelectName != "" {
		t.Errorf("el pendiente quedó en %q, want vacío tras consumirse", got.pendingSelectName)
	}
}

func TestProjectsLoadedWithoutPendingKeepsSelection(t *testing.T) {
	m := newTestModel(t)
	m.pendingSelectName = ""
	m.dashProjectIdx = 0

	next, _ := m.Update(projectsLoadedMsg{projects: m.projects})
	got := next.(Model)
	if got.dashProjectIdx != 0 {
		t.Errorf("sin pendiente la selección cambió a %d", got.dashProjectIdx)
	}
	if got.pendingSelectName != "" {
		t.Errorf("apareció un pendiente %q sin pedirlo", got.pendingSelectName)
	}
}

// Con el detalle ya abierto y comentarios cargados, reabrirlo con Enter vuelve a
// dejar la selección en -1.
func TestReopeningDetailClearsCommentSelection(t *testing.T) {
	m := newTestModel(t)
	m.cursor = 0
	m.detailOpen = false
	m.detailTask = nil
	m.detailCommentSel = 3

	next, _ := press(m, "enter")
	got := next

	if !got.detailOpen {
		t.Fatal("Enter no abrió el detalle")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1 (nada seleccionado)", got.detailCommentSel)
	}
	if got.detailComments != nil {
		t.Error("los comentarios arrancan sin cargar, no como lista vacía")
	}
}

// Abrir el modal de tags deja el índice de sugerencia en -1, por el mismo motivo
// que el detalle con los comentarios.
func TestOpeningTagModalClearsSuggestionIndex(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	m.tagInput = "algo"
	m.tagSuggestIdx = 4

	next, _ := press(m, "t")
	got := next

	if !got.tagOpen {
		t.Fatal("la tecla t no abrió el modal de tags")
	}
	if got.tagSuggestIdx != -1 {
		t.Errorf("tagSuggestIdx = %d, want -1", got.tagSuggestIdx)
	}
	if got.tagInput != "" {
		t.Errorf("el input quedó en %q, want vacío", got.tagInput)
	}
}

// En Kanban, abrir el detalle sobre la tarjeta seleccionada también limpia la
// selección de comentarios.
func TestKanbanOpeningDetailClearsCommentSelection(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewKanban
	cols := m.kanbanColumns()
	m.kanbanCol = clampTo(m.kanbanCol, len(cols))
	m.kanbanRow = clampTo(m.kanbanRow, len(cols[m.kanbanCol].tasks))
	m.detailCommentSel = 2

	next, _ := press(m, "enter")
	got := next

	if !got.detailOpen {
		t.Fatal("Enter en Kanban no abrió el detalle")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1", got.detailCommentSel)
	}
}

// Con una sola columna no hay a dónde moverse con las flechas horizontales, y la
// fila no se resetea: el reset es consecuencia de cambiar de columna, no de
// apretar la tecla. Con más de una columna, mover sí resetea.
func TestKanbanColumnArrowsWithASingleColumn(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 2, []string{"todo", "done"})
	if len(m.kanbanColumns()) != 2 {
		t.Skipf("el fixture tiene %d columnas", len(m.kanbanColumns()))
	}
	m.kanbanCol = 0
	m.kanbanRow = 1

	next, _ := press(m, "h")
	if next.kanbanRow != 1 {
		t.Errorf("con una sola columna a la izquierda la fila pasó a %d, want 1", next.kanbanRow)
	}

	// Y con dos columnas, mover a la izquierda desde la primera no mueve y la fila
	// se queda: no hubo cambio de columna.
	next2, _ := press(m, "h")
	if next2.kanbanCol != 0 {
		t.Errorf("la columna se movió a %d desde el borde", next2.kanbanCol)
	}
	if next2.kanbanRow != 1 {
		t.Errorf("la fila se reseteó a %d sin cambiar de columna", next2.kanbanRow)
	}
}
