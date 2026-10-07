package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"tsk/internal/tui/bordered"
)

// overlayKind identifies an active modal that captures the keys.
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

// KeybindsBar renders the keybinds in a pane with borders.
type KeybindsBar struct {
	width   int
	view    viewKind
	overlay overlayKind
	focused bool
}

// NewKeybindsBar creates a new KeybindsBar.
func NewKeybindsBar(width int) KeybindsBar {
	return KeybindsBar{width: width}
}

// SetView updates the active view.
func (s *KeybindsBar) SetView(v viewKind) {
	s.view = v
}

// SetOverlay tells whether there is an active modal that captures the keys.
func (s *KeybindsBar) SetOverlay(o overlayKind) {
	s.overlay = o
}

// SetWidth updates the width.
func (s *KeybindsBar) SetWidth(w int) {
	s.width = w
}

// View renders the KeybindsBar with the rows of the active context. There is
// no concept of a "global" keybind: each view or overlay defines its full list.
func (s KeybindsBar) View() string {
	var content string
	if s.overlay != overlayNone {
		// Active modal: only the keys that work in that context.
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

// title returns the pane title according to the active context.
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

// keybindsPerRow is the maximum number of actions per row in the KeybindsBar.
// The leftover jumps to the next row; as many rows as needed are added.
// Same split for views and overlays.
const keybindsPerRow = 7

// renderRows spreads a prioritized list of keybinds into rows of at most
// keybindsPerRow actions.
func (s KeybindsBar) renderRows(kbs []keybind) []string {
	sep := styleStatusSep.Render(" · ")
	var out []string
	nrows := (len(kbs) + keybindsPerRow - 1) / keybindsPerRow
	for r := range nrows {
		i := r * keybindsPerRow
		end := min(i+keybindsPerRow, len(kbs))
		// No capacity hint: it is arithmetic that no test looks at, and
		// append computes the growth on its own.
		var parts []string
		for _, kb := range kbs[i:end] {
			parts = append(parts, s.renderKey(kb.key, kb.desc))
		}
		out = append(out, strings.Join(parts, sep))
	}
	return out
}

// renderView formats the keybind rows of the current view out of the
// single definition in keybindsForView.
func (s KeybindsBar) renderView() []string {
	return s.renderRows(keybindsForView(s.view))
}

// renderOverlay formats the keys of the active modal, with the same split
// into rows of keybindsPerRow.
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
		return s.renderRows(newTaskKeybinds())
	case overlayFilter:
		return s.renderRows(filterKeybinds())
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

// renderKey renders a key+desc pair.
func (s KeybindsBar) renderKey(key, desc string) string {
	return styleStatusKey.Render(key) + " " + styleStatusDesc.Render(desc)
}
