package tui

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

// Un segundo turno en la interfaz: aquí viven los caminos donde una escritura
// funciona y la lectura que va detrás falla, que es el patrón que la base en
// sólo lectura no alcanza. Para provocarlos hay que dejar la tabla con el tipo
// que el Scan espera cambiado -- las tablas no son STRICT, así que el truco no
// es un valor raro sino quitar la autoincrementación: una fila con id nulo no se
// puede escanear en un int64.

// sabotearCommentsDejaFilaIlegible convierte la tabla de comentarios en una sin
// clave primaria y le mete una fila con id nulo, de modo que cualquier lectura
// falle. Las escrituras siguen funcionando: por eso sirve para el patrón
// "escribo y luego no puedo leer".
func sabotearCommentsDejaFilaIlegible(t *testing.T, m *Model, taskID int64) {
	t.Helper()
	conn := m.database.Conn()
	if _, err := conn.Exec(`DROP TABLE comments`); err != nil {
		t.Fatalf("DROP TABLE comments: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE comments (
		id TEXT, task_id INTEGER NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("CREATE TABLE comments: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO comments (id, task_id, body, created_at) VALUES (NULL, ?, 'ilegible', '2020-01-01')`,
		taskID,
	); err != nil {
		t.Fatalf("INSERT INTO comments: %v", err)
	}
	// Y uno con id válido, para que borrar un comentario siga teniendo algo que
	// borrar: lo que se quiere fallar es la LECTURA posterior, no el borrado.
	if _, err := conn.Exec(
		`INSERT INTO comments (id, task_id, body, created_at) VALUES ('7', ?, 'real', '2020-01-01')`,
		taskID,
	); err != nil {
		t.Fatalf("INSERT INTO comments: %v", err)
	}
}

// sabotearTasksDejaFilaIlegible hace lo mismo con las tareas, para los caminos
// que escriben una tarea y recargan el listado detrás.
func sabotearTasksDejaFilaIlegible(t *testing.T, m *Model, projectID int64) {
	t.Helper()
	conn := m.database.Conn()
	if _, err := conn.Exec(`DROP TABLE tasks`); err != nil {
		t.Fatalf("DROP TABLE tasks: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE tasks (
		id TEXT, project_id INTEGER NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'backlog', priority INTEGER NOT NULL DEFAULT 0,
		assignee TEXT NOT NULL DEFAULT '', position INTEGER NOT NULL DEFAULT 0,
		estimate REAL NOT NULL DEFAULT 0, tags TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')), completed_at TEXT)`); err != nil {
		t.Fatalf("CREATE TABLE tasks: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO tasks (id, project_id, title) VALUES (NULL, ?, 'ilegible')`, projectID,
	); err != nil {
		t.Fatalf("INSERT INTO tasks: %v", err)
	}
}

func projectIDDe(t *testing.T, m *Model) int64 {
	t.Helper()
	if len(m.projects) == 0 {
		t.Fatal("el modelo no tiene proyectos cargados")
	}
	return m.projects[0].ID
}

// Guardar un comentario y cargar la lista son dos operaciones seguidas. Si la
// escritura va y la lectura no, el mensaje sale con la lista vacía y sin
// selección, que es lo que evita que la interfaz seleção un índice imposible.
func TestCommentCmdsWhenTheReadAfterTheWriteFails(t *testing.T) {
	t.Run("añadir", func(t *testing.T) {
		m := newDetailModel(t, 0)
		taskID := m.detailTask.ID
		sabotearCommentsDejaFilaIlegible(t, m, taskID)

		msg := mustMsg(t, m.addCommentCmd(taskID, "nuevo"))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentsLoadedMsg", msg)
		}
		if len(loaded.comments) != 0 || loaded.selectIdx != -1 {
			t.Errorf("han salido %d comentarios y sel=%d, want 0 y -1",
				len(loaded.comments), loaded.selectIdx)
		}
	})

	t.Run("borrar", func(t *testing.T) {
		m := newDetailModel(t, 2)
		taskID := m.detailTask.ID
		sabotearCommentsDejaFilaIlegible(t, m, taskID)

		// El id 7 es el que metió el sabotaje: es el comentario borrable.
		msg := mustMsg(t, m.deleteCommentCmd(taskID, 7, 1))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentsLoadedMsg", msg)
		}
		// El borrado sí funcionó: el mensaje no debe conservar la selección que
		// se queda sólo para el caso de fallo del borrado.
		if loaded.selectIdx != -1 {
			t.Errorf("selectIdx = %d, want -1", loaded.selectIdx)
		}
	})
}

// Los tres caminos donde se escribe y luego se recarga el listado: el alta de
// tarea, el guardado de la descripción inline y el guardado del editor externo.
// Con la lectura rota, no hay recarga, y el modelo se queda con lo que tenía.
func TestWriteThenReloadWhenTheReloadFails(t *testing.T) {
	t.Run("alta de tarea", func(t *testing.T) {
		m := newTestModel(t)
		abierto, _ := pulsar(t, m, "i")
		conTitulo, _ := pulsar(t, abierto, "n", "u", "e", "v", "a")

		sabotearTasksDejaFilaIlegible(t, conTitulo, projectIDDe(t, conTitulo))

		cmd := conTitulo.createTaskCmd(
			conTitulo.currentProjectName(), "nueva", "", "Me", model.PriorityLow, nil)
		if cmd == nil {
			t.Fatal("crear la tarea no ha lanzado ningún comando")
		}
		if msg := cmd(); msg != nil {
			t.Errorf("con el listado ilegible ha salido %T, want nil", msg)
		}
	})

	t.Run("descripción inline", func(t *testing.T) {
		m := newTestModel(t)
		id := m.tasks[0].ID
		sabotearTasksDejaFilaIlegible(t, m, projectIDDe(t, m))

		if msg := m.saveDescriptionCmd(id, "nueva")(); msg != nil {
			t.Errorf("con el listado ilegible ha salido %T, want nil", msg)
		}
	})

	t.Run("editor externo", func(t *testing.T) {
		m := newTestModel(t)
		id := m.tasks[0].ID
		sabotearTasksDejaFilaIlegible(t, m, projectIDDe(t, m))

		if msg := m.updateTaskFromEdit(id, "Title: otro\n")(); msg != nil {
			t.Errorf("con el listado ilegible ha salido %T, want nil", msg)
		}
	})
}

// Poner una tag escribe en dos sitios: la tabla de tasks_tags y, en algunos
// caminos, la propia tarea. Con la lectura de la tarea rota, el comando entero
// se queda sin mensaje.
func TestToggleTagWhenTheWriteFails(t *testing.T) {
	m := newTestModel(t)
	tarea := m.tasks[0]
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_ = tarea

	if msg := m.toggleTagCmd(1, "nueva")(); msg != nil {
		t.Errorf("con la base cerrada ha salido %T, want nil", msg)
	}
}

// El modal de personas tiene un segundo nivel con sus propias teclas. Sin días
// libres, la navegación no tiene nada que recorrer y "a" sigue funcionando;
// con días libres, todo el juego de teclas aplica.
func TestAssigneeDetailKeys(t *testing.T) {
	m := newAssigneeModel(t, 3)
	m.assigneeModalOpen = true
	m.currentView = viewList
	m.assigneeDetail = true

	nombre := m.currentAssignee()
	if nombre == "" {
		t.Fatal("el fixture no ha dejado ninguna persona seleccionada")
	}
	for _, dia := range []string{"2026-10-05", "2026-10-12", "2026-10-19"} {
		mustAddOffDay(t, m.database, nombre, dia, dia, "puente")
	}
	reloadOffDays(t, m)

	offs := m.assigneeOffDays(nombre)
	if len(offs) < 3 {
		t.Fatalf("la persona %q tiene %d días libres, want 3", nombre, len(offs))
	}

	abajo, _ := pulsar(t, m, "j")
	if abajo.assigneeOffdayIdx != 1 {
		t.Errorf("j ha dejado el off-day en %d, want 1", abajo.assigneeOffdayIdx)
	}
	abajo2, _ := pulsar(t, abajo, "j")
	if abajo2.assigneeOffdayIdx != 2 {
		t.Errorf("j desde 1 ha dejado el off-day en %d, want 2", abajo2.assigneeOffdayIdx)
	}
	arriba, _ := pulsar(t, abajo2, "k")
	if arriba.assigneeOffdayIdx != 1 {
		t.Errorf("k ha dejado el off-day en %d, want 1", arriba.assigneeOffdayIdx)
	}

	// "a" abre el alta de un día libre para la persona enfocada.
	alta, cmd := pulsar(t, arriba, "a")
	if !alta.offdayFormOpen {
		t.Error("a no ha abierto el alta de día libre")
	}
	_ = cmd

	// x pide confirmar el borrado, igual que d.
	confirmado, _ := pulsar(t, arriba, "x")
	if !confirmado.confirmOpen || confirmado.confirmAction != "delete-offday" {
		t.Errorf("x no ha pedido confirmar: open=%v action=%q",
			confirmado.confirmOpen, confirmado.confirmAction)
	}

	// esc sale del detalle sin cerrar el modal entero.
	atras, _ := pulsar(t, arriba, "esc")
	if atras.assigneeDetail {
		t.Error("esc no ha salido del detalle de la persona")
	}
	if !atras.assigneeModalOpen {
		t.Error("esc en el detalle ha cerrado el modal entero")
	}
}

// El ciclo de opciones del modal de filtros no puede reportar un índice
// imposible cuando el campo no tiene ninguna. El único campo así es uno que no
// existe, que es exactamente lo que pasa si el número de campos y el switch se
// desincronizan.
func TestFilterCycleWithAnUnknownField(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = 99

	if opts := m.filterVisibleOptions(); len(opts) != 0 {
		t.Fatalf("un campo inexistente tiene %d opciones (%v), want 0", len(opts), opts)
	}

	antes := m.filterOptionIdx
	for _, adelante := range []bool{true, false} {
		m.filterCycle(adelante)
		if m.filterOptionIdx != antes {
			t.Errorf("con un campo sin opciones el índice se ha movido a %d, want %d",
				m.filterOptionIdx, antes)
		}
	}
}

// El nombre de una vista es lo que aparece en la barra y en la ayuda. Una vista
// que no existe tiene que decirlo, no imprimir un entero vacío.
func TestUnknownViewHasAName(t *testing.T) {
	if got := viewKind(42).String(); got != "?" {
		t.Errorf("una vista inexistente se llama %q, want %q", got, "?")
	}
}

// Editar un proyecto con el orden de la lista vacío lo deja como está; con
// orden, lo cambia. Las dos mitades del mismo comando, y la segunda no estaba
// ejecutándose.
func TestProjectEditWithAListOrder(t *testing.T) {
	m := newDashModel(t, "api")
	original := m.projects[0].Workflow

	msg := mustMsg(t, m.saveProjectCmd(true, "api", "api", "", "done,backlog,todo"))
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("el mensaje es %T, want projectSavedMsg", msg)
	}
	if saved.err != nil {
		t.Fatalf("editar con un orden válido ha fallado: %v", saved.err)
	}

	revisado, err := m.database.GetProject("api")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if len(revisado.ListOrder) != 3 {
		t.Errorf("el orden de la lista es %v, want tres estados", revisado.ListOrder)
	}
	// El workflow viene vacío en la llamada, así que no debe haber cambiado.
	if len(revisado.Workflow) != len(original) {
		t.Errorf("el workflow ha pasado de %v a %v con el orden vacío", original, revisado.Workflow)
	}
}

// h en el kanban cambia de columna y reinicia la fila, porque el índice de fila
// es por columna. Quedarse en la columna anterior dejando el índice donde estaba
// haría que la selección apuntara a otra tarea.
func TestKanbanColumnChangeResetsTheRow(t *testing.T) {
	m := newKanbanModel(t, 6)
	m.currentView = viewKanban

	cols := len(m.kanbanColumns())
	if cols < 2 {
		t.Skip("el fixture necesita dos columnas")
	}

	m.kanbanCol = 1
	m.kanbanRow = 2

	izquierda, _ := pulsar(t, m, "h")
	if izquierda.kanbanCol != 0 {
		t.Errorf("h ha dejado la columna en %d, want 0", izquierda.kanbanCol)
	}
	if izquierda.kanbanRow != 0 {
		t.Errorf("h no ha reiniciado la fila: %d, want 0", izquierda.kanbanRow)
	}
}

// La tarea del gantt sólo existe si el cursor cae en una fila de tarea: la
// cabecera de una persona no es una tarea, y una vista sin filas tampoco.
func TestSelectedTaskInTheGantt(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan", "@maria"}, 3)
	m.currentView = viewGantt
	m.width, m.height = 140, 40

	filas := m.ganttRows()
	if len(filas) == 0 {
		t.Fatal("el fixture no ha dejado filas")
	}

	tareaIdx, cabeceraIdx := -1, -1
	for i, f := range filas {
		switch {
		case f.kind == ganttTaskRow && tareaIdx < 0:
			tareaIdx = i
		case f.kind == ganttAssigneeRow && cabeceraIdx < 0:
			cabeceraIdx = i
		}
	}
	if tareaIdx < 0 || cabeceraIdx < 0 {
		t.Fatalf("el fixture no ha dejado los dos tipos de fila: %d filas", len(filas))
	}

	m.ganttCursor = tareaIdx
	tsk := m.selectedTask()
	if tsk == nil {
		t.Fatalf("con el cursor en la fila %d (una tarea) no hay tarea seleccionada", tareaIdx)
	}
	if tsk.ID != filas[tareaIdx].entry.Task.ID {
		t.Errorf("la tarea seleccionada es %d, want %d", tsk.ID, filas[tareaIdx].entry.Task.ID)
	}

	m.ganttCursor = cabeceraIdx
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("con el cursor en una cabecera hay tarea: %q", tsk.Title)
	}

	m.ganttCursor = len(filas) + 5
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("con el cursor fuera de rango hay tarea: %q", tsk.Title)
	}

	// El borde exacto, que es lo que separa `< len(rows)` de `<= len(rows)`: con
	// el cursor en la fila justo posterior a la última no hay nada, aunque sea
	// "casi" un índice válido.
	m.ganttCursor = len(filas)
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("con el cursor en la primera fila de más hay tarea: %q", tsk.Title)
	}

	// Y el otro borde, por debajo.
	m.ganttCursor = -1
	if tsk := m.selectedTask(); tsk != nil {
		t.Errorf("con el cursor en -1 hay tarea: %q", tsk.Title)
	}
}

