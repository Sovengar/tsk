package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/model"
)

// modelWithTasks construye un modelo con n tareas en el proyecto "api" y un
// tamaño de página explícito, para poder situar el cursor a voluntad.
func modelWithTasks(t *testing.T, n, pageSize int) *Model {
	t.Helper()
	m := newTestModel(t)
	m.pageSize = pageSize

	tasks := make([]model.Task, 0, n)
	for i := range n {
		tasks = append(tasks, model.Task{
			ID:     int64(i + 1),
			Title:  fmt.Sprintf("tarea %02d", i),
			Status: "todo",
		})
	}
	m.tasks = tasks
	m.invalidateFilterCache()
	return m
}

// listWindow es el cálculo único de la paginación. Estos casos fijan sus
// invariantes exactos, incluidos los bordes que antes vivían repartidos por
// cinco call sites distintos (y por eso cada copia era un hueco de test).
func TestListWindow(t *testing.T) {
	tests := []struct {
		name                string
		total, size, cursor int
		wantPage, wantPages int
		wantStart, wantEnd  int
	}{
		{"lista vacía", 0, 5, 0, 0, 1, 0, 0},
		{"una página exacta", 10, 5, 0, 0, 2, 0, 5},
		{"cursor en la primera", 12, 5, 0, 0, 3, 0, 5},
		{"cursor a mitad de página", 12, 5, 3, 0, 3, 0, 5},
		{"último de la primera página", 12, 5, 4, 0, 3, 0, 5},
		{"primer de la segunda", 12, 5, 5, 1, 3, 5, 10},
		{"último de la segunda", 12, 5, 11, 2, 3, 10, 12},
		{"cursor más allá del final", 12, 5, 99, 2, 3, 10, 12},
		{"cursor negativo", 12, 5, -3, 0, 3, 0, 5},
		{"página de tamaño 1", 3, 1, 2, 2, 3, 2, 3},
		{"más tareas que páginas exactas", 20, 5, 19, 3, 4, 15, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithTasks(t, tt.total, tt.size)
			m.cursor = tt.cursor

			w := m.listWindow()
			if w.page != tt.wantPage || w.pages != tt.wantPages ||
				w.start != tt.wantStart || w.end != tt.wantEnd {
				t.Errorf("listWindow = {page:%d pages:%d start:%d end:%d}, want {page:%d pages:%d start:%d end:%d}",
					w.page, w.pages, w.start, w.end, tt.wantPage, tt.wantPages, tt.wantStart, tt.wantEnd)
			}
			if w.total != tt.total || w.size != tt.size {
				t.Errorf("total/size = %d/%d, want %d/%d", w.total, w.size, tt.total, tt.size)
			}
			// Invariante que la vista depende: la ventana nunca excede el total
			// y nunca se invierte.
			if w.start > w.end || w.end > w.total {
				t.Errorf("ventana inválida: start=%d end=%d total=%d", w.start, w.end, w.total)
			}
		})
	}
}

func TestListWindowClampCursor(t *testing.T) {
	// `cursor` fija la página (y con ella la ventana); `probe` es el valor que se
	// mete en clampCursor, que es lo que hace j al bajar y p al subir.
	tests := []struct {
		name                string
		total, size, cursor int
		probe               int
		want                int
	}{
		{"lista vacía siempre a 0", 0, 5, 0, 7, 0},
		{"lista vacía con probe negativo", 0, 5, 0, -2, 0},
		{"dentro de la ventana", 12, 5, 0, 3, 3},
		{"bajar no pasa del final de la página", 12, 5, 0, 5, 4},
		{"subir no pasa del inicio de la página", 12, 5, 5, 4, 5},
		{"último de la última página", 12, 5, 11, 11, 11},
		{"bajar en la última página se queda", 12, 5, 11, 12, 11},
		{"muy por encima se recorta", 12, 5, 99, 99, 11},
		{"negativo se recorta al inicio", 12, 5, 0, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithTasks(t, tt.total, tt.size)
			m.cursor = tt.cursor

			w := m.listWindow()
			if got := w.clampCursor(tt.probe); got != tt.want {
				t.Errorf("clampCursor(%d) = %d, want %d", tt.probe, got, tt.want)
			}
			// Invariante: nunca fuera de la ventana visible.
			if tt.total > 0 {
				if got := w.clampCursor(tt.probe); got < w.start || got > w.end-1 {
					t.Errorf("clampCursor devolvió %d, fuera de [%d,%d]", got, w.start, w.end)
				}
			}
		})
	}
}

