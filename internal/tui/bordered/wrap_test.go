package bordered

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// linesplit parte la salida en líneas ya sin ANSI, que es lo que se compara.
func linesplit(t *testing.T, out string) []string {
	t.Helper()
	raw := strings.Split(out, "\n")
	out2 := make([]string, len(raw))
	for i, l := range raw {
		out2[i] = ansi.Strip(l)
	}
	return out2
}

// boxLines separa las líneas de borde del contenido.
func boxLines(t *testing.T, out string) (top, bottom string, body []string) {
	t.Helper()
	ls := linesplit(t, out)
	if len(ls) < 3 {
		t.Fatalf("una caja necesita al menos 3 líneas, tiene %d: %q", len(ls), out)
	}
	return ls[0], ls[len(ls)-1], ls[1 : len(ls)-1]
}

// Todas las líneas de una caja miden exactamente `width`. Es la invariante que
// sostiene el layout entero: si una línea se descuadra, el modal se ve roto.
func assertUniformWidth(t *testing.T, out string, width int) {
	t.Helper()
	for i, l := range linesplit(t, out) {
		if w := ansi.StringWidth(l); w != width {
			t.Errorf("línea %d mide %d, want %d: %q", i, w, width, l)
		}
	}
}

func roundBorder() lipgloss.Border { return lipgloss.RoundedBorder() }

// unwrap quita el primer y el último carácter de una fila de caja. Trabaja con
// runes: los caracteres del borde son multibyte y un slice por bytes parte el
// glifo por la mitad.
func unwrap(t *testing.T, line string) string {
	t.Helper()
	r := []rune(ansi.Strip(line))
	if len(r) < 2 {
		t.Fatalf("fila demasiado corta para quitarle el marco: %q", line)
	}
	return string(r[1 : len(r)-1])
}

// --- Ancho mínimo y anchos imposibles ------------------------------------

// Por debajo de 2 columnas no hay caja posible: el ancho sube a 2 en vez de
// producir un borde de anchura cero o negativa.
//
// Con el interior en 0 el contenido no se puede recortar (wrapLine devuelve la
// línea entera cuando no hay ancho), así que las filas de contenido pueden
// exceder la caja. Lo que sí tiene que cumplirse es que el borde superior e
// inferior midan lo mismo y que el render no reviente con anchuras imposibles.
func TestRenderMinimumWidth(t *testing.T) {
	for _, width := range []int{-5, 0, 1, 2, 3} {
		out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", "x", width)
		top, bottom, body := boxLines(t, out)
		if len(body) == 0 {
			t.Errorf("width=%d: la caja debe tener al menos una fila de contenido", width)
		}
		if ansi.StringWidth(top) != ansi.StringWidth(bottom) {
			t.Errorf("width=%d: borde superior mide %d y el inferior %d, deben coincidir",
				width, ansi.StringWidth(top), ansi.StringWidth(bottom))
		}
		// El valor del recorte importa: por debajo de 2 la caja mide 2, así que
		// el borde superior son 2 guiones entre esquinas y nada más.
		if width < 2 && top != "╭╮" {
			t.Errorf("width=%d: borde superior = %q, want el mínimo ╭╮", width, top)
		}
		// A 2 columnas el interior es 0, así que no cabe ni un carácter de
		// relleno entre las esquinas.
		if width == 2 && top != "╭╮" {
			t.Errorf("width=2: borde superior = %q, want ╭╮ (interior 0)", top)
		}
	}
}

