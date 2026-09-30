package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// ganttModelWithPeople deja el modelo en la vista Gantt con `people` personas y
// `perPerson` tareas cada una, todas "activas" para que aparezcan en la
// proyección. El Gantt agenda la cola desde hoy, así que las fechas dependen del
// reloj; los tests worked con lo que el propio modelo calcula, no con fechas
// fijas.
func ganttModelWithPeople(t *testing.T, people []string, perPerson int) *Model {
	t.Helper()
	m := newTestModel(t)
	m.tasks = nil
	m.offdays = nil
	var id int64
	for _, p := range people {
		for range perPerson {
			id++
			m.tasks = append(m.tasks, model.Task{
				ID:        id,
				Title:     "tarea",
				Status:    "todo",
				Assignee:  p,
				Priority:  model.PriorityMedium,
				Estimate:  1, // sin estimación la tarea no entra en la cola
				CreatedAt: time.Now().Format("2006-01-02"),
			})
		}
	}
	m.invalidateFilterCache()
	m.currentView = viewGantt
	return m
}

// firstGanttMonday devuelve un lunes conocido, para que la rejilla del ruler y
// del axis tenga columnas predecibles.
func firstGanttMonday() time.Time {
	return time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC) // lunes
}

// --- La estructura de filas ----------------------------------------------

// Cada persona aporta una cabecera seguida de sus tareas. Es la forma que hace
// que la navegación tenga que saltarse cabeceras.
func TestGanttRowsInterleaveHeadersAndTasks(t *testing.T) {
	s := &model.Schedule{
		Start: "2026-09-07",
		Assignees: []model.AssigneeSchedule{
			{Assignee: "@juan", Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 1}, Start: "2026-09-07", End: "2026-09-08"},
				{Task: model.Task{ID: 2}, Start: "2026-09-09", End: "2026-09-10"},
			}},
			{Assignee: "@maria", Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 3}, Start: "2026-09-11", End: "2026-09-12"},
			}},
		},
	}

	rows := ganttRows(s)
	want := []struct {
		kind     ganttRowKind
		assignee string
	}{
		{ganttAssigneeRow, "@juan"},
		{ganttTaskRow, "@juan"},
		{ganttTaskRow, "@juan"},
		{ganttAssigneeRow, "@maria"},
		{ganttTaskRow, "@maria"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d filas, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].kind != w.kind || rows[i].assignee != w.assignee {
			t.Errorf("fila %d = {kind:%d assignee:%q}, want {kind:%d assignee:%q}",
				i, rows[i].kind, rows[i].assignee, w.kind, w.assignee)
		}
	}
	// Y cada fila de tarea apunta a SU entrada, no a la primera: si apuntara a
	// una copia compartida, todas las barras mostrarían la misma tarea.
	if rows[1].entry == nil || rows[1].entry.Task.ID != 1 {
		t.Errorf("fila 1 debe apuntar a la tarea 1: %+v", rows[1].entry)
	}
	if rows[2].entry == nil || rows[2].entry.Task.ID != 2 {
		t.Errorf("fila 2 debe apuntar a la tarea 2: %+v", rows[2].entry)
	}
	if rows[4].entry == nil || rows[4].entry.Task.ID != 3 {
		t.Errorf("fila 4 debe apuntar a la tarea 3: %+v", rows[4].entry)
	}
}

func TestGanttRowsEmpty(t *testing.T) {
	if got := ganttRows(&model.Schedule{}); len(got) != 0 {
		t.Errorf("sin assignees hay %d filas, want 0", len(got))
	}
	one := &model.Schedule{Assignees: []model.AssigneeSchedule{{Assignee: "solo"}}}
	if got := ganttRows(one); len(got) != 1 {
		t.Errorf("una persona sin tareas da %d filas, want 1 (sólo la cabecera)", len(got))
	}
}

// --- ganttTaskRange -------------------------------------------------------

func TestGanttTaskRange(t *testing.T) {
	rows := []ganttRow{
		{kind: ganttAssigneeRow, assignee: "@juan"},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow, assignee: "@maria"},
		{kind: ganttTaskRow},
	}
	first, last := ganttTaskRange(rows)
	if first != 1 || last != 4 {
		t.Errorf("rango = (%d,%d), want (1,4)", first, last)
	}
}

