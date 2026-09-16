package tui

import (
	"image/color"
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

	var borderFg color.Color = lipgloss.Color("8")
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
	if limit < 1 {
		limit = 1
	}

	maxLines := p.maxLines
	if maxLines < 1 {
		maxLines = 1
	}

	wrapped := strings.Split(ansi.Wrap(desc, limit, " "), "\n")
	if len(wrapped) > maxLines {
		wrapped = wrapped[:maxLines]
		last := strings.TrimRight(ansi.Truncate(wrapped[len(wrapped)-1], limit-1, ""), " ")
		wrapped[len(wrapped)-1] = last + "…"
	}

	lines := make([]string, len(wrapped))
	for i, l := range wrapped {
		lines[i] = strings.Repeat(" ", previewIndent) + l
	}
	return lines
}
