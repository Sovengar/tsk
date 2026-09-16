package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"taskd/internal/model"
)

// renderList renderiza la vista List.
func (m *Model) renderList() string {
	w := m.width

	// Header
	title := styleTitle.Render("tsk") + " — List"
	projectNames := []string{}
	for _, p := range m.projects {
		projectNames = append(projectNames, p.Name)
	}
	if len(projectNames) > 0 {
		title += " — " + strings.Join(projectNames, " + ")
	}

	sep := styleSep.Render(strings.Repeat("─", w-2))

	// Filter bar
	filterBar := m.renderFilterBar()

	// Column headers
	headers := fmt.Sprintf("%-6s %-10s %-12s %-12s %-24s %-40s", "ID", "Priority", "Status", "Assignee", "Title", "Description")
	headerLine := styleColumnHeader.Render(headers)

	sep2 := styleSep.Render(strings.Repeat("─", w-2))

	// Tasks
	taskLines := []string{}
	for i, t := range m.filteredTasks() {
		prio := priorityChar(t.Priority) + " " + model.PriorityLabel(t.Priority)
		desc := t.Description
		if len(desc) > 40 {
			desc = desc[:40]
		}
		title := t.Title
		if len(title) > 24 {
			title = title[:24]
		}
		line := fmt.Sprintf("%-6d %-10s %-12s %-12s %-24s %-40s", t.ID, prio, t.Status, t.Assignee, title, desc)

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

	// Status line (just below filters)
	total := len(m.filteredTasks())
	selLine := ""
	if total > 0 && m.cursor >= 0 && m.cursor < total {
		t := m.filteredTasks()[m.cursor]
		selLine = fmt.Sprintf("Selected: #%d — %s", t.ID, t.Title)
	}
	statusLine := styleDim.Render(fmt.Sprintf("  Total: %d tasks    %s", total, selLine))

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		filterBar,
		statusLine,
		sep,
		headerLine,
		sep2,
		strings.Join(taskLines, "\n"),
	)
}

func (m *Model) renderFilterBar() string {
	parts := []string{}
	if m.filterProject != "" {
		parts = append(parts, "Project: "+m.filterProject)
	} else {
		parts = append(parts, "Project: all")
	}
	if m.filterStatus != "" {
		parts = append(parts, "Status: "+m.filterStatus)
	} else {
		parts = append(parts, "Status: all")
	}
	if m.filterAssignee != "" {
		parts = append(parts, "Assignee: "+m.filterAssignee)
	} else {
		parts = append(parts, "Assignee: all")
	}
	if m.filterPriority >= 0 {
		parts = append(parts, fmt.Sprintf("Priority: %d", m.filterPriority))
	} else {
		parts = append(parts, "Priority: all")
	}
	return styleFilterDim.Render("  " + strings.Join(parts, "    "))
}
