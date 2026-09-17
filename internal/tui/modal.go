package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// renderModalBox dibuja las líneas en una caja con borde redondeado y un título
// embebido en la línea superior izquierda. width es el ancho total del modal
// (bordes incluidos).
func renderModalBox(title string, lines []string, width int) string {
	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		lipgloss.Color("8"),
		bordered.AlignLeft,
		title,
		strings.Join(lines, "\n"),
		width,
	)
}

// modalWidthFor ajusta el ancho preferido del modal al ancho disponible.
func modalWidthFor(preferred, w int) int {
	if preferred > w-2 {
		return w - 2
	}
	return preferred
}

// overlayModal centra un modal ya renderizado sobre content, preservando el
// fondo. totalWidth es el ancho total del modal (bordes incluidos). Los keybinds
// se muestran en la barra inferior, no dentro del modal.
func overlayModal(content, modal string, totalWidth, w int) string {
	lines := strings.Split(content, "\n")
	modalLines := strings.Split(modal, "\n")
	modalH := len(modalLines)

	startY := (len(lines) - modalH) / 2
	if startY < 0 {
		startY = 0
	}
	for len(lines) < startY+modalH {
		lines = append(lines, strings.Repeat(" ", w))
	}

	startX := (w - totalWidth) / 2
	if startX < 0 {
		startX = 0
	}

	for i, ml := range modalLines {
		lines[startY+i] = OverlayLine(lines[startY+i], ml, startX)
	}
	return strings.Join(lines, "\n")
}
