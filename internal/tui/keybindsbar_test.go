package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestKeybindsBarNormalViewShowsGlobal(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetView(viewList)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "quit") {
		t.Errorf("la vista normal debe mostrar keybinds globales:\n%s", out)
	}
	if !strings.Contains(out, "insert task") {
		t.Errorf("la vista List debe mostrar sus keybinds:\n%s", out)
	}
	if !strings.Contains(out, "Keybinds") {
		t.Errorf("falta el título del pane:\n%s", out)
	}
}

func TestKeybindsBarDetailOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetView(viewList)
	kb.SetOverlay(overlayDetail)

	out := ansi.Strip(kb.View())
	// Solo teclas del modal de detalle.
	for _, want := range []string{"comment", "select", "delete/done", "edit", "start", "cancel", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en el overlay de detalle:\n%s", want, out)
		}
	}
	// Las globales y de la vista ya no aplican.
	for _, unwanted := range []string{"quit", "insert task", "filter"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("no debería mostrarse %q en el detalle:\n%s", unwanted, out)
		}
	}
	if !strings.Contains(out, "Keybinds · Detail") {
		t.Errorf("el título debe indicar el contexto Detail:\n%s", out)
	}
}

func TestKeybindsBarNewTaskOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetOverlay(overlayNewTask)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "create") || strings.Contains(out, "quit") {
		t.Errorf("overlay de nueva tarea incorrecto:\n%s", out)
	}
}

func TestKeybindsBarFilterOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetOverlay(overlayFilter)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "change") || strings.Contains(out, "quit") {
		t.Errorf("overlay de filtros incorrecto:\n%s", out)
	}
}

func TestOverlayKindPriority(t *testing.T) {
	m := newTestModel(t)

	if m.overlayKind() != overlayNone {
		t.Errorf("sin modales: %v, want overlayNone", m.overlayKind())
	}

	m.detailOpen = true
	if m.overlayKind() != overlayDetail {
		t.Errorf("con detalle: %v, want overlayDetail", m.overlayKind())
	}

	// La nueva tarea tiene prioridad sobre el detalle (igual que handleKey).
	m.newTaskOpen = true
	m.filterOpen = true
	if m.overlayKind() != overlayNewTask {
		t.Errorf("con nueva tarea: %v, want overlayNewTask", m.overlayKind())
	}

	m.newTaskOpen = false
	if m.overlayKind() != overlayDetail {
		t.Errorf("detalle antes que filtros: %v, want overlayDetail", m.overlayKind())
	}

	m.detailOpen = false
	if m.overlayKind() != overlayFilter {
		t.Errorf("con filtros: %v, want overlayFilter", m.overlayKind())
	}
}

// TestDetailOpenSwitchesKeybinds verifica que abrir el detalle cambia el
// contexto de la barra de keybinds.
func TestDetailOpenSwitchesKeybinds(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")

	if m.overlayKind() != overlayNone {
		t.Fatalf("antes de abrir: %v, want overlayNone", m.overlayKind())
	}

	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Fatal("enter debe abrir el detalle")
	}
	if m.overlayKind() != overlayDetail {
		t.Errorf("con el detalle abierto: %v, want overlayDetail", m.overlayKind())
	}
}
