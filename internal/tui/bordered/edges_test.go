package bordered

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Los bordes tienen tres suelos y un par de bordes de comparación que sólo se
// ven con entradas que nadie construía: una caja de dos columnas, un título más
// ancho que la caja, y texto ANSI malformado.

// Una caja necesita al menos dos columnas: con menos, el borde no tiene sitio
// para los dos lados y el resultado sería una caja de anchura negativa.
func TestRenderWidthFloor(t *testing.T) {
	for _, width := range []int{-5, 0, 1, 2, 3} {
		out := RenderWithTitleEx(lipgloss.RoundedBorder(), nil, AlignLeft, " T ", "cuerpo", width)
		lines := strings.Split(out, "\n")
		if len(lines) < 3 {
			t.Fatalf("con width %d salen %d líneas", width, len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w < 2 {
				t.Errorf("con width %d la línea %d mide %d: %q", width, i, w, ansi.Strip(l))
			}
		}
		if !strings.HasPrefix(ansi.Strip(lines[0]), "╭") {
			t.Errorf("con width %d la primera línea no es el borde superior: %q", width, ansi.Strip(lines[0]))
		}
	}
}

// Por debajo del ancho del contenido, la caja se ajusta a lo que el contenido
// necesita en vez de encogerse: un borde más estrecho que su propia caja
// interior no se puede dibujar, y lo que sale es una caja del tamaño del texto.
//
// Es el motivo del suelo de dos columnas: nunca se dibuja una caja de anchura
// negativa, y si el mínimo no cabe se cede al contenido.
func TestRenderWithTitleBelowContentWidth(t *testing.T) {
	out := ansi.Strip(RenderWithTitleEx(lipgloss.RoundedBorder(), nil, AlignLeft, " T ", "cuerpo", 2))
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("salen %d líneas: %q", len(lines), out)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w < 2 {
			t.Errorf("la línea %d mide %d: %q", i, w, l)
		}
	}
	if !strings.Contains(lines[1], "cuerpo") {
		t.Errorf("el contenido no está: %q", lines[1])
	}
}

// Un título más ancho que la caja se recorta al ancho interior en vez de
// desbordar el borde.
func TestRenderWithTitleWiderThanBox(t *testing.T) {
	largo := strings.Repeat("T", 100)
	out := ansi.Strip(RenderWithTitleEx(lipgloss.RoundedBorder(), nil, AlignLeft, largo, "cuerpo", 20))
	for i, l := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(l); w > 20 {
			t.Errorf("la línea %d mide %d, más que la caja de 20: %q", i, w, l)
		}
	}
}

// El título alineado a la derecha también se recorta, y el relleno va por delante.
func TestRenderWithTitleRightAlignWiderThanBox(t *testing.T) {
	out := ansi.Strip(RenderWithTitleEx(lipgloss.RoundedBorder(), nil, AlignRight, strings.Repeat("T", 100), "cuerpo", 20))
	for i, l := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(l); w > 20 {
			t.Errorf("la línea %d mide %d: %q", i, w, l)
		}
	}
}

// Una línea de contenido que cabe justa no se envuelve: son una fila, no dos.
func TestContentExactlyFitsDoesNotWrap(t *testing.T) {
	// Caja de 10: 2 de bordes, 8 de interior.
	body := strings.Repeat("x", 8)
	out := ansi.Strip(RenderWithTitleEx(lipgloss.RoundedBorder(), nil, AlignLeft, "", body, 10))
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Errorf("una línea de 8 en una caja de 10 sale en %d líneas: %q", len(lines), out)
	}
	if interior := strings.Trim(lines[1], "│║ "); interior != body {
		t.Errorf("el contenido es %q, want %q", interior, body)
	}
}

// Y una de 9 sí se envuelve en dos.
func TestContentOneColumnTooLongWraps(t *testing.T) {
	out := ansi.Strip(RenderWithTitleEx(lipgloss.RoundedBorder(), nil, AlignLeft, "", strings.Repeat("x", 9), 10))
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Errorf("una línea de 9 en una caja de 10 sale en %d líneas, want 4: %q", len(lines), out)
	}
}

// El recorrido de segmentos es donde el guard de "ESC suelto" vive, así que se
// comprueba ahí y no sobre el render: ansi.Strip se come un ESC suelto junto con
// el carácter que sigue, y una aserción sobre el texto limpio no distinguiría
// "el bucle se paró" de "el b se perdió al limpiar".
func TestParseAnsiSegmentsHandlesMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		wantN int
		// El último segmento es el texto sin estilo, y debe acabar con el resto
		// de la entrada.
		wantLastSuffix string
	}{
		{
			// ESC [ 3 sin byte final: se consume entero, como está documentado.
			name:           "secuencia truncada se consume entera",
			in:             "\\x1b[3",
			wantN:          1,
			wantLastSuffix: "\x1b[3",
		},
		{
			// ESC [ t sí es una secuencia completa: "t" es un byte final válido.
			name:           "secuencia corta con final válido",
			in:             "\\x1b[ttexto",
			wantN:          2,
			wantLastSuffix: "texto",
		},
		{
			// Un ESC que no abre CSI: es texto, y el bucle tiene que avanzar o se
			// quedaría aquí para siempre. Este test cuelga si la guarda falta.
			name:           "ESC suelto es texto",
			in:             "a\\x1bb",
			wantN:          1,
			wantLastSuffix: "a\x1bb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := unquote(tt.in)
			segments := parseAnsiSegments(in)
			if len(segments) != tt.wantN {
				t.Fatalf("salieron %d segmentos, want %d: %+v", len(segments), tt.wantN, segments)
			}
			last := segments[len(segments)-1]
			if !strings.HasSuffix(last.text+last.style, tt.wantLastSuffix) {
				t.Errorf("el último segmento es %+v, want que acabe en %q", last, tt.wantLastSuffix)
			}
		})
	}
}

// unquote convierte los \x1b de la tabla en bytes de escape, para que la tabla
// se lea sin barras.
func unquote(s string) string {
	return strings.NewReplacer("\\x1b", "\x1b", "\\n", "\n").Replace(s)
}
