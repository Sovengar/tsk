package tui

import (
	"strings"
	"testing"
)

// The detail's height split is six chained operations with four different
// floors. It came out of the render with no way to check it.

func TestDetailHeightBudget(t *testing.T) {
	tests := []struct {
		name                  string
		maxHeight, comments   int
		wantComment, wantDesc int
	}{
		// avail = maxHeight - 12, floor of 3. The comment budget takes whatever
		// there is up to maxCommentLines = avail-2, and the description is
		// left with the rest (minimum 1).
		{"roomy with many comments", 40, 20, 20, 8},
		{"roomy with few comments", 40, 3, 3, 25},
		{"roomy with no comments", 40, 0, 1, 27},
		{"roomy with 25 comments", 25, 4, 4, 9},

		// The comment budget cannot go past the reserved space: with avail=3
		// only 1 fits even if there are 20.
		{"many comments at little height", 15, 20, 1, 2},

		// avail has a floor of 3: below that, the box still makes sense.
		{"minimum height", 12, 0, 1, 2},
		{"zero height", 0, 5, 1, 2},
		{"negative height", -10, 5, 1, 2},

		// A comment always has at least one line (or the "(no comments)").
		{"zero comments gives 1", 30, 0, 1, 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotC, gotD := detailHeightBudget(tt.maxHeight, tt.comments)
			if gotC != tt.wantComment || gotD != tt.wantDesc {
				t.Errorf("detailHeightBudget(%d, %d) = (%d,%d), want (%d,%d)",
					tt.maxHeight, tt.comments, gotC, gotD, tt.wantComment, tt.wantDesc)
			}
		})
	}
}

// The two budgets are at least 1 and the sum never exceeds avail.
func TestDetailHeightBudgetInvariants(t *testing.T) {
	for h := -5; h <= 60; h++ {
		for comments := -2; comments <= 40; comments++ {
			c, d := detailHeightBudget(h, comments)
			if c < 1 {
				t.Fatalf("h=%d comments=%d: comment budget %d, want >= 1", h, comments, c)
			}
			if d < 1 {
				t.Fatalf("h=%d comments=%d: description budget %d, want >= 1", h, comments, d)
			}
			avail := max(h-12, 3)
			if c+d > avail {
				t.Fatalf("h=%d comments=%d: %d+%d exceeds avail=%d", h, comments, c, d, avail)
			}
		}
	}
}

// More comments never take the description's budget below 1.
func TestDetailHeightBudgetCommentsMonotonic(t *testing.T) {
	for h := 13; h <= 60; h++ {
		prevDesc := 1 << 30
		for comments := 0; comments <= 40; comments++ {
			_, d := detailHeightBudget(h, comments)
			if d > prevDesc {
				t.Fatalf("h=%d: with %d comments the description=%d, more than before (%d)", h, comments, d, prevDesc)
			}
			prevDesc = d
		}
	}
}

// truncateAt: byte truncation, which is what fits an ASCII timestamp.
func TestTruncateAt(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than the limit", "abc", 5, "abc"},
		{"exactly the limit", "abcde", 5, "abcde"},
		{"one byte too many", "abcdef", 5, "abcde"},
		{"far too much", strings.Repeat("x", 40), 5, "xxxxx"},
		{"zero limit", "abc", 0, ""},
		{"negative limit returns empty, it does not blow up", "abc", -1, ""},
		{"very negative limit", "abc", -99, ""},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateAt(tt.s, tt.n); got != tt.want {
				t.Errorf("truncateAt(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// The date formatters: empty is always the long dash, and the rest is
// truncated to the requested format.
func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "—"},
		// 19 bytes: "2026-09-30T18:27:52" fits whole, the trailing Z falls off.
		{"full with zone", "2026-09-30T18:27:52Z", "2026-09-30T18:27:52"},
		{"exactly 19", "2026-09-30T18:27:52", "2026-09-30T18:27:52"},
		{"short", "2026-09-30", "2026-09-30"},
		{"short garbage", "x", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTime(tt.in); got != tt.want {
				t.Errorf("formatTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// formatCommentTime replaces the "T" with a space and truncates to 16: date
// and time up to the minute.
func TestFormatCommentTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "—"},
		{"full", "2026-09-30T18:27:52Z", "2026-09-30 18:27"},
		{"without zone", "2026-09-30T18:27:52", "2026-09-30 18:27"},
		{"exactly 16", "2026-09-30 18:27", "2026-09-30 18:27"},
		{"short", "2026", "2026"},
		{"without T", "2026-09-30", "2026-09-30"},
		{"only spaces", "   ", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCommentTime(tt.in); got != tt.want {
				t.Errorf("formatCommentTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Only the FIRST "T" is replaced. Here the second survives the truncation
// and is still a T, which proves the Replace has count 1.
func TestFormatCommentTimeReplacesOnlyFirstT(t *testing.T) {
	got := formatCommentTime("T2026-09-30T18:2")
	if got != " 2026-09-30T18:2" {
		t.Errorf("formatCommentTime = %q, want \" 2026-09-30T18:2\" (only the 1st T)", got)
	}
	if strings.Count(got, "T") != 1 {
		t.Errorf("got %q, want a single T: the second one is not replaced", got)
	}
}

// formatCompleted is formatTime: the empty case does not need its own branch.
func TestFormatCompletedEqualsFormatTime(t *testing.T) {
	for _, in := range []string{
		"", "2026-09-30T18:27:52Z", "2026-09-30", "short", "x", "2026-09-30T18:27:5",
	} {
		if got, want := formatCompleted(in), formatTime(in); got != want {
			t.Errorf("formatCompleted(%q) = %q but formatTime = %q", in, got, want)
		}
	}
}

func TestFormatCompletedEmpty(t *testing.T) {
	if got := formatCompleted(""); got != "—" {
		t.Errorf("formatCompleted(\"\") = %q, want —", got)
	}
}