// La carga de proyectos son dos consultas. Si la de archivados falla, el
// programa arranca igual con la de activos: perder el panel de archivados es
// mejor que no arrancar.
func TestLoadProjectsWithAnUnreadableArchive(t *testing.T) {
	m := newTestModel(t)

	conn := m.database.Conn()
	if _, err := conn.Exec(`DROP TABLE projects`); err != nil {
		t.Fatalf("DROP TABLE projects: %v", err)
	}
	// Una vista con tipos que no se pueden escanear: la consulta de activos
	//选出rá filas, la de archivados fallará al convertirlas.
	if _, err := conn.Exec(`CREATE TABLE projects (
		id BLOB, name BLOB, workflow BLOB, list_order BLOB, archived BLOB)`); err != nil {
		t.Fatalf("CREATE TABLE projects: %v", err)
	}
	for _, archived := range []int{0, 1} {
		if _, err := conn.Exec(
			`INSERT INTO projects VALUES (x'00ff', x'00ff', x'00ff', x'00ff', ?1)`, archived,
		); err != nil {
			t.Fatalf("INSERT INTO projects: %v", err)
		}
	}

	msg := mustMsg(t, m.loadProjects())
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("el mensaje es %T, want projectsLoadedMsg", msg)
	}
	if loaded.archivedProjects != nil {
		t.Errorf("con la tabla de archivados ilegible hay %d archivados, want nil",
			len(loaded.archivedProjects))
	}
}

