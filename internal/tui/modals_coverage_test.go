package tui

import (
	"strings"
	"testing"
)

// El modal de filtros tiene cinco campos y cada uno tiene su propio eje de
// valores. Lo que no se ejercitaba eran los valores "all" de los campos que no
// son el de proyecto, y los bordes de la aritmética de índices con listas de 0
// y 1 elementos.

func TestFilterFieldOptionsEdges(t *testing.T) {
	t.Run("el estado con un filtro de proyecto que ya no existe", func(t *testing.T) {
		m := newTestModel(t)
		m.filterProject = "proyecto-borrado"

		// Sin el proyecto, el estado sólo puede ofrecer los dos globales: no hay
		// workflow que得要enseñar.
		opts := m.filterFieldOptions(filterFieldStatus)
		if len(opts) != 2 {
			t.Errorf("con un filtro inexistente el estado ofrece %d opciones (%v), want 2",
				len(opts), opts)
		}
	})

	t.Run("un campo que no existe", func(t *testing.T) {
		m := newTestModel(t)
		if opts := m.filterFieldOptions(99); opts != nil {
			t.Errorf("un campo inexistente devuelve %v, want nil", opts)
		}
		if got := filterFieldLabel(99); got != "?" {
			t.Errorf("filterFieldLabel(99) = %q, want %q", got, "?")
		}
	})
}

func TestFilterApplySelectionPerField(t *testing.T) {
	t.Run("assignee", func(t *testing.T) {
		m := newTestModel(t)
		m.filterAssignee = "@juan"

		m.filterApplySelection(filterFieldAssignee, "all")
		if m.filterAssignee != "" {
			t.Errorf("con \"all\" el filtro de assignee es %q, want vacío", m.filterAssignee)
		}

		m.filterApplySelection(filterFieldAssignee, "@maria")
		if m.filterAssignee != "@maria" {
			t.Errorf("el filtro de assignee es %q, want @maria", m.filterAssignee)
		}
	})

	t.Run("status", func(t *testing.T) {
		m := newTestModel(t)
		m.filterStatus = "doing"

		m.filterApplySelection(filterFieldStatus, "all")
		if m.filterStatus != "" {
			t.Errorf("con \"all\" el filtro de estado es %q, want vacío", m.filterStatus)
		}

		m.filterApplySelection(filterFieldStatus, "todo")
		if m.filterStatus != "todo" {
			t.Errorf("el filtro de estado es %q, want todo", m.filterStatus)
		}
	})

	t.Run("tag", func(t *testing.T) {
		m := newTestModel(t)
		m.filterTag = "bloqueado"

		m.filterApplySelection(filterFieldTag, "all")
		if m.filterTag != "" {
			t.Errorf("con \"all\" el filtro de tag es %q, want vacío", m.filterTag)
		}

		m.filterApplySelection(filterFieldTag, "urgente")
		if m.filterTag != "urgente" {
			t.Errorf("el filtro de tag es %q, want urgente", m.filterTag)
		}
	})

	// Los tres valores que no son "med"/"high", que es lo único que se pulsaba.
	t.Run("prioridad: all, none y low", func(t *testing.T) {
		for _, tc := range []struct {
			valor string
			want  int
		}{
			{"all", -1},
			{"none", 0},
			{"low", 1},
			{"med", 2},
			{"high", 3},
		} {
			m := newTestModel(t)
			m.filterApplySelection(filterFieldPriority, tc.valor)
			if m.filterPriority != tc.want {
				t.Errorf("prioridad %q ha dejado filterPriority=%d, want %d",
					tc.valor, m.filterPriority, tc.want)
			}
			// Y el valor legible que se muestra de vuelta en el modal.
			if got := m.filterCurrentValue(filterFieldPriority); got != tc.valor {
				t.Errorf("filterCurrentValue tras %q es %q", tc.valor, got)
			}
		}
	})

	t.Run("proyecto", func(t *testing.T) {
		m := newTestModel(t)
		m.filterProject = "api"

		m.filterApplySelection(filterFieldProject, "all")
		if m.filterProject != "" {
			t.Errorf("con \"all\" el filtro de proyecto es %q, want vacío", m.filterProject)
		}

		m.filterApplySelection(filterFieldProject, "web")
		if m.filterProject != "web" {
			t.Errorf("el filtro de proyecto es %q, want web", m.filterProject)
		}
	})
}

