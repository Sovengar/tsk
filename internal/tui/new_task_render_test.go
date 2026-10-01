package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
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

// Pegar texto va al campo activo, y en los campos con sugerencias limpia la
// selección: la lista va a cambiar y el cursor se quedaría apuntando a algo que
// ya no está.
func TestNewTaskPasteGoesToActiveField(t *testing.T) {
	tests := []struct {
		name       string
		field      int
		wantTitle  string
		wantAssign string
		wantTagIn  string
	}{
		// El responsable viene puesto en "Me" al abrir el alta, así que pegar
		// encima de él lo concatena en vez de sustituirlo.
		{"title", newTaskFieldTitle, "pegado", "", ""},
		{"assignee", newTaskFieldAssignee, "", "Mepegado", ""},
		{"tags", newTaskFieldTags, "", "", "pegado"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			next, _ := press(m, "i")
			m2 := next
			m2.newTaskFieldIdx = tt.field
			m2.newTaskAssigneeSuggIdx = 3
			m2.newTaskTagSuggIdx = 3
			m2.newTaskErr = "Title is required"

			pasted, _ := m2.Update(tea.PasteMsg{Content: "pegado"})
			got := pasted.(Model)

			if got.newTaskTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", got.newTaskTitle, tt.wantTitle)
			}
			wantAssign := tt.wantAssign
			if wantAssign == "" {
				wantAssign = "Me" // el valor por defecto del campo
			}
			if got.newTaskAssignee != wantAssign {
				t.Errorf("assignee = %q, want %q", got.newTaskAssignee, wantAssign)
			}
			if got.newTaskTagInput != tt.wantTagIn {
				t.Errorf("tagInput = %q, want %q", got.newTaskTagInput, tt.wantTagIn)
			}
			// El error de "falta el título" sólo se limpia pegando en el título:
			// en los demás campos el título sigue faltando.
			if tt.field == newTaskFieldTitle && got.newTaskErr != "" {
				t.Errorf("pegar en el título no limpia el error: %q", got.newTaskErr)
			}
			if tt.field != newTaskFieldTitle && got.newTaskErr == "" {
				t.Errorf("pegar en %s limpió el error sin tocar el título: %q", tt.name, got.newTaskErr)
			}
			if tt.field == newTaskFieldAssignee && got.newTaskAssigneeSuggIdx != -1 {
				t.Errorf("suggIdx de assignee = %d, want -1", got.newTaskAssigneeSuggIdx)
			}
			if tt.field == newTaskFieldTags && got.newTaskTagSuggIdx != -1 {
				t.Errorf("suggIdx de tags = %d, want -1", got.newTaskTagSuggIdx)
			}
		})
	}
}

// Pegar en la descripción va al textarea, no a los inputs de una línea: un
// salto de línea es una línea nueva, no un espacio.
func TestNewTaskPasteIntoDescriptionKeepsNewlines(t *testing.T) {
	m := newTestModel(t)
	next, _ := press(m, "i")
	m2 := next
	m2.newTaskFieldIdx = newTaskFieldDescription
	m2.newTaskTitle = "antes"
	// El textarea sólo acepta el pegado con el foco puesto, que es lo que hace el
	// handler al abrir el modal.
	m2.newTaskTextarea.Focus()

	pasted, _ := m2.Update(tea.PasteMsg{Content: "uno\ndos"})
	got := pasted.(Model)

	if got.newTaskTitle != "antes" {
		t.Errorf("el título cambió al pegar en la descripción: %q", got.newTaskTitle)
	}
	valor := got.newTaskTextarea.Value()
	if !strings.Contains(valor, "uno") || !strings.Contains(valor, "dos") {
		t.Errorf("el textarea quedó en %q", valor)
	}
	if !strings.Contains(valor, "\n") {
		t.Errorf("el salto de línea no se conservó: %q", valor)
	}
}

// Abrir el alta deja el modelo en un estado conocido: foco en prioridad, sin
// título, "Me" como responsable, ninguna sugerencia seleccionada y sin tags.
// Los dos -1 son "nada seleccionado", y un 1 ahí apuntaría a una lista de
// sugerencias que todavía no existe.
func TestOpeningNewTaskResetsToInitialState(t *testing.T) {
	m := newTestModel(t)

	// Antes de abrir, el estado es distinto: hay sugerencia seleccionada y tags.
	next, _ := press(m, "i")
	got := next

	if !got.newTaskOpen {
		t.Fatal("la tecla i no abrió el alta")
	}
	if got.newTaskFieldIdx != newTaskFieldPriority {
		t.Errorf("el foco abre en %d, want %d (prioridad)", got.newTaskFieldIdx, newTaskFieldPriority)
	}
	if got.newTaskTitle != "" {
		t.Errorf("el título arranca en %q, want vacío", got.newTaskTitle)
	}
	if got.newTaskAssignee != "Me" {
		t.Errorf("el responsable arranca en %q, want \"Me\"", got.newTaskAssignee)
	}
	if got.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx de assignee = %d, want -1", got.newTaskAssigneeSuggIdx)
	}
	if got.newTaskTagSuggIdx != -1 {
		t.Errorf("suggIdx de tags = %d, want -1", got.newTaskTagSuggIdx)
	}
	if len(got.newTaskTags) != 0 {
		t.Errorf("arranca con %d tags, want ninguna", len(got.newTaskTags))
	}
	if got.newTaskErr != "" {
		t.Errorf("arranca con error %q, want ninguno", got.newTaskErr)
	}
	if got.newTaskPriority != model.PriorityLow {
		t.Errorf("la prioridad arranca en %d, want %d", got.newTaskPriority, model.PriorityLow)
	}
}