// La fila 0 de un gantt con filas es siempre una CABECERA de persona, nunca una
// tarea: ganttRows emite la cabecera antes que las entradas de esa persona, y
// no emite la cabecera de nadie que no tenga entradas.
//
// Es lo que hace que selectedTask pueda conformarse con el resto del rango sin
// // y la comparación: con el cursor en 0, la fila 0 no es una tarea, así que la
// comparación con el cero del límite inferior da lo mismo que un "> 0". El test
// fija el invariante para que, si algún día ganttRows deja de cumplirlo, se
// note aquí y no en un mutant que sobrevive sin explicación.
func TestLaFilaCeroDelGanttEsUnaCabecera(t *testing.T) {
	for _, personas := range [][]string{
		{"@juan"},
		{"@juan", "@maria"},
		nil, // sin responsables no hay ni cabecera ni tarea
	} {
		t.Run(strings.Join(personas, "+"), func(t *testing.T) {
			m := ganttModelWithPeople(t, personas, 3)
			m.currentView = viewGantt
			m.width, m.height = 140, 40

			filas := m.ganttRows()
			if len(filas) == 0 {
				if len(personas) > 0 {
					t.Fatal("hay responsables pero no hay filas")
				}
				return
			}
			if filas[0].kind != ganttAssigneeRow {
				t.Errorf("la fila 0 es %v, want una cabecera de persona: de eso depende que "+
					"selectedTask rechace el cursor 0 sin mirar el límite inferior", filas[0].kind)
			}

			// Y con el cursor ahí no hay tarea seleccionada, que es lo mismo que
			// diría un "> 0" en el código.
			m.ganttCursor = 0
			if tsk := m.selectedTask(); tsk != nil {
				t.Errorf("con el cursor en la fila 0 hay tarea: %q", tsk.Title)
			}
		})
	}
}

