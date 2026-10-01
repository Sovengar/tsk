package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// El modal de tags mezcla tres cosas: qué sugerencias salen, cuál está
// seleccionada y cuáles ya están aplicadas a la tarea. Los tests que lo cubren
// buscaban el texto de una tag concreta, con lo que una condición de marcado
// puesta al revés -- marcar todas, o ninguna -- seguía mostrando las mismas
// palabras.

// tagRender devuelve el modal de tags sin colores.
func tagRender(t *testing.T, m *Model) string {
	t.Helper()
	m.width = 120
	return ansi.Strip(m.renderTagModal(""))
}

// newTagModel abre el modal de tags sobre la primera tarea, con tags distintas
// en la base de datos para que haya sugerencias.
func newTagModel(t *testing.T, tags ...string) *Model {
	t.Helper()
	m := newTestModel(t)
	for _, tag := range tags {
		if _, err := m.database.CreateTaskFull("api", "con "+tag, "", "@juan", 1, "todo", 0, []string{tag}); err != nil {
			t.Fatalf("CreateTaskFull(%q): %v", tag, err)
		}
	}
	m.tasks, _ = m.database.ListTasks("", "", "")
	m.filteredT = nil

	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	m.tagOpen = true
	m.tagInput = ""
	m.tagSuggestIdx = -1
	return m
}

