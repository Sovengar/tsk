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
		{"1", "Dashboard"},
		{"2", "List"},
		{"3", "Kanban"},
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
			{"Tab/j/k", "Cycle projects"},
			{"n", "New task"},
		}
	case viewList:
		return []keybind{
			{"j/k", "Move cursor"},
			{"Enter", "Open detail"},
			{"n", "New task"},
			{"e", "Edit task in $EDITOR"},
			{"s", "Start task (advance status)"},
			{"d", "Mark done"},
			{"x", "Cancel task"},
			{"H", "Toggle hidden"},
			{"/", "Open filters"},
			{"Tab", "Next view"},
		}
	case viewKanban:
		return []keybind{
			{"h/l", "Move between columns"},
			{"j/k", "Move within column"},
			{"Tab", "Next column"},
			{"n", "New task"},
			{"e", "Edit task in $EDITOR"},
			{"s", "Advance status (→)"},
			{"S", "Retreat status (←)"},
			{"d", "Mark done"},
			{"x", "Cancel task"},
			{"Enter", "Open detail"},
		}
	}
	return nil
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

	// Center horizontally — account for border (2 chars total)
	totalModalW := modalWidth + 2
	startX := (w - totalModalW) / 2
	if startX < 0 {
		startX = 0
	}

	for i, ml := range modalLines {
		y := startY + i
		if y >= totalLines {
			break
		}
		lineRunes := []rune(lines[y])
		modalRunes := []rune(ml)

		// How many chars can we write starting at startX
		room := len(lineRunes) - startX
		if room <= 0 {
			// Line too short — pad it
			pad := startX + len(modalRunes) - len(lineRunes)
			if pad > 0 {
				lineRunes = append(lineRunes, make([]rune, pad)...)
			}
			room = len(modalRunes)
		}
		if len(modalRunes) > room {
			modalRunes = modalRunes[:room]
		}

		copy(lineRunes[startX:startX+len(modalRunes)], modalRunes)
		lines[y] = string(lineRunes)
	}

	return strings.Join(lines, "\n")
}
