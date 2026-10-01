package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// Estas pruebas miden el render de Kanban en columnas exactas. La razón: el
// reparto de ancho entre columnas está embebido en el render y comparar "sale
// algo" no lo ve. Un signo movido cambia dónde acaba la última columna sin
// quitar ni una palabra de la pantalla.
//
// Todos los índices son por runes. Las esquinas del recuadro y los guiones
// horizontales son multibyte, y strings.Index daría la posición en bytes: con
// ellos de por medio, la última columna parecía estar tres veces más a la
// derecha de donde estaba.

// boardRow devuelve la línea del board: la primera que tiene esquinas de
// columna, y las columnas donde están sus bordes.
func boardRow(t *testing.T, out string) []rune {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		runes := []rune(line)
		for i, r := range runes {
			if r == '╔' || (r == '╭' && i > 1) {
				return runes
			}
		}
	}
	t.Fatalf("no se encontró la fila del board:\n%s", out)
	return nil
}

// La última columna llega justo al borde interior del marco, sin hueco detrás.
// El hueco va *entre* columnas: si también fuera detrás de la última, el board
// se quedaría kanbanGap columnas corto del borde sin que se notara.
func TestKanbanLastColumnReachesInnerEdge(t *testing.T) {
	tests := []struct {
		name  string
		width int
	}{
		// Sólo con holgura: si los anchos mínimos no caben, el board no llega al
		// borde y esta comprobación no dice nada del reparto.
		{"muy ancha", 240},
		{"ancha", 160},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newKanbanModel(t, 3)
			m.width = tt.width

			runes := boardRow(t, ansi.Strip(m.renderKanban(30)))
			// El marco cierra con "╮│": la esquina de la última columna y su
			// propio borde derecho, y detrás el del marco.
			if len(runes) != tt.width {
				t.Fatalf("la fila mide %d columnas, want %d", len(runes), tt.width)
			}
			// La fila acaba en "...╮│": la esquina de la última columna
			// justo en la última columna interior, y detrás el borde del marco.
			if runes[len(runes)-1] != '│' {
				t.Fatalf("la fila no termina en el borde del marco: %q", string(runes[len(runes)-3:]))
			}
			if r := runes[len(runes)-2]; r != '╮' && r != '╝' && r != '│' {
				t.Errorf("la última columna no llega al borde interior, acaba en %q: %q",
					string(r), string(runes[len(runes)-8:]))
			}
		})
	}
}

// Entre columnas hay kanbanGap de hueco, ni uno más ni uno menos. Con tres
// columnas son dos huecos.
func TestKanbanGapBetweenColumns(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 1, []string{"todo", "doing", "done"})
	m.width = 240

	runes := boardRow(t, ansi.Strip(m.renderKanban(30)))
	// Las columnas que no están seleccionadas abren con "╭" y cierran con "╮";
	// la seleccionada usa el doble borde. Los índices de las aperturas, en orden.
	var aperturas []int
	for i, r := range runes {
		if r == '╭' || r == '╔' {
			aperturas = append(aperturas, i)
		}
	}
	want := len(m.kanbanColumns())
	if len(aperturas) != want {
		t.Fatalf("encontré %d aperturas de columna y el board tiene %d: %q", len(aperturas), want, string(runes[:60]))
	}
	aperturas = aperturas[1:] // la primera es la del marco exterior

	// Cada columna ocupa su ancho más kanbanGap de hueco a la derecha. El hueco
	// se mide entre el cierre de una y la apertura de la siguiente.
	for i := 0; i+1 < len(aperturas); i++ {
		cierre := cierreDeColumna(runes, aperturas[i])
		if got := aperturas[i+1] - cierre - 1; got != kanbanGap {
			t.Errorf("entre las columnas %d y %d hay %d columnas de hueco, want %d",
				i, i+1, got, kanbanGap)
		}
	}
}

// cierreDeColumna devuelve el índice del borde derecho de la columna que abre en
// apertura, o -1 si no se encuentra.
func cierreDeColumna(runes []rune, apertura int) int {
	for i := apertura + 1; i < len(runes); i++ {
		if runes[i] == '╮' || runes[i] == '╝' {
			return i
		}
	}
	return -1
}

