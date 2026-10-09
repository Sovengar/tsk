package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// detailKeybinds returns the keys of the task detail modal, in order of
// priority. Single source for the KeybindsBar and the help modal.
func detailKeybinds() []keybind {
	return []keybind{
		{"j/k", "select comment"},
		{"c", "new comment"},
		{"t", "tags"},
		{"a", "ask AI"},
		{"d", "delete/done"},
		{"e/E", "edit/editor"},
		{"s", "start"},
		{"x", "cancel"},
		{"Esc", "close"},
	}
}

// helpModalWidth is the preferred inner width of the help modal. The set is
// the view keys plus the detail keys, and it fits comfortably in 46 columns.
const helpModalWidth = 46

func (m *Model) renderHelpModal(content string) string {
	w := m.width

	title := styleTitle.Render("  Keybindings — " + m.currentView.String())

	sep := styleSep.Render(strings.Repeat("─", 40))

	// View keys (include the common ones: there is no global section anymore)
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

	// New task modal keys
	viewLines = append(viewLines, "")
	viewLines = append(viewLines, styleColumnHeader.Render("  New task (modal)"))
	for _, kb := range newTaskKeybinds() {
		viewLines = append(viewLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	// Filter modal keys
	viewLines = append(viewLines, "")
	viewLines = append(viewLines, styleColumnHeader.Render("  Filters (modal)"))
	for _, kb := range filterKeybinds() {
		viewLines = append(viewLines, "  "+styleWarn.Render(kb.key)+"  "+kb.desc)
	}

	footer := styleDim.Render("  Press ? or Esc to close")

	body := strings.Join(viewLines, "\n")

	// modalWidthFor always leaves two columns of margin; otherwise the modal
	// would stick to the edge of the screen.
	modalWidth := modalWidthFor(helpModalWidth, w)

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

	// The vertical and horizontal centering is overlayModal's, not a second
	// version: it was reimplemented here line by line, with its own
	// floors, and that is why each of those numbers was a place where a mutant
	// could slip in without any test noticing. The width
	// that overlayModal receives includes the borders.
	return overlayModal(content, modal, modalWidth+2, w)
}
