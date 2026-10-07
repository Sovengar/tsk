package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

// listFixedRows is the height of the list box that is not task rows:
// top and bottom borders, filter bar, two separators and the
// column header.
const listFixedRows = 6

// filterHeaderRows is the height of the filter header shared by the
// views: the filter bar and its separator.
const filterHeaderRows = 2

// listColumn is a column of the List table. The widths are display
// (screen columns), not bytes.
type listColumn struct {
	header string
	width  int
}

// listColumns defines the columns in order. When the width is not enough for
// all of them they are dropped from the end, so Description is the first to
// fall and Tags the second. Tags comes before Description so that "blocked"
// is visible on common-width terminals.
var listColumns = []listColumn{
	{"Priority", 10},
	{"Status", 12},
	{"Assignee", 12},
	{"Title", 24},
	{"Tags", 10},
	{"Description", 40},
}

// visibleListColumns returns how many columns fit in the available width.
// It always leaves at least one so the table is never left empty.
func visibleListColumns(avail int) int {
	used := 0
	count := 0
	for i, col := range listColumns {
		need := col.width
		if i > 0 {
			need++ // separator space
		}
		if used+need > avail {
			break
		}
		used += need
		count++
	}
	count = max(count, 1)
	return count
}

// formatListRow aligns the cells of a row to the width of each column.
func formatListRow(cells []string, count int) string {
	parts := make([]string, count)
	for i := range count {
		parts[i] = cellWidth(cells[i], listColumns[i].width)
	}
	return strings.Join(parts, " ")
}

// renderList renders the List view within the available height.
func (m *Model) renderList(maxHeight int) string {
	w := m.width
	innerW := w - 2 // interior width for the content inside the border

	sep := styleSep.Render(strings.Repeat("─", separatorWidth(innerW)))

	// Columns that fit in the available width. If there is not enough for all,
	// the last ones are dropped (Description first) instead of being cut in half.
	cols := visibleListColumns(listContentWidth(innerW))

	headerCells := make([]string, len(listColumns))
	for i, col := range listColumns {
		headerCells[i] = col.header
	}
	headerLine := styleColumnHeader.Render("  " + formatListRow(headerCells, cols))

	tasks := m.filteredTasks()
	pageStart, pageEnd := m.pageBounds()

	// Only the current page is painted, trimmed to the available height if it
	// does not fit whole. The arithmetic lives in listWindowForHeight.
	start, end := listWindowForHeight(pageStart, pageEnd, m.cursor, maxHeight)

	// Tasks
	taskLines := []string{}
	for i, t := range tasks[start:end] {
		i += start
		prio := priorityChar(t.Priority) + " " + model.PriorityShortLabel(t.Priority)
		cells := []string{
			prio,
			t.Status,
			t.Assignee,
			singleLine(t.Title),
			singleLine(strings.Join(t.Tags, ",")),
			singleLine(t.Description),
		}
		line := formatListRow(cells, cols)

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
		m.renderFilterHeader(innerW),
		headerLine,
		sep,
		strings.Join(taskLines, "\n"),
	)

	// No row may exceed the inner width: if it did, the border would
	// re-wrap it and the box would grow beyond the calculated height, pushing
	// the preview and the KeybindsBar off the screen.
	content = truncateLines(content, innerW)

	// Wrap with a rounded border. The pagination caption is embedded in
	// the bottom border, aligned to the right.
	borderFg := lipgloss.Color("8") // gray by default
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

// renderFilterBar builds the bar with the active filters: label in gray,
// value in blue, and "all" when there is no filter.
func (m *Model) renderFilterBar() string {
	var parts []string

	parts = append(parts, m.renderFilterPart("Project", m.filterProject))
	// In Gantt the status is always "active" (done/cancelled stay out of the
	// projection), so showing it would be noise: it is omitted from the header.
	if m.currentView != viewGantt {
		parts = append(parts, m.renderFilterPart("Status", m.filterStatus))
	}
	parts = append(parts, m.renderFilterPart("Assignee", m.filterAssignee))
	parts = append(parts, m.renderFilterPart("Tag", m.filterTag))

	if m.filterPriority >= 0 {
		parts = append(parts, styleFilterDim.Render("Priority: ")+styleStatusKey.Render(fmt.Sprintf("%d", m.filterPriority)))
	} else {
		parts = append(parts, styleFilterDim.Render("Priority: ")+styleStatusKey.Render("all"))
	}

	return "  " + strings.Join(parts, "    ")
}

// renderFilterHeader draws the filter bar followed by a separator, at the
// inner width of the box. It is the common header of List, Kanban and Gantt.
func (m *Model) renderFilterHeader(innerW int) string {
	sep := styleSep.Render(strings.Repeat("─", separatorWidth(innerW)))
	return lipgloss.JoinVertical(lipgloss.Left, m.renderFilterBar(), sep)
}

// renderFilterPart renders one part of the filter: label in gray, value in blue.
func (m *Model) renderFilterPart(label, value string) string {
	if value == "" {
		value = "all"
	}
	return styleFilterDim.Render(label+": ") + styleStatusKey.Render(value)
}
