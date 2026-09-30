package tui

import (
	"testing"

	"tsk/internal/model"
)

// Estas funciones se extrajeron de la navegación de List, Kanban y Dashboard
// para poder comprobarlas exhaustivamente. Antes cada call site llevaba su
// propia copia de la cuenta con su propio borde, y eso era a la vez la fuente
// de los minion y la razón por la que no había forma de fijarlos sin montar una
// vista entera y una base de datos.

// cycleIndex da la vuelta por el final: es lo que hacen j/k en un ciclo, donde
// bajar desde la última fila vuelve a la primera.
func TestCycleIndex(t *testing.T) {
	tests := []struct {
		name      string
		idx, n, d int
		want      int
	}{
		{"abajo en medio", 1, 3, 1, 2},
		{"abajo en la última da la vuelta", 2, 3, 1, 0},
		{"arriba en la primera da la vuelta", 0, 3, -1, 2},
		{"arriba en medio", 1, 3, -1, 0},
		{"lista de uno siempre es cero", 0, 1, 1, 0},
		{"lista de uno arriba también", 0, 1, -1, 0},
		{"lista vacía no mueve", 5, 0, 1, 5},
		{"lista vacía no mueve hacia atrás", 5, 0, -1, 5},
		{"n negativo no mueve", 3, -2, 1, 3},
		{"índice fuera de rango se normaliza", 7, 3, 1, 2},
		{"salto mayor que la lista", 0, 3, 5, 2},
		{"salto negativo mayor que la lista", 0, 3, -5, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cycleIndex(tt.idx, tt.n, tt.d); got != tt.want {
				t.Errorf("cycleIndex(%d, %d, %d) = %d, want %d", tt.idx, tt.n, tt.d, got, tt.want)
			}
		})
	}
}

// La propiedad que hace que cycleIndex sea utilizable en navegación: para
// cualquier delta, el resultado siempre está dentro de la lista.
func TestCycleIndexAlwaysInRange(t *testing.T) {
	for n := 1; n <= 8; n++ {
		for idx := -2; idx <= n+2; idx++ {
			for d := -9; d <= 9; d++ {
				got := cycleIndex(idx, n, d)
				if got < 0 || got >= n {
					t.Fatalf("cycleIndex(%d, %d, %d) = %d, fuera de [0,%d)", idx, n, d, got, n)
				}
			}
		}
	}
}

// Y recorrer con delta +1 n veces vuelve al punto de partida, para cualquier
// índice inicial.
func TestCycleIndexIsAPermutation(t *testing.T) {
	for n := 1; n <= 8; n++ {
		for start := range n {
			seen := map[int]bool{}
			idx := start
			for range n {
				if seen[idx] {
					t.Fatalf("n=%d: el ciclo repitió %d antes de completar %d vueltas", n, idx, n)
				}
				seen[idx] = true
				idx = cycleIndex(idx, n, 1)
			}
			if idx != start {
				t.Errorf("n=%d: tras %d pasos desde %d se vuelve a %d", n, n, start, idx)
			}
		}
	}
}

// shiftIndex se queda en el extremo en vez de dar la vuelta: es lo que hacen las
// flechas al moverse entre columnas del Kanban.
func TestShiftIndex(t *testing.T) {
	tests := []struct {
		name      string
		idx, n, d int
		want      int
	}{
		{"a la derecha en medio", 1, 3, 1, 2},
		{"a la derecha en la última se queda", 2, 3, 1, 2},
		{"a la izquierda en medio", 1, 3, -1, 0},
		{"a la izquierda en la primera se queda", 0, 3, -1, 0},
		{"lista vacía devuelve cero", 4, 0, 1, 0},
		{"lista negativa devuelve cero", 4, -1, -1, 0},
		{"índice fuera de rango se recorta", 9, 3, 1, 2},
		{"índice negativo se recorta", -9, 3, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shiftIndex(tt.idx, tt.n, tt.d); got != tt.want {
				t.Errorf("shiftIndex(%d, %d, %d) = %d, want %d", tt.idx, tt.n, tt.d, got, tt.want)
			}
		})
	}
}

