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
	// El ancho interior de las cajas: el mismo que el de un modal, porque es el
	// mismo descuento por los dos bordes.
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

	// Description. En modo edición la sección se reemplaza por el textarea
	// integrado en la misma caja, en lugar de superponer otro modal.
	var descLines []string
	if m.descEditOpen {
		descLines = m.descEditorLines()
	} else {
		desc := "(no description)"
		if t.Description != "" {
			desc = t.Description
		}
		descLines = strings.Split(desc, "\n")
		// Identar cada línea no vacía, igual que la metadata.
		for i := range descLines {
			if strings.TrimSpace(descLines[i]) != "" {
				descLines[i] = "  " + descLines[i]
			}
		}
	}

	commentLines := m.renderCommentLines(w)

	// Reparto de alto entre comentarios y descripción. La aritmética vive en
	// detailHeightBudget: aquí sólo se aplica.
	commentBudget, descBudget := detailHeightBudget(h, len(m.detailComments))

	if len(descLines) > descBudget {
		descLines = descLines[:descBudget]
	}

	// Ventana de comentarios que sigue a la selección.
	visibleComments := commentLines
	if len(commentLines) > commentBudget {
		cursor := max(m.detailCommentSel, 0)
		start, end := visibleRange(cursor, len(commentLines), commentBudget)
		visibleComments = commentLines[start:end]
	}

	// Caja 1: tarea + descripción.
	sep := styleSep.Render(strings.Repeat("─", inner))
	// Sin capacidad inicial: append la calcula, y una pista de capacidad es una
	// aritmética más que un mutante más que nobody va a matar.
	taskContent := make([]string, 0, len(meta)+len(descLines)+2) //nolint:mnd
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
	return truncateAt(s, 16)
}

func formatTime(s string) string {
	if s == "" {
		return "—"
	}
	return truncateAt(s, 19)
}

// formatCompleted es formatTime: el caso vacío ya lo cubre. La guarda que tenía
// aquí devolvía lo mismo que la de formatTime, así que era una rama que ningún
// test podía distinguir.
func formatCompleted(s string) string {
	return formatTime(s)
}
