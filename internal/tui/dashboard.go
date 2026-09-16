package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderDashboard renderiza la vista Dashboard.
func (m *Model) renderDashboard() string {
	w := m.width

	// Header
	header := styleTitle.Render("tsk")
	headerLine := "\n" + header

	// Projects line with selection indicator
	projParts := []string{}
	for i, p := range m.projects {
		count := 0
		for _, t := range m.tasks {
			if t.ProjectName == p.Name && t.IsActive() {
				count++
			}
		}
		if i == m.dashProjectIdx {
			projParts = append(projParts, styleSelected.Render(fmt.Sprintf("[%s(%d)]", p.Name, count)))
		} else {
			projParts = append(projParts, fmt.Sprintf("%s(%d)", p.Name, count))
		}
	}
	projectsLine := "Projects: " + strings.Join(projParts, "  ")

	// Separator
	sep := styleSep.Render(strings.Repeat("─", w-2))

	// Filter by selected project if any
	selectedProject := ""
	if m.dashProjectIdx >= 0 && m.dashProjectIdx < len(m.projects) {
		selectedProject = m.projects[m.dashProjectIdx].Name
	}

	// Stats section
	overviewLines := []string{}
	overviewLines = append(overviewLines, styleColumnHeader.Render("Overview"))
	overviewLines = append(overviewLines, "")

	totalActive := 0
	totalDone := 0
	totalCancelled := 0
	byStatus := map[string]int{}
	for _, t := range m.tasks {
		if selectedProject != "" && t.ProjectName != selectedProject {
			continue
		}
		if t.Status == "cancelled" {
			totalCancelled++
			byStatus["cancelled"]++
		} else if t.Status == "done" {
			totalDone++
			byStatus["done"]++
		} else {
			totalActive++
			byStatus[t.Status]++
		}
	}

	overviewLines = append(overviewLines, fmt.Sprintf("  Total         %d", totalActive+totalDone+totalCancelled))

	// Build status bars from project workflows
	statusOrder := m.mergedWorkflow()
	for _, status := range statusOrder {
		count := byStatus[status]
		if count == 0 {
			continue
		}
		bar := strings.Repeat("░", count)
		label := status
		if len(label) > 14 {
			label = label[:14]
		}
		overviewLines = append(overviewLines, fmt.Sprintf("  %-14s %d  %s", label, count, bar))
	}

	overview := strings.Join(overviewLines, "\n")

	// Team workload
	teamLines := []string{}
	teamLines = append(teamLines, "")
	teamLines = append(teamLines, styleColumnHeader.Render("Team Workload"))
	teamLines = append(teamLines, "")

	assigneeTasks := map[string]int{}
	assigneeActive := map[string]int{}
	for _, t := range m.tasks {
		if selectedProject != "" && t.ProjectName != selectedProject {
			continue
		}
		assigneeTasks[t.Assignee]++
		if t.IsActive() {
			assigneeActive[t.Assignee]++
		}
	}
	for assignee, total := range assigneeTasks {
		active := assigneeActive[assignee]
		teamLines = append(teamLines, fmt.Sprintf("  %-14s %d tasks (%d active)", assignee, total, active))
	}

	team := strings.Join(teamLines, "\n")

	// Active tasks panel
	activeLines := []string{}
	activeLines = append(activeLines, styleColumnHeader.Render("Active"))
	activeLines = append(activeLines, "")

	for _, t := range m.tasks {
		if !t.IsActive() {
			continue
		}
		if selectedProject != "" && t.ProjectName != selectedProject {
			continue
		}
		prio := priorityChar(t.Priority)
		activeLines = append(activeLines, fmt.Sprintf("  %d  %s %-20s %s", t.ID, prio, truncate(t.Title, 20), t.Assignee))
		if len(activeLines) > 15 {
			break
		}
	}

	active := strings.Join(activeLines, "\n")

	// Layout: two columns
	leftContent := lipgloss.JoinVertical(lipgloss.Left, overview, team)
	rightContent := active

	leftW := w/2 - 2
	rightW := w/2 - 2

	left := lipgloss.NewStyle().Width(leftW).Render(leftContent)
	right := lipgloss.NewStyle().Width(rightW).Render(rightContent)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerLine,
		projectsLine,
		sep,
		columns,
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}
