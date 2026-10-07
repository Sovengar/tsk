package tui

import (
	"strings"
	"testing"
	"time"

	"tsk/internal/model"
)

// renderGanttRuler builds a line of labelW + 1 + dayCols columns: each
// week's label aligned to its Monday, over a grid of spaces.
//
// The "+1" in the middle is the separator column between the labels and the
// calendar. Changing it to "*1" removes a column from the grid, and the result
// comes out THE SAME as long as the last column stays empty -- TrimRight eats
// it. That is why a test that measures the width of the line tells nothing
// apart, and that is why the mutant survived: with any normal number of
// visible days the week label (4 letters, in columns of 7) always has room to spare.
//
// The case that separates the two is dayCols == 1: the grid is of
// labelW+2 columns and the label is written at column labelW+1, which is the
// last one. There the "+1" is exactly what gives the first character room.
// Without it, the condition `at+i < len(ruler)` discards the whole label and
// the gantt is left without the labeled Mondays.

func rulerOf(labelW, dayCols int) string {
	monday := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	m := &Model{}
	return stripANSI(m.renderGanttRuler(monday, 0, labelW, dayCols))
}

// stripANSI removes the color codes so the line can be measured.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Four visible days: the Monday label (four letters) takes column
// labelW+1 and the next four, so its LAST character falls in the last
// column of the grid. That column exists because of the "+1": without it, the
// condition `at+i < len(ruler)` discards the last character and the gantt shows "1MA".
func TestGanttRulerLabelsTheLastVisibleDay(t *testing.T) {
	label := model.WeekOfMonthLabel(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	if label != "1MAR" {
		t.Fatalf("the fixture's label is %q, want 1MAR", label)
	}

	for _, labelW := range []int{0, 1, 2, 5} {
		t.Run("labelW="+itoa(labelW), func(t *testing.T) {
			const dayCols = 4
			ruler := rulerOf(labelW, dayCols)

			if !strings.Contains(ruler, label) {
				t.Errorf("with four visible days the Monday label does not come out whole: %q, want %q",
					ruler, label)
			}
			// The grid is of labelW+1+dayCols columns, and the line neither goes
			// past it nor falls short: the label reaches the end.
			if n := len([]rune(ruler)); n != labelW+1+dayCols {
				t.Errorf("the line measures %d columns, want %d (labelW+%d days): %q",
					n, labelW+1+dayCols, dayCols, ruler)
			}
		})
	}
}

// With zero visible days there is no Monday to label and the line comes out
// empty: it is the other edge of the same range, and it says the condition
// `at+i < len(ruler)` is the one that protects, not that the "+1" is superfluous.
func TestGanttRulerWithoutVisibleDays(t *testing.T) {
	for _, labelW := range []int{0, 3, 8} {
		if r := rulerOf(labelW, 0); r != "" {
			t.Errorf("labelW=%d with no visible days comes out %q, want empty", labelW, r)
		}
	}
}

// With spare days the "+1" goes unnoticed: it is the case the rest of the
// tests start from, and that is why it did not kill the mutant. It is here to
// leave on record that the difference is margin, not content.
func TestGanttRulerWithDaysToSpare(t *testing.T) {
	label := model.WeekOfMonthLabel(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))

	ruler := rulerOf(5, 21)
	if !strings.Contains(ruler, label) {
		t.Errorf("with three visible weeks the first label does not come out: %q", ruler)
	}
	// The three weeks are seven columns apart from one another.
	for _, want := range []string{"1MAR", "2MAR", "3MAR"} {
		if !strings.Contains(ruler, want) {
			t.Errorf("with 21 days the label %q is missing: %q", want, ruler)
		}
	}
	// And with four days, the label stretches to the right edge. That
	// contrast -- slack on one side, nothing on the other -- is what makes
	// the "+1" readable.
	if r := rulerOf(5, 4); !strings.Contains(r, label) {
		t.Errorf("with four visible days the label does not come out: %q", r)
	}
}

// itoa avoids fmt for a single number.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
