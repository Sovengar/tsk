package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// El modo edición del modal de proyecto no tenía ni un test: el camino entero --
// precargar los campos, parsear el workflow, actualizar y avisar -- estaba sin
// ejecutar. Es el camino que se usa al renombrar un proyecto, así que no es un
// hueco menor.

func TestProjectModalEditRoundTrip(t *testing.T) {
	m := newDashModel(t, "api")
	m.currentView = viewDashboard

	p := m.selectedDashProject()
	if p == nil {
		t.Fatal("el fixture no ha dejado ningún proyecto seleccionado")
	}
	original := p.Name

	abierto, _ := pulsar(t, m, "e")
	if !abierto.projectModalEdit || abierto.projectEditingName != original {
		t.Fatalf("el modal no se ha abierto en edición: edit=%v original=%q",
			abierto.projectModalEdit, abierto.projectEditingName)
	}
	if abierto.projectNameInput != original {
		t.Errorf("el nombre no se ha precargado: %q", abierto.projectNameInput)
	}
	if abierto.projectWorkflowInput != strings.Join(p.Workflow, ",") {
		t.Errorf("el workflow no se ha precargado: %q", abierto.projectWorkflowInput)
	}
	if abierto.projectListOrderInput != strings.Join(p.ListOrder, ",") {
		t.Errorf("el orden de la lista no se ha precargado: %q", abierto.projectListOrderInput)
	}

	// Borrar el nombre entero y escribir otro, que es lo que hace el usuario.
	// Cuatro backspaces borran el nombre entero (5 letras del fixture), que es
	// el camino real para renombrar.
	renombrado := abierto
	renombrado.projectNameInput = ""
	renombrado, _ = pulsar(t, abierto, "backspace", "backspace", "backspace",
		"backspace", "backspace", "backspace")
	for _, k := range []string{"z", "e", "t", "a"} {
		renombrado, _ = pulsar(t, renombrado, k)
	}
	if renombrado.projectNameInput != "zeta" {
		t.Fatalf("la escritura a mano ha dejado %q, want zeta", renombrado.projectNameInput)
	}

	guardado, cmd := pulsar(t, renombrado, "enter")
	if guardado.projectModalOpen {
		t.Error("enter no ha cerrado el modal")
	}
	if cmd == nil {
		t.Fatal("enter no ha lanzado el guardado")
	}

	msg := mustMsg(t, cmd)
	guardadoMsg, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("el mensaje es %T, want projectSavedMsg", msg)
	}
	if guardadoMsg.err != nil {
		t.Fatalf("el guardado ha fallado: %v", guardadoMsg.err)
	}
	if guardadoMsg.action != "edit" {
		t.Errorf("la acción es %q, want edit", guardadoMsg.action)
	}
	if guardadoMsg.name != "zeta" {
		t.Errorf("el nombre guardado es %q, want zeta", guardadoMsg.name)
	}

	// Y el proyecto existe con el nombre nuevo y con el workflow intacto, que es
	// lo que significa "vacío = mantener el actual".
	revisado, err := m.database.GetProject("zeta")
	if err != nil {
		t.Fatalf("GetProject(zeta): %v", err)
	}
	if len(revisado.Workflow) != len(p.Workflow) {
		t.Errorf("el workflow ha pasado de %v a %v", p.Workflow, revisado.Workflow)
	}

	aplicado, _ := updateMsg(t, guardado, msg)
	if !strings.Contains(ansi.Strip(aplicado.View().Content), "updated") {
		t.Errorf("no se avisa de la actualización:\n%s", ansi.Strip(aplicado.View().Content))
	}
	if aplicado.pendingSelectName != "zeta" {
		t.Errorf("pendingSelectName = %q, want zeta para que el dashboard lo seleccione", aplicado.pendingSelectName)
	}
}

// Renombrar a un nombre que ya existe es el error más probable al editar, y el
// modal se reabre para no perder lo escrito.
func TestProjectModalEditRejectsADuplicateName(t *testing.T) {
	m := newDashModel(t, "api")
	m.currentView = viewDashboard

	otro := ""
	for _, p := range m.projects {
		if p.Name != "api" {
			otro = p.Name
			break
		}
	}
	if otro == "" {
		t.Skip("el fixture necesita un segundo proyecto")
	}

	msg := mustMsg(t, m.saveProjectCmd(true, "api", otro, "", ""))
	saved, ok := msg.(projectSavedMsg)
	if !ok {
		t.Fatalf("el mensaje es %T, want projectSavedMsg", msg)
	}
	if saved.err == nil {
		t.Fatal("renombrar a un nombre duplicado no ha fallado")
	}
	if saved.action != "edit" {
		t.Errorf("la acción es %q, want edit", saved.action)
	}

	reabierto, _ := updateMsg(t, m, msg)
	if !reabierto.projectModalOpen {
		t.Error("el modal no se ha reabierto tras el error, want reopen para no perder lo escrito")
	}
}

