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

// minAnchoBorde es el ancho mínimo de una caja: dos columnas, una por cada
// esquina. Por debajo no queda interior, así que la caja se dibujaría pero sin
// hueco para el contenido.
const minAnchoBorde = 2

func RenderWithTitleEx(border lipgloss.Border, borderFg color.Color, align int, title, content string, width int) string {
	return RenderWithTitlesEx(border, borderFg, title, align, "", AlignLeft, content, width)
}

// RenderWithTitlesEx renderiza un borde con un título en la línea superior y
// otro texto en la línea inferior, cada uno con su propia alineación. Un título
// vacío no se dibuja (la línea queda rellena por completo).
func RenderWithTitlesEx(border lipgloss.Border, borderFg color.Color, topTitle string, topAlign int, bottomTitle string, bottomAlign int, content string, width int) string {
	// El suelo son dos columnas, una por cada esquina del borde: por debajo, no
	// queda interior donde dibujar nada. Se escribe con max y no con un if a
	// propósito -- es una clamped y max dice lo que hace; el if hacía falta sólo
	// porque no había forma de decirlo en una línea.
	width = max(width, minAnchoBorde)

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

	// Suelo en cero porque strings.Repeat revienta con un número negativo, y
	// porque un interior negativo no se puede rellenar. Con el suelo de width ya
	// no es alcanzable -- tlW + trW son los caracteres de una esquina, uno cada
	// uno -- pero el suelo se queda: las esquinas anchas de un borde de terceros
	// podrían pasarse, y entonces el suelo es lo que evita el pánico.
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

	// El recorte va SIEMPRE, sin condición. ansi.Truncate es identidad cuando el
	// texto ya cabe -- verificado con un texto de anchura exactamente igual al
	// límite -- y por eso el `if titleWidth > innerWidth` que había aquí era una
	// rama que sólo se distinguía de no-escribirla por un caso donde las dos
	// dan lo mismo. Ahora no hay rama: se recorta, y si cabía no cambia nada.
	//
	// El ancho se recorta con min y no reasignándolo dentro de un if, por el
	// mismo motivo.
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
				// Cada línea envuelta cabe en innerWidth... o no: wrapLine corta
				// por columnas y una palabra más larga que el interior se
				// desborda. El suelo en cero es lo que evita el pánico de
				// strings.Repeat, y el suelo en "<= 0" haría lo mismo porque
				// padding negativo es lo único que hay que impedir.
				padding := max(innerWidth-ansi.StringWidth(wl), 0)
				result = append(result,
					styledChar(style, leftChar)+wl+strings.Repeat(" ", padding)+styledChar(style, rightChar))
			}
		}
	}

	// El bucle siempre deja al menos una línea: strings.Split devuelve como
	// mínimo un elemento, y cada vuelta del bucle añade uno al menos. La defensa
	// que había aquí contra un resultado vacío era inalcanzable.
	return result
}

type ansiSegment struct {
	style string
	text  string
}

// isCSIFinalizer dice si r es el byte final de una secuencia CSI.
//
// El rango 0x40..0x7E es el de los "final bytes" de ECMA-48: cualquier otro
// byte de la secuencia es un parámetro.
func isCSIFinalizer(r rune) bool {
	return r >= 0x40 && r <= 0x7E
}

const csiPrefix = "\x1b["

// parseAnsiSegments parte una línea en segmentos de estilo y de texto.
//
// Se resuelve con strings.Index e IndexFunc en vez de con bucles que avanzan un
// índice a mano. No es un detalle estilístico: un bucle con `i++` garantiza el
// progreso sólo porque nadie invierte el `++`, y esa es exactamente la clase de
// mutante que sale aquí. Con un índice que avanza dentro de una librería, o con
// un recorte de slice que siempre acorta, el progreso no depende de que una
// expresión siga diciendo lo que dice hoy.
//
// Las dos búsquedas son por BYTES, y aquí da igual: una CSI es ASCII y el byte
// final también, así que cortar justo después de él nunca parte un glifo.
func parseAnsiSegments(s string) []ansiSegment {
	var segments []ansiSegment

	for len(s) > 0 {
		if strings.HasPrefix(s, csiPrefix) {
			// Primer final byte después del prefijo; si la secuencia está
			// truncada, se consume entera.
			end := len(s)
			if k := strings.IndexFunc(s[len(csiPrefix):], isCSIFinalizer); k >= 0 {
				end = len(csiPrefix) + k + 1
			}
			segments = append(segments, ansiSegment{style: s[:end], text: ""})
			s = s[end:]
			continue
		}

		// Texto: hasta la siguiente CSI o hasta el final.
		//
		// La búsqueda empieza en s[1:], no en s. Aquí ya sabemos que s NO
		// empieza por CSI -- el HasPrefix de arriba lo habría cogido -- así que
		// buscarla desde el principio sólo podía dar 0, que es justo el valor que
		// el `>= 0` no alcanzaba y por el que el mutante no se distinguía: con `k > 0`
		// el caso k == 0 era inalcanzable y por lo mismo indistinguible.
		//
		// Buscando desde el 1, un k == 0 significa "la CSI empieza justo después
		// del primer carácter", que es el caso normal de un texto con color. Así
		// el 0 es un valor de verdad y la comparación se puede comprobar por los
		// dos lados.
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