// La columna de "cancelled" no está en el workflow del proyecto, y aun así se
// añade al final del board en cuanto hay una tarea cancelada.
//
// Es el estado que la base de datos acepta siempre, esté o no en el workflow, así
// que una tarea puede quedar ahí sin que ninguna columna del workflow la
// contemplara. Sin esta columna esas tareas serían invisibles en el board.
func TestKanbanCancelledColumnAppearsOnlyWithCancelledTasks(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 2, []string{"todo", "done"})

	if tieneColumna(m, model.CancelledStatus) {
		t.Error("apareció la columna de canceladas sin ninguna tarea cancelada")
	}

	moverACancelada(t, m, m.tasks[0].ID)
	reloadTasks(t, m)

	if !tieneColumna(m, model.CancelledStatus) {
		t.Error("con una tarea cancelada no apareció su columna")
	}
	if tieneColumna(m, "nunca-existe") {
		t.Error("apareció una columna para un estado que no existe")
	}
}

// Y esa columna va al final, no intercalada: el orden del workflow manda y
// "cancelled" está fuera de él.
func TestKanbanCancelledColumnGoesLast(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 2, []string{"todo", "done"})
	antes := columnasDe(m)

	moverACancelada(t, m, m.tasks[0].ID)
	reloadTasks(t, m)

	despues := columnasDe(m)
	if len(despues) != len(antes)+1 {
		t.Fatalf("columnas %v -> %v, want una más", antes, despues)
	}
	if despues[len(despues)-1] != model.CancelledStatus {
		t.Errorf("la columna de canceladas quedó en %q, want la última", despues[len(despues)-1])
	}
	for i, c := range antes {
		if despues[i] != c {
			t.Errorf("el workflow se reordenó: %q pasó de la posición %d", c, i)
		}
	}
}

// moverACancelada mueve una tarea a "cancelled", que es el único estado que se
// acepta aunque no esté en el workflow del proyecto. Por eso una columna de
// canceladas puede existir sin que ningún proyecto la tenga declarada.
func moverACancelada(t *testing.T, m *Model, id int64) {
	t.Helper()
	if _, err := m.database.MoveTask(id, model.CancelledStatus); err != nil {
		t.Fatalf("MoveTask(%d, cancelled): %v", id, err)
	}
	m.filterStatus = "" // el filtro por defecto deja fuera las canceladas
}

// columnasDe devuelve los estados de las columnas del board, en orden.
func columnasDe(m *Model) []string {
	cols := m.kanbanColumns()
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.status
	}
	return out
}

// El header de una columna dice cuántas se ven y cuántas hay, y sólo lleva el
// par cuando no caben todas.
func TestKanbanHeaderCountsShownOverTotal(t *testing.T) {
	tests := []struct {
		name  string
		shown int
		total int
		want  string
	}{
		{"caban todas", 3, 3, "─ todo (3) "},
		{"no caben", 3, 9, "─ todo (3/9) "},
		{"ninguna visible", 0, 9, "─ todo (0/9) "},
		{"columna vacía", 0, 0, "─ todo (0) "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kanbanHeader("todo", tt.shown, tt.total); got != tt.want {
				t.Errorf("kanbanHeader(%d, %d) = %q, want %q", tt.shown, tt.total, got, tt.want)
			}
		})
	}
}

// Con más tarjetas que filas caben, el header de esa columna dice cuántas se ven
// de cuántas hay. Es el mismo shown/total del caso anterior, pero medido sobre
// el board de verdad: sin él, un "end - start" mal puesto no se notaría.
func TestKanbanHeaderShowsWindowWhenOverflowing(t *testing.T) {
	m := newKanbanModel(t, 8)
	m.width = 240

	out := ansi.Strip(m.renderKanban(14))
	if !strings.Contains(out, "(2/8)") && !strings.Contains(out, "(1/8)") {
		t.Errorf("el header no dice cuántas se ven de ocho:\n%s", out)
	}
	if strings.Contains(out, "(8/8)") {
		t.Errorf("con ocho tarjetas y un alto corto el header no puede decir 8/8:\n%s", out)
	}
}

// Cuando el cursor está al final de una columna más larga que la ventana, la
// ventana se desplaza y el header dice cuántas se ven desde la que está
// seleccionada, no desde la primera. Es el otro uso de "end - start": con el
// cursor arriba, start es cero y el par no distingue una cuenta de otra.
func TestKanbanHeaderFollowsScrolledWindow(t *testing.T) {
	m := newKanbanModelWithWorkflow(t, 20, []string{"todo", "done"})
	m.width = 240
	m.kanbanCol = 0
	m.kanbanRow = 19 // abajo del todo
	m.clampKanbanCursor()

	out := ansi.Strip(m.renderKanban(14))
	want := fmt.Sprintf("(%d/20)", kanbanMaxCards(14))
	if !strings.Contains(out, want) {
		t.Errorf("el header no dice %q con el cursor al final:\n%s", want, out)
	}
	if strings.Contains(out, "(1/20)") {
		t.Errorf("con el cursor abajo la ventana no puede seguir en la primera tarjeta:\n%s", out)
	}
	// Y el desplazamiento se ve en las tarjetas: la primera ya no está y la última
	// sí. El header no lo distingue, porque el número de visibles es el mismo
	// desplazamiento o no.
	if !strings.Contains(out, "tarea 19") {
		t.Errorf("con el cursor abajo no se ve la última tarjeta:\n%s", out)
	}
	if strings.Contains(out, "tarea 0 ") {
		t.Errorf("con el cursor abajo sigue en pantalla la primera tarjeta:\n%s", out)
	}
}

