package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// dashboardChrome is the height of the dashboard box that is not columns:
// top and bottom borders, projects line and separator.
const dashboardChrome = 4

// dashboardTeamHeader is the number of lines the Team Workload header takes.
const dashboardTeamHeader = 3

// dashStatusLabelWidth is how much of a status fits in the Overview column.
// The format's "%-14s" reserves the slot; truncating here keeps a long name
// from overflowing the bar that goes behind it.
const dashStatusLabelWidth = 14

// dashboardActiveHeader is the number of lines the Active header takes.
const dashboardActiveHeader = 2

// renderDashboard renders the Dashboard view within the available height.
func (m *Model) renderDashboard(maxHeight int) string {
	w := m.width

	// Rows available for the two columns.
	colLines := maxHeight - dashboardChrome
	colLines = max(colLines, 3)

	// Projects line with selection indicator
	list := m.dashProjectList()
	projParts := []string{}
	for i, p := range list {
		count := dashProjectTasks(m.tasks, p.Name)
		if i == m.dashProjectIdx {
			projParts = append(projParts, styleSelected.Render(fmt.Sprintf("[%s(%d)]", p.Name, count)))
		} else {
			projParts = append(projParts, fmt.Sprintf("%s(%d)", p.Name, count))
		}
	}
	label := "Projects: "
	if m.showArchived {
		label = "Archived: "
	}
	projectsLine := label
	if len(projParts) == 0 {
		projectsLine += styleDim.Render("(none)")
	} else {
		projectsLine += strings.Join(projParts, "  ")
	}

	// Separator
	sep := styleSep.Render(strings.Repeat("─", w-4))

	// Filter by selected project if any
	selectedProject := ""
	if p := m.selectedDashProject(); p != nil {
		selectedProject = p.Name
	}

	// Stats section
	overviewLines := []string{}
	overviewLines = append(overviewLines, styleColumnHeader.Render("Overview"))
	overviewLines = append(overviewLines, "")

	totalActive, totalDone, totalCancelled, byStatus := dashStatusCounts(m.tasks, selectedProject)

	overviewLines = append(overviewLines, fmt.Sprintf("  Total         %d", totalActive+totalDone+totalCancelled))

	// Build status bars from project workflows
	statusOrder := m.mergedWorkflow()
	for _, status := range statusOrder {
		// The count comes out of a map of counters, so it cannot be
		// negative: the comparison against zero has no other possible side.
		count := byStatus[status]
		if count == 0 {
			continue
		}
		bar := strings.Repeat("░", count)
		// min instead of comparing lengths: at exactly 14 the truncation does
		// nothing, so the comparison was another equivalent mutant.
		label := status[:min(len(status), dashStatusLabelWidth)]
		overviewLines = append(overviewLines, fmt.Sprintf("  %-14s %d  %s", label, count, bar))
	}

	overview := strings.Join(overviewLines, "\n")

	// Team workload
	assigneeTasks, assigneeActive := dashAssigneeCounts(m.tasks, selectedProject)

	// Stable order + row cap: the leftover height is discarded at the bottom.
	assignees := make([]string, 0, len(assigneeTasks))
	for assignee := range assigneeTasks {
		assignees = append(assignees, assignee)
	}
	sort.Strings(assignees)

	teamLines := []string{}
	teamLines = append(teamLines, "")
	teamLines = append(teamLines, styleColumnHeader.Render("Team Workload"))
	teamLines = append(teamLines, "")

	teamBudget := dashRowsAvailable(colLines, len(overviewLines), dashboardTeamHeader)
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

	activeBudget := dashRowsAvailable(colLines, 0, dashboardActiveHeader)
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
	innerW := w - 2 // interior width for the content inside the border
	leftW, rightW := dashColumnWidths(innerW)

	// Truncate each column to its width before rendering it: if a line does
	// not fit, lipgloss would wrap it and the box would grow beyond the
	// calculated height, pushing the KeybindsBar off the screen.
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

	// Wrap with a rounded border
	borderFg := lipgloss.Color("8")
	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		borderFg,
		bordered.AlignLeft,
		" Dashboard ",
		content,
		w,
	)
}
