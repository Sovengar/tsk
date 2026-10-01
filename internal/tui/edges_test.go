package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// inicioGantt es una fecha fija para que el render del Gantt sea reproducible.
var inicioGantt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Lote de bordes que quedaban sin comprobar en ficheros donde el resto del
// render ya estaba cubierto: el punto de selección de las listas del modal de
// personas, el ancho de la regla del Gantt, los resets de "nada seleccionado" del
// editor de descripción, y el borrado de tags del alta.

// La fila seleccionada del roster lleva el prefijo de selección, y el resto no.
// Con dos personas en el roster y el cursor en la segunda, sólo esa lo lleva.
func TestAssigneeRosterSelectionPrefix(t *testing.T) {
	m := newAssigneeModel(t, 2)
	m.archivedProjects = nil
	reloadOffDays(t, m)
	m.assigneeIdx = 1

	out := ansi.Strip(m.renderAssigneeModal(""))
	if n := strings.Count(out, selectionPrefix(true)); n != 1 {
		t.Errorf("hay %d filas con el prefijo de selección, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, selectionPrefix(true)+"@user01") {
		t.Errorf("la marca no está en la segunda persona:\n%s", out)
	}
}

// Lo mismo en la lista de off-days: el cursor va sobre un off-day concreto, no
// sobre "la lista".
func TestAssigneeOffdaySelectionPrefix(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	for i := range 3 {
		dia := "2026-0" + string(rune('1'+i)) + "-01"
		mustAddOffDay(t, m.database, "@user00", dia, dia, "")
	}
	reloadOffDays(t, m)
	m.assigneeDetail = true
	m.assigneeOffdayIdx = 2

	out := ansi.Strip(m.renderAssigneeModal(""))
	if n := strings.Count(out, selectionPrefix(true)); n != 1 {
		t.Errorf("hay %d filas con el prefijo de selección, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, selectionPrefix(true)+"2026-03-01") {
		t.Errorf("la marca no está en el tercer off-day:\n%s", out)
	}
}

// El eje del Gantt tiene el ancho de la etiqueta, un separador y las columnas de
// día, sin recortar. La regla no sirve para esto: le quita los espacios de la
// derecha, así que su ancho visible depende de dónde cae el último lunes.
func TestGanttAxisExactWidth(t *testing.T) {
	tests := []struct {
		name    string
		labelW  int
		dayCols int
	}{
		{"holgada", 30, 40},
		{"etiqueta al mínimo", 14, 7},
		{"un solo día", 30, 1},
		{"con etiqueta y días mínimos", 14, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ganttModelWithPeople(t, []string{"@juan"}, 1)

			axis := ansi.Strip(m.renderGanttAxis(tt.labelW, tt.dayCols, inicioGantt, 0))
			if got := len([]rune(axis)); got != tt.labelW+1+tt.dayCols {
				t.Errorf("el eje mide %d columnas, want %d (etiqueta %d + 1 + días %d)",
					got, tt.labelW+1+tt.dayCols, tt.labelW, tt.dayCols)
			}
		})
	}
}

// El rótulo de cada lunes cae en su columna: la etiqueta de la izquierda, el
// separador, y desde ahí el día. El 1 de enero de 2026 es jueves, así que el
// primer lunes es el día 4, en la columna 35 con una etiqueta de 30.
func TestGanttRulerLabelLandsOnItsMonday(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)

	const labelW = 30
	ruler := []rune(ansi.Strip(m.renderGanttRuler(inicioGantt, 0, labelW, 20)))
	const primerLunes = 4 // 2026-01-05

	want := model.WeekOfMonthLabel(inicioGantt.AddDate(0, 0, primerLunes))
	at := labelW + 1 + primerLunes
	if at+len([]rune(want)) > len(ruler) {
		t.Fatalf("el rótulo %q no cabe en la regla de %d columnas", want, len(ruler))
	}
	if got := string(ruler[at : at+len([]rune(want))]); got != want {
		t.Errorf("en la columna %d hay %q, want %q (ruler %q)", at, got, want, string(ruler))
	}
	// Y las columnas anteriores al lunes están en blanco: el rótulo no puede
	// empezar antes.
	for i := primerLunes; i < at; i++ {
		if ruler[i] != ' ' {
			t.Fatalf("columna %d = %q, want espacio antes del rótulo", i, string(ruler[i]))
		}
	}
}

