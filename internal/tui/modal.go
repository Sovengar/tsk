package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// renderModalBox draws the lines into a box with a rounded border and a title
// embedded in the top-left line. width is the total width of the modal
// (borders included).
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

// modalWidthFor adjusts the modal's preferred width to the available width.
//
// A min instead of an if: the threshold is "fits entirely", so at preferred == w-2
// both forms return the same and the comparison was an equivalent mutant.
func modalWidthFor(preferred, w int) int {
	return min(preferred, w-2)
}

// overlayModal centers an already rendered modal over content, preserving the
// background. totalWidth is the total width of the modal (borders included). The keybinds
// are shown in the bottom bar, not inside the modal.
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