func TestShiftIndexAlwaysInRange(t *testing.T) {
	for n := 1; n <= 8; n++ {
		for idx := -3; idx <= n+3; idx++ {
			for d := -5; d <= 5; d++ {
				got := shiftIndex(idx, n, d)
				if got < 0 || got >= n {
					t.Fatalf("shiftIndex(%d, %d, %d) = %d, fuera de [0,%d)", idx, n, d, got, n)
				}
			}
		}
	}
}

// inRange es la guarda antes de cada tasks[idx]: su borde inferior importa
// (un -1 indexaría por detrás) y el superior también (fuera de rango revienta).
func TestInRange(t *testing.T) {
	tests := []struct {
		name   string
		idx, n int
		want   bool
	}{
		{"dentro", 0, 1, true},
		{"último válido", 2, 3, true},
		{"uno más allá", 3, 3, false},
		{"negativo", -1, 3, false},
		{"lista vacía", 0, 0, false},
		{"lista vacía y negativo", -1, 0, false},
		{"lista negativa", 0, -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inRange(tt.idx, tt.n); got != tt.want {
				t.Errorf("inRange(%d, %d) = %v, want %v", tt.idx, tt.n, got, tt.want)
			}
		})
	}
}

// nextPriority: el ciclo depende del estado. En backlog hay cuatro peldaños
// (incluye none) y fuera de backlog sólo tres, así que la misma prioridad
// avanza distinto según dónde esté la tarea.
func TestNextPriority(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		priority int
		want     int
	}{
		// backlog: none → low → med → high → none
		{"backlog desde none", "backlog", model.PriorityNone, model.PriorityLow},
		{"backlog desde low", "backlog", model.PriorityLow, model.PriorityMedium},
		{"backlog desde med", "backlog", model.PriorityMedium, model.PriorityHigh},
		{"backlog desde high da la vuelta", "backlog", model.PriorityHigh, model.PriorityNone},

		// fuera de backlog: low → med → high → low
		{"doing desde low", "doing", model.PriorityLow, model.PriorityMedium},
		{"doing desde med", "doing", model.PriorityMedium, model.PriorityHigh},
		{"doing desde high da la vuelta", "doing", model.PriorityHigh, model.PriorityLow},
		// none NO está en el ciclo de fuera de backlog: salta directo a low.
		{"doing desde none entra al ciclo", "doing", model.PriorityNone, model.PriorityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPriority(tt.status, tt.priority); got != tt.want {
				t.Errorf("nextPriority(%q, %d) = %d, want %d", tt.status, tt.priority, got, tt.want)
			}
		})
	}
}

// La prioridad se recorta a 0..3 antes de ciclar: llega de la base y allí no
// hay garantía de rango. Sin el recorte un 7 con el ciclo de tres peldaños se
// quedaría en 7 (= 1 = low) y "subir" no movería nada.
func TestNextPriorityClampsOutOfRange(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		priority int
		want     int
	}{
		{"backlog desde 7 acota a high y vuelve a none", "backlog", 7, model.PriorityNone},
		{"backlog desde 99", "backlog", 99, model.PriorityNone},
		{"backlog desde -5 acota a none", "backlog", -5, model.PriorityLow},
		{"doing desde 7 acota a high y vuelve a low", "doing", 7, model.PriorityLow},
		{"doing desde -5 acota a none", "doing", -5, model.PriorityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPriority(tt.status, tt.priority); got != tt.want {
				t.Errorf("nextPriority(%q, %d) = %d, want %d", tt.status, tt.priority, got, tt.want)
			}
		})
	}
}

// La salida siempre cae en el rango de prioridades válido.
func TestNextPriorityAlwaysValid(t *testing.T) {
	statuses := []string{"backlog", "todo", "doing", "reviewing", "done"}
	for _, status := range statuses {
		for p := -3; p <= 9; p++ {
			got := nextPriority(status, p)
			if got < model.PriorityNone || got > model.PriorityHigh {
				t.Errorf("nextPriority(%q, %d) = %d, fuera del rango 0-3", status, p, got)
			}
		}
	}
}

