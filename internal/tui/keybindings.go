package tui

import "charm.land/bubbletea/v2"

// keybind is a key/description pair shown in the bar and in the help.
type keybind struct {
	key  string
	desc string
}

// viewKind is the active view.
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

// keybindsForView returns, in priority order, the keys of a view. It is
// the single source of truth for the KeybindsBar and the help modal.
//
// The common keys (1/2/3/4, hjkl, ?, q) come first for being the most
// repeated; then the rest by frequency of use. The bar spreads them over
// rows of keybindsPerRow, so 7 entries are enough to fill a row.
func keybindsForView(v viewKind) []keybind {
	// Common: they apply in almost any view, but they are listed inside each
	// one (not as a "global" row) so they can be omitted where they do not apply.
	common := []keybind{
		{"1/2/3/4", "List / Kanban / Gantt / Dash"},
		{"hjkl", "arrows"},
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

// handleGlobalKeys processes keys that work in every view.
// Returns true if the key was handled.
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
		// The help and the open filter close their own modal, and the active
		// filter clears it here. The branch that closed filterOpen was here for
		// symmetry with the other two, but handleKey routes to the filter modal
		// BEFORE the global keys, so it never arrived with the filter
		// open: it was dead code.
		if m.helpOpen {
			m.helpOpen = false
			return m, nil, true
		}
		if m.filterActive {
			m.filterActive = false
			m.filterText = ""
			return m, nil, true
		}
	case "?":
		m.helpOpen = !m.helpOpen
		return m, nil, true
	}
	return m, nil, false
}
