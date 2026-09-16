package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// minContentHeight es el alto mínimo que se reserva para el contenido de la
// vista, para que la caja no se degrade en terminales chicas.
const minContentHeight = 8

// lineCount devuelve cuántas filas ocupa un bloque de texto.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// truncateLines corta cada línea al ancho dado. Evita que el helper de bordes
// re-wrapée una línea que no entra y agregue filas de más, lo que rompería el
// cálculo de alto.
func truncateLines(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if ansi.StringWidth(line) > width {
			lines[i] = ansi.Truncate(line, width, "")
		}
	}
	return strings.Join(lines, "\n")
}

// contentBudget calcula las filas disponibles para el contenido de la vista
// descontando el preview y la barra de keybinds.
func contentBudget(total, previewH, keybindsH int) int {
	budget := total - previewH - keybindsH
	if budget < minContentHeight {
		budget = minContentHeight
	}
	return budget
}

// joinSections une bloques verticalmente sin dejar filas vacías entre ellos.
func joinSections(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, s := range sections {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, "\n")
}

// visibleRange devuelve el rango [start, end) de una ventana de size elementos
// sobre un total, manteniendo el cursor dentro de la ventana.
func visibleRange(cursor, total, size int) (int, int) {
	if total <= 0 || size <= 0 {
		return 0, 0
	}
	if size >= total {
		return 0, total
	}

	start := cursor - size/2
	if start < 0 {
		start = 0
	}
	if start > total-size {
		start = total - size
	}
	return start, start + size
}

// truncate corta un texto a max caracteres agregando un sufijo "..".
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}

// singleLine colapsa un texto multilínea a una sola línea para que no rompa
// una fila de tabla.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