// Con un solo proyecto no hay a dónde ciclar el filtro de proyecto: "all" y el
// propio proyecto ya cubren todo. Lo que hay que comprobar es que no se queda
// en un índice imposible.
func TestCycleProjectFilterWithNoProjects(t *testing.T) {
	m := newTestModel(t)
	m.projects = nil

	if opts := m.filterFieldOptions(filterFieldProject); len(opts) != 1 {
		t.Fatalf("sin proyectos hay %d opciones (%v), want 1", len(opts), opts)
	}

	antes := m.filterProject
	for range 3 {
		m.cycleProjectFilter(1)
		m.cycleProjectFilter(-1)
	}
	if m.filterProject != antes {
		t.Errorf("el filtro ha pasado de %q a %q sin proyectos", antes, m.filterProject)
	}
}

// El ciclo en vivo de opciones no puede日报道 nada si no hay opciones: el
// índice se queda donde estaba en vez de moverse a -1.
func TestFilterCycleWithNoOptions(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = filterFieldTag
	m.filterSearch = "no-existe-nada"

	antes := m.filterOptionIdx
	m.filterCycle(true)
	if m.filterOptionIdx != antes {
		t.Errorf("el índice se ha movido a %d sin opciones, want %d", m.filterOptionIdx, antes)
	}
	if !m.filterActive && m.filterTag != "" {
		t.Errorf("el ciclo sin opciones ha dejado un filtro colgado: %q", m.filterTag)
	}
}

func TestFilterModalKeys(t *testing.T) {
	t.Run("shift+tab retrocede de campo", func(t *testing.T) {
		m := newTestModel(t)
		m.filterOpen = true
		m.filterFieldIdx = 0

		siguiente, _ := pulsar(t, m, "shift+tab")
		// Con cinco campos, desde el primero el shift+tab da la vuelta al último.
		want := filterFieldCount - 1
		if siguiente.filterFieldIdx != want {
			t.Errorf("shift+tab desde 0 ha dejado el campo en %d, want %d",
				siguiente.filterFieldIdx, want)
		}

		// Y tab desde el último vuelve al primero.
		siguiente, _ = pulsar(t, siguiente, "tab")
		if siguiente.filterFieldIdx != 0 {
			t.Errorf("tab desde el último ha dejado el campo en %d, want 0", siguiente.filterFieldIdx)
		}
	})

	t.Run("backspace recorta la búsqueda y al vacío no hace nada", func(t *testing.T) {
		m := newTestModel(t)
		m.filterOpen = true
		m.filterSearch = "api"
		m.filterOptionIdx = 2

		siguiente, _ := pulsar(t, m, "backspace")
		if siguiente.filterSearch != "ap" {
			t.Errorf("la búsqueda es %q, want ap", siguiente.filterSearch)
		}
		if siguiente.filterOptionIdx != 0 {
			t.Errorf("backspace no ha vuelto al principio de la lista: %d", siguiente.filterOptionIdx)
		}

		siguiente.filterSearch = ""
		otro, _ := pulsar(t, siguiente, "backspace")
		if otro.filterSearch != "" {
			t.Errorf("backspace con la búsqueda vacía ha dejado %q", otro.filterSearch)
		}
	})
}

