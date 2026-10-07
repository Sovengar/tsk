package tui

import (
	"strings"
	"testing"
)

// separatorWidth is the width of the separator that goes under the filter bar:
// the inner width minus the two columns of the box, with a floor at zero
// because strings.Repeat blows up with a negative number.
//
// The floor at 0 is what makes two of its comparisons equivalent:
//
//	max(innerW, 0) vs max(innerW, 1)	-- the floor's "> 0"
//	max(innerW, 0) vs max(innerW-1, 0)	-- the floor's "-1"
//
// In the three cases, for the only value that can change the result, the
// separator comes out empty either way: a line with no dashes is
// visually the same as one with a dash. That is why a width is needed where
// the separator DOES have dashes, and its LENGTH compared.

func TestSeparatorWidthNeverGoesNegative(t *testing.T) {
	for _, w := range []int{-5, -1, 0, 1, 2, 3} {
		got := separatorWidth(w)
		if got < 0 {
			t.Errorf("separatorWidth(%d) = %d, negative: strings.Repeat blows up on that", w, got)
		}
	}
	// With the exact width for one dash and nothing more, one dash comes out; with less, zero.
	if got := separatorWidth(3); got != 1 {
		t.Errorf("separatorWidth(3) = %d, want 1", got)
	}
	if got := separatorWidth(2); got != 0 {
		t.Errorf("separatorWidth(2) = %d, want 0: there is not even one dash to draw", got)
	}
}

// The header's separator has to measure what the width says, not one column
// more nor one less. That is what tells w-2 from w+2 in the call that draws
// it, and it is what no test looked at: it was checked that the filter bar
// came out, not how much space the line below took.
func TestHeaderSeparatorMeasuresTheWidth(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120} {
		t.Run("width="+itoa(width), func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = viewList
			m.filterStatus = ""

			header := stripANSI(m.renderFilterHeader(width - 2))
			lines := strings.Split(header, "\n")

			// The separator is the line that starts with dashes. Lipgloss
			// pads it with spaces up to the width of the bar above, so
			// the line is longer than the separator: what counts is
			// where the dashes end.
			dashes := countDashes(lines)
			if dashes == 0 {
				t.Fatalf("the header has no separator: %q", header)
			}

			want := separatorWidth(width - 2)
			if dashes != want {
				t.Errorf("the separator measures %d dashes, want %d (the width passed minus the two of the box)",
					dashes, want)
			}
		})
	}
}

// The separator inside the kanban box. Its width is set by the truncateLines
// that comes after, not the w-2 of the call: that is why the
// `renderFilterHeader(w+2)` of the kanban is an equivalent mutant, and this
// test leaves on record that the final width is the right one and not the call's.
func TestKanbanSeparatorMeasuresTheVisibleWidth(t *testing.T) {
	for _, width := range []int{60, 120, 200} {
		t.Run("w="+itoa(width), func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = viewKanban
			m.width, m.height = width, 30

			lines := strings.Split(stripANSI(m.renderKanban(28)), "\n")

			// The second line with dashes is the separator: the first one is
			// the top border and the third one is already the board.
			var separator string
			for _, l := range lines {
				// Box interior made only of dashes: neither the top border
				// nor the board's cells meet that.
				interior := strings.Trim(l, "|│ ")
				if interior != "" && strings.Trim(interior, "─") == "" {
					separator = interior
					break
				}
			}
			if separator == "" {
				t.Fatalf("the kanban has no filter separator:\n%s", strings.Join(lines, "\n"))
			}

			// The separator fills the inner width: two borders and the dashes.
			// With a w+2 in the call, the truncateLines after it leaves it
			// the same, and that is what makes the mutant equivalent.
			want := separatorWidth(width - 2)
			if got := len([]rune(separator)); got != want {
				t.Errorf("the separator measures %d dashes, want %d (the inner width minus nothing: %q)",
					got, want, separator)
			}
		})
	}
}

// countDashes counts the dashes of the separator: the header line that starts
// with them. Lipgloss pads it with spaces up to the width of the filter bar
// above, so the line is longer than the separator and only the dashes have to
// be counted, not the runes.
func countDashes(lines []string) int {
	for _, l := range lines {
		dashes := 0
		for _, r := range l {
			if r == '─' {
				dashes++
				continue
			}
			break
		}
		if dashes > 0 {
			return dashes
		}
	}
	return 0
}