// Sin texto escrito salen todas las tags que hay; escribiendo un prefijo, sólo
// las que empiezan por él. El prefijo se busca en minúsculas y sin espacios.
func TestTagSuggestionsFilterByPrefix(t *testing.T) {
	m := newTagModel(t, "bug", "build", "chore")

	if got := m.tagSuggestions(); len(got) != 3 {
		t.Errorf("sin escribir salen %d sugerencias, want 3: %v", len(got), got)
	}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"vacío sale todo", "", []string{"bug", "build", "chore"}},
		{"sólo espacios equivale a vacío", "   ", []string{"bug", "build", "chore"}},
		{"prefijo", "bu", []string{"bug", "build"}},
		{"prefijo en minúsculas contra mayúsculas", "BU", []string{"bug", "build"}},
		{"con espacios alrededor", "  bu  ", []string{"bug", "build"}},
		{"sin coincidencias", "zzz", nil},
		{"prefijo del medio no cuenta", "ug", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.tagInput = tt.input
			got := m.tagSuggestions()
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("tagSuggestions con %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Una tag que no existe en ninguna tarea no sale como sugerencia, pero el modal
// lo ofrece como creación nueva.
func TestTagModalOffersNewTag(t *testing.T) {
	m := newTagModel(t, "bug")
	m.tagInput = "nueva"

	out := tagRender(t, m)
	if !strings.Contains(out, "(nueva tag)") {
		t.Errorf("una tag sin sugerencias no lo dice:\\n%s", out)
	}
	if strings.Contains(out, "bug") {
		t.Errorf("salió una sugerencia que no empieza por el prefijo:\\n%s", out)
	}
}

// Con sugerencias, no sale el "(nueva tag)": hay de dónde elegir.
func TestTagModalNoNewTagWhenThereAreSuggestions(t *testing.T) {
	m := newTagModel(t, "bug")
	m.tagInput = "bu"

	if out := tagRender(t, m); strings.Contains(out, "(nueva tag)") {
		t.Errorf("con sugerencias sale igualmente el (nueva tag):\\n%s", out)
	}
}

// La línea "Current:" dice (none) sin tags y las tags unidas por comas con
// ellas. Es lo que le dice a quien está en el modal qué tiene puesto ya.
func TestTagModalCurrentLine(t *testing.T) {
	m := newTagModel(t)
	m.detailTask.Tags = nil
	if out := tagRender(t, m); !strings.Contains(out, "Current: (none)") {
		t.Errorf("sin tags no dice (none):\\n%s", out)
	}

	m.detailTask.Tags = []string{"uno", "dos"}
	if out := tagRender(t, m); !strings.Contains(out, "Current: uno, dos") {
		t.Errorf("con tags no las lista:\\n%s", out)
	}
}

// Las sugerencias se recortan al tope, y el que se ve es el principio de la
// lista, no un subconjunto disperso.
func TestTagModalClipsSuggestions(t *testing.T) {
	tags := make([]string, tagMaxSuggestions+5)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%02d", i)
	}
	m := newTagModel(t, tags...)

	suggs := m.tagSuggestions()
	if len(suggs) != len(tags) {
		t.Fatalf("el modelo tiene %d sugerencias, want %d", len(suggs), len(tags))
	}

	out := tagRender(t, m)
	for _, tag := range tags[:tagMaxSuggestions] {
		if !strings.Contains(out, tag) {
			t.Errorf("no se ve %q, que debería estar entre las %d primeras:\\n%s", tag, tagMaxSuggestions, out)
		}
	}
	for _, tag := range tags[tagMaxSuggestions:] {
		if strings.Contains(out, tag) {
			t.Errorf("se ve %q, que está más allá del tope:\\n%s", tag, out)
		}
	}
}

// Las tags que la tarea ya tiene salen con un tick, y las que no sin él. Es la
// diferencia entre "puedes quitar esto" y "esto lo puedes añadir".
func TestTagModalMarksAppliedTags(t *testing.T) {
	m := newTagModel(t, "aplicada", "libre")
	m.detailTask.Tags = []string{"aplicada"}
	m.tagInput = ""

	out := tagRender(t, m)
	if !strings.Contains(out, "✓ aplicada") {
		t.Errorf("la tag aplicada no lleva el tick:\\n%s", out)
	}
	for _, linea := range strings.Split(out, "\n") {
		if strings.Contains(linea, "libre") && strings.Contains(linea, "✓") {
			t.Errorf("una tag no aplicada lleva el tick: %q", linea)
		}
	}
}

// Escribir reinicia la selección: si no, el cursor se quedaría apuntando a una
// sugerencia que ya no está en la lista.
func TestTagTypingResetsSuggestionIndex(t *testing.T) {
	m := newTagModel(t, "bug", "build")
	m.tagSuggestIdx = 1

	for _, key := range []string{"b", "u"} {
		next, _ := press(m, key)
		if next.tagSuggestIdx != -1 {
			t.Errorf("tras escribir %q el índice de sugerencia es %d, want -1", key, next.tagSuggestIdx)
		}
	}

	// Backspace también: se sigue editando el mismo input.
	m2 := newTagModel(t, "bug")
	m2.tagSuggestIdx = 0
	m2.tagInput = "bu"
	next, _ := press(m2, "backspace")
	if next.tagSuggestIdx != -1 {
		t.Errorf("tras backspace el índice es %d, want -1", next.tagSuggestIdx)
	}
	if next.tagInput != "b" {
		t.Errorf("tras backspace el input es %q, want %q", next.tagInput, "b")
	}
}

// BUG: el espacio no se puede escribir en el input de tags.
//
// Bubbletea no reporta el espacio como un carácter suelto sino con la cadena
// "space", de modo que la condición `len(key) == 1` del handler lo descarta y la
// tecla se pierde entera. Una tag con espacio -- "in progress", "waiting for QA" --
// no se puede escribir a mano: hay que elegirla de las sugerencias o editar el
// fichero.
//
// El test fija el comportamiento actual, no el que debería. Cuando se arregle,
// hay que cambiar esta expectativa.
func TestTagSpaceIsNotTypedIntoTheInput(t *testing.T) {
	m := newTagModel(t)
	m.tagInput = "co"

	next, _ := press(m, "space")
	if next.tagInput != "co" {
		t.Errorf("tras el espacio el input es %q, want %q sin cambio (bug conocido)", next.tagInput, "co")
	}

	// El resto de imprimibles sí pasan: son los que la condición sí ve.
	m2 := newTagModel(t)
	m2.tagInput = ""
	for _, key := range []string{"a", "Z", "1", "9", "-"} {
		next, _ := press(m2, key)
		if next.tagInput == "" {
			t.Errorf("la tecla %q no llegó al input:\n%s", key, ansi.Strip(m2.renderTagModal("")))
		}
		m2.tagInput = next.tagInput
	}
}

// Esc cierra el modal y limpia el estado, incluido el índice de sugerencia.
func TestTagEscResetsState(t *testing.T) {
	m := newTagModel(t, "bug")
	m.tagSuggestIdx = 2
	m.tagInput = "bu"

	next, _ := press(m, "esc")
	if next.tagOpen {
		t.Error("esc no cerró el modal")
	}
	if next.tagInput != "" {
		t.Errorf("el input quedó en %q", next.tagInput)
	}
	if next.tagSuggestIdx != -1 {
		t.Errorf("el índice quedó en %d, want -1", next.tagSuggestIdx)
	}
}

// El tick de una tag aplicada depende de la tarea del detalle: sin tarea abierta
// no hay nada contra lo que comparar.
func TestTagModalWithoutOpenTask(t *testing.T) {
	m := newTagModel(t, "bug")
	m.detailTask = nil
	m.detailOpen = false

	out := tagRender(t, m)
	if strings.Contains(out, "✓") {
		t.Errorf("sin tarea abierta sale un tick:\\n%s", out)
	}
	if !strings.Contains(out, "Current: (none)") {
		t.Errorf("sin tarea abierta:\\n%s", out)
	}
}

// hasTag es lo que decide el tick del render, así que el render y la regla de
// negocio no pueden discrepar.
func TestTagAppliedFlagMatchesHasTag(t *testing.T) {
	m := newTagModel(t, "aplicada", "libre")
	m.detailTask.Tags = []string{"aplicada"}
	m.tagInput = ""

	out := tagRender(t, m)
	for _, tag := range []string{"aplicada", "libre"} {
		lineaConTag := ""
		for _, linea := range strings.Split(out, "\n") {
			if strings.Contains(linea, tag) && !strings.Contains(linea, "Current:") {
				lineaConTag = linea
				break
			}
		}
		if lineaConTag == "" {
			t.Errorf("no encontré la fila de %q:\n%s", tag, out)
			continue
		}
		tieneTick := strings.Contains(lineaConTag, "✓")
		if tieneTick != model.HasTag(m.detailTask.Tags, tag) {
			t.Errorf("%q: el render dice tick=%v y HasTag=%v", tag, tieneTick, model.HasTag(m.detailTask.Tags, tag))
		}
	}
}

// La fila seleccionada va en negrita, y sólo una: con el índice en -1 (nada
// seleccionado) no hay ninguna en negrita, y con un índice válido exactamente
// esa. El tick y la negrita son marcas distintas y no se sustituyen.
func TestTagModalHighlightsExactlyOneSuggestion(t *testing.T) {
	tests := []struct {
		name  string
		idx   int
		wantN int
	}{
		{"nada seleccionado", -1, 0},
		{"primera", 0, 1},
		{"la del medio", 1, 1},
		{"la última", 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTagModel(t, "aaa", "bbb", "ccc")
			m.tagInput = ""
			m.tagSuggestIdx = tt.idx

			raw := m.renderTagModal("")
			if got := countBoldLines(raw); got != tt.wantN {
				t.Errorf("con el índice en %d hay %d filas en negrita, want %d", tt.idx, got, tt.wantN)
			}
		})
	}
}

