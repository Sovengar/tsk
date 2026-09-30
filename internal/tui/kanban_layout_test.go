package tui

import (
	"fmt"
	"strings"
	"testing"
)

// clampKanban mantiene el cursor dentro del board. Se prueba con el número de
// tarjetas de cada columna como dato de entrada, sin construir el board: la
// regla es "columna dentro, fila dentro de esa columna", y eso es lo que se fija.
func TestClampKanban(t *testing.T) {
	tests := []struct {
		name             string
		col, row         int
		colLens          []int
		wantCol, wantRow int
	}{
		{"sin columnas", 3, 5, nil, 0, 0},
		{
			name:    "cursor válido",
			colLens: []int{3, 2, 4}, col: 1, row: 1,
			wantCol: 1, wantRow: 1,
		},
		{
			name:    "columna negativa se sube a 0",
			colLens: []int{3, 2, 4}, col: -2, row: 1,
			wantCol: 0, wantRow: 1,
		},
		{
			name:    "columna más allá del final baja a la última",
			colLens: []int{3, 2, 4}, col: 9, row: 1,
			wantCol: 2, wantRow: 1,
		},
		{
			name:    "fila dentro",
			colLens: []int{5}, col: 0, row: 3,
			wantCol: 0, wantRow: 3,
		},
		{
			name:    "fila más allá del final de la columna se recorta",
			colLens: []int{2}, col: 0, row: 7,
			wantCol: 0, wantRow: 1,
		},
		{
			name:    "columna vacía deja la fila en 0",
			colLens: []int{3, 0, 2}, col: 1, row: 5,
			wantCol: 1, wantRow: 0,
		},
		{
			name:    "la columna vacía no borra la columna seleccionada",
			colLens: []int{3, 0, 2}, col: 1, row: 0,
			wantCol: 1, wantRow: 0,
		},
		{
			name:    "una sola columna vacía",
			colLens: []int{0}, col: 0, row: 4,
			wantCol: 0, wantRow: 0,
		},
		{
			name:    "una sola columna con tarjetas",
			colLens: []int{3}, col: 0, row: 99,
			wantCol: 0, wantRow: 2,
		},
		{
			// El caso que dispara el clamp: se filtró y la columna seleccionada
			// perdió tarjetas, así que la fila ya no existe dentro de ella.
			name:    "la columna perdió tarjetas al filtrar",
			colLens: []int{4, 1}, col: 1, row: 3,
			wantCol: 1, wantRow: 0,
		},
		{
			name:    "fila negativa se sube a 0",
			colLens: []int{3, 2}, col: 0, row: -5,
			wantCol: 0, wantRow: 0,
		},
		{
			name:    "todas las columnas vacías",
			colLens: []int{0, 0, 0}, col: 2, row: 2,
			wantCol: 2, wantRow: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, row := clampKanban(tt.col, tt.row, tt.colLens)
			if col != tt.wantCol || row != tt.wantRow {
				t.Errorf("clampKanban(%d, %d, %v) = (%d,%d), want (%d,%d)",
					tt.col, tt.row, tt.colLens, col, row, tt.wantCol, tt.wantRow)
			}
		})
	}
}

// El resultado siempre está dentro del board: es la invariante de la que
// depende selectedTask() para no indexar fuera de rango.
func TestClampKanbanAlwaysInRange(t *testing.T) {
	boards := [][]int{
		nil, {}, {0}, {1}, {3, 0, 2}, {2, 2, 2}, {5, 1, 0, 4},
	}
	for _, colLens := range boards {
		for col := -3; col <= len(colLens)+3; col++ {
			for row := -3; row <= 8; row++ {
				gotCol, gotRow := clampKanban(col, row, colLens)
				if len(colLens) == 0 {
					if gotCol != 0 || gotRow != 0 {
						t.Fatalf("clampKanban(%d,%d,%v) = (%d,%d), want (0,0) sin columnas",
							col, row, colLens, gotCol, gotRow)
					}
					continue
				}
				if gotCol < 0 || gotCol >= len(colLens) {
					t.Fatalf("clampKanban(%d,%d,%v) dio columna %d, fuera de [0,%d)",
						col, row, colLens, gotCol, len(colLens))
				}
				n := colLens[gotCol]
				if n == 0 {
					if gotRow != 0 {
						t.Fatalf("clampKanban(%d,%d,%v) dio fila %d en una columna vacía",
							col, row, colLens, gotRow)
					}
					continue
				}
				if gotRow < 0 || gotRow >= n {
					t.Fatalf("clampKanban(%d,%d,%v) dio fila %d, fuera de [0,%d)",
						col, row, colLens, gotRow, n)
				}
			}
		}
	}
}