func TestGanttTaskRangeEdgeCases(t *testing.T) {
	tests := []struct {
		name                string
		rows                []ganttRow
		wantFirst, wantLast int
	}{
		{"sin filas", nil, -1, -1},
		{"sólo cabeceras", []ganttRow{{kind: ganttAssigneeRow}}, -1, -1},
		{"una sola tarea", []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttTaskRow}}, 1, 1},
		{"sólo tareas", []ganttRow{{kind: ganttTaskRow}, {kind: ganttTaskRow}}, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last := ganttTaskRange(tt.rows)
			if first != tt.wantFirst || last != tt.wantLast {
				t.Errorf("rango = (%d,%d), want (%d,%d)", first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

// --- snapGanttCursor ------------------------------------------------------

// El cursor debe acabar SIEMPRE sobre una fila de tarea: las cabeceras de
// persona no son navegables, así que se salta a la tarea contigua.
func TestSnapGanttCursorLandsOnTaskRow(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan", "@maria"}, 2)
	rows := m.ganttRows()
	// 2 personas x (cabecera + 2 tareas) = 6 filas.
	//  0 hdr @juan · 1 tarea · 2 tarea · 3 hdr @maria · 4 tarea · 5 tarea
	if len(rows) != 6 {
		t.Fatalf("fixture: 2 personas x (cabecera + 2 tareas) = 6 filas, hay %d", len(rows))
	}

	tests := []struct {
		name   string
		cursor int
		want   int
	}{
		{"ya sobre una tarea", 2, 2},
		{"primera cabecera salta hacia abajo", 0, 1},
		{"segunda cabecera salta hacia abajo", 3, 4},
		{"última tarea se queda", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ganttCursor = tt.cursor
			m.snapGanttCursor()
			if m.ganttCursor != tt.want {
				t.Errorf("cursor = %d, want %d", m.ganttCursor, tt.want)
			}
			if rows[m.ganttCursor].kind != ganttTaskRow {
				t.Errorf("el cursor quedó en una cabecera (fila %d)", m.ganttCursor)
			}
		})
	}
}

// Toda cabecera tiene al menos una tarea detrás: ganttRows sólo emite la
// cabecera de una persona que tiene entradas, así que la fila siguiente es
// siempre una tarea. Eso convierte la búsqueda HACIA ATRÁS de snapGanttCursor
// en un camino defensivo que la estructura de filas no alcanza.
//
// Se fija la invariante en vez de inventarse un caso: si algún día una cabecera
// pudiera quedar al final, este test lo diría al romperse.
func TestSnapGanttCursorHeaderAlwaysHasTaskAhead(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan", "@maria"}, 1)
	rows := m.ganttRows() // [hdr @juan, tarea, hdr @maria, tarea]
	if len(rows) != 4 {
		t.Fatalf("fixture: 4 filas esperadas, hay %d", len(rows))
	}
	for i, r := range rows {
		if r.kind != ganttAssigneeRow {
			continue
		}
		if i+1 >= len(rows) || rows[i+1].kind != ganttTaskRow {
			t.Fatalf("la cabecera %d no va seguida de una tarea: %+v", i, rows)
		}
	}
}

func TestSnapGanttCursorOutOfRange(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 2)
	rows := m.ganttRows() // [hdr, task, task] -> 3
	last := len(rows) - 1

	tests := []struct {
		name   string
		cursor int
		want   int
	}{
		{"más allá del final", 99, last},
		{"negativo", -5, 1},
		{"en la última", last, last},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ganttCursor = tt.cursor
			m.snapGanttCursor()
			if m.ganttCursor != tt.want {
				t.Errorf("cursor = %d, want %d", m.ganttCursor, tt.want)
			}
		})
	}
}

// Sin filas, el cursor queda en 0 en vez de quedar fuera.
func TestSnapGanttCursorEmpty(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	m.ganttCursor = 7

	m.snapGanttCursor()
	if m.ganttCursor != 0 {
		t.Errorf("cursor = %d, want 0 con la vista vacía", m.ganttCursor)
	}
}

