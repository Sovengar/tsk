package bordered

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// linesplit splits the output into lines already without ANSI, which is what is compared.
func linesplit(t *testing.T, out string) []string {
	t.Helper()
	raw := strings.Split(out, "\n")
	out2 := make([]string, len(raw))
	for i, l := range raw {
		out2[i] = ansi.Strip(l)
	}
	return out2
}

// boxLines separates the border lines from the content.
func boxLines(t *testing.T, out string) (top, bottom string, body []string) {
	t.Helper()
	ls := linesplit(t, out)
	if len(ls) < 3 {
		t.Fatalf("a box needs at least 3 lines, it has %d: %q", len(ls), out)
	}
	return ls[0], ls[len(ls)-1], ls[1 : len(ls)-1]
}

// All the lines of a box measure exactly `width`. It is the invariant that
// holds up the whole layout: if one line goes out of sync, the modal looks broken.
func assertUniformWidth(t *testing.T, out string, width int) {
	t.Helper()
	for i, l := range linesplit(t, out) {
		if w := ansi.StringWidth(l); w != width {
			t.Errorf("line %d measures %d, want %d: %q", i, w, width, l)
		}
	}
}

func roundBorder() lipgloss.Border { return lipgloss.RoundedBorder() }

// unwrap removes the first and the last character of a box row. It works
// with runes: the border characters are multibyte and a byte slice splits the
// glyph in half.
func unwrap(t *testing.T, line string) string {
	t.Helper()
	r := []rune(ansi.Strip(line))
	if len(r) < 2 {
		t.Fatalf("row too short to remove its frame: %q", line)
	}
	return string(r[1 : len(r)-1])
}

// --- Minimum width and impossible widths --------------------------------

// Below 2 columns there is no possible box: the width goes up to 2 instead
// of producing a zero or negative width border.
//
// With the interior at 0 the content cannot be truncated (wrapLine returns
// the whole line when there is no width), so content rows may
// exceed the box. What does have to hold is that the top and bottom borders
// measure the same and that the render does not blow up with impossible widths.
func TestRenderMinimumWidth(t *testing.T) {
	for _, width := range []int{-5, 0, 1, 2, 3} {
		out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", "x", width)
		top, bottom, body := boxLines(t, out)
		if len(body) == 0 {
			t.Errorf("width=%d: the box must have at least one content row", width)
		}
		if ansi.StringWidth(top) != ansi.StringWidth(bottom) {
			t.Errorf("width=%d: top border measures %d and bottom %d, they must match",
				width, ansi.StringWidth(top), ansi.StringWidth(bottom))
		}
		// The truncation value matters: below 2 the box measures 2, so
		// the top border is 2 dashes between corners and nothing else.
		if width < 2 && top != "╭╮" {
			t.Errorf("width=%d: top border = %q, want the minimum ╭╮", width, top)
		}
		// At 2 columns the interior is 0, so not even one padding character
		// fits between the corners.
		if width == 2 && top != "╭╮" {
			t.Errorf("width=2: top border = %q, want ╭╮ (interior 0)", top)
		}
	}
}

// A border with wide corners (double-width characters) can leave the
// interior at zero or negative; it must never be negative.
func TestRenderWithWideCorners(t *testing.T) {
	b := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "｛", TopRight: "｝", BottomLeft: "｟", BottomRight: "～",
	}
	// The corners take 2 columns each and the sides 1, so with
	// interior >= 1 the border measures width and the content measures width - 2.
	for _, width := range []int{5, 6, 8, 10, 20} {
		inner := width - 4
		out := RenderWithTitlesEx(b, nil, "", AlignLeft, "", AlignLeft, "long content", width)
		top, bottom, body := boxLines(t, out)

		if got := ansi.StringWidth(top); got != width {
			t.Errorf("width=%d: top border measures %d, want %d: %q", width, got, width, top)
		}
		if got := ansi.StringWidth(bottom); got != width {
			t.Errorf("width=%d: bottom border measures %d, want %d: %q", width, got, width, bottom)
		}
		for i, l := range body {
			if got := ansi.StringWidth(l); got != inner+2 {
				t.Errorf("width=%d: row %d measures %d, want %d: %q", width, i, got, inner+2, l)
			}
		}
	}
}

// --- Missing border characters -----------------------------------------

