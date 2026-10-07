package tui

import (
	"fmt"
	"strings"
	"testing"
)

// clampKanban keeps the cursor inside the board. It is tested with the
// number of cards of each column as input data, without building the board:
// the rule is "column inside, row inside that column", and that is what is checked.
func TestClampKanban(t *testing.T) {
	tests := []struct {
		name             string
		col, row         int
		colLens          []int
		wantCol, wantRow int
	}{
		{"no columns", 3, 5, nil, 0, 0},
		{
			name:    "valid cursor",
			colLens: []int{3, 2, 4}, col: 1, row: 1,
			wantCol: 1, wantRow: 1,
		},
		{
			name:    "negative column is raised to 0",
			colLens: []int{3, 2, 4}, col: -2, row: 1,
			wantCol: 0, wantRow: 1,
		},
		{
			name:    "column beyond the end drops to the last",
			colLens: []int{3, 2, 4}, col: 9, row: 1,
			wantCol: 2, wantRow: 1,
		},
		{
			name:    "row inside",
			colLens: []int{5}, col: 0, row: 3,
			wantCol: 0, wantRow: 3,
		},
		{
			name:    "row beyond the end of the column is clamped",
			colLens: []int{2}, col: 0, row: 7,
			wantCol: 0, wantRow: 1,
		},
		{
			name:    "empty column leaves the row at 0",
			colLens: []int{3, 0, 2}, col: 1, row: 5,
			wantCol: 1, wantRow: 0,
		},
		{
			name:    "the empty column does not clear the selected column",
			colLens: []int{3, 0, 2}, col: 1, row: 0,
			wantCol: 1, wantRow: 0,
		},
		{
			name:    "a single empty column",
			colLens: []int{0}, col: 0, row: 4,
			wantCol: 0, wantRow: 0,
		},
		{
			name:    "a single column with cards",
			colLens: []int{3}, col: 0, row: 99,
			wantCol: 0, wantRow: 2,
		},
		{
			// The case that triggers the clamp: a filter was applied and the
			// selected column lost cards, so the row no longer exists inside it.
			name:    "the column lost cards when filtering",
			colLens: []int{4, 1}, col: 1, row: 3,
			wantCol: 1, wantRow: 0,
		},
		{
			name:    "negative row is raised to 0",
			colLens: []int{3, 2}, col: 0, row: -5,
			wantCol: 0, wantRow: 0,
		},
		{
			name:    "all columns empty",
			colLens: []int{0, 0, 0}, col: 2, row: 2,
			wantCol: 2, wantRow: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, row := clampKanban(tt.col, tt.row, tt.colLens)
			if col != tt.wantCol || row != tt.wantRow {
				t.Errorf("clampKanban(%d, %d, %v) = (%d,%d), want (%d,%d)",
					tt.col, tt.row, tt.colLens, col, row, tt.wantCol, tt.wantRow)
			}
		})
	}
}

// The result is always inside the board: it is the invariant that
// selectedTask() depends on to not index out of range.
func TestClampKanbanAlwaysInRange(t *testing.T) {
	boards := [][]int{
		nil, {}, {0}, {1}, {3, 0, 2}, {2, 2, 2}, {5, 1, 0, 4},
	}
	for _, colLens := range boards {
		for col := -3; col <= len(colLens)+3; col++ {
			for row := -3; row <= 8; row++ {
				gotCol, gotRow := clampKanban(col, row, colLens)
				if len(colLens) == 0 {
					if gotCol != 0 || gotRow != 0 {
						t.Fatalf("clampKanban(%d,%d,%v) = (%d,%d), want (0,0) with no columns",
							col, row, colLens, gotCol, gotRow)
					}
					continue
				}
				if gotCol < 0 || gotCol >= len(colLens) {
					t.Fatalf("clampKanban(%d,%d,%v) gave column %d, outside [0,%d)",
						col, row, colLens, gotCol, len(colLens))
				}
				n := colLens[gotCol]
				if n == 0 {
					if gotRow != 0 {
						t.Fatalf("clampKanban(%d,%d,%v) gave row %d in an empty column",
							col, row, colLens, gotRow)
					}
					continue
				}
				if gotRow < 0 || gotRow >= n {
					t.Fatalf("clampKanban(%d,%d,%v) gave row %d, outside [0,%d)",
						col, row, colLens, gotRow, n)
				}
			}
		}
	}
}

