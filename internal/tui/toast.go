package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// toastDuration es cuánto tiempo se muestra un toast antes de expirar.
const toastDuration = 3 * time.Second

// toastExpiredMsg solicita limpiar el toast si sigue siendo el actual.
type toastExpiredMsg struct{ seq int }

// setToast guarda un mensaje transitorio y devuelve el comando que lo expira.
// El contador de secuencia evita que un tick viejo borre un toast más nuevo.
func (m *Model) setToast(text, kind string) tea.Cmd {
	m.toast = text
	m.toastKind = kind
	m.toastSeq++
	seq := m.toastSeq
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastExpiredMsg{seq: seq}
	})
}

// renderToast dibuja el toast en una línea, o "" si no hay nada que mostrar.
func (m Model) renderToast() string {
	if m.toast == "" {
		return ""
	}
	style := styleStatusActive
	if m.toastKind == "error" {
		style = styleError
	}
	text := m.toast
	if m.width > 4 {
		text = ansi.Truncate(text, m.width-2, "…")
	}
	return style.Render(" " + text)
}