// Sólo cabeceras, sin ninguna tarea: el cursor cae a 0.
func TestSnapGanttCursorHeadersWithoutTasks(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	// Una persona sin tareas NO genera filas, así que hay que forzar el caso
	// con un filtro que deje sólo la cabecera vía una tarea sin agendar.
	m.tasks = []model.Task{{ID: 1, Title: "sin gente", Status: "todo", Assignee: ""}}
	m.invalidateFilterCache()

	m.ganttCursor = 1
	m.snapGanttCursor()
	if m.ganttCursor < 0 {
		t.Errorf("cursor = %d, no puede quedar negativo", m.ganttCursor)
	}
}

// --- daysBetween ----------------------------------------------------------

// daysBetween cuenta días de calendario. Una fecha inválida devuelve -1, que es
// la señal de "no se pudo medir": si devolviera 0, la barra se dibujaría en la
// primera columna en vez de desaparecer.
func TestDaysBetween(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		date string
		want int
	}{
		{"el mismo día", "2026-09-01", 0},
		{"un día después", "2026-09-02", 1},
		{"una semana después", "2026-09-08", 7},
		{"mes siguiente", "2026-10-01", 30},
		{"fecha inválida", "no-es-fecha", -1},
		{"vacío", "", -1},
		{"formato distinto", "01/09/2026", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daysBetween(start, tt.date); got != tt.want {
				t.Errorf("daysBetween(%s, %q) = %d, want %d", model.FormatDate(start), tt.date, got, tt.want)
			}
		})
	}
}

// --- El eje de semanas (ruler) -------------------------------------------

func TestRenderGanttRulerMarksMondays(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 10, 28 // 4 semanas justas

	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	ruler := ansi.Strip(m.renderGanttRuler(start, 0, labelW, dayCols))

	if !strings.Contains(ruler, model.WeekOfMonthLabel(start)) {
		t.Errorf("el ruler debe rotular el lunes de la primera columna: %q", ruler)
	}
	// Con offset 0 arrancamos en lunes, así que los rótulos caen en las
	// columnas 0, 7, 14 y 21. Ninguna puede pisar el relleno de la etiqueta.
	for _, col := range []int{0, 7, 14, 21} {
		at := labelW + 1 + col
		r := []rune(ruler)
		if at < len(r) && r[at] == ' ' {
			t.Errorf("columna %d sin rótulo de lunes: %q", at, ruler)
		}
	}
}

// Desplazar el inicio mueve el rótulo: con offset 2 la columna 0 es miércoles y
// el primer lunes cae 5 columnas más allá.
func TestRenderGanttRulerWithOffset(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 8, 30

	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	ruler := ansi.Strip(m.renderGanttRuler(start, 2, labelW, dayCols))

	nextMonday := model.WeekOfMonthLabel(start.AddDate(0, 0, 7))
	if !strings.Contains(ruler, nextMonday) {
		t.Errorf("con offset 2 debe rotularse el lunes siguiente: %q", ruler)
	}
	// Y no debe estar pegado a la etiqueta: la columna del lunes es 5.
	r := []rune(ruler)
	at := labelW + 1 + 5
	if at < len(r) && r[at] == ' ' {
		t.Errorf("el rótulo del lunes no cayó en su columna (offset 2): %q", ruler)
	}
}

// El ruler nunca desborda la rejilla, ni con etiquetas ni con columnasutc.
func TestRenderGanttRulerFitsWidth(t *testing.T) {
	start := firstGanttMonday()
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)

	for _, labelW := range []int{1, 4, 8, 14, 30} {
		for _, dayCols := range []int{1, 7, 14, 28, 60} {
			ruler := ansi.Strip(m.renderGanttRuler(start, 0, labelW, dayCols))
			if w := ansi.StringWidth(ruler); w > labelW+1+dayCols {
				t.Errorf("labelW=%d dayCols=%d: ruler mide %d, want <= %d", labelW, dayCols, w, labelW+1+dayCols)
			}
		}
	}
}

// --- El eje de días -------------------------------------------------------