// Un borde con esquinas anchas (caracteres de doble ancho) puede dejar el
// interior en cero o negativo; nunca debe ser negativo.
func TestRenderWithWideCorners(t *testing.T) {
	b := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "｛", TopRight: "｝", BottomLeft: "｟", BottomRight: "～",
	}
	// Las esquinas ocupan 2 columnas cada una y los laterales 1, así que con
	// interior >= 1 el borde mide width y el contenido mide width - 2.
	for _, width := range []int{5, 6, 8, 10, 20} {
		inner := width - 4
		out := RenderWithTitlesEx(b, nil, "", AlignLeft, "", AlignLeft, "contenido largo", width)
		top, bottom, body := boxLines(t, out)

		if got := ansi.StringWidth(top); got != width {
			t.Errorf("width=%d: borde superior mide %d, want %d: %q", width, got, width, top)
		}
		if got := ansi.StringWidth(bottom); got != width {
			t.Errorf("width=%d: borde inferior mide %d, want %d: %q", width, got, width, bottom)
		}
		for i, l := range body {
			if got := ansi.StringWidth(l); got != inner+2 {
				t.Errorf("width=%d: fila %d mide %d, want %d: %q", width, i, got, inner+2, l)
			}
		}
	}
}

// --- Caracteres de borde ausentes ----------------------------------------

// Un borde con caracteres vacíos se rellena con espacios: sin esto la caja
// saldría con huecos y las líneas no cuadrarían.
func TestRenderFillsMissingBorderChars(t *testing.T) {
	// width 24, interior 22, contenido "cuerpo" (6 columnas) + 16 de relleno.
	const (
		topOK    = "┌" + "──────────────────────" + "┐"
		bottomOK = "└" + "──────────────────────" + "┘"
		bodyOK   = "│cuerpo                │"
	)
	_ = bottomOK

	tests := []struct {
		name   string
		border lipgloss.Border
		want   []string
	}{
		{
			name: "sin línea superior",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Left: "│", Right: "│", Bottom: "─",
			},
			// La línea superior se rellena de espacios: es el hueco visible.
			want: []string{"┌" + "                      " + "┐", bodyOK, bottomOK},
		},
		{
			name: "sin línea inferior",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Left: "│", Right: "│", Top: "─",
			},
			want: []string{topOK, bodyOK, "└" + "                      " + "┘"},
		},
		{
			name: "sin laterales",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
				Top: "─", Bottom: "─",
			},
			// Sin │ el contenido va pegado al borde, con un espacio a cada lado.
			want: []string{topOK, " cuerpo                 ", bottomOK},
		},
		{
			name: "sin línea superior ni laterales",
			border: lipgloss.Border{
				TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘", Bottom: "─",
			},
			want: []string{"┌" + "                      " + "┐", " cuerpo                 ", bottomOK},
		},
		{
			name: "sin horizontales",
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
			out := RenderWithTitleEx(tt.border, nil, AlignLeft, "", "cuerpo", width)
			// Si el carácter de borde faltara en vez de sustituirse por un
			// espacio, alguna línea mediría menos que el ancho pedido.
			assertUniformWidth(t, out, width)

			// Salida exacta: fija QUÉ línea lleva el hueco y con qué carácter.
			// Un aserto laxo ("alguna línea tiene un espacio") no distingue un
			// relleno de espacios de un borde dibujado con otro carácter.
			want := strings.Join(tt.want, "\n")
			if got := ansi.Strip(out); got != want {
				t.Errorf("salida =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// Cuando las esquinas anchas dejan el interior en negativo, se recorta a 0 en
// vez de propagar un interior negativo (que daría un relleno negativo y un
// panic en strings.Repeat).
func TestRenderWideCornersWithNegativeInnerWidth(t *testing.T) {
	b := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "｛", TopRight: "｝", BottomLeft: "｟", BottomRight: "～",
	}
	for _, width := range []int{1, 2, 3, 4} {
		// No debe entrar en panic: es la razón del recorte a 0.
		out := RenderWithTitlesEx(b, nil, "", AlignLeft, "", AlignLeft, "abc", width)
		top, bottom, body := boxLines(t, out)
		if ansi.StringWidth(top) != ansi.StringWidth(bottom) {
			t.Errorf("width=%d: bordes descuadrados: %q vs %q", width, top, bottom)
		}
		// Con el interior recortado a 0 no hay a dónde envolver, así que el
		// contenido queda en UNA fila. Si el recorte fuera a 1 en vez de a 0,
		// "abc" se partiría en tres filas de una columna.
		if len(body) != 1 {
			t.Errorf("width=%d: %d filas de cuerpo, want 1 (interior recortado a 0): %q", width, len(body), body)
		}
	}
}

// --- Contenido multilínea ------------------------------------------------

// Cada salto de línea del contenido produce una fila de la caja, y sólo una.
func TestRenderContentLinePerNewline(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"una línea", "a", 1},
		{"tres líneas", "a\nb\nc", 3},
		{"línea vacía al principio", "\na", 2},
		{"línea vacía al final", "a\n", 2},
		{"sólo saltos", "\n\n", 3},
		{"con saltos intercalados", "a\n\nb", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const width = 20
			out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", tt.content, width)
			_, _, body := boxLines(t, out)
			if len(body) != tt.want {
				t.Errorf("líneas de cuerpo = %d, want %d: %q", len(body), tt.want, body)
			}
			assertUniformWidth(t, out, width)
		})
	}
}

