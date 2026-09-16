package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

const (
	// kanbanGap es la separación horizontal (en columnas) entre columnas del board.
	kanbanGap = 2
	// kanbanMinColWidth es el ancho mínimo (bordes incluidos) de una columna.
	kanbanMinColWidth = 12
	// kanbanBoardChrome es el alto del board que no son tarjetas: bordes
	// superior e inferior del board, su separador y los de cada columna.
	kanbanBoardChrome = 6
	// kanbanCardRows es el alto de una tarjeta más su separación.
	kanbanCardRows = 3
)

// kanbanColumn agrupa las tareas de un estado para renderizar el board.
type kanbanColumn struct {
	status       string
	tasks        []model.Task
	showPriority bool
}

// priorityChar devuelve el caracter de prioridad con color.
func priorityChar(p int) string {
	ch := model.PriorityBar(p)
	switch p {
	case model.PriorityHigh:
		return stylePriorityHigh.Render(ch)
	case model.PriorityMedium:
		return stylePriorityMedium.Render(ch)
	case model.PriorityLow:
		return stylePriorityLow.Render(ch)
	default:
		return stylePriorityNone.Render(ch)
	}
}

// kanbanColumns agrupa las tareas por estado siguiendo el workflow del proyecto
// y agrega una columna final para cancelled cuando corresponde. Es la única
// fuente de verdad del board: la usan el render, la navegación y el preview, así
// el resaltado y la tarea seleccionada nunca se contradicen.
func (m *Model) kanbanColumns() []kanbanColumn {
	workflow := m.mergedWorkflow()
	cols := make([]kanbanColumn, 0, len(workflow)+1)
	for _, status := range workflow {
		cols = append(cols, kanbanColumn{
			status:       status,
			tasks:        m.tasksInColumn(status),
			showPriority: true,
		})
	}
	if cancelled := m.tasksInColumn(model.CancelledStatus); len(cancelled) > 0 {
		cols = append(cols, kanbanColumn{status: model.CancelledStatus, tasks: cancelled})
	}
	return cols
}

// clampKanbanCursor mantiene el cursor dentro del board actual. Las columnas
// cambian al togglear tareas ocultas (H), así que el índice puede quedar fuera.
func (m *Model) clampKanbanCursor() {
	cols := m.kanbanColumns()
	if len(cols) == 0 {
		m.kanbanCol, m.kanbanRow = 0, 0
		return
	}
	if m.kanbanCol < 0 {
		m.kanbanCol = 0
	}
	if m.kanbanCol >= len(cols) {
		m.kanbanCol = len(cols) - 1
	}
	if n := len(cols[m.kanbanCol].tasks); n == 0 {
		m.kanbanRow = 0
	} else if m.kanbanRow >= n {
		m.kanbanRow = n - 1
	}
}

