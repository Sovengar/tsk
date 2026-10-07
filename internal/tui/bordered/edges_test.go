package bordered

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// A box's borders have three floors --the total width, the interior and the
// padding of each line-- and each floor had its comparison against an edge
// that no test could reach. These tests reach them from both sides.
//
// The common background is the same in all three: a clamp to a minimum can be
// written with max and stays an identity at the edge, or a condition whose
// edge has to be provoked. With max() there is no edge to provoke: there is
// no comparison to mutate.

// boxOf is a shortcut for a rounded box with the requested width.
func boxOf(width int) lipgloss.Border { return lipgloss.RoundedBorder() }

func render(t *testing.T, content string, width int) string {
	t.Helper()
	return RenderWithTitleEx(boxOf(width), nil, AlignLeft, " T ", content, width)
}

func linesOf(s string) []string { return strings.Split(strings.TrimRight(s, "\n"), "\n") }

// The floor of the total width: two columns, one per corner. A width of 1
// goes up to 2 and the box is drawn with no interior.
func TestMinimumBoxWidth(t *testing.T) {
	t.Run("below the minimum the box does not collapse", func(t *testing.T) {
		for _, w := range []int{-5, 0, 1} {
			out := render(t, "hello", w)
			lines := linesOf(out)
			if len(lines) < 3 {
				t.Fatalf("width=%d: the box has %d lines, want at least 3 (top, middle, bottom)",
					w, len(lines))
			}
			// The middle line has to exist and not blow up: it is the interior
			// of width 0.
			if !strings.Contains(lines[1], "│") {
				t.Errorf("width=%d: the middle line has no borders: %q", w, lines[1])
			}
		}
	})

	t.Run("exactly the minimum", func(t *testing.T) {
		out := render(t, "", 2)
		lines := linesOf(out)
		if len(lines) != 3 {
			t.Errorf("with width=2 there are %d lines, want 3", len(lines))
		}
		// The interior of a box of 2 is 0 columns: the two borders together.
		if n := len([]rune(strings.Trim(lines[1], "│"))); n != 0 {
			t.Errorf("the interior measures %d columns, want 0", n)
		}
	})
}

// The floor of the interior: it cannot be negative because the wide corners
// of a third-party border eat the whole width. With a border of
// three-column corners and width 3, the interior is -3.
func TestInteriorCannotBeNegative(t *testing.T) {
	wideBorder := lipgloss.Border{
		Top: "-", Bottom: "-", Left: "|", Right: "|",
		TopLeft: "/3./", TopRight: "/3./",
		BottomLeft: "\\3.\\", BottomRight: "\\3.\\",
	}
	// With a width smaller than the corners, the interior comes out negative.
	for _, w := range []int{2, 3, 4} {
		out := RenderWithTitleEx(wideBorder, nil, AlignLeft, "", "content", w)
		for i, l := range linesOf(out) {
			if strings.Contains(l, "Repeat") || strings.Contains(l, "panic") {
				t.Errorf("w=%d line %d: the output looks broken: %q", w, i, l)
			}
		}
	}
	// And with a normal border, the interior is the width minus 2.
	out := render(t, "abcd", 10)
	lines := linesOf(out)
	if n := len([]rune(strings.Trim(lines[1], "│ "))); n != 4 {
		t.Errorf("with width=10 the visible interior measures %d columns, want 4", n)
	}
}

// The title truncation: when the title already fits whole, the truncation is
// an identity and it has to BE NOTICED that nothing was truncated. A title of
// exactly the same width as the interior is the edge, and there `>` and `>=`
// give the same -- that is why a title that fills the interior with one
// character less and one that fills it too much are needed.
func TestTitleIsTruncatedOnlyWhenItDoesNotFit(t *testing.T) {
	const width = 20 // interior = 18

	t.Run("fits with room to spare", func(t *testing.T) {
		out := RenderWithTitleEx(boxOf(width), nil, AlignLeft, " short ", "content", width)
		if !strings.Contains(out, "short") {
			t.Errorf("the short title disappeared: %q", out)
		}
	})

	t.Run("fills the interior exactly", func(t *testing.T) {
		// 18 columns of title = exactly the interior.
		title := strings.Repeat("T", 18)
		out := RenderWithTitleEx(boxOf(width), nil, AlignLeft, title, "x", width)
		if n := strings.Count(out, "T"); n != 18 {
			t.Errorf("%d title characters come out, want 18: the interior is 18 and the title fills it", n)
		}
	})

	t.Run("does not fit and is truncated", func(t *testing.T) {
		title := strings.Repeat("T", 25)
		out := RenderWithTitleEx(boxOf(width), nil, AlignLeft, title, "x", width)
		if n := strings.Count(out, "T"); n != 18 {
			t.Errorf("a title of 25 comes out with %d characters, want 18 (the interior width)", n)
		}
	})

	t.Run("the interior does not change because of the title", func(t *testing.T) {
		for _, long := range []int{0, 5, 18, 25, 100} {
			out := RenderWithTitleEx(boxOf(width), nil, AlignLeft,
				strings.Repeat("T", long), "content", width)
			lines := linesOf(out)
			for i, l := range lines {
				if n := len([]rune(l)); n != width {
					t.Errorf("title of %d, line %d measures %d columns, want %d",
						long, i, n, width)
				}
			}
		}
	})
}

