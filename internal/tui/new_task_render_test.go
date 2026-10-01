package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// El modal de nueva tarea tiene seis campos y el foco va en uno solo: la fila
// enfocada lleva un "▸" delante y el resto no. Ese marcador es lo único que dice
// dónde está el cursor, así que comprobar que aparece exactamente una vez es lo
// que distingue el campo enfocado de cualquiera de los otros cinco.

// newTaskRender devuelve el modal de alta sin colores, con el campo indicado
// enfocado.
func newTaskRender(t *testing.T, field int) string {
	t.Helper()
	m := newTestModel(t)
	next, _ := press(m, "i") // abrir el alta
	next.width = 120
	next.newTaskFieldIdx = field
	next.newTaskTitle = "mi título"
	next.newTaskAssignee = "@juan"
	return ansi.Strip(next.renderNewTaskModal(""))
}

// Cada campo enfocado deja su marca, y sólo la suya.
func TestNewTaskFocusMarkerPerField(t *testing.T) {
	campos := []struct {
		name  string
		field int
		label string
	}{
		{"priority", newTaskFieldPriority, "Priority"},
		{"title", newTaskFieldTitle, "Title"},
		{"assignee", newTaskFieldAssignee, "Assignee"},
		{"tags", newTaskFieldTags, "Tags"},
		{"description", newTaskFieldDescription, "Description"},
	}
	for _, campo := range campos {
		t.Run(campo.name, func(t *testing.T) {
			out := newTaskRender(t, campo.field)

			if n := strings.Count(out, "▸"); n != 1 {
				t.Errorf("con el foco en %s hay %d marcadores, want 1:\n%s", campo.name, n, out)
			}
			// El marcador va pegado a la etiqueta del campo enfocado.
			for _, linea := range strings.Split(out, "\n") {
				if !strings.Contains(linea, "▸") {
					continue
				}
				if !strings.Contains(linea, campo.label) {
					t.Errorf("el marcador está en %q, y el campo enfocado es %q", linea, campo.label)
				}
			}
		})
	}
}

// El texto del campo enfocado lleva además el cursor de escritura, y el de los
// demás no. El cursor va pegado al valor, no en la etiqueta.
func TestNewTaskWriteCursorOnlyOnFocusedField(t *testing.T) {
	enTitle := newTaskRender(t, newTaskFieldTitle)
	if !strings.Contains(enTitle, cursorGlyph) {
		t.Errorf("con el foco en Title no sale el cursor de escritura:\n%s", enTitle)
	}

	// Sólo los campos de texto llevan cursor de escritura: la prioridad es un
	// selector y no se escribe en él.
	out := newTaskRender(t, newTaskFieldAssignee)
	if n := strings.Count(out, cursorGlyph); n != 1 {
		t.Errorf("con el foco en Assignee hay %d cursores de escritura, want 1:\n%s", n, out)
	}
}

// Con el foco fuera del título, el valor sale sin cursor pegado.
func TestNewTaskNoWriteCursorOutsideTitle(t *testing.T) {
	out := newTaskRender(t, newTaskFieldPriority)
	// El cursor sólo puede aparecer pegado al valor del campo enfocado, y el
	// de prioridad no lleva cursor aunque esté enfocado.
	if strings.Contains(out, "mi título"+cursorGlyph) {
		t.Errorf("con el foco fuera, el título sigue con cursor:\n%s", out)
	}
}