// Ciclar muchas veces en backlog recorre los cuatro estados y vuelve al punto
// de partida; fuera de backlog recorre los tres.
func TestNextPriorityCycles(t *testing.T) {
	t.Run("backlog recorre 4 estados", func(t *testing.T) {
		p := model.PriorityNone
		for range 4 {
			p = nextPriority("backlog", p)
		}
		if p != model.PriorityNone {
			t.Errorf("4 pasos en backlog = %d, want %d (vuelta al inicio)", p, model.PriorityNone)
		}
	})

	t.Run("fuera de backlog recorre 3 estados", func(t *testing.T) {
		p := model.PriorityLow
		for range 3 {
			p = nextPriority("doing", p)
		}
		if p != model.PriorityLow {
			t.Errorf("3 pasos fuera de backlog = %d, want %d (vuelta al inicio)", p, model.PriorityLow)
		}
	})
}

// taskAt es la guarda que estaba escrita delante de cada tasks[m.cursor].
// Recibir el índice como parámetro es lo que permite matar su mutante de
// BOUNDARY: idx == len(tasks) no ocurre nunca desde el teclado porque el cursor
// llega acotado, pero aquí se puede pedir exactamente ese caso.
func TestTaskAt(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Title: "primera"},
		{ID: 2, Title: "segunda"},
		{ID: 3, Title: "tercera"},
	}
	tests := []struct {
		name   string
		tasks  []model.Task
		idx    int
		wantID int64
		wantOK bool
	}{
		{"primera", tasks, 0, 1, true},
		{"del medio", tasks, 1, 2, true},
		{"última", tasks, 2, 3, true},
		{"uno más allá", tasks, 3, 0, false},
		{"muy más allá", tasks, 99, 0, false},
		{"negativo", tasks, -1, 0, false},
		{"lista vacía", nil, 0, 0, false},
		{"lista vacía con índice negativo", nil, -5, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskAt(tt.tasks, tt.idx)
			if !tt.wantOK {
				if got != nil {
					t.Fatalf("taskAt(%d) = %+v, want nil", tt.idx, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("taskAt(%d) = nil, want la tarea %d", tt.idx, tt.wantID)
			}
			if got.ID != tt.wantID {
				t.Errorf("taskAt(%d).ID = %d, want %d", tt.idx, got.ID, tt.wantID)
			}
		})
	}
}

// Con cualquier lista y cualquier índice, taskAt devuelve algo o nil, nunca
// entra en pánico: es lo que hace seguro usarlo delante de cada tasks[idx].
func TestTaskAtNeverPanics(t *testing.T) {
	sizes := []int{0, 1, 2, 5}
	for _, n := range sizes {
		tasks := make([]model.Task, n)
		for i := range tasks {
			tasks[i] = model.Task{ID: int64(i + 1)}
		}
		for idx := -3; idx <= n+3; idx++ {
			got := taskAt(tasks, idx)
			if inRange(idx, n) {
				if got == nil || got.ID != int64(idx+1) {
					t.Fatalf("taskAt(n=%d, %d) = %+v, want la tarea %d", n, idx, got, idx+1)
				}
			} else if got != nil {
				t.Fatalf("taskAt(n=%d, %d) = %+v, want nil", n, idx, got)
			}
		}
	}
}

// previewBudgetFor: lo que cabe entre el mínimo de 1 línea y el tope, sin dejar
// que el preview empuje el contenido fuera de la pantalla.
func TestPreviewBudgetFor(t *testing.T) {
	const keybinds = 2
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{"terminal normal da el tope", 60, previewMaxLines},
		{"justo en el tope", previewMaxLines + keybinds + minContentHeight + 2, previewMaxLines},
		{"un poco menos del tope", previewMaxLines + keybinds + minContentHeight + 1, previewMaxLines - 1},
		{"terminal pequeño da menos", 20, 20 - keybinds - minContentHeight - 2},
		{"terminal mínimo da 1", 10, 1},
		{"altura cero da 1", 0, 1},
		{"altura negativa da 1", -10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := previewBudgetFor(tt.height, keybinds)
			if got != tt.want {
				t.Errorf("previewBudgetFor(%d, %d) = %d, want %d", tt.height, keybinds, got, tt.want)
			}
			if got < 1 || got > previewMaxLines {
				t.Errorf("previewBudgetFor(%d, %d) = %d, fuera de [1,%d]", tt.height, keybinds, got, previewMaxLines)
			}
		})
	}
}