// nextGanttTaskRow devuelve -1 cuando no hay ninguna fila de tarea a partir del
// punto. Desde el cursor es impracticable -- habría un invariante roto -- pero
// como función pura el caso es alcanzable, y es el que devuelve el valor que
// snapGanttCursor guarda cuando no encuentra nada.
func TestNextGanttTaskRowSinTareas(t *testing.T) {
	casos := []struct {
		nombre string
		rows   []ganttRow
		from   int
	}{
		{"lista vacía", nil, 0},
		{"lista vacía con cursor negativo", nil, -1},
		{"sólo cabeceras", []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttAssigneeRow}}, 0},
		{"cursor más allá del final", []ganttRow{{kind: ganttTaskRow}}, 5},
		{"desde la última fila", []ganttRow{{kind: ganttTaskRow}}, 1},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := nextGanttTaskRow(c.rows, c.from); got != -1 {
				t.Errorf("nextGanttTaskRow(%d filas, from=%d) = %d, want -1", len(c.rows), c.from, got)
			}
		})
	}

	// Y el caso normal: encuentra la siguiente tarea, saltando las cabeceras.
	rows := []ganttRow{
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
	}
	for desde, want := range map[int]int{0: 1, 1: 1, 2: 2, 3: 4} {
		if got := nextGanttTaskRow(rows, desde); got != want {
			t.Errorf("nextGanttTaskRow(from=%d) = %d, want %d", desde, got, want)
		}
	}

	// Y con un cursor negativo busca desde el principio, que es lo que evita un
	// slice con índice negativo.
	if got := nextGanttTaskRow(rows, -3); got != 1 {
		t.Errorf("nextGanttTaskRow(from=-3) = %d, want 1: debe arrancar en la primera fila", got)
	}
}