// El modal de personas tiene dos niveles: la lista y el detalle de una persona.
// Los dos tienen su juego de teclas, y el del detalle sólo actúa si hay una
// persona seleccionada con días libres.
func TestAssigneeModalKeys(t *testing.T) {
	t.Run("navegar la lista y cerrarla", func(t *testing.T) {
		m := newAssigneeModel(t, 3)
		m.assigneeModalOpen = true
		m.currentView = viewList

		if len(m.assigneeRoster()) < 2 {
			t.Skip("el fixture necesita dos personas en el roster")
		}

		abajo, _ := pulsar(t, m, "j")
		if abajo.assigneeIdx == m.assigneeIdx {
			t.Error("j no ha movido el índice de persona")
		}
		arriba, _ := pulsar(t, abajo, "k")
		if arriba.assigneeIdx != m.assigneeIdx {
			t.Errorf("k no ha vuelto al índice %d: está en %d", m.assigneeIdx, arriba.assigneeIdx)
		}

		cerrado, _ := pulsar(t, arriba, "esc")
		if cerrado.assigneeModalOpen {
			t.Error("esc no ha cerrado el modal de personas")
		}
	})

	t.Run("el detalle de una persona", func(t *testing.T) {
		m := newAssigneeModel(t, 3)
		m.assigneeModalOpen = true
		m.currentView = viewList
		m.assigneeDetail = true

		offs := m.assigneeOffDays(m.currentAssignee())
		if len(offs) < 2 {
			t.Skip("el fixture necesita dos días libres para la persona seleccionada")
		}

		abajo, _ := pulsar(t, m, "j")
		if abajo.assigneeOffdayIdx != 1 {
			t.Errorf("j ha dejado el off-day en %d, want 1", abajo.assigneeOffdayIdx)
		}
		arriba, _ := pulsar(t, abajo, "k")
		if arriba.assigneeOffdayIdx != 0 {
			t.Errorf("k ha dejado el off-day en %d, want 0", arriba.assigneeOffdayIdx)
		}

		// d pide confirmar el borrado del off-day enfocado.
		confirmado, _ := pulsar(t, arriba, "d")
		if !confirmado.confirmOpen || confirmado.confirmAction != "delete-offday" {
			t.Errorf("d no ha pedido confirmar el borrado: open=%v action=%q",
				confirmado.confirmOpen, confirmado.confirmAction)
		}
		if confirmado.confirmOffday.ID == 0 {
			t.Error("el off-day pendiente no se ha copiado a la confirmación")
		}

		// a abre el alta de un día libre.
		alta, cmd := pulsar(t, arriba, "a")
		if !alta.offdayFormOpen {
			t.Error("a no ha abierto el alta de día libre")
		}
		_ = cmd

		// esc vuelve al nivel de lista, no cierra el modal entero.
		atras, _ := pulsar(t, arriba, "esc")
		if atras.assigneeDetail {
			t.Error("esc no ha salido del detalle de la persona")
		}
		if !atras.assigneeModalOpen {
			t.Error("esc en el detalle ha cerrado el modal entero")
		}
	})

}

// El resultado de borrar un off-day tiene su propio texto, y el "Done" por
// defecto es la rama que se ejecutaba cuando el mensaje no traía acción
// conocida.
func TestOffDaySavedMessage(t *testing.T) {
	m := newAssigneeModel(t, 2)

	conNombre, _ := updateMsg(t, m, offdaySavedMsg{action: "delete", name: "@juan"})
	if !strings.Contains(conNombre.toast, "@juan") {
		t.Errorf("el aviso no nombra a la persona: %q", conNombre.toast)
	}
	if conNombre.toastKind != "info" {
		t.Errorf("el aviso es de tipo %q, want info", conNombre.toastKind)
	}

	defecto, _ := updateMsg(t, m, offdaySavedMsg{action: "lo-que-sea"})
	if defecto.toast != "Done" {
		t.Errorf("el aviso por defecto es %q, want Done", defecto.toast)
	}

	// Un alta fallida reabre el formulario para no perder lo escrito; una baja
	// fallida no, porque no había nada escrito.
	alta, _ := updateMsg(t, m, offdaySavedMsg{err: errAccionFallida, action: "add"})
	if !alta.offdayFormOpen {
		t.Error("un alta fallida no ha reabierto el formulario")
	}
	baja, _ := updateMsg(t, m, offdaySavedMsg{err: errAccionFallida, action: "delete"})
	if baja.offdayFormOpen {
		t.Error("una baja fallida ha reabierto el formulario de alta")
	}
	if baja.toastKind != "error" {
		t.Errorf("el aviso de error es de tipo %q, want error", baja.toastKind)
	}
}