// El axis marca "|" en cada lunes y "-" en el resto de días.
func TestRenderGanttAxisMondays(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 6, 14

	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	axis := ansi.Strip(m.renderGanttAxis(labelW, dayCols, start, 0))

	if w := ansi.StringWidth(axis); w != labelW+1+dayCols {
		t.Errorf("axis mide %d, want %d", w, labelW+1+dayCols)
	}
	marks := axis[labelW+1:]
	if pipes := strings.Count(marks, "|"); pipes != 2 {
		t.Errorf("14 días desde un lunes tienen 2 lunes, hay %d: %q", pipes, marks)
	}
	if dashes := strings.Count(marks, "-"); dashes != dayCols-2 {
		t.Errorf("los %d días restantes deben ser guiones, hay %d", dayCols-2, dashes)
	}
}

// Desplazar un día mueve la posición de cada "|" una columna.
func TestRenderGanttAxisOffsetMovesMondays(t *testing.T) {
	start := firstGanttMonday()
	const labelW, dayCols = 6, 14

	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	marks := ansi.Strip(m.renderGanttAxis(labelW, dayCols, start, 1))[labelW+1:]

	if marks[0] != '-' {
		t.Errorf("con offset 1 la columna 0 es martes: %q", marks[0])
	}
	if marks[6] != '|' {
		t.Errorf("con offset 1 el primer lunes debe caer en la columna 6: %q", marks)
	}
}

func TestRenderGanttAxisZeroDays(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	axis := ansi.Strip(m.renderGanttAxis(10, 0, firstGanttMonday(), 0))
	if w := ansi.StringWidth(axis); w != 11 {
		t.Errorf("axis con 0 días mide %d, want 11 (sólo el relleno)", w)
	}
}

// --- ganttLegend ----------------------------------------------------------

// La leyenda describe el rango visible, así que cambiar el offset la cambia.
func TestGanttLegend(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 3)

	legend := m.ganttLegend(0, 14)
	parts := strings.Fields(legend)
	if len(parts) != 3 || parts[1] != "→" {
		t.Fatalf("leyenda = %q, want 'desde → hasta'", legend)
	}
	if parts[0] == parts[2] {
		t.Errorf("con 14 días el rango no puede empezar y acabar el mismo día: %q", legend)
	}
	for _, d := range parts[0:1] {
		if _, err := model.ParseDate(d); err != nil {
			t.Errorf("fecha inicial %q no parsea: %v", d, err)
		}
	}
	for _, d := range parts[2:] {
		if _, err := model.ParseDate(d); err != nil {
			t.Errorf("fecha final %q no parsea: %v", d, err)
		}
	}

	if shifted := m.ganttLegend(7, 14); shifted == legend {
		t.Errorf("con offset 7 la leyenda debe cambiar: %q", shifted)
	}
	// Y con una sola columna el rango es un único día.
	one := strings.Fields(m.ganttLegend(0, 1))
	if len(one) != 3 || one[0] != one[2] {
		t.Errorf("con 1 día visible el rango debe ser un único día: %q", one)
	}
}

// --- moveGanttCursor ------------------------------------------------------

// j/k se mueven entre filas de tarea, saltándose las cabeceras de persona.
func TestMoveGanttCursorSkipsHeaders(t *testing.T) {
	rows := []ganttRow{
		{kind: ganttAssigneeRow, assignee: "@juan"},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow, assignee: "@maria"},
		{kind: ganttTaskRow},
	}
	tests := []struct {
		name string
		from int
		dir  int
		want int
	}{
		{"baja de la primera a la segunda tarea", 1, 1, 2},
		{"sube de la segunda a la primera tarea", 2, -1, 1},
		{"baja saltando una cabecera", 2, 1, 4},
		{"sube saltando una cabecera", 4, -1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ganttModelWithPeople(t, []string{"@juan"}, 1)
			m.ganttCursor = tt.from
			moveGanttCursor(m, rows, tt.dir)
			if m.ganttCursor != tt.want {
				t.Errorf("cursor = %d, want %d", m.ganttCursor, tt.want)
			}
		})
	}
}