// renderKanban renderiza la vista Kanban dentro del alto disponible.
func (m *Model) renderKanban(maxHeight int) string {
	w := m.width

	sep := styleSep.Render(strings.Repeat("─", w-4))

	cols := m.kanbanColumns()

	// Solo se muestran las tarjetas que entran en el alto disponible, con la
	// ventana desplazada en la columna activa para que el cursor sea visible.
	colContentHeight := maxHeight - kanbanBoardChrome
	if colContentHeight < 2 {
		colContentHeight = 2
	}
	maxCards := (colContentHeight + 1) / kanbanCardRows
	if maxCards < 1 {
		maxCards = 1
	}

	windows := make([]columnWindow, len(cols))
	for i, col := range cols {
		if i == m.kanbanCol {
			start, end := visibleRange(m.kanbanRow, len(col.tasks), maxCards)
			windows[i] = columnWindow{start: start, end: end}
			continue
		}
		end := len(col.tasks)
		if end > maxCards {
			end = maxCards
		}
		windows[i] = columnWindow{end: end}
	}

	// Calcular el ancho de cada columna: nunca menos que su header, y el resto
	// del ancho disponible repartido en partes iguales. w-2 es el ancho interno
	// del borde que envuelve la vista.
	headers := make([]string, len(cols))
	minWidths := make([]int, len(cols))
	for i, col := range cols {
		total := len(col.tasks)
		shown := windows[i].end - windows[i].start
		if shown < total {
			headers[i] = fmt.Sprintf("─ %s (%d/%d) ", col.status, shown, total)
		} else {
			headers[i] = fmt.Sprintf("─ %s (%d) ", col.status, total)
		}
		minWidths[i] = lipgloss.Width(headers[i]) + 2 // + bordes
		if minWidths[i] < kanbanMinColWidth {
			minWidths[i] = kanbanMinColWidth
		}
	}
	widths := kanbanColumnWidths(minWidths, w-2)

	var views []string
	for i, col := range cols {
		selected := i == m.kanbanCol
		view := m.renderKanbanColumn(col, widths[i], headers[i], selected, windows[i])
		if i < len(cols)-1 {
			view = lipgloss.NewStyle().MarginRight(kanbanGap).Render(view)
		}
		views = append(views, view)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, views...)

	content := lipgloss.JoinVertical(lipgloss.Left,
		sep,
		board,
	)

	// Envolver con borde redondeado
	var borderFg color.Color = lipgloss.Color("8")
	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		borderFg,
		bordered.AlignLeft,
		" Kanban ",
		content,
		w,
	)
}

// columnWindow es el rango [start, end) de tarjetas visibles de una columna.
type columnWindow struct {
	start, end int
}

// kanbanColumnWidths reparte el ancho disponible entre las columnas partiendo
// de su ancho mínimo. El sobrante se reparte en partes iguales para aprovechar
// todo el ancho y evitar espacio muerto a la derecha.
func kanbanColumnWidths(minWidths []int, avail int) []int {
	n := len(minWidths)
	if n == 0 {
		return nil
	}

	widths := make([]int, n)
	total := 0
	for i, mw := range minWidths {
		widths[i] = mw
		total += mw
	}

	free := avail - total - kanbanGap*(n-1)
	if free <= 0 {
		return widths
	}

	each := free / n
	for i := range widths {
		widths[i] += each
	}
	for i := 0; i < free%n; i++ {
		widths[i]++
	}
	return widths
}

// renderKanbanColumn renderiza una columna con ancho fijo (bordes incluidos),
// mostrando solo las tarjetas del rango [win.start, win.end).
func (m *Model) renderKanbanColumn(col kanbanColumn, width int, headerText string, selected bool, win columnWindow) string {
	var cards []string
	for j := win.start; j < win.end; j++ {
		t := col.tasks[j]
		var card string
		if col.showPriority {
			card = fmt.Sprintf("  %s %s\n     %s", priorityChar(t.Priority), t.Title, t.Assignee)
		} else {
			card = fmt.Sprintf("  %s\n     %s", t.Title, t.Assignee)
		}
		// Recortar a width-4 (bordes + prefijo) para que ninguna línea de la
		// tarjeta exceda el ancho interior: si lo hiciera, lipgloss la
		// wrapéaría y la columna crecería más allá del alto calculado.
		card = truncateLines(card, width-4)
		if selected && j == m.kanbanRow {
			card = styleSelected.Render("> " + card)
		} else {
			card = "  " + card
		}
		cards = append(cards, card)
	}

	content := strings.Join(cards, "\n\n")
	if content == "" {
		content = styleDim.Render("  (empty)")
	}

	border := lipgloss.RoundedBorder()
	header := styleColumnHeader.Render(headerText)
	if selected {
		border = lipgloss.DoubleBorder()
		header = styleSelected.Render(headerText)
	}

	colContent := lipgloss.JoinVertical(lipgloss.Left, header, content)
	return lipgloss.NewStyle().
		Width(width).
		Border(border, true).
		Render(colContent)
}
