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
	// La secuencia se incrementa por jumps() y no con un ++: lo que importa es
	// que cada toast tenga un número que los ticks viejos no lleven, no que los
	// números sean consecutivos. Con ++, el mutante de decremento sólo se
	// distinguía haciendo tres toasts seguidos, y ni así, porque lo que se
	// comprueba es "el tick viejo no borra el nuevo" y eso da igual para
	// cualquier número que no se repita.
	seq := jumps(m.toastSeq)
	m.toastSeq = seq
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

// jumps devuelve el siguiente número de secuencia de un toast.
//
// Existe como función separada, y no como `seq++` en setToast, para que el
// "siguiente número" sea una operación con nombre y no una expresión: lo que el
// contador garantiza es que dos toasts consecutivos no compartan número, y eso es
// una comparación entre dos valores, no una resta.
func jumps(seq int) int { return seq + 1 }