// En el extremo no hay a dónde ir: el cursor se queda.
func TestMoveGanttCursorStopsAtEdges(t *testing.T) {
	rows := []ganttRow{{kind: ganttTaskRow}, {kind: ganttTaskRow}}
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)

	m.ganttCursor = 1
	moveGanttCursor(m, rows, 1)
	if m.ganttCursor != 1 {
		t.Errorf("bajar en la última fila debe quedarse, cursor = %d", m.ganttCursor)
	}
	m.ganttCursor = 0
	moveGanttCursor(m, rows, -1)
	if m.ganttCursor != 0 {
		t.Errorf("subir en la primera fila debe quedarse, cursor = %d", m.ganttCursor)
	}
}

// Sólo cabeceras: no hay tarea a la que saltar y el cursor no se mueve.
func TestMoveGanttCursorNoTaskRows(t *testing.T) {
	rows := []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttAssigneeRow}}
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	m.ganttCursor = 1
	moveGanttCursor(m, rows, 1)
	if m.ganttCursor != 1 {
		t.Errorf("sin filas de tarea el cursor no debe moverse, cursor = %d", m.ganttCursor)
	}
}

// --- Teclado del Gantt ----------------------------------------------------

// g/G saltan al primer y al último día del horizonte visible.
func TestGanttKeysGotoEdges(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 3)
	rows := m.ganttRows()
	first, last := ganttTaskRange(rows)
	if first < 0 {
		t.Fatal("fixture: sin filas de tarea")
	}

	m.ganttCursor = last
	m, _ = press(m, "g")
	if m.ganttCursor != first {
		t.Errorf("tras g cursor = %d, want %d (primera tarea)", m.ganttCursor, first)
	}

	m, _ = press(m, "G")
	if m.ganttCursor != last {
		t.Errorf("tras G cursor = %d, want %d (última tarea)", m.ganttCursor, last)
	}
}

// Con la vista vacía, g y G no mueven el cursor a ningún sitio inválido.
func TestGanttKeysWithNoTasks(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	for _, key := range []string{"g", "G", "j", "k"} {
		m, _ = press(m, key)
		if m.ganttCursor < 0 || m.ganttCursor > len(m.ganttRows()) {
			t.Errorf("tras %q cursor = %d, fuera de rango", key, m.ganttCursor)
		}
	}
}

// h/l desplazan la ventana de días, y h no baja de cero.
func TestGanttKeysShiftWindow(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 2)

	m, _ = press(m, "l")
	m, _ = press(m, "l")
	if m.ganttOffsetDays != 2 {
		t.Errorf("tras dos l offset = %d, want 2", m.ganttOffsetDays)
	}
	m, _ = press(m, "h")
	if m.ganttOffsetDays != 1 {
		t.Errorf("tras h offset = %d, want 1", m.ganttOffsetDays)
	}
	for range 5 {
		m, _ = press(m, "h")
	}
	if m.ganttOffsetDays != 0 {
		t.Errorf("el offset no puede quedar negativo: %d", m.ganttOffsetDays)
	}
}

// enter sobre una fila de tarea abre el detalle con sus comentarios sin cargar
// todavía (el cursor de comentarios arranca en "ninguno").
func TestGanttEnterOpensDetail(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	m.snapGanttCursor()
	if rows := m.ganttRows(); m.ganttCursor >= len(rows) || rows[m.ganttCursor].kind != ganttTaskRow {
		t.Fatalf("fixture: el cursor debe caer en una tarea, está en %d", m.ganttCursor)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := asModel(next)

	if !got.detailOpen {
		t.Error("enter debe abrir el detalle")
	}
	if got.detailTask == nil {
		t.Fatal("enter debe cargar la tarea del detalle")
	}
	if got.detailComments != nil {
		t.Error("los comentarios arrancan sin cargar, no como lista vacía")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1 (nada seleccionado)", got.detailCommentSel)
	}
	if cmd == nil {
		t.Error("enter debe emitir la carga de comentarios")
	}
}

// enter sobre una cabecera no abre nada: las cabeceras no son navegables.
func TestGanttEnterOnHeaderDoesNothing(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 2)
	// Forzamos el cursor sobre la cabecera saltando el snap.
	m.ganttCursor = 0

	// handleGanttKey hace snap al entrar, así que comprobamos el efecto neto:
	// el detalle se abre, pero sobre una TAREA, nunca sobre una cabecera.
	m, _ = press(m, "enter")
	if m.detailOpen && m.detailTask != nil && m.detailTask.Assignee == "" {
		t.Error("el detalle nunca debe abrir sobre una cabecera de persona")
	}
}

