package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Estas son las funciones de layout más antiguas del programa: llevan desde antes
// que el resto y sus tests sólo cubrían el camino feliz. Todas son puras y todas
// tienen bordes que nadie miró: el ancho cero, el texto que cabe justo, y el
// límite más pequeño que el sufijo.

// truncateLines recorta cada línea al ancho dado. Es lo que evita que el helper
// de bordes re-wrappee una línea y añada filas de más.
func TestTruncateLines(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"corta lo que no cabe", "abcdefghij", 4, "abcd"},
		{"corta justo lo que cabe", "abcd", 4, "abcd"},
		{"una columna de más", "abcde", 4, "abcd"},
		{"vacío", "", 4, ""},
		{"una línea", "a\nbb\nccc", 2, "a\nbb\ncc"},
		{"ancho de una columna", "abc", 1, "a"},
		{"ancho cero no recorta", "abc", 0, "abc"},
		{"ancho negativo no recorta", "abc", -5, "abc"},
		{"línea vacía entre dos", "ab\n\ncd", 3, "ab\n\ncd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateLines(tt.in, tt.width); got != tt.want {
				t.Errorf("truncateLines(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
			}
		})
	}
}

// El ancho se mide en columnas de display, no en bytes: los glifos del borde son
// multibyte y un slice por bytes los partiría por la mitad.
func TestTruncateLinesCountsColumnsNotBytes(t *testing.T) {
	// "─" son tres bytes y una columna.
	linea := strings.Repeat("─", 10) // 30 bytes
	got := truncateLines(linea, 5)
	if want := strings.Repeat("─", 5); got != want {
		t.Errorf("truncateLines = %q (%d runes), want %q", got, len([]rune(got)), want)
	}
}

// Y los códigos ANSI no cuentan como ancho.
func TestTruncateLinesIgnoresAnsi(t *testing.T) {
	conColor := "\x1b[31mabc\x1b[0m"
	if got := truncateLines(conColor, 5); ansi.Strip(got) != "abc" {
		t.Errorf("con color: %q", ansi.Strip(got))
	}
	if got := truncateLines(conColor, 2); ansi.StringWidth(got) > 2 {
		t.Errorf("con color y recorte, la línea mide %d", ansi.StringWidth(got))
	}
}

// Nunca devuelve una línea más ancha que el límite, salvo que el límite sea
// inválido, en cuyo caso devuelve el original sin tocar.
func TestTruncateLinesRespectsWidth(t *testing.T) {
	for width := 1; width <= 12; width++ {
		for _, in := range []string{"a", "ab", "abcdefghijklmn", strings.Repeat("─", 30), "x\n" + strings.Repeat("y", 20)} {
			for _, line := range strings.Split(truncateLines(in, width), "\n") {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("truncateLines(%q, %d) devolvió una línea de %d columnas", in, width, w)
				}
			}
		}
	}
}

// cellWidth rellena a la derecha hasta el ancho y trunca a la izquierda lo que
// exceda. Con un ancho menor que uno no hay celda, así que devuelve vacío.
func TestCellWidth(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"rellena", "ab", 5, "ab   "},
		{"exacto", "abcde", 5, "abcde"},
		{"trunca", "abcdefgh", 5, "abc.."},
		{"vacío se rellena entero", "", 3, "   "},
		// Con una sola columna no cabe ni el sufijo "..", así que la celda queda
		// en blanco en vez de cortada a media letra.
		{"ancho de uno", "abc", 1, " "},
		{"ancho cero", "abc", 0, ""},
		{"ancho negativo", "abc", -3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cellWidth(tt.in, tt.width); got != tt.want {
				t.Errorf("cellWidth(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
			}
		})
	}
}

// Toda celda sale con el ancho pedido, medido en columnas, salvo las de ancho
// inválido, que salen vacías.
func TestCellWidthAlwaysFills(t *testing.T) {
	for width := 1; width <= 12; width++ {
		for _, in := range []string{"", "a", "abcdefghij", strings.Repeat("─", 20), "\x1b[31mab\x1b[0m"} {
			if got := ansi.StringWidth(cellWidth(in, width)); got != width {
				t.Errorf("cellWidth(%q, %d) mide %d, want %d", in, width, got, width)
			}
		}
	}
}

// truncate acorta dejando dos puntos al final. Por debajo del tamaño del sufijo no
// cabe la elipsis, así que recorta a pelo; y sin ese suelo, s[:max-2] con max de
// 0 o 1 hace un slice con índice negativo y revienta.
func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"corta lo que no cabe", "abcdefghij", 5, "abc.."},
		{"corta justo lo que cabe", "abcde", 5, "abcde"},
		{"una columna de más", "abcdef", 5, "abc.."},
		{"vacío", "", 5, ""},
		{"el límite es el sufijo", "abc", 2, ".."},
		{"el límite es un punto", "abc", 1, "a"},
		{"límite cero", "abc", 0, ""},
		{"límite negativo", "abc", -4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.in, tt.limit); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}

// Nunca devuelve más caracteres de los pedidos, ni revienta con un límite
// negativo.
func TestTruncateNeverExceedsLimit(t *testing.T) {
	for limit := -5; limit <= 12; limit++ {
		for _, in := range []string{"", "a", "abcdefghijklmn"} {
			got := truncate(in, limit)
			if limit <= 0 {
				if got != "" {
					t.Fatalf("truncate(%q, %d) = %q, want vacío", in, limit, got)
				}
				continue
			}
			if len(got) > limit {
				t.Errorf("truncate(%q, %d) = %q (%d caracteres)", in, limit, got, len(got))
			}
		}
	}
}

// visibleRange ya tiene casos sueltos en layout_test.go; lo que faltaba eran los
// bordes y un barrido que comprobara los tres invariantes a la vez.
func TestVisibleRangeEdges(t *testing.T) {
	tests := []struct {
		name                string
		cursor, total, size int
		wantStart           int
		wantEnd             int
	}{
		{"cabe justo", 0, 10, 10, 0, 10},
		{"cursor por encima del final", 99, 20, 5, 15, 20},
		{"cursor negativo", -5, 20, 5, 0, 5},
		{"total negativo", 0, -3, 5, 0, 0},
		{"tamaño cero", 3, 20, 0, 0, 0},
		{"tamaño negativo", 3, 20, -2, 0, 0},
		{"tamaño de uno", 4, 10, 1, 4, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visibleRange(tt.cursor, tt.total, tt.size)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.cursor, tt.total, tt.size, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// Lo único que la ventana garantiza es que cabe dentro del total y que el cursor
// queda dentro de la ventana, cuando el tamaño lo permite.
func TestVisibleRangeProperties(t *testing.T) {
	for total := 0; total <= 30; total++ {
		for size := 0; size <= 30; size++ {
			for cursor := -5; cursor <= 35; cursor++ {
				start, end := visibleRange(cursor, total, size)

				if start < 0 || end < start {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): rango imposible",
						cursor, total, size, start, end)
				}
				if end > total {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): se sale del total",
						cursor, total, size, start, end)
				}
				if total <= 0 || size <= 0 {
					if start != 0 || end != 0 {
						t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d), want (0, 0)",
							cursor, total, size, start, end)
					}
					continue
				}
				if size < total && end-start != size {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): ventana de %d, want %d",
						cursor, total, size, start, end, end-start, size)
				}
				acotado := clampTo(cursor, total)
				if size < total && (acotado < start || acotado >= end) {
					t.Fatalf("cursor=%d total=%d size=%d -> (%d, %d): el cursor quedó fuera",
						cursor, total, size, start, end)
				}
			}
		}
	}
}
