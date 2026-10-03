package model

import (
	"testing"
	"time"
)

// Mon es el lunes de referencia de varios tests (2026-09-14).
const monday = "2026-09-14"

func mustDate(t *testing.T, s string) (d time.Time) {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func task(assignee, status string, estimate float64) Task {
	return Task{Assignee: assignee, Status: status, Estimate: estimate}
}

func TestBuildSchedulePacksFractions(t *testing.T) {
	tasks := []Task{
		task("@a", "todo", 0.5),
		task("@a", "todo", 0.5),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)

	if len(s.Assignees) != 1 {
		t.Fatalf("assignees = %d, want 1", len(s.Assignees))
	}
	a := s.Assignees[0]
	if len(a.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(a.Entries))
	}
	for i, e := range a.Entries {
		if e.Start != monday || e.End != monday {
			t.Errorf("entry %d = %s→%s, want %s→%s", i, e.Start, e.End, monday, monday)
		}
	}
	if a.End != monday {
		t.Errorf("assignee end = %s, want %s", a.End, monday)
	}
}

func TestBuildScheduleSpillsToNextDay(t *testing.T) {
	tasks := []Task{
		task("@a", "todo", 0.5),
		task("@a", "todo", 0.75),
		task("@a", "todo", 0.25),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)
	a := s.Assignees[0]

	if a.Entries[0].Start != monday || a.Entries[0].End != monday {
		t.Errorf("t1 = %s→%s, want %s→%s", a.Entries[0].Start, a.Entries[0].End, monday, monday)
	}
	if a.Entries[1].Start != monday || a.Entries[1].End != "2026-09-15" {
		t.Errorf("t2 = %s→%s, want %s→2026-09-15", a.Entries[1].Start, a.Entries[1].End, monday)
	}
	if a.Entries[2].Start != "2026-09-15" || a.Entries[2].End != "2026-09-15" {
		t.Errorf("t3 = %s→%s, want 2026-09-15", a.Entries[2].Start, a.Entries[2].End)
	}
}

func TestBuildScheduleSkipsWeekend(t *testing.T) {
	// Viernes 2026-09-18.
	friday := mustDate(t, "2026-09-18")
	tasks := []Task{
		task("@a", "todo", 1),
		task("@a", "todo", 1),
	}
	s := BuildSchedule(tasks, nil, friday, 1)
	a := s.Assignees[0]

	if a.Entries[0].Start != "2026-09-18" || a.Entries[0].End != "2026-09-18" {
		t.Errorf("t1 = %s→%s, want Friday", a.Entries[0].Start, a.Entries[0].End)
	}
	// El segundo cae el lunes siguiente, saltando sáb/dom.
	if a.Entries[1].Start != "2026-09-21" || a.Entries[1].End != "2026-09-21" {
		t.Errorf("t2 = %s→%s, want Monday 2026-09-21", a.Entries[1].Start, a.Entries[1].End)
	}
}

func TestBuildScheduleOffDayPerAssignee(t *testing.T) {
	offdays := []OffDay{
		{Assignee: "@alice", StartDate: monday, EndDate: monday},
	}
	tasks := []Task{
		task("@alice", "todo", 1),
		task("@bob", "todo", 1),
	}
	s := BuildSchedule(tasks, offdays, mustDate(t, monday), 1)
	byName := map[string]AssigneeSchedule{}
	for _, a := range s.Assignees {
		byName[a.Assignee] = a
	}

	if got := byName["@alice"].Entries[0]; got.Start != "2026-09-15" || got.End != "2026-09-15" {
		t.Errorf("alice (off) = %s→%s, want 2026-09-15", got.Start, got.End)
	}
	if got := byName["@bob"].Entries[0]; got.Start != monday || got.End != monday {
		t.Errorf("bob (working) = %s→%s, want %s", got.Start, got.End, monday)
	}
}

