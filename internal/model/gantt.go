package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// dateLayout es el formato canónico de fecha del Gantt (YYYY-MM-DD).
const dateLayout = "2006-01-02"

// UnassignedAssignee es el valor de assignee que no se proyecta en el Gantt.
const UnassignedAssignee = "unassigned"

// epsilon tolera el error de redondeo al comparar fracciones de día.
const epsilon = 1e-9

// OffDay es un día no laborable de una persona: vacaciones, feriado o
// ausencia. El rango [StartDate, EndDate] es inclusivo en ambos extremos; un
// solo día usa la misma fecha en ambos campos.
type OffDay struct {
	ID        int64  `json:"id"`
	Assignee  string `json:"assignee"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Note      string `json:"note,omitempty"`
}

// ScheduleEntry es una tarea proyectada en el calendario.
type ScheduleEntry struct {
	Task              Task    `json:"task"`
	Start             string  `json:"start"`
	End               string  `json:"end"`
	Estimate          float64 `json:"estimate"`
	EstimateDefaulted bool    `json:"estimate_defaulted"`
}

// AssigneeSchedule es la cola secuencial proyectada de una persona.
type AssigneeSchedule struct {
	Assignee string          `json:"assignee"`
	Entries  []ScheduleEntry `json:"entries"`
	End      string          `json:"end"`
}

// Schedule es la proyección completa: una cola por persona más las tareas sin
// responsable (que no se pueden agendar).
type Schedule struct {
	Start      string             `json:"start"`
	Assignees  []AssigneeSchedule `json:"assignees"`
	Unassigned []Task             `json:"unassigned"`
}

// ParseDate convierte "YYYY-MM-DD" a time.Time en UTC.
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, s, time.UTC)
}

// FormatDate formatea un time.Time como "YYYY-MM-DD".
func FormatDate(t time.Time) string {
	return t.Format(dateLayout)
}

// IsUnassigned indica si una tarea no tiene responsable y por tanto no se agenda.
func IsUnassigned(assignee string) bool {
	return assignee == "" || assignee == UnassignedAssignee
}

// IsOffDay indica si day es no laborable para assignee: fin de semana o un
// off-day registrado. Se usa para pintar el calendario, no para agendar.
func IsOffDay(offdays []OffDay, assignee string, day time.Time) bool {
	day = truncateDay(day)
	if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return true
	}
	return isOff(day, assignee, offRangesByAssignee(offdays))
}

// FormatEstimate da formato a un estimate en días: entero sin decimales
// ("2d") y fracción con su valor exacto ("0.5d").
func FormatEstimate(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10) + "d"
	}
	return strconv.FormatFloat(v, 'f', -1, 64) + "d"
}

// WeekOfMonthLabel etiqueta la semana que empieza en el lunes t por su número
// dentro del mes pegado al mes en mayúsculas, por ejemplo "1SEP" o "3OCT". La
// semana y el mes se determinan por el jueves de esa semana (la semana que
// contiene el día 1 es la 1), así el mes queda alineado con donde realmente cae.
func WeekOfMonthLabel(t time.Time) string {
	thu := t.AddDate(0, 0, 3) // lunes + 3 = jueves
	return fmt.Sprintf("%d%s", (thu.Day()-1)/7+1, strings.ToUpper(thu.Format("Jan")))
}

// offRange es un rango de días no laborables ya parseado a time.Time.
type offRange struct {
	start, end time.Time
}

// BuildSchedule proyecta una cola secuencial por persona: cada persona hace
// una tarea a la vez, en el orden en que llegan (prioridad → estado → id), y
// consume días laborables (lunes a viernes, salvo sus off-days). No hay
// dependencias entre tareas ni paralelismo dentro de una persona.
//
// Las tareas done/cancelled se ignoran; las tareas sin responsable (vacío o
// "unassigned") se devuelven en Unassigned sin agendar. Una tarea sin estimate
// usa defaultEstimate y queda marcada con EstimateDefaulted.
func BuildSchedule(tasks []Task, offdays []OffDay, start time.Time, defaultEstimate float64) *Schedule {
	if defaultEstimate <= 0 {
		defaultEstimate = 1
	}
	start = truncateDay(start)
	lookup := offRangesByAssignee(offdays)

	sched := &Schedule{Start: FormatDate(start)}
	queues := map[string][]Task{}
	var order []string
	for _, t := range tasks {
		if !t.IsActive() {
			continue
		}
		if IsUnassigned(t.Assignee) {
			sched.Unassigned = append(sched.Unassigned, t)
			continue
		}
		if _, ok := queues[t.Assignee]; !ok {
			order = append(order, t.Assignee)
		}
		queues[t.Assignee] = append(queues[t.Assignee], t)
	}
	sort.Strings(order)

	for _, assignee := range order {
		s := AssigneeSchedule{Assignee: assignee}
		cursor := nextWorkingDay(start, assignee, lookup)
		capacity := 1.0 // fracción de día aún disponible en cursor

		for _, t := range queues[assignee] {
			est := t.Estimate
			def := false
			if est <= 0 {
				est = defaultEstimate
				def = true
			}

			remaining := est
			var entryStart time.Time
			started := false
			for remaining > epsilon {
				if capacity <= epsilon {
					cursor = nextWorkingDay(cursor.AddDate(0, 0, 1), assignee, lookup)
					capacity = 1.0
				}
				if !started {
					entryStart = cursor
					started = true
				}
				consume := remaining
				if consume > capacity {
					consume = capacity
				}
				remaining -= consume
				capacity -= consume
				if remaining > epsilon {
					cursor = nextWorkingDay(cursor.AddDate(0, 0, 1), assignee, lookup)
					capacity = 1.0
				}
			}

			s.Entries = append(s.Entries, ScheduleEntry{
				Task:              t,
				Start:             FormatDate(entryStart),
				End:               FormatDate(cursor),
				Estimate:          est,
				EstimateDefaulted: def,
			})
		}
		if n := len(s.Entries); n > 0 {
			s.End = s.Entries[n-1].End
		}
		sched.Assignees = append(sched.Assignees, s)
	}
	return sched
}

// FilterSchedule devuelve una copia de s con sólo las tareas que cumplen keep.
// Es un filtro de VISTA: no recalcula fechas ni colas, así que una tarea oculta
// sigue ocupando su lugar en la línea de tiempo de la persona (puede haber
// huecos entre las tareas visibles). Las personas que quedan sin tareas se
// descartan.
func FilterSchedule(s *Schedule, keep func(Task) bool) *Schedule {
	out := &Schedule{Start: s.Start, Assignees: []AssigneeSchedule{}, Unassigned: []Task{}}
	for _, a := range s.Assignees {
		filtered := AssigneeSchedule{Assignee: a.Assignee, Entries: []ScheduleEntry{}}
		for _, e := range a.Entries {
			if keep(e.Task) {
				filtered.Entries = append(filtered.Entries, e)
			}
		}
		if len(filtered.Entries) == 0 {
			continue
		}
		filtered.End = filtered.Entries[len(filtered.Entries)-1].End
		out.Assignees = append(out.Assignees, filtered)
	}
	for _, t := range s.Unassigned {
		if keep(t) {
			out.Unassigned = append(out.Unassigned, t)
		}
	}
	return out
}

// nextWorkingDay avanza day hasta el primer día laborable para assignee:
// sábado y domingo siempre son no laborables, más sus off-days.
func nextWorkingDay(day time.Time, assignee string, lookup map[string][]offRange) time.Time {
	for {
		wd := day.Weekday()
		if wd != time.Saturday && wd != time.Sunday && !isOff(day, assignee, lookup) {
			return day
		}
		day = day.AddDate(0, 0, 1)
	}
}

// isOff indica si day cae dentro de algún off-day de assignee.
func isOff(day time.Time, assignee string, lookup map[string][]offRange) bool {
	for _, r := range lookup[assignee] {
		if !day.Before(r.start) && !day.After(r.end) {
			return true
		}
	}
	return false
}

// offRangesByAssignee agrupa los off-days por persona, ignorando rangos con
// fechas inválidas.
func offRangesByAssignee(offdays []OffDay) map[string][]offRange {
	m := map[string][]offRange{}
	for _, o := range offdays {
		s, err := ParseDate(o.StartDate)
		if err != nil {
			continue
		}
		e, err := ParseDate(o.EndDate)
		if err != nil {
			continue
		}
		if e.Before(s) {
			s, e = e, s
		}
		m[o.Assignee] = append(m[o.Assignee], offRange{start: s, end: e})
	}
	return m
}

// truncateDay normaliza un time.Time a medianoche UTC conservando el día de
// calendario local de entrada.
func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
