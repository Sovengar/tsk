package tui

import "github.com/charmbracelet/x/ansi"

// OverlayLine overlays modalLine onto bgLine at display position startX.
// Preserves the background's ANSI codes outside the modal area.
func OverlayLine(bgLine, modalLine string, startX int) string {
	bgWidth := ansi.StringWidth(bgLine)
	modalWidth := ansi.StringWidth(modalLine)

	before := ansi.Cut(bgLine, 0, startX)
	after := ansi.Cut(bgLine, startX+modalWidth, bgWidth)

	return before + "\033[0m" + modalLine + "\033[0m" + after
}
