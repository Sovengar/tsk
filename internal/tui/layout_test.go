package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestVisibleRange(t *testing.T) {
	tests := []struct {
		name      string
		cursor    int
		total     int
		size      int
		wantStart int
		wantEnd   int
	}{
		{name: "entra todo", cursor: 3, total: 5, size: 10, wantStart: 0, wantEnd: 5},
		{name: "cursor al inicio", cursor: 0, total: 20, size: 5, wantStart: 0, wantEnd: 5},
		{name: "cursor centrado", cursor: 10, total: 20, size: 5, wantStart: 8, wantEnd: 13},
		{name: "cursor al final", cursor: 19, total: 20, size: 5, wantStart: 15, wantEnd: 20},
		{name: "sin elementos", cursor: 0, total: 0, size: 5, wantStart: 0, wantEnd: 0},
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

// TestVisibleRangeContainsCursor verifica el invariante: el rango siempre es
// válido y el cursor siempre cae dentro.
func TestVisibleRangeContainsCursor(t *testing.T) {
	const total, size = 37, 6
	for cursor := 0; cursor < total; cursor++ {
		start, end := visibleRange(cursor, total, size)
		if start < 0 || end > total || start >= end {
			t.Fatalf("rango inválido (%d, %d) para cursor %d", start, end, cursor)
		}
		if cursor < start || cursor >= end {
			t.Fatalf("cursor %d fuera del rango (%d, %d)", cursor, start, end)
		}
	}
}

func TestContentBudget(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		preview  int
		keybinds int
		want     int
	}{
		{name: "descuenta ambos", total: 30, preview: 8, keybinds: 4, want: 18},
		{name: "aplica el piso mínimo", total: 10, preview: 8, keybinds: 4, want: minContentHeight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contentBudget(tt.total, tt.preview, tt.keybinds); got != tt.want {
				t.Errorf("contentBudget = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJoinSections(t *testing.T) {
	tests := []struct {
		name     string
		sections []string
		want     string
	}{
		{name: "salta vacíos", sections: []string{"a", "", "b"}, want: "a\nb"},
		{name: "todo vacío", sections: []string{"", ""}, want: ""},
		{name: "sin vacíos", sections: []string{"a", "b"}, want: "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinSections(tt.sections...); got != tt.want {
				t.Errorf("joinSections = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLineCount(t *testing.T) {
	if got := lineCount(""); got != 0 {
		t.Errorf("lineCount(\"\") = %d, want 0", got)
	}
	if got := lineCount("a\nb\nc"); got != 3 {
		t.Errorf("lineCount = %d, want 3", got)
	}
}

func TestSingleLineCollapsesNewlines(t *testing.T) {
	if got := singleLine("hola\nque tal"); got != "hola que tal" {
		t.Errorf("singleLine = %q, want %q", got, "hola que tal")
	}
}

// TestCellWidthIgnoresAnsi verifica que el relleno se mida en columnas de
// pantalla y no en bytes: una celda con color mide igual que una sin color.
// Este es el bug que desalineaba la tabla de la List.
func TestCellWidthIgnoresAnsi(t *testing.T) {
	styled := stylePriorityHigh.Render("●") + " H"

	if got := ansi.StringWidth(cellWidth(styled, 10)); got != 10 {
		t.Errorf("cellWidth → ancho %d, want 10", got)
	}
	if got := ansi.Strip(cellWidth(styled, 10)); !strings.HasPrefix(got, "● H") {
		t.Errorf("cellWidth alteró el contenido: %q", got)
	}

	// Premisa del bug: %-Ns de fmt cuenta bytes, así que la celda coloreada no
	// llega a las 10 columnas y corre todo lo que viene después.
	if got := ansi.StringWidth(fmt.Sprintf("%-10s", styled)); got != 3 {
		t.Errorf("premisa inválida: %%-10s midió %d columnas, se esperaba 3", got)
	}
}

func TestCellWidthTruncates(t *testing.T) {
	if got := cellWidth("una palabra larga", 8); got != "una pa.." {
		t.Errorf("cellWidth = %q, want %q", got, "una pa..")
	}
}

// El ancho útil de un modal: el total menos los dos bordes. modalWidthFor deja
// siempre margen, así que en uso real nunca llega a 1; el suelo cubre el total
// que venga ya recortado por otro camino.
func TestModalInnerWidth(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"ancho de un modal", 58, 56},
		{"modal ancho", 120, 118},
		{"modal de tres", 3, 1},
		{"suelo en 1", 2, 1},
		{"uno más del suelo", 1, 1},
		{"cero", 0, 1},
		{"negativo", -10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modalInnerWidth(tt.in); got != tt.want {
				t.Errorf("modalInnerWidth(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// Nunca devuelve 0 ni negativo: un ancho de recorte en 0 o menos haría que
// truncateLines no recorte nada, que es lo contrario de lo que se quiere.
func TestModalInnerWidthNeverZero(t *testing.T) {
	for w := -50; w <= 200; w++ {
		if got := modalInnerWidth(w); got < 1 {
			t.Fatalf("modalInnerWidth(%d) = %d, want >= 1", w, got)
		}
	}
}