// The floor of the padding: a word longer than the interior overflows, and
// the floor at zero is what keeps strings.Repeat from blowing up with a
// negative number. This is the case that really matters -- a long task title
// in a narrow terminal -- and the one a `max` leaves on record.
func TestWordWiderThanTheInterior(t *testing.T) {
	const width = 12 // interior = 10
	long := strings.Repeat("x", 40)

	out := RenderWithTitleEx(boxOf(width), nil, AlignLeft, "", long, width)

	lines := linesOf(out)
	if len(lines) < 3 {
		t.Fatalf("only %d lines: the long word produced nothing", len(lines))
	}
	// All the lines measure the box width, not one more.
	for i, l := range lines {
		if n := len([]rune(l)); n != width {
			t.Errorf("line %d measures %d columns, want %d: %q", i, n, width, l)
		}
	}
	// And the word comes out whole on several lines, not cut mid-column.
	if got := strings.Count(out, "x"); got != 40 {
		t.Errorf("%d characters of the word come out, want 40: the padding truncated it", got)
	}
}

// The padding of each line with variable width, which is what makes the floor
// useful in the normal case: content that takes exactly the interior, one
// that falls short and one that goes past.
func TestPaddingPerLineWidth(t *testing.T) {
	const width = 20 // interior = 18
	content := "short\n" + strings.Repeat("y", 18) + "\n" + strings.Repeat("z", 19)

	out := RenderWithTitleEx(boxOf(width), nil, AlignLeft, "", content, width)
	lines := linesOf(out)

	if len(lines) < 5 {
		t.Fatalf("only %d lines for 3 content lines", len(lines))
	}
	for i, l := range lines {
		if n := len([]rune(l)); n != width {
			t.Errorf("line %d measures %d columns, want %d: %q", i, n, width, l)
		}
	}
}

// parseAnsiSegments splits a string with ANSI codes into style and text
// pieces. The case that matters is a CSI that starts RIGHT after the first
// character: it is the colored text of a lifetime, and it is where the search
// for the next CSI returns 0.
//
// Returning 0 is exactly what the previous code could not distinguish: it
// searched from the beginning of s, and since the `if strings.HasPrefix`
// above had already discarded that s started with CSI, that 0 never happened.
// With the search starting at s[1:] the 0 is a real value, and this check ties it down.
func TestParseAnsiSegmentsSplitsAtFirstCSI(t *testing.T) {
	const esc = "\x1b["
	red := esc + "31m"
	reset := "\x1b[0m"

	cases := []struct {
		name  string
		input string
		want  []ansiSegment
	}{
		{"text and then CSI", "a" + red + "b",
			[]ansiSegment{
				{style: "", text: "a"},
				{style: red, text: ""},
				{style: "", text: "b"},
			}},
		{"CSI at the second character", "x" + red + "y" + reset + "z",
			[]ansiSegment{
				{style: "", text: "x"},
				{style: red, text: ""},
				{style: "", text: "y"},
				{style: reset, text: ""},
				{style: "", text: "z"},
			}},
		{"CSI right at the start", red + "b",
			[]ansiSegment{
				{style: red, text: ""},
				{style: "", text: "b"},
			}},
		{"CSI at the end", "a" + red,
			[]ansiSegment{
				{style: "", text: "a"},
				{style: red, text: ""},
			}},
		{"truncated CSI", "a" + "\x1b[31",
			[]ansiSegment{
				{style: "", text: "a"},
				{style: "\x1b[31", text: ""},
			}},
		{"empty CSI", "a" + esc + "m" + "b",
			[]ansiSegment{
				{style: "", text: "a"},
				{style: esc + "m", text: ""},
				{style: "", text: "b"},
			}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAnsiSegments(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("segments = %d, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i].style != tc.want[i].style || got[i].text != tc.want[i].text {
					t.Errorf("segment %d = {style:%q text:%q}, want {style:%q text:%q}",
						i, got[i].style, got[i].text, tc.want[i].style, tc.want[i].text)
				}
			}
			// What really matters: rearming the segments returns the original,
			// without losing a single byte of style.
			var b strings.Builder
			for _, seg := range got {
				b.WriteString(seg.style)
				b.WriteString(seg.text)
			}
			if b.String() != tc.input {
				t.Errorf("reassembled comes out %q, want %q", b.String(), tc.input)
			}
		})
	}
}

// The lone ESC: it does not open a CSI, so it is text, and it has to advance
// the loop by itself. This is the case that prevented adding a safety
// "end = 1", and it also checks that the search from s[1:] does not eat intermediate ESCs.
func TestParseAnsiSegmentsWithLoneEsc(t *testing.T) {
	input := "a\x1bb"
	got := parseAnsiSegments(input)

	if len(got) != 1 {
		t.Fatalf("segments = %d, want 1 (a lone ESC is text, not style): %+v", len(got), got)
	}
	if got[0].style != "" || got[0].text != input {
		t.Errorf("segment = {style:%q text:%q}, want {style:\"\" text:%q}",
			got[0].style, got[0].text, input)
	}
}

// A text that starts and ends in ESC, with a CSI in between: it checks that
// the first character is emitted as text and is not lost by searching from s[1:].
func TestParseAnsiSegmentsKeepsFirstCharacter(t *testing.T) {
	const red = "\x1b[31m"
	input := "á" + red + "é" // 'á' takes two bytes in UTF-8

	got := parseAnsiSegments(input)
	if len(got) != 3 {
		t.Fatalf("segments = %d, want 3: %+v", len(got), got)
	}
	if got[0].text != "á" {
		t.Errorf("the first segment is %q, want %q: the first character was split", got[0].text, "á")
	}
	if got[2].text != "é" {
		t.Errorf("the last segment is %q, want %q", got[2].text, "é")
	}
}