// El workflow del modal es texto libre separado por comas, así que una lista de
// comas es un error de parseo, no un workflow vacío.
func TestProjectModalWorkflowParseErrors(t *testing.T) {
	m := newDashModel(t, "api")

	for _, tc := range []struct {
		nombre    string
		workflow  string
		listOrder string
	}{
		{"workflow vacío", ", ,", ""},
		{"orden de lista vacío", "backlog,done", " , "},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			msg := mustMsg(t, m.saveProjectCmd(true, "api", "api", tc.workflow, tc.listOrder))
			saved, ok := msg.(projectSavedMsg)
			if !ok {
				t.Fatalf("el mensaje es %T, want projectSavedMsg", msg)
			}
			if saved.err == nil {
				t.Fatalf("%q no ha producido error", tc.nombre)
			}
			if saved.action != "edit" {
				t.Errorf("la acción es %q, want edit", saved.action)
			}
		})
	}

	t.Run("crear", func(t *testing.T) {
		for _, tc := range []struct{ workflow, listOrder string }{
			{", ,", ""},
			{"", ", ,"},
		} {
			msg := mustMsg(t, m.saveProjectCmd(false, "", "nuevo", tc.workflow, tc.listOrder))
			saved := msg.(projectSavedMsg)
			if saved.err == nil {
				t.Errorf("crear con workflow=%q listOrder=%q no ha fallado", tc.workflow, tc.listOrder)
			}
			if saved.action != "create" {
				t.Errorf("la acción es %q, want create", saved.action)
			}
		}
	})

	t.Run("crear con workflow y orden válidos", func(t *testing.T) {
		msg := mustMsg(t, m.saveProjectCmd(false, "", "nuevo", "backlog,done", "done,backlog"))
		saved := msg.(projectSavedMsg)
		if saved.err != nil {
			t.Fatalf("crear con valores válidos ha fallado: %v", saved.err)
		}

		creado, err := m.database.GetProject("nuevo")
		if err != nil {
			t.Fatalf("GetProjectByName: %v", err)
		}
		if len(creado.Workflow) != 2 || len(creado.ListOrder) != 2 {
			t.Errorf("el proyecto creado tiene workflow=%v listOrder=%v",
				creado.Workflow, creado.ListOrder)
		}
	})
}

// Los cinco campos de texto del modal comparten el mismo editor de una sola
// línea. Los bordes: buffer vacío, el espacio que no debe entrar en los campos
// de lista, y el puntero nil que la interfaz nunca produce pero la función sí
// tiene que tolerar.
func TestEditTextInput(t *testing.T) {
	t.Run("backspace sobre un buffer vacío no hace nada", func(t *testing.T) {
		s := ""
		editTextInput(&s, "backspace")
		if s != "" {
			t.Errorf("el buffer es %q, want vacío", s)
		}
	})

	t.Run("backspace quita un carácter", func(t *testing.T) {
		s := "abc"
		editTextInput(&s, "backspace")
		if s != "ab" {
			t.Errorf("el buffer es %q, want ab", s)
		}
	})

	t.Run("el espacio entra con su nombre", func(t *testing.T) {
		s := ""
		editTextInput(&s, "space")
		if s != " " {
			t.Errorf("el buffer es %q, want un espacio", s)
		}
	})

	// Bubbletea entrega el espacio con nombre, así que la rama del carácter
	// suelto no se ejercita desde la interfaz. Se prueba aquí porque es código
	// vivo y su comportamiento (meter un espacio) es parte del contrato.
	t.Run("el espacio suelto también entra", func(t *testing.T) {
		s := ""
		editTextInput(&s, " ")
		if s != " " {
			t.Errorf("el buffer es %q, want un espacio", s)
		}
	})

	t.Run("un puntero nil no revienta", func(t *testing.T) {
		editTextInput(nil, "a")
		editTextInput(nil, "backspace")
	})

	t.Run("el modal con un índice de campo imposible no revienta", func(t *testing.T) {
		m := newProjectModel(t, 3)
		m.projectModalField = 7

		siguiente, _ := press(m, "a")
		if siguiente.projectNameInput != m.projectNameInput {
			t.Error("una tecla ha escrito en un campo con índice fuera de rango")
		}
	})
}

