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

	// El recorte va con min y sin if: descLines[:descBudget] es lo mismo cuando
	// cabe entero, así que la condición sólo decidía lo que el slice ya decidía.
	descLines = descLines[:min(len(descLines), descBudget)]

	// Ventana de comentarios que sigue a la selección.
	//
	// visibleRange ya devuelve la página entera cuando la ventana es mayor que
	// el total, así que el `if` de fuera era redundante: outside == inside. Se
	// quita, y con él la condición que sólo se distinguía cuando el recorte era
	// una identidad.
	start, end := ventanaDeComentarios(len(commentLines), commentBudget, m.detailCommentSel)
	visibleComments := commentLines[start:end]

	// Caja 1: tarea + descripción.
	sep := styleSep.Render(strings.Repeat("─", inner))
	// Sin pista de capacidad: es una aritmética que ningún test puede mirar,
	// porque una capacidad es un consejo y el resultado es el mismo diga lo que
	// diga. append calcula el crecimiento por su cuenta.
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

	// Center vertically. A igual alto el relleno son cero saltos, que es lo mismo
	// que no rellenar, así que el if no decidía nada: se deja el recorte a un
	// max() que dice en una línea lo que el if dizia en tres.
	content = padVertical(content, h)

	return content
}

// padVertical centra verticalmente un bloque de alto `target` relleno con saltos
// de línea por arriba. Si el bloque ya es más alto, no lo toca.
func padVertical(content string, target int) string {
	topPad := max(target-lineCount(content), 0) / 2
	if topPad == 0 {
		return content
	}
	return strings.Repeat("\n", topPad) + content
}

// ventanaDeComentarios devuelve el rango [start, end) de las líneas de
// comentario que se pintan, siguiendo a la selección.
//
// Se separa porque su llamada tenía un `if` alrededor que era redundante:
// visibleRange ya devuelve la ventana entera cuando la ventana es mayor o igual
// que el total, así que outside == inside y la condición sólo se distinguía en el
// caso donde las dos ramas dan el mismo resultado.
func ventanaDeComentarios(total, budget, sel int) (int, int) {
	return visibleRange(max(sel, 0), total, budget)
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
		// Las dos columnas menos son el prefijo de 2 y el hueco entre la fecha y
		// el cuerpo. Se escriben como constantes con nombre porque el `width - 2`
		// a pelo no decía qué estaba descontando, y con un nombre la cuenta se
		// puede comprobar: un comentario con el cuerpo justo en el borde tiene que
		// perder un carácter más con el margen correcto que con uno más estrecho.
		lines = append(lines, truncateLines(line, width-prefijoComentario-huecoFecha))
	}
	return lines
}

const (
	// prefijoComentario son las columnas del marcador de selección ("  " o "> ").
	// Los dos casos miden lo mismo.
	prefijoComentario = 2
	// huecoFecha son las columnas entre la fecha y el cuerpo del comentario.
	huecoFecha = 2
)

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