// El modal de tags ofrece sugerencias y navega entre ellas con las flechas.
// Sin sugerencias, el índice no se mueve: es la diferencia entre "no hay nada
// que elegir" y "eligiendo la primera".
func TestTagModalSuggestionNavigation(t *testing.T) {
	t.Run("con sugerencias", func(t *testing.T) {
		m := newTestModel(t)
		m.tasks[0].Tags = []string{"alfa", "beta", "gamma"}
		m.tagOpen = true
		m.detailTask = &m.tasks[0]

		sug := m.tagSuggestions()
		if len(sug) < 2 {
			t.Fatalf("el fixture necesita dos sugerencias, tiene %v", sug)
		}

		abajo, _ := pulsar(t, m, "down")
		if abajo.tagSuggestIdx != 0 {
			t.Errorf("down ha dejado la sugerencia en %d, want 0", abajo.tagSuggestIdx)
		}
		arriba, _ := pulsar(t, abajo, "up")
		// up desde 0 cicla al último, no se queda en -1.
		if arriba.tagSuggestIdx != len(sug)-1 {
			t.Errorf("up desde 0 ha dejado la sugerencia en %d, want %d",
				arriba.tagSuggestIdx, len(sug)-1)
		}
	})

	t.Run("sin sugerencias", func(t *testing.T) {
		m := newTestModel(t)
		m.tagOpen = true
		m.detailTask = &m.tasks[0]

		siguiente, _ := pulsar(t, m, "down")
		if siguiente.tagSuggestIdx != -1 {
			t.Errorf("sin sugerencias down ha dejado el índice en %d, want -1", siguiente.tagSuggestIdx)
		}
	})
}

// Enter con el input vacío elige la sugerencia; si no hay sugerencia ni tarea,
// no hace nada y el modal sigue abierto para no perder lo escrito.
func TestTagModalEnterWithoutSuggestions(t *testing.T) {
	m := newTestModel(t)
	m.tagOpen = true
	m.detailTask = &m.tasks[0]
	m.tagInput = "   "
	m.tagSuggestIdx = -1

	siguiente, cmd := pulsar(t, m, "enter")
	if cmd != nil {
		t.Error("enter sin sugerencia ha lanzado un toggle")
	}
	if !siguiente.tagOpen {
		t.Error("enter sin sugerencia ha cerrado el modal")
	}
}

// La navegación del gantt salta de fila de tarea a fila de tarea, saltándose
// las cabeceras de persona. Para comprobar que eso siempre funciona hay que
// verificar el invariante del que depende: toda cabecera va inmediatamente
// seguida de una fila de tarea, así que buscar hacia delante desde una cabecera
// siempre encuentra algo.
//
// Este test es el que sostiene la afirmación del comentario de snapGanttCursor.
// Si dejara de ser cierta, la búsqueda hacia atrás dejaría de ser inalcanzable
// y esto fallaría antes de que nadie lo notara en la pantalla.
func TestEveryGanttAssigneeHeaderIsFollowedByATask(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan", "@maria", "@nico"}, 3)
	m.currentView = viewGantt
	m.width, m.height = 140, 40

	filas := m.ganttRows()
	cabeceras := 0
	for i, f := range filas {
		if f.kind != ganttAssigneeRow {
			continue
		}
		cabeceras++
		if i+1 >= len(filas) || filas[i+1].kind != ganttTaskRow {
			t.Fatalf("la cabecera %d (%s) no va seguida de una tarea: %+v",
				i, f.assignee, filas[min(i+1, len(filas)-1)])
		}
	}
	if cabeceras == 0 {
		t.Fatal("el fixture no ha dejado ninguna cabecera de persona")
	}

	// Y con ese invariante, apoyar el cursor en cualquier cabecera acaba en una
	// tarea, nunca en un índice imposible.
	for i, f := range filas {
		if f.kind != ganttAssigneeRow {
			continue
		}
		m.ganttCursor = i
		m.snapGanttCursor()
		if filas[m.ganttCursor].kind != ganttTaskRow {
			t.Errorf("con el cursor en la cabecera %d el gantt se ha quedado en una fila %v, want una tarea",
				i, filas[m.ganttCursor].kind)
		}
	}
}
