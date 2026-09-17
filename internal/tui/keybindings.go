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
// Las teclas comunes (1/2/3, hjkl, H, ?, q) van primero por ser las más
// repetidas; después el resto por frecuencia de uso. La barra las reparte en
// filas de keybindsPerRow, así que bastan 7 entradas para llenar una fila.
func keybindsForView(v viewKind) []keybind {
	// Comunes: aplican en casi cualquier vista, pero se listan dentro de cada
	// una (no como fila "global") para poder omitirlas donde no aplican.
	common := []keybind{
		{"1/2/3/4", "Dash / List / Kanban / Gantt"},
		{"hjkl", "arrows"},
		{"H", "hidden"},
		{"?", "help"},
		{"q", "quit"},
	}

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
		}
	case viewList:
		viewKeys = []keybind{
			{"Enter", "detail"},
			{"i", "insert task"},
			{"s", "start"},
			{"d", "done"},
			{"x", "cancel"},
			{"e", "edit task"},
			{"/", "filters"},
			{"Ctrl+p", "priority"},
			{"n/p  N/P", "page nav"},
		}
	case viewKanban:
		viewKeys = []keybind{
			{"Tab", "next column"},
			{"s/S", "status"},
			{"i", "insert task"},
			{"Enter", "detail"},
			{"d", "done"},
			{"x", "cancel"},
			{"e", "edit task"},
			{"Ctrl+p", "priority"},
		}
	case viewGantt:
		viewKeys = []keybind{
			{"j/k", "task"},
			{"h/l", "move dates"},
			{"g/G", "first/last"},
			{"Enter", "detail"},
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
		m.currentView = viewDashboard
		return m, nil, true
	case "2":
		m.currentView = viewList
		return m, m.loadTasks(), true
	case "3":
		m.currentView = viewKanban
		return m, m.loadTasks(), true
	case "4":
		m.currentView = viewGantt
		m.ganttCursor = 0
		m.ganttOffsetDays = 0
		return m, tea.Batch(m.loadTasks(), m.loadOffDays()), true
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
