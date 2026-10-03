package tui

import (
	"strings"
	"testing"
	"time"

	"tsk/internal/model"
)

// renderGanttRuler construye una línea de labelW + 1 + dayCols columnas: la
// etiqueta de cada semana alineada a su lunes, sobre una rejilla de espacios.
//
// El "+1" del medio es la columna separadora entre las etiquetas y el calendario.
// Cambiarlo por "*1" quita una columna de la rejilla, y el resultado sale IGUAL
// mientras la última columna quede vacía -- TrimRight se la come. Por eso un
// test que mide el ancho de la línea no distingue nada, y por eso el mutante
// sobrevivía: con cualquier número normal de días visibles la etiqueta de la
// semana (4 letras, en columnas de 7) siempre tiene sitio de sobra.
//
// El caso que separa las dos cosas es dayCols == 1: la rejilla es de
// labelW+2 columnas y la etiqueta se escribe en la columna labelW+1, que es la
// última. Ahí el "+1" es justo lo que le da sitio al primer carácter. Sin él,
// la condición `at+i < len(ruler)` descarta la etiqueta entera y el gantt se
// queda sin los lunes rotulados.

func rulerDe(labelW, dayCols int) string {
	lunes := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	m := &Model{}
	return sinANSI(m.renderGanttRuler(lunes, 0, labelW, dayCols))
}

// sinANSI quita los códigos de color para poder medir la línea.
func sinANSI(s string) string {
	var b strings.Builder
	enEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			enEscape = true
		case enEscape:
			if r == 'm' {
				enEscape = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Cuatro días visibles: la etiqueta del lunes (cuatro letras) ocupa la columna
// labelW+1 y las cuatro siguientes, así que su ÚLTIMO carácter cae en la última
// columna de la rejilla. Esa columna existe por el "+1": sin él, la condición
// `at+i < len(ruler)` descarta el último carácter y el gantt muestra "1MA".
func TestGanttRulerEtiquetaAlUltimoDiaVisible(t *testing.T) {
	etiqueta := model.WeekOfMonthLabel(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	if etiqueta != "1MAR" {
		t.Fatalf("la etiqueta del fixture es %q, want 1MAR", etiqueta)
	}

	for _, labelW := range []int{0, 1, 2, 5} {
		t.Run("labelW="+itoa(labelW), func(t *testing.T) {
			const dayCols = 4
			ruler := rulerDe(labelW, dayCols)

			if !strings.Contains(ruler, etiqueta) {
				t.Errorf("con cuatro días visibles la etiqueta del lunes no sale entera: %q, want %q",
					ruler, etiqueta)
			}
			// La rejilla es de labelW+1+dayCols columnas, y la línea ni la pasa
			// ni se queda corta: la etiqueta llega hasta el final.
			if n := len([]rune(ruler)); n != labelW+1+dayCols {
				t.Errorf("la línea mide %d columnas, want %d (labelW+%d días): %q",
					n, labelW+1+dayCols, dayCols, ruler)
			}
		})
	}
}

// Con cero días visibles no hay lunes que rotular y la línea sale vacía: es el
// otro borde del mismo rango, y dice que la condición `at+i < len(ruler)` es la
// que protege, no que el "+1" sobre.
func TestGanttRulerSinDiasVisibles(t *testing.T) {
	for _, labelW := range []int{0, 3, 8} {
		if r := rulerDe(labelW, 0); r != "" {
			t.Errorf("labelW=%d sin días visibles sale %q, want vacío", labelW, r)
		}
	}
}

// Con los días de sobra, el "+1" no se nota: es el caso desde el que sale el
// resto de los tests, y por eso no mataba el mutante. Está aquí para dejar escrito
// que la diferencia es de margen, no de contenido.
func TestGanttRulerConDiasDeSobra(t *testing.T) {
	etiqueta := model.WeekOfMonthLabel(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))

	ruler := rulerDe(5, 21)
	if !strings.Contains(ruler, etiqueta) {
		t.Errorf("con tres semanas visibles la primera etiqueta no sale: %q", ruler)
	}
	// Las tres semanas están a siete columnas una de otra.
	for _, want := range []string{"1MAR", "2MAR", "3MAR"} {
		if !strings.Contains(ruler, want) {
			t.Errorf("con 21 días falta la etiqueta %q: %q", want, ruler)
		}
	}
	// Y con cuatro días, la etiqueta se estira hasta el borde derecho. Ese
	// contraste -- holgura por un lado, nada por el otro -- es el que hace
	// legible el "+1".
	if r := rulerDe(5, 4); !strings.Contains(r, etiqueta) {
		t.Errorf("con cuatro días visibles la etiqueta no sale: %q", r)
	}
}

// itoa evita fmt para un solo número.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
