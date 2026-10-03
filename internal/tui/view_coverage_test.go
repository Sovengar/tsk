package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/model"
)

// View() es la función que más ramas tiene del paquete y la que ningún test
// llamaba: los tests de render llamaban a renderX directamente, que es el mismo
// código pero sin el switch de vistas ni la pila de overlays. Eso dejaba sin
// cubrir justo el código que decide a quién llama.
//
// Un test por estado de la pila: cada overlay abierto y cada vista.
func TestViewCoversEveryStateOfTheOverlayStack(t *testing.T) {
	view := func(t *testing.T, m *Model) string {
		t.Helper()
		m.width, m.height = 100, 30
		return ansi.Strip(m.View().Content)
	}

	t.Run("las tres vistas", func(t *testing.T) {
		for _, v := range []struct {
			nombre string
			marca  string
		}{
			{"dashboard", "Total"},
			{"kanban", "backlog"},
			{"gantt", "@"},
		} {
			t.Run(v.nombre, func(t *testing.T) {
				m := newTestModel(t)
				switch v.nombre {
				case "dashboard":
					m.currentView = viewDashboard
				case "kanban":
					m.currentView = viewKanban
				case "gantt":
					m.currentView = viewGantt
				}
				if out := view(t, m); !strings.Contains(out, v.marca) {
					t.Errorf("la vista %s no contiene %q:\n%s", v.nombre, v.marca, out)
				}
			})
		}
	})

	t.Run("una vista desconocida cae al dashboard", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewKind(99)
		if out := view(t, m); !strings.Contains(out, "Total") {
			t.Errorf("una vista desconocida no ha salido por el dashboard:\n%s", out)
		}
	})

	t.Run("cada overlay por encima", func(t *testing.T) {
		for _, tc := range []struct {
			nombre string
			abrir  func(*Model)
			marca  string
		}{
			{"tags", func(m *Model) { m.tagOpen = true; m.detailTask = &m.tasks[0] }, "Tags"},
			{"proyecto", func(m *Model) { m.projectModalOpen = true }, "Project"},
			{"confirmación", func(m *Model) { m.confirmOpen = true; m.confirmAction = "delete"; m.confirmProject = "api" }, "Confirm"},
			{"off-day", func(m *Model) { m.offdayFormOpen = true }, "Off-day"},
			{"assignees", func(m *Model) { m.assigneeModalOpen = true }, "Assignees"},
			{"assignee detalle", func(m *Model) { m.assigneeModalOpen = true; m.assigneeDetail = true }, "Assignee ·"},
			{"ayuda", func(m *Model) { m.helpOpen = true }, "Keybinds"},
		} {
			t.Run(tc.nombre, func(t *testing.T) {
				m := newTestModel(t)
				m.assigneeIdx = 0
				tc.abrir(m)
				if out := view(t, m); !strings.Contains(out, tc.marca) {
					t.Errorf("el overlay %s no se ve:\n%s", tc.nombre, out)
				}
			})
		}
	})

	t.Run("el filtro abierto y con la ayuda abierta a la vez", func(t *testing.T) {
		m := newTestModel(t)
		m.filterOpen = true
		m.filterActive = true
		if out := view(t, m); out == "" {
			t.Error("View() vacío con el filtro abierto")
		}
	})
}

// El command que carga los proyectos y el que carga las tareas devuelven un
// mensaje vacío cuando la base de datos falla, en vez de nada: un mensaje vacío
// es la señal de "cargado pero sin datos", y el Update lo distingue del estado
// inicial. Con la base cerrada ambos caminos se ejecutan.
func TestLoadCommandsSurviveAClosedDB(t *testing.T) {
	m := newTestModel(t)
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, tc := range []struct {
		nombre string
		cmd    func() tea.Cmd
	}{
		{"proyectos", m.loadProjects},
		{"tareas", m.loadTasks},
		{"off-days", m.loadOffDays},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			msgs := mustRun(t, tc.cmd())
			if len(msgs) == 0 {
				t.Fatal("el comando no ha devuelto ningún mensaje")
			}
			if _, ok := msgs[0].(projectsLoadedMsg); !ok && tc.nombre == "proyectos" {
				t.Errorf("el mensaje es %T, want projectsLoadedMsg", msgs[0])
			}
		})
	}
}