// A border with empty characters is padded with spaces: without this the box
// would come out with holes and the lines would not add up.
func TestRenderFillsMissingBorderChars(t *testing.T) {
	// width 24, interior 22, content "body" (4 columns) + 18 of padding.
	const (
		topOK    = "┌" + "──────────────────────" + "┐"
		bottomOK = "└" + "──────────────────────" + "┘"
		bodyOK   = "│body                  │"
	)
	_ = bottomOK

	tests := []struct {
		name   string
		border lipgloss.Border
		want   []string
	}{
		{
			name: "no top line",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Left: "│", Right: "│", Bottom: "─",
			},
			// The top line is padded with spaces: it is the visible gap.
			want: []string{"┌" + "                      " + "┐", bodyOK, bottomOK},
		},
		{
			name: "no bottom line",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Left: "│", Right: "│", Top: "─",
			},
			want: []string{topOK, bodyOK, "└" + "                      " + "┘"},
		},
		{
			name: "no sides",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Top: "─", Bottom: "─",
			},
			// Without │ the content goes glued to the border, with one space on each side.
			want: []string{topOK, " body                   ", bottomOK},
		},
		{
			name: "no top line or sides",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘", Bottom: "─",
			},
			want: []string{"┌" + "                      " + "┐", " body                   ", bottomOK},
		},
		{
			name: "no horizontal lines",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Left: "│", Right: "│",
			},
			want: []string{"┌" + "                      " + "┐", bodyOK, "└" + "                      " + "┘"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const width = 24
			out := RenderWithTitleEx(tt.border, nil, AlignLeft, "", "body", width)
			// If the border character were missing instead of being replaced by
			// a space, some line would measure less than the requested width.
			assertUniformWidth(t, out, width)

			// Exact output: it pins down WHICH line carries the gap and with what
			// character. A loose assert ("some line has a space") does not tell
			// a space padding from a border drawn with another character.
			want := strings.Join(tt.want, "\n")
			if got := ansi.Strip(out); got != want {
				t.Errorf("output =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// When the wide corners leave the interior negative, it is truncated to 0
// instead of propagating a negative interior (which would give a negative
// padding and a panic in strings.Repeat).
func TestRenderWideCornersWithNegativeInnerWidth(t *testing.T) {
	b := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "｛", TopRight: "｝", BottomLeft: "｟", BottomRight: "～",
	}
	for _, width := range []int{1, 2, 3, 4} {
		// It must not go into panic: that is the reason for the truncation to 0.
		out := RenderWithTitlesEx(b, nil, "", AlignLeft, "", AlignLeft, "abc", width)
		top, bottom, body := boxLines(t, out)
		if ansi.StringWidth(top) != ansi.StringWidth(bottom) {
			t.Errorf("width=%d: misaligned borders: %q vs %q", width, top, bottom)
		}
		// With the interior truncated to 0 there is nowhere to wrap, so the
		// content stays in ONE row. If the truncation were to 1 instead of 0,
		// "abc" would be split into three rows of one column.
		if len(body) != 1 {
			t.Errorf("width=%d: %d body rows, want 1 (interior clamped to 0): %q", width, len(body), body)
		}
	}
}

// --- Multi-line content ------------------------------------------------

// Each line break of the content produces a row of the box, and only one.
func TestRenderContentLinePerNewline(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"one line", "a", 1},
		{"three lines", "a\nb\nc", 3},
		{"empty line at the beginning", "\na", 2},
		{"empty line at the end", "a\n", 2},
		{"only newlines", "\n\n", 3},
		{"with interleaved newlines", "a\n\nb", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const width = 20
			out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", tt.content, width)
			_, _, body := boxLines(t, out)
			if len(body) != tt.want {
				t.Errorf("body lines = %d, want %d: %q", len(body), tt.want, body)
			}
			assertUniformWidth(t, out, width)
		})
	}
}

// A line wider than the interior is WRAPPED into several rows, it is
// neither discarded nor truncated.
func TestRenderWrapsWideContent(t *testing.T) {
	const width = 12
	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", strings.Repeat("x", 30), width)
	_, _, body := boxLines(t, out)

	if len(body) < 3 {
		t.Fatalf("30 columns in 10 of interior should wrap into 3+, there are %d: %q", len(body), body)
	}
	assertUniformWidth(t, out, width)
	for i, l := range body {
		inner := unwrap(t, l)
		if strings.TrimSpace(inner) != strings.Repeat("x", len([]rune(inner))) {
			t.Errorf("body %d = %q, want only x", i, inner)
		}
	}
}

