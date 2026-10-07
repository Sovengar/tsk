package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		name      string
		cursor    int
		total     int
		size      int
		wantStart int
		wantEnd   int
	}{
		{name: "everything fits", cursor: 3, total: 5, size: 10, wantStart: 0, wantEnd: 5},
		{name: "cursor at the start", cursor: 0, total: 20, size: 5, wantStart: 0, wantEnd: 5},
		{name: "cursor centered", cursor: 10, total: 20, size: 5, wantStart: 8, wantEnd: 13},
		{name: "cursor at the end", cursor: 19, total: 20, size: 5, wantStart: 15, wantEnd: 20},
		{name: "no items", cursor: 0, total: 0, size: 5, wantStart: 0, wantEnd: 0},
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

// TestVisibleRangeContainsCursor verifies the invariant: the range is always
// valid and the cursor always falls inside.
func TestVisibleRangeContainsCursor(t *testing.T) {
	const total, size = 37, 6
	for cursor := 0; cursor < total; cursor++ {
		start, end := visibleRange(cursor, total, size)
		if start < 0 || end > total || start >= end {
			t.Fatalf("invalid range (%d, %d) for cursor %d", start, end, cursor)
		}
		if cursor < start || cursor >= end {
			t.Fatalf("cursor %d outside the range (%d, %d)", cursor, start, end)
		}
	}
}

func TestContentBudget(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		preview  int
		keybinds int
		want     int
	}{
		{name: "discounts both", total: 30, preview: 8, keybinds: 4, want: 18},
		{name: "applies the minimum floor", total: 10, preview: 8, keybinds: 4, want: minContentHeight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contentBudget(tt.total, tt.preview, tt.keybinds); got != tt.want {
				t.Errorf("contentBudget = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJoinSections(t *testing.T) {
	tests := []struct {
		name     string
		sections []string
		want     string
	}{
		{name: "skips empties", sections: []string{"a", "", "b"}, want: "a\nb"},
		{name: "all empty", sections: []string{"", ""}, want: ""},
		{name: "without empties", sections: []string{"a", "b"}, want: "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinSections(tt.sections...); got != tt.want {
				t.Errorf("joinSections = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLineCount(t *testing.T) {
	if got := lineCount(""); got != 0 {
		t.Errorf("lineCount(\"\") = %d, want 0", got)
	}
	if got := lineCount("a\nb\nc"); got != 3 {
		t.Errorf("lineCount = %d, want 3", got)
	}
}

func TestSingleLineCollapsesNewlines(t *testing.T) {
	if got := singleLine("hello\nhow are you"); got != "hello how are you" {
		t.Errorf("singleLine = %q, want %q", got, "hello how are you")
	}
}

// TestCellWidthIgnoresAnsi verifies that padding is measured in screen
// columns and not in bytes: a colored cell measures the same as a colorless one.
// This is the bug that misaligned the List table.
func TestCellWidthIgnoresAnsi(t *testing.T) {
	styled := stylePriorityHigh.Render("●") + " H"

	if got := ansi.StringWidth(cellWidth(styled, 10)); got != 10 {
		t.Errorf("cellWidth → width %d, want 10", got)
	}
	if got := ansi.Strip(cellWidth(styled, 10)); !strings.HasPrefix(got, "● H") {
		t.Errorf("cellWidth altered the content: %q", got)
	}

	// The bug's premise: fmt's %-Ns counts bytes, so the colored cell does not
	// reach 10 columns and shifts everything that comes after.
	if got := ansi.StringWidth(fmt.Sprintf("%-10s", styled)); got != 3 {
		t.Errorf("invalid premise: %%-10s measured %d columns, expected 3", got)
	}
}

func TestCellWidthTruncates(t *testing.T) {
	if got := cellWidth("a long word", 8); got != "a long.." {
		t.Errorf("cellWidth = %q, want %q", got, "a long..")
	}
}

// The usable width of a modal: the total minus the two borders. modalWidthFor always
// leaves margin, so in real use it never reaches 1; the floor covers the total
// that arrives already trimmed down another path.
func TestModalInnerWidth(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"modal width", 58, 56},
		{"wide modal", 120, 118},
		{"three-wide modal", 3, 1},
		{"floor at 1", 2, 1},
		{"one above the floor", 1, 1},
		{"zero", 0, 1},
		{"negative", -10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modalInnerWidth(tt.in); got != tt.want {
				t.Errorf("modalInnerWidth(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// It never returns 0 or negative: a truncate width of 0 or less would make
// truncateLines truncate nothing, which is the opposite of what is wanted.
func TestModalInnerWidthNeverZero(t *testing.T) {
	for w := -50; w <= 200; w++ {
		if got := modalInnerWidth(w); got < 1 {
			t.Fatalf("modalInnerWidth(%d) = %d, want >= 1", w, got)
		}
	}
}
