package tui

import (
	"testing"

	"tsk/internal/model"
)

// El roster del modal de personas seconstruye con una función pura para poder
// comprobarlo sin modelo ni base de datos. Estas son las reglas que importan:
// quién aparece, qué se cuenta y en qué orden.

func assigneeTasks(fixture ...model.Task) []model.Task { return fixture }
func offdays(fixture ...model.OffDay) []model.OffDay   { return fixture }

var rosterTasks = assigneeTasks(
	model.Task{ID: 1, Assignee: "@juan", Status: "todo"},
	model.Task{ID: 2, Assignee: "@juan", Status: "doing"},
	model.Task{ID: 3, Assignee: "@juan", Status: "done"},
	model.Task{ID: 4, Assignee: "@maria", Status: "todo"},
	model.Task{ID: 5, Assignee: "", Status: "todo"},
	model.Task{ID: 6, Assignee: model.UnassignedAssignee, Status: "todo"},
)

var rosterOffDays = offdays(
	model.OffDay{ID: 1, Assignee: "@juan", StartDate: "2026-07-01", EndDate: "2026-07-02"},
	model.OffDay{ID: 2, Assignee: "@juan", StartDate: "2026-08-01", EndDate: "2026-08-01"},
	model.OffDay{ID: 3, Assignee: "@ana", StartDate: "2026-09-01", EndDate: "2026-09-02"},
)

func rosterByName(roster []assigneeSummary, name string) *assigneeSummary {
	for i := range roster {
		if roster[i].Name == name {
			return &roster[i]
		}
	}
	return nil
}

// "Me" está siempre en el roster aunque nadie tenga nada asignado: es el valor
// al que se atribuyen las tareas nuevas.
func TestBuildAssigneeRosterAlwaysHasMe(t *testing.T) {
	roster := buildAssigneeRoster(nil, nil)
	if len(roster) != 1 {
		t.Fatalf("sin nada asignado el roster tiene %d entradas, want 1 (sólo Me)", len(roster))
	}
	if roster[0].Name != "Me" {
		t.Errorf("nombre = %q, want Me", roster[0].Name)
	}
	if roster[0].Total != 0 || roster[0].Active != 0 || roster[0].OffDayCount != 0 {
		t.Errorf("Me = %+v, want todo a cero", roster[0])
	}
}

// Sin responsable no hay a quién atribuir nada: vacío y "unassigned" se
// saltan, en tareas y en off-days.
func TestBuildAssigneeRosterSkipsUnassigned(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	if s := rosterByName(roster, ""); s != nil {
		t.Errorf("una tarea sin responsable no debe crear entrada: %+v", s)
	}
	if s := rosterByName(roster, model.UnassignedAssignee); s != nil {
		t.Errorf("unassigned no debe crear entrada: %+v", s)
	}
	// @ana sólo tiene un off-day y ninguna tarea: sigue apareciendo.
	if s := rosterByName(roster, "@ana"); s == nil {
		t.Error("quien tiene un off-day debe aparecer aunque no tenga tareas")
	} else if s.Total != 0 || s.OffDayCount != 1 {
		t.Errorf("@ana = %+v, want 0 tareas y 1 off-day", *s)
	}
}

// Los totales y las activas se cuentan por separado.
func TestBuildAssigneeRosterCounts(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)

	juan := rosterByName(roster, "@juan")
	if juan == nil {
		t.Fatal("@juan no aparece")
	}
	if juan.Total != 3 {
		t.Errorf("@juan.Total = %d, want 3", juan.Total)
	}
	if juan.Active != 2 {
		t.Errorf("@juan.Active = %d, want 2 (la done no cuenta)", juan.Active)
	}
	if juan.OffDayCount != 2 {
		t.Errorf("@juan.OffDayCount = %d, want 2", juan.OffDayCount)
	}

	maria := rosterByName(roster, "@maria")
	if maria == nil {
		t.Fatal("@maria no aparece")
	}
	if maria.Total != 1 || maria.Active != 1 || maria.OffDayCount != 0 {
		t.Errorf("@maria = %+v, want 1/1/0", *maria)
	}
}

// El roster sale ordenado por nombre: el índice es la posición y tiene que ser
// estable entre renders aunque el mapa de detrás cambie de orden.
func TestBuildAssigneeRosterSorted(t *testing.T) {
	for range 20 {
		roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
		for i := 1; i < len(roster); i++ {
			if roster[i-1].Name >= roster[i].Name {
				t.Fatalf("roster desordenado en %d: %q antes que %q (%v)",
					i, roster[i-1].Name, roster[i].Name, roster)
			}
		}
	}
}

// Me aparece ordenada entre las demás, no siempre primera.
func TestBuildAssigneeRosterMeIsSorted(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	names := make([]string, len(roster))
	for i, s := range roster {
		names[i] = s.Name
	}
	want := []string{"@ana", "@juan", "@maria", "Me"}
	if len(names) != len(want) {
		t.Fatalf("roster = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("roster = %v, want %v", names, want)
			break
		}
	}
}

