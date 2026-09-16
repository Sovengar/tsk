package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
)

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

// renderKanban renderiza la vista Kanban.
func (m *Model) renderKanban() string {
	w := m.width

	// Header
	title := styleTitle.Render("tsk") + " — Kanban"
	projectNames := []string{}
	for _, p := range m.projects {
		projectNames = append(projectNames, p.Name)
	}
	if len(projectNames) > 0 {
		title += " — " + strings.Join(projectNames, " + ")
	}

	sep := styleSep.Render(strings.Repeat("─", w-2))

	// Build columns from merged workflow
	workflow := m.mergedWorkflow()
	columns := make([][]string, len(workflow))
	columnTasks := make([][]model.Task, len(workflow))
	for i := range columns {
		columns[i] = []string{}
		columnTasks[i] = []model.Task{}
	}
	cancelledCol := []string{}
	cancelledTasks := []model.Task{}

	// Group tasks by status
	for _, t := range m.tasks {
		if t.Status == model.CancelledStatus {
			cancelledCol = append(cancelledCol, fmt.Sprintf("  %d  %s\n     %s", t.ID, t.Title, t.Assignee))
			cancelledTasks = append(cancelledTasks, t)
			continue
		}
		for i, status := range workflow {
			if t.Status == status {
				prio := priorityChar(t.Priority)
				card := fmt.Sprintf("  %d  %s %s\n     %s", t.ID, prio, t.Title, t.Assignee)
				columns[i] = append(columns[i], card)
				columnTasks[i] = append(columnTasks[i], t)
				break
			}
		}
	}

	// Render columns
	colWidth := (w - 4) / len(workflow)
	if colWidth < 15 {
		colWidth = 15
	}

	var colViews []string
	for i, status := range workflow {
		// Highlight selected column header
		headerText := fmt.Sprintf("─ %s (%d) ", status, len(columns[i]))
		if i == m.kanbanCol {
			header := styleSelected.Render(headerText)
			content := strings.Join(columns[i], "\n\n")
			if content == "" {
				content = styleDim.Render("  (empty)")
			} else {
				// Mark selected task
				var taskLines []string
				for j, line := range columns[i] {
					if j == m.kanbanRow && i == m.kanbanCol {
						taskLines = append(taskLines, styleSelected.Render("> "+line))
					} else {
						taskLines = append(taskLines, "  "+line)
					}
				}
				content = strings.Join(taskLines, "\n\n")
			}
			colContent := lipgloss.JoinVertical(lipgloss.Left, header, content)
			colView := lipgloss.NewStyle().
				Width(colWidth).
				Border(lipgloss.DoubleBorder(), true).
				Render(colContent)
			colViews = append(colViews, colView)
		} else {
			header := styleColumnHeader.Render(headerText)
			content := strings.Join(columns[i], "\n\n")
			if content == "" {
				content = styleDim.Render("  (empty)")
			}
			colContent := lipgloss.JoinVertical(lipgloss.Left, header, content)
			colView := lipgloss.NewStyle().
				Width(colWidth).
				Border(lipgloss.RoundedBorder(), true).
				Render(colContent)
			colViews = append(colViews, colView)
		}
	}

	// Cancelled column (if any tasks)
	if len(cancelledCol) > 0 {
		headerText := fmt.Sprintf("─ cancelled (%d) ", len(cancelledCol))
		cancelledW := colWidth
		if cancelledW > 20 {
			cancelledW = 20
		}
		// Check if cancelled column is selected (shouldn't happen, but handle it)
		header := styleColumnHeader.Render(headerText)
		content := strings.Join(cancelledCol, "\n\n")
		colContent := lipgloss.JoinVertical(lipgloss.Left, header, content)
		colView := lipgloss.NewStyle().
			Width(cancelledW).
			Border(lipgloss.RoundedBorder(), true).
			Render(colContent)
		colViews = append(colViews, colView)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, colViews...)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		sep,
		board,
	)
}
