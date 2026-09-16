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

func RenderWithTitleEx(border lipgloss.Border, borderFg color.Color, align int, title, content string, width int) string {
	return RenderWithTitlesEx(border, borderFg, title, align, "", AlignLeft, content, width)
}

// RenderWithTitlesEx renderiza un borde con un título en la línea superior y
// otro texto en la línea inferior, cada uno con su propia alineación. Un título
// vacío no se dibuja (la línea queda rellena por completo).
func RenderWithTitlesEx(border lipgloss.Border, borderFg color.Color, topTitle string, topAlign int, bottomTitle string, bottomAlign int, content string, width int) string {
	if width < 2 {
		width = 2
	}

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

	innerWidth := width - tlW - trW
	if innerWidth < 0 {
		innerWidth = 0
	}

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

	if titleWidth > innerWidth {
		title = ansi.Truncate(title, innerWidth, "")
		titleWidth = innerWidth
	}

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
				displayWidth := ansi.StringWidth(wl)
				padding := innerWidth - displayWidth
				if padding < 0 {
					padding = 0
				}
				paddedLine := wl + strings.Repeat(" ", padding)
				result = append(result, styledChar(style, leftChar)+paddedLine+styledChar(style, rightChar))
			}
		}
	}

	if len(result) == 0 {
		emptyLine := strings.Repeat(" ", innerWidth)
		result = append(result, styledChar(style, leftChar)+emptyLine+styledChar(style, rightChar))
	}

	return result
}

type ansiSegment struct {
	style string
	text  string
}

func parseAnsiSegments(s string) []ansiSegment {
	var segments []ansiSegment
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\033' && i+1 < len(runes) && runes[i+1] == '[' {
			j := i + 2
			for j < len(runes) {
				b := runes[j]
				if b >= 0x40 && b <= 0x7E {
					j++
					break
				}
				j++
			}
			segments = append(segments, ansiSegment{style: string(runes[i:j]), text: ""})
			i = j
			continue
		}
		j := i
		for j < len(runes) && !(runes[j] == '\033' && j+1 < len(runes) && runes[j+1] == '[') {
			j++
		}
		segments = append(segments, ansiSegment{style: "", text: string(runes[i:j])})
		i = j
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