// A colored line measuring EXACTLY the interior must not be split. The
// padding branch uses `<=` and not `<` on purpose: even if the width matches,
// the line can carry style changes in the middle, and wrapping would split
// there. With `<` that row would break in two and the modal's height would change.
func TestRenderColoredLineAtExactInnerWidth(t *testing.T) {
	const width = 16
	// 14 of interior. We color a text that measures EXACTLY 14 columns and
	// whose style change falls in the middle: "aaaa" red + "bbbbbbbbbb" green.
	//
	// Here is the `<=` vs `<` edge of the padding branch. With `<=` the line
	// enters the padding branch (1 row) because its visible width already fits
	// exactly. With `<` (mutated) it falls into the wrapping one, and wrapLine
	// splits at the style change: 2 rows and the modal's height changes.
	styled := "\033[31m" + strings.Repeat("a", 4) + "\033[0m" +
		"\033[32m" + strings.Repeat("b", 10) + "\033[0m"
	if got := ansi.StringWidth(ansi.Strip(styled)); got != 14 {
		t.Fatalf("fixture: the content measures %d columns, want 14", got)
	}

	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", styled, width)
	_, _, body := boxLines(t, out)
	if len(body) != 1 {
		t.Errorf("content of 14 columns in interior 14: %d rows, want 1 (must not split): %q", len(body), body)
	}
	assertUniformWidth(t, out, width)
}

// The padding up to the inner width is exact on every row.
func TestRenderPadsContentToInnerWidth(t *testing.T) {
	const width = 16
	// "abcdefghijklmn" measures just the 14 of interior: with the border
	// misplaced (`<` instead of `<=`) this row would wrap in two and the height would change.
	tests := []string{"", "x", "ab", "abc", "abcdefghij", "abcdefghijklmn"}
	for _, content := range tests {
		out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", content, width)
		_, _, body := boxLines(t, out)
		if len(body) != 1 {
			t.Fatalf("content %q: %d rows, want 1", content, len(body))
		}
		inner := unwrap(t, body[0])
		if ansi.StringWidth(inner) != width-2 {
			t.Errorf("content %q: interior measures %d, want %d (%q)", content, ansi.StringWidth(inner), width-2, inner)
		}
		if !strings.HasPrefix(inner, content) {
			t.Errorf("content %q: the padding ate text (%q)", content, inner)
		}
		if strings.TrimRight(inner, " ") != content {
			t.Errorf("content %q: must only add spaces on the right (%q)", content, inner)
		}
	}
}

// With the interior at 0 there is nowhere to wrap, so wrapLine returns the
// whole line and the padding comes out negative: it is truncated to 0 instead of subtracting.
func TestRenderNegativePaddingIsClamped(t *testing.T) {
	b := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "｛", TopRight: "｝", BottomLeft: "｟", BottomRight: "～",
	}
	// width 3 -> negative interior -> 0. The content does not fit and even so
	// the padding cannot be negative.
	out := RenderWithTitlesEx(b, nil, "", AlignLeft, "", AlignLeft, "body", 3)
	_, _, body := boxLines(t, out)
	// The interior is 0 and "body" can be neither wrapped nor truncated, so
	// the row is the content as-is between the sides, with NO extra padding: a
	// `padding = 1` instead of `padding = 0` would add one space too many.
	if len(body) != 1 {
		t.Fatalf("body = %q, want one row", body)
	}
	if body[0] != "│body│" {
		t.Errorf("row = %q, want │body│ (padding clamped to 0)", body[0])
	}
}

// --- Alignment of the border titles -------------------------------------

