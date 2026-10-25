package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// OverlayLine overlays modalLine onto bgLine at display position startX.
// Preserves the background's ANSI codes outside the modal area.
func OverlayLine(bgLine, modalLine string, startX int) string {
	bgWidth := ansi.StringWidth(bgLine)
	modalWidth := ansi.StringWidth(modalLine)

	before := ansi.Cut(bgLine, 0, startX)
	// A background line shorter than startX (a blank vertical pad, for example)
	// would leave the modal piece starting at column 0, split from the rest of
	// the box: pad it up to the overlay column first.
	if beforeWidth := ansi.StringWidth(before); beforeWidth < startX {
		before += strings.Repeat(" ", startX-beforeWidth)
	}
	after := ansi.Cut(bgLine, startX+modalWidth, bgWidth)

	return before + "\033[0m" + modalLine + "\033[0m" + after
}
