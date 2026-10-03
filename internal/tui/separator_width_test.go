package tui

import (
	"strings"
	"testing"
)

// separatorWidth es el ancho del separador que va bajo la barra de filtros:
// el ancho interior menos las dos columnas del recuadro, con suelo en cero
// porque strings.Repeat revienta con un número negativo.
//
// El suelo en 0 es lo que hace que dos de sus comparaciones sean equivalentes:
//
//	max(innerW, 0) vs max(innerW, 1)	-- el "> 0" del suelo
//	max(innerW, 0) vs max(innerW-1, 0)	-- el "-1" del suelo
//
// En los tres casos, para el único valor que llega a cambiar el resultado, el
// separador queda vacío de todas formas: una línea sin guiones es
// visualmente la misma que una de un guion. Por eso hace falta un ancho en el
// que el separador SÍ tenga guiones, y comparar su LONGITUD.

func TestSeparatorWidthNoPasaDeCero(t *testing.T) {
	for _, w := range []int{-5, -1, 0, 1, 2, 3} {
		got := separatorWidth(w)
		if got < 0 {
			t.Errorf("separatorWidth(%d) = %d, negativo: strings.Repeat revienta con eso", w, got)
		}
	}
	// Con el ancho justo para un guion y nada más, sale un guion; con menos, cero.
	if got := separatorWidth(3); got != 1 {
		t.Errorf("separatorWidth(3) = %d, want 1", got)
	}
	if got := separatorWidth(2); got != 0 {
		t.Errorf("separatorWidth(2) = %d, want 0: no hay ni un guion que dibujar", got)
	}
}

// El separador del header tiene que medir lo que dice el ancho, sin una columna
// más ni una menos. Es lo que distingue w-2 de w+2 en la llamada que lo dibuja, y
// es lo que ningún test miraba: se comprobaba que la barra de filtros saliera,
// no cuántooccupaba la línea de debajo.
func TestElSeparadorDelHeaderMideElAncho(t *testing.T) {
	for _, ancho := range []int{20, 40, 80, 120} {
		t.Run("ancho="+itoa(ancho), func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = viewList
			m.filterStatus = ""

			header := sinANSI(m.renderFilterHeader(ancho - 2))
			lineas := strings.Split(header, "\n")

			// El separador es la línea que empieza por guiones. Lipgloss
			// la rellena de espacios hasta el ancho de la barra de arriba, así
			// que la línea es más larga que el separador: lo que cuenta es
			// dónde acaban los guiones.
			guiones := guionesDe(lineas)
			if guiones == 0 {
				t.Fatalf("el header no tiene separador: %q", header)
			}

			want := separatorWidth(ancho - 2)
			if guiones != want {
				t.Errorf("el separador mide %d guiones, want %d (el ancho que se le pasó menos los dos del recuadro)",
					guiones, want)
			}
		})
	}
}

// El separador dentro de la caja del kanban. Su ancho lo fija el truncateLines
// que va después, no el w-2 de la llamada: por eso el `renderFilterHeader(w+2)`
// del kanban es un mutante equivalente, y este test es lo que deja escrito que el
// ancho final es el correcto y no el de la llamada.
func TestElSeparadorDelKanbanMideElAnchoVisible(t *testing.T) {
	for _, ancho := range []int{60, 120, 200} {
		t.Run("w="+itoa(ancho), func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = viewKanban
			m.width, m.height = ancho, 30

			lineas := strings.Split(sinANSI(m.renderKanban(28)), "\n")

			// La segunda línea con guiones es el separador: la primera es el
			// borde de arriba y la tercera ya es el tablero.
			var separador string
			for _, l := range lineas {
				// Interior de la caja hecho sólo de guiones: ni el borde de
				// arriba ni las celdas del tablero cumplen eso.
				interior := strings.Trim(l, "|│ ")
				if interior != "" && strings.Trim(interior, "─") == "" {
					separador = interior
					break
				}
			}
			if separador == "" {
				t.Fatalf("el kanban no tiene separador de filtros:\n%s", strings.Join(lineas, "\n"))
			}

			// El separador llena el ancho interior: dos bordes y los guiones.
			// Con un w+2 en la llamada, el truncateLines de después lo deja
			// igual, y eso es lo que hace que el mutante sea equivalente.
			want := separatorWidth(ancho - 2)
			if got := len([]rune(separador)); got != want {
				t.Errorf("el separador mide %d guiones, want %d (el ancho interior menos nada: %q)",
					got, want, separador)
			}
		})
	}
}

// guionesDe cuenta los guiones del separador: la línea del header que empieza por
// ellos. Lipgloss la rellena de espacios hasta el ancho de la barra de filtros
// que hay encima, así que la línea es más larga que el separador y hay que
// contar sólo los guiones, no los runes.
func guionesDe(lineas []string) int {
	for _, l := range lineas {
		guiones := 0
		for _, r := range l {
			if r == '─' {
				guiones++
				continue
			}
			break
		}
		if guiones > 0 {
			return guiones
		}
	}
	return 0
}

