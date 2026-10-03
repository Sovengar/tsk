package tui

import (
	"fmt"
	"strings"
	"testing"
)

// descriptionLines envuelve la descripción al ancho y la recorta a maxLines. El
// recorte tiene dos mitades y las dos necesitan un caso exacto:
//
//	si len(wrapped) > maxLines	-- hay más líneas de las que caben
//	recortar a limit-1		-- reservar celda para la elipsis
//
// El "> maxLines" es lo que hace que un texto que cabe JUSTO no lleve elipsis
// puesta: con ">= maxLines", un texto de exactamente maxLines líneas saldría
// con "…" en la última, que es un texto que se ve entero y al que le aparece un
// signo de que le falta algo.
//
// Y el límite de recorte (limit-1, limit, limit-2) sólo se ve cuando la última
// línea está LLENA, porque ansi.Wrap entrega líneas de limit columnas o menos.

func TestPreviewBarNoPoneElipsisEnUnTextoQueCabeJusto(t *testing.T) {
	// Se construye al revés: se elige un texto y se mide cuántas líneas ocupa,
	// y se ajusta maxLines a ese número exacto.
	const ancho = 40
	desc := "primera linea con palabras " +
		"segunda linea que continua " +
		"tercera que tambien sigue"

	// El texto ocupa exactamente tres líneas a este ancho, así que maxLines=3
	// es el caso del borde: cabe todo y no sobra nada.
	for _, maxLines := range []int{3} {
		t.Run(fmt.Sprintf("cabe en %d líneas", maxLines), func(t *testing.T) {
			p := PreviewBar{width: ancho, maxLines: maxLines}
			lineas := p.descriptionLines(desc)
			if len(lineas) != maxLines {
				t.Fatalf("con maxLines=%d salen %d líneas: el texto no cabe en ese número", maxLines, len(lineas))
			}
			// El texto original está entero, así que no le falta nada: la
			// elipsis no tendría por qué aparecer.
			plano := strings.Join(lineas, " ")
			for _, palabra := range []string{"primera", "segunda", "tercera", "tambien"} {
				if !strings.Contains(plano, palabra) {
					t.Fatalf("el texto no cabe en %d líneas: falta %q, sale %q", maxLines, palabra, plano)
				}
			}
			if strings.Contains(plano, "…") {
				t.Errorf("un texto que cabe justo lleva elipsis: %q", plano)
			}
		})
	}

	// Y con una línea más de las que caben, la elipsis aparece y se pierde algo.
	p := PreviewBar{width: ancho, maxLines: 1}
	lineas := p.descriptionLines(desc)
	if len(lineas) != 1 {
		t.Fatalf("con maxLines=1 salen %d líneas", len(lineas))
	}
	if !strings.Contains(lineas[0], "…") {
		t.Errorf("un texto que NO cabe no lleva elipsis: %q", lineas[0])
	}
	if strings.Contains(strings.Join(lineas, " "), "tercera") {
		t.Error("con recorte=no debería verse la última parte del texto")
	}
}

// El límite de recorte de la elipsis (limit-1, limit, limit-2) es
// intentionally NO comprobado aquí: ansi.Wrap entrega líneas de limit columnas
// o menos, así que truncar la última línea no quita nada y los tres límites dan
// el mismo resultado. Es un mutante equivalente y está en .mutation-allowlist
// con ese motivo, no con el de un hueco.
//
// Lo que sí importa y se comprueba arriba es que la línea CON la elipsis quepa
// en el ancho interno: si el recorte no reservara la celda, el "…" la empujaría
// una columna de más y bordered la re-wrapearía en dos.

// recortarParaElipsis: el suelo y el límite.
//
// El límite es lo que hace el recorte correcto y el suelo es lo que evita el
// pánico. Los dos se comprueban por sus lados, incluido el caso degenerado de un
// límite más pequeño que la elipsis, que es una terminal muy estrecha.
func TestRecortarParaElipsis(t *testing.T) {
	t.Run("cabe entera", func(t *testing.T) {
		got := recortarParaElipsis("texto corto", 40)
		if !strings.Contains(got, "texto corto") {
			t.Errorf("un texto que cabe ha perdido parte: %q", got)
		}
		if !strings.HasSuffix(got, elipsisDescription) {
			t.Errorf("falta la elipsis al final: %q", got)
		}
	})

	t.Run("no cabe y se recorta dejando hueco para la elipsis", func(t *testing.T) {
		const limit = 10
		got := recortarParaElipsis(strings.Repeat("x", 100), limit)

		// La línea con la elipsis tiene que caber en el límite: si no, al
		// rellenar la caja la re-wrapearía en dos y rompería el tope de alto.
		if n := len([]rune(got)); n > limit {
			t.Errorf("la línea recortada mide %d columnas y el límite es %d: %q", n, limit, got)
		}
		if !strings.HasSuffix(got, elipsisDescription) {
			t.Errorf("falta la elipsis: %q", got)
		}
		// Y con el límite exacto: el texto llena hasta el hueco de la elipsis.
		if n := len([]rune(got)); n != limit {
			t.Errorf("la línea mide %d columnas, want %d (el límite entero)", n, limit)
		}
	})

	t.Run("límite menor que la elipsis", func(t *testing.T) {
		// Aquí el suelo en 0 es lo que evita el negativo. Sale sólo la elipsis.
		for _, limit := range []int{0, 1} {
			got := recortarParaElipsis("texto", limit)
			if got != elipsisDescription {
				t.Errorf("con límite %d sale %q, want sólo la elipsis", limit, got)
			}
		}
	})

	t.Run("texto vacío", func(t *testing.T) {
		got := recortarParaElipsis("", 10)
		if got != elipsisDescription {
			t.Errorf("con texto vacío sale %q, want sólo la elipsis", got)
		}
	})

	t.Run("la elipsis ocupa lo que dice", func(t *testing.T) {
		if anchoElipsis != len([]rune(elipsisDescription)) {
			t.Errorf("anchoElipsis = %d pero la elipsis mide %d: el hueco reservado no es el real",
				anchoElipsis, len([]rune(elipsisDescription)))
		}
	})
}

// El ancho mínimo de una columna del kanban: la cabecera más los bordes, con el
// suelo absoluto debajo.
func TestAnchoMinimoDeColumna(t *testing.T) {
	casos := []struct {
		nombre   string
		cabecera int
		want     int
	}{
		{"cabe muy corta", 1, kanbanMinColWidth},
		{"justo en el suelo", kanbanMinColWidth - anchosBorde, kanbanMinColWidth},
		{"una más que el suelo", kanbanMinColWidth - anchosBorde + 1, kanbanMinColWidth + 1},
		{"muy ancha", 100, 100 + anchosBorde},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := anchoMinimoDeColumna(c.cabecera); got != c.want {
				t.Errorf("anchoMinimoDeColumna(%d) = %d, want %d", c.cabecera, got, c.want)
			}
		})
	}

	// La cuenta de los bordes es la que hace el borde entre "cabe por el suelo" y
	// "cabe por la cabecera": con una columna menos de bordes, el primer caso
	// cambiaría.
	if got := anchoMinimoDeColumna(0); got != kanbanMinColWidth {
		t.Errorf("una cabecera de 0 columnas da %d, want el suelo %d", got, kanbanMinColWidth)
	}
}
