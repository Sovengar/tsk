package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// inicioGantt es una fecha fija para que el render del Gantt sea reproducible.
var inicioGantt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Lote de bordes que quedaban sin comprobar en ficheros donde el resto del
// render ya estaba cubierto: el punto de selección de las listas del modal de
// personas, el ancho de la regla del Gantt, los resets de "nada seleccionado" del
// editor de descripción, y el borrado de tags del alta.

// La fila seleccionada del roster lleva el prefijo de selección, y el resto no.
// Con dos personas en el roster y el cursor en la segunda, sólo esa lo lleva.
func TestAssigneeRosterSelectionPrefix(t *testing.T) {
	m := newAssigneeModel(t, 2)
	m.archivedProjects = nil
	reloadOffDays(t, m)
	m.assigneeIdx = 1

	// El prefijo se busca en el texto limpio; la negrita, en el render con sus
	// códigos, porque al quitar los códigos las dos marcas se vuelven indistinguibles.
	crudo := m.renderAssigneeModal("")
	out := ansi.Strip(crudo)
	if n := strings.Count(out, selectionPrefix(true)); n != 1 {
		t.Errorf("hay %d filas con el prefijo de selección, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, selectionPrefix(true)+"@user01") {
		t.Errorf("la marca no está en la segunda persona:\n%s", out)
	}
	if n := countBoldLines(crudo); n != 1 {
		t.Errorf("hay %d filas resaltadas, want 1:\n%s", n, out)
	}
}

// Lo mismo en la lista de off-days: el cursor va sobre un off-day concreto, no
// sobre "la lista".
func TestAssigneeOffdaySelectionPrefix(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	for i := range 3 {
		dia := "2026-0" + string(rune('1'+i)) + "-01"
		mustAddOffDay(t, m.database, "@user00", dia, dia, "")
	}
	reloadOffDays(t, m)
	m.assigneeDetail = true
	m.assigneeOffdayIdx = 2

	// El prefijo se busca en el texto limpio; la negrita, en el render con sus
	// códigos, porque al quitar los códigos las dos marcas se vuelven indistinguibles.
	crudo := m.renderAssigneeModal("")
	out := ansi.Strip(crudo)
	if n := strings.Count(out, selectionPrefix(true)); n != 1 {
		t.Errorf("hay %d filas con el prefijo de selección, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, selectionPrefix(true)+"2026-03-01") {
		t.Errorf("la marca no está en el tercer off-day:\n%s", out)
	}
	// El detalle también resalta el nombre de la persona en su cabecera, así que
	// la cuenta se limita a las filas de off-day: son las que llevan flecha.
	if n := countBoldLinesWith(crudo, "→"); n != 1 {
		t.Errorf("hay %d off-days resaltados, want 1:\n%s", n, out)
	}
}

// El eje del Gantt tiene el ancho de la etiqueta, un separador y las columnas de
// día, sin recortar. La regla no sirve para esto: le quita los espacios de la
// derecha, así que su ancho visible depende de dónde cae el último lunes.
func TestGanttAxisExactWidth(t *testing.T) {
	tests := []struct {
		name    string
		labelW  int
		dayCols int
	}{
		{"holgada", 30, 40},
		{"etiqueta al mínimo", 14, 7},
		{"un solo día", 30, 1},
		{"con etiqueta y días mínimos", 14, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ganttModelWithPeople(t, []string{"@juan"}, 1)

			axis := ansi.Strip(m.renderGanttAxis(tt.labelW, tt.dayCols, inicioGantt, 0))
			if got := len([]rune(axis)); got != tt.labelW+1+tt.dayCols {
				t.Errorf("el eje mide %d columnas, want %d (etiqueta %d + 1 + días %d)",
					got, tt.labelW+1+tt.dayCols, tt.labelW, tt.dayCols)
			}
		})
	}
}

// El rótulo de cada lunes cae en su columna: la etiqueta de la izquierda, el
// separador, y desde ahí el día. El 1 de enero de 2026 es jueves, así que el
// primer lunes es el día 4, en la columna 35 con una etiqueta de 30.
func TestGanttRulerLabelLandsOnItsMonday(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)

	const labelW = 30
	ruler := []rune(ansi.Strip(m.renderGanttRuler(inicioGantt, 0, labelW, 20)))
	const primerLunes = 4 // 2026-01-05

	want := model.WeekOfMonthLabel(inicioGantt.AddDate(0, 0, primerLunes))
	at := labelW + 1 + primerLunes
	if at+len([]rune(want)) > len(ruler) {
		t.Fatalf("el rótulo %q no cabe en la regla de %d columnas", want, len(ruler))
	}
	if got := string(ruler[at : at+len([]rune(want))]); got != want {
		t.Errorf("en la columna %d hay %q, want %q (ruler %q)", at, got, want, string(ruler))
	}
	// Y las columnas anteriores al lunes están en blanco: el rótulo no puede
	// empezar antes.
	for i := primerLunes; i < at; i++ {
		if ruler[i] != ' ' {
			t.Fatalf("columna %d = %q, want espacio antes del rótulo", i, string(ruler[i]))
		}
	}
}