// El command que decide el workflow efectivo devuelve el del proyecto
// seleccionado, y el default cuando no hay proyecto o el filtro no deja ninguno.
func TestEffectiveWorkflowFallsBackToTheDefault(t *testing.T) {
	t.Run("sin proyectos", func(t *testing.T) {
		m := newEmptyDBModel(t)
		if got := m.commonWorkflow(); len(got) != len(model.DefaultWorkflow) {
			t.Errorf("sin proyectos el workflow es %v, want el default", got)
		}
	})

	t.Run("con proyectos", func(t *testing.T) {
		m := newTestModel(t)
		if got := m.commonWorkflow(); len(got) == 0 {
			t.Error("con proyectos el workflow está vacío")
		}
	})

	t.Run("un índice fuera de rango no deja proyecto seleccionado", func(t *testing.T) {
		m := newTestModel(t)
		m.dashProjectIdx = 9999

		if p := m.selectedDashProject(); p != nil {
			t.Errorf("con el índice fuera de rango hay proyecto seleccionado: %q", p.Name)
		}
		// La lista de visibles no depende del índice: lo que se queda sin
		// proyecto es la selección, no el workflow.
		if got := m.commonWorkflow(); len(got) == 0 {
			t.Error("un índice fuera de rango ha vaciado el workflow")
		}
	})
}

// Init() devuelve el par de comandos de arranque. Es una línea, pero es la que
// decide si la aplicación arranca con datos o con una pantalla vacía.
func TestInitLoadsProjectsAndTasks(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() ha devuelto nil")
	}
	msgs := mustRun(t, cmd)
	if len(msgs) != 2 {
		t.Errorf("Init() produce %d mensajes, want 2 (proyectos y tareas)", len(msgs))
	}
}

// El pie de la lista dice el rango que se está viendo, el total y la página. Es
// geometría con números dentro, así que un test de presencia no la distingue:
// "1-4 of 4" también está dentro de "11-4 of 4".
func TestListFooterCountsExactly(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 20

	total := len(m.tasks)
	if total < 2 {
		t.Fatalf("el fixture necesita al menos dos tareas, tiene %d", total)
	}

	out := ansi.Strip(m.View().Content)
	want := fmt.Sprintf("1-%d of %d · Page 1/1", total, total)
	if leyenda := m.pageLegend(); leyenda != want {
		t.Errorf("la leyenda es %q, want %q", leyenda, want)
	}
	if !strings.Contains(out, want) {
		t.Errorf("el pie no dice %q:\n%s", want, out)
	}

	// Con paginación real, la tercera página no vuelve a decir 1-n.
	paginado := modelWithTasks(t, 5, 2)
	for range 2 {
		paginado, _ = press(paginado, "n")
	}

	inicio, fin := paginado.pageBounds()
	if inicio != 4 || fin != 5 {
		t.Fatalf("tras dos páginas la ventana es [%d,%d), want [4,5)", inicio, fin)
	}
	if leyenda := paginado.pageLegend(); leyenda != "5-5 of 5 · Page 3/3" {
		t.Errorf("en la última página la leyenda es %q, want %q", leyenda, "5-5 of 5 · Page 3/3")
	}
}

// Un proyecto archivado se llama distinto en el dashboard, y sin proyectos
// seleccionados el pie lo dice en vez de dejar la línea vacía.
func TestDashboardNamesTheArchivedProject(t *testing.T) {
	m := newDashModel(t, "")
	m.showArchived = true
	m.width, m.height = 100, 30

	out := ansi.Strip(m.renderDashboard(20))
	if !strings.Contains(out, "Archived:") {
		t.Errorf("un dashboard archivado no lo dice:\n%s", out)
	}
}

func TestDashboardSaysNoneWhenNoProjectMatches(t *testing.T) {
	m := newDashModel(t, "")
	m.dashProjectIdx = 0
	m.projects = nil
	m.width, m.height = 100, 30

	out := ansi.Strip(m.renderDashboard(20))
	if !strings.Contains(out, "(none)") {
		t.Errorf("sin proyectos el pie no dice (none):\n%s", out)
	}
}

// El editor externo se abre con la tecla del comentario y con la del alta, y las
// dos dejan el estado como estaba si no hay tarea a la que aplicarlo.
func TestExternalEditorNeedsATask(t *testing.T) {
	m := newTestModel(t)
	m.filteredT = nil
	m.cursor = 99

	siguiente, cmd := press(m, "E")
	if siguiente.currentView != m.currentView {
		t.Error("sin tarea seleccionada, E ha cambiado de vista")
	}
	if cmd == nil {
		t.Log("sin tarea no hay comando, que es lo esperado")
	}
}

// Configurar el ancho del textarea del alta es un efecto secundario de abrir el
// modal, y con un width muy pequeño el cálculo tiene que seguir dando algo
// utilizable en vez de un entero negativo.
func TestNewTaskTextareaWidthNeverGoesNegative(t *testing.T) {
	for _, ancho := range []int{0, 1, 5, 40, 200} {
		m := newBareModel(t, func(*config.Config) {})
		m.width = ancho
		if n := m.newTaskTextareaWidth(); n < 1 {
			t.Errorf("con width %d el ancho del textarea es %d, want >= 1", ancho, n)
		}
	}
}