// Y cuando caben todas, el header no lleva el par.
func TestKanbanHeaderOmitsPairWhenAllVisible(t *testing.T) {
	m := newKanbanModel(t, 2)
	m.width = 240

	out := ansi.Strip(m.renderKanban(40))
	if strings.Contains(out, "(2/2)") {
		t.Errorf("con las dos tarjetas visibles el header no debería llevar el par:\n%s", out)
	}
	if !strings.Contains(out, "todo (2)") {
		t.Errorf("el header no dice cuántas hay:\n%s", out)
	}
}

func tieneColumna(m *Model, status string) bool {
	for _, c := range m.kanbanColumns() {
		if c.status == status {
			return true
		}
	}
	return false
}

// reloadProjects vuelve a leer los proyectos, como haría un projectsLoadedMsg.
func reloadProjects(t *testing.T, m *Model) {
	t.Helper()
	var err error
	if m.projects, err = m.database.ListProjects(); err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
}

// newKanbanModel parte de una DB con un único proyecto, el workflow indicado y n
// tarjetas en "todo".
//
// Cargar los proyectos y filtrar por "solo" importa: sin el filtro el board usa
// la unión de los workflows de todos los proyectos, y el proyecto auxiliar "api"
// de newEmptyDBModel trae el workflow por defecto, con siete estados y
// "cancelled" dentro. Las columnas del test serían las del default.
func newKanbanModel(t *testing.T, tasks int) *Model {
	t.Helper()
	return newKanbanModelWithWorkflow(t, tasks, []string{"todo", "done"})
}

func newKanbanModelWithWorkflow(t *testing.T, tasks int, workflow []string) *Model {
	t.Helper()
	m := newEmptyDBModel(t)
	mustCreateProject(t, m.database, "solo", workflow)
	for i := range tasks {
		mustCreateTask(t, m.database, "solo", fmt.Sprintf("tarea %d", i), "", "@juan", 1, "todo")
	}
	reloadTasks(t, m)
	reloadProjects(t, m)
	// El filtro de proyecto es lo que hace que el board use el workflow de
	// "solo" y no la unión de todos. Sin él, el proyecto auxiliar "api" de
	// newEmptyDBModel mete su workflow por defecto, que trae "cancelled", y la
	// columna aparecería siempre.
	m.filterProject = "solo"
	m.currentView = viewKanban
	m.width = 240
	m.clampKanbanCursor()
	return m
}

// La tarjeta de una columna lleva la prioridad si la columna la muestra, y sólo el
// título si no. Y el responsable va con sus tags detrás cuando las tiene.
func TestKanbanCardContent(t *testing.T) {
	tags := []string{"bug", "urgente"}
	conTags := model.Task{ID: 1, Title: "arreglar", Assignee: "@juan", Priority: 3, Tags: tags}

	tests := []struct {
		name          string
		task          model.Task
		showPriority  bool
		wantPrioridad bool
		wantTags      bool
	}{
		{"con prioridad y sin tags", model.Task{ID: 1, Title: "t", Assignee: "@juan", Priority: 2}, true, true, false},
		{"sin prioridad y sin tags", model.Task{ID: 1, Title: "t", Assignee: "@juan", Priority: 2}, false, false, false},
		{"con tags y con prioridad", conTags, true, true, true},
		{"con tags y sin prioridad", conTags, false, false, true},
		{"sin responsable", model.Task{ID: 1, Title: "t", Priority: 1}, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newKanbanModel(t, 0)
			col := kanbanColumn{status: "todo", tasks: []model.Task{tt.task}, showPriority: tt.showPriority}

			out := ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", false, columnWindow{end: 1}))
			// El carácter de prioridad viene con color, así que se compara sin él:
			// el render también se mide sin él.
			barra := ansi.Strip(priorityChar(tt.task.Priority))
			if tiene := strings.Contains(out, barra); tiene != tt.wantPrioridad {
				t.Errorf("la barra de prioridad %q sale=%v, want %v:\n%s",
					barra, tiene, tt.wantPrioridad, out)
			}
			// Las tags van dos columnas detrás del responsable, no pegadas. Sin ese
			// hueco los nombres se leen como un bloque y el "> 0 tags" del contador
			// deja de empezar en columna fija.
			junto := "@juan  " + strings.Join(tags, ",")
			if tiene := strings.Contains(out, junto); tiene != tt.wantTags {
				t.Errorf("el hueco con las tags sale=%v, want %v:\n%s", tiene, tt.wantTags, out)
			}
			if tiene := strings.Contains(out, strings.Join(tags, ",")); tiene != tt.wantTags {
				t.Errorf("las tags salen=%v, want %v:\n%s", tiene, tt.wantTags, out)
			}
			if !strings.Contains(out, "t") && !strings.Contains(out, "arreglar") {
				t.Errorf("no sale el título:\n%s", out)
			}
		})
	}
}