// stepGanttCursor es la aritmética del salto del gantt, sacada del recorrido para
// que se pueda comprobar con cualquier cursor, incluso los que el recorrido real
// nunca produce (negativos, más allá del final).
//
// Los bordes de esa aritmética son justo lo que el recorrido repartido no dejaba
// ver: el suelo en 0 del clamp, el +1 del offset hacia delante, y la cuenta desde
// el final hacia atrás.
func TestStepGanttCursor(t *testing.T) {
	// Cabecera, tarea, tarea, cabecera, tarea.
	rows := []ganttRow{
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
		{kind: ganttTaskRow},
		{kind: ganttAssigneeRow},
		{kind: ganttTaskRow},
	}

	t.Run("hacia delante", func(t *testing.T) {
		casos := []struct{ desde, want int }{
			{0, 1}, // desde una cabecera
			{1, 2}, // desde una tarea
			{3, 4}, // desde una cabecera con una tarea detrás
			{2, 4}, // salta la cabecera intermedia
		}
		for _, c := range casos {
			got, ok := stepGanttCursor(rows, c.desde, 1)
			if !ok {
				t.Errorf("stepGanttCursor(desde=%d, +1) dice que no hay salto", c.desde)
				continue
			}
			if got != c.want {
				t.Errorf("stepGanttCursor(desde=%d, +1) = %d, want %d", c.desde, got, c.want)
			}
		}
	})

	t.Run("hacia atrás", func(t *testing.T) {
		casos := []struct{ desde, want int }{
			{4, 2},
			{3, 2},
			{2, 1},
		}
		for _, c := range casos {
			got, ok := stepGanttCursor(rows, c.desde, -1)
			if !ok {
				t.Errorf("stepGanttCursor(desde=%d, -1) dice que no hay salto", c.desde)
				continue
			}
			if got != c.want {
				t.Errorf("stepGanttCursor(desde=%d, -1) = %d, want %d", c.desde, got, c.want)
			}
		}
	})

	t.Run("cursores fuera de rango", func(t *testing.T) {
		// El clamp es lo que evita que el slice se corte al revés. Con un cursor
		// negativo, el suelo en 0 y no en -1 es lo que decide.
		for _, desde := range []int{-1, -5, -100} {
			if got, ok := stepGanttCursor(rows, desde, 1); !ok || got != 1 {
				t.Errorf("stepGanttCursor(desde=%d, +1) = (%d, %v), want (1, true): el clamp debe arrancar en 0",
					desde, got, ok)
			}
			if _, ok := stepGanttCursor(rows, desde, -1); ok {
				t.Errorf("stepGanttCursor(desde=%d, -1) dice que hay salto: no hay nada por encima del 0",
					desde)
			}
		}
		// Y con el cursor más allá del final: el techo es len(rows), no
		// len(rows)-1, porque el destino de "hacia delante" es cursor+1.
		for _, desde := range []int{5, 6, 100} {
			if got, ok := stepGanttCursor(rows, desde, 1); ok {
				t.Errorf("stepGanttCursor(desde=%d, +1) = (%d, true), want sin salto", desde, got)
			}
			if got, ok := stepGanttCursor(rows, desde, -1); !ok || got != 4 {
				t.Errorf("stepGanttCursor(desde=%d, -1) = (%d, %v), want (4, true): el techo es la longitud",
					desde, got, ok)
			}
		}
	})

	t.Run("sin nada a donde saltar", func(t *testing.T) {
		soloCabeceras := []ganttRow{{kind: ganttAssigneeRow}, {kind: ganttAssigneeRow}}
		if got, ok := stepGanttCursor(soloCabeceras, 0, 1); ok {
			t.Errorf("sin tareas, +1 = (%d, true), want sin salto", got)
		}
		if got, ok := stepGanttCursor(soloCabeceras, 1, -1); ok {
			t.Errorf("sin tareas, -1 = (%d, true), want sin salto", got)
		}
		if got, ok := stepGanttCursor(nil, 0, 1); ok {
			t.Errorf("lista vacía, +1 = (%d, true), want sin salto", got)
		}
	})

	t.Run("dir cero va hacia delante", func(t *testing.T) {
		// dir == 0 no es "no mover": entra por la rama de "hacia delante" porque
		// no es negativo, y el destino es from+1. Es lo que separa `dir < 0` de
		// `dir <= 0`: con el `<=`, dir cero entraría por la rama de atrás y
		// devolvería -1 en vez del índice de la fila siguiente.
		for _, desde := range []int{0, 1, 2, 3} {
			got, gotOK := stepGanttCursor(rows, desde, 0)
			want, wantOK := stepGanttCursor(rows, desde, 1)
			if got != want || gotOK != wantOK {
				t.Errorf("stepGanttCursor(desde=%d, 0) = (%d, %v), want (%d, %v) (lo mismo que +1)",
					desde, got, gotOK, want, wantOK)
			}
		}
		// Y los casos en los que "hacia delante" no encuentra nada: dir cero
		// tiene que decir lo mismo, que no hay salto.
		if got, ok := stepGanttCursor(rows, 4, 0); ok {
			t.Errorf("stepGanttCursor(desde=4, 0) = (%d, true), want sin salto", got)
		}
		if got, ok := stepGanttCursor(rows, 99, 0); ok {
			t.Errorf("stepGanttCursor(desde=99, 0) = (%d, true), want sin salto", got)
		}
	})
}