// visibleRange con total y size de 1: el borde que hace alcanzable el `< 1`.
//
// Con `total <= 0` el mutante equivalente era `total < 1` y al revés, porque sobre
// enteros son la misma condición. `< 1` tiene un borde que sí existe -- el 1 -- y
// ese 1 es un caso real: una lista con un elemento y una ventana que lo contiene.
//
// Los cuatro casos de la esquina [0|1] x [0|1] se comprueban, más el interior.
func TestVisibleRangeEnLaEsquinaCeroYUno(t *testing.T) {
	casos := []struct {
		nombre              string
		cursor, total, size int
		wantStart, wantEnd  int
	}{
		{"0 de 0", 0, 0, 0, 0, 0},
		{"0 de 1", 0, 0, 1, 0, 0},
		{"1 de 0", 0, 1, 0, 0, 0},
		{"1 de 1", 0, 1, 1, 0, 1},
		{"1 de 2 con ventana 1", 0, 1, 2, 0, 1}, // la ventana se recorta al total
		{"1 de 5 con ventana 1", 3, 1, 5, 0, 1},
		{"el interior", 2, 5, 2, 1, 3},
		{"negativos", 0, -1, 5, 0, 0},
		{"tamaño negativo", 0, 5, -1, 0, 0},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			start, end := visibleRange(c.cursor, c.total, c.size)
			if start != c.wantStart || end != c.wantEnd {
				t.Errorf("visibleRange(%d, %d, %d) = (%d, %d), want (%d, %d)",
					c.cursor, c.total, c.size, start, end, c.wantStart, c.wantEnd)
			}
		})
	}
}

// El caso total == size: la ventana es exactamente la lista entera. No hace falta
// una guarda para él -- visibleRange la recorta al total y devuelve la página
// entera -- y por eso listWindowForHeight ya no tiene ese `if`.
func TestListWindowForHeightSinGuardaDePaginaEntera(t *testing.T) {
	// La página cabe exactamente en el alto visible.
	start, end := listWindowForHeight(0, 10, 5, 10+listFixedRows)
	if start != 0 || end != 10 {
		t.Errorf("la página entera da (%d, %d), want (0, 10)", start, end)
	}

	// Y con el cursor en cualquier punto de una página que cabe: da igual dónde
	// esté, porque entra entera.
	for _, cursor := range []int{0, 3, 9, 50} {
		start, end := listWindowForHeight(0, 10, cursor, 10+listFixedRows)
		if start != 0 || end != 10 {
			t.Errorf("cursor=%d con la página justa da (%d, %d), want (0, 10)", cursor, start, end)
		}
	}

	// Y una página que NO cabe: la ventana se mueve siguiendo al cursor, que es
	// justo lo que la guarda que se quitó habría dejado pasar sin comprobar.
	for _, cursor := range []int{0, 1, 2} {
		start, end := listWindowForHeight(0, 3, cursor, 3+listFixedRows)
		if start < 0 || end > 3 || end <= start {
			t.Errorf("cursor=%d con ventana exacta da (%d, %d), fuera de la página", cursor, start, end)
		}
		if start > cursor || cursor >= end {
			t.Errorf("cursor=%d fuera de la ventana [%d, %d)", cursor, start, end)
		}
	}

	// Con un cursor fuera de la página, el clamp lo mete dentro en vez de dejar
	// la ventana en un sitio raro: la ventana exacta de una página de 3 con el
	// cursor en 9 tiene que acabar en el final de la página.
	start, end = listWindowForHeight(0, 3, 9, 3+listFixedRows)
	if start != 0 || end != 3 {
		t.Errorf("con el cursor en 9 y ventana exacta da (%d, %d), want (0, 3)", start, end)
	}
}

// listWindowForHeight con visible == 1: el borde que hace alcanzable el `< 1`.
//
// Con `visible <= 0` el mutante equivalente es `visible < 1` y al revés: sobre
// enteros son la misma condición, y por eso ningún test podía distinguirlas. `< 1`
// tiene un borde que sí ocurre -- visible == 1, una sola fila para tareas -- y ese
// borde es una ventana de una fila dentro de una página de tres, que es lo que
// distingue una cosa de la otra.
func TestListWindowForHeightConUnaFilaVisible(t *testing.T) {
	// Una página de 3 con una sola fila visible: la ventana se recorta a una.
	for _, cursor := range []int{0, 1, 2} {
		start, end := listWindowForHeight(0, 3, cursor, listFixedRows+1)
		if end-start != 1 {
			t.Errorf("cursor=%d con una fila visible da [%d, %d), want una fila", cursor, start, end)
		}
		if start > cursor || cursor >= end {
			t.Errorf("cursor=%d fuera de la ventana [%d, %d)", cursor, start, end)
		}
	}

	// Y el caso en el que visible == 0, que es el otro lado del borde: se pinta
	// la página entera.
	for _, alto := range []int{listFixedRows, listFixedRows - 1, 0} {
		start, end := listWindowForHeight(0, 3, 1, alto)
		if start != 0 || end != 3 {
			t.Errorf("con alto=%d (visible<=0) da (%d, %d), want la página entera (0, 3)",
				alto, start, end)
		}
	}
}