func TestListWindowPageNavigation(t *testing.T) {
	tests := []struct {
		name                string
		total, size, cursor int
		wantNext, wantPrev  int
		nextOK, prevOK      bool
	}{
		{"primera página no tiene anterior", 12, 5, 0, 5, 0, true, false},
		{"página intermedia tiene ambas", 12, 5, 6, 10, 0, true, true},
		{"última página no tiene siguiente", 12, 5, 11, 0, 5, false, true},
		{"lista vacía no navega", 0, 5, 0, 0, 0, false, false},
		{"una sola página no navega", 3, 5, 1, 0, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithTasks(t, tt.total, tt.size)
			m.cursor = tt.cursor
			w := m.listWindow()

			if got, ok := w.nextPageStart(); got != tt.wantNext || ok != tt.nextOK {
				t.Errorf("nextPageStart() = %d, %v; want %d, %v", got, ok, tt.wantNext, tt.nextOK)
			}
			if got, ok := w.prevPageStart(); got != tt.wantPrev || ok != tt.prevOK {
				t.Errorf("prevPageStart() = %d, %v; want %d, %v", got, ok, tt.wantPrev, tt.prevOK)
			}
		})
	}
}

// Casos de pageLegend que el test existente no cubre: cursor más allá del
// final (la página se recorta) y cursor negativo (la ventana no se invierte).
func TestPageLegendOutOfRangeCursor(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		pageSize int
		cursor   int
		want     string
	}{
		{"cursor más allá del final se recorta a la última página", 4, 3, 99, "4-4 of 4 · Page 2/2"},
		{"cursor negativo se recorta a la primera", 4, 3, -2, "1-3 of 4 · Page 1/2"},
		{"exactamente una página", 3, 3, 2, "1-3 of 3 · Page 1/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.tasks = make([]model.Task, tt.total)
			m.invalidateFilterCache()
			m.pageSize = tt.pageSize
			m.cursor = tt.cursor

			if got := m.pageLegend(); got != tt.want {
				t.Errorf("pageLegend() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Navegación real por teclado. El comportamiento es el de siempre (j/k se
// detienen en el borde de la página, n/p saltan de página, N/P van a los
// extremos); lo que cambia es que ya no hay ningún incremento dentro de un if,
// así que el mutante de ese incremento no puede colgarse.
func TestListNavigationByKey(t *testing.T) {
	t.Run("j se detiene al final de la página", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 0

		for range 10 {
			m, _ = press(m, "j")
		}
		if m.cursor != 4 {
			t.Errorf("cursor = %d tras bajar de más, want 4 (final de la primera página)", m.cursor)
		}
	})

	t.Run("k se detiene al inicio de la página", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 7

		for range 10 {
			m, _ = press(m, "k")
		}
		if m.cursor != 5 {
			t.Errorf("cursor = %d tras subir de más, want 5 (inicio de la segunda página)", m.cursor)
		}
	})

	t.Run("n salta de página y p vuelve", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 0

		m, _ = press(m, "n")
		if m.cursor != 5 {
			t.Errorf("tras n cursor = %d, want 5", m.cursor)
		}
		m, _ = press(m, "p")
		if m.cursor != 0 {
			t.Errorf("tras p cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("n en la última página no hace nada", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 11

		m, _ = press(m, "n")
		if m.cursor != 11 {
			t.Errorf("cursor = %d, want 11 (n en la última página no mueve)", m.cursor)
		}
	})

	t.Run("p en la primera página no hace nada", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 3

		m, _ = press(m, "p")
		if m.cursor != 3 {
			t.Errorf("cursor = %d, want 3", m.cursor)
		}
	})

	t.Run("N va a la última tarea y P a la primera", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList

		m, _ = press(m, "N")
		if m.cursor != 11 {
			t.Errorf("tras N cursor = %d, want 11", m.cursor)
		}
		m, _ = press(m, "P")
		if m.cursor != 0 {
			t.Errorf("tras P cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("navegar con la lista vacía no mueve el cursor", func(t *testing.T) {
		m := modelWithTasks(t, 0, 5)
		m.currentView = viewList

		for _, k := range []string{"j", "k", "n", "p", "N", "P"} {
			m, _ = press(m, k)
			if m.cursor != 0 {
				t.Errorf("tras %q con la lista vacía cursor = %d, want 0", k, m.cursor)
			}
		}
	})

	t.Run("j con una sola tarea no se sale", func(t *testing.T) {
		m := modelWithTasks(t, 1, 5)
		m.currentView = viewList
		m.cursor = 0

		m, _ = press(m, "j")
		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})
}

// El tamaño de página efectivo: 0 o negativo cae al default de la config.
func TestListPageSizeFallsBackToDefault(t *testing.T) {
	m := newTestModel(t)
	for _, size := range []int{0, -1, -100} {
		m.pageSize = size
		if got, want := m.listPageSize(), config.DefaultPageSize; got != want {
			t.Errorf("listPageSize() con pageSize=%d = %d, want %d", size, got, want)
		}
	}
	m.pageSize = 7
	if got := m.listPageSize(); got != 7 {
		t.Errorf("listPageSize() = %d, want 7", got)
	}
}

// El separador de la barra de filtros deja dos columnas para el recuadro, y el
// suelo en cero evita que strings.Revpeat reviente con un negativo.
func TestSeparatorWidth(t *testing.T) {
	tests := []struct {
		name   string
		innerW int
		want   int
	}{
		{"holgado", 118, 116},
		{"normal", 78, 76},
		{"justo", 2, 0},
		{"una de menos", 1, 0},
		{"cero", 0, 0},
		{"negativo", -10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := separatorWidth(tt.innerW); got != tt.want {
				t.Errorf("separatorWidth(%d) = %d, want %d", tt.innerW, got, tt.want)
			}
		})
	}
}

// Nunca negativo: es lo que evita el panic de strings.Repeat.
func TestSeparatorWidthNeverNegative(t *testing.T) {
	for innerW := -100; innerW <= 300; innerW++ {
		if got := separatorWidth(innerW); got < 0 {
			t.Fatalf("separatorWidth(%d) = %d, want >= 0", innerW, got)
		}
	}
}

// El separador renderizado mide el ancho interior menos dos, y eso se comprueba
// sobre el texto plano, que es donde un "- 2" movido se nota.
func TestRenderFilterHeaderSeparatorWidth(t *testing.T) {
	for _, width := range []int{120, 80, 40} {
		m := newTestModel(t)
		m.width = width

		header := ansi.Strip(m.renderFilterHeader(width - 2))
		lines := strings.Split(header, "\n")
		if len(lines) < 2 {
			t.Fatalf("la cabecera no tiene separador:\n%s", header)
		}
		// Se cuentan los guiones, no el ancho de la línea: la barra de filtros
		// tiene un ancho natural y JoinVertical rellena todas las líneas hasta
		// la más ancha, así que la última mide siempre lo mismo.
		got := strings.Count(lines[len(lines)-1], "─")
		if want := width - 4; got != want {
			t.Errorf("con %d de ventana el separador tiene %d guiones, want %d", width, got, want)
		}
	}
}

// La página se recorta al alto disponible con la ventana siguiendo al cursor. Sin
// alto suficiente se pinta la página entera: es preferible que la caja desborde
// a que salga vacía.
func TestListWindowForHeight(t *testing.T) {
	tests := []struct {
		name                                  string
		pageStart, pageEnd, cursor, maxHeight int
		wantStart, wantEnd                    int
	}{
		{"cabe entera", 0, 10, 5, 40, 0, 10},
		{"justo cabe", 0, 10, 5, 10 + listFixedRows, 0, 10},
		// Con cinco filas para tareas y diez en la página, la ventana de cinco se
		// desplaza siguiendo al cursor.
		{"no cabe, cursor al principio", 0, 10, 0, 5 + listFixedRows, 0, 5},
		{"no cabe, cursor al final", 0, 10, 9, 5 + listFixedRows, 5, 10},
		{"no cabe, cursor en medio", 0, 10, 5, 5 + listFixedRows, 3, 8},
		{"página aparte", 20, 30, 22, 5 + listFixedRows, 20, 25},
		{"altura mínima", 0, 10, 3, listFixedRows, 0, 10},
		{"altura cero", 0, 10, 3, 0, 0, 10},
		{"altura negativa", 0, 10, 3, -50, 0, 10},
		{"página vacía", 0, 0, 0, 40, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := listWindowForHeight(tt.pageStart, tt.pageEnd, tt.cursor, tt.maxHeight)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("listWindowForHeight(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.pageStart, tt.pageEnd, tt.cursor, tt.maxHeight, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// Lo que garantiza: la ventana nunca se sale de la página, y cuando hay que
// recortar, el cursor sigue dentro.
func TestListWindowForHeightProperties(t *testing.T) {
	for pageStart := 0; pageStart <= 20; pageStart += 10 {
		for pageEnd := pageStart; pageEnd <= pageStart+12; pageEnd++ {
			for cursor := pageStart - 3; cursor <= pageEnd+3; cursor++ {
				for _, maxHeight := range []int{-10, 0, 1, listFixedRows, listFixedRows + 3, listFixedRows + 7, 40} {
					start, end := listWindowForHeight(pageStart, pageEnd, cursor, maxHeight)

					if start < pageStart || end > pageEnd || end < start {
						t.Fatalf("page=[%d,%d) cursor=%d alto=%d -> [%d,%d): fuera de la página",
							pageStart, pageEnd, cursor, maxHeight, start, end)
					}
					visible := maxHeight - listFixedRows
					if visible <= 0 || pageEnd-pageStart <= visible {
						if start != pageStart || end != pageEnd {
							t.Fatalf("page=[%d,%d) cursor=%d alto=%d -> [%d,%d), want la página entera",
								pageStart, pageEnd, cursor, maxHeight, start, end)
						}
						continue
					}
					if end-start != visible {
						t.Fatalf("page=[%d,%d) cursor=%d alto=%d -> [%d,%d): ventana de %d, want %d",
							pageStart, pageEnd, cursor, maxHeight, start, end, end-start, visible)
					}
					// El cursor se acota a la página antes de mirarlo. Fuera de la
					// página no es una posición -- el clamp general de la lista ya
					// lo dejó dentro --, y lo que tiene que quedar dentro de la
					// ventana es su equivalente en esta página.
					acotado := pageStart + clampTo(cursor-pageStart, pageEnd-pageStart)
					if acotado < start || acotado >= end {
						t.Fatalf("page=[%d,%d) cursor=%d alto=%d -> [%d,%d): el cursor quedó fuera",
							pageStart, pageEnd, cursor, maxHeight, start, end)
					}
				}
			}
		}
	}
}

// La barra de filtros muestra "Priority: all" cuando no hay filtro, y el número
// cuando lo hay. El -1 no es un filtro: es la ausencia de filtro, y el 0 es un
// filtro de verdad (sin prioridad). Por eso el cero sale como "0" y no como "all".
func TestFilterBarPriorityValue(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		want     string
	}{
		{"sin filtro", -1, "Priority: all"},
		{"filtro de cero", 0, "Priority: 0"},
		{"filtro de uno", 1, "Priority: 1"},
		{"filtro de tres", 3, "Priority: 3"},
		{"por debajo del rango", -7, "Priority: all"},
		{"por encima del rango", 9, "Priority: 9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.filterPriority = tt.priority
			m.width = 120

			out := ansi.Strip(m.renderFilterBar())
			if !strings.Contains(out, tt.want) {
				t.Errorf("con prioridad %d la barra no dice %q:\n%s", tt.priority, tt.want, out)
			}
		})
	}
}

// El filtro de estado no sale en el Gantt, porque ahí cada columna es un estado y
// la barra repetiría lo que ya se ve.
func TestFilterBarHidesStatusOnGantt(t *testing.T) {
	for _, vista := range []viewKind{viewList, viewKanban, viewDashboard} {
		m := newTestModel(t)
		m.currentView = vista
		m.width = 120
		if out := ansi.Strip(m.renderFilterBar()); !strings.Contains(out, "Status:") {
			t.Errorf("en la vista %v no sale el filtro de estado:\n%s", vista, out)
		}
	}

	m := newTestModel(t)
	m.currentView = viewGantt
	m.width = 120
	if out := ansi.Strip(m.renderFilterBar()); strings.Contains(out, "Status:") {
		t.Errorf("en el Gantt sale el filtro de estado:\n%s", out)
	}
}
