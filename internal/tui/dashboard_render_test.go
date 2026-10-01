package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// El Dashboard junta tres bloques con budgets de filas y un filtro por proyecto.
// Su aritmética está en el render y comparar "sale la palabra" no la ve: un
// signo movido en el total cambia el número, y una condición de tope mal puesta
// cambia cuántas filas caben sin quitar ninguna palabra de la pantalla.

// dashRender devuelve el dashboard renderizado, sin colores.
func dashRender(t *testing.T, m *Model) string {
	t.Helper()
	m.currentView = viewDashboard
	m.width = 140
	m.height = 40
	return ansi.Strip(m.renderDashboard(40))
}

// newDashModel parte del modelo de test, con el filtro por proyecto apuntando a
// project y sin filtro de estado, para que el recorte sea observable.
//
// El recorte por proyecto del Dashboard NO es el filtro general: sale del
// selector de proyecto propio, el "[api(3)]" de la línea de arriba. Se elige con
// dashProjectIdx, y -1 es "todos". Por defecto New lo deja en el primer
// proyecto, así que sin tocarlo el Overview contaría un solo proyecto siempre y
// el filtro general no tendría efecto ninguno sobre esta vista.
func newDashModel(t *testing.T, project string) *Model {
	t.Helper()
	m := newTestModel(t)
	m.filterStatus = ""
	m.filteredT = nil
	if project == "" {
		m.dashProjectIdx = -1
		return m
	}
	for i, p := range m.dashProjectList() {
		if p.Name == project {
			m.dashProjectIdx = i
			return m
		}
	}
	t.Fatalf("el fixture no tiene el proyecto %q", project)
	return m
}

// El total del Overview es la suma de activas, terminadas y canceladas. Las
// canceladas van con signo más: el total es "todas las tareas", y con sólo
// activas y hechas una tarea cancelada se contaría por su ausencia.
func TestDashTotalCountsEveryTask(t *testing.T) {
	m := newDashModel(t, "")
	mustCreateTask(t, m.database, "api", "cancelada", "", "@juan", 3, model.CancelledStatus)
	reloadTasks(t, m)
	activas, hechas, canceladas, _ := dashStatusCounts(m.tasks, "")
	todas := activas + hechas + canceladas
	if todas == 0 {
		t.Fatal("el fixture no dejó tareas")
	}

	out := dashRender(t, m)
	if want := "  Total         " + strconv.Itoa(todas); !strings.Contains(out, want) {
		t.Errorf("el total no dice %q:\n%s", want, out)
	}

	// La línea de proyectos lista los proyectos con su recuento. Sin esto, una
	// condición que se disparara siempre -- del tipo "si no hay partes, muestra
	// (none)"Evaluate mal puesta -- pasaría el test del total.
	if !strings.Contains(out, "api(") || !strings.Contains(out, "web(") {
		t.Errorf("la línea de proyectos no lista los proyectos:\n%s", out)
	}
	if strings.Contains(out, "(none)") {
		t.Errorf("salió (none) con proyectos en la lista:\n%s", out)
	}
}

