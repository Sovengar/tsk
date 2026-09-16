package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// dashboardChrome es el alto de la caja del dashboard que no son columnas:
// bordes superior e inferior, línea de proyectos y separador.
const dashboardChrome = 4

// dashboardTeamHeader son las líneas que ocupa la cabecera de Team Workload.
const dashboardTeamHeader = 3

// dashboardActiveHeader son las líneas que ocupa la cabecera de Active.
const dashboardActiveHeader = 2

// renderDashboard renderiza la vista Dashboard dentro del alto disponible.
func (m *Model) renderDashboard(maxHeight int) string {
	w := m.width

	// Filas disponibles para las dos columnas.
	colLines := maxHeight - dashboardChrome
	if colLines < 3 {
		colLines = 3
	}

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
	sep := styleSep.Render(strings.Repeat("─", w-4))

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

	// Orden estable + tope de filas: el sobrante de alto se descarta por abajo.
	assignees := make([]string, 0, len(assigneeTasks))
	for assignee := range assigneeTasks {
		assignees = append(assignees, assignee)
	}
	sort.Strings(assignees)

	teamLines := []string{}
	teamLines = append(teamLines, "")
	teamLines = append(teamLines, styleColumnHeader.Render("Team Workload"))
	teamLines = append(teamLines, "")

	teamBudget := colLines - len(overviewLines) - dashboardTeamHeader
	if teamBudget < 0 {
		teamBudget = 0
	}
	for _, assignee := range assignees {
		if len(teamLines)-dashboardTeamHeader >= teamBudget {
			break
		}
		teamLines = append(teamLines, fmt.Sprintf("  %-14s %d tasks (%d active)", assignee, assigneeTasks[assignee], assigneeActive[assignee]))
	}

	team := strings.Join(teamLines, "\n")

	// Active tasks panel
	activeLines := []string{}
	activeLines = append(activeLines, styleColumnHeader.Render("Active"))
	activeLines = append(activeLines, "")

	activeBudget := colLines - dashboardActiveHeader
	if activeBudget < 0 {
		activeBudget = 0
	}
	for _, t := range m.tasks {
		if len(activeLines)-dashboardActiveHeader >= activeBudget {
			break
		}
		if !t.IsActive() {
			continue
		}
		if selectedProject != "" && t.ProjectName != selectedProject {
			continue
		}
		prio := priorityChar(t.Priority)
		activeLines = append(activeLines, fmt.Sprintf("  %d  %s %-20s %s", t.ID, prio, truncate(t.Title, 20), t.Assignee))
	}

	active := strings.Join(activeLines, "\n")

	// Layout: two columns
	innerW := w - 2 // ancho interior para el contenido dentro del borde
	leftW := innerW/2 - 1
	rightW := innerW/2 - 1

	// Recortar cada columna a su ancho antes de renderizarla: si una línea no
	// entra, lipgloss la wrapéaría y la caja crecería más allá del alto
	// calculado, empujando el KeybindsBar fuera de la pantalla.
	leftContent := truncateLines(lipgloss.JoinVertical(lipgloss.Left, overview, team), leftW)
	rightContent := truncateLines(active, rightW)

	left := lipgloss.NewStyle().Width(leftW).Render(leftContent)
	right := lipgloss.NewStyle().Width(rightW).Render(rightContent)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	content := lipgloss.JoinVertical(lipgloss.Left,
		projectsLine,
		sep,
		columns,
	)

	// Envolver con borde redondeado
	var borderFg color.Color = lipgloss.Color("8")
	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		borderFg,
		bordered.AlignLeft,
		" Dashboard ",
		content,
		w,
	)
}
