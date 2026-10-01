package tui

import (
	"testing"

	"tsk/internal/config"
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

// clampTo es el borde que repetían cuatro clamps distintos del programa. Con
// lista vacía devuelve 0, que es lo que todos ellos mostraban.
func TestClampTo(t *testing.T) {
	tests := []struct {
		name   string
		idx, n int
		want   int
	}{
		{"dentro", 2, 5, 2},
		{"último válido", 4, 5, 4},
		{"uno más allá", 5, 5, 4},
		{"muy más allá", 99, 5, 4},
		{"negativo", -3, 5, 0},
		{"lista de uno", 7, 1, 0},
		{"lista vacía", 4, 0, 0},
		{"lista vacía con negativo", -4, 0, 0},
		{"n negativo", 4, -2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampTo(tt.idx, tt.n); got != tt.want {
				t.Errorf("clampTo(%d, %d) = %d, want %d", tt.idx, tt.n, got, tt.want)
			}
		})
	}
}

func TestClampToAlwaysInRange(t *testing.T) {
	for n := 0; n <= 8; n++ {
		for idx := -5; idx <= 13; idx++ {
			got := clampTo(idx, n)
			if n <= 0 {
				if got != 0 {
					t.Fatalf("clampTo(%d, %d) = %d, want 0 sin lista", idx, n, got)
				}
				continue
			}
			if got < 0 || got >= n {
				t.Fatalf("clampTo(%d, %d) = %d, fuera de [0,%d)", idx, n, got, n)
			}
		}
	}
}

// firstValidIndex es el "si el índice no vale, el primero" sin mirar elementos.
func TestFirstValidIndex(t *testing.T) {
	tests := []struct {
		name   string
		idx, n int
		want   int
	}{
		{"dentro", 2, 5, 2},
		{"negativo", -1, 5, 0},
		{"fuera por arriba", 9, 5, 4},
		{"lista de uno", 3, 1, 0},
		{"lista vacía", 3, 0, 0},
		{"lista negativa", 3, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstValidIndex(tt.idx, tt.n); got != tt.want {
				t.Errorf("firstValidIndex(%d, %d) = %d, want %d", tt.idx, tt.n, got, tt.want)
			}
		})
	}
}

// firstOrAt devuelve el primer elemento cuando el índice no vale. Es lo que
// distingue esta función de taskAt2, que devuelve "".
func TestFirstOrAt(t *testing.T) {
	items := []string{"a", "b", "c"}
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{"primera", 0, "a"},
		{"del medio", 1, "b"},
		{"última", 2, "c"},
		{"negativo cae en la primera", -1, "a"},
		{"fuera por arriba cae en la última", 9, "c"},
		{"uno más allá cae en la última", 3, "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstOrAt(items, tt.idx); got != tt.want {
				t.Errorf("firstOrAt(%d) = %q, want %q", tt.idx, got, tt.want)
			}
		})
	}
}

// Y sobre lista vacía devuelve "", como taskAt2: no hay primer elemento.
func TestFirstOrAtEmpty(t *testing.T) {
	for _, idx := range []int{-1, 0, 5} {
		if got := firstOrAt(nil, idx); got != "" {
			t.Errorf("firstOrAt(nil, %d) = %q, want vacío", idx, got)
		}
	}
}

// taskAt2 devuelve "" fuera de rango, a diferencia de firstOrAt.
func TestTaskAt2OutOfRange(t *testing.T) {
	items := []string{"a", "b", "c"}
	for _, idx := range []int{-1, 3, 99} {
		if got := taskAt2(items, idx); got != "" {
			t.Errorf("taskAt2(%d) = %q, want vacío fuera de rango", idx, got)
		}
	}
	if got := taskAt2(items, 1); got != "b" {
		t.Errorf("taskAt2(1) = %q, want b", got)
	}
	if got := taskAt2(nil, 0); got != "" {
		t.Errorf("taskAt2(nil, 0) = %q, want vacío", got)
	}
}

// La selección del detalle no envuelve por arriba: -1 significa "nada
// seleccionado", y desde ahí "j" va al primer comentario, no al segundo.
func TestNextCommentSel(t *testing.T) {
	tests := []struct {
		name   string
		sel, n int
		want   int
	}{
		{"nada seleccionado va al primero", -1, 3, 0},
		{"del primero al segundo", 0, 3, 1},
		{"del segundo al tercero", 1, 3, 2},
		{"en el último se queda", 2, 3, 2},
		{"más allá del último se queda", 9, 3, 2},
		{"un solo comentario", -1, 1, 0},
		{"un solo comentario ya en el", 0, 1, 0},
		{"sin comentarios", -1, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextCommentSel(tt.sel, tt.n); got != tt.want {
				t.Errorf("nextCommentSel(%d, %d) = %d, want %d", tt.sel, tt.n, got, tt.want)
			}
		})
	}
}

// Nunca sale de [0, n-1] para n >= 1, y es monótona hasta saturar en el último.
func TestNextCommentSelStaysInRange(t *testing.T) {
	for n := 1; n <= 6; n++ {
		prev := -1
		for sel := -1; sel <= 8; sel++ {
			got := nextCommentSel(sel, n)
			if got < 0 || got > n-1 {
				t.Fatalf("nextCommentSel(%d, %d) = %d, fuera de [0,%d]", sel, n, got, n-1)
			}
			if got < prev {
				t.Fatalf("nextCommentSel(%d, %d) = %d retrocede desde %d", sel, n, got, prev)
			}
			prev = got
		}
	}
}

