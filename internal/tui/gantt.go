package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

// ganttRulerRows son las filas fijas del Gantt que no son de datos: la regla de
// semanas y su eje.
const ganttRulerRows = 2

// ganttRowKind distingue una cabecera de persona de una fila de tarea.
type ganttRowKind int

const (
	ganttAssigneeRow ganttRowKind = iota
	ganttTaskRow
)

// ganttRow es una fila navegable del Gantt.
type ganttRow struct {
	kind     ganttRowKind
	assignee string
	entry    *model.ScheduleEntry
}

// ganttSchedule proyecta la cola de cada persona a partir de TODAS las tareas
// y off-days cargados. El punto de partida es hoy. La cola es global: la
// capacidad de una persona se reparte entre todos sus proyectos, así que las
// fechas no dependen de qué proyecto estés mirando.
func (m *Model) ganttSchedule() *model.Schedule {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// El saneo del estimate (0 o negativo -> 1 día) lo hace BuildSchedule. Aquí
	// había un guard idéntico que sólo podía producir el mismo valor: su mutant
	// era equivalente por construcción, no una laguna de cobertura.
	return model.BuildSchedule(m.tasks, m.offdays, start, m.config.DefaultEstimateDays)
}

// ganttDisplay aplica los filtros activos como filtro de VISTA sobre la
// proyección global: sólo decide qué filas se ven, sin recalcular fechas.
func (m *Model) ganttDisplay() *model.Schedule {
	return model.FilterSchedule(m.ganttSchedule(), func(t model.Task) bool {
		return m.taskMatchesFilter(t)
	})
}

// ganttRows aplana un schedule en filas navegables: cabecera por persona y una
// fila por tarea, en el orden de la cola.
func ganttRows(s *model.Schedule) []ganttRow {
	var rows []ganttRow
	for _, a := range s.Assignees {
		rows = append(rows, ganttRow{kind: ganttAssigneeRow, assignee: a.Assignee})
		for i := range a.Entries {
			rows = append(rows, ganttRow{kind: ganttTaskRow, assignee: a.Assignee, entry: &a.Entries[i]})
		}
	}
	return rows
}

// ganttRows devuelve las filas del schedule actual, ya filtradas para la vista.
func (m *Model) ganttRows() []ganttRow {
	return ganttRows(m.ganttDisplay())
}

// snapGanttCursor reencuadra el cursor dentro de las filas actuales y lo apoya
// siempre sobre una fila de tarea: las cabeceras de persona no son navegables.
//
// Que "siempre" sea cierto es estructural: ganttRows sólo emite la cabecera de
// una persona que tiene entradas, así que la fila siguiente a una cabecera es
// siempre una tarea, y la búsqueda hacia delante la encuentra. Los llamantes no
// necesitan comprobar el tipo de la fila.
func (m *Model) snapGanttCursor() {
	rows := m.ganttRows()
	if len(rows) == 0 {
		m.ganttCursor = 0
		return
	}
	m.ganttCursor = max(m.ganttCursor, 0)
	if m.ganttCursor >= len(rows) {
		m.ganttCursor = len(rows) - 1
	}
	if rows[m.ganttCursor].kind == ganttTaskRow {
		return
	}
	// Cabecera: saltar a la primera tarea posterior. No hay caso "si no hay, a la
	// anterior": ganttRows sólo emite la cabecera de una persona que tiene
	// entradas, así que la fila siguiente a una cabecera es siempre una tarea y
	// esta búsqueda termina siempre. La búsqueda hacia atrás que había aquí no
	// era una red de seguridad: era código muerto con su propio comentario
	// diciendo que no podía llegar a ejecutarse.
	//
	// Se recorre con range sobre sub-rebanadas en vez de con un for de índice
	// manual: un `i++` invertido deja el bucle colgado y el mutant se reporta
	// como TIMED OUT, no como muerto.
	// El salto se hace con un índice que cuenta desde el cursor, y no con el
	// desplazamiento dentro de la sub-rebanada: `1 + i` con i siendo el índice
	// de la sub-rebanada era una suma de dos números que describen lo mismo, y
	// su mutante (`+ 2 * i`, `+ i - i`) no se distinguía porque i siempre era 0
	// -- la fila siguiente a una cabecera es siempre una tarea.
	//
	// Con nextGanttTaskRow el salto es una operación con nombre y el índice es
	// el absoluto desde el principio, que es lo que se guarda. Menos aritmética
	// que pueda mutar sin que nadie lo note.
	m.ganttCursor = nextGanttTaskRow(rows, m.ganttCursor)
}