// visibleRange with total and size of 1: the edge that makes `< 1` reachable.
//
// With `total <= 0` the equivalent mutant was `total < 1` and vice versa,
// because on integers they are the same condition. `< 1` has an edge that does
// exist -- the 1 -- and that 1 is a real case: a list with one element and a
// window containing it.
// The four cases of the corner [0|1] x [0|1] are checked, plus the interior.
func TestVisibleRangeAtCornerZeroAndOne(t *testing.T) {
	cases := []struct {
		name                string
		cursor, total, size int
		wantStart, wantEnd  int
	}{
		{"0 of 0", 0, 0, 0, 0, 0},
		{"0 of 1", 0, 0, 1, 0, 0},
		{"1 of 0", 0, 1, 0, 0, 0},
		{"1 of 1", 0, 1, 1, 0, 1},
		{"1 of 2 with window 1", 0, 1, 2, 0, 1}, // the window is truncated to the total
		{"1 of 5 with window 1", 3, 1, 5, 0, 1},
		{"the interior", 2, 5, 2, 1, 3},
		{"negatives", 0, -1, 5, 0, 0},
		{"negative size", 0, 5, -1, 0, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end := visibleRange(c.cursor, c.total, c.size)
			if start != c.wantStart || end != c.wantEnd {
				t.Errorf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)",
					c.cursor, c.total, c.size, start, end, c.wantStart, c.wantEnd)
			}
		})
	}
}

// The case total == size: the window is exactly the whole list. No guard is
// needed for it -- visibleRange truncates it to the total and returns the
// whole page -- and that is why listWindowForHeight no longer has that `if`.
func TestListWindowForHeightWithoutWholePageGuard(t *testing.T) {
	// The page fits exactly in the visible height.
	start, end := listWindowForHeight(0, 10, 5, 10+listFixedRows)
	if start != 0 || end != 10 {
		t.Errorf("the whole page gives (%d, %d), want (0, 10)", start, end)
	}

	// And with the cursor anywhere on a page that fits: it does not matter
	// where it is, because it enters whole.
	for _, cursor := range []int{0, 3, 9, 50} {
		start, end := listWindowForHeight(0, 10, cursor, 10+listFixedRows)
		if start != 0 || end != 10 {
			t.Errorf("cursor=%d with the exact page gives (%d, %d), want (0, 10)", cursor, start, end)
		}
	}

	// And a page that does NOT fit: the window moves following the cursor,
	// which is exactly what the removed guard would have let through without checking.
	for _, cursor := range []int{0, 1, 2} {
		start, end := listWindowForHeight(0, 3, cursor, 3+listFixedRows)
		if start < 0 || end > 3 || end <= start {
			t.Errorf("cursor=%d with an exact window gives (%d, %d), outside the page", cursor, start, end)
		}
		if start > cursor || cursor >= end {
			t.Errorf("cursor=%d outside the window [%d, %d)", cursor, start, end)
		}
	}

	// With the cursor out of the page, the clamp puts it inside instead of
	// leaving the window in a weird place: the exact window of a page of 3
	// with the cursor at 9 has to end at the end of the page.
	start, end = listWindowForHeight(0, 3, 9, 3+listFixedRows)
	if start != 0 || end != 3 {
		t.Errorf("with the cursor at 9 and an exact window it gives (%d, %d), want (0, 3)", start, end)
	}
}

// listWindowForHeight with visible == 1: the edge that makes `< 1` reachable.
//
// With `visible <= 0` the equivalent mutant is `visible < 1` and vice versa:
// on integers they are the same condition, and that is why no test could tell
// them apart. `< 1` has an edge that does happen -- visible == 1, a single row
// for tasks -- and that edge is a window of one row inside a page of three,
// which is what tells one thing from the other.
func TestListWindowForHeightWithOneVisibleRow(t *testing.T) {
	// A page of 3 with a single visible row: the window is truncated to one.
	for _, cursor := range []int{0, 1, 2} {
		start, end := listWindowForHeight(0, 3, cursor, listFixedRows+1)
		if end-start != 1 {
			t.Errorf("cursor=%d with one visible row gives [%d, %d), want one row", cursor, start, end)
		}
		if start > cursor || cursor >= end {
			t.Errorf("cursor=%d outside the window [%d, %d)", cursor, start, end)
		}
	}

	// And the case where visible == 0, the other side of the edge: the whole
	// page is painted.
	for _, height := range []int{listFixedRows, listFixedRows - 1, 0} {
		start, end := listWindowForHeight(0, 3, 1, height)
		if start != 0 || end != 3 {
			t.Errorf("with height=%d (visible<=0) it gives (%d, %d), want the whole page (0, 3)",
				height, start, end)
		}
	}
}