// En la línea de proyectos, el que está seleccionado va entre corchetes y
// resaltado. Con "todos" seleccionados no hay ninguno entre corchetes, y con un
// proyecto elegido, exactamente ese.
//
// Los corchetes son lo que distingue la condición de su versión negada: marking
// todos o ninguno deja los nombres en la pantalla igual, así que un test que
// buscara "api(" no lo notaría.
func TestDashSelectedProjectIsBracketed(t *testing.T) {
	todos := newDashModel(t, "")
	if n := countBracketed(dashRender(t, todos)); n != 0 {
		t.Errorf("con \"todos\" seleccionados hay %d proyectos entre corchetes, want 0", n)
	}

	m := newDashModel(t, "api")
	out := dashRender(t, m)
	if n := countBracketed(out); n != 1 {
		t.Errorf("con api seleccionado hay %d proyectos entre corchetes, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "[api(") {
		t.Errorf("el proyecto entre corchetes no es el seleccionado:\n%s", out)
	}
	if strings.Contains(out, "[web(") {
		t.Errorf("apareció un proyecto que no estaba seleccionado entre corchetes:\n%s", out)
	}
}

// countBracketed cuenta los pares de corchetes de la línea de proyectos.
func countBracketed(rendered string) int {
	n := 0
	for _, line := range strings.Split(rendered, "\n") {
		i := strings.Index(line, "Projects:")
		if i < 0 {
			continue
		}
		n += strings.Count(line[i:], "[")
	}
	return n
}

// Con un proyecto seleccionado en el Dashboard, el total cuenta sólo ese
// proyecto. El fixture reparte cuatro tareas entre "api" y "web", así que el
// número filtrado y el global no coinciden y un total que se olvidara del
// proyecto se vería.
func TestDashTotalRespectsSelectedProject(t *testing.T) {
	m := newDashModel(t, "api")

	a, d, c, _ := dashStatusCounts(m.tasks, "api")
	api := a + d + c
	a, d, c, _ = dashStatusCounts(m.tasks, "")
	todas := a + d + c
	if api == todas {
		t.Fatalf("el fixture no distingue: api=%d, todas=%d", api, todas)
	}

	out := dashRender(t, m)
	if !strings.Contains(out, "  Total         "+strconv.Itoa(api)) {
		t.Errorf("con api seleccionado el total no es %d:\n%s", api, out)
	}
	if strings.Contains(out, "  Total         "+strconv.Itoa(todas)) {
		t.Errorf("el total es el de todos los proyectos (%d), no el de api (%d):\n%s", todas, api, out)
	}
}

// Un estado sin tareas no sale: el Overview lista los del workflow con barra, y
// una barra vacía no dice nada.
func TestDashOverviewSkipsEmptyStatuses(t *testing.T) {
	m := newDashModel(t, "")

	out := dashRender(t, m)
	for _, linea := range strings.Split(out, "\n") {
		fields := strings.Fields(linea)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "todo" || fields[0] == "backlog" {
			continue
		}
		if len(fields) >= 2 && fields[1] == "0" {
			t.Errorf("salió un estado con cero tareas: %q", linea)
		}
	}
	if !strings.Contains(out, "backlog") {
		t.Errorf("el estado con tareas no salió:\n%s", out)
	}
}

// Un nombre de estado más largo que 14 columnas se recorta, y el recuento va
// detrás en su sitio: sin el recorte la barra descuadraría la tabla.
func TestDashOverviewClipsLongStatusNames(t *testing.T) {
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	largo := "revision-pendiente"
	mustCreateProject(t, database, "largo", []string{largo, model.DoneStatus})
	mustCreateTask(t, database, "largo", "tarea", "", "@juan", 1, largo)

	m := New(database, config.Defaults())
	m.width, m.height = 140, 40
	reloadTasks(t, &m)
	reloadProjects(t, &m)
	m.filterStatus = ""

	out := dashRender(t, &m)
	// El recorte son 14 bytes del nombre entero: "revision-pendi" de 18.
	if !strings.Contains(out, "revision-pendi 1") {
		t.Errorf("no se vio el estado recortado a 14:\n%s", out)
	}
	if strings.Contains(out, largo) {
		t.Errorf("el estado de %d caracteres salió sin recortar:\n%s", len(largo), out)
	}
}

// El panel Active sale ordenado y con la cabecera; con el filtro puesto, sólo
// las tareas del proyecto.
func TestDashActiveRespectsSelectedProject(t *testing.T) {
	m := newDashModel(t, "api")

	out := dashRender(t, m)
	if !strings.Contains(out, "Active") {
		t.Fatalf("no se ve el panel Active:\n%s", out)
	}
	if strings.Contains(out, "Fix checkout") {
		t.Errorf("salió una tarea del proyecto filtrado fuera:\n%s", out)
	}
	if !strings.Contains(out, "N+1") {
		t.Errorf("no salió ninguna tarea del proyecto del filtro:\n%s", out)
	}
}

// El panel Active está acotado por el alto disponible: con muchas tareas y poco
// alto, corta por abajo en vez de desbordar la caja.
func TestDashActiveTruncatesToBudget(t *testing.T) {
	m := newTestModel(t)
	for range 20 {
		mustCreateTask(t, m.database, "api", "tarea extra", "", "@juan", 1, "todo")
	}
	reloadTasks(t, m)
	m.filterProject = "api"
	m.filterStatus = ""
	m.currentView = viewDashboard
	m.width, m.height = 140, 40

	holgado := strings.Count(ansi.Strip(m.renderDashboard(40)), "\n")
	estrecho := strings.Count(ansi.Strip(m.renderDashboard(14)), "\n")
	if estrecho >= holgado {
		t.Errorf("con 14 de alto salen %d líneas y con 40 salen %d: el alto no acota",
			estrecho, holgado)
	}
	if estrecho == 0 {
		t.Error("con 14 de alto no sale nada")
	}
}

// El equipo sale ordenado alfabéticamente, que es lo que hace que dos renders
// seguidos se vean iguales.
func TestDashTeamIsSorted(t *testing.T) {
	m := newTestModel(t)
	for _, name := range []string{"@zeta", "@alfa"} {
		mustCreateTask(t, m.database, "api", "tarea de "+name, "", name, 1, "todo")
	}
	reloadTasks(t, m)
	m.filterProject = "api"
	m.filterStatus = ""

	out := dashRender(t, m)
	iAlfa := strings.Index(out, "@alfa")
	iZeta := strings.Index(out, "@zeta")
	if iAlfa < 0 || iZeta < 0 {
		t.Fatalf("faltan personas en el panel de equipo:\n%s", out)
	}
	if iAlfa > iZeta {
		t.Errorf("@zeta sale antes que @alfa: el equipo no va ordenado\n%s", out)
	}
}