// filaEsTarea dice si el índice cae sobre una fila de tarea del gantt, es decir,
// si hay una tarea seleccionada.
//
// El rango va con inRange, el mismo helper que usa el Kanban para su cursor. La
// condición suelta -- `>= 0 && < len(rows)` -- tenía un `>= 0` cuyo mutante
// (`> 0`) sólo se distinguía en la fila 0, y la fila 0 de un gantt con filas es
// siempre una cabecera de persona: ganttRows emite la cabecera antes que las
// entradas de esa persona. Así que el mutante era indistinguible no por falta de
// test sino porque el valor que lo separaba no existía.
//
// inRange no tiene ese borde: comprobar que el índice está en la lista y que la
// fila de ese índice es una tarea son dos preguntas y las dos se contestan.
func filaEsTarea(rows []ganttRow, i int) bool {
	return inRange(i, len(rows)) && rows[i].kind == ganttTaskRow
}

// nextGanttTaskRow devuelve el índice de la primera fila de tarea en o después de
// `from`, o -1 si no hay ninguna.
//
// La búsqueda empieza en `from` y no en `from+1` a propósito: quien llama ya sabe
// que la fila actual no es una tarea, así que empezar en ella no cambia el
// resultado, y empezar en `from` hace que el caso "no hay ninguna fila" sea un
// -1 de verdad y no un acierto con i == 0.
func nextGanttTaskRow(rows []ganttRow, from int) int {
	for i := max(from, 0); i < len(rows); i++ {
		if rows[i].kind == ganttTaskRow {
			return i
		}
	}
	return -1
}

// ganttNoTasks es el centinela que devuelve ganttTaskRange cuando la vista no
// tiene ninguna fila de tarea. Es -1 y no 0 porque la fila 0 siempre es una
// cabecera de persona cuando hay tareas.
const ganttNoTasks = -1

// ganttTaskRange devuelve el índice de la primera y última fila de tarea del
// Gantt, o ganttNoTasks si no hay ninguna.
func ganttTaskRange(rows []ganttRow) (first, last int) {
	first, last = ganttNoTasks, ganttNoTasks
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	return first, last
}

// moveGanttCursor salta a la siguiente/anterior fila de tarea en la dirección
// dir (+1 baja, -1 sube), ignorando las cabeceras de persona.
//
// El recorrido va con slices.IndexFunc sobre la fila,y la condición del rango
// desaparece con él. Antes era `for i := cursor + dir; i >= 0 && i < len(rows)`
// y su `i >= 0` sólo se distinguía de `i > 0` si la fila 0 fuese una tarea, cosa
// que no ocurre nunca porque la fila 0 es siempre la cabecera de la primera
// persona. Con IndexFunc la búsqueda no tiene un límite inferior que comparar --
// es "desde aquí hasta el final" o "hasta aquí desde el principio", y las dos
// direcciones las dice el paso.
func moveGanttCursor(m *Model, rows []ganttRow, dir int) {
	// El salto devuelve un ok en vez de un centinela porque cualquier comparación
	// contra un -1 tiene el problema del borde: "distinto de -1" y "mayor que -1"
	// son la misma condición sobre enteros, así que su mutante no lo mata ningún
	// test; y "< -1" es siempre falsa, que rompe los saltos al índice 0.
	//
	// Con el ok, el "no hay salto" es un hecho y no un número, y la única pregunta
	// que hace el llamante es si hay salto. Su mutante -- negar el ok -- sí se ve:
	// el cursor se iría al centinela.
	if destino, ok := stepGanttCursor(rows, m.ganttCursor, dir); ok {
		m.ganttCursor = destino
	}
}

