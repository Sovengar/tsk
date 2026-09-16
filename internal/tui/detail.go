package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
)

// renderDetail renderiza el modal de detalle de tarea.
func (m *Model) renderDetail(t *model.Task) string {
	w := m.width
	h := m.height

	// Priority with colored character
	prio := priorityChar(t.Priority) + " " + model.PriorityLabel(t.Priority)

	// Title line
	title := fmt.Sprintf("#%d — %s                             %s", t.ID, t.Title, prio)

	// Separator
	sep := styleSep.Render(strings.Repeat("─", w-4))

	// Metadata
	lines := []string{}
	lines = append(lines, fmt.Sprintf("  Status:     %-20s Project:   %s", t.Status, t.ProjectName))
	lines = append(lines, fmt.Sprintf("  Assignee:   %-20s Created:   %s", t.Assignee, formatTime(t.CreatedAt)))
	lines = append(lines, fmt.Sprintf("  Updated:    %-20s Completed: %s", formatTime(t.UpdatedAt), formatCompleted(t.CompletedAt)))

	// Description
	desc := "  (no description)"
	if t.Description != "" {
		desc = t.Description
	}
	descLines := strings.Split(desc, "\n")
	descHeight := h - 12
	if descHeight < 3 {
		descHeight = 3
	}
	if len(descLines) > descHeight {
		descLines = descLines[:descHeight]
	}

	// Actions
	actions := styleHelp.Render("  Actions: [e] Edit  [s] Start  [d] Done  [x] Cancel  Esc close")

	// Build modal
	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		styleTitle.Render("  "+title),
		sep,
		strings.Join(lines, "\n"),
		sep,
		"  Description:",
		strings.Join(descLines, "\n"),
		sep,
		actions,
	)

	// Center vertically
	totalLines := strings.Count(content, "\n") + 1
	if totalLines < h {
		topPad := (h - totalLines) / 2
		content = strings.Repeat("\n", topPad) + content
	}

	return content
}

func formatTime(s string) string {
	if s == "" {
		return "—"
	}
	// Simple truncation to date+time
	if len(s) >= 19 {
		return s[:19]
	}
	return s
}

func formatCompleted(s string) string {
	if s == "" {
		return "—"
	}
	return formatTime(s)
}