// countBoldLines cuenta las líneas del modal que llevan el estilo de selección,
// que es negrita. Se cuenta sobre el render sin quitar los códigos: si se
// quitaran, las tres filas serían indistinguibles.
func countBoldLines(rendered string) int {
	n := 0
	for _, linea := range strings.Split(rendered, "\n") {
		// El estilo de selección abre con la secuencia de negrita de lipgloss.
		if strings.Contains(linea, "\x1b[1m") {
			n++
		}
	}
	return n
}

// Tab y Enter completan y consumen: ambas cosas limpian la selección de
// sugerencias, porque el input pasa a ser el valor completo y la lista de
// sugerencias va a cambiar.
func TestTagTabAndEnterClearSuggestionIndex(t *testing.T) {
	tests := []struct {
		key       string
		wantInput string
	}{
		// Tab completa: el input pasa a ser la sugerencia entera.
		{"tab", "build"},
		// Enter la agrega y deja el input vacío para la siguiente.
		{"enter", ""},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := newTagModel(t, "bug", "build")
			m.tagInput = "bu"
			m.tagSuggestIdx = 1

			next, _ := press(m, tt.key)
			got := next

			if got.tagSuggestIdx != -1 {
				t.Errorf("tras %q el índice es %d, want -1", tt.key, got.tagSuggestIdx)
			}
			if got.tagInput != tt.wantInput {
				t.Errorf("tras %q el input es %q, want %q", tt.key, got.tagInput, tt.wantInput)
			}
		})
	}
}