// Una línea más ancha que el interior se ENVUELVE en varias filas, no se
// descarta ni se recorta.
func TestRenderWrapsWideContent(t *testing.T) {
	const width = 12
	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", strings.Repeat("x", 30), width)
	_, _, body := boxLines(t, out)

	if len(body) < 3 {
		t.Fatalf("30 columnas en 10 de interior deberían envolver en 3+, hay %d: %q", len(body), body)
	}
	assertUniformWidth(t, out, width)
	for i, l := range body {
		inner := unwrap(t, l)
		if strings.TrimSpace(inner) != strings.Repeat("x", len([]rune(inner))) {
			t.Errorf("cuerpo %d = %q, want sólo x", i, inner)
		}
	}
}

// Una línea coloreada que mide EXACTAMENTE el interior no debe partirse. La
// rama del relleno usa `<=` y no `<` a propósito: aunque el ancho cuadre, la
// línea puede llevar cambios de estilo en medio, y envolver partiría ahí. Con
// `<` esa fila se rompería en dos y el alto del modal cambiaría.
func TestRenderColoredLineAtExactInnerWidth(t *testing.T) {
	const width = 16
	// 14 de interior. Coloreamos un texto que mide EXACTAMENTE 14 columnas y
	// cuyo cambio de estilo cae a mitad: "aaaa" rojo + "bbbbbbbbbb" verde.
	//
	// Aquí está el borde `<=` vs `<` de la rama de relleno. Con `<=` la línea
	// entra en la rama de relleno (1 fila) porque su ancho visible ya cabe
	// justo. Con `<` (mutado) cae en la de envolver, y wrapLine parte en el
	// cambio de estilo: 2 filas y el alto del modal cambia.
	styled := "\033[31m" + strings.Repeat("a", 4) + "\033[0m" +
		"\033[32m" + strings.Repeat("b", 10) + "\033[0m"
	if got := ansi.StringWidth(ansi.Strip(styled)); got != 14 {
		t.Fatalf("fixture: el contenido mide %d columnas, want 14", got)
	}

	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", styled, width)
	_, _, body := boxLines(t, out)
	if len(body) != 1 {
		t.Errorf("contenido de 14 columnas en interior 14: %d filas, want 1 (no debe partirse): %q", len(body), body)
	}
	assertUniformWidth(t, out, width)
}

// El relleno hasta el ancho interior es exacto en cada fila.
func TestRenderPadsContentToInnerWidth(t *testing.T) {
	const width = 16
	// "abcdefghijklmn" mide justo los 14 de interior: con el borde mal puesto
	// (`<` en vez de `<=`) esta fila se envolvería en dos y el alto cambiaría.
	tests := []string{"", "x", "ab", "abc", "abcdefghij", "abcdefghijklmn"}
	for _, content := range tests {
		out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "", content, width)
		_, _, body := boxLines(t, out)
		if len(body) != 1 {
			t.Fatalf("contenido %q: %d filas, want 1", content, len(body))
		}
		inner := unwrap(t, body[0])
		if ansi.StringWidth(inner) != width-2 {
			t.Errorf("contenido %q: interior mide %d, want %d (%q)", content, ansi.StringWidth(inner), width-2, inner)
		}
		if !strings.HasPrefix(inner, content) {
			t.Errorf("contenido %q: el relleno se comió texto (%q)", content, inner)
		}
		if strings.TrimRight(inner, " ") != content {
			t.Errorf("contenido %q: sólo debe añadir espacios a la derecha (%q)", content, inner)
		}
	}
}

