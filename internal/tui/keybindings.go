package tui

import "charm.land/bubbletea/v2"

// viewKind es la vista activa.
type viewKind int

const (
	viewDashboard viewKind = iota
	viewList
	viewKanban
)

func (v viewKind) String() string {
	switch v {
	case viewDashboard:
		return "Dashboard"
	case viewList:
		return "List"
	case viewKanban:
		return "Kanban"
	}
	return "?"
}

func (v viewKind) next() viewKind {
	switch v {
	case viewDashboard:
		return viewList
	case viewList:
		return viewKanban
	default:
		return viewDashboard
	}
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
		return m, nil, true
	case "?":
		m.helpOpen = !m.helpOpen
		return m, nil, true
	}
	return m, nil, false
}