// Hacia atrás, el -1 es un tope real: desde el primer comentario se vuelve a
// "nada seleccionado" y de ahí no se sale hacia -2.
func TestPrevCommentSel(t *testing.T) {
	tests := []struct {
		name string
		sel  int
		want int
	}{
		{"del último", 3, 2},
		{"del segundo", 1, 0},
		{"del primero vuelve a nada", 0, -1},
		{"desde nada se queda", -1, -1},
		{"muy negativo", -7, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prevCommentSel(tt.sel); got != tt.want {
				t.Errorf("prevCommentSel(%d) = %d, want %d", tt.sel, got, tt.want)
			}
		})
	}
}

func TestPrevCommentSelNeverBelowMinusOne(t *testing.T) {
	for sel := -20; sel <= 20; sel++ {
		if got := prevCommentSel(sel); got < -1 {
			t.Fatalf("prevCommentSel(%d) = %d, want >= -1", sel, got)
		}
	}
}

// Un pageSize de 0 o negativo no es "una página": es configuración ausente, y
// cae al valor por defecto igual que el resto de la config.
func TestResolvePageSize(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"cero", 0, config.DefaultPageSize},
		{"negativo", -1, config.DefaultPageSize},
		{"muy negativo", -100, config.DefaultPageSize},
		{"uno", 1, 1},
		{"diez", 10, 10},
		{"muy grande", 100000, 100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePageSize(tt.in); got != tt.want {
				t.Errorf("resolvePageSize(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// El editor por defecto son las tres rutas de "c" (comentario), "E" (edición
// completa) y la de descripción. Comparten función para que no diverjan.
func TestEditorCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"sin configurar", "", "nvim"},
		{"vim", "vim", "vim"},
		{"con argumentos", "code --wait", "code --wait"},
		{"un espacio no es vacío", " ", " "},
		{"guion", "-", "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editorCommand(tt.in); got != tt.want {
				t.Errorf("editorCommand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// El reparto del Gantt tiene dos regímenes y dos suelos. Como función pura se
// comprueba sobre un barrido de anchos, que es lo que la caja rellenada con
// espacios no dejaba ver.
func TestGanttLabelAndDays(t *testing.T) {
	tests := []struct {
		name        string
		innerW      int
		wantLabel   int
		wantDayCols int
	}{
		{"holgado", 118, 30, 87},
		{"justo en el umbral", 70, 30, 39},
		{"una menos que el umbral", 69, 23, 45},
		{"un tercio exacto", 60, 20, 39},
		{"con suelo de etiqueta", 45, 15, 29},
		{"etiqueta al mínimo", 43, 14, 28},
		{"suelo de etiqueta por la regla de un tercio", 30, 14, 15},
		{"días al mínimo", 20, 14, 7},
		{"por debajo del suelo de días", 18, 14, 7},
		{"cero", 0, 14, 7},
		{"negativo", -50, 14, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, days := ganttLabelAndDays(tt.innerW)
			if label != tt.wantLabel || days != tt.wantDayCols {
				t.Errorf("ganttLabelAndDays(%d) = (%d, %d), want (%d, %d)",
					tt.innerW, label, days, tt.wantLabel, tt.wantDayCols)
			}
		})
	}
}

// Las dos mitades más el separador llenan el ancho interior mientras haya sitio
// para las columnas de día; cuando no hay, la etiqueta manda y los días se
// quedan en su mínimo.
func TestGanttLabelAndDaysProperties(t *testing.T) {
	for innerW := -20; innerW <= 300; innerW++ {
		label, days := ganttLabelAndDays(innerW)
		if label < ganttMinLabelWidth {
			t.Fatalf("innerW=%d: etiqueta %d, want >= %d", innerW, label, ganttMinLabelWidth)
		}
		if days < ganttMinDayCols {
			t.Fatalf("innerW=%d: días %d, want >= %d", innerW, days, ganttMinDayCols)
		}
		if label+1+days > innerW {
			// Sólo puede pasar cuando los mínimos no caben, que es lo que hace
			// el suelo: preferimos desbordar a quedarnos sin día visible.
			if label != ganttMinLabelWidth || days != ganttMinDayCols {
				t.Fatalf("innerW=%d: (%d + 1 + %d) se sale y no está en los mínimos (%d, %d)",
					innerW, label, days, label, days)
			}
		}
	}
}

// El régimen cambia justo en el umbral: 70 columnas interiores mantienen la
// etiqueta fija, 69 la dividen. Ese par es el que distingue el ">=" del "<".
func TestGanttLabelAndDaysThresholdIsExact(t *testing.T) {
	if l, _ := ganttLabelAndDays(ganttLabelThreshold); l != ganttFixedLabelWidth {
		t.Errorf("en el umbral la etiqueta es %d, want %d", l, ganttFixedLabelWidth)
	}
	if l, _ := ganttLabelAndDays(ganttLabelThreshold - 1); l == ganttFixedLabelWidth {
		t.Errorf("una columna por debajo del umbral la etiqueta sigue siendo %d, want el tercio",
			ganttFixedLabelWidth)
	}
}
