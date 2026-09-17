package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// overlayKind identifica un modal activo que captura las teclas.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayDetail
	overlayNewTask
	overlayFilter
)

// KeybindsBar renderiza los keybinds en un pane con bordes.
type KeybindsBar struct {
	width   int
	view    viewKind
	overlay overlayKind
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

// SetOverlay indica si hay un modal activo que captura las teclas.
func (s *KeybindsBar) SetOverlay(o overlayKind) {
	s.overlay = o
}

// SetWidth actualiza el ancho.
func (s *KeybindsBar) SetWidth(w int) {
	s.width = w
}

// View renderiza el KeybindsBar con una o dos filas de keybinds.
func (s KeybindsBar) View() string {
	var content string
	if s.overlay != overlayNone {
		// Modal activo: solo las teclas que funcionan en ese contexto.
		content = strings.Join(s.renderOverlay(), "\n")
	} else {
		// Vista normal: fila global + filas específicas de la vista.
		content = s.renderGlobal() + "\n" + s.renderViewSpecific()
	}

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
		s.title(),
		content,
		s.width,
	)
}

// title devuelve el título del pane según el contexto activo.
func (s KeybindsBar) title() string {
	switch s.overlay {
	case overlayDetail:
		return " Keybinds · Detail "
	case overlayNewTask:
		return " Keybinds · New task "
	case overlayFilter:
		return " Keybinds · Filters "
	}
	return " Keybinds "
}

// renderOverlay formatea las teclas del modal activo.
func (s KeybindsBar) renderOverlay() []string {
	sep := styleStatusSep.Render(" · ")
	join := func(parts ...string) string { return strings.Join(parts, sep) }

	switch s.overlay {
	case overlayDetail:
		return []string{
			join(
				s.renderKey("c", "comment"),
				s.renderKey("j/k", "select"),
				s.renderKey("d", "delete/done"),
				s.renderKey("Esc", "close"),
			),
			join(
				s.renderKey("e", "edit"),
				s.renderKey("s", "start"),
				s.renderKey("x", "cancel"),
			),
		}
	case overlayNewTask:
		return []string{
			join(
				s.renderKey("Enter", "create"),
				s.renderKey("Esc", "cancel"),
				s.renderKey("Tab", "next field"),
				s.renderKey("←→/1-4", "priority"),
				s.renderKey("↑↓", "suggestions"),
			),
		}
	case overlayFilter:
		return []string{
			join(
				s.renderKey("Tab/↑↓", "field"),
				s.renderKey("←→", "change"),
				s.renderKey("Enter", "close"),
				s.renderKey("Esc", "cancel"),
			),
		}
	}
	return nil
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
			s.renderKey("Ctrl+p", "priority"),
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
			s.renderKey("Ctrl+p", "priority"),
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
