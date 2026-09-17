package tui

import (
	"fmt"
	"image/color"
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

// ganttSchedule proyecta la cola de cada persona a partir de las tareas y
// off-days cargados. El punto de partida es hoy.
func (m *Model) ganttSchedule() *model.Schedule {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	estimate := m.config.DefaultEstimateDays
	if estimate <= 0 {
		estimate = 1
	}
	return model.BuildSchedule(m.tasks, m.offdays, start, estimate)
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

// ganttRows devuelve las filas del schedule actual.
func (m *Model) ganttRows() []ganttRow {
	return ganttRows(m.ganttSchedule())
}

// clampGanttCursor mantiene el cursor dentro de las filas actuales.
func (m *Model) clampGanttCursor() {
	n := len(m.ganttRows())
	if n == 0 {
		m.ganttCursor = 0
		return
	}
	if m.ganttCursor < 0 {
		m.ganttCursor = 0
	}
	if m.ganttCursor >= n {
		m.ganttCursor = n - 1
	}
}

// handleGanttKey navega el Gantt: j/k sobre filas, h/l desplaza la ventana de
// días, g/G va al inicio/fin, Enter abre el detalle de la tarea.
func (m Model) handleGanttKey(key string) (tea.Model, tea.Cmd) {
	m.clampGanttCursor()
	rows := m.ganttRows()

	switch key {
	case "j", "down":
		if m.ganttCursor < len(rows)-1 {
			m.ganttCursor++
		}
	case "k", "up":
		if m.ganttCursor > 0 {
			m.ganttCursor--
		}
	case "h", "left":
		if m.ganttOffsetDays > 0 {
			m.ganttOffsetDays--
		}
	case "l", "right":
		m.ganttOffsetDays++
	case "g":
		m.ganttCursor = 0
	case "G":
		if len(rows) > 0 {
			m.ganttCursor = len(rows) - 1
		}
	case "enter":
		if m.ganttCursor >= 0 && m.ganttCursor < len(rows) && rows[m.ganttCursor].kind == ganttTaskRow {
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
	w := m.width
	innerW := w - 2

	s := m.ganttSchedule()
	rows := ganttRows(s)

	// Reparto de ancho: etiqueta a la izquierda, días a la derecha.
	labelW := 30
	if innerW < 70 {
		labelW = innerW / 3
	}
	if labelW < 14 {
		labelW = 14
	}
	dayCols := innerW - labelW - 1
	if dayCols < 7 {
		dayCols = 7
	}

	start, _ := model.ParseDate(s.Start)
	offset := m.ganttOffsetDays
	if offset < 0 {
		offset = 0
	}

	// Ventana vertical que sigue al cursor.
	visible := maxHeight - listFixedRows - ganttRulerRows
	if visible < 1 {
		visible = 1
	}
	vStart, vEnd := 0, len(rows)
	if len(rows) > visible {
		vStart, vEnd = visibleRange(m.ganttCursor, len(rows), visible)
	}

	lines := []string{
		m.renderGanttRuler(start, offset, labelW, dayCols),
		m.renderGanttAxis(labelW, dayCols, start, offset),
	}

	if len(rows) == 0 {
		lines = append(lines, styleDim.Render("  No active tasks."))
	}

	for i := vStart; i < vEnd; i++ {
		row := rows[i]
		selected := i == m.ganttCursor
		lines = append(lines, m.renderGanttRow(row, start, offset, dayCols, labelW, selected))
	}

	content := strings.Join(lines, "\n")
	content = truncateLines(content, innerW)

	var borderFg color.Color = lipgloss.Color("8")
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
	for col := 0; col < dayCols; col++ {
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
	for col := 0; col < dayCols; col++ {
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
	for col := 0; col < dayCols; col++ {
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
	label := fmt.Sprintf("  #%d %s %s%s", e.Task.ID, e.Task.Title, model.FormatEstimate(e.Estimate), mark)
	line := cellWidth(label, labelW) + " " + string(cells)
	if selected {
		return styleSelected.Render(line)
	}
	return line
}

// ganttLegend describe el rango visible y el horizonte total.
func (m *Model) ganttLegend(offset, dayCols int) string {
	s := m.ganttSchedule()
	start, err := model.ParseDate(s.Start)
	if err != nil {
		return "Gantt"
	}
	from := start.AddDate(0, 0, offset).Format("2006-01-02")
	to := start.AddDate(0, 0, offset+dayCols-1).Format("2006-01-02")
	return fmt.Sprintf("%s → %s · h/l scroll ", from, to)
}

// daysBetween cuenta los días de calendario entre dos fechas YYYY-MM-DD.
func daysBetween(start time.Time, date string) int {
	d, err := model.ParseDate(date)
	if err != nil {
		return -1
	}
	return int(d.Sub(start).Hours() / 24)
}