// stepGanttCursor devuelve el índice de la fila de tarea a la que salta el cursor
// desde `from` en la dirección `dir`, y un false si no hay ninguna.
//
// El segundo valor es un booleano y no un centinela por lo que dice el llamante:
// comparar un índice contra -1 tiene un borde que ningún test alcanza.
//
// Toda la aritmética vive aquí y no en el llamador, y es a propósito. Con el
// recorrido repartido entre moveGanttCursor y sus dos ramas, cada línea llevaba su
// propio `max(min(...))`: el suelo en 0 no se distinguía del suelo en 1, y el `+ 1`
// del offset no se distinguía de `+ 2`, porque los dos valores que los separan --
// un cursor negativo y un cursor más allá del final -- sólo se dan en estados que
// el recorrido anterior no producía. Aquí, en cambio, `from` es un parámetro: un
// test puede pasar el que quiera y comprobar cada rama por sus dos lados.
//
// Las dos direcciones se resuelven con el mismo bucle, sobre una sub-rebanada que
// va desde la posición de destino hasta el final o hasta el principio. La
// sub-rebanada se acota con un clamp porque un cursor fuera de rango no debe hacer
// que el slice se corte al revés.
func stepGanttCursor(rows []ganttRow, from, dir int) (int, bool) {
	if dir < 0 {
		hasta := clamp(from, 0, len(rows))
		for i, r := range slices.Backward(rows[:hasta]) {
			if r.kind == ganttTaskRow {
				return i, true
			}
		}
		return 0, false
	}

	desde := clamp(from+1, 0, len(rows))
	for i, r := range rows[desde:] {
		if r.kind == ganttTaskRow {
			return desde + i, true
		}
	}
	return 0, false
}

// clamp acota v al rango [lo, hi]. Con lo <= hi -- que es el caso de todos los
// usos aquí: el 0 es el suelo y el hi es la longitud de una lista -- el rango
// siempre tiene un valor dentro, así que no hay que decidir qué pasa si no.
func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// handleGanttKey navega el Gantt: j/k sobre filas, h/l desplaza la ventana de
// días, g/G va al inicio/fin, Enter abre el detalle de la tarea.
func (m Model) handleGanttKey(key string) (tea.Model, tea.Cmd) {
	m.snapGanttCursor()
	rows := m.ganttRows()

	switch key {
	case "/":
		return m, m.openFilterModal()
	case "tab":
		// Cambiar de proyecto ciclando el filtro Project, como en el Dashboard.
		m.cycleProjectFilter(1)
		m.ganttCursor = 0
		m.snapGanttCursor()
	case "j", "down":
		moveGanttCursor(&m, rows, 1)
	case "k", "up":
		moveGanttCursor(&m, rows, -1)
	case "h", "left":
		if m.ganttOffsetDays > 0 {
			m.ganttOffsetDays--
		}
	case "l", "right":
		m.ganttOffsetDays++
	case "g":
		// -1 es el centinela de "no hay tareas". Comparar contra él en vez de
		// contra 0 deja el `>=` como lo que es: el BOUNDARY de `>= 0` es
		// equivalente porque toda cabecera ocupa la fila 0.
		if first, _ := ganttTaskRange(rows); first != ganttNoTasks {
			m.ganttCursor = first
		}
	case "G":
		if _, last := ganttTaskRange(rows); last != ganttNoTasks {
			m.ganttCursor = last
		}
	case "enter":
		// El snap de entrada deja el cursor sobre una fila de tarea siempre, así
		// que la comprobación de `kind` que había aquí no podía ser falsa. Lo que
		// sí hace falta es el rango: sin filas no hay nada que abrir.
		if inRange(m.ganttCursor, len(rows)) {
			t := rows[m.ganttCursor].entry.Task
			m.detailOpen = true
			m.detailTask = &t
			m.detailComments = nil
			m.detailCommentSel = -1
			return m, m.loadCommentsCmd(t.ID)
		}
	}
	return m, nil
}

// renderGantt dibuja el Gantt dentro del alto disponible: una fila por tarea,
// una columna por día, agrupadas por persona.
func (m *Model) renderGantt(maxHeight int) string {
	// Los filtros pueden haber dejado menos filas: reencuadrar el cursor.
	m.snapGanttCursor()

	w := m.width
	innerW := w - 2

	s := m.ganttDisplay()
	rows := ganttRows(s)

	// Reparto de ancho: etiqueta a la izquierda, días a la derecha.
	labelW, dayCols := ganttLabelAndDays(innerW)

	start, _ := model.ParseDate(s.Start)
	offset := m.ganttOffsetDays
	offset = max(offset, 0)

	// Ventana vertical que sigue al cursor.
	visible := maxHeight - listFixedRows - ganttRulerRows
	visible = max(visible, 1)
	// visibleRange ya devuelve la ventana completa cuando todo cabe, así que el
	// `if len(rows) > visible` de antes sólo tenía dos ramas con el mismo
	// resultado: su BOUNDARY era un mutant equivalente.
	vStart, vEnd := visibleRange(m.ganttCursor, len(rows), visible)

	lines := []string{
		m.renderFilterHeader(innerW),
		m.renderGanttRuler(start, offset, labelW, dayCols),
		m.renderGanttAxis(labelW, dayCols, start, offset),
	}

	if len(rows) == 0 {
		lines = append(lines, styleDim.Render("  No active tasks."))
	}

	for i, row := range rows[vStart:vEnd] {
		selected := i+vStart == m.ganttCursor
		lines = append(lines, m.renderGanttRow(row, start, offset, dayCols, labelW, selected))
	}

	content := strings.Join(lines, "\n")
	content = truncateLines(content, innerW)

	borderFg := lipgloss.Color("8")
	return bordered.RenderWithTitlesEx(
		lipgloss.RoundedBorder(),
		borderFg,
		" Gantt ",
		bordered.AlignLeft,
		" "+m.ganttLegend(offset, dayCols)+" ",
		bordered.AlignRight,
		content,
		w,
	)
}