// Los off-days sin responsable tampoco crean entradas nuevas.
func TestBuildAssigneeRosterOffDayWithoutAssignee(t *testing.T) {
	ods := offdays(
		model.OffDay{ID: 1, Assignee: "", StartDate: "2026-07-01", EndDate: "2026-07-01"},
		model.OffDay{ID: 2, Assignee: model.UnassignedAssignee, StartDate: "2026-07-01", EndDate: "2026-07-01"},
	)
	roster := buildAssigneeRoster(nil, ods)
	if len(roster) != 1 {
		t.Errorf("roster = %+v, want sólo Me", roster)
	}
}

// tasksForAssignee sólo devuelve las no cerradas de esa persona.
func TestTasksForAssignee(t *testing.T) {
	got := tasksForAssignee(rosterTasks, "@juan")
	if len(got) != 2 {
		t.Fatalf("got %d tareas, want 2 (la done se queda fuera)", len(got))
	}
	for _, task := range got {
		if task.Assignee != "@juan" {
			t.Errorf("se coló una tarea de %q", task.Assignee)
		}
		if !task.IsActive() {
			t.Errorf("se coló una tarea cerrada: %+v", task)
		}
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("el orden de entrada no se respetó: %d, %d", got[0].ID, got[1].ID)
	}
}

func TestTasksForAssigneeNoMatch(t *testing.T) {
	if got := tasksForAssignee(rosterTasks, "@nadie"); len(got) != 0 {
		t.Errorf("got %d, want 0 para una persona sin tareas", len(got))
	}
	if got := tasksForAssignee(nil, "@juan"); len(got) != 0 {
		t.Errorf("got %d, want 0 sin tareas", len(got))
	}
}

// tasksForAssignee es un filtro por nombre Y NICAMENTE salta a los sin
// responsable, a diferencia de buildAssigneeRoster. La asimetría es real y está
// fijada a propósito: el único llamante pasa el nombre seleccionado en el modal,
// que sale del roster, y ahí siempre hay "Me" al menos. Añadir el filtro sería
// una condición más sin ningún caso que la ejercite.
func TestTasksForAssigneeMatchesEmptyAssignee(t *testing.T) {
	if got := tasksForAssignee(rosterTasks, ""); len(got) != 1 {
		t.Errorf("filtro por nombre con la vacía = %d tareas, want 1 (la del fixture)", len(got))
	}
	if got := tasksForAssignee(rosterTasks, model.UnassignedAssignee); len(got) != 1 {
		t.Errorf("filtro por nombre con unassigned = %d tareas, want 1", len(got))
	}
	// Pero esas personas no están en el roster, así que nunca llegan a ser
	// nombre seleccionado.
	if s := rosterByName(buildAssigneeRoster(rosterTasks, nil), ""); s != nil {
		t.Errorf("la vacía no debe aparecer en el roster: %+v", s)
	}
}

// offDaysForAssignee conserva el orden de carga, que es como los da la DB.
func TestOffDaysForAssignee(t *testing.T) {
	got := offDaysForAssignee(rosterOffDays, "@juan")
	if len(got) != 2 {
		t.Fatalf("got %d off-days, want 2", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("el orden cambió: %d, %d", got[0].ID, got[1].ID)
	}
	for _, o := range got {
		if o.Assignee != "@juan" {
			t.Errorf("se coló un off-day de %q", o.Assignee)
		}
	}
}

func TestOffDaysForAssigneeNoMatch(t *testing.T) {
	if got := offDaysForAssignee(rosterOffDays, "@nadie"); len(got) != 0 {
		t.Errorf("got %d, want 0", len(got))
	}
	if got := offDaysForAssignee(nil, "@juan"); len(got) != 0 {
		t.Errorf("got %d, want 0 sin off-days", len(got))
	}
}

// nameAt es la guarda antes de roster[idx]: un índice fuera de rango da "" en vez
// de reventar, y ése es el valor que los callers tratan como "nadie".
func TestNameAt(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{"primera", 0, "@ana"},
		{"del medio", 2, "@maria"},
		{"última", len(roster) - 1, "Me"},
		{"uno más allá", len(roster), ""},
		{"muy más allá", 99, ""},
		{"negativo", -1, ""},
		{"roster vacío", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := roster
			if tt.name == "roster vacío" {
				r = nil
			}
			if got := nameAt(r, tt.idx); got != tt.want {
				t.Errorf("nameAt(%d) = %q, want %q", tt.idx, got, tt.want)
			}
		})
	}
}

// nameAt y el roster no se desincronizan: el nombre de la posición i es el del
// roster en esa posición, para todo el rango.
func TestNameAtMatchesRoster(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	for i := range roster {
		if got := nameAt(roster, i); got != roster[i].Name {
			t.Errorf("nameAt(%d) = %q, want %q", i, got, roster[i].Name)
		}
	}
}