// El modal de confirmación es un switch sobre la acción pendiente, y cada rama
// tiene su propio caso degenerado: borrar un off-day que no existe, archivar
// sin proyecto, y una acción que no es ninguna de las conocidas.
func TestConfirmModalEdges(t *testing.T) {
	t.Run("un off-day que no existe", func(t *testing.T) {
		m := newTestModel(t)
		m.confirmOpen = true
		m.confirmAction = "delete-offday"
		m.confirmOffday = model.OffDay{}

		siguiente, cmd := pulsar(t, m, "y")
		if cmd != nil {
			t.Error("borrar un off-day inexistente ha lanzado un comando")
		}
		if siguiente.confirmOpen {
			t.Error("el modal sigue abierto")
		}
	})

	t.Run("una acción de proyecto sin nombre", func(t *testing.T) {
		for _, accion := range []string{"archive", "unarchive"} {
			m := newTestModel(t)
			m.confirmOpen = true
			m.confirmAction = accion
			m.confirmProject = ""

			if siguiente, cmd := pulsar(t, m, "y"); cmd != nil {
				t.Errorf("%s sin nombre de proyecto ha lanzado un comando", accion)
			} else if siguiente.confirmOpen {
				t.Errorf("%s sin nombre de proyecto ha dejado el modal abierto", accion)
			}
		}
	})

	t.Run("una acción desconocida", func(t *testing.T) {
		m := newTestModel(t)
		m.confirmOpen = true
		m.confirmAction = "lo-que-sea"

		siguiente, cmd := pulsar(t, m, "y")
		if cmd != nil {
			t.Error("una acción desconocida ha lanzado un comando")
		}
		if siguiente.confirmOpen {
			t.Error("una acción desconocida ha dejado el modal abierto")
		}
	})

	t.Run("cada forma de decir no limpia el estado", func(t *testing.T) {
		for _, k := range []string{"n", "N", "esc", "q"} {
			m := newTestModel(t)
			m.confirmOpen = true
			m.confirmAction = "archive"
			m.confirmProject = "api"
			m.confirmOffday = model.OffDay{ID: 7, Assignee: "@juan"}

			siguiente, cmd := pulsar(t, m, k)
			if cmd != nil {
				t.Errorf("%q ha lanzado un comando de acción", k)
			}
			if siguiente.confirmOpen || siguiente.confirmAction != "" || siguiente.confirmProject != "" {
				t.Errorf("%q no ha limpiado el estado de confirmación", k)
			}
			if siguiente.confirmOffday.ID != 0 {
				t.Errorf("%q no ha limpiado el off-day pendiente", k)
			}
		}
	})

	t.Run("y también acepta enter", func(t *testing.T) {
		m := newDashModel(t, "api")
		m.currentView = viewDashboard
		m.confirmOpen = true
		m.confirmAction = "archive"
		m.confirmProject = "api"

		_, cmd := pulsar(t, m, "enter")
		if cmd == nil {
			t.Fatal("enter no ha confirmado la acción")
		}
		msg := mustMsg(t, cmd)
		saved, ok := msg.(projectSavedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want projectSavedMsg", msg)
		}
		if saved.err != nil {
			t.Fatalf("archivar ha fallado: %v", saved.err)
		}
		// GetProject no filtra por archivado: hay que mirar la lista de activos.
		activos, err := m.database.ListProjects()
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		for _, p := range activos {
			if p.Name == "api" {
				t.Error("el proyecto sigue en la lista de activos tras archivarlo")
			}
		}
	})
}

// El resultado de una operación de proyecto decide el texto del aviso y si el
// modal se reabre. Los cuatro textos y el "Done" por defecto son ramas
// distintas del mismo switch.
func TestProjectSavedMessages(t *testing.T) {
	for _, tc := range []struct {
		accion    string
		nombre    string
		want      string
		seleccion bool
	}{
		{"create", "nuevo", `Project "nuevo" created`, true},
		{"edit", "nuevo", `Project "nuevo" updated`, true},
		{"archive", "api", `Project "api" archived`, false},
		{"unarchive", "api", `Project "api" restored`, true},
		{"cualquier-otra", "sin-relevancia", "Done", false},
	} {
		t.Run(tc.accion, func(t *testing.T) {
			m := newDashModel(t, "api")
			m.width, m.height = 100, 30

			siguiente, _ := updateMsg(t, m, projectSavedMsg{name: tc.nombre, action: tc.accion})
			out := ansi.Strip(siguiente.View().Content)
			if !strings.Contains(out, tc.want) {
				t.Errorf("el aviso no dice %q:\n%s", tc.want, out)
			}
			if (siguiente.pendingSelectName == tc.nombre) != tc.seleccion {
				t.Errorf("pendingSelectName = %q, la selección %v", siguiente.pendingSelectName, tc.seleccion)
			}
		})
	}

	t.Run("un error de archivado no reabre el modal", func(t *testing.T) {
		m := newDashModel(t, "api")
		siguiente, _ := updateMsg(t, m, projectSavedMsg{err: errAccionFallida, action: "archive"})
		if siguiente.projectModalOpen {
			t.Error("un archivado fallido ha reabierto el modal de proyecto")
		}
	})
}

// El título del modal distingue creación de edición, y el texto de la pista
// cambia con él: en crear, vacío significa "usa el default"; en editar, vacío
// significa "no toques lo que hay".
func TestProjectModalTitleFollowsTheMode(t *testing.T) {
	crear := newProjectModel(t, 0)
	crear.projectModalEdit = false
	if out := ansi.Strip(crear.renderProjectModal("")); !strings.Contains(out, "New Project") {
		t.Errorf("el modal de creación no dice New Project:\n%s", out)
	} else if !strings.Contains(out, "empty = default") {
		t.Errorf("el modal de creación no avisa de que vacío = default:\n%s", out)
	}

	editar := *crear
	editar.projectModalEdit = true
	out := ansi.Strip(editar.renderProjectModal(""))
	if !strings.Contains(out, "Edit Project") {
		t.Errorf("el modal de edición no dice Edit Project:\n%s", out)
	}
	if !strings.Contains(out, "keep current") {
		t.Errorf("el modal de edición no avisa de que vacío = mantener:\n%s", out)
	}
}
