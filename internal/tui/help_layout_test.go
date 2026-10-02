package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// El modal de ayuda se centra sobre el contenido. La posición y el ancho
// importan: un anchoPreferred distinto, o un total con bordes mal contado,
// desplaza la caja varias columnas y no se nota en una captura rápida.
//
// Los números esperados son literales, no derivados de helpModalWidth. Derivar
// la expectativa de la constante la convierte en una tautología: si la constante
// cambia, el test la sigue y pasa.

// helpBox devuelve el borde superior de la caja del modal de ayuda recortado a
// su ancho, y la columna en la que empieza.
//
// El fondo se hace de líneas tan anchas como la pantalla porque OverlayLine
// pega el modal justo donde acaba el fondo cuando éste es más corto: con un
// fondo estrecho la caja se pegaría al texto y la posición medida no diría nada
// del centrado.
func helpBox(t *testing.T, m *Model) (box string, col int) {
	t.Helper()
	fondo := make([]string, 40)
	for i := range fondo {
		fondo[i] = strings.Repeat("·", m.width)
	}

	out := ansi.Strip(m.renderHelpModal(strings.Join(fondo, "\n")))
	for _, line := range strings.Split(out, "\n") {
		// Runes, no bytes: tanto el punto del fondo como la esquina del recuadro
		// son multibyte, y strings.Index daría la posición en bytes. Con puntos
		// de dos bytes, la caja parecía estar al doble de su columna.
		runes := []rune(line)
		for i, r := range runes {
			if r != '╭' {
				continue
			}
			ancho := 48 // el ancho interior preferido más los dos bordes
			if m.width < ancho {
				ancho = m.width
			}
			if i+ancho > len(runes) {
				t.Fatalf("la caja se sale de la línea en la columna %d: %q", i, line)
			}
			return string(runes[i : i+ancho]), i
		}
	}
	t.Fatalf("el modal de ayuda no dibujó ninguna caja:\n%s", out)
	return "", 0
}

// En pantallas anchas la caja va centrada: 46 de interior más 2 de bordes son 48,
// y sobre 120 columnas sobra (120-48)/2 = 36 a cada lado.
func TestHelpModalCentred(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		wantCol       int
		wantBoxWidth  int
		skipIfRecorta bool
	}{
		// Ancho sobrado: caja de 48 centrada.
		{"muy ancha", 120, 36, 48, false},
		{"ancha", 60, 6, 48, false},
		// Paridad impar: con 49 columnas, un total de 47 daría startX 1 y uno de
		// 48 daría 0. Esta fila es la que distingue el +2 del +1.
		{"impar justa", 49, 0, 48, false},
		// Ya no cabe el ancho preferido: modalWidthFor deja w-2 y los bordes se
		// los gastan, así que la caja llega al borde sin margen.
		{"justa", 48, 0, 48, false},
		{"una menos", 47, 0, 47, false},
		{"estrecha", 30, 0, 30, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.width = tt.width

			box, col := helpBox(t, m)
			if got := len([]rune(box)); got != tt.wantBoxWidth {
				t.Errorf("en %d columnas la caja mide %d, want %d", tt.width, got, tt.wantBoxWidth)
			}
			if col != tt.wantCol {
				t.Errorf("la caja empieza en la columna %d, want %d", col, tt.wantCol)
			}
		})
	}
}

// El fondo se ve a los dos lados de la caja: el modal se superpone, no borra.
func TestHelpModalKeepsBackgroundOnBothSides(t *testing.T) {
	m := newTestModel(t)
	m.width = 120
	fondo := make([]string, 40)
	for i := range fondo {
		fondo[i] = strings.Repeat("·", m.width)
	}

	out := ansi.Strip(m.renderHelpModal(strings.Join(fondo, "\n")))
	for _, line := range strings.Split(out, "\n") {
		runes := []rune(line)
		for i, r := range runes {
			if r != '╭' {
				continue
			}
			izquierda := runes[:i]
			derecha := runes[i+48:]
			if n := countDot(izquierda); n != 36 {
				t.Errorf("a la izquierda de la caja hay %d puntos, want 36", n)
			}
			if n := countDot(derecha); n != 36 {
				t.Errorf("a la derecha de la caja hay %d puntos, want 36", n)
			}
			return
		}
	}
	t.Fatal("el modal de ayuda no dibujó ninguna caja")
}

func countDot(runes []rune) int {
	n := 0
	for _, r := range runes {
		if r == '·' {
			n++
		}
	}
	return n
}
