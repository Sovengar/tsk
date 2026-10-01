package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/db"
)

// El modal de filtros mezcla tres cosas en el mismo render: qué valor tiene cada
// campo, cómo se marca el aplicado y el del cursor, y el recorte de la lista de
// opciones. Los tests que lo cubren buscaban palabras sueltas, con lo que una
// condición de marcado puesta al revés -- marcar todos en vez del elegido, o
// ninguno -- seguía mostrando las mismas palabras.

// filterRender devuelve el modal de filtros sin colores.
func filterRender(t *testing.T, m *Model) string {
	t.Helper()
	m.width = 120
	return ansi.Strip(m.renderFilterModal(""))
}

// El valor mostrado de cada campo: vacío significa "all", y la prioridad vacía
// del filtro es -1, no 0. El 0 es "sin prioridad", que es un filtro de verdad.
func TestFilterCurrentValueLabels(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
		field int
		want  string
	}{
		{"proyecto vacío", nil, filterFieldProject, "all"},
		{"proyecto con valor", func(m *Model) { m.filterProject = "api" }, filterFieldProject, "api"},
		{"estado vacío", nil, filterFieldStatus, "all"},
		{"estado con valor", func(m *Model) { m.filterStatus = "doing" }, filterFieldStatus, "doing"},
		{"estado por defecto", func(m *Model) { m.filterStatus = statusFilterAllActive }, filterFieldStatus, statusFilterAllActive},
		{"asignado vacío", nil, filterFieldAssignee, "all"},
		{"asignado con valor", func(m *Model) { m.filterAssignee = "@juan" }, filterFieldAssignee, "@juan"},
		{"prioridad sin filtro", func(m *Model) { m.filterPriority = -1 }, filterFieldPriority, "all"},
		{"prioridad cero", func(m *Model) { m.filterPriority = 0 }, filterFieldPriority, "none"},
		{"prioridad baja", func(m *Model) { m.filterPriority = 1 }, filterFieldPriority, "low"},
		{"prioridad media", func(m *Model) { m.filterPriority = 2 }, filterFieldPriority, "med"},
		{"prioridad alta", func(m *Model) { m.filterPriority = 3 }, filterFieldPriority, "high"},
		{"tag vacía", nil, filterFieldTag, "all"},
		{"tag con valor", func(m *Model) { m.filterTag = "bug" }, filterFieldTag, "bug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			if tt.setup != nil {
				tt.setup(m)
			}
			if got := m.filterCurrentValue(tt.field); got != tt.want {
				t.Errorf("valor del campo %d = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

// Un campo sin Recognised valor cae en "all": el switch de prioridades no tiene
// caso para un -2, y aun así el modal tiene que mostrar algo.
func TestFilterCurrentValueUnknownFallsBack(t *testing.T) {
	for _, p := range []int{-2, 4, 99} {
		m := &Model{filterPriority: p}
		if got := m.filterCurrentValue(filterFieldPriority); got != "all" {
			t.Errorf("prioridad %d = %q, want all", p, got)
		}
	}
	if got := (&Model{}).filterCurrentValue(filterFieldCount); got != "all" {
		t.Errorf("campo fuera de rango = %q, want all", got)
	}
}

// Con el campo de proyecto enfocado y sin búsqueda, la línea del input muestra el
// valor actual atenuado. Con búsqueda, muestra lo escrito.
func TestFilterInputShowsCurrentValueOrSearch(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = filterFieldProject
	m.filterProject = "api"

	if !strings.Contains(filterRender(t, m), "api") {
		t.Errorf("sin búsqueda no se ve el valor actual:\n%s", filterRender(t, m))
	}

	m.filterSearch = "we"
	if !strings.Contains(filterRender(t, m), "we") {
		t.Errorf("con búsqueda no se ve lo escrito:\n%s", filterRender(t, m))
	}
	if strings.Contains(filterRender(t, m), "api") {
		t.Errorf("con búsqueda se sigue viendo el valor actual:\n%s", filterRender(t, m))
	}
}

// Con más opciones de las que caben, sale el pie "n/total"; con las justas, no.
// El tope cuenta también la opción "all", así que el borde está una etiqueta antes
// de lo que parece. Los casos se sitúan a ambos lados.
func TestFilterOptionsFooter(t *testing.T) {
	tests := []struct {
		name string
		tags int
	}{
		{"dos por debajo", filterMaxVisibleOptions - 2},
		{"justo en el tope", filterMaxVisibleOptions - 1},
		{"una de más", filterMaxVisibleOptions},
		{"muchas de más", filterMaxVisibleOptions + 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.filterOpen = true
			m.filterFieldIdx = filterFieldTag
			for i := range tt.tags {
				mustCreateTaskWithTags(t, m.database, "api", "tarea", "", "@juan", 1, "todo",
					[]string{fmt.Sprintf("etiqueta%02d", i)})
			}
			m.tasks, _ = m.database.ListTasks("", "", "")
			m.filteredT = nil

			total := len(m.filterFieldOptions(filterFieldTag))
			want := total > filterMaxVisibleOptions

			out := filterRender(t, m)
			if got := strings.Contains(out, "/"+strconv.Itoa(total)); got != want {
				t.Errorf("con %d opciones (%d etiquetas) el pie %v, want %v\n%s",
					total, tt.tags, got, want, out)
			}
		})
	}
}

// El valor aplicado lleva un punto y el del cursor un triángulo, y no son la
// misma fila salvo que coincidan. Aquí se separan a propósito: el aplicado es
// "all" y el cursor va en la segunda opción.
func TestFilterMarksAppliedAndCursorSeparately(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = filterFieldProject
	m.filterProject = "" // aplicado: "all", que es la primera opción
	m.filterOptionIdx = 1

	out := filterRender(t, m)
	// "all" es la primera y está aplicada: sólo el punto.
	if !strings.Contains(out, "● all") {
		t.Errorf("la opción aplicada no lleva el punto:\n%s", out)
	}
	// La segunda es la del cursor: sólo el triángulo.
	if !strings.Contains(out, "▸ ") {
		t.Errorf("no se ve el cursor:\n%s", out)
	}
	// Ninguna fila lleva los dos: eso sería el caso de "aplicado y seleccionada".
	if strings.Contains(out, "● ▸") || strings.Contains(out, "▸ ●") {
		t.Errorf("una fila lleva las dos marcas, no deberían:\\n%s", out)
	}
}

// mustCreateTaskWithTags crea una tarea con tags, que es lo que hace que el campo
// de tags del modal tenga opciones que listar.
func mustCreateTaskWithTags(t *testing.T, database *db.DB, projectName, title, description, assignee string, priority int, status string, tags []string) {
	t.Helper()
	if _, err := database.CreateTaskFull(projectName, title, description, assignee, priority, status, 0, tags); err != nil {
		t.Fatalf("CreateTaskFull(%q): %v", title, err)
	}
}
