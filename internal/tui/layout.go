package tui

import (
	"strings"

	"tsk/internal/model"

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

// cellWidth ajusta un texto al ancho de display indicado: lo trunca con ".." si
// sobra y lo rellena con espacios si falta. Mide columnas de pantalla, no bytes,
// así que las celdas con color (ANSI) quedan alineadas con las que no. Es lo que
// reemplaza a %-Ns de fmt, que cuenta bytes y desalinea las celdas coloreadas.
func cellWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	s = ansi.Truncate(s, width, "..")
	if pad := width - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// contentBudget calcula las filas disponibles para el contenido de la vista
// descontando el preview y la barra de keybinds.
func contentBudget(total, previewH, keybindsH int) int {
	budget := total - previewH - keybindsH
	budget = max(budget, minContentHeight)
	return budget
}

// ---- Aritmética de índices -------------------------------------------------
//
// Estas tres funciones existen porque la misma cuenta aparecía escrita en varios
// sitios (navegación de List, de Kanban y del filtro de proyectos) con su
// propio borde. Repetida, cada copia es un sitio donde un mutante puede colgar
// el bucle y donde ningún test alcanza. Al sacarlas a funciones puras el
// contrato queda en un sitio y se puede comprobar exhaustivamente, sin montar
// una vista entera ni una base de datos.

// cycleIndex mueve idx dentro de [0, n) dando la vuelta por el final.
//
// n <= 0 devuelve idx sin tocar: no hay lista a la que moverse y es mejor dejar
// el índice como estaba que inventar un 0. delta puede ser negativo.
func cycleIndex(idx, n, delta int) int {
	if n <= 0 {
		return idx
	}
	return ((idx+delta)%n + n) % n
}

// shiftIndex mueve idx dentro de [0, n) SIN dar la vuelta: se queda en el
// extremo en lugar de saltar al otro lado.
func shiftIndex(idx, n, delta int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx+delta, 0), n-1)
}

// inRange dice si idx es un índice válido dentro de una lista de n elementos.
// Es la guarda que aparece antes de cada tasks[idx] del repo.
func inRange(idx, n int) bool {
	return idx >= 0 && idx < n
}

// taskAt devuelve la tarea en la posición idx, o nil si el índice no es válido.
//
// Se extrajo porque la guarda "cursor < len(tasks)" estaba escrita delante de
// cada tasks[m.cursor] con su propia forma. Su mutante de BOUNDARY (que pasa `<`
// a `<=`) sólo se puede matar si la función recibe el índice: desde el teclado el
// cursor siempre llega acotado, así que la diferencia es inalcanzable. Con el
// índice como parámetro se puede probar directamente el caso idx == len(tasks).
func taskAt(tasks []model.Task, idx int) *model.Task {
	if !inRange(idx, len(tasks)) {
		return nil
	}
	return &tasks[idx]
}

// previewBudgetFor son las líneas de descripción que caben sin empujar el
// contenido ni los keybinds fuera de la pantalla.
//
// height - keybinds - minContentHeight, menos los dos bordes de la caja del
// preview, y acotado entre 1 y previewMaxLines. El mínimo de 1 garantiza que la
// descripción siempre tiene al menos una línea, aunque el terminal sea minúsculo.
func previewBudgetFor(height, keybindsHeight int) int {
	budget := height - keybindsHeight - minContentHeight - 2 // 2 = bordes de la caja
	return min(max(budget, 1), previewMaxLines)
}

// matchesStatus dice si el estado de una tarea pasa el filtro de estado.
//
// Hay tres modos: el de "todas las activas" (todo menos done y cancelled), el
// de "todas" (sin restricción) y el de un estado concreto (coincidencia
// exacta).
func matchesStatus(statusFilter, taskStatus string, active bool) bool {
	switch statusFilter {
	case statusFilterAllActive:
		return active
	case "":
		return true
	default:
		return taskStatus == statusFilter
	}
}

// matchesPriority aplica el filtro de prioridad. -1 significa "cualquiera".
func matchesPriority(filterPriority, taskPriority int) bool {
	return filterPriority < 0 || filterPriority == taskPriority
}

// nextPriority es el ciclo de la tecla de prioridad (ctrl+p).
//
// El ciclo depende del estado: en backlog incluye "none" (none→low→med→high→none,
// cuatro peldaños) y fuera de backlog no (low→med→high→low, tres). Por eso no es
// un `% 4` global.
//
// priority se recorta antes de ciclar porque llega de la base y allí no hay
// garantía de que esté en 0..3: un 7 fuera de rango haría que low→low con el
// ciclo de tres peldaños. Recortar hace que la función sea total.
func nextPriority(status string, priority int) int {
	p := min(max(priority, model.PriorityNone), model.PriorityHigh)
	if status == "backlog" {
		return (p + 1) % 4
	}
	return (p % 3) + 1
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
	start = max(start, 0)
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