// kanbanMaxCards: how many whole cards fit in the available height.
func TestKanbanMaxCards(t *testing.T) {
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{"large terminal", 40, (40 - kanbanBoardChrome - filterHeaderRows + 1) / kanbanCardRows},
		{"just enough for 1 card", kanbanBoardChrome + filterHeaderRows + kanbanCardRows, 1},
		// The round-up counts the last one even if only its top part fits:
		// with 5 content rows 2 cards fit and the second one is truncated.
		{"the last card is counted even if truncated", kanbanBoardChrome + filterHeaderRows + 2*kanbanCardRows - 1, 2},
		{"for exactly 2 cards", kanbanBoardChrome + filterHeaderRows + 2*kanbanCardRows, 2},
		{"medium terminal", 20, (20 - kanbanBoardChrome - filterHeaderRows + 1) / kanbanCardRows},
		{"minimum height gives 1", 0, 1},
		{"negative height gives 1", -5, 1},
		// The rounding: with 4 content rows 1 card fits, not 2. It is the
		// edge where adding one row too many to the budget would change the
		// result, so it pins down that the columns are what they are.
		{"with 4 content rows 1 card fits", kanbanBoardChrome + filterHeaderRows + 4, 1},
		{"with 5 content rows 2 cards fit", kanbanBoardChrome + filterHeaderRows + 5, 2},
		{"with 2 content rows 1 fits", kanbanBoardChrome + filterHeaderRows + 2, 1},
		{"with 1 content row 1 fits", kanbanBoardChrome + filterHeaderRows + 1, 1},
		{"with 0 content rows 1 fits", kanbanBoardChrome + filterHeaderRows, 1},
		{"with negative content 1 fits", kanbanBoardChrome + filterHeaderRows - 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kanbanMaxCards(tt.height); got != tt.want {
				t.Errorf("kanbanMaxCards(%d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

// Never less than 1: a board with no visible cards is not a board.
func TestKanbanMaxCardsNeverZero(t *testing.T) {
	for h := -20; h <= 100; h++ {
		if got := kanbanMaxCards(h); got < 1 {
			t.Fatalf("kanbanMaxCards(%d) = %d, want >= 1", h, got)
		}
	}
}

// Taller never gives fewer cards.
func TestKanbanMaxCardsGrowsWithHeight(t *testing.T) {
	prev := 0
	for h := 0; h <= 80; h++ {
		got := kanbanMaxCards(h)
		if got < prev {
			t.Fatalf("kanbanMaxCards(%d) = %d < %d of the previous terminal", h, got, prev)
		}
		prev = got
	}
}

// kanbanHeader: with hidden cards it says how many of how many; if they all
// fit, only the total. The text changes length, and that width is the
// minimum of the column, so getting it wrong throws the whole board off.
func TestKanbanHeader(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		shown, total int
		want         string
	}{
		{"all visible", "doing", 5, 5, "─ doing (5) "},
		{"all visible at zero", "todo", 0, 0, "─ todo (0) "},
		{"truncated", "doing", 3, 12, "─ doing (3/12) "},
		{"truncated by one", "backlog", 1, 20, "─ backlog (1/20) "},
		{"all but one truncated", "done", 9, 10, "─ done (9/10) "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kanbanHeader(tt.status, tt.shown, tt.total)
			if got != tt.want {
				t.Errorf("kanbanHeader(%q, %d, %d) = %q, want %q", tt.status, tt.shown, tt.total, got, tt.want)
			}
		})
	}
}

// shown > total cannot happen through the viewer, but the function is total:
// it lands in the "everything visible" branch instead of inventing an impossible percentage.
func TestKanbanHeaderShownAboveTotal(t *testing.T) {
	got := kanbanHeader("doing", 12, 5)
	want := fmt.Sprintf("─ %s (%d) ", "doing", 5)
	if got != want {
		t.Errorf("with shown > total = %q, want %q", got, want)
	}
}

// The truncation branch is exactly "they do not all fit": at the shown == total
// edge the percentage no longer fits.
func TestKanbanHeaderBoundary(t *testing.T) {
	for total := 0; total <= 20; total++ {
		at := kanbanHeader("s", total, total)
		if strings.Contains(at, "/") {
			t.Errorf("shown == total = %q, it should not carry a percentage", at)
		}
		below := kanbanHeader("s", total-1, total)
		if total > 0 && !strings.Contains(below, "/") {
			t.Errorf("shown = total-1 = %q, it should carry a percentage", below)
		}
	}
}

// kanbanColumnWidths spreads the remainder in equal parts and gives the
// leftover columns to the first ones. It was already pure; what was missing were the borders.
func TestKanbanColumnWidthsEdges(t *testing.T) {
	tests := []struct {
		name  string
		mins  []int
		avail int
	}{
		{"a single column", []int{12}, 40},
		{"exact remainder", []int{10, 10}, 22}, // 20 + gap 2 = 22, free 0
		{"remainder of 1", []int{10, 10}, 23},  // free 1: one column +1
		{"remainder of 2", []int{10, 10}, 24},  // free 2: both +1
		{"remainder of 3", []int{10, 10}, 25},  // free 3: 1 each +1 to the first
		{"negative minimums", []int{-5, -5}, 40},
		{"no space", []int{30, 30, 30}, 10},
		{"negative avail", []int{10, 10}, -50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			widths := kanbanColumnWidths(tt.mins, tt.avail)
			if len(widths) != len(tt.mins) {
				t.Fatalf("got %d columns, want %d", len(widths), len(tt.mins))
			}
			for i, w := range widths {
				if w < tt.mins[i] {
					t.Errorf("column %d: width %d below the minimum %d", i, w, tt.mins[i])
				}
			}
		})
	}
}

// The split uses up all the remainder: the total width plus the gaps is
// exactly the available one, or the minimums if it does not fit.
func TestKanbanColumnWidthsUsesAllSpace(t *testing.T) {
	for _, mins := range [][]int{
		{16, 12, 20, 15, 13},
		{10, 10},
		{12, 12, 12, 12},
		{40},
		{5, 5, 5, 5, 5, 5, 5},
	} {
		for avail := 20; avail <= 200; avail += 7 {
			widths := kanbanColumnWidths(mins, avail)
			total := 0
			minsTotal := 0
			for i, w := range widths {
				total += w
				minsTotal += mins[i]
			}
			total += kanbanGap * (len(widths) - 1)

			if minsTotal+kanbanGap*(len(mins)-1) >= avail {
				// It does not fit: the minimums are respected and no space is invented.
				if total != minsTotal+kanbanGap*(len(mins)-1) {
					t.Errorf("mins=%v avail=%d: no space, the total should be %d and is %d",
						mins, avail, minsTotal+kanbanGap*(len(mins)-1), total)
				}
				continue
			}
			if total != avail {
				t.Errorf("mins=%v avail=%d: the total is %d, want %d (space left over or missing)",
					mins, avail, total, avail)
			}
		}
	}
}

// The columns differ by at most 1: that is what makes the split
// "spread into equal parts" and not arbitrary.
func TestKanbanColumnWidthsBalanced(t *testing.T) {
	for _, mins := range [][]int{{16, 12, 20, 15, 13}, {10, 10}, {12, 12, 12}} {
		for avail := 40; avail <= 200; avail++ {
			widths := kanbanColumnWidths(mins, avail)
			// With no remainder there is nothing to spread: each column keeps
			// its minimum, which may be very unequal among them.
			total := 0
			for _, w := range widths {
				total += w
			}
			if total+kanbanGap*(len(mins)-1) >= avail {
				continue
			}
			minW, maxW := widths[0], widths[0]
			for _, w := range widths {
				minW = min(minW, w)
				maxW = max(maxW, w)
			}
			if maxW-minW > 1 {
				t.Fatalf("mins=%v avail=%d: uneven split %v", mins, avail, widths)
			}
		}
	}
}

func TestKanbanColumnWidthsEmpty(t *testing.T) {
	if got := kanbanColumnWidths(nil, 100); got != nil {
		t.Errorf("got %v, want nil with no columns", got)
	}
	if got := kanbanColumnWidths([]int{}, 100); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}
