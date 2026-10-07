package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

const (
	// kanbanGap is the horizontal separation (in columns) between board columns.
	kanbanGap = 2
	// kanbanMinColWidth is the minimum width (borders included) of a column.
	kanbanMinColWidth = 12
	// borderWidths are the two columns each column card takes in borders, on
	// top of its content.
	borderWidths = 2
	// cardMargin are the columns the card leaves free inside its column:
	// two from the column borders and two from the card's prefix ("  ● "
	// or "> ● ").
	cardMargin = 4
	// kanbanBoardChrome is the board height that is not cards: top and bottom
	// borders of the board, its separator and those of each column.
	kanbanBoardChrome = 6
	// kanbanCardRows is the height of one card plus its separation.
	kanbanCardRows = 3
)

// kanbanColumn groups a status's tasks to render the board.
type kanbanColumn struct {
	status       string
	tasks        []model.Task
	showPriority bool
}

// priorityChar returns the priority character with color.
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

// kanbanColumns groups tasks by status following the project's workflow and
// adds a final column for cancelled when appropriate. It is the only source
// of truth of the board: the render, the navigation and the preview use it,
// so the highlight and the selected task never contradict each other.
func (m *Model) kanbanColumns() []kanbanColumn {
	workflow := m.kanbanWorkflow()
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

// clampKanbanCursor keeps the cursor inside the current board. The columns
// change when filtering by status (e.g. "all active" empties done/cancelled),
// so the index may end up out.
func (m *Model) clampKanbanCursor() {
	cols := m.kanbanColumns()
	colLens := make([]int, len(cols))
	for i, col := range cols {
		colLens[i] = len(col.tasks)
	}
	m.kanbanCol, m.kanbanRow = clampKanban(m.kanbanCol, m.kanbanRow, colLens)
}

// renderKanban renders the Kanban view within the available height.
func (m *Model) renderKanban(maxHeight int) string {
	// The filters may have left fewer cards: reframe the cursor.
	m.clampKanbanCursor()

	w := m.width

	cols := m.kanbanColumns()

	// Only the cards that fit in the available height are shown, with the
	// window scrolled in the active column so the cursor is visible.
	maxCards := kanbanMaxCards(maxHeight)

	windows := make([]columnWindow, len(cols))
	for i, col := range cols {
		if i == m.kanbanCol {
			start, end := visibleRange(m.kanbanRow, len(col.tasks), maxCards)
			windows[i] = columnWindow{start: start, end: end}
			continue
		}
		end := len(col.tasks)
		end = min(end, maxCards)
		windows[i] = columnWindow{end: end}
	}

	// Calculate each column's width: never less than its header, and the rest
	// of the available width split in equal parts. w-2 is the inner width of
	// the border wrapping the view.
	headers := make([]string, len(cols))
	minWidths := make([]int, len(cols))
	for i, col := range cols {
		total := len(col.tasks)
		shown := windows[i].end - windows[i].start
		headers[i] = kanbanHeader(col.status, shown, total)
		// The column's minimum is its header's plus the borders, and never
		// less than the absolute minimum.
		//
		// The sum goes inside minColumnWidth and not inline because the
		// raw `+ borderWidths` had an edge no test could tell apart: a column's
		// two borders are 2 columns no matter what, and changing the sum for a
		// subtraction gave a smaller minimum than the floor of
		// kanbanMinColWidth in every case of the fixture, so the split of the
		// remainder always hid it.
		minWidths[i] = minColumnWidth(lipgloss.Width(headers[i]))
	}
	widths := kanbanColumnWidths(minWidths, w-2)

	// The separation goes BEFORE every column except the first, instead of
	// AFTER every column except the last. Both forms give the same result --
	// n-1 separators between n columns -- but the "before" one has a condition
	// with a value that is reached and told apart: with the "after" one, the
	// `i < len(cols)-1` only told itself apart from `i < len(cols)` when the
	// last column added an invisible margin, because the box truncated it anyway.
	var views []string
	for i, col := range cols {
		selected := i == m.kanbanCol
		view := m.renderKanbanColumn(col, widths[i], headers[i], selected, windows[i])
		if i > 0 {
			view = lipgloss.NewStyle().MarginLeft(kanbanGap).Render(view)
		}
		views = append(views, view)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, views...)

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderFilterHeader(w-2),
		board,
	)

	// No row may exceed the inner width: if it did, the border would
	// re-wrap it and the board would grow beyond the calculated height.
	content = truncateLines(content, w-2)

	// Wrap with a rounded border
	borderFg := lipgloss.Color("8")
	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		borderFg,
		bordered.AlignLeft,
		" Kanban ",
		content,
		w,
	)
}