func TestBuildScheduleOffDayRange(t *testing.T) {
	// Toda la semana laboral libre → arranca el lunes siguiente.
	offdays := []OffDay{
		{Assignee: "@a", StartDate: monday, EndDate: "2026-09-18"},
	}
	s := BuildSchedule([]Task{task("@a", "todo", 1)}, offdays, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries[0]
	if e.Start != "2026-09-21" || e.End != "2026-09-21" {
		t.Errorf("start/end = %s→%s, want 2026-09-21", e.Start, e.End)
	}
}

func TestBuildScheduleDefaultEstimate(t *testing.T) {
	tasks := []Task{task("@a", "todo", 0)}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 2)
	e := s.Assignees[0].Entries[0]

	if e.Estimate != 2 {
		t.Errorf("estimate = %v, want 2 (default)", e.Estimate)
	}
	if !e.EstimateDefaulted {
		t.Error("EstimateDefaulted should be true")
	}
	if e.Start != monday || e.End != "2026-09-15" {
		t.Errorf("range = %s→%s, want %s→2026-09-15", e.Start, e.End, monday)
	}
}

// Un defaultEstimate no positivo se normaliza a 1 día. Los tests previos sólo
// pasaban 1 y 2, así que el borde exacto (0 y negativo) quedaba sin cubrir.
func TestBuildScheduleDefaultEstimateNoPositivo(t *testing.T) {
	tests := []struct {
		name       string
		defaultEst float64
	}{
		{"cero", 0},
		{"negativo", -2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := BuildSchedule([]Task{task("@a", "todo", 0)}, nil, mustDate(t, monday), tt.defaultEst)
			e := s.Assignees[0].Entries[0]
			if e.Estimate != 1 {
				t.Errorf("estimate = %v, want 1 (normalizado)", e.Estimate)
			}
			if !e.EstimateDefaulted {
				t.Error("EstimateDefaulted should be true")
			}
			if e.Start != monday || e.End != monday {
				t.Errorf("range = %s→%s, want %s→%s (1 día = el lunes entero)", e.Start, e.End, monday, monday)
			}
		})
	}
}

