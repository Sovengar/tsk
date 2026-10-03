package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
	"tsk/internal/tui/bordered"
)

const (
	// previewMaxLines es el máximo de líneas de descripción que muestra el preview.
	previewMaxLines = 25
	// previewIndent es la indentación del contenido dentro de la caja.
	previewIndent = 2
)

// PreviewBar muestra la descripción de la tarea seleccionada en una caja con borde.
type PreviewBar struct {
	width    int
	maxLines int
	task     *model.Task
}

// elipsisDescription es lo que se pone al final de una descripción que no cabe.
const elipsisDescription = "…"

// anchoElipsis es lo que ocupa la elipsis. Es una constante y no
// ansi.StringWidth(elipsisDescription) en línea porque el ancho de una cadena se
// calculaba en cada recorte, y el mutante de restarlo dos veces o de sumarlo
// --cambios ambos indistinguibles con el suelo en 0-- quedaba en una expresión
// que no decía qué estaba descontando.
const anchoElipsis = 1

// recortarParaElipsis deja el hueco justo para la elipsis y devuelve la línea con
// la elipsis pegada.
//
// El suelo en 0 es lo que evita que strings.Repeat y ansi.Truncate revienten con
// números negativos cuando el límite es más pequeño que la elipsis -- una terminal
// de dos columnas con la caja de descripción.
func recortarParaElipsis(ultima string, limit int) string {
	room := max(limit-anchoElipsis, 0)
	return strings.TrimRight(ansi.Truncate(ultima, room, ""), " ") + elipsisDescription
}

// NewPreviewBar crea un nuevo PreviewBar.
func NewPreviewBar(width int) PreviewBar {
	return PreviewBar{width: width, maxLines: previewMaxLines}
}

// SetWidth actualiza el ancho.
func (p *PreviewBar) SetWidth(w int) {
	p.width = w
}

// SetMaxLines ajusta cuántas líneas de descripción se muestran como máximo.
func (p *PreviewBar) SetMaxLines(n int) {
	p.maxLines = n
}

// SetTask actualiza la tarea seleccionada.
func (p *PreviewBar) SetTask(t *model.Task) {
	p.task = t
}

// View renderiza la caja. Devuelve "" si no hay tarea seleccionada.
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

// descriptionLines envuelve la descripción al ancho disponible y la recorta a
// maxLines. Al truncar reserva una celda para el "…": si la última línea
// excediera el ancho interno, bordered la re-wrapéaría y agregaría una fila
// de más, rompiendo el tope de alto.
func (p PreviewBar) descriptionLines(desc string) []string {
	if strings.TrimSpace(desc) == "" {
		return []string{strings.Repeat(" ", previewIndent) + styleDim.Render("(no description)")}
	}

	limit := p.width - previewIndent - 2 // bordes izquierdo y derecho
	limit = max(limit, 1)

	maxLines := p.maxLines
	maxLines = max(maxLines, 1)

	wrapped := strings.Split(ansi.Wrap(desc, limit, " "), "\n")
	if len(wrapped) > maxLines {
		wrapped = wrapped[:maxLines]
		// Se reserva una celda para la elipsis ANTES de recortar, y el recorte
		// va con un límite derivado de ella en vez de con `limit - 1`.
		//
		// `limit - 1` era un número que ya estaba en el sitio por lo que hacía
		// la elipsis, y por eso mismo no se distinguía de `limit` ni de
		// `limit - 2`: ansi.Wrap entrega líneas de limit columnas o menos, así que
		// truncar a cualquiera de los tres daba lo mismo. Ahora el límite es el
		// hueco que queda tras poner la elipsis, que es un número con nombre, y
		// `limit - anchoElipsis` sí dice qué quiere decir.
		//
		// El recorte no es decorativo: si la última línea llega a limit columnas,
		// añadirle la elipsis sin recortar la empujaría una columna de más y la
		// bordered la re-wrapearía en dos, rompiendo el tope de alto.
		last := wrapped[len(wrapped)-1]
		wrapped[len(wrapped)-1] = recortarParaElipsis(last, limit)
	}

	lines := make([]string, len(wrapped))
	for i, l := range wrapped {
		lines[i] = strings.Repeat(" ", previewIndent) + l
	}
	return lines
}