// The alignment decides where the padding falls: to the left of the text,
// to the right, or spread out.
func TestRenderTitleAlignment(t *testing.T) {
	const width = 30
	tests := []struct {
		name        string
		align       int
		wantPrefix  string // what is before the title, without the corner
		wantPostfix int    // padding characters after the title
	}{
		{"left", AlignLeft, "╭", width - 3},
		{"right", AlignRight, "", 0},
		{"center", AlignCenter, "", 0}, // the rest is checked by width
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderWithTitleEx(roundBorder(), nil, tt.align, "TIT", "", width)
			top, _, _ := boxLines(t, out)
			assertUniformWidth(t, out, width)

			runes := []rune(top)
			idx := strings.Index(top, "TIT")
			if idx < 0 {
				t.Fatalf("the title does not appear: %q", top)
			}
			runeIdx := len([]rune(top[:idx]))
			_ = runes
			switch tt.align {
			case AlignLeft:
				if !strings.HasPrefix(top, tt.wantPrefix) {
					t.Errorf("align left = %q, want prefix %q", top, tt.wantPrefix)
				}
			case AlignRight:
				// Right alignment leaves the padding BEFORE the title: the dashes
				// go from the corner to the start of the text.
				got := runeIdx - 1
				want := width - 2 - len("TIT")
				if got != want {
					t.Errorf("align right: %d dashes before the title, want %d (%q)", got, want, top)
				}
				if len(runes)-1-(runeIdx+len("TIT")) != 0 {
					t.Errorf("align right: leftover padding after the title (%q)", top)
				}
			case AlignCenter:
				// The leftover padding is spread: the left never exceeds the right
				// by more than one.
				left := runeIdx - 1 // without the corner
				right := len(runes[runeIdx+len("TIT") : len(runes)-1])
				if left > right+1 || right > left+1 {
					t.Errorf("center: %d on the left and %d on the right are not spread out (%q)", left, right, top)
				}
				if left+right+len("TIT") != width-2 {
					t.Errorf("center: %d+%d+%d != %d", left, right, len("TIT"), width-2)
				}
			}
		})
	}
}

// A title wider than the interior is truncated to the available width,
// instead of overflowing the box.
func TestRenderTruncatesOverlongTitle(t *testing.T) {
	const width = 10 // interior = 8
	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "TITLE TOO LONG", "", width)
	assertUniformWidth(t, out, width)

	top, _, _ := boxLines(t, out)
	if strings.Contains(top, "LONG") {
		t.Errorf("the title should be truncated, but it appears whole: %q", top)
	}
	if !strings.Contains(top, "TIT") {
		t.Errorf("it should keep the beginning of the title: %q", top)
	}
}

// The title width is measured in COLUMNS, not in bytes: a title with accents
// or emoji takes less than its rune length suggests.
func TestRenderMeasuresTitleInColumns(t *testing.T) {
	const width = 14
	wide := "áéí" // 3 runes, 6 bytes, 3 columns
	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, wide, "", width)
	assertUniformWidth(t, out, width)

	top, _, _ := boxLines(t, out)
	if !strings.Contains(top, wide) {
		t.Errorf("the title fits with room to spare and must appear whole: %q", top)
	}
}

// --- Colors -------------------------------------------------------------

// With border color, the ANSI wraps the lines but the visible width does
// not change: if the style altered the width, the box would go out of sync on screen.
func TestRenderWithBorderColorKeepsVisibleWidth(t *testing.T) {
	const width = 24
	fg := color.RGBA{R: 0x88, G: 0x00, B: 0xAA, A: 0xFF}
	out := RenderWithTitleEx(roundBorder(), fg, AlignLeft, " T ", "text body", width)

	if !strings.Contains(out, "\033[") {
		t.Error("with border color the output must include ANSI sequences")
	}
	assertUniformWidth(t, out, width)
}

// --- wrapLine: the wrapping logic keeping the style ---------------

// Wrapping cannot split an ANSI sequence in half nor lose the color.
func TestWrapLinePreservesAnsiStyles(t *testing.T) {
	red := "\033[31m"
	reset := "\033[0m"
	line := red + strings.Repeat("word ", 6) + reset

	chunks := wrapLine(line, 10)
	if len(chunks) < 2 {
		t.Fatalf("a line of %d columns in 10 should split: %v", ansi.StringWidth(line), chunks)
	}
	for i, c := range chunks {
		if w := ansi.StringWidth(c); w > 10 {
			t.Errorf("chunk %d measures %d, want <= 10: %q", i, w, c)
		}
		if !strings.Contains(c, red) {
			t.Errorf("chunk %d lost the style: %q", i, c)
		}
		if !strings.HasSuffix(c, reset) {
			t.Errorf("chunk %d does not close the style: %q", i, c)
		}
	}
	// And no text is lost.
	var joined string
	for _, c := range chunks {
		joined += ansi.Strip(c)
	}
	if strings.TrimSpace(joined) != strings.TrimSpace(ansi.Strip(line)) {
		t.Errorf("the wrapped text differs:\n original: %q\n joined:   %q", ansi.Strip(line), joined)
	}
}