// Más keybinds dejan menos presupuesto, nunca más.
func TestPreviewBudgetShrinksWithKeybinds(t *testing.T) {
	for height := 12; height <= 60; height++ {
		prev := previewBudgetFor(height, 0)
		for k := 1; k <= 8; k++ {
			got := previewBudgetFor(height, k)
			if got > prev {
				t.Errorf("height=%d: más keybinds (%d)dio más presupuesto: %d > %d", height, k, got, prev)
			}
			prev = got
		}
	}
}

// El presupuesto nunca sale del rango acotado, para cualquier entrada.
func TestPreviewBudgetAlwaysBounded(t *testing.T) {
	for height := -5; height <= 80; height++ {
		for keybinds := -2; keybinds <= 20; keybinds++ {
			got := previewBudgetFor(height, keybinds)
			if got < 1 || got > previewMaxLines {
				t.Fatalf("previewBudgetFor(%d, %d) = %d, fuera de [1,%d]", height, keybinds, got, previewMaxLines)
			}
		}
	}
}

// matchesStatus tiene tres modos: activas, todas, o un estado exacto.
func TestMatchesStatus(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		status string
		active bool
		want   bool
	}{
		{"todas las activas acepta una activa", statusFilterAllActive, "doing", true, true},
		{"todas las activas rechaza una inactiva", statusFilterAllActive, "done", false, false},
		{"todas no restringe", "", "done", false, true},
		{"todas no restringe con activa", "", "todo", true, true},
		{"estado exacto coincide", "doing", "doing", false, true},
		{"estado exacto no coincide", "doing", "todo", false, false},
		{"estado exacto con activa tampoco filtra", "todo", "todo", true, true},
		{"un estado desconocido sólo coincide consigo mismo", "nuevo", "nuevo", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesStatus(tt.filter, tt.status, tt.active); got != tt.want {
				t.Errorf("matchesStatus(%q, %q, %v) = %v, want %v", tt.filter, tt.status, tt.active, got, tt.want)
			}
		})
	}
}

// El filtro de estado "todas las activas" NO mira el campo status, sólo el
// flag active: es lo que distingue ese modo del de coincidencia exacta.
func TestMatchesStatusAllActiveIgnoresStatusName(t *testing.T) {
	for _, status := range []string{"todo", "doing", "done", "cancelled", "cualquiera"} {
		if !matchesStatus(statusFilterAllActive, status, true) {
			t.Errorf("con active=true el estado %q debería pasar", status)
		}
		if matchesStatus(statusFilterAllActive, status, false) {
			t.Errorf("con active=false el estado %q no debería pasar", status)
		}
	}
}

// matchesPriority: -1 es "cualquiera", cualquier otro valor exige coincidencia.
func TestMatchesPriority(t *testing.T) {
	tests := []struct {
		name           string
		filter, actual int
		want           bool
	}{
		{"sin filtro acepta cualquiera", -1, 0, true},
		{"sin filtro acepta la máxima", -1, 3, true},
		{"filtro none coincide con none", 0, 0, true},
		{"filtro none no coincide con high", 0, 3, false},
		{"filtro high coincide con high", 3, 3, true},
		{"filtro high no coincide con none", 3, 0, false},
		// Cualquier negativo desactiva el filtro, no sólo -1: es lo que hacía
		// el `if m.filterPriority >= 0 && ...` original y se conserva.
		{"otro negativo también desactiva el filtro", -2, 1, true},
		{"otro negativo con la misma prioridad", -2, -2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPriority(tt.filter, tt.actual); got != tt.want {
				t.Errorf("matchesPriority(%d, %d) = %v, want %v", tt.filter, tt.actual, got, tt.want)
			}
		})
	}
}