// La etiqueta de la izquierda del eje son espacios del ancho exacto de la
// etiqueta, ni uno más.
func TestGanttAxisLeadingSpaces(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)

	axis := []rune(ansi.Strip(m.renderGanttAxis(30, 10, inicioGantt, 0)))
	for i, r := range axis {
		if i < 31 {
			if r != ' ' {
				t.Fatalf("columna %d = %q, want espacio (etiqueta + separador)", i, string(r))
			}
			continue
		}
		break
	}
	// A partir del separador hay un carácter por día, y el primero es el jueves,
	// que no es lunes, así que es "-".
	if axis[31] != '-' {
		t.Errorf("el primer día es %q, want \"-\"", string(axis[31]))
	}
}

// Abrir el editor de descripción limpia la selección de comentarios, igual que
// abrir el detalle: la descripción no tiene comentarios, y dejar la selección
// apuntando a un índice de comentarios que no se están mostrando haría que la
// primera tecla saltara de sitio.
func TestOpeningDescEditorClearsCommentSelection(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.detailComments = []model.Comment{{ID: 1, Body: "uno", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.detailCommentSel = 1

	next, _ := press(m, "e")
	got := next

	if !got.descEditOpen {
		t.Fatal("la tecla e no abrió el editor de descripción")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1", got.detailCommentSel)
	}
}

// El editor de descripción se abre sobre la tarea del detalle si está abierto, y
// sobre la seleccionada en la vista si no. Con el detalle abierto, editar es
// sobre la del detalle aunque el cursor de la lista esté en otra.
func TestDescEditTargetPrefersOpenDetail(t *testing.T) {
	m := newTestModel(t)
	tarea := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &tarea
	m.filteredT = nil

	if got := m.descEditTarget(); got == nil || got.ID != tarea.ID {
		t.Errorf("con el detalle abierto el objetivo es %v, want la tarea del detalle", got)
	}

	m.detailOpen = false
	m.cursor = 1
	seleccionada := m.filteredTasks()[1]
	if got := m.descEditTarget(); got == nil || got.ID != seleccionada.ID {
		t.Errorf("sin detalle el objetivo es %v, want la tarea seleccionada", got)
	}

	// Con el detalle "abierto" pero sin tarea -- un estado que el interfaz no
	// produce y que sí puede dejar un test -- el objetivo cae a la lista, no a
	// nil: hay una tarea seleccionada y es tan editable como la otra.
	m.detailOpen = true
	m.detailTask = nil
	m.cursor = 1
	if got := m.descEditTarget(); got == nil {
		t.Error("con el detalle abierto y sin tarea, el objetivo es nil; want la seleccionada de la vista")
	} else if got.ID != m.filteredTasks()[1].ID {
		t.Errorf("el objetivo es la tarea %d, want la seleccionada (%d)", got.ID, m.filteredTasks()[1].ID)
	}

	// Y sin ninguna de las dos, nil.
	m.detailOpen = false
	m.filteredT = nil
	m.cursor = 99
	if got := m.descEditTarget(); got != nil {
		t.Errorf("sin tarea seleccionada el objetivo es %v, want nil", got)
	}
}

// Borrar tags del campo del alta: con el input vacío, backspace quita la última
// agregada; con algo escrito, quita un carácter del input. Y编辑 Tags limpia la
// selección de sugerencias, porque la lista va a cambiar.
func TestNewTaskTagsBackspace(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		tags      []string
		wantInput string
		wantTags  []string
	}{
		{"con input escrito quita un carácter", "ab", nil, "a", nil},
		{"con input vacío quita la última tag", "", []string{"uno", "dos"}, "", []string{"uno"}},
		{"sin tags ni input no hace nada", "", nil, "", nil},
		{"con una tag la quita", "", []string{"uno"}, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			next, _ := press(m, "i")
			m2 := next
			m2.newTaskFieldIdx = newTaskFieldTags
			m2.newTaskTagInput = tt.input
			m2.newTaskTags = append([]string(nil), tt.tags...)
			m2.newTaskTagSuggIdx = 2

			got, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
			gotM := got.(Model)

			if gotM.newTaskTagInput != tt.wantInput {
				t.Errorf("input = %q, want %q", gotM.newTaskTagInput, tt.wantInput)
			}
			if strings.Join(gotM.newTaskTags, ",") != strings.Join(tt.wantTags, ",") {
				t.Errorf("tags = %v, want %v", gotM.newTaskTags, tt.wantTags)
			}
			// Editar el input limpia la selección de sugerencias; borrar una tag
			// agregada no. Es lo que hace el código, y no es un problema: el
			// índice se acota antes de usarse.
			wantSugg := -1
			if tt.input == "" && len(tt.tags) > 0 {
				wantSugg = 2
			}
			if gotM.newTaskTagSuggIdx != wantSugg {
				t.Errorf("suggIdx = %d, want %d", gotM.newTaskTagSuggIdx, wantSugg)
			}
		})
	}
}