// With a non-positive max width nothing is wrapped: the line is returned
// as-is, because there is nowhere to split it.
func TestWrapLineNonPositiveWidth(t *testing.T) {
	for _, w := range []int{0, -1, -10} {
		line := "long text"
		got := wrapLine(line, w)
		if len(got) != 1 {
			t.Fatalf("wrapLine(%q, %d) returned %d lines, want 1", line, w, len(got))
		}
		if got[0] != line {
			t.Errorf("wrapLine(%q, %d) = %q, want the line intact", line, w, got[0])
		}
	}
}

// An unstyled text line that does not fit is split by exact columns.
func TestWrapLineSplitsAtExactWidth(t *testing.T) {
	got := wrapLine(strings.Repeat("a", 25), 10)
	if len(got) != 3 {
		t.Fatalf("got %d chunks, want 3: %v", len(got), got)
	}
	for i, want := range []int{10, 10, 5} {
		if len(got[i]) != want || got[i] != strings.Repeat("a", want) {
			t.Errorf("chunk %d = %q, want %d aes", i, got[i], want)
		}
	}
}

// A short line fits whole and is not split.
func TestWrapLineShortLineUnchanged(t *testing.T) {
	got := wrapLine("short", 10)
	if len(got) != 1 || got[0] != "short" {
		t.Errorf("wrapLine(%q, 10) = %v, want a line intact", "short", got)
	}
}

// A styled line whose style changes in the middle must split coherently:
// each piece carries its own style, not the one before nor the one after.
func TestWrapLineHandlesStyleChange(t *testing.T) {
	line := "\033[31mred\033[0m\033[32mgreen\033[0m"
	chunks := wrapLine(line, 5)
	for i, c := range chunks {
		if ansi.StringWidth(c) > 5 {
			t.Errorf("chunk %d measures %d, want <= 5: %q", i, ansi.StringWidth(c), c)
		}
	}
	joined := ""
	for _, c := range chunks {
		joined += ansi.Strip(c)
	}
	if joined != "redgreen" {
		t.Errorf("joined text = %q, want redgreen", joined)
	}
}

// --- parseAnsiSegments ---------------------------------------------------

