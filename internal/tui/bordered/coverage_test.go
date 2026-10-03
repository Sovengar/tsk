package bordered

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// timeout es un plazo generoso: si el bucle de parseo se cuelga, lo que se
// quiere es que el test lo diga, no que cuelgue la suite entera.
func timeout() <-chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(2*time.Second, func() { close(ch) })
	return ch
}

// Los bordes tienen tres caminos de "nada que dibujar" que ningún test pisaba:
// una caja cuyo contenido se queda vacío, un ESC suelto en medio del texto (que
// no abre CSI y por tanto es texto), y un texto sin estilos que se parte en un
// único trozo.

// Una caja sin contenido tiene que ser una caja, no una cadena vacía: el borde
// es lo que la hace reconocible.
func TestCajaVacia(t *testing.T) {
	for _, contenido := range []string{"", "\n", "\n\n\n", "   ", "\t"} {
		t.Run(strings.ReplaceAll(contenido, "\n", "nl"), func(t *testing.T) {
			out := RenderWithTitleEx(lipgloss.RoundedBorder(), lipgloss.Color("8"),
				AlignLeft, " Título ", contenido, 20)

			if out == "" {
				t.Fatal("una caja sin contenido ha salido vacía")
			}
			if !strings.Contains(out, "╭") || !strings.Contains(out, "╰") {
				t.Errorf("una caja sin contenido no tiene bordes:\n%s", out)
			}
			// Y todas las líneas tienen el ancho pedido: una línea más corta es
			// justo lo que se repite con el bloque vacío.
			for _, linea := range strings.Split(out, "\n") {
				if n := ansi.StringWidth(linea); n != 20 {
					t.Errorf("una línea mide %d, want 20:\n%s", n, linea)
				}
			}
		})
	}
}

// Un ESC que no abre una secuencia CSI es texto, y el bucle tiene que avanzar
// uno o no saldría nunca. La forma más directa de comprobarlo es que la
// función termine.
func TestEscSueltoEsTexto(t *testing.T) {
	for _, s := range []string{
		"\x1b",              // ESC al principio
		"a\x1bb",            // ESC en medio
		"\x1b\x1b",          // ESC ESC
		"texto\x1bmás",      // ESC suelto dentro
		"\x1b[",             // ESC sin el resto de la secuencia
		"\x1b]0;título\x07", // OSC: no es CSI y sí es texto
	} {
		t.Run(strings.ReplaceAll(strings.ReplaceAll(s, "\x1b", "ESC"), "\x07", "BEL"), func(t *testing.T) {
			terminado := make(chan []ansiSegment, 1)
			go func() { terminado <- parseAnsiSegments(s) }()

			select {
			case segs := <-terminado:
				if len(segs) == 0 {
					t.Errorf("%q ha salido sin segmentos, want al menos uno", s)
				}
				// Y los segmentos tienen que cubrir la cadena entera entre
				// texto y estilo. Un "\x1b[" truncado va entero al estilo y
				// ninguno al texto, así que hay que mirar los dos.
				var unido strings.Builder
				for _, seg := range segs {
					unido.WriteString(seg.style)
					unido.WriteString(seg.text)
				}
				// ansi.Strip no sirve aquí: se lleva también el ESC suelto,
				// que es justo lo que se está comprobando. Lo que importa es
				// que los segmentos devuelvan todos los bytes.
				if unido.Len() != len(s) {
					t.Errorf("los segmentos cubren %d bytes de %d", unido.Len(), len(s))
				}
			case <-timeout():
				t.Fatalf("%q ha colgado el bucle de parseo", s)
			}
		})
	}
}

// Un texto sin ningún estilo se parte en un solo trozo. El `chunks` vacío del
// final es la defensa para cuando el texto es exactamente "".
func TestTextoSinEstilos(t *testing.T) {
	t.Run("vacío", func(t *testing.T) {
		if got := wrapLine("", 20); len(got) != 1 || got[0] != "" {
			t.Errorf("wrapLine(%q) = %q, want un solo trozo vacío", "", got)
		}
	})

	t.Run("normal", func(t *testing.T) {
		if got := wrapLine("hola", 20); len(got) != 1 || got[0] != "hola" {
			t.Errorf("wrapLine(%q) = %q, want un solo trozo", "hola", got)
		}
	})

	t.Run("con estilos", func(t *testing.T) {
		conEstilo := "\x1b[31mrojo\x1b[0m"
		got := wrapLine(conEstilo, 20)
		if len(got) != 1 {
			t.Fatalf("wrapLine con estilos ha dado %d trozos (%q), want 1", len(got), got)
		}
		if limpio := ansi.Strip(got[0]); limpio != "rojo" {
			t.Errorf("el texto con estilos es %q, want rojo", limpio)
		}
	})
}