// Con el interior en 0 no hay a dónde envolver, así que wrapLine devuelve la
// línea entera y el relleno sale negativo: se recorta a 0 en vez de restar.
func TestRenderNegativePaddingIsClamped(t *testing.T) {
	b := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "｛", TopRight: "｝", BottomLeft: "｟", BottomRight: "～",
	}
	// width 3 -> interior negativo -> 0. El contenido no cabe y aun así el
	// relleno no puede ser negativo.
	out := RenderWithTitlesEx(b, nil, "", AlignLeft, "", AlignLeft, "cuerpo", 3)
	_, _, body := boxLines(t, out)
	// El interior es 0 y "cuerpo" no se puede envolver ni recortar, así que la
	// fila es el contenido tal cual entre los laterales, SIN relleno extra: un
	// `padding = 1` en vez de `padding = 0` añadiría un espacio de más.
	if len(body) != 1 {
		t.Fatalf("cuerpo = %q, want una fila", body)
	}
	if body[0] != "│cuerpo│" {
		t.Errorf("fila = %q, want │cuerpo│ (relleno recortado a 0)", body[0])
	}
}

// --- Alineación de los títulos del borde ---------------------------------

// La alineación decide dónde cae el relleno: a la izquierda del texto, a la
// derecha, o repartido.
func TestRenderTitleAlignment(t *testing.T) {
	const width = 30
	tests := []struct {
		name        string
		align       int
		wantPrefix  string // lo que hay antes del título, sin la esquina
		wantPostfix int    // caracteres de relleno después del título
	}{
		{"izquierda", AlignLeft, "╭", width - 3},
		{"derecha", AlignRight, "", 0},
		{"centro", AlignCenter, "", 0}, // el resto se comprueba por ancho
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderWithTitleEx(roundBorder(), nil, tt.align, "TIT", "", width)
			top, _, _ := boxLines(t, out)
			assertUniformWidth(t, out, width)

			runes := []rune(top)
			idx := strings.Index(top, "TIT")
			if idx < 0 {
				t.Fatalf("el título no aparece: %q", top)
			}
			runeIdx := len([]rune(top[:idx]))
			_ = runes
			switch tt.align {
			case AlignLeft:
				if !strings.HasPrefix(top, tt.wantPrefix) {
					t.Errorf("align left = %q, want prefijo %q", top, tt.wantPrefix)
				}
			case AlignRight:
				// Alinear a la derecha deja el relleno ANTES del título: los
				// guiones van de la esquina al principio del texto.
				got := runeIdx - 1
				want := width - 2 - len("TIT")
				if got != want {
					t.Errorf("align right: %d guiones antes del título, want %d (%q)", got, want, top)
				}
				if len(runes)-1-(runeIdx+len("TIT")) != 0 {
					t.Errorf("align right: sobra relleno tras el título (%q)", top)
				}
			case AlignCenter:
				// El relleno sobrante se reparte: la izquierda nunca excede a
				// la derecha en más de uno.
				left := runeIdx - 1 // sin la esquina
				right := len(runes[runeIdx+len("TIT") : len(runes)-1])
				if left > right+1 || right > left+1 {
					t.Errorf("centro: %d a la izquierda y %d a la derecha no están repartidos (%q)", left, right, top)
				}
				if left+right+len("TIT") != width-2 {
					t.Errorf("centro: %d+%d+%d != %d", left, right, len("TIT"), width-2)
				}
			}
		})
	}
}