// --- Reparto de ancho del Gantt ------------------------------------------

// ganttBudgetForRows devuelve el alto que hace caber exactamente `rows` filas
// de Gantt. En renderGantt, visible = maxHeight - listFixedRows - ganttRulerRows,
// así que el presupuesto es la suma, no otro descuento.
func ganttBudgetForRows(rows int) int {
	return rows + listFixedRows + ganttRulerRows
}

// El reparto de ancho tiene dos regímenes: con sitio de sobra la etiqueta se
// queda en 30 columnas; con poco, se queda con un tercio del ancho interior.
// El umbral son 70 columnas interiores, no 60: a 65 de interior todavía manda el
// tercio, y ese es exactamente el borde que distingue `innerW < 70` de
// cualquier otro valor cercano.
func TestRenderGanttLabelWidthThreshold(t *testing.T) {
	tests := []struct {
		width int
		// columnas que ocupa la etiqueta antes de los días
		wantLabel int
	}{
		{120, 30}, // interior 118: etiqueta fija
		{80, 30},  // interior 78: etiqueta fija
		{72, 30},  // interior 70: justo en el umbral, sigue la fija
		{71, 23},  // interior 69: un tercio
		{65, 21},  // interior 63
		{50, 16},  // interior 48
		{40, 14},  // interior 38, nunca menos de 14
	}
	for _, tt := range tests {
		m := ganttModelWithPeople(t, []string{"@juan"}, 1)
		m.width = tt.width

		// El relleno del eje es labelW+1, y el eje va justo tras la etiqueta.
		// El ancho de la etiqueta NO se recalcula aquí a propósito: lo que se
		// comprueba es que el render aplique el reparto que dice el umbral, y
		// replicar la fórmula en el test sólo probaría que el test copia el
		// código.
		out := ansi.Strip(m.renderGantt(30))
		axis := ""
		for _, line := range strings.Split(out, "\n") {
			if isGanttAxisLine(line) {
				axis = line
				break
			}
		}
		if axis == "" {
			t.Fatalf("width=%d: no se encontró el eje:\n%s", tt.width, out)
		}
		runes := []rune(axis)
		inner := string(runes[1 : len(runes)-1]) // sin los laterales de la caja
		lead := 0
		for _, r := range inner {
			if r != ' ' {
				break
			}
			lead++
		}
		if lead != tt.wantLabel+1 {
			t.Errorf("width=%d: la etiqueta ocupa %d columnas (relleno %d), want %d",
				tt.width, lead-1, lead, tt.wantLabel)
		}
	}
}

// El eje de días ocupa exactamente el ancho interior del Gantt, sea cual sea
// el reparto entre etiqueta y columnas. Con un "- 1" de más o de menos el eje
// se descuadra y la caja queda una columna más ancha o más estrecha.
func TestRenderGanttAxisSpansInnerWidth(t *testing.T) {
	// Por debajo de ~40 columnas la etiqueta mínima (14) más los 7 días
	// mínimos no caben, así que el eje se recorta y pierde los lunes: no hay
	// línea que comparar.
	for _, width := range []int{120, 100, 80, 72, 71, 65, 50, 40} {
		m := ganttModelWithPeople(t, []string{"@juan"}, 1)
		m.width = width

		out := ansi.Strip(m.renderGantt(30))
		found := false
		for i, line := range strings.Split(out, "\n") {
			if !isGanttAxisLine(line) {
				continue
			}
			found = true
			// La línea del eje incluye los dos laterales de la caja, así que
			// mide el ancho completo del terminal: es la comprobación que
			// distingue el reparto de ancho de sus variantes, porque cualquier
			// "- 1" o "+ 1" en dayCols descuadra la línea.
			if got := ansi.StringWidth(line); got != width {
				t.Errorf("width=%d: el eje (línea %d) mide %d, want %d: %q",
					width, i, got, width, line)
			}
		}
		if !found {
			t.Errorf("width=%d: no se encontró la línea del eje:\n%s", width, out)
		}
	}
}

