package model

import (
	"testing"
	"time"
)

// PriorityLabel y PriorityBar tienen un `default` que los tests existentes no
// tocaban, y los tres casos no prioritarios devolvían etiquetas distintas.
func TestPriorityLabelsCoverEveryPriority(t *testing.T) {
	for _, tc := range []struct {
		prio     int
		label    string
		short    string
		bar      string
		esUltima bool
	}{
		{prio: PriorityLow, label: "LOW", short: "L", bar: "●"},
		{prio: PriorityMedium, label: "MED", short: "M", bar: "●"},
		{prio: PriorityHigh, label: "HIGH", short: "H", bar: "●"},
		{prio: 0, label: "none", short: "-", bar: " "},
		{prio: 99, label: "none", short: "-", bar: " "},
	} {
		t.Run(tc.label+"/"+tc.short, func(t *testing.T) {
			if got := PriorityLabel(tc.prio); got != tc.label {
				t.Errorf("PriorityLabel(%d) = %q, want %q", tc.prio, got, tc.label)
			}
			if got := PriorityShortLabel(tc.prio); got != tc.short {
				t.Errorf("PriorityShortLabel(%d) = %q, want %q", tc.prio, got, tc.short)
			}
			if got := PriorityBar(tc.prio); got != tc.bar {
				t.Errorf("PriorityBar(%d) = %q, want %q", tc.prio, got, tc.bar)
			}
		})
	}
}

// ParseTagsJSON no falla nunca: un JSON que no es una lista de cadenas es lo
// mismo que no tener tags. Perder las tags de una tarea es malo, pero perder la
// tarea entera de la vista sería peor.
func TestParseTagsJSONIsTotal(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{in: ``, want: nil},
		{in: `   `, want: nil},
		{in: `null`, want: nil},
		{in: `[`, want: nil},
		{in: `{"a":1}`, want: nil},
		{in: `["uno"]`, want: []string{"uno"}},
		{in: `["  uno  ","DOS","dos"]`, want: []string{"uno", "dos"}},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got := ParseTagsJSON(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseTagsJSON(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("ParseTagsJSON(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ParseWorkflowJSON sí devuelve error, a diferencia de ParseTagsJSON: un workflow
// ilegible es un dato corrupto que hay que reportar, no algo que se pueda
// sustituir por un default sin que nadie se entere.
func TestParseWorkflowJSONReportsBadInput(t *testing.T) {
	if _, err := ParseWorkflowJSON(`["backlog"`); err == nil {
		t.Error("ParseWorkflowJSON con JSON truncado: want error")
	}
	got, err := ParseWorkflowJSON(`["backlog","done"]`)
	if err != nil {
		t.Fatalf("ParseWorkflowJSON: %v", err)
	}
	if len(got) != 2 || got[0] != "backlog" || got[1] != "done" {
		t.Errorf("ParseWorkflowJSON = %v, want [backlog done]", got)
	}
}

// Los off-days con fechas ilegíveis se descartan en vez de tumbar el calendario
// entero, y un rango escrito al revés se normaliza. Los dos son cosas que un
// usuario puede escribir sin querer.
func TestOffRangesSkipBadDatesAndNormalizeReversedRanges(t *testing.T) {
	offdays := []OffDay{
		{Assignee: "@juan", StartDate: "2026-03-02", EndDate: "2026-03-01", Note: "al revés"},
		{Assignee: "@juan", StartDate: "no-es-fecha", EndDate: "2026-03-05", Note: "inicio malo"},
		{Assignee: "@juan", StartDate: "2026-03-06", EndDate: "tampoco", Note: "fin malo"},
		{Assignee: "@maria", StartDate: "2026-03-03", EndDate: "2026-03-04", Note: "bueno"},
	}

	got := offRangesByAssignee(offdays)

	juan, ok := got["@juan"]
	if !ok {
		t.Fatalf("no hay rangos para @juan: %v", got)
	}
	if len(juan) != 1 {
		t.Fatalf("@juan tiene %d rangos, want 1 (los dos ilegibles se descartan)", len(juan))
	}
	if want := (time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)); !juan[0].start.Equal(want) {
		t.Errorf("el rango empieza el %s, want %s: un rango al revés se normaliza",
			juan[0].start.Format("2006-01-02"), want.Format("2006-01-02"))
	}

	if len(got["@maria"]) != 1 {
		t.Errorf("@maria tiene %d rangos, want 1", len(got["@maria"]))
	}
}

// El filtro del calendario tiene que dejar pasar las tareas sin responsable:
// son las que no aparecen en ninguna cola de persona, así que si el filtro las
// eliminara desaparecerían de la vista sin más.
func TestFilterScheduleKeepsUnassignedTasks(t *testing.T) {
	tarea := func(assignee string) Task {
		return Task{Assignee: assignee, Priority: PriorityHigh}
	}
	horquilla := &Schedule{
		Start: "2026-03-01",
		Assignees: []AssigneeSchedule{{
			Assignee: "@juan",
			Entries:  []ScheduleEntry{{Task: tarea("@juan")}},
		}},
		Unassigned: []Task{tarea(""), tarea(""), tarea("")},
	}

	filtrada := FilterSchedule(horquilla, func(Task) bool { return true })

	if len(filtrada.Unassigned) != 3 {
		t.Errorf("quedan %d tareas sin responsable, want 3", len(filtrada.Unassigned))
	}
	if len(filtrada.Assignees) != 1 || len(filtrada.Assignees[0].Entries) != 1 {
		t.Errorf("la cola de @juan no sobrevive intacta: %+v", filtrada.Assignees)
	}

	// Y un filtro que lo rechaza todo vacía las dos listas, sin dejar personas
	// con una cola vacía colgando.
	filtrada = FilterSchedule(horquilla, func(Task) bool { return false })
	if len(filtrada.Unassigned) != 0 || len(filtrada.Assignees) != 0 {
		t.Errorf("con un filtro que lo rechaza todo queda %d sin responsable y %d colas",
			len(filtrada.Unassigned), len(filtrada.Assignees))
	}

	// El original no se toca: es una copia, no un recorte in situ.
	if len(horquilla.Unassigned) != 3 || len(horquilla.Assignees) != 1 {
		t.Error("FilterSchedule ha modificado el schedule de entrada")
	}
}
