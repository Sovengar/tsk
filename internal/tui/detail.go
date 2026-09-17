package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

// detailBorderFg es el color del borde de las cajas del detalle, igual que la
// barra de keybinds para que las tres cajas se vean como un conjunto.
var detailBorderFg = lipgloss.Color("8")

// renderDetail renderiza el modal de detalle de tarea dentro del alto
// disponible, como dos cajas con borde redondeado: tarea+descripción arriba y
// comentarios abajo. La tercera caja (keybinds) la agrega el layout.
func (m *Model) renderDetail(t *model.Task, maxHeight int) string {
	w := m.width
	h := maxHeight
	inner := w - 2 // ancho interior de las cajas (descuenta los bordes)
	if inner < 1 {
		inner = 1
	}

	// Priority with colored character
	prio := priorityChar(t.Priority) + " " + model.PriorityLabel(t.Priority)

	// Metadata
	meta := []string{
		fmt.Sprintf("  Status:     %-20s Project:   %s", t.Status, t.ProjectName),
		fmt.Sprintf("  Assignee:   %-20s Estimate:  %s", t.Assignee, model.FormatEstimate(t.Estimate)),
		fmt.Sprintf("  Created:    %-20s Updated:   %s", formatTime(t.CreatedAt), formatTime(t.UpdatedAt)),
		fmt.Sprintf("  Completed:  %s", formatCompleted(t.CompletedAt)),
	}

	// Description
	desc := "(no description)"
	if t.Description != "" {
		desc = t.Description
	}
	descLines := strings.Split(desc, "\n")
	// Identar cada línea no vacía, igual que la metadata.
	for i := range descLines {
		if strings.TrimSpace(descLines[i]) != "" {
			descLines[i] = "  " + descLines[i]
		}
	}

	commentLines := m.renderCommentLines(w)

	// Reparto de alto entre descripción y comentarios.
	// fixed = líneas que no son contenido:
	//   caja tarea:      2 bordes + 4 meta + sep + "Description:" = 8
	//   separación:      1
	//   caja comentarios: 2 bordes = 2
	const fixed = 11
	avail := h - fixed
	if avail < 3 {
		avail = 3
	}

	// Al menos una línea de comentarios (o "(no comments)").
	maxCommentLines := avail - 2
	if maxCommentLines < 1 {
		maxCommentLines = 1
	}
	commentBudget := len(m.detailComments)
	if commentBudget < 1 {
		commentBudget = 1
	}
	if commentBudget > maxCommentLines {
		commentBudget = maxCommentLines
	}
	descBudget := avail - commentBudget
	if descBudget < 1 {
		descBudget = 1
	}

	if len(descLines) > descBudget {
		descLines = descLines[:descBudget]
	}

	// Ventana de comentarios que sigue a la selección.
	visibleComments := commentLines
	if len(commentLines) > commentBudget {
		cursor := m.detailCommentSel
		if cursor < 0 {
			cursor = 0
		}
		start, end := visibleRange(cursor, len(commentLines), commentBudget)
		visibleComments = commentLines[start:end]
	}

	// Caja 1: tarea + descripción.
	sep := styleSep.Render(strings.Repeat("─", inner))
	taskContent := make([]string, 0, len(meta)+2+len(descLines))
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

	// Caja 2: comentarios.
	commentsBox := bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		detailBorderFg,
		bordered.AlignLeft,
		styleTitle.Render(fmt.Sprintf(" Comments (%d) ", len(m.detailComments))),
		strings.Join(visibleComments, "\n"),
		w,
	)

	content := taskBox + "\n\n" + commentsBox

	// Center vertically
	totalLines := lineCount(content)
	if totalLines < h {
		topPad := (h - totalLines) / 2
		content = strings.Repeat("\n", topPad) + content
	}

	return content
}

// renderCommentLines renderiza cada comentario en una línea, marcando el
// seleccionado. Los comentarios multilínea se colapsan a una sola línea.
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
		lines = append(lines, truncateLines(line, width-2))
	}
	return lines
}

// formatCommentTime acorta un timestamp RFC3339 a "YYYY-MM-DD HH:MM".
func formatCommentTime(s string) string {
	if s == "" {
		return "—"
	}
	s = strings.Replace(s, "T", " ", 1)
	if len(s) >= 16 {
		return s[:16]
	}
	return s
}

func formatTime(s string) string {
	if s == "" {
		return "—"
	}
	// Simple truncation to date+time
	if len(s) >= 19 {
		return s[:19]
	}
	return s
}

func formatCompleted(s string) string {
	if s == "" {
		return "—"
	}
	return formatTime(s)
}