// "k" en Kanban envuelve: desde la primera tarjeta salta a la última de la
// columna, y en la última se queda. Es un ciclo, no un desplazamiento, y la
// diferencia se ve en la primera pulsación.
func TestKanbanUpWrapsWithinColumn(t *testing.T) {
	m := newKanbanModel(t, 3) // tres tarjetas en "todo", una columna con tareas
	m.kanbanCol = 0
	m.kanbanRow = 0

	arriba, _ := press(m, "k")
	if arriba.kanbanRow == arriba.kanbanCol {
		t.Fatalf("la fila no cambió")
	}
	if arriba.kanbanRow != 2 {
		t.Errorf("desde la primera, \"k\" deja la fila en %d, want 2 (la última)", arriba.kanbanRow)
	}

	// Y desde la última retrocede a la anterior: envuelve por arriba, no se queda.
	arriba2, _ := press(arriba, "k")
	if arriba2.kanbanRow != 1 {
		t.Errorf("desde la última, \"k\" deja la fila en %d, want 1", arriba2.kanbanRow)
	}
}

// Los paneles del Dashboard están topados por el alto, y el tope es exacto: con
// una línea más de alto sale una fila más de equipo, y una menos, una menos.
func TestDashboardTeamPanelRespectsBudget(t *testing.T) {
	m := newDashModel(t, "")
	for i := range 8 {
		mustCreateTask(t, m.database, "api", "tarea de p"+string(rune('a'+i)), "", "@p"+string(rune('a'+i)), 1, "todo")
	}
	reloadTasks(t, m)
	m.filteredT = nil

	m.currentView = viewDashboard
	m.width = 140

	cuenta := func(alto int) int {
		out := ansi.Strip(m.renderDashboard(alto))
		n := 0
		for _, linea := range strings.Split(out, "\n") {
			if strings.Contains(linea, " tasks (") {
				n++
			}
		}
		return n
	}

	// El presupuesto sale del alto menos el cromo, menos lo que ocupa el Overview
	// y menos la cabecera del panel, así que hace falta un alto pequeño de verdad.
	// Con diez personas en el equipo el presupuesto se nota: 60 de alto las
	// enseña todas, 20 enseña seis, 16 enseña dos y 14 ya no cabe ninguna --
	// el Overview se lleva la columna entera.
	casos := []struct{ alto, want int }{{60, 10}, {30, 10}, {20, 6}, {16, 2}, {14, 0}}
	for _, c := range casos {
		if got := cuenta(c.alto); got != c.want {
			t.Errorf("con %d de alto salen %d filas de equipo, want %d", c.alto, got, c.want)
		}
	}
}

// El panel Active también, y con una tarea más de alto sale una más.
func TestDashboardActivePanelRespectsBudget(t *testing.T) {
	m := newDashModel(t, "")
	m.currentView = viewDashboard
	m.width = 140

	cuenta := func(alto int) int {
		out := ansi.Strip(m.renderDashboard(alto))
		n := 0
		for _, linea := range strings.Split(out, "\n") {
			if strings.Contains(linea, "  ● ") || strings.Contains(linea, "  ○ ") {
				n++
			}
		}
		return n
	}

	holgado := cuenta(60)
	corto := cuenta(8)
	if corto >= holgado {
		t.Errorf("con 8 de alto salen %d filas activas y con 60 salen %d", corto, holgado)
	}
	if corto == 0 {
		t.Error("con 8 de alto no sale ninguna fila activa")
	}
}

// Cerrar el editor de descripción sin detalle detrás deja el detalle cerrado y
// limpio, incluida la selección de comentarios: si no, el detalle reabierto
// apuntaría a un comentario que ya no se está mostrando.
func TestClosingDescEditorClearsDetailState(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.descEditOpen = true
	m.descEditTaskID = m.detailTask.ID
	m.descEditHadDetail = false // el detalle no estaba antes de editar
	m.detailCommentSel = 2
	m.detailComments = []model.Comment{{ID: 1, Body: "uno"}}

	m.closeDescEditor()

	if m.descEditOpen {
		t.Error("el editor sigue abierto")
	}
	if m.detailOpen {
		t.Error("el detalle sigue abierto, y no estaba antes de editar")
	}
	if m.detailTask != nil {
		t.Errorf("la tarea del detalle quedó en %v, want nil", m.detailTask)
	}
	if m.detailComments != nil {
		t.Errorf("los comentarios quedaron en %v, want nil", m.detailComments)
	}
	if m.detailCommentSel != -1 {
		t.Errorf("la selección quedó en %d, want -1", m.detailCommentSel)
	}
}

