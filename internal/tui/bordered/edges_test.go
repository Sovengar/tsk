package bordered

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// Los bordes de una caja tienen tres suelos --el ancho total, el interior y el
// relleno de cada línea-- y cada suelo tenía su comparación con un borde que
// ningún test podía alcanzar. Estos tests los alcanzan por los dos lados.
//
// El fondo común es el mismo en los tres: una clamped a un mínimo se puede
// escribir con max y queda como una identidad en el borde, o como una condición
// cuyo borde hay que provocar. Con max() no hay borde que provocar: no hay
// comparación que mutar.

// cajaDe es un atajo para una caja redondeada con el ancho pedido.
func cajaDe(width int) lipgloss.Border { return lipgloss.RoundedBorder() }

func render(t *testing.T, content string, width int) string {
	t.Helper()
	return RenderWithTitleEx(cajaDe(width), nil, AlignLeft, " T ", content, width)
}

func lineasDe(s string) []string { return strings.Split(strings.TrimRight(s, "\n"), "\n") }

// El suelo del ancho total: dos columnas, una por esquina. Un width de 1 sube a
// 2 y la caja se dibuja sin interior.
func TestAnchoMinimoDeLaCaja(t *testing.T) {
	t.Run("por debajo del mínimo la caja no colapsa", func(t *testing.T) {
		for _, w := range []int{-5, 0, 1} {
			out := render(t, "hola", w)
			lineas := lineasDe(out)
			if len(lineas) < 3 {
				t.Fatalf("width=%d: la caja tiene %d líneas, want al menos 3 (arriba, medio, abajo)",
					w, len(lineas))
			}
			// La línea del medio tiene que existir y no reventar: es el interior
			// de anchura 0.
			if !strings.Contains(lineas[1], "│") {
				t.Errorf("width=%d: la línea del medio no tiene bordes: %q", w, lineas[1])
			}
		}
	})

	t.Run("exactamente el mínimo", func(t *testing.T) {
		out := render(t, "", 2)
		lineas := lineasDe(out)
		if len(lineas) != 3 {
			t.Errorf("con width=2 hay %d líneas, want 3", len(lineas))
		}
		// El interior de una caja de 2 es 0 columnas: los dos bordes juntos.
		if n := len([]rune(strings.Trim(lineas[1], "│"))); n != 0 {
			t.Errorf("el interior mide %d columnas, want 0", n)
		}
	})
}

// El suelo del interior: no puede ser negativo porque las esquinas anchas de un
// borde de terceros se comen el ancho entero. Con un borde de esquinas de tres
// columnas y width 3, el interior es -3.
func TestInteriorNoPuedeSerNegativo(t *testing.T) {
	ancho := lipgloss.Border{
		Top: "-", Bottom: "-", Left: "|", Right: "|",
		TopLeft: "/3./", TopRight: "/3./",
		BottomLeft: "\\3.\\", BottomRight: "\\3.\\",
	}
	// Con un ancho menor que las esquinas, el interior sale negativo.
	for _, w := range []int{2, 3, 4} {
		out := RenderWithTitleEx(ancho, nil, AlignLeft, "", "contenido", w)
		for i, l := range lineasDe(out) {
			if strings.Contains(l, "Repeat") || strings.Contains(l, "panic") {
				t.Errorf("w=%d línea %d: la salida parece rota: %q", w, i, l)
			}
		}
	}
	// Y con un borde normal, el interior es el ancho menos 2.
	out := render(t, "abcd", 10)
	lineas := lineasDe(out)
	if n := len([]rune(strings.Trim(lineas[1], "│ "))); n != 4 {
		t.Errorf("con width=10 el interior visible mide %d columnas, want 4", n)
	}
}

