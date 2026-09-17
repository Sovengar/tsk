package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestKeybindsBarNormalViewShowsCommonAndViewKeys(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetView(viewList)

	out := ansi.Strip(kb.View())
	// Teclas comunes, ahora parte de la lista de la vista.
	if !strings.Contains(out, "quit") {
		t.Errorf("la vista normal debe mostrar las teclas comunes:\n%s", out)
	}
	if !strings.Contains(out, "insert task") {
		t.Errorf("la vista List debe mostrar sus keybinds:\n%s", out)
	}
	// List usa Tab para ciclar proyectos.
	if !strings.Contains(out, "cycle project") {
		t.Errorf("List debe mostrar Tab como cycle projects:\n%s", out)
	}
	if !strings.Contains(out, "Keybinds") {
		t.Errorf("falta el título del pane:\n%s", out)
	}
}

// TestKeybindsForViewCommonFirst verifica que las teclas antes globales
// encabezan la lista (son las más repetidas) en todas las vistas. En Gantt, H
// (hidden) no aplica y por tanto no se lista.
func TestKeybindsForViewCommonFirst(t *testing.T) {
	for _, v := range []viewKind{viewDashboard, viewList, viewKanban} {
		want := []string{"1/2/3/4", "hjkl", "H", "?", "q"}
		kbs := keybindsForView(v)
		for i, key := range want {
			if i >= len(kbs) || kbs[i].key != key {
				t.Fatalf("vista %s: keybind[%d].key = %q, want %q", v, i, kbs[i].key, key)
			}
		}
	}

	kbs := keybindsForView(viewGantt)
	want := []string{"1/2/3/4", "hjkl", "?", "q"}
	for i, key := range want {
		if i >= len(kbs) || kbs[i].key != key {
			t.Fatalf("vista Gantt: keybind[%d].key = %q, want %q", i, kbs[i].key, key)
		}
	}
	for _, kb := range kbs {
		if kb.key == "H" {
			t.Errorf("Gantt no debe listar H (hidden): %+v", kb)
		}
	}
}

// TestKeybindsBarMaxSevenPerRow verifica que ninguna fila supera 7 acciones.
func TestKeybindsBarMaxSevenPerRow(t *testing.T) {
	for _, v := range []viewKind{viewDashboard, viewList, viewKanban, viewGantt} {
		var kb KeybindsBar
		kb.SetWidth(200)
		kb.SetView(v)

		out := ansi.Strip(kb.View())
		for _, line := range strings.Split(out, "\n") {
			// 7 acciones => 6 separadores "·".
			if n := strings.Count(line, "·"); n > keybindsPerRow-1 {
				t.Errorf("vista %s: fila con más de %d acciones:\n%s", v, keybindsPerRow, line)
			}
		}
	}
}

// TestKeybindsBarDetailAllActions verifica que el detalle muestre todas sus
// acciones repartidas en filas de a lo sumo keybindsPerRow.
func TestKeybindsBarDetailAllActions(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(200)
	kb.SetView(viewList)
	kb.SetOverlay(overlayDetail)

	out := ansi.Strip(kb.View())
	for _, line := range strings.Split(out, "\n") {
		if n := strings.Count(line, "·"); n > keybindsPerRow-1 {
			t.Errorf("fila del detalle con más de %d acciones:\n%s", keybindsPerRow, line)
		}
	}
	for _, want := range []string{"j/k", "select comment", "new comment", "tags", "delete/done", "edit/editor", "start", "cancel", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en el detalle:\n%s", want, out)
		}
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
	// Las comunes y de la vista ya no aplican.
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
	m, _ = press(m, "1")

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