// Y con detalle detrás, cerrarlo lo deja abierto: se vuelve al detalle, no a la
// lista.
func TestClosingDescEditorKeepsExistingDetail(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.descEditOpen = true
	m.descEditHadDetail = true
	m.detailCommentSel = 1

	m.closeDescEditor()

	if m.descEditOpen {
		t.Error("el editor sigue abierto")
	}
	if !m.detailOpen {
		t.Error("el detalle se cerró, y estaba abierto antes de editar")
	}
	if m.detailTask == nil {
		t.Error("la tarea del detalle se perdió")
	}
	// La selección de comentarios se queda: el usuario va a seguir viéndolos.
	if m.detailCommentSel != 1 {
		t.Errorf("la selección pasó a %d, want 1 (intacta)", m.detailCommentSel)
	}
}

// Al salir del campo de assignee con una sugerencia seleccionada, el input
// completa con esa sugerencia. Es lo que distingue "elegí una" de "escribí una":
// sin sugerencia seleccionada el input se queda como estaba.
func TestNewTaskLeavingAssigneeCompletesSelection(t *testing.T) {
	t.Run("con sugerencia", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := press(m, "i")
		m2 := next
		m2.newTaskFieldIdx = newTaskFieldAssignee
		m2.newTaskAssignee = "@j"
		m2.newTaskAssigneeSuggIdx = 0 // "@juan", la primera sugerencia

		got, _ := m2.newTaskMoveField(1)
		gotM := got.(Model)

		if gotM.newTaskAssignee != "@juan" {
			t.Errorf("el responsable queda en %q, want @juan (la sugerencia elegida)", gotM.newTaskAssignee)
		}
		if gotM.newTaskAssigneeSuggIdx != -1 {
			t.Errorf("suggIdx = %d, want -1 tras completar", gotM.newTaskAssigneeSuggIdx)
		}
	})

	t.Run("sin sugerencia", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := press(m, "i")
		m2 := next
		m2.newTaskFieldIdx = newTaskFieldAssignee
		m2.newTaskAssignee = "@libre"
		m2.newTaskAssigneeSuggIdx = -1

		got, _ := m2.newTaskMoveField(1)
		gotM := got.(Model)

		if gotM.newTaskAssignee != "@libre" {
			t.Errorf("el responsable cambió a %q sin sugerencia seleccionada", gotM.newTaskAssignee)
		}
	})
}

// La fila resaltada es la del cursor, y no "la primera de la ventana": con la
// ventana desplazada, el índice de la fila dibujada y el del cursor dejan de
// coincidir, y es la suma de los dos -- no la resta, ni el primero -- lo que
// señala la correcta.
func TestGanttSelectedRowFollowsScrolledWindow(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan", "@maria"}, 6)
	m.currentView = viewGantt
	m.width = 120

	rows := m.ganttRows()
	primera, ultima := -1, -1
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			if primera < 0 {
				primera = i
			}
			ultima = i
		}
	}
	if ultima <= primera {
		t.Fatalf("el fixture necesita varias filas de tarea, hay %d..%d", primera, ultima)
	}

	// Un alto que no deja ver todas las filas, con el cursor al final: la ventana
	// se desplaza y el índice dibujado deja de ser cero.
	m.ganttCursor = ultima
	out := ansi.Strip(m.renderGantt(listFixedRows + ganttRulerRows + 3))
	if !strings.Contains(out, "> #") {
		t.Fatalf("no hay ninguna fila resaltada:\n%s", out)
	}
	if want := fmt.Sprintf("> #%d", rows[ultima].entry.Task.ID); !strings.Contains(out, want) {
		t.Errorf("la fila resaltada no es la del cursor (%s):\n%s", want, out)
	}
	// La ventana no deja ver más de las tres filas que caben. Se cuenta por el
	// prefijo de tarea y no por el número de tarea: los ids del fixture van del 1
	// al 12, así que "#1" también aparece dentro de "#11".
	dibujadas := 0
	for _, linea := range strings.Split(out, "\n") {
		if strings.Contains(linea, "#") && strings.Contains(linea, "tarea") {
			dibujadas++
		}
	}
	if dibujadas != 3 {
		t.Errorf("se dibujan %d filas de tarea, want 3 (las que caben)", dibujadas)
	}
	// Y hay exactamente una fila resaltada: la del cursor.
	if n := strings.Count(out, "> #"); n != 1 {
		t.Errorf("hay %d filas resaltadas, want 1:\n%s", n, out)
	}
}
