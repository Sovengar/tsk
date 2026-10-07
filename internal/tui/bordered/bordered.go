package bordered

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	AlignCenter = iota
	AlignLeft
	AlignRight
)

// minBorderWidth is the minimum width of a box: two columns, one for each
// corner. Below that there is no interior left, so the box would be drawn but
// with no room for the content.
const minBorderWidth = 2

func RenderWithTitleEx(border lipgloss.Border, borderFg color.Color, align int, title, content string, width int) string {
	return RenderWithTitlesEx(border, borderFg, title, align, "", AlignLeft, content, width)
}

// RenderWithTitlesEx renders a border with a title on the top line and
// other text on the bottom line, each with its own alignment. An empty title
// is not drawn (the line is fully padded).
func RenderWithTitlesEx(border lipgloss.Border, borderFg color.Color, topTitle string, topAlign int, bottomTitle string, bottomAlign int, content string, width int) string {
	// The floor is two columns, one for each corner of the border: below that,
	// there is no interior left to draw anything in. It is written with max
	// and not with an if on purpose -- it is a clamp and max says what it does;
	// the if was only needed because there was no way to say it in one line.
	width = max(width, minBorderWidth)

	topLeft := border.TopLeft
	topRight := border.TopRight
	bottomLeft := border.BottomLeft
	bottomRight := border.BottomRight
	topChar := border.Top
	leftChar := border.Left
	rightChar := border.Right
	bottomChar := border.Bottom

	if topChar == "" {
		topChar = " "
	}
	if leftChar == "" {
		leftChar = " "
	}
	if rightChar == "" {
		rightChar = " "
	}
	if bottomChar == "" {
		bottomChar = " "
	}

	tlW := ansi.StringWidth(topLeft)
	trW := ansi.StringWidth(topRight)

	// Floor at zero because strings.Repeat blows up with a negative number, and
	// because a negative interior cannot be padded. With the width floor it is
	// no longer reachable -- tlW + trW are the characters of a corner, one each
	// -- but the floor stays: the wide corners of a third-party border could
	// go past it, and then the floor is what prevents the panic.
	innerWidth := max(width-tlW-trW, 0)

	var borderStyle *ansi.Style
	if borderFg != nil {
		s := ansi.NewStyle().ForegroundColor(borderFg)
		borderStyle = &s
	}

	topLine := buildBorderLine(borderStyle, topLeft, topChar, topRight, innerWidth, topAlign, topTitle)
	contentLines := buildContentLines(borderStyle, leftChar, rightChar, content, innerWidth)
	bottomLine := buildBorderLine(borderStyle, bottomLeft, bottomChar, bottomRight, innerWidth, bottomAlign, bottomTitle)

	var b strings.Builder
	b.WriteString(topLine)
	b.WriteString("\n")
	for i, line := range contentLines {
		b.WriteString(line)
		if i < len(contentLines)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(bottomLine)

	return b.String()
}

func repeatStyled(style *ansi.Style, char string, n int) string {
	s := strings.Repeat(char, n)
	if style != nil {
		return style.Styled(s)
	}
	return s
}

func styledChar(style *ansi.Style, char string) string {
	if style != nil {
		return style.Styled(char)
	}
	return char
}

func buildBorderLine(style *ansi.Style, left, fill, right string, innerWidth, align int, title string) string {
	titleDisplay := ansi.Strip(title)
	titleWidth := ansi.StringWidth(string(titleDisplay))

	// The truncation ALWAYS happens, with no condition. ansi.Truncate is an
	// identity when the text already fits -- verified with a text of exactly
	// the same width as the limit -- and that is why the
	// `if titleWidth > innerWidth` that was here was a branch that only told
	// itself apart from not writing it in a case where both give the same. Now there is no branch: it truncates, and if it fit nothing changes.
	//
	// The width is truncated with min and not by reassigning it inside an if,
	// for the same reason.
	title = ansi.Truncate(title, innerWidth, "")
	titleWidth = min(titleWidth, innerWidth)

	remaining := innerWidth - titleWidth
	var leftPad, rightPad int

	switch align {
	case AlignLeft:
		leftPad = 0
		rightPad = remaining
	case AlignRight:
		leftPad = remaining
		rightPad = 0
	default:
		leftPad = remaining / 2
		rightPad = remaining - leftPad
	}

	leftPadStr := repeatStyled(style, fill, leftPad)
	rightPadStr := repeatStyled(style, fill, rightPad)
	titleStyled := styledChar(style, title)

	return styledChar(style, left) + leftPadStr + titleStyled + rightPadStr + styledChar(style, right)
}

func buildContentLines(style *ansi.Style, leftChar, rightChar, content string, innerWidth int) []string {
	rawLines := strings.Split(content, "\n")
	var result []string

	for _, line := range rawLines {
		displayWidth := ansi.StringWidth(line)

		if displayWidth <= innerWidth {
			padding := innerWidth - displayWidth
			paddedLine := line + strings.Repeat(" ", padding)
			result = append(result, styledChar(style, leftChar)+paddedLine+styledChar(style, rightChar))
		} else {
			wrapped := wrapLine(line, innerWidth)
			for _, wl := range wrapped {
				// Each wrapped line fits innerWidth... or not: wrapLine cuts by
				// columns and a word longer than the interior overflows. The floor
				// at zero is what prevents the panic of strings.Repeat, and a floor
				// at "<= 0" would do the same because negative padding is the only
				// thing that has to be prevented.
				padding := max(innerWidth-ansi.StringWidth(wl), 0)
				result = append(result,
					styledChar(style, leftChar)+wl+strings.Repeat(" ", padding)+styledChar(style, rightChar))
			}
		}
	}

	// The loop always leaves at least one line: strings.Split returns at
	// least one element, and each turn of the loop adds at least one. The
	// defense that was here against an empty result was unreachable.
	return result
}

type ansiSegment struct {
	style string
	text  string
}

// isCSIFinalizer tells whether r is the final byte of a CSI sequence.
//
// The 0x40..0x7E range is the "final bytes" range of ECMA-48: any other
// byte of the sequence is a parameter.
func isCSIFinalizer(r rune) bool {
	return r >= 0x40 && r <= 0x7E
}

const csiPrefix = "\x1b["

// parseAnsiSegments splits a line into style and text segments.
//
// It is solved with strings.Index and IndexFunc instead of loops that advance
// an index by hand. It is not a stylistic detail: a loop with `i++` guarantees
// the progress only because nobody inverts the `++`, and that is exactly the
// class of mutant that shows up here. With an index that advances inside a
// library, or a slice truncation that always shortens, the progress does not
// depend on an expression keeping saying what it says today.
//
// The two searches are by BYTES, and here it does not matter: a CSI is ASCII
// and so is the final byte, so cutting right after it never splits a glyph.
func parseAnsiSegments(s string) []ansiSegment {
	var segments []ansiSegment

	for len(s) > 0 {
		if strings.HasPrefix(s, csiPrefix) {
			// First final byte after the prefix; if the sequence is truncated,
			// it is consumed whole.
			end := len(s)
			if k := strings.IndexFunc(s[len(csiPrefix):], isCSIFinalizer); k >= 0 {
				end = len(csiPrefix) + k + 1
			}
			segments = append(segments, ansiSegment{style: s[:end], text: ""})
			s = s[end:]
			continue
		}

		// Text: up to the next CSI or up to the end.
		//
		// The search starts at s[1:], not at s. Here we already know s does NOT
		// start with CSI -- the HasPrefix above would have caught it -- so
		// searching it from the beginning could only give 0, which is exactly the
		// value the `>= 0` did not reach and for which the mutant did not tell:
		// with `k > 0` the k == 0 case was unreachable and for the same reason indistinguishable.
		//
		// Searching from 1, a k == 0 means "the CSI starts right after the
		// first character", which is the normal case of colored text. So the 0
		// is a real value and the comparison can be checked from both sides.
		end := len(s)
		if k := strings.Index(s[1:], csiPrefix); k >= 0 {
			end = 1 + k
		}
		segments = append(segments, ansiSegment{style: "", text: s[:end]})
		s = s[end:]
	}

	return segments
}

func wrapLine(line string, maxDisplayWidth int) []string {
	if maxDisplayWidth <= 0 {
		return []string{line}
	}

	segments := parseAnsiSegments(line)

	var chunks []string
	var current []rune
	currentWidth := 0
	activeStyle := ""

	flush := func() {
		if len(current) == 0 {
			return
		}
		text := string(current)
		if activeStyle != "" {
			text = activeStyle + text + "\033[0m"
		}
		chunks = append(chunks, text)
		current = current[:0]
		currentWidth = 0
	}

	for _, seg := range segments {
		for _, r := range seg.text {
			rw := ansi.StringWidth(string(r))
			if currentWidth+rw > maxDisplayWidth {
				flush()
			}
			current = append(current, r)
			currentWidth += rw
		}
		if seg.style == "\033[0m" {
			flush()
			activeStyle = ""
		} else if seg.style != "" {
			activeStyle = seg.style
		}
	}

	flush()

	if len(chunks) == 0 {
		chunks = append(chunks, "")
	}

	return chunks
}