// Un estimate que no llega a medio minuto no consume NINGÚN día: no se reserva
// nada, así que la entrada se queda sin día de inicio (FormatDate del zero value)
// y el calendario no avanza. Fija ese suelo, que es lo que separa `restante > 0`
// de `restante >= 0`.
//
// Antes el suelo eran 1e-9 DÍAS y la comparación era de coma flotante, así que el
// borde era inalcanzable: ningún estimate cae exactamente en 1e-9. Ahora la
// aritmética es en minutos enteros, y 0 es un valor de verdad.
func TestBuildScheduleEstimateEnElSuelo(t *testing.T) {
	s := BuildSchedule([]Task{task("@a", "todo", 1e-9)}, nil, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries[0]

	if e.Estimate != 1e-9 {
		t.Errorf("estimate = %v, want 1e-9 (no se sanea un estimate explícito)", e.Estimate)
	}
	if e.EstimateDefaulted {
		t.Error("1e-9 no es 0: no debería marcarse EstimateDefaulted")
	}
	if e.End != monday {
		t.Errorf("end = %s, want %s (no consume día)", e.End, monday)
	}
	if e.Start == monday {
		t.Error("start = lunes: un estimate de epsilon no debería asignar día de inicio")
	}
}

func TestBuildScheduleExplicitEstimateNotDefaulted(t *testing.T) {
	s := BuildSchedule([]Task{task("@a", "todo", 0.25)}, nil, mustDate(t, monday), 1)
	e := s.Assignees[0].Entries[0]
	if e.EstimateDefaulted {
		t.Error("explicit estimate should not be marked defaulted")
	}
	if e.Estimate != 0.25 {
		t.Errorf("estimate = %v, want 0.25", e.Estimate)
	}
}

func TestBuildScheduleIgnoresTerminalTasks(t *testing.T) {
	tasks := []Task{
		task("@a", "done", 1),
		task("@a", "cancelled", 1),
		task("@a", "todo", 1),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)
	if len(s.Assignees) != 1 || len(s.Assignees[0].Entries) != 1 {
		t.Fatalf("expected only 1 scheduled entry, got %+v", s.Assignees)
	}
}

func TestBuildScheduleUnassigned(t *testing.T) {
	tasks := []Task{
		task("unassigned", "todo", 1),
		task("", "todo", 1),
		task("@a", "todo", 1),
	}
	s := BuildSchedule(tasks, nil, mustDate(t, monday), 1)
	if len(s.Unassigned) != 2 {
		t.Errorf("unassigned = %d, want 2", len(s.Unassigned))
	}
	if len(s.Assignees) != 1 || s.Assignees[0].Assignee != "@a" {
		t.Errorf("assignees = %+v, want only @a", s.Assignees)
	}
}

func TestFilterScheduleKeepsRealDates(t *testing.T) {
	tasks := []Task{
		{ID: 1, ProjectName: "api", Assignee: "@a", Status: "todo", Estimate: 1},
		{ID: 2, ProjectName: "web", Assignee: "@a", Status: "todo", Estimate: 1},
	}
	full := BuildSchedule(tasks, nil, mustDate(t, monday), 1)

	// Filtrar a "web" no debe recortar su fecha: sigue en el martes, aunque en
	// la vista no se vea la tarea de "api" del lunes.
	onlyWeb := FilterSchedule(full, func(t Task) bool { return t.ProjectName == "web" })
	if len(onlyWeb.Assignees) != 1 || len(onlyWeb.Assignees[0].Entries) != 1 {
		t.Fatalf("filtered assignees = %+v", onlyWeb.Assignees)
	}
	web := onlyWeb.Assignees[0].Entries[0]
	if web.Start != "2026-09-15" || web.End != "2026-09-15" {
		t.Errorf("web task = %s→%s, want 2026-09-15 (real slot, not recalculated)", web.Start, web.End)
	}
	if onlyWeb.Assignees[0].End != "2026-09-15" {
		t.Errorf("assignee end = %s, want 2026-09-15", onlyWeb.Assignees[0].End)
	}
}

func TestFilterScheduleDropsEmptyAndFiltersUnassigned(t *testing.T) {
	tasks := []Task{
		{ID: 1, ProjectName: "api", Assignee: "@a", Status: "todo", Estimate: 1},
		{ID: 2, ProjectName: "api", Assignee: "unassigned", Status: "todo", Estimate: 1},
	}
	full := BuildSchedule(tasks, nil, mustDate(t, monday), 1)

	onlyWeb := FilterSchedule(full, func(t Task) bool { return t.ProjectName == "web" })
	if len(onlyWeb.Assignees) != 0 {
		t.Errorf("assignees = %+v, want none", onlyWeb.Assignees)
	}
	if len(onlyWeb.Unassigned) != 0 {
		t.Errorf("unassigned = %+v, want none", onlyWeb.Unassigned)
	}
	if onlyWeb.Assignees == nil || onlyWeb.Unassigned == nil {
		t.Error("slices should be non-nil for clean JSON")
	}
}

func TestWeekOfMonthLabel(t *testing.T) {
	// t es el lunes que abre cada semana.
	tests := map[string]string{
		"2026-08-17": "3AUG",
		"2026-08-24": "4AUG",
		"2026-08-31": "1SEP",
		"2026-09-07": "2SEP",
		"2026-09-14": "3SEP",
		"2026-09-28": "1OCT",
		"2026-10-05": "2OCT",
	}
	for date, want := range tests {
		if got := WeekOfMonthLabel(mustDate(t, date)); got != want {
			t.Errorf("WeekOfMonthLabel(%s) = %q, want %q", date, got, want)
		}
	}
}

func TestFormatEstimate(t *testing.T) {
	tests := map[float64]string{1: "1d", 2: "2d", 0.5: "0.5d", 0.25: "0.25d", 1.5: "1.5d"}
	for in, want := range tests {
		if got := FormatEstimate(in); got != want {
			t.Errorf("FormatEstimate(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestIsOffDay(t *testing.T) {
	offdays := []OffDay{{Assignee: "@a", StartDate: monday, EndDate: "2026-09-15"}}
	// Fin de semana siempre.
	if !IsOffDay(offdays, "@a", mustDate(t, "2026-09-19")) {
		t.Error("Saturday should be off")
	}
	// Rango del assignee.
	if !IsOffDay(offdays, "@a", mustDate(t, "2026-09-15")) {
		t.Error("off-day in range should be off")
	}
	// Otra persona no se ve afectada.
	if IsOffDay(offdays, "@b", mustDate(t, "2026-09-15")) {
		t.Error("other assignee should work")
	}
	if IsOffDay(offdays, "@a", mustDate(t, "2026-09-16")) {
		t.Error("day after range should be working")
	}
}

// El borde del día lleno.
//
// El reparto empqueta tareas: dos de medio día caben en el mismo día, y al
// llenarse la siguiente salta al laborable siguiente. Ese salto es un `libre == 0`
// -- una comparación de igualdad, no un epsilon -- y se alcanza con cualquier
// estimate que llene el día entero.
//
// Estos dos casos son los lados opuestos del mismo borde, y son los que
// separan `libre == 0` de `libre != 0`: si el salto no ocurriera, las dos tareas
// de un día empezarían el mismo día.
func TestBuildScheduleEmpaquetaYThenSalta(t *testing.T) {
	t.Run("dos medias caben en el mismo día", func(t *testing.T) {
		s := BuildSchedule([]Task{
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
		}, nil, mustDate(t, monday), 1)

		e := s.Assignees[0].Entries
		if len(e) != 2 {
			t.Fatalf("entradas = %d, want 2", len(e))
		}
		if e[0].Start != monday || e[1].Start != monday {
			t.Errorf("inicios = %s, %s; want %s los dos: medio día + medio día llena el día",
				e[0].Start, e[1].Start, monday)
		}
	})

	t.Run("al llenarse el día, la siguiente salta", func(t *testing.T) {
		s := BuildSchedule([]Task{
			task("@a", "todo", 1.0),
			task("@a", "todo", 1.0),
		}, nil, mustDate(t, monday), 1)

		e := s.Assignees[0].Entries
		if len(e) != 2 {
			t.Fatalf("entradas = %d, want 2", len(e))
		}
		if e[0].Start != monday {
			t.Errorf("la primera empieza el %s, want %s", e[0].Start, monday)
		}
		if e[1].Start == monday {
			t.Errorf("la segunda también empieza el %s: el día ya estaba lleno y tenía que saltar",
				e[1].Start)
		}
		if e[1].Start != "2026-09-15" {
			t.Errorf("la segunda empieza el %s, want 2026-09-15 (el martes)", e[1].Start)
		}
	})

	t.Run("tres medias no caben en un día", func(t *testing.T) {
		s := BuildSchedule([]Task{
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
			task("@a", "todo", 0.5),
		}, nil, mustDate(t, monday), 1)

		e := s.Assignees[0].Entries
		// Dos medias llenan el día; la tercera ya no cabe y salta. Es el mismo
		// borde por el otro lado del relleno.
		if e[0].Start != monday || e[1].Start != monday {
			t.Errorf("inicios = %s, %s; want %s los dos", e[0].Start, e[1].Start, monday)
		}
		for i := 2; i < len(e); i++ {
			if e[i].Start == monday {
				t.Errorf("la tarea %d empieza el %s: el día ya estaba lleno", i, e[i].Start)
			}
		}
		if e[2].Start != "2026-09-15" {
			t.Errorf("la tercera empieza el %s, want 2026-09-15", e[2].Start)
		}
	})
}

// El suelo en cero del estimate: por debajo de medio minuto no se pinta barra,
// pero la tarea SIGUE en el calendario y el resto de la cola no se descuadra.
func TestBuildScheduleSueloRedondeadoNoDescolocaLaCola(t *testing.T) {
	s := BuildSchedule([]Task{
		task("@a", "todo", 0.0001), // se redondea a 0 minutos
		task("@a", "todo", 1.0),
	}, nil, mustDate(t, monday), 1)

	e := s.Assignees[0].Entries
	if len(e) != 2 {
		t.Fatalf("entradas = %d, want 2", len(e))
	}
	if e[0].Start == monday {
		t.Error("la tarea de 0 minutos tiene día de inicio: no debería pintar barra")
	}
	// Y la siguiente sigue en el lunes: la tarea de 0 minutos no ha gastado día.
	if e[1].Start != monday {
		t.Errorf("la segunda empieza el %s, want %s: una tarea de 0 minutos no consume día",
			e[1].Start, monday)
	}
}

// minutosDe es la función pura del refactor, y su redondeo es una decisión con
// consecuencias visibles: por debajo de medio minuto no hay barra.
func TestMinutosDeRedondeaAlMinuto(t *testing.T) {
	casos := []struct {
		est  float64
		want int
	}{
		{0, 0},
		{0.0001, 0},
		{1.0 / (3 * minutosPorDia), 0}, // un tercio de minuto, se redondea a 0
		{1.0 / (2 * minutosPorDia), 1}, // medio minuto: round() va hacia arriba
		{0.5, 720},
		{1, minutosPorDia},
		{1.5, 2160},
		{-1, -minutosPorDia},
	}
	for _, c := range casos {
		if got := minutosDe(c.est); got != c.want {
			t.Errorf("minutosDe(%v) = %d, want %d", c.est, got, c.want)
		}
	}
}
