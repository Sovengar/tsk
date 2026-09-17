package tui

import "charm.land/bubbletea/v2"

// keybind es un par tecla/descripción mostrado en la barra y en la ayuda.
type keybind struct {
	key  string
	desc string
}

// viewKind es la vista activa.
type viewKind int

const (
	viewDashboard viewKind = iota
	viewList
	viewKanban
	viewGantt
)

func (v viewKind) String() string {
	switch v {
	case viewDashboard:
		return "Dashboard"
	case viewList:
		return "Tasklist"
	case viewKanban:
		return "Kanban"
	case viewGantt:
		return "Gantt"
	}
	return "?"
}

// keybindsForView devuelve, en orden de prioridad, las teclas de una vista. Es
// la única fuente de verdad para la KeybindsBar y el modal de ayuda.
//
// Las teclas comunes (1/2/3/4, hjkl, H, ?, q) van primero por ser las más
// repetidas; después el resto por frecuencia de uso. La barra las reparte en
// filas de keybindsPerRow, así que bastan 7 entradas para llenar una fila.
func keybindsForView(v viewKind) []keybind {
	// Comunes: aplican en casi cualquier vista, pero se listan dentro de cada
	// una (no como fila "global") para poder omitirlas donde no aplican.
	common := []keybind{
		{"1/2/3/4", "List / Kanban / Gantt / Dash"},
		{"hjkl", "arrows"},
	}
	// H oculta done/cancelled; en Gantt no aplica porque la proyección ya los
	// excluye, así que se omite para no mostrar una tecla muerta.
	if v != viewGantt {
		common = append(common, keybind{"H", "hidden"})
	}
	common = append(common, keybind{"?", "help"}, keybind{"q", "quit"})

	var viewKeys []keybind
	switch v {
	case viewDashboard:
		viewKeys = []keybind{
			{"Tab", "cycle projects"},
			{"i", "Insert new project"},
			{"e", "edit project"},
			{"d", "archive project"},
			{"r", "restore project"},
			{"A", "archived"},
			{"m", "assignees"},
		}
	case viewList:
		viewKeys = []keybind{
			{"Tab", "cycle projects"},
			{"Enter", "detail"},
			{"i", "insert task"},
			{"s", "start"},
			{"d", "done"},
			{"x", "cancel"},
			{"e/E", "edit/editor"},
			{"/", "filters"},
			{"Ctrl+p", "priority"},
			{"n/p  N/P", "page nav"},
		}
	case viewKanban:
		viewKeys = []keybind{
			{"Tab", "cycle projects"},
			{"s/S", "status"},
			{"i", "insert task"},
			{"Enter", "detail"},
			{"d", "done"},
			{"x", "cancel"},
			{"e/E", "edit/editor"},
			{"Ctrl+p", "priority"},
			{"/", "filters"},
		}
	case viewGantt:
		viewKeys = []keybind{
			{"Tab", "cycle projects"},
			{"g/G", "first/last"},
			{"Enter", "detail"},
			{"/", "filters"},
		}
	}
	return append(common, viewKeys...)
}

// handleGlobalKeys procesa teclas que funcionan en todas las vistas.
// Devuelve true si la tecla fue manejada.
func handleGlobalKeys(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit, true
	case "1":
		m.currentView = viewList
		return m, m.loadTasks(), true
	case "2":
		m.currentView = viewKanban
		return m, m.loadTasks(), true
	case "3":
		m.currentView = viewGantt
		m.ganttCursor = 0
		m.ganttOffsetDays = 0
		m.snapGanttCursor()
		return m, tea.Batch(m.loadTasks(), m.loadOffDays()), true
	case "4":
		m.currentView = viewDashboard
		return m, nil, true
	case "esc":
		if m.helpOpen {
			m.helpOpen = false
			return m, nil, true
		}
		if m.filterOpen {
			m.filterOpen = false
			return m, nil, true
		}
		if m.filterActive {
			m.filterActive = false
			m.filterText = ""
			return m, nil, true
		}
	case "H":
		// H oculta done/cancelled en List/Kanban. En Gantt no aplica (la
		// proyección ya las excluye), así que la tecla queda deshabilitada.
		if m.currentView == viewGantt {
			return m, nil, true
		}
		m.filterActiveOnly = !m.filterActiveOnly
		m.invalidateFilterCache()
		m.clampKanbanCursor()
		return m, nil, true
	case "?":
		m.helpOpen = !m.helpOpen
		return m, nil, true
	}
	return m, nil, false
}
