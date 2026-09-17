package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// KeybindsBar renderiza los keybinds en un pane con bordes.
type KeybindsBar struct {
	width   int
	view    viewKind
	focused bool
}

// NewKeybindsBar crea un nuevo KeybindsBar.
func NewKeybindsBar(width int) KeybindsBar {
	return KeybindsBar{width: width}
}

// SetView actualiza la vista activa.
func (s *KeybindsBar) SetView(v viewKind) {
	s.view = v
}

// SetWidth actualiza el ancho.
func (s *KeybindsBar) SetWidth(w int) {
	s.width = w
}

// View renderiza el KeybindsBar con dos filas de keybinds.
func (s KeybindsBar) View() string {
	// Row 1: global keybinds
	global := s.renderGlobal()

	// Row 2: view-specific keybinds
	viewSpecific := s.renderViewSpecific()

	content := global + "\n" + viewSpecific

	// Border color
	var borderFg color.Color
	if s.focused {
		borderFg = lipgloss.Color("12") // blue
	} else {
		borderFg = lipgloss.Color("8") // grey
	}

	return bordered.RenderWithTitleEx(
		lipgloss.RoundedBorder(),
		borderFg,
		bordered.AlignLeft,
		" Keybinds ",
		content,
		s.width,
	)
}

// renderGlobal formatea los keybinds globales.
func (s KeybindsBar) renderGlobal() string {
	parts := []string{
		s.renderKey("1/2/3", "Dash/List/Kanban"),
		s.renderKey("Tab", "switch"),
		s.renderKey("hjkl", "Arrows"),
		s.renderKey("H", "hidden"),
		s.renderKey("?", "help"),
		s.renderKey("q", "quit"),
	}
	return strings.Join(parts, styleStatusSep.Render(" · "))
}

// renderViewSpecific formatea los keybinds de la vista actual en dos filas.
func (s KeybindsBar) renderViewSpecific() string {
	var row1, row2 []string

	switch s.view {
	case viewDashboard:
		row1 = []string{
			s.renderKey("i", "insert task"),
		}
	case viewList:
		row1 = []string{
			s.renderKey("n/p N/P", "page nav"),
			s.renderKey("Enter", "detail"),
			s.renderKey("e", "edit"),
			s.renderKey("i", "insert task"),
			s.renderKey("/", "filter"),
		}
		row2 = []string{
			s.renderKey("s", "start"),
			s.renderKey("d", "done"),
			s.renderKey("x", "cancel"),
		}
	case viewKanban:
		row1 = []string{
			s.renderKey("s/S", "status"),
			s.renderKey("e", "edit"),
			s.renderKey("d", "done"),
			s.renderKey("x", "cancel"),
		}
		row2 = []string{
			s.renderKey("i", "insert task"),
			s.renderKey("Enter", "detail"),
		}
	}

	sep := styleStatusSep.Render(" · ")
	lines := strings.Join(row1, sep)
	if len(row2) > 0 {
		lines += "\n" + strings.Join(row2, sep)
	}
	return lines
}

// renderKey renderiza un par key+desc.
func (s KeybindsBar) renderKey(key, desc string) string {
	return styleStatusKey.Render(key) + " " + styleStatusDesc.Render(desc)
}
