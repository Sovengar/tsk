package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

// listFixedRows es el alto de la caja de la lista que no son filas de tareas:
// bordes superior e inferior, barra de filtros, dos separadores y la cabecera
// de columnas.
const listFixedRows = 6

// renderList renderiza la vista List dentro del alto disponible.
func (m *Model) renderList(maxHeight int) string {
	w := m.width
	innerW := w - 2 // ancho interior para el contenido dentro del borde

	sep := styleSep.Render(strings.Repeat("─", innerW-2))

	// Filter bar
	filterBar := m.renderFilterBar()

	// Column headers (el ID no se muestra como columna)
	headers := fmt.Sprintf("%-10s %-12s %-12s %-24s %-40s", "Priority", "Status", "Assignee", "Title", "Description")
	headerLine := styleColumnHeader.Render(headers)

	sep2 := styleSep.Render(strings.Repeat("─", innerW-2))

	tasks := m.filteredTasks()
	pageStart, pageEnd := m.pageBounds()

	// Solo se pinta la página actual. Si la página no entra en el alto
	// disponible, se recorta la ventana manteniendo el cursor visible
	// (fallback para terminales chicas).
	start, end := pageStart, pageEnd
	if visible := maxHeight - listFixedRows; visible > 0 && end-start > visible {
		relStart, relEnd := visibleRange(m.cursor-pageStart, end-pageStart, visible)
		start, end = pageStart+relStart, pageStart+relEnd
	}

	// Tasks
	taskLines := []string{}
	for i := start; i < end; i++ {
		t := tasks[i]
		prio := priorityChar(t.Priority) + " " + model.PriorityShortLabel(t.Priority)
		desc := truncate(singleLine(t.Description), 40)
		title := truncate(singleLine(t.Title), 24)
		line := fmt.Sprintf("%-10s %-12s %-12s %-24s %-40s", prio, t.Status, t.Assignee, title, desc)

		if i == m.cursor {
			line = styleSelected.Render("> " + line)
		} else {
			line = "  " + line
		}
		taskLines = append(taskLines, line)
	}

	if len(taskLines) == 0 {
		taskLines = append(taskLines, styleDim.Render("  No tasks found."))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		filterBar,
		sep,
		headerLine,
		sep2,
		strings.Join(taskLines, "\n"),
	)

	// Ninguna fila debe exceder el ancho interior: si lo hiciera, el borde la
	// re-wrapéaría y la caja crecería más allá del alto calculado, empujando el
	// preview y el KeybindsBar fuera de la pantalla.
	content = truncateLines(content, innerW)

	// Envolver con borde redondeado. La leyenda de paginación va incrustada en
	// el borde inferior, alineada a la derecha.
	var borderFg color.Color = lipgloss.Color("8") // gris por defecto
	return bordered.RenderWithTitlesEx(
		lipgloss.RoundedBorder(),
		borderFg,
		" "+m.currentView.String()+" ",
		bordered.AlignLeft,
		" "+m.pageLegend()+" ",
		bordered.AlignRight,
		content,
		w,
	)
}

func (m *Model) renderFilterBar() string {
	var parts []string

	parts = append(parts, m.renderFilterPart("Project", m.filterProject))
	parts = append(parts, m.renderFilterPart("Status", m.filterStatus))
	parts = append(parts, m.renderFilterPart("Assignee", m.filterAssignee))

	if m.filterPriority >= 0 {
		parts = append(parts, styleFilterDim.Render("Priority: ")+styleStatusKey.Render(fmt.Sprintf("%d", m.filterPriority)))
	} else {
		parts = append(parts, styleFilterDim.Render("Priority: ")+styleStatusKey.Render("all"))
	}

	return "  " + strings.Join(parts, "    ")
}

// renderFilterPart renderiza una parte del filtro: label en gris, valor en azul.
func (m *Model) renderFilterPart(label, value string) string {
	if value == "" {
		value = "all"
	}
	return styleFilterDim.Render(label+": ") + styleStatusKey.Render(value)
}