// kanbanMaxCards: cuántas tarjetas enteras caben en el alto disponible.
func TestKanbanMaxCards(t *testing.T) {
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{"terminal grande", 40, (40 - kanbanBoardChrome - filterHeaderRows + 1) / kanbanCardRows},
		{"justo para 1 tarjeta", kanbanBoardChrome + filterHeaderRows + kanbanCardRows, 1},
		// El round-up cuenta la última aunque sólo entre su parte superior:
		// con 5 filas de contenido entran 2 tarjetas y la segunda se recorta.
		{"la última tarjeta se cuenta aunque se recorte", kanbanBoardChrome + filterHeaderRows + 2*kanbanCardRows - 1, 2},
		{"para 2 tarjetas justas", kanbanBoardChrome + filterHeaderRows + 2*kanbanCardRows, 2},
		{"terminal mediano", 20, (20 - kanbanBoardChrome - filterHeaderRows + 1) / kanbanCardRows},
		{"altura mínima da 1", 0, 1},
		{"altura negativa da 1", -5, 1},
		// El redondeo: con 4 filas de contenido entra 1 tarjeta, no 2. Es el
		// borde donde sumar una fila de más al presupuesto cambiaría el
		// resultado, así que fija que las columnas son las que son.
		{"con 4 filas de contenido entra 1 tarjeta", kanbanBoardChrome + filterHeaderRows + 4, 1},
		{"con 5 filas de contenido entran 2 tarjetas", kanbanBoardChrome + filterHeaderRows + 5, 2},
		{"con 2 filas de contenido entra 1", kanbanBoardChrome + filterHeaderRows + 2, 1},
		{"con 1 fila de contenido entra 1", kanbanBoardChrome + filterHeaderRows + 1, 1},
		{"con 0 filas de contenido entra 1", kanbanBoardChrome + filterHeaderRows, 1},
		{"con contenido negativo entra 1", kanbanBoardChrome + filterHeaderRows - 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kanbanMaxCards(tt.height); got != tt.want {
				t.Errorf("kanbanMaxCards(%d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

// Nunca menos de 1: un tablero sin tarjetas visibles no es un tablero.
func TestKanbanMaxCardsNeverZero(t *testing.T) {
	for h := -20; h <= 100; h++ {
		if got := kanbanMaxCards(h); got < 1 {
			t.Fatalf("kanbanMaxCards(%d) = %d, want >= 1", h, got)
		}
	}
}

// Más alto nunca da menos tarjetas.
func TestKanbanMaxCardsGrowsWithHeight(t *testing.T) {
	prev := 0
	for h := 0; h <= 80; h++ {
		got := kanbanMaxCards(h)
		if got < prev {
			t.Fatalf("kanbanMaxCards(%d) = %d < %d del terminal anterior", h, got, prev)
		}
		prev = got
	}
}

// kanbanHeader: con tarjetas ocultas dice cuántas de cuántas; si caben todas,
// sólo el total. El texto cambia de longitud, y ese ancho es el mínimo de la
// columna, así que equivocarse descuadra el board entero.
func TestKanbanHeader(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		shown, total int
		want         string
	}{
		{"todo visible", "doing", 5, 5, "─ doing (5) "},
		{"todo visible en cero", "todo", 0, 0, "─ todo (0) "},
		{"con recorte", "doing", 3, 12, "─ doing (3/12) "},
		{"recorte de una", "backlog", 1, 20, "─ backlog (1/20) "},
		{"recorte de todo menos uno", "done", 9, 10, "─ done (9/10) "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kanbanHeader(tt.status, tt.shown, tt.total)
			if got != tt.want {
				t.Errorf("kanbanHeader(%q, %d, %d) = %q, want %q", tt.status, tt.shown, tt.total, got, tt.want)
			}
		})
	}
}

// shown > total no puede pasar por el visor, pero la función es total: cae en la
// rama de "todo visible" en vez de inventar un porcentaje imposible.
func TestKanbanHeaderShownAboveTotal(t *testing.T) {
	got := kanbanHeader("doing", 12, 5)
	want := fmt.Sprintf("─ %s (%d) ", "doing", 5)
	if got != want {
		t.Errorf("con shown > total = %q, want %q", got, want)
	}
}

// La rama del recorte es exactamente "no caben todas": en el borde shown == total
// ya no cabe el porcentaje.
func TestKanbanHeaderBoundary(t *testing.T) {
	for total := 0; total <= 20; total++ {
		at := kanbanHeader("s", total, total)
		if strings.Contains(at, "/") {
			t.Errorf("shown == total = %q, no debería llevar porcentaje", at)
		}
		below := kanbanHeader("s", total-1, total)
		if total > 0 && !strings.Contains(below, "/") {
			t.Errorf("shown = total-1 = %q, debería llevar porcentaje", below)
		}
	}
}