// El recorte del título: cuando el título ya cabe entero, el recorte es una
// identidad y tiene que NOTARSE que no se ha recortado. Un título de anchura
// exactamente igual al interior es el borde, y ahí el `>` y el `>=` dan lo mismo
// -- por eso hace falta un título que llene el interior con un carácter menos y
// uno que lo llene de más.
func TestElTituloSeRecortaSoloCuandoNoCabe(t *testing.T) {
	const ancho = 20 // interior = 18

	t.Run("cabe de sobra", func(t *testing.T) {
		out := RenderWithTitleEx(cajaDe(ancho), nil, AlignLeft, " corto ", "contenido", ancho)
		if !strings.Contains(out, "corto") {
			t.Errorf("el título corto ha desaparecido: %q", out)
		}
	})

	t.Run("llena el interior exacto", func(t *testing.T) {
		// 18 columnas de título = exactamente el interior.
		titulo := strings.Repeat("T", 18)
		out := RenderWithTitleEx(cajaDe(ancho), nil, AlignLeft, titulo, "x", ancho)
		if n := strings.Count(out, "T"); n != 18 {
			t.Errorf("salen %d caracteres de título, want 18: el interior es de 18 y el título llena", n)
		}
	})

	t.Run("no cabe y se recorta", func(t *testing.T) {
		titulo := strings.Repeat("T", 25)
		out := RenderWithTitleEx(cajaDe(ancho), nil, AlignLeft, titulo, "x", ancho)
		if n := strings.Count(out, "T"); n != 18 {
			t.Errorf("un título de 25 sale con %d caracteres, want 18 (el ancho del interior)", n)
		}
	})

	t.Run("el interior no cambia por culpa del título", func(t *testing.T) {
		for _, largo := range []int{0, 5, 18, 25, 100} {
			out := RenderWithTitleEx(cajaDe(ancho), nil, AlignLeft,
				strings.Repeat("T", largo), "contenido", ancho)
			lineas := lineasDe(out)
			for i, l := range lineas {
				if n := len([]rune(l)); n != ancho {
					t.Errorf("título de %d, línea %d mide %d columnas, want %d",
						largo, i, n, ancho)
				}
			}
		}
	})
}

// El suelo del relleno: una palabra más larga que el interior se desborda, y el
// suelo en cero es lo que evita que strings.Repeat reviente con un número
// negativo. Este es el caso que de verdad importa -- un título de tarea largo en
// una terminal estrecha -- y el que un `max` deja escrito.
func TestPalabraMasAnchaQueElInterior(t *testing.T) {
	const ancho = 12 // interior = 10
	larga := strings.Repeat("x", 40)

	out := RenderWithTitleEx(cajaDe(ancho), nil, AlignLeft, "", larga, ancho)

	lineas := lineasDe(out)
	if len(lineas) < 3 {
		t.Fatalf("sólo %d líneas: la palabra larga no ha producido nada", len(lineas))
	}
	// Todas las líneas miden el ancho de la caja, ni una más.
	for i, l := range lineas {
		if n := len([]rune(l)); n != ancho {
			t.Errorf("línea %d mide %d columnas, want %d: %q", i, n, ancho, l)
		}
	}
	// Y la palabra sale entera en varias líneas, no cortada a media columna.
	if got := strings.Count(out, "x"); got != 40 {
		t.Errorf("salen %d caracteres de la palabra, want 40: el relleno la truncó", got)
	}
}

// El relleno de cada línea con ancho variable, que es lo que hace el suelo útil
// en el caso normal: un contenido que ocupa justo el interior, uno que se queda
// corto y uno que se pasa.
func TestRellenoSegunElAnchoDeCadaLinea(t *testing.T) {
	const ancho = 20 // interior = 18
	contenido := "corto\n" + strings.Repeat("y", 18) + "\n" + strings.Repeat("z", 19)

	out := RenderWithTitleEx(cajaDe(ancho), nil, AlignLeft, "", contenido, ancho)
	lineas := lineasDe(out)

	if len(lineas) < 5 {
		t.Fatalf("sólo %d líneas para 3 de contenido", len(lineas))
	}
	for i, l := range lineas {
		if n := len([]rune(l)); n != ancho {
			t.Errorf("línea %d mide %d columnas, want %d: %q", i, n, ancho, l)
		}
	}
}

