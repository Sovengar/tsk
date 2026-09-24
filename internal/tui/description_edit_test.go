package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// ctrlS arma el keypress de guardado del editor inline.
func ctrlS() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
}

// send aplica un mensaje crudo y normaliza el modelo devuelto a *Model.
func send(m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	switch v := next.(type) {
	case *Model:
		return v, cmd
	case Model:
		return &v, cmd
	default:
		panic("unexpected model type")
	}
}

// TestDescEditorOpensInList verifica que "e" abre el editor inline (no nvim)
// precargado con la descripción de la tarea seleccionada.
func TestDescEditorOpensInList(t *testing.T) {
	m := newTestModel(t)
	want := m.tasks[0]

	m, _ = press(m, "e")
	if !m.descEditOpen {
		t.Fatal("e debe abrir el editor inline de descripción")
	}
	if !m.detailOpen {
		t.Error("el editor se integra en el detalle: debe abrirse el detalle")
	}
	if m.descEditTaskID != want.ID {
		t.Errorf("taskID = %d, want %d", m.descEditTaskID, want.ID)
	}
	if got := m.descEditTextarea.Value(); got != want.Description {
		t.Errorf("textarea = %q, want %q", got, want.Description)
	}
	if m.overlayKind() != overlayDescEdit {
		t.Errorf("overlay = %v, want overlayDescEdit", m.overlayKind())
	}
}

// TestDescEditorTypingAndNewline verifica que Enter inserta salto de línea en
// lugar de guardar.
func TestDescEditorTypingAndNewline(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "e")

	base := m.descEditTextarea.Value()
	m, _ = press(m, "x")
	if got := m.descEditTextarea.Value(); got != base+"x" {
		t.Fatalf("tras tipear: %q, want %q", got, base+"x")
	}

	m, _ = press(m, "enter")
	if got := m.descEditTextarea.Value(); !strings.Contains(got, "\n") {
		t.Errorf("Enter debe insertar salto de línea, got %q", got)
	}
	if !m.descEditOpen {
		t.Error("Enter no debe cerrar el editor")
	}
}

// TestDescEditorEscCancels verifica que Esc cierra sin persistir.
func TestDescEditorEscCancels(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	m, _ = press(m, "e")
	m, _ = press(m, "!")
	m, _ = press(m, "esc")

	if m.descEditOpen {
		t.Fatal("Esc debe cerrar el editor")
	}
	if m.detailOpen {
		t.Error("al editar desde la lista, Esc debe volver a la lista (cerrar el detalle)")
	}
	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != task.Description {
		t.Errorf("Esc no debe persistir: %q, want %q", got.Description, task.Description)
	}
}

// TestDescEditorCtrlSSaves verifica que Ctrl+S persiste la descripción.
func TestDescEditorCtrlSSaves(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]

	m, _ = press(m, "e")
	m.descEditTextarea.SetValue("nueva descripción")
	m, cmd := send(m, ctrlS())

	if m.descEditOpen {
		t.Fatal("Ctrl+S debe cerrar el editor")
	}
	if cmd == nil {
		t.Fatal("Ctrl+S debe devolver cmd de guardado")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("el cmd de guardado devolvió nil")
	}

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "nueva descripción" {
		t.Errorf("description = %q, want %q", got.Description, "nueva descripción")
	}
}

// TestDescEditorFromDetailKeepsDetail verifica que editar desde el detalle
// mantiene el modal abierto detrás y refleja el cambio al guardar.
func TestDescEditorFromDetailKeepsDetail(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Fatal("enter debe abrir el detalle")
	}

	m, _ = press(m, "e")
	if !m.descEditOpen {
		t.Fatal("e en detalle debe abrir el editor inline")
	}
	if !m.detailOpen {
		t.Fatal("el detalle debe quedar abierto detrás")
	}
	if m.overlayKind() != overlayDescEdit {
		t.Errorf("overlay = %v, want overlayDescEdit", m.overlayKind())
	}

	m.descEditTextarea.SetValue("editada desde detalle")
	m, cmd := send(m, ctrlS())
	if cmd == nil {
		t.Fatal("Ctrl+S debe devolver cmd")
	}
	if !m.detailOpen || m.detailTask == nil {
		t.Fatal("el detalle debe seguir abierto tras guardar")
	}
	if m.detailTask.Description != "editada desde detalle" {
		t.Errorf("detailTask.Description = %q", m.detailTask.Description)
	}
}

// TestExternalEditorKeyUppercase verifica que "E" sigue abriendo el editor
// externo (cerrando el detalle en ese caso).
func TestExternalEditorKeyUppercase(t *testing.T) {
	m := newTestModel(t)

	m, cmd := press(m, "E")
	if cmd == nil {
		t.Fatal("E debe devolver cmd del editor externo")
	}
	if m.descEditOpen {
		t.Error("E no debe abrir el editor inline")
	}

	// Desde el detalle: E cierra el detalle y lanza el editor externo.
	m, _ = press(m, "enter")
	m, cmd = press(m, "E")
	if cmd == nil {
		t.Fatal("E en detalle debe devolver cmd del editor externo")
	}
	if m.detailOpen {
		t.Error("E debe cerrar el detalle")
	}
	if m.descEditOpen {
		t.Error("E no debe abrir el editor inline")
	}
}

// TestDescEditorCtrlCCopiesSelection verifica que Ctrl+C copia solo cuando hay
// selección (y no devuelve cmd si no la hay).
func TestDescEditorCtrlCCopiesSelection(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "e")
	m.descEditTextarea.SetValue("hola mundo")

	m, cmd := send(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Error("Ctrl+C sin selección no debe devolver cmd")
	}

	m.descEditTextarea.SelectAll()
	_, cmd = send(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C con selección debe devolver cmd de copiado")
	}
}

// TestDescEditorRendersInDetail verifica que el editor se integra en la caja
// del detalle (título y metadata visibles) sin desbordar el ancho.
func TestDescEditorRendersInDetail(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	m, _ = press(m, "e")

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Description:") {
		t.Errorf("el detalle editado no muestra la sección Description:\n%s", out)
	}
	if !strings.Contains(out, task.Title) {
		t.Errorf("el editor no debe tapar el título del detalle:\n%s", out)
	}
	if !strings.Contains(out, "Status:") {
		t.Errorf("el editor no debe tapar la metadata del detalle:\n%s", out)
	}
	// El textarea integrado no debe forzar líneas más anchas que la terminal.
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("línea de ancho %d supera el ancho %d: %q", w, m.width, line)
		}
	}
}