// clamp por los tres lados: el suelo, el techo y el medio.
func TestClamp(t *testing.T) {
	casos := []struct{ v, lo, hi, want int }{
		{-5, 0, 10, 0},  // por debajo del suelo
		{0, 0, 10, 0},   // justo el suelo
		{3, 0, 10, 3},   // en medio
		{10, 0, 10, 10}, // justo el techo
		{15, 0, 10, 10}, // por encima del techo
		{-1, -5, 5, -1}, // suelo negativo
		{9, 5, 5, 5},    // lo == hi
		{1, 3, 3, 3},    // lo > v, hi == lo
	}
	for _, c := range casos {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}

// moveGanttCursor con un salto que no existe: el centinela tiene que dejar el
// cursor donde está.
//
// stepGanttCursor devuelve ganttNoTasks cuando no hay fila de tarea hacia donde
// saltar, y moveGanttCursor tiene que distinguir ese -1 de un índice de verdad.
// Con `>= 0` en vez de `!= ganttNoTasks` las dos formas son la misma condición
// sobre enteros, así que su mutante no se podía matar. Con el centinela
// nombrado, la comparación dice lo que compara.
func TestMoveGanttCursorSinSaltoNoMueveElCursor(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan"}, 2)
	m.currentView = viewGantt
	m.width, m.height = 140, 40
	filas := m.ganttRows()

	// Dos casos sin destino: el cursor en la última fila y yendo hacia delante, y
	// el cursor en la primera yendo hacia atrás.
	casos := []struct {
		nombre   string
		posicion int
		dir      int
	}{
		{"primera fila hacia atrás", 0, -1},
		{"última fila hacia delante", len(filas) - 1, 1},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			local := *m
			local.ganttCursor = c.posicion
			// Que de verdad no hay salto.
			if got, ok := stepGanttCursor(filas, c.posicion, c.dir); ok {
				t.Fatalf("el fixture no sirve: stepGanttCursor(%d, %d) = (%d, true), want sin salto",
					c.posicion, c.dir, got)
			}

			moveGanttCursor(&local, filas, c.dir)

			if local.ganttCursor != c.posicion {
				t.Errorf("el cursor se ha movido a %d sin destino; estaba en %d",
					local.ganttCursor, c.posicion)
			}
		})
	}

	// Y el caso normal, para que el test no pase por no hacer nada: con destino sí
	// se mueve, y al sitio exacto que dice stepGanttCursor.
	local := *m
	local.ganttCursor = len(filas) - 2
	want, ok := stepGanttCursor(filas, local.ganttCursor, 1)
	if !ok {
		t.Fatal("el fixture no sirve: hay destino")
	}
	moveGanttCursor(&local, filas, 1)
	if local.ganttCursor != want {
		t.Errorf("el cursor ha ido a %d, want %d", local.ganttCursor, want)
	}
}