// Un título más ancho que el interior se trunca al ancho disponible, en vez de
// desbordar la caja.
func TestRenderTruncatesOverlongTitle(t *testing.T) {
	const width = 10 // 8 de interior
	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, "TITULO DEMASIADO LARGO", "", width)
	assertUniformWidth(t, out, width)

	top, _, _ := boxLines(t, out)
	if strings.Contains(top, "LARGO") {
		t.Errorf("el título debería truncarse, pero aparece entero: %q", top)
	}
	if !strings.Contains(top, "TIT") {
		t.Errorf("debería conservar el principio del título: %q", top)
	}
}

// El ancho del título se mide en COLUMNAS, no en bytes: un título con acentos o
// emoji ocupa menos de lo que su longitud en runes sugiere.
func TestRenderMeasuresTitleInColumns(t *testing.T) {
	const width = 14
	wide := "áéí" // 6 runes, 3 columnas
	out := RenderWithTitleEx(roundBorder(), nil, AlignLeft, wide, "", width)
	assertUniformWidth(t, out, width)

	top, _, _ := boxLines(t, out)
	if !strings.Contains(top, wide) {
		t.Errorf("el título cabe de sobra y debe aparecer entero: %q", top)
	}
}

// --- Colores -------------------------------------------------------------

// Con color de borde, el ANSI envuelve las líneas pero el ancho visible no
// cambia: si el estilo alterara el ancho, la caja se descuadraría en pantalla.
func TestRenderWithBorderColorKeepsVisibleWidth(t *testing.T) {
	const width = 24
	fg := color.RGBA{R: 0x88, G: 0x00, B: 0xAA, A: 0xFF}
	out := RenderWithTitleEx(roundBorder(), fg, AlignLeft, " T ", "cuerpo del texto", width)

	if !strings.Contains(out, "\033[") {
		t.Error("con color de borde la salida debe incluir secuencias ANSI")
	}
	assertUniformWidth(t, out, width)
}

// --- wrapLine: la lógica de envolver conservando el estilo ---------------

// Envolver no puede partir una secuencia ANSI por la mitad ni perder el color.
func TestWrapLinePreservesAnsiStyles(t *testing.T) {
	red := "\033[31m"
	reset := "\033[0m"
	line := red + strings.Repeat("palabra ", 6) + reset

	chunks := wrapLine(line, 10)
	if len(chunks) < 2 {
		t.Fatalf("una línea de %d columnas en 10 debería partirse: %v", ansi.StringWidth(line), chunks)
	}
	for i, c := range chunks {
		if w := ansi.StringWidth(c); w > 10 {
			t.Errorf("chunk %d mide %d, want <= 10: %q", i, w, c)
		}
		if !strings.Contains(c, red) {
			t.Errorf("chunk %d perdió el estilo: %q", i, c)
		}
		if !strings.HasSuffix(c, reset) {
			t.Errorf("chunk %d no cierra el estilo: %q", i, c)
		}
	}
	// Y nada de texto se pierde.
	var joined string
	for _, c := range chunks {
		joined += ansi.Strip(c)
	}
	if strings.TrimSpace(joined) != strings.TrimSpace(ansi.Strip(line)) {
		t.Errorf("el texto envuelto difiere:\n original: %q\n unido:   %q", ansi.Strip(line), joined)
	}
}

// Con ancho máximo no positivo no se envuelve nada: se devuelve la línea tal
// cual, porque no hay a dónde partirla.
func TestWrapLineNonPositiveWidth(t *testing.T) {
	for _, w := range []int{0, -1, -10} {
		line := "texto largo"
		got := wrapLine(line, w)
		if len(got) != 1 {
			t.Fatalf("wrapLine(%q, %d) devolvió %d líneas, want 1", line, w, len(got))
		}
		if got[0] != line {
			t.Errorf("wrapLine(%q, %d) = %q, want la línea intacta", line, w, got[0])
		}
	}
}

// Una línea de texto sin estilo que no cabe se parte por columnas exactas.
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

