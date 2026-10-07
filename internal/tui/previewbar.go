package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

const (
	// previewMaxLines is the maximum number of description lines the preview shows.
	previewMaxLines = 25
	// previewIndent is the indentation of the content inside the box.
	previewIndent = 2
)

// PreviewBar shows the selected task's description in a bordered box.
type PreviewBar struct {
	width    int
	maxLines int
	task     *model.Task
}

// ellipsisDescription is what is put at the end of a description that does not fit.
const ellipsisDescription = "…"

// ellipsisWidth is what the ellipsis takes up. It is a constant and not an
// inline ansi.StringWidth(ellipsisDescription) because a string's width was
// computed on every truncation, and the mutant of subtracting it twice or
// adding it --both changes indistinguishable with the floor at 0-- was left
// in an expression that did not say what it was discounting.
const ellipsisWidth = 1

// truncateForEllipsis leaves the exact gap for the ellipsis and returns the
// line with the ellipsis glued on.
//
// The floor at 0 is what keeps strings.Repeat and ansi.Truncate from blowing
// up with negative numbers when the limit is smaller than the ellipsis -- a
// two-column terminal with the description box.
func truncateForEllipsis(last string, limit int) string {
	room := max(limit-ellipsisWidth, 0)
	return strings.TrimRight(ansi.Truncate(last, room, ""), " ") + ellipsisDescription
}

// NewPreviewBar creates a new PreviewBar.
func NewPreviewBar(width int) PreviewBar {
	return PreviewBar{width: width, maxLines: previewMaxLines}
}

// SetWidth updates the width.
func (p *PreviewBar) SetWidth(w int) {
	p.width = w
}

// SetMaxLines adjusts how many description lines are shown at most.
func (p *PreviewBar) SetMaxLines(n int) {
	p.maxLines = n
}

// SetTask updates the selected task.
func (p *PreviewBar) SetTask(t *model.Task) {
	p.task = t
}

// View renders the box. It returns "" if there is no selected task.
func (p PreviewBar) View() string {
	if p.task == nil {
		return ""
	}

	content := strings.Join(p.descriptionLines(p.task.Description), "\n")

	borderFg := lipgloss.Color("8")
	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		borderFg,
		bordered.AlignLeft,
		" Description ",
		content,
		p.width,
	)
}

// descriptionLines wraps the description to the available width and
// truncates it to maxLines. When truncating it reserves a cell for the "…":
// if the last line exceeded the inner width, bordered would re-wrap it and
// add an extra row, breaking the height cap.
func (p PreviewBar) descriptionLines(desc string) []string {
	if strings.TrimSpace(desc) == "" {
		return []string{strings.Repeat(" ", previewIndent) + styleDim.Render("(no description)")}
	}

	limit := p.width - previewIndent - 2 // left and right borders
	limit = max(limit, 1)

	maxLines := p.maxLines
	maxLines = max(maxLines, 1)

	wrapped := strings.Split(ansi.Wrap(desc, limit, " "), "\n")
	if len(wrapped) > maxLines {
		wrapped = wrapped[:maxLines]
		// A cell is reserved for the ellipsis BEFORE truncating, and the
		// truncation goes with a limit derived from it instead of `limit - 1`.
		//
		// `limit - 1` was a number already in place because of what the
		// ellipsis did, and for that very reason it did not tell itself apart
		// from `limit` nor `limit - 2`: ansi.Wrap delivers lines of limit
		// columns or fewer, so truncating to any of the three gave the same.
		// Now the limit is the gap left after putting the ellipsis, which is a
		// named number, and `limit - ellipsisWidth` does say what it means.
		//
		// The truncation is not decorative: if the last line reaches limit
		// columns, adding the ellipsis without truncating would push it one
		// column too far and bordered would re-wrap it in two, breaking the height cap.
		last := wrapped[len(wrapped)-1]
		wrapped[len(wrapped)-1] = truncateForEllipsis(last, limit)
	}

	lines := make([]string, len(wrapped))
	for i, l := range wrapped {
		lines[i] = strings.Repeat(" ", previewIndent) + l
	}
	return lines
}
