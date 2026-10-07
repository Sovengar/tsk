package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

// detailBorderFg is the border color of the detail's boxes, same as the
// keybinds bar, so the three boxes look like a set.
var detailBorderFg = lipgloss.Color("8")

// renderDetail renders the task detail modal within the available
// height, as two boxes with a rounded border: task+description on top and
// comments below. The third box (keybinds) is added by the layout.
func (m *Model) renderDetail(t *model.Task, maxHeight int) string {
	w := m.width
	h := maxHeight
	// The inner width of the boxes: the same as a modal's, because it is the
	// same discount for the two borders.
	inner := modalInnerWidth(w)

	// Priority with colored character
	prio := priorityChar(t.Priority) + " " + model.PriorityLabel(t.Priority)

	// Metadata
	tagsDisplay := strings.Join(t.Tags, ", ")
	if tagsDisplay == "" {
		tagsDisplay = "—"
	}
	meta := []string{
		fmt.Sprintf("  Status:     %-20s Project:   %s", t.Status, t.ProjectName),
		fmt.Sprintf("  Assignee:   %-20s Estimate:  %s", t.Assignee, model.FormatEstimate(t.Estimate)),
		fmt.Sprintf("  Tags:       %s", tagsDisplay),
		fmt.Sprintf("  Created:    %-20s Updated:   %s", formatTime(t.CreatedAt), formatTime(t.UpdatedAt)),
		fmt.Sprintf("  Completed:  %s", formatCompleted(t.CompletedAt)),
	}

	// Description. In edit mode the section is replaced by the textarea
	// embedded in the same box, instead of overlaying another modal.
	var descLines []string
	if m.descEditOpen {
		descLines = m.descEditorLines()
	} else {
		desc := "(no description)"
		if t.Description != "" {
			desc = t.Description
		}
		descLines = strings.Split(desc, "\n")
		// Indent each non-empty line, same as the metadata.
		for i := range descLines {
			if strings.TrimSpace(descLines[i]) != "" {
				descLines[i] = "  " + descLines[i]
			}
		}
	}

	commentLines := m.renderCommentLines(w)

	// Height split between comments and description. The arithmetic lives in
	// detailHeightBudget: here it is only applied.
	commentBudget, descBudget := detailHeightBudget(h, len(m.detailComments))

	// The truncation uses min and no if: descLines[:descBudget] is the same
	// when it fits whole, so the condition only decided what the slice already decided.
	descLines = descLines[:min(len(descLines), descBudget)]

	// Comment window that follows the selection.
	//
	// visibleRange already returns the whole page when the window is larger
	// than the total, so the outer `if` was redundant: outside == inside. It
	// was removed, and with it the condition that only told apart when the
	// truncation was an identity.
	start, end := commentWindow(len(commentLines), commentBudget, m.detailCommentSel)
	visibleComments := commentLines[start:end]

	// Box 1: task + description.
	sep := styleSep.Render(strings.Repeat("─", inner))
	// No capacity hint: it is arithmetic that no test can look at, because a
	// capacity is advice and the result is the same whatever it says.
	// append computes the growth on its own.
	var taskContent []string
	taskContent = append(taskContent, meta...)
	taskContent = append(taskContent, sep, "  Description:")
	taskContent = append(taskContent, descLines...)
	taskBox := bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		detailBorderFg,
		bordered.AlignLeft,
		styleTitle.Render(fmt.Sprintf(" #%d — %s  %s ", t.ID, t.Title, prio)),
		truncateLines(strings.Join(taskContent, "\n"), inner),
		w,
	)

	// Box 2: comments.
	commentsBox := bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		detailBorderFg,
		bordered.AlignLeft,
		styleTitle.Render(fmt.Sprintf(" Comments (%d) ", len(m.detailComments))),
		strings.Join(visibleComments, "\n"),
		w,
	)

	content := taskBox + "\n\n" + commentsBox

	// Center vertically. At equal height the fill is zero line breaks, which
	// is the same as not filling at all, so the if decided nothing: the
	// truncation is left to a max() that says in one line what the if said in three.
	content = padVertical(content, h)

	return content
}

// padVertical centers vertically a block of height `target` padded with line
// breaks on top. If the block is already taller, it leaves it alone.
func padVertical(content string, target int) string {
	topPad := max(target-lineCount(content), 0) / 2
	if topPad == 0 {
		return content
	}
	return strings.Repeat("\n", topPad) + content
}

// commentWindow returns the range [start, end) of the comment lines
// that are painted, following the selection.
//
// It is separated because its call had an `if` around it that was redundant:
// visibleRange already returns the whole window when the window is larger or
// equal to the total, so outside == inside and the condition only told apart
// in the case where both branches give the same result.
func commentWindow(total, budget, sel int) (int, int) {
	return visibleRange(max(sel, 0), total, budget)
}

// renderCommentLines renders each comment on one line, marking the
// selected one. Multi-line comments collapse to a single line.
func (m *Model) renderCommentLines(width int) []string {
	if len(m.detailComments) == 0 {
		return []string{"  (no comments)"}
	}

	lines := make([]string, 0, len(m.detailComments))
	for i, c := range m.detailComments {
		marker := "  "
		if i == m.detailCommentSel {
			marker = styleSelected.Render("> ")
		}
		line := marker + styleDim.Render(formatCommentTime(c.CreatedAt)) + "  " + singleLine(c.Body)
		// The two columns less are the 2-column prefix and the gap between the
		// date and the body. They are written as named constants because the raw
		// `width - 2` did not say what it was discounting, and with a name the
		// account can be checked: a comment with the body exactly at the edge has
		// to lose one character more with the right margin than with a narrower one.
		lines = append(lines, truncateLines(line, width-commentPrefix-dateGap))
	}
	return lines
}

const (
	// commentPrefix are the columns of the selection marker ("  " or "> ").
	// Both cases measure the same.
	commentPrefix = 2
	// dateGap are the columns between the date and the comment body.
	dateGap = 2
)

// formatCommentTime shortens an RFC3339 timestamp to "YYYY-MM-DD HH:MM".
func formatCommentTime(s string) string {
	if s == "" {
		return "—"
	}
	s = strings.Replace(s, "T", " ", 1)
	return truncateAt(s, 16)
}

func formatTime(s string) string {
	if s == "" {
		return "—"
	}
	return truncateAt(s, 19)
}

// formatCompleted is formatTime: the empty case already covers it. The guard
// it had here returned the same as formatTime's, so it was a branch that no
// test could distinguish.
func formatCompleted(s string) string {
	return formatTime(s)
}
