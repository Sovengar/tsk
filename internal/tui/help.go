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

// helpModalWidth es el ancho interior preferido del modal de ayuda. El conjunto
// son las teclas de vista más las de detalle, y cabe de sobra en 46 columnas.
const helpModalWidth = 46

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

	// modalWidthFor deja siempre dos columnas de margen; si no, el modal se
	// pegaría al borde de la pantalla.
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

	// El centrado vertical y horizontal es el de overlayModal, no una segunda
	// versión: estaba reimplementado aquí línea por línea, con sus propios
	// suelos, y por eso cada uno de esos números era un sitio donde un mutante
	// podía colarse sin que ningún test lo notara. El ancho que recibe
	// overlayModal incluye los bordes.
	return overlayModal(content, modal, modalWidth+2, w)
}