// La tarjeta seleccionada lleva "> " delante y las demás "  ". Es la única
// diferencia entre ellas, así que lo que se compara es el prefijo de la primera
// línea de la tarjeta, no su texto: el texto lleva la barra de prioridad con
// color y comparar sobre el render sin color no lo deja claro.
func TestKanbanSelectedCardIsMarked(t *testing.T) {
	m := newKanbanModel(t, 0)
	task := model.Task{ID: 1, Title: "tarea", Assignee: "@juan", Priority: 2}
	// Con showPriority la tarjeta ya trae su propio prefijo de dos columnas, así
	// que el prefijo de selección se distingue del de la tarjeta. Sin la barra,
	// los dos prefijos se solapan y quitarlos sería invisible.
	col := kanbanColumn{status: "todo", tasks: []model.Task{task}, showPriority: true}

	seleccionada := primeraTarjeta(t, ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", true, columnWindow{end: 1})), "tarea")
	if !strings.HasPrefix(seleccionada, "> ") {
		t.Errorf("la tarjeta seleccionada empieza por %q, want \"> \"", seleccionada)
	}

	noSeleccionada := primeraTarjeta(t, ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", false, columnWindow{end: 1})), "tarea")
	if strings.HasPrefix(noSeleccionada, "> ") {
		t.Errorf("una tarjeta no seleccionada empieza por %q", noSeleccionada)
	}
	if !strings.HasPrefix(noSeleccionada, "    ") {
		t.Errorf("una tarjeta no seleccionada no lleva el prefijo de dos columnas: %q", noSeleccionada)
	}
}

// Con dos tarjetas y el cursor en la segunda, sólo esa lleva la marca.
func TestKanbanOnlyCursorRowIsMarked(t *testing.T) {
	m := newKanbanModel(t, 0)
	m.kanbanRow = 1
	col := kanbanColumn{status: "todo", tasks: []model.Task{
		{ID: 1, Title: "primera", Assignee: "@juan", Priority: 2},
		{ID: 2, Title: "segunda", Assignee: "@juan", Priority: 2},
	}}

	out := ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", true, columnWindow{end: 2}))
	if n := strings.Count(out, "> "); n != 1 {
		t.Errorf("hay %d marcas, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "segunda") {
		t.Errorf("no sale la segunda tarjeta:\n%s", out)
	}
}

// primeraTarjeta devuelve la primera línea que contiene el texto de una tarjeta,
// con el borde izquierdo de la columna quitado. El resto del recuadro -- la caja,
// la cabecera -- se salta buscando la línea que trae el texto.
//
// Todo por runes: los bordes y los guiones son multibyte, y un corte por bytes
// parte el carácter y devuelve basura.
func primeraTarjeta(t *testing.T, renderizado, titulo string) string {
	t.Helper()
	for _, linea := range strings.Split(renderizado, "\n") {
		i := strings.Index(linea, titulo)
		if i < 0 {
			continue
		}
		runes := []rune(linea[:i])
		// Fuera el borde izquierdo: "║", "│", "╔" o "╭".
		for len(runes) > 0 && strings.ContainsRune("║│╔╭", runes[0]) {
			runes = runes[1:]
		}
		return string(runes)
	}
	t.Fatalf("la columna no tiene la tarjeta %q:\n%s", titulo, renderizado)
	return ""
}

// Una columna sin tarjetas dice "(empty)" en vez de salirse con una caja vacía.
func TestKanbanEmptyColumnSaysSo(t *testing.T) {
	m := newKanbanModel(t, 0)
	col := kanbanColumn{status: "todo", tasks: nil}

	out := ansi.Strip(m.renderKanbanColumn(col, 40, "─ todo ", false, columnWindow{}))
	if !strings.Contains(out, "(empty)") {
		t.Errorf("una columna vacía no lo dice:\n%s", out)
	}
}
