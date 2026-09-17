package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// detailKeybinds devuelve las teclas del modal de detalle de tarea, en orden de
// prioridad. Fuente única para la KeybindsBar y el modal de ayuda.
func detailKeybinds() []keybind {
	return []keybind{
		{"j/k", "select comment"},
		{"c", "new comment"},
		{"t", "tags"},
		{"d", "delete/done"},
		{"e/E", "edit/editor"},
		{"s", "start"},
		{"x", "cancel"},
		{"Esc", "close"},
	}
}

func (m *Model) renderHelpModal(content string) string {
	w := m.width

	title := styleTitle.Render("  Keybindings — " + m.currentView.String())

	sep := styleSep.Render(strings.Repeat("─", 40))

	// View keys (incluyen las comunes: ya no hay sección global)
	var viewLines []string
	viewLines = append(viewLines, styleColumnHeader.Render("  "+m.currentView.String()))
	for _, kb := range keybindsForView(m.currentView) {
		viewLines = append(viewLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	// Detail modal keys
	viewLines = append(viewLines, "")
	viewLines = append(viewLines, styleColumnHeader.Render("  Task detail (modal)"))
	for _, kb := range detailKeybinds() {
		viewLines = append(viewLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	footer := styleDim.Render("  Press ? or Esc to close")

	body := strings.Join(viewLines, "\n")

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
