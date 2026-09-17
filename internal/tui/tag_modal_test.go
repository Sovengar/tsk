package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// typeTag escribe cada carácter en el modal de tags.
func typeTag(m Model, text string) Model {
	for _, ch := range text {
		next, _ := m.handleTagModalKey(string(ch))
		m = next.(Model)
	}
	return m
}

// openTagModal abre el detalle de la primera tarea y su modal de tags.
func openTagModal(t *testing.T, m *Model) Model {
	t.Helper()
	task := (*m).tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	next, _ := m.handleDetailKey("t")
	got := next.(Model)
	if !got.tagOpen {
		t.Fatal("\"t\" no abrió el modal de tags")
	}
	return got
}

// TestTagModalTogglesTag verifica que Enter agregue la tag escrita y que, si ya
// está aplicada, la quite; el modal queda abierto tras cada toggle.
func TestTagModalTogglesTag(t *testing.T) {
	m := newTestModel(t)
	taskID := m.tasks[0].ID
	m2 := openTagModal(t, m)

	m2 = typeTag(m2, "blocked")
	next, cmd := m2.handleTagModalKey("enter")
	m2 = next.(Model)
	if cmd == nil {
		t.Fatal("Enter no devolvió comando de toggle")
	}
	if m2.tagOpen != true {
		t.Error("el modal debería seguir abierto tras el toggle")
	}
	if m2.tagInput != "" {
		t.Errorf("el input debería limpiarse tras el toggle: %q", m2.tagInput)
	}
	next, _ = m2.Update(cmd())
	m2 = next.(Model)

	got, err := m2.database.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if !model.HasTag(got.Tags, "blocked") {
		t.Fatalf("la tag no se agregó: %v", got.Tags)
	}
	if m2.detailTask == nil || !model.HasTag(m2.detailTask.Tags, "blocked") {
		t.Errorf("el detalle no refleja la tag nueva: %+v", m2.detailTask)
	}

	// Segundo toggle: la misma tag se quita.
	m2 = typeTag(m2, "blocked")
	next, cmd = m2.handleTagModalKey("enter")
	m2 = next.(Model)
	next, _ = m2.Update(cmd())
	m2 = next.(Model)

	got, _ = m2.database.GetTask(taskID)
	if model.HasTag(got.Tags, "blocked") {
		t.Errorf("la tag no se quitó: %v", got.Tags)
	}
}

// TestTagModalSuggestionsCompleteAndClose verifica sugerencias, Tab y Esc.
func TestTagModalSuggestionsCompleteAndClose(t *testing.T) {
	m := newTestModel(t)
	taskID := m.tasks[0].ID
	if _, err := m.database.SetTaskTags(taskID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks

	m2 := openTagModal(t, m)

	if suggs := m2.tagSuggestions(); !hasOption(suggs, "blocked") {
		t.Fatalf("las sugerencias no incluyen la tag existente: %v", suggs)
	}

	// Filtrar por prefijo y completar con Tab.
	m2 = typeTag(m2, "block")
	if suggs := m2.tagSuggestions(); !hasOption(suggs, "blocked") {
		t.Fatalf("el filtro por prefijo no encontró blocked: %v", suggs)
	}
	next, _ := m2.handleTagModalKey("tab")
	m2 = next.(Model)
	if m2.tagInput != "blocked" {
		t.Errorf("Tab no completó la sugerencia: %q", m2.tagInput)
	}

	next, _ = m2.handleTagModalKey("esc")
	m2 = next.(Model)
	if m2.tagOpen {
		t.Error("Esc no cerró el modal de tags")
	}
	if !m2.detailOpen {
		t.Error("cerrar el modal de tags no debería cerrar el detalle")
	}
}

// TestRenderTagModalShowsCurrent y sugerencias.
func TestRenderTagModalShowsCurrent(t *testing.T) {
	m := newTestModel(t)
	taskID := m.tasks[0].ID
	if _, err := m.database.SetTaskTags(taskID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks

	m2 := openTagModal(t, m)
	m2.width = 100
	out := ansi.Strip(m2.renderTagModal("base"))
	if !strings.Contains(out, "Tags") {
		t.Errorf("el modal no muestra el título Tags:\n%s", out)
	}
	if !strings.Contains(out, "Current: blocked") {
		t.Errorf("el modal no muestra las tags actuales:\n%s", out)
	}
	if !strings.Contains(out, "✓ blocked") {
		t.Errorf("la sugerencia aplicada debería marcarse con ✓:\n%s", out)
	}
}
