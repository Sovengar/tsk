package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type keybind struct {
	key  string
	desc string
}

func (m *Model) globalKeybinds() []keybind {
	return []keybind{
		{"1/2/3", "Dash / List / Kanban"},
		{"Tab", "Next view"},
		{"hjkl", "Arrows (←↓↑→)"},
		{"H", "Toggle hidden (done/cancelled)"},
		{"?", "Toggle this help"},
		{"Esc", "Close / cancel"},
		{"q", "Quit"},
	}
}

func (m *Model) viewKeybinds() []keybind {
	switch m.currentView {
	case viewDashboard:
		return []keybind{
			{"Tab", "Cycle projects"},
			{"i", "Insert new task"},
		}
	case viewList:
		return []keybind{
			{"n/p  N/P", "Page navigation (next/prev, first/last)"},
			{"Enter", "Open detail"},
			{"i", "Insert new task"},
			{"e", "Edit task in $EDITOR"},
			{"Ctrl+p", "Cycle priority (backlog: includes none)"},
			{"s", "Start task (advance status)"},
			{"d", "Mark done"},
			{"x", "Cancel task"},
			{"/", "Open filters"},
		}
	case viewKanban:
		return []keybind{
			{"Tab", "Next column"},
			{"i", "Insert new task"},
			{"e", "Edit task in $EDITOR"},
			{"s", "Advance status (→)"},
			{"S", "Retreat status (←)"},
			{"d", "Mark done"},
			{"x", "Cancel task"},
			{"Enter", "Open detail"},
			{"Ctrl+p", "Cycle priority (backlog: includes none)"},
		}
	}
	return nil
}

// detailKeybinds devuelve los keybinds del modal de detalle de tarea.
func detailKeybinds() []keybind {
	return []keybind{
		{"c", "New comment ($EDITOR)"},
		{"j/k", "Select comment"},
		{"d", "Delete selected comment / Done"},
		{"e", "Edit task in $EDITOR"},
		{"s", "Start task"},
		{"x", "Cancel task"},
		{"Esc", "Deselect comment / close"},
	}
}

func (m *Model) renderHelpModal(content string) string {
	w := m.width

	title := styleTitle.Render("  Keybindings — " + m.currentView.String())

	sep := styleSep.Render(strings.Repeat("─", 40))

	// Global keys
	var globalLines []string
	globalLines = append(globalLines, styleColumnHeader.Render("  Global"))
	for _, kb := range m.globalKeybinds() {
		globalLines = append(globalLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	// View-specific keys
	var viewLines []string
	viewLines = append(viewLines, "")
	viewLines = append(viewLines, styleColumnHeader.Render("  "+m.currentView.String()))
	for _, kb := range m.viewKeybinds() {
		viewLines = append(viewLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	// Detail modal keys
	viewLines = append(viewLines, "")
	viewLines = append(viewLines, styleColumnHeader.Render("  Task detail (modal)"))
	for _, kb := range detailKeybinds() {
		viewLines = append(viewLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	footer := styleDim.Render("  Press ? or Esc to close")

	body := strings.Join(append(globalLines, viewLines...), "\n")

	// Calculate modal dimensions
	modalWidth := 46
	if modalWidth > w-2 {
		modalWidth = w - 2
	}

	modal := lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		sep,
		body,
		sep,
		footer,
	)

	modal = lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder(), true).
		Render(modal)

	// Overlay on content
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	modalLines := strings.Split(modal, "\n")
	modalH := len(modalLines)

	// Center vertically
	startY := (totalLines - modalH) / 2
	if startY < 0 {
		startY = 0
	}

	// Pad background if modal is taller (like dbx does)
	for len(lines) < startY+modalH {
		lines = append(lines, strings.Repeat(" ", w))
	}

	// Center horizontally — account for border (2 chars total)
	totalModalW := modalWidth + 2
	startX := (w - totalModalW) / 2
	if startX < 0 {
		startX = 0
	}

	for i, ml := range modalLines {
		y := startY + i
		lines[y] = OverlayLine(lines[y], ml, startX)
	}

	return strings.Join(lines, "\n")
}