// renderGanttRuler es la línea superior con la etiqueta de cada semana
// ("1SEP") alineada a cada lunes visible.
func (m *Model) renderGanttRuler(start time.Time, offset, labelW, dayCols int) string {
	ruler := make([]rune, labelW+1+dayCols)
	for i := range ruler {
		ruler[i] = ' '
	}
	for col := range dayCols {
		day := start.AddDate(0, 0, offset+col)
		if day.Weekday() != time.Monday {
			continue
		}
		label := model.WeekOfMonthLabel(day)
		at := labelW + 1 + col
		for i, r := range label {
			if at+i < len(ruler) {
				ruler[at+i] = r
			}
		}
	}
	return styleDim.Render(strings.TrimRight(string(ruler), " "))
}

// renderGanttAxis dibuja un "|" en cada lunes y "-" en el resto de días.
func (m *Model) renderGanttAxis(labelW, dayCols int, start time.Time, offset int) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", labelW+1))
	for col := range dayCols {
		if start.AddDate(0, 0, offset+col).Weekday() == time.Monday {
			b.WriteString("|")
		} else {
			b.WriteString("-")
		}
	}
	return styleSep.Render(b.String())
}

// renderGanttRow dibuja una fila: cabecera de persona o barra de tarea.
func (m *Model) renderGanttRow(row ganttRow, start time.Time, offset, dayCols, labelW int, selected bool) string {
	if row.kind == ganttAssigneeRow {
		return styleColumnHeader.Render(cellWidth(row.assignee, labelW+1+dayCols))
	}

	e := row.entry
	d0 := daysBetween(start, e.Start)
	d1 := daysBetween(start, e.End)

	cells := make([]rune, dayCols)
	for col := range dayCols {
		d := offset + col
		day := start.AddDate(0, 0, d)
		switch {
		case d >= d0 && d <= d1:
			cells[col] = '█'
		case model.IsOffDay(m.offdays, row.assignee, day):
			cells[col] = '·'
		default:
			cells[col] = ' '
		}
	}

	mark := ""
	if e.EstimateDefaulted {
		mark = "~"
	}
	prefix := "  "
	if selected {
		prefix = "> "
	}
	label := fmt.Sprintf("%s#%d %s %s%s", prefix, e.Task.ID, e.Task.Title, model.FormatEstimate(e.Estimate), mark)
	line := cellWidth(label, labelW) + " " + string(cells)
	if selected {
		return styleSelected.Render(line)
	}
	return line
}

// ganttLegend describe el rango visible y el horizonte total.
func (m *Model) ganttLegend(offset, dayCols int) string {
	s := m.ganttSchedule()
	// El error de ParseDate no se comprueba: s.Start lo pone ganttSchedule
	// formateando un time.Time con el mismo layout que ParseDate lee, así que el
	// parseo no puede fallar. Antes había un return "Gantt" de reserva que era
	// inalcanzable y que, si alguna vez hubiera podido dispararse, habría
	// escondido un fallo real detrás de una etiqueta sin fechas.
	start, _ := model.ParseDate(s.Start)
	from := start.AddDate(0, 0, offset).Format("2006-01-02")
	to := start.AddDate(0, 0, offset+dayCols-1).Format("2006-01-02")
	return fmt.Sprintf("%s → %s ", from, to)
}

// daysBetween cuenta los días de calendario entre dos fechas YYYY-MM-DD.
func daysBetween(start time.Time, date string) int {
	d, err := model.ParseDate(date)
	if err != nil {
		return -1
	}
	return int(d.Sub(start).Hours() / 24)
}
