package tui

import (
	"fmt"
	"strings"
	"testing"
)

// descriptionLines wraps the description to the width and truncates it to
// maxLines. The truncation has two halves and both need an exact case:
//
//	if len(wrapped) > maxLines	-- there are more lines than fit
//	truncate to limit-1		-- reserve a cell for the ellipsis
//
// The "> maxLines" is what makes a text that JUST fits not carry a planted
// ellipsis: with ">= maxLines", a text of exactly maxLines lines would come
// out with "…" on the last one, which is a text that looks whole and yet has
// a sign on it that something is missing.
//
// And the truncation limit (limit-1, limit, limit-2) is only seen when the
// last line is FULL, because ansi.Wrap delivers lines of limit columns or fewer.

func TestPreviewBarOmitsEllipsisWhenTextFitsExactly(t *testing.T) {
	// It is built the other way around: a text is chosen and how many lines it
	// takes is measured, and maxLines is adjusted to that exact number.
	const width = 40
	desc := "first line with words " +
		"second line continues here " +
		"third one also follows"

	// The text takes exactly three lines at this width, so maxLines=3
	// is the edge case: everything fits and nothing is left over.
	for _, maxLines := range []int{3} {
		t.Run(fmt.Sprintf("fits in %d lines", maxLines), func(t *testing.T) {
			p := PreviewBar{width: width, maxLines: maxLines}
			lines := p.descriptionLines(desc)
			if len(lines) != maxLines {
				t.Fatalf("with maxLines=%d there are %d lines: the text does not fit in that number", maxLines, len(lines))
			}
			// The original text is whole, so nothing is missing from it: the
			// ellipsis would have no reason to appear.
			flat := strings.Join(lines, " ")
			for _, word := range []string{"first", "second", "third", "also"} {
				if !strings.Contains(flat, word) {
					t.Fatalf("the text does not fit in %d lines: %q is missing, got %q", maxLines, word, flat)
				}
			}
			if strings.Contains(flat, "…") {
				t.Errorf("a text that just fits carries an ellipsis: %q", flat)
			}
		})
	}

	// And with one more line than fits, the ellipsis appears and something is lost.
	p := PreviewBar{width: width, maxLines: 1}
	lines := p.descriptionLines(desc)
	if len(lines) != 1 {
		t.Fatalf("with maxLines=1 there are %d lines", len(lines))
	}
	if !strings.Contains(lines[0], "…") {
		t.Errorf("a text that does NOT fit carries no ellipsis: %q", lines[0])
	}
	if strings.Contains(strings.Join(lines, " "), "third") {
		t.Error("when truncated the last part of the text should not be visible")
	}
}

// The ellipsis truncation limit (limit-1, limit, limit-2) is
// intentionally NOT checked here: ansi.Wrap delivers lines of limit columns
// or fewer, so truncating the last line removes nothing and the three limits
// give the same result. It is an equivalent mutant and it is in
// .mutation-allowlist for that reason, not for the hole one.
//
// What does matter and is checked above is that the line WITH the ellipsis
// fits inside the inner width: if the truncation did not reserve the cell,
// the "…" would push it one column too far and bordered would re-wrap it in two.

// truncateForEllipsis: the floor and the limit.
//
// The limit is what does the correct truncation and the floor is what
// prevents the panic. Both are checked from their sides, including the
// degenerate case of a limit smaller than the ellipsis, which is a very narrow terminal.
func TestTruncateForEllipsis(t *testing.T) {
	t.Run("fits whole", func(t *testing.T) {
		got := truncateForEllipsis("short text", 40)
		if !strings.Contains(got, "short text") {
			t.Errorf("a text that fits lost part of itself: %q", got)
		}
		if !strings.HasSuffix(got, ellipsisDescription) {
			t.Errorf("the ellipsis is missing at the end: %q", got)
		}
	})

	t.Run("does not fit and is truncated leaving room for the ellipsis", func(t *testing.T) {
		const limit = 10
		got := truncateForEllipsis(strings.Repeat("x", 100), limit)

		// The line with the ellipsis has to fit in the limit: if not, on
		// padding the box it would re-wrap in two and break the height cap.
		if n := len([]rune(got)); n > limit {
			t.Errorf("the truncated line measures %d columns and the limit is %d: %q", n, limit, got)
		}
		if !strings.HasSuffix(got, ellipsisDescription) {
			t.Errorf("the ellipsis is missing: %q", got)
		}
		// And with the exact limit: the text fills up to the ellipsis gap.
		if n := len([]rune(got)); n != limit {
			t.Errorf("the line measures %d columns, want %d (the whole limit)", n, limit)
		}
	})

	t.Run("limit smaller than the ellipsis", func(t *testing.T) {
		// Here the floor at 0 is what prevents the negative. Only the ellipsis comes out.
		for _, limit := range []int{0, 1} {
			got := truncateForEllipsis("text", limit)
			if got != ellipsisDescription {
				t.Errorf("with limit %d got %q, want only the ellipsis", limit, got)
			}
		}
	})

	t.Run("empty text", func(t *testing.T) {
		got := truncateForEllipsis("", 10)
		if got != ellipsisDescription {
			t.Errorf("with empty text got %q, want only the ellipsis", got)
		}
	})

	t.Run("the ellipsis takes what it says", func(t *testing.T) {
		if ellipsisWidth != len([]rune(ellipsisDescription)) {
			t.Errorf("ellipsisWidth = %d but the ellipsis measures %d: the reserved gap is not the real one",
				ellipsisWidth, len([]rune(ellipsisDescription)))
		}
	})
}

// The minimum width of a kanban column: the header plus the borders, with
// the absolute floor below.
func TestMinColumnWidth(t *testing.T) {
	cases := []struct {
		name   string
		header int
		want   int
	}{
		{"fits very short", 1, kanbanMinColWidth},
		{"exactly at the floor", kanbanMinColWidth - borderWidths, kanbanMinColWidth},
		{"one more than the floor", kanbanMinColWidth - borderWidths + 1, kanbanMinColWidth + 1},
		{"very wide", 100, 100 + borderWidths},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := minColumnWidth(c.header); got != c.want {
				t.Errorf("minColumnWidth(%d) = %d, want %d", c.header, got, c.want)
			}
		})
	}

	// The count of the borders is what makes the edge between "fits by the
	// floor" and "fits by the header": with one column less of borders, the
	// first case would change.
	if got := minColumnWidth(0); got != kanbanMinColWidth {
		t.Errorf("a 0-column header gives %d, want the floor %d", got, kanbanMinColWidth)
	}
}
