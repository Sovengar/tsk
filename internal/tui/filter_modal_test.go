package tui

import (
	"testing"

	"tsk/internal/model"
)

// filterOptionIndex busca una opción dentro de la lista de un campo.
func filterOptionIndex(opts []string, want string) int {
	for i, o := range opts {
		if o == want {
			return i
		}
	}
	return -1
}

// TestFilterStatusOptionsScopedToProject verifica que con un proyecto
// seleccionado el selector de estado ofrezca sólo los estados de ese proyecto.
func TestFilterStatusOptionsScopedToProject(t *testing.T) {
	m := newTestModel(t)

	m.filterProject = "api"
	opts := m.filterFieldOptions(filterFieldStatus)
	// api usa el workflow por defecto.
	for _, want := range append([]string{"all"}, model.DefaultWorkflow...) {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("con api seleccionado falta %q en %v", want, opts)
		}
	}
	// Ningún estado que no pertenezca a api.
	if filterOptionIndex(opts, "reviewing") < 0 {
		t.Errorf("api debería ofrecer reviewing: %v", opts)
	}

	// web define un workflow propio más corto.
	m.filterProject = "web"
	opts = m.filterFieldOptions(filterFieldStatus)
	for _, want := range []string{"all", "todo", "doing", "done"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("con web seleccionado falta %q en %v", want, opts)
		}
	}
	for _, absent := range []string{"backlog", "reviewing", "cancelled"} {
		if filterOptionIndex(opts, absent) >= 0 {
			t.Errorf("con web seleccionado no debería ofrecerse %q: %v", absent, opts)
		}
	}
}

// TestFilterStatusClearedOnProjectSwitch verifica que al cambiar de proyecto
// se limpie el filtro de estado si ese estado no existe en el nuevo contexto.
func TestFilterStatusClearedOnProjectSwitch(t *testing.T) {
	m := newTestModel(t)

	// api tiene "reviewing"; web no.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "reviewing")
	if m.filterStatus != "reviewing" {
		t.Fatalf("precondición: filterStatus = %q, want reviewing", m.filterStatus)
	}

	m.filterApplySelection(filterFieldProject, "web")
	if m.filterStatus != "" {
		t.Errorf("al pasar a web, reviewing no existe: filterStatus = %q, want \"\"", m.filterStatus)
	}

	// Un estado común sobrevive el cambio.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "doing")
	m.filterApplySelection(filterFieldProject, "web")
	if m.filterStatus != "doing" {
		t.Errorf("doing existe en web: filterStatus = %q, want doing", m.filterStatus)
	}

	// Volver a "all projects": sólo sobrevive lo que está en la intersección.
	m.filterApplySelection(filterFieldProject, "api")
	m.filterApplySelection(filterFieldStatus, "cancelled")
	m.filterApplySelection(filterFieldProject, "all")
	if m.filterStatus != "" {
		t.Errorf("cancelled no está en todos: filterStatus = %q, want \"\"", m.filterStatus)
	}
}

// TestFilterStatusOptionsAllProjectsIsIntersection verifica que sin proyecto
// seleccionado el selector sólo ofrezca estados comunes a todos los proyectos.
func TestFilterStatusOptionsAllProjectsIsIntersection(t *testing.T) {
	m := newTestModel(t)

	opts := m.filterFieldOptions(filterFieldStatus)
	// api (default) ∩ web ([todo,doing,done]) = [todo,doing,done].
	for _, want := range []string{"all", "todo", "doing", "done"} {
		if filterOptionIndex(opts, want) < 0 {
			t.Errorf("intersección: falta %q en %v", want, opts)
		}
	}
	for _, absent := range []string{"backlog", "reviewing", "cancelled"} {
		if filterOptionIndex(opts, absent) >= 0 {
			t.Errorf("intersección no debería incluir %q (no está en todos): %v", absent, opts)
		}
	}
}
