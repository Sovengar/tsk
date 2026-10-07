package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The help modal is centered over the content. The position and the width
// matter: a different preferred width, or a total with the borders miscounted,
// shifts the box several columns and it does not show in a quick screenshot.
//
// The expected numbers are literals, not derived from helpModalWidth. Deriving
// the expectation from the constant turns it into a tautology: if the constant
// changes, the test follows it and passes.

// helpBox returns the top border of the help modal's box trimmed to its
// width, and the column where it starts.
//
// The background is made of lines as wide as the screen because OverlayLine
// pastes the modal right where the background ends when it is shorter: with a
// narrow background the box would stick to the text and the measured position
// would say nothing about the centering.
func helpBox(t *testing.T, m *Model) (box string, col int) {
	t.Helper()
	background := make([]string, 40)
	for i := range background {
		background[i] = strings.Repeat("·", m.width)
	}

	out := ansi.Strip(m.renderHelpModal(strings.Join(background, "\n")))
	for _, line := range strings.Split(out, "\n") {
		// Runes, not bytes: both the background dot and the box corner are
		// multibyte, and strings.Index would give the position in bytes. With
		// two-byte dots, the box looked like it was at double its column.
		runes := []rune(line)
		for i, r := range runes {
			if r != '╭' {
				continue
			}
			width := 48 // the preferred inner width plus the two borders
			if m.width < width {
				width = m.width
			}
			if i+width > len(runes) {
				t.Fatalf("the box goes out of the line at column %d: %q", i, line)
			}
			return string(runes[i : i+width]), i
		}
	}
	t.Fatalf("the help modal did not draw any box:\n%s", out)
	return "", 0
}

// On wide screens the box is centered: 46 inside plus 2 borders are 48,
// and on 120 columns there is (120-48)/2 = 36 to spare on each side.
func TestHelpModalCentred(t *testing.T) {
	tests := []struct {
		name            string
		width           int
		wantCol         int
		wantBoxWidth    int
		skipIfTruncates bool
	}{
		// Plenty of room: a box of 48, centered.
		{"very wide", 120, 36, 48, false},
		{"wide", 60, 6, 48, false},
		// Odd parity: with 49 columns, a total of 47 would give startX 1 and one
		// of 48 would give 0. This row is what distinguishes the +2 from the +1.
		{"odd exact", 49, 0, 48, false},
		// The preferred width no longer fits: modalWidthFor leaves w-2 and the
		// borders spend them, so the box reaches the edge with no margin.
		{"exact", 48, 0, 48, false},
		{"one less", 47, 0, 47, false},
		{"narrow", 30, 0, 30, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.width = tt.width

			box, col := helpBox(t, m)
			if got := len([]rune(box)); got != tt.wantBoxWidth {
				t.Errorf("at %d columns the box measures %d, want %d", tt.width, got, tt.wantBoxWidth)
			}
			if col != tt.wantCol {
				t.Errorf("the box starts at column %d, want %d", col, tt.wantCol)
			}
		})
	}
}

// The background shows on both sides of the box: the modal overlays it, it does not erase it.
func TestHelpModalKeepsBackgroundOnBothSides(t *testing.T) {
	m := newTestModel(t)
	m.width = 120
	background := make([]string, 40)
	for i := range background {
		background[i] = strings.Repeat("·", m.width)
	}

	out := ansi.Strip(m.renderHelpModal(strings.Join(background, "\n")))
	for _, line := range strings.Split(out, "\n") {
		runes := []rune(line)
		for i, r := range runes {
			if r != '╭' {
				continue
			}
			left := runes[:i]
			right := runes[i+48:]
			if n := countDot(left); n != 36 {
				t.Errorf("to the left of the box there are %d dots, want 36", n)
			}
			if n := countDot(right); n != 36 {
				t.Errorf("to the right of the box there are %d dots, want 36", n)
			}
			return
		}
	}
	t.Fatal("the help modal did not draw any box")
}

func countDot(runes []rune) int {
	n := 0
	for _, r := range runes {
		if r == '·' {
			n++
		}
	}
	return n
}