// parseAnsiSegments parte una cadena con códigos ANSI en trozos de estilo y de
// texto. El caso que importa es una CSI que empieza JUSTO después del primer
// carácter: es el texto con color de toda la vida, y es donde la búsqueda de la
// siguiente CSI devuelve 0.
//
// Devolver 0 es justo lo que el código anterior no podía distinguir: buscaba
// desde el principio de s, y como el `if strings.HasPrefix` de arriba ya había
// descartado que s empezara por CSI, ese 0 no ocurría nunca. Con la búsqueda
// empezando en s[1:] el 0 es un valor de verdad, y esta comprobación lo ata.
func TestParseAnsiSegmentsParteEnLaPrimeraCSI(t *testing.T) {
	const esc = "\x1b["
	rojo := esc + "31m"
	fin := "\x1b[0m"

	casos := []struct {
		nombre  string
		entrada string
		want    []ansiSegment
	}{
		{"texto y luego CSI", "a" + rojo + "b",
			[]ansiSegment{
				{style: "", text: "a"},
				{style: rojo, text: ""},
				{style: "", text: "b"},
			}},
		{"CSI en el segundo carácter", "x" + rojo + "y" + fin + "z",
			[]ansiSegment{
				{style: "", text: "x"},
				{style: rojo, text: ""},
				{style: "", text: "y"},
				{style: fin, text: ""},
				{style: "", text: "z"},
			}},
		{"CSI justo al principio", rojo + "b",
			[]ansiSegment{
				{style: rojo, text: ""},
				{style: "", text: "b"},
			}},
		{"CSI al final", "a" + rojo,
			[]ansiSegment{
				{style: "", text: "a"},
				{style: rojo, text: ""},
			}},
		{"CSI truncada", "a" + "\x1b[31",
			[]ansiSegment{
				{style: "", text: "a"},
				{style: "\x1b[31", text: ""},
			}},
		{"CSI vacía", "a" + esc + "m" + "b",
			[]ansiSegment{
				{style: "", text: "a"},
				{style: esc + "m", text: ""},
				{style: "", text: "b"},
			}},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			got := parseAnsiSegments(tc.entrada)
			if len(got) != len(tc.want) {
				t.Fatalf("segmentos = %d, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i].style != tc.want[i].style || got[i].text != tc.want[i].text {
					t.Errorf("segmento %d = {style:%q text:%q}, want {style:%q text:%q}",
						i, got[i].style, got[i].text, tc.want[i].style, tc.want[i].text)
				}
			}
			// Lo que de verdad importa: rearmar los segmentos devuelve el
			// original, sin perder ni un byte de estilo.
			var b strings.Builder
			for _, seg := range got {
				b.WriteString(seg.style)
				b.WriteString(seg.text)
			}
			if b.String() != tc.entrada {
				t.Errorf("rearmando sale %q, want %q", b.String(), tc.entrada)
			}
		})
	}
}

// El ESC suelto: no abre CSI, así que es texto, y tiene que avanzar el bucle por
// sí solo. Este caso es el que impedía añadir un "end = 1" de seguridad, y de
// paso comprueba que la búsqueda desde s[1:] no se come ESCs intermedios.
func TestParseAnsiSegmentsConEscSuelto(t *testing.T) {
	entrada := "a\x1bb"
	got := parseAnsiSegments(entrada)

	if len(got) != 1 {
		t.Fatalf("segmentos = %d, want 1 (un ESC suelto es texto, no estilo): %+v", len(got), got)
	}
	if got[0].style != "" || got[0].text != entrada {
		t.Errorf("segmento = {style:%q text:%q}, want {style:\"\" text:%q}",
			got[0].style, got[0].text, entrada)
	}
}

// Un texto que empieza y acaba en ESC, con CSI en medio: comprueba que el primer
// carácter se emite como texto y no se pierde por buscar desde s[1:].
func TestParseAnsiSegmentsNoPierdeElPrimerCaracter(t *testing.T) {
	const rojo = "\x1b[31m"
	entrada := "á" + rojo + "é" // 'á' ocupa dos bytes en UTF-8

	got := parseAnsiSegments(entrada)
	if len(got) != 3 {
		t.Fatalf("segmentos = %d, want 3: %+v", len(got), got)
	}
	if got[0].text != "á" {
		t.Errorf("el primer segmento es %q, want %q: el primer carácter se ha partido", got[0].text, "á")
	}
	if got[2].text != "é" {
		t.Errorf("el último segmento es %q, want %q", got[2].text, "é")
	}
}
