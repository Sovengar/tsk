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
	overlayTag
	overlayNewTask
	overlayFilter
	overlayProject
	overlayConfirm
	overlayDescEdit
	overlayAssignee
	overlayAssigneeDetail
	overlayOffdayForm
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

// View renderiza el KeybindsBar con las filas del contexto activo. No existe el
// concepto de keybind "global": cada vista u overlay define su lista completa.
func (s KeybindsBar) View() string {
	var content string
	if s.overlay != overlayNone {
		// Modal activo: solo las teclas que funcionan en ese contexto.
		content = strings.Join(s.renderOverlay(), "\n")
	} else {
		content = strings.Join(s.renderView(), "\n")
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
	case overlayTag:
		return " Keybinds · Tags "
	case overlayNewTask:
		return " Keybinds · New task "
	case overlayFilter:
		return " Keybinds · Filters "
	case overlayProject:
		return " Keybinds · Project "
	case overlayConfirm:
		return " Keybinds · Confirm "
	case overlayDescEdit:
		return " Keybinds · Description "
	case overlayAssignee:
		return " Keybinds · Assignees "
	case overlayAssigneeDetail:
		return " Keybinds · Assignee "
	case overlayOffdayForm:
		return " Keybinds · Off-day "
	}
	return " Keybinds "
}

// keybindsPerRow es el máximo de acciones por fila en la KeybindsBar. El
// sobrante salta a la fila siguiente; se añaden tantas filas como haga falta.
// Mismo reparto para vistas y overlays.
const keybindsPerRow = 7

// renderRows reparte una lista priorizada de keybinds en filas de
// keybindsPerRow acciones como máximo.
func (s KeybindsBar) renderRows(kbs []keybind) []string {
	sep := styleStatusSep.Render(" · ")
	var out []string
	for i := 0; i < len(kbs); i += keybindsPerRow {
		end := min(i+keybindsPerRow, len(kbs))
		parts := make([]string, 0, end-i)
		for _, kb := range kbs[i:end] {
			parts = append(parts, s.renderKey(kb.key, kb.desc))
		}
		out = append(out, strings.Join(parts, sep))
	}
	return out
}

// renderView formatea las filas de keybinds de la vista actual a partir de la
// definición única en keybindsForView.
func (s KeybindsBar) renderView() []string {
	return s.renderRows(keybindsForView(s.view))
}

// renderOverlay formatea las teclas del modal activo, con el mismo reparto en
// filas de keybindsPerRow.
func (s KeybindsBar) renderOverlay() []string {
	switch s.overlay {
	case overlayDetail:
		return s.renderRows(detailKeybinds())
	case overlayTag:
		return s.renderRows([]keybind{
			{"Enter", "toggle tag"},
			{"↑↓", "suggestion"},
			{"Tab", "complete"},
			{"Esc", "close"},
		})
	case overlayNewTask:
		return s.renderRows([]keybind{
			{"Enter", "create"},
			{"Esc", "cancel"},
			{"Tab", "next field"},
			{"←→/1-4", "priority"},
			{"↑↓", "suggestions"},
		})
	case overlayFilter:
		return s.renderRows([]keybind{
			{"Tab/↑↓", "field"},
			{"←→", "change"},
			{"Enter", "close"},
			{"Esc", "cancel"},
		})
	case overlayProject:
		return s.renderRows([]keybind{
			{"Enter", "save"},
			{"Esc", "cancel"},
			{"Tab", "next field"},
		})
	case overlayConfirm:
		return s.renderRows([]keybind{
			{"y", "confirm"},
			{"n/Esc", "cancel"},
		})
	case overlayDescEdit:
		return s.renderRows([]keybind{
			{"Ctrl+S", "save"},
			{"Ctrl+C", "copy"},
			{"Ctrl+V", "paste"},
			{"Esc", "cancel"},
			{"Enter", "newline"},
		})
	case overlayAssignee:
		return s.renderRows([]keybind{
			{"Enter", "open"},
			{"a", "add off-day"},
			{"j/k", "move"},
			{"Esc", "close"},
		})
	case overlayAssigneeDetail:
		return s.renderRows([]keybind{
			{"a", "add off-day"},
			{"d/x", "delete off-day"},
			{"j/k", "move"},
			{"Esc", "back"},
		})
	case overlayOffdayForm:
		return s.renderRows([]keybind{
			{"Enter", "save"},
			{"Tab", "next field"},
			{"Esc", "cancel"},
		})
	}
	return nil
}

// renderKey renderiza un par key+desc.
func (s KeybindsBar) renderKey(key, desc string) string {
	return styleStatusKey.Render(key) + " " + styleStatusDesc.Render(desc)
}
