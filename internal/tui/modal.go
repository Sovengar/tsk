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
//
// Un min en vez de un if: el umbral es "cabe entero", así que a preferred == w-2
// las dos formas devuelven lo mismo y la comparación era un mutante equivalente.
func modalWidthFor(preferred, w int) int {
	return min(preferred, w-2)
}

// overlayModal centra un modal ya renderizado sobre content, preservando el
// fondo. totalWidth es el ancho total del modal (bordes incluidos). Los keybinds
// se muestran en la barra inferior, no dentro del modal.
func overlayModal(content, modal string, totalWidth, w int) string {
	lines := strings.Split(content, "\n")
	modalLines := strings.Split(modal, "\n")
	modalH := len(modalLines)

	startY := (len(lines) - modalH) / 2
	startY = max(startY, 0)
	for len(lines) < startY+modalH {
		lines = append(lines, strings.Repeat(" ", w))
	}

	startX := (w - totalWidth) / 2
	startX = max(startX, 0)

	for i, ml := range modalLines {
		lines[startY+i] = OverlayLine(lines[startY+i], ml, startX)
	}
	return strings.Join(lines, "\n")
}