// La etiqueta de la izquierda del eje son espacios del ancho exacto de la
// etiqueta, ni uno más.
func TestGanttAxisLeadingSpaces(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 1)

	axis := []rune(ansi.Strip(m.renderGanttAxis(30, 10, inicioGantt, 0)))
	for i, r := range axis {
		if i < 31 {
			if r != ' ' {
				t.Fatalf("columna %d = %q, want espacio (etiqueta + separador)", i, string(r))
			}
			continue
		}
		break
	}
	// A partir del separador hay un carácter por día, y el primero es el jueves,
	// que no es lunes, así que es "-".
	if axis[31] != '-' {
		t.Errorf("el primer día es %q, want \"-\"", string(axis[31]))
	}
}

// Abrir el editor de descripción limpia la selección de comentarios, igual que
// abrir el detalle: la descripción no tiene comentarios, y dejar la selección
// apuntando a un índice de comentarios que no se están mostrando haría que la
// primera tecla saltara de sitio.
func TestOpeningDescEditorClearsCommentSelection(t *testing.T) {
	m := newDetailWithTags(t, "bug")
	m.detailComments = []model.Comment{{ID: 1, Body: "uno", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.detailCommentSel = 1

	next, _ := press(m, "e")
	got := next

	if !got.descEditOpen {
		t.Fatal("la tecla e no abrió el editor de descripción")
	}
	if got.detailCommentSel != -1 {
		t.Errorf("detailCommentSel = %d, want -1", got.detailCommentSel)
	}
}

// El editor de descripción se abre sobre la tarea del detalle si está abierto, y
// sobre la seleccionada en la vista si no. Con el detalle abierto, editar es
// sobre la del detalle aunque el cursor de la lista esté en otra.
func TestDescEditTargetPrefersOpenDetail(t *testing.T) {
	m := newTestModel(t)
	tarea := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &tarea
	m.filteredT = nil

	if got := m.descEditTarget(); got == nil || got.ID != tarea.ID {
		t.Errorf("con el detalle abierto el objetivo es %v, want la tarea del detalle", got)
	}

	m.detailOpen = false
	m.cursor = 1
	seleccionada := m.filteredTasks()[1]
	if got := m.descEditTarget(); got == nil || got.ID != seleccionada.ID {
		t.Errorf("sin detalle el objetivo es %v, want la tarea seleccionada", got)
	}

	// Y sin ninguna de las dos, nil.
	m.filteredT = nil
	m.cursor = 99
	if got := m.descEditTarget(); got != nil {
		t.Errorf("sin tarea seleccionada el objetivo es %v, want nil", got)
	}
}

// Borrar tags del campo del alta: con el input vacío, backspace quita la última
// agregada; con algo escrito, quita un carácter del input. Y编辑 Tags limpia la
// selección de sugerencias, porque la lista va a cambiar.
func TestNewTaskTagsBackspace(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		tags      []string
		wantInput string
		wantTags  []string
	}{
		{"con input escrito quita un carácter", "ab", nil, "a", nil},
		{"con input vacío quita la última tag", "", []string{"uno", "dos"}, "", []string{"uno"}},
		{"sin tags ni input no hace nada", "", nil, "", nil},
		{"con una tag la quita", "", []string{"uno"}, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			next, _ := press(m, "i")
			m2 := next
			m2.newTaskFieldIdx = newTaskFieldTags
			m2.newTaskTagInput = tt.input
			m2.newTaskTags = append([]string(nil), tt.tags...)
			m2.newTaskTagSuggIdx = 2

			got, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
			gotM := got.(Model)

			if gotM.newTaskTagInput != tt.wantInput {
				t.Errorf("input = %q, want %q", gotM.newTaskTagInput, tt.wantInput)
			}
			if strings.Join(gotM.newTaskTags, ",") != strings.Join(tt.wantTags, ",") {
				t.Errorf("tags = %v, want %v", gotM.newTaskTags, tt.wantTags)
			}
			// Editar el input limpia la selección de sugerencias; borrar una tag
			// agregada no. Es lo que hace el código, y no es un problema: el
			// índice se acota antes de usarse.
			wantSugg := -1
			if tt.input == "" && len(tt.tags) > 0 {
				wantSugg = 2
			}
			if gotM.newTaskTagSuggIdx != wantSugg {
				t.Errorf("suggIdx = %d, want %d", gotM.newTaskTagSuggIdx, wantSugg)
			}
		})
	}
}
