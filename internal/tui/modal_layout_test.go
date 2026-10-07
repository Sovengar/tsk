package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// modalWidthFor and overlayModal are the base of all modals: the first one
// decides how wide they are and the second where they land. The tests that
// existed used them in passing, so their edges -- the exact threshold and the
// fill when the modal is taller than the background -- were covered by nobody.

func TestModalWidthFor(t *testing.T) {
	tests := []struct {
		name         string
		preferred, w int
		want         int
	}{
		{"pref fits with room to spare", 52, 120, 52},
		{"pref exactly at the limit", 58, 60, 58},
		{"pref one too many", 59, 60, 58},
		{"does not fit", 52, 40, 38},
		{"two-column screen", 52, 2, 0},
		{"three-column screen", 52, 3, 1},
		{"one-column screen", 52, 1, -1},
		{"pref of zero", 0, 40, 0},
		{"negative pref", -5, 40, -5},
		{"negative screen", 52, -10, -12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modalWidthFor(tt.preferred, tt.w); got != tt.want {
				t.Errorf("modalWidthFor(%d, %d) = %d, want %d", tt.preferred, tt.w, got, tt.want)
			}
		})
	}
}

// The threshold is "the modal plus its two borders has to fit": at w-2
// screen columns the preferred modal fits exactly, and one more no longer does.
func TestModalWidthForThreshold(t *testing.T) {
	const pref = 52
	if got := modalWidthFor(pref, pref+2); got != pref {
		t.Errorf("at %d columns the modal is %d, want %d", pref+2, got, pref)
	}
	if got := modalWidthFor(pref, pref+1); got != pref+1-2 {
		t.Errorf("at %d columns the modal is %d, want %d", pref+1, got, pref-1)
	}
}

// overlayModal centers the modal and leaves the background around it. With a
// background of the same height as the modal, the vertical one goes unnoticed; with a
// taller one, the modal goes in the middle.
func TestOverlayModalCentresVertically(t *testing.T) {
	modal := strings.Join([]string{"╭──╮", "│  │", "╰──╯"}, "\n") // 3 lines

	for _, background := range []struct {
		name    string
		height  int
		wantTop int
	}{
		{"same height, no padding", 3, 0},
		{"one too many", 4, 0},
		{"two too many", 5, 1},
		{"four too many", 7, 2},
		{"ten too many", 13, 5},
	} {
		t.Run(background.name, func(t *testing.T) {
			lines := make([]string, background.height)
			for i := range lines {
				lines[i] = strings.Repeat("·", 40)
			}

			out := strings.Split(overlayModal(strings.Join(lines, "\n"), modal, 4, 40), "\n")
			if len(out) != background.height {
				t.Fatalf("%d lines came out, want %d", len(out), background.height)
			}
			first := -1
			for i, l := range out {
				if strings.Contains(l, "╭") {
					first = i
					break
				}
			}
			if first != background.wantTop {
				t.Errorf("the modal starts at line %d, want %d", first, background.wantTop)
			}
		})
	}
}

// With a background shorter than the modal, the background is padded so the
// modal fits whole. Without that padding the modal would go out of the lines.
func TestOverlayModalPadsShortContent(t *testing.T) {
	modal := strings.Join([]string{"╭──╮", "│  │", "╰──╯"}, "\n")

	out := strings.Split(overlayModal(strings.Repeat("·", 40), modal, 4, 40), "\n")
	if len(out) != 3 {
		t.Fatalf("%d lines came out, want 3", len(out))
	}
	for i, l := range out {
		if !strings.Contains(l, "╭") && !strings.Contains(l, "│") && !strings.Contains(l, "╰") {
			t.Errorf("line %d does not have the modal: %q", i, l)
		}
	}
}

// And with a modal taller than an empty background, the padding is blank lines, not
// dots: the background had nothing to preserve.
func TestOverlayModalPadsEmptyContent(t *testing.T) {
	modal := strings.Join([]string{"╭──╮", "│  │", "╰──╯"}, "\n")

	out := strings.Split(overlayModal("", modal, 4, 40), "\n")
	if len(out) < 3 {
		t.Fatalf("%d lines came out, want at least 3", len(out))
	}
}

// The modal is also centered horizontally, leaving background on both sides.
func TestOverlayModalCentresHorizontally(t *testing.T) {
	modal := "╭──╮" // 4 columns
	background := strings.Repeat("·", 40)

	// No ANSI: the reset codes OverlayLine interleaves count as runes but do
	// not take a column, so they have to be removed before measuring.
	out := strings.Split(ansi.Strip(overlayModal(background, modal, 4, 40)), "\n")
	if len(out) != 1 {
		t.Fatalf("%d lines came out, want 1", len(out))
	}
	// Column, not byte: "·" is two bytes and "╭" three, so strings.Index would
	// give a position different from the one it occupies on screen.
	i := len([]rune(out[0][:strings.Index(out[0], "╭")]))
	if i != 18 {
		t.Errorf("the modal starts at column %d, want 18 (centered in 40)", i)
	}
	if strings.Count(out[0], "·") != 36 {
		t.Errorf("%d background columns remain, want 36", strings.Count(out[0], "·"))
	}
}

// A modal wider than the screen sticks to the edge instead of starting at a
// negative column.
func TestOverlayModalWiderThanScreen(t *testing.T) {
	modal := strings.Repeat("x", 60)

	out := ansi.Strip(overlayModal(strings.Repeat("·", 20), modal, 60, 20))
	if !strings.HasPrefix(out, "x") {
		t.Errorf("a modal wider than the screen does not start at the beginning: %.20q", out)
	}
	if strings.Contains(out, "·") {
		t.Errorf("background remained inside a modal that covers it entirely: %.40q", out)
	}
}