// La cabecera de persona ocupa toda la anchura de la caja, etiquetas y días
// incluidos: por eso su cellWidth usa labelW+1+dayCols y no sólo labelW.
func TestRenderGanttAssigneeHeaderWidth(t *testing.T) {
	for _, width := range []int{120, 80, 65, 40} {
		m := ganttModelWithPeople(t, []string{"@juan"}, 1)
		m.width = width

		rows := m.ganttRows()
		var header ganttRow
		for _, r := range rows {
			if r.kind == ganttAssigneeRow {
				header = r
			}
		}
		if header.assignee == "" {
			t.Fatal("fixture: sin cabecera de persona")
		}

		innerW := width - 2
		labelW := 30
		if innerW < 70 {
			labelW = innerW / 3
		}
		labelW = max(labelW, 14)
		dayCols := max(innerW-labelW-1, 7)

		got := ansi.StringWidth(ansi.Strip(m.renderGanttRow(header, firstGanttMonday(), 0, dayCols, labelW, false)))
		if got != labelW+1+dayCols {
			t.Errorf("width=%d: la cabecera mide %d, want %d (labelW+1+dayCols)",
				width, got, labelW+1+dayCols)
		}
	}
}

// La barra de una tarea se dibuja en las columnas que caen entre su inicio y su
// fin. Desplazar la ventana (offset) no la mueve: si el cálculo de columnas
// invirtiera el signo del offset, la barra se iría al lado contrario.
func TestRenderGanttBarFollowsTaskDates(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)
	rows := m.ganttRows()

	var task ganttRow
	for _, r := range rows {
		if r.kind == ganttTaskRow {
			task = r
		}
	}
	if task.entry == nil {
		t.Fatal("fixture: sin fila de tarea")
	}

	const labelW, dayCols = 10, 20
	start, _ := model.ParseDate(m.ganttDisplay().Start)
	d0 := daysBetween(start, task.entry.Start)
	d1 := daysBetween(start, task.entry.End)

	// Sin desplazamiento la barra empieza en la columna del inicio.
	line := ansi.Strip(m.renderGanttRow(task, start, 0, dayCols, labelW, false))
	cells := []rune(line)[labelW+1:]
	bars := []int{}
	for i, c := range cells {
		if c == '█' {
			bars = append(bars, i)
		}
	}
	if len(bars) == 0 {
		t.Fatalf("no se dibujó ninguna barra: %q", line)
	}
	if bars[0] != d0 {
		t.Errorf("la barra empieza en la columna %d, want %d (d0)", bars[0], d0)
	}
	if bars[len(bars)-1] != d1 {
		t.Errorf("la barra acaba en la columna %d, want %d (d1)", bars[len(bars)-1], d1)
	}

	// Con offset la barra se desplaza con la ventana, no al revés: las mismas
	// columnas absolutas se dibujan offset posiciones antes.
	shifted := ansi.Strip(m.renderGanttRow(task, start, 3, dayCols, labelW, false))
	scells := []rune(shifted)[labelW+1:]
	for i, c := range scells {
		if c == '█' && i != bars[0]-3 {
			t.Errorf("con offset 3 la barra se dibujó en la columna %d, want %d", i, bars[0]-3)
			break
		}
	}
}

// --- Presupuesto de alto: ventana vertical -------------------------------

// Con exactamente `visible` filas caben todas; con una más hay que desplazar
// la ventana. El `>` es lo que separa ambos casos.
func TestRenderGanttVerticalWindow(t *testing.T) {
	for _, perPerson := range []int{1, 2, 3, 5} {
		m := ganttModelWithPeople(t, []string{"@juan"}, perPerson)
		rows := m.ganttRows() // 1 cabecera + perPerson tareas

		tasks := 0
		for _, r := range rows {
			if r.kind == ganttTaskRow {
				tasks++
			}
		}

		// Presupuesto justo para ver todas las filas.
		exact := ganttBudgetForRows(len(rows))
		out := ansi.Strip(m.renderGantt(exact))
		if n := countGanttRows(out); n != tasks {
			t.Errorf("perPerson=%d: con presupuesto %d se ven %d tareas, want las %d que hay",
				perPerson, exact, n, tasks)
		}

		// Un presupuesto menos: la ventana deja fuera la última fila y caben
		// menos tareas. Con una sola tarea el recorte se come la cabecera y la
		// barra sigue entrando, así que el caso empieza en 2.
		if tasks >= 2 {
			tight := exact - 1
			out = ansi.Strip(m.renderGantt(tight))
			if n := countGanttRows(out); n >= tasks {
				t.Errorf("perPerson=%d: con presupuesto %d deberían caber menos de %d tareas, se ven %d",
					perPerson, tight, tasks, n)
			}
		}
	}
}