// columnWindow is the range [start, end) of visible cards of a column.
type columnWindow struct {
	start, end int
}

// kanbanColumnWidths splits the available width among the columns starting
// from their minimum width. The remainder is split in equal parts to use up
// all the width and avoid dead space on the right.
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

	// free is the remainder after giving each column its minimum and paying
	// the separations. If there is no remainder (or there is debt), it keeps
	// the minimums: nobody grows and nobody shrinks.
	// The split of the remainder is a min() and not an early return. With the
	// `if free <= 0`, the `<= 0` was equivalent to its `> 0` -- with free == 0
	// `free / n` gave 0 and the split loop did nothing -- so the condition
	// only told itself apart in the case where both branches give the same.
	//
	// With max, the "no remainder" case is a 0 that the split already knows
	// how to handle, and there is no branch to check.
	free := max(avail-total-kanbanGap*(n-1), 0)

	each := free / n
	for i := range widths {
		widths[i] += each
	}
	for i := range free % n {
		widths[i]++
	}
	return widths
}

// minColumnWidth is the minimum width a column needs with a header of
// `header` columns: the header, the card's two borders and the absolute
// minimum.
func minColumnWidth(header int) int {
	return max(header+borderWidths, kanbanMinColWidth)
}

// withTags adds a task's tags to the assignee line, separated by two spaces.
// With no tags the line stays as it was: with an empty list the join gives
// "" and pasting it anyway added two invisible spaces, because the box pads
// on the right.
func withTags(assignee string, tags []string) string {
	if len(tags) == 0 {
		return assignee
	}
	return assignee + "  " + styleDim.Render(strings.Join(tags, ","))
}

// renderKanbanColumn renders a column with a fixed width (borders included),
// showing only the cards of the range [win.start, win.end).
func (m *Model) renderKanbanColumn(col kanbanColumn, width int, headerText string, selected bool, win columnWindow) string {
	var cards []string
	// range over the slice instead of an index advanced by hand: an
	// inverted `j--` would leave the loop spinning forever and the mutant
	// would be reported as TIMED OUT instead of dead, which is the worst signal for a gate.
	for j, t := range col.tasks[win.start:win.end] {
		j += win.start
		assigneeLine := t.Assignee
		// The separator goes INSIDE the tags line instead of around it. With an
		// empty list, join gives "" and adding it anyway put two extra spaces
		// that the box did not let you see because it pads on the right: that
		// is why the condition told itself apart from `>= 0`. Now the line is
		// built with a helper that knows what to do with an empty list,
		// without the decision being hidden in a "> 0".
		assigneeLine = withTags(assigneeLine, t.Tags)
		var card string
		if col.showPriority {
			card = fmt.Sprintf("  %s %s\n     %s", priorityChar(t.Priority), t.Title, assigneeLine)
		} else {
			card = fmt.Sprintf("  %s\n     %s", t.Title, assigneeLine)
		}
		// Truncate to the useful width (borders + prefix) so that no line of
		// the card exceeds the inner width: if it did, lipgloss would wrap it
		// and the column would grow beyond the calculated height.
		card = truncateLines(card, width-cardMargin)
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
