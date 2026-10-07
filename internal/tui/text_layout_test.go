package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// These are the oldest layout functions of the program: they predate the rest
// and their tests only covered the happy path. They are all pure and they all
// have edges nobody looked at: zero width, text that just fits, and the
// limit smaller than the suffix.

// truncateLines truncates each line to the given width. It is what keeps the
// border helper from re-wrapping a line and adding extra rows.
func TestTruncateLines(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"truncates what does not fit", "abcdefghij", 4, "abcd"},
		{"truncates exactly what fits", "abcd", 4, "abcd"},
		{"one column too many", "abcde", 4, "abcd"},
		{"empty", "", 4, ""},
		{"one line", "a\nbb\nccc", 2, "a\nbb\ncc"},
		{"one-column width", "abc", 1, "a"},
		{"zero width does not truncate", "abc", 0, "abc"},
		{"negative width does not truncate", "abc", -5, "abc"},
		{"empty line between two", "ab\n\ncd", 3, "ab\n\ncd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateLines(tt.in, tt.width); got != tt.want {
				t.Errorf("truncateLines(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
			}
		})
	}
}

// The width is measured in display columns, not in bytes: the border glyphs
// are multibyte and a byte slice would split them in half.
func TestTruncateLinesCountsColumnsNotBytes(t *testing.T) {
	// "─" is three bytes and one column.
	line := strings.Repeat("─", 10) // 30 bytes
	got := truncateLines(line, 5)
	if want := strings.Repeat("─", 5); got != want {
		t.Errorf("truncateLines = %q (%d runes), want %q", got, len([]rune(got)), want)
	}
}

// And the ANSI codes do not count as width.
func TestTruncateLinesIgnoresAnsi(t *testing.T) {
	withColor := "\x1b[31mabc\x1b[0m"
	if got := truncateLines(withColor, 5); ansi.Strip(got) != "abc" {
		t.Errorf("with color: %q", ansi.Strip(got))
	}
	if got := truncateLines(withColor, 2); ansi.StringWidth(got) > 2 {
		t.Errorf("with color and truncation, the line measures %d", ansi.StringWidth(got))
	}
}

// It never returns a line wider than the limit, unless the limit is
// invalid, in which case it returns the original untouched.
func TestTruncateLinesRespectsWidth(t *testing.T) {
	for width := 1; width <= 12; width++ {
		for _, in := range []string{"a", "ab", "abcdefghijklmn", strings.Repeat("─", 30), "x\n" + strings.Repeat("y", 20)} {
			for _, line := range strings.Split(truncateLines(in, width), "\n") {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("truncateLines(%q, %d) returned a line of %d columns", in, width, w)
				}
			}
		}
	}
}

// cellWidth pads on the right up to the width and truncates on the left what
// exceeds it. With a width under one there is no cell, so it returns empty.
func TestCellWidth(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"pads", "ab", 5, "ab   "},
		{"exact", "abcde", 5, "abcde"},
		{"truncates", "abcdefgh", 5, "abc.."},
		{"empty is filled entirely", "", 3, "   "},
		// With a single column not even the ".." suffix fits, so the cell stays
		// blank instead of being cut mid-letter.
		{"width of one", "abc", 1, " "},
		{"zero width", "abc", 0, ""},
		{"negative width", "abc", -3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cellWidth(tt.in, tt.width); got != tt.want {
				t.Errorf("cellWidth(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
			}
		})
	}
}

// Every cell comes out with the requested width, measured in columns, except
// the invalid-width ones, which come out empty.
func TestCellWidthAlwaysFills(t *testing.T) {
	for width := 1; width <= 12; width++ {
		for _, in := range []string{"", "a", "abcdefghij", strings.Repeat("─", 20), "\x1b[31mab\x1b[0m"} {
			if got := ansi.StringWidth(cellWidth(in, width)); got != width {
				t.Errorf("cellWidth(%q, %d) measures %d, want %d", in, width, got, width)
			}
		}
	}
}

// truncate shortens leaving two dots at the end. Below the suffix size the
// ellipsis does not fit, so it cuts raw; and without that floor, s[:max-2] with
// max of 0 or 1 makes a slice with a negative index and blows up.
func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"truncates what does not fit", "abcdefghij", 5, "abc.."},
		{"truncates exactly what fits", "abcde", 5, "abcde"},
		{"one column too many", "abcdef", 5, "abc.."},
		{"empty", "", 5, ""},
		{"the limit is the suffix", "abc", 2, ".."},
		{"the limit is one dot", "abc", 1, "a"},
		{"zero limit", "abc", 0, ""},
		{"negative limit", "abc", -4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.in, tt.limit); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}

// It never returns more characters than requested, nor blows up with a
// negative limit.
func TestTruncateNeverExceedsLimit(t *testing.T) {
	for limit := -5; limit <= 12; limit++ {
		for _, in := range []string{"", "a", "abcdefghijklmn"} {
			got := truncate(in, limit)
			if limit <= 0 {
				if got != "" {
					t.Fatalf("truncate(%q, %d) = %q, want empty", in, limit, got)
				}
				continue
			}
			if len(got) > limit {
				t.Errorf("truncate(%q, %d) = %q (%d characters)", in, limit, got, len(got))
			}
		}
	}
}

// visibleRange already has loose cases in layout_test.go; what was missing were
// the edges and a sweep checking the three invariants at once.
func TestVisibleRangeEdges(t *testing.T) {
	tests := []struct {
		name                string
		cursor, total, size int
		wantStart           int
		wantEnd             int
	}{
		{"fits exactly", 0, 10, 10, 0, 10},
		{"cursor above the end", 99, 20, 5, 15, 20},
		{"negative cursor", -5, 20, 5, 0, 5},
		{"negative total", 0, -3, 5, 0, 0},
		{"zero size", 3, 20, 0, 0, 0},
		{"negative size", 3, 20, -2, 0, 0},
		{"size of one", 4, 10, 1, 4, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visibleRange(tt.cursor, tt.total, tt.size)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.cursor, tt.total, tt.size, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// The only thing the window guarantees is that it fits inside the total and
// that the cursor stays inside the window, when the size allows it.
func TestVisibleRangeProperties(t *testing.T) {
	for total := 0; total <= 30; total++ {
		for size := 0; size <= 30; size++ {
			for cursor := -5; cursor <= 35; cursor++ {
				start, end := visibleRange(cursor, total, size)

				if start < 0 || end < start {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): impossible range",
						cursor, total, size, start, end)
				}
				if end > total {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): it goes past the total",
						cursor, total, size, start, end)
				}
				if total <= 0 || size <= 0 {
					if start != 0 || end != 0 {
						t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d), want (0, 0)",
							cursor, total, size, start, end)
					}
					continue
				}
				if size < total && end-start != size {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): window of %d, want %d",
						cursor, total, size, start, end, end-start, size)
				}
				clamped := clampTo(cursor, total)
				if size < total && (clamped < start || clamped >= end) {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): the cursor ended up outside",
						cursor, total, size, start, end)
				}
			}
		}
	}
}
