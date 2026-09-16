package tui

import "github.com/charmbracelet/x/ansi"

// OverlayLine superpone modalLine sobre bgLine en la posición de display startX.
// Preserva códigos ANSI del fondo fuera del área del modal.
func OverlayLine(bgLine, modalLine string, startX int) string {
	bgWidth := ansi.StringWidth(bgLine)
	modalWidth := ansi.StringWidth(modalLine)

	before := ansi.Cut(bgLine, 0, startX)
	after := ansi.Cut(bgLine, startX+modalWidth, bgWidth)

	return before + "\033[0m" + modalLine + "\033[0m" + after
}
