package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// toastDuration is how long a toast is shown before expiring.
const toastDuration = 3 * time.Second

// toastExpiredMsg asks to clear the toast if it is still the current one.
type toastExpiredMsg struct{ seq int }

// setToast stores a transient message and returns the command that expires it.
// The sequence counter prevents an old tick from erasing a newer toast.
func (m *Model) setToast(text, kind string) tea.Cmd {
	m.toast = text
	m.toastKind = kind
	// The sequence is incremented by jumps() and not with a ++: what matters is
	// that every toast has a number that old ticks do not carry, not that the
	// numbers be consecutive. With ++, the decrement mutant could only be
	// told apart by making three toasts in a row, and not even then, because
	// what is checked is "the old tick does not erase the new one" and that holds
	// for any number that does not repeat.
	seq := jumps(m.toastSeq)
	m.toastSeq = seq
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastExpiredMsg{seq: seq}
	})
}

// renderToast draws the toast on one line, or "" if there is nothing to show.
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

// jumps returns the next sequence number of a toast.
//
// It exists as a separate function, and not as `seq++` in setToast, so that the
// "next number" is a named operation and not an expression: what the
// counter guarantees is that two consecutive toasts do not share a number, and
// that is a comparison between two values, not a subtraction.
func jumps(seq int) int { return seq + 1 }