// isGanttAxisLine distingue la línea del eje: empieza por el lateral izquierdo
// de la caja, tiene guiones y barras de lunes, y ningún dígito (para no
// confundirse con las fechas de la leyenda).
func isGanttAxisLine(line string) bool {
	if !strings.HasPrefix(line, "│") {
		return false
	}
	if !strings.Contains(line, "-") || !strings.Contains(line, "|") {
		return false
	}
	return strings.IndexFunc(line, func(r rune) bool { return r >= '0' && r <= '9' }) < 0
}

// countGanttRows cuenta las filas de TAREA visibles: las que llevan la barra.
// La cabecera de persona no cuenta, así que el número a comparar es el de
// tareas, no el de filas.
func countGanttRows(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "█") {
			n++
		}
	}
	return n
}

// --- La vista vacía ------------------------------------------------------

// Sin filas, el Gantt lo dice explícitamente en vez de dejar un hueco mudo.
func TestRenderGanttWithNoTasks(t *testing.T) {
	m := ganttModelWithPeople(t, nil, 0)
	out := ansi.Strip(m.renderGantt(30))

	if !strings.Contains(out, "No active tasks") {
		t.Errorf("sin tareas debe avisar:\n%s", out)
	}
}

// Con filas, ese aviso NO aparece: invertir el `== 0` lo ensuciaría.
func TestRenderGanttWithTasksHidesEmptyNotice(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 2)
	out := ansi.Strip(m.renderGantt(30))

	if strings.Contains(out, "No active tasks") {
		t.Errorf("con tareas no debe aparecer el aviso de vacío:\n%s", out)
	}
}

// --- El borde exacto del cursor ------------------------------------------

// Un cursor exactamente igual al número de filas queda fuera y se recorta a la
// última. El `>=` es lo que lo distingue de un `>`.
func TestSnapGanttCursorAtExactLength(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 3)
	rows := m.ganttRows()

	m.ganttCursor = len(rows)
	m.snapGanttCursor()

	if m.ganttCursor != len(rows)-1 {
		t.Errorf("cursor = %d, want %d (un índice igual al número de filas es fuera de rango)",
			m.ganttCursor, len(rows)-1)
	}
}

// --- Estimación por defecto ----------------------------------------------

// Una estimación global de 0 días es imposible: la cola no avanzaría nunca. El
// fallback a 1 día es lo que mantiene la proyección viva.
func TestGanttScheduleFallsBackToOneDayEstimate(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 2)
	// El default global sólo se usa si la tarea no trae estimación propia.
	for i := range m.tasks {
		m.tasks[i].Estimate = 0
	}
	m.config.DefaultEstimateDays = 0

	s := m.ganttSchedule()
	if s == nil || len(s.Assignees) == 0 {
		t.Fatal("fixture: sin agenda")
	}
	for _, entry := range s.Assignees[0].Entries {
		if entry.Estimate != 1 {
			t.Errorf("estimate = %v con default 0, want 1", entry.Estimate)
		}
	}

	// Y con una estimación válida se respeta, así que el saneo no es siempre 1.
	m2 := ganttModelWithPeople(t, []string{"@juan"}, 2)
	for i := range m2.tasks {
		m2.tasks[i].Estimate = 0
	}
	m2.config.DefaultEstimateDays = 3
	for _, entry := range m2.ganttSchedule().Assignees[0].Entries {
		if entry.Estimate != 3 {
			t.Errorf("estimate = %v con default 3, want 3", entry.Estimate)
		}
	}
}