// The parser separates style and text, preserving the order.
func TestParseAnsiSegments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []ansiSegment
	}{
		{
			name: "plain text",
			in:   "hello",
			want: []ansiSegment{{style: "", text: "hello"}},
		},
		{
			name: "style at the start",
			in:   "\033[31mred",
			want: []ansiSegment{{style: "\033[31m", text: ""}, {style: "", text: "red"}},
		},
		{
			name: "style in the middle",
			in:   "a\033[1mb",
			want: []ansiSegment{{style: "", text: "a"}, {style: "\033[1m", text: ""}, {style: "", text: "b"}},
		},
		{
			name: "reset alone",
			in:   "\033[0m",
			want: []ansiSegment{{style: "\033[0m", text: ""}},
		},
		{
			name: "multiple styles",
			in:   "\033[1ma\033[31mb",
			want: []ansiSegment{
				{style: "\033[1m", text: ""},
				{style: "", text: "a"},
				{style: "\033[31m", text: ""},
				{style: "", text: "b"},
			},
		},
		{
			// 0x7E ('~') is the last valid byte of a CSI sequence: the
			// loop has to include it or the sequence is truncated and the rest
			// of the text is read as if it were part of the style.
			name: "CSI ending in ~",
			in:   "\033[1~after",
			want: []ansiSegment{
				{style: "\033[1~", text: ""},
				{style: "", text: "after"},
			},
		},
		{
			name: "CSI with parameters and ~",
			in:   "\033[3;5~x",
			want: []ansiSegment{
				{style: "\033[3;5~", text: ""},
				{style: "", text: "x"},
			},
		},
		{
			// '@' (0x40) is the FIRST byte of the finalizer range, so it is a
			// valid final byte. If the range started at 0x41, this sequence
			// would swallow the text that comes behind it.
			name: "CSI ending in @",
			in:   "\033[?7@after",
			want: []ansiSegment{
				{style: "\033[?7@", text: ""},
				{style: "", text: "after"},
			},
		},
		{
			// And the same at the other end of the range.
			name: "CSI ending in ~",
			in:   "\033[1~final",
			want: []ansiSegment{
				{style: "\033[1~", text: ""},
				{style: "", text: "final"},
			},
		},
		{
			// 'm' (0x6D) is the SGR finalizer, the most common case.
			name: "SGR",
			in:   "\033[31mred\033[0m",
			want: []ansiSegment{
				{style: "\033[31m", text: ""},
				{style: "", text: "red"},
				{style: "\033[0m", text: ""},
			},
		},
		{
			name: "empty string",
			in:   "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAnsiSegments(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d segments %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("segment %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestParseAnsiSegmentsTerminates is the guarantee that the cases above do NOT
// cover: that the parser always advances, whatever the input.
//
// The previous suite had no truncated input, so the exact shape of the loop
// —`j := i` or `j := i+1`— was indistinguishable from any other. That is
// exactly the condition under which the earlier version could not advance: if
// the text scan and the escape scan disagreed, the text segment discarded i
// without consuming it and the outer loop did not progress. With a "\033[" at
// the end of a line that is a render hang, not an exotic mutant.
//
// These cases pin it down without depending on the alphabet: they run as-is,
// and a non-advance shows up as a timeout instead of as a wrong value.
func TestParseAnsiSegmentsTerminates(t *testing.T) {
	for _, in := range []string{
		"\033",                       // lone ESC: not enough to be CSI
		"\033[",                      // CSI without final byte
		"a\033",                      // text + truncated ESC
		"a\033[",                     // text + incomplete CSI
		"\033[1",                     // CSI with parameter but no final byte
		"a\033[1m",                   // complete CSI after text
		"\033[0m\033[",               // complete CSI and then truncated
		"[\033[",                     // lone bracket + truncated CSI
		"\033[[",                     // double bracket after ESC
		"\033[\033[",                 // ESC inside a CSI
		"\033[\033",                  // truncated ESC inside a CSI
		"\033[;;;;;;",                // long CSI without final byte
		"\033[1;2;3;4;5",             // parameterized CSI without final byte
		strings.Repeat("\033[", 200), // many truncated CSIs in a row
		strings.Repeat("a", 5000),    // long text with no CSI
		strings.Repeat("a\033[1m", 500),
	} {
		// That the call finishes is the assertion. The content is checked
		// only so that the case is a test and not a compilation smoke: an
		// empty text segment would be as bad as a hang.
		got := parseAnsiSegments(in)
		if len(got) == 0 && in != "" {
			t.Errorf("%q: all segments were lost", in)
			continue
		}
		// Rebuild: every character of the input must appear once, in one
		// segment, and the rebuilt text must equal the input without styles.
		var text strings.Builder
		for _, seg := range got {
			text.WriteString(seg.text)
		}
		if want := stripCSI(in); text.String() != want {
			t.Errorf("%q: reconstructed text %q, want %q", in, text.String(), want)
		}
	}
}

// stripCSI removes the complete CSI sequences, which is the text the parser
// must return as styled segments.
func stripCSI(s string) string {
	var out strings.Builder
	for len(s) > 0 {
		if strings.HasPrefix(s, csiPrefix) {
			end := len(s)
			if k := strings.IndexFunc(s[len(csiPrefix):], isCSIFinalizer); k >= 0 {
				end = len(csiPrefix) + k + 1
			}
			s = s[end:]
			continue
		}
		end := strings.Index(s, csiPrefix)
		if end < 0 {
			end = len(s)
		}
		if end == 0 {
			end = 1
		}
		out.WriteString(s[:end])
		s = s[end:]
	}
	return out.String()
}

// --- Internal helpers ----------------------------------------------------

func TestRepeatStyled(t *testing.T) {
	if got := repeatStyled(nil, "-", 3); got != "---" {
		t.Errorf("repeatStyled(nil) = %q, want ---", got)
	}
	if got := repeatStyled(nil, "-", 0); got != "" {
		t.Errorf("repeatStyled(nil, -, 0) = %q, want empty", got)
	}
	if got := repeatStyled(nil, "ab", 2); got != "abab" {
		t.Errorf("repeatStyled with a 2-char value = %q, want abab", got)
	}

	style := ansi.NewStyle()
	withStyle := repeatStyled(&style, "-", 3)
	if ansi.StringWidth(withStyle) != 3 {
		t.Errorf("with style the width must still be 3, it is %d", ansi.StringWidth(withStyle))
	}
}

func TestStyledChar(t *testing.T) {
	if got := styledChar(nil, "x"); got != "x" {
		t.Errorf("styledChar(nil) = %q, want x", got)
	}
	style := ansi.NewStyle()
	if got := styledChar(&style, "x"); ansi.Strip(got) != "x" {
		t.Errorf("styledChar with style = %q, want x after strip", got)
	}
}
