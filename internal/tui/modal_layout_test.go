package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// modalWidthFor y overlayModal son la base de todos los modales: el primero
// decide cuánto ancho tienen y el segundo dónde caen. Los tests que había los
// usaban de paso, así que sus bordes -- el umbral exacto y el relleno cuando el
// modal es más alto que el fondo -- no los cubría nadie.

func TestModalWidthFor(t *testing.T) {
	tests := []struct {
		name         string
		preferred, w int
		want         int
	}{
		{"pref entra de sobra", 52, 120, 52},
		{"pref exactamente en el límite", 58, 60, 58},
		{"pref una de más", 59, 60, 58},
		{"no cabe", 52, 40, 38},
		{"pantalla de dos columnas", 52, 2, 0},
		{"pantalla de tres columnas", 52, 3, 1},
		{"pantalla de una columna", 52, 1, -1},
		{"pref de cero", 0, 40, 0},
		{"pref negativo", -5, 40, -5},
		{"pantalla negativa", 52, -10, -12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modalWidthFor(tt.preferred, tt.w); got != tt.want {
				t.Errorf("modalWidthFor(%d, %d) = %d, want %d", tt.preferred, tt.w, got, tt.want)
			}
		})
	}
}

// El umbral es "el modal más sus dos bordes tiene que caber": en w-2 columnas
// de pantalla el modal preferido entra justo, y una más ya no.
func TestModalWidthForThreshold(t *testing.T) {
	const pref = 52
	if got := modalWidthFor(pref, pref+2); got != pref {
		t.Errorf("en %d columnas el modal es %d, want %d", pref+2, got, pref)
	}
	if got := modalWidthFor(pref, pref+1); got != pref+1-2 {
		t.Errorf("en %d columnas el modal es %d, want %d", pref+1, got, pref-1)
	}
}

// overlayModal centra el modal y deja el fondo alrededor. Con un fondo del mismo
// alto que el modal, el vertical no se nota; con uno más alto, el modal va en
// medio.
func TestOverlayModalCentresVertically(t *testing.T) {
	modal := strings.Join([]string{"╭──╮", "│  │", "╰──╯"}, "\n") // 3 líneas

	for _, fondo := range []struct {
		name    string
		alto    int
		wantTop int
	}{
		{"mismo alto, sin relleno", 3, 0},
		{"una de más", 4, 0},
		{"dos de más", 5, 1},
		{"cuatro de más", 7, 2},
		{"diez de más", 13, 5},
	} {
		t.Run(fondo.name, func(t *testing.T) {
			lineas := make([]string, fondo.alto)
			for i := range lineas {
				lineas[i] = strings.Repeat("·", 40)
			}

			out := strings.Split(overlayModal(strings.Join(lineas, "\n"), modal, 4, 40), "\n")
			if len(out) != fondo.alto {
				t.Fatalf("salieron %d líneas, want %d", len(out), fondo.alto)
			}
			primera := -1
			for i, l := range out {
				if strings.Contains(l, "╭") {
					primera = i
					break
				}
			}
			if primera != fondo.wantTop {
				t.Errorf("el modal empieza en la línea %d, want %d", primera, fondo.wantTop)
			}
		})
	}
}

// Con un fondo más corto que el modal, el fondo se rellena para que quepa entero.
// Sin ese relleno el modal se saldría de las líneas.
func TestOverlayModalPadsShortContent(t *testing.T) {
	modal := strings.Join([]string{"╭──╮", "│  │", "╰──╯"}, "\n")

	out := strings.Split(overlayModal(strings.Repeat("·", 40), modal, 4, 40), "\n")
	if len(out) != 3 {
		t.Fatalf("salieron %d líneas, want 3", len(out))
	}
	for i, l := range out {
		if !strings.Contains(l, "╭") && !strings.Contains(l, "│") && !strings.Contains(l, "╰") {
			t.Errorf("la línea %d no tiene el modal: %q", i, l)
		}
	}
}

// Y con un modal más alto que un fondo vacío, el relleno son líneas en blanco, no
// puntos: el fondo no tenía nada que conservar.
func TestOverlayModalPadsEmptyContent(t *testing.T) {
	modal := strings.Join([]string{"╭──╮", "│  │", "╰──╯"}, "\n")

	out := strings.Split(overlayModal("", modal, 4, 40), "\n")
	if len(out) < 3 {
		t.Fatalf("salieron %d líneas, want al menos 3", len(out))
	}
}

// El modal se centra también a lo horizontal, dejando fondo a los dos lados.
func TestOverlayModalCentresHorizontally(t *testing.T) {
	modal := "╭──╮" // 4 columnas
	fondo := strings.Repeat("·", 40)

	// Sin ANSI: los códigos de reset que OverlayLine intercala cuentan como runes
	// pero no ocupan columna, así que hay que quitarlos antes de medir.
	out := strings.Split(ansi.Strip(overlayModal(fondo, modal, 4, 40)), "\n")
	if len(out) != 1 {
		t.Fatalf("salieron %d líneas, want 1", len(out))
	}
	// Columna, no byte: "·" son dos bytes y "╭" tres, así que strings.Index daría
	// una posición distinta de la que ocupa en pantalla.
	i := len([]rune(out[0][:strings.Index(out[0], "╭")]))
	if i != 18 {
		t.Errorf("el modal empieza en la columna %d, want 18 (centrado en 40)", i)
	}
	if strings.Count(out[0], "·") != 36 {
		t.Errorf("quedan %d columnas de fondo, want 36", strings.Count(out[0], "·"))
	}
}

// Un modal más ancho que la pantalla se pega al borde en vez de empezar en
// columna negativa.
func TestOverlayModalWiderThanScreen(t *testing.T) {
	modal := strings.Repeat("x", 60)

	out := ansi.Strip(overlayModal(strings.Repeat("·", 20), modal, 60, 20))
	if !strings.HasPrefix(out, "x") {
		t.Errorf("un modal más ancho que la pantalla no va al principio: %.20q", out)
	}
	if strings.Contains(out, "·") {
		t.Errorf("quedó fondo dentro de un modal que lo cubre entero: %.40q", out)
	}
}