// Una línea corta cabe entera y no se parte.
func TestWrapLineShortLineUnchanged(t *testing.T) {
	got := wrapLine("corta", 10)
	if len(got) != 1 || got[0] != "corta" {
		t.Errorf("wrapLine(%q, 10) = %v, want una línea intacta", "corta", got)
	}
}

// Una línea con estilo que cambia a mitad debe partirse de forma coherente: cada
// trozo lleva su propio estilo, no el de antes ni el de después.
func TestWrapLineHandlesStyleChange(t *testing.T) {
	line := "\033[31mrojo\033[0m\033[32mverde\033[0m"
	chunks := wrapLine(line, 5)
	for i, c := range chunks {
		if ansi.StringWidth(c) > 5 {
			t.Errorf("chunk %d mide %d, want <= 5: %q", i, ansi.StringWidth(c), c)
		}
	}
	joined := ""
	for _, c := range chunks {
		joined += ansi.Strip(c)
	}
	if joined != "rojoverde" {
		t.Errorf("texto unido = %q, want rojoverde", joined)
	}
}

// --- parseAnsiSegments ---------------------------------------------------

// El parser separa estilo y texto, conservando el orden.
func TestParseAnsiSegments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []ansiSegment
	}{
		{
			name: "texto plano",
			in:   "hola",
			want: []ansiSegment{{style: "", text: "hola"}},
		},
		{
			name: "estilo al principio",
			in:   "\033[31mrojo",
			want: []ansiSegment{{style: "\033[31m", text: ""}, {style: "", text: "rojo"}},
		},
		{
			name: "estilo en medio",
			in:   "a\033[1mb",
			want: []ansiSegment{{style: "", text: "a"}, {style: "\033[1m", text: ""}, {style: "", text: "b"}},
		},
		{
			name: "reset solo",
			in:   "\033[0m",
			want: []ansiSegment{{style: "\033[0m", text: ""}},
		},
		{
			name: "varios estilos",
			in:   "\033[1ma\033[31mb",
			want: []ansiSegment{
				{style: "\033[1m", text: ""},
				{style: "", text: "a"},
				{style: "\033[31m", text: ""},
				{style: "", text: "b"},
			},
		},
		{
			// 0x7E ('~') es el último byte válido de una secuencia CSI: el
			// bucle tiene que incluirlo o la secuencia se trunca y el resto del
			// texto se lee como si fuera parte del estilo.
			name: "CSI terminado en ~",
			in:   "\033[1~después",
			want: []ansiSegment{
				{style: "\033[1~", text: ""},
				{style: "", text: "después"},
			},
		},
		{
			name: "CSI con parámetros y ~",
			in:   "\033[3;5~x",
			want: []ansiSegment{
				{style: "\033[3;5~", text: ""},
				{style: "", text: "x"},
			},
		},
		{
			name: "cadena vacía",
			in:   "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAnsiSegments(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d segmentos %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("segmento %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- Helpers internos ----------------------------------------------------

func TestRepeatStyled(t *testing.T) {
	if got := repeatStyled(nil, "-", 3); got != "---" {
		t.Errorf("repeatStyled(nil) = %q, want ---", got)
	}
	if got := repeatStyled(nil, "-", 0); got != "" {
		t.Errorf("repeatStyled(nil, -, 0) = %q, want vacío", got)
	}
	if got := repeatStyled(nil, "ab", 2); got != "abab" {
		t.Errorf("repeatStyled con char de 2 = %q, want abab", got)
	}

	style := ansi.NewStyle()
	withStyle := repeatStyled(&style, "-", 3)
	if ansi.StringWidth(withStyle) != 3 {
		t.Errorf("con estilo el ancho debe seguir siendo 3, es %d", ansi.StringWidth(withStyle))
	}
}

func TestStyledChar(t *testing.T) {
	if got := styledChar(nil, "x"); got != "x" {
		t.Errorf("styledChar(nil) = %q, want x", got)
	}
	style := ansi.NewStyle()
	if got := styledChar(&style, "x"); ansi.Strip(got) != "x" {
		t.Errorf("styledChar con estilo = %q, want x tras strip", got)
	}
}
