package model

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// dateLayout es el formato canónico de fecha del Gantt (YYYY-MM-DD).
const dateLayout = "2006-01-02"

// UnassignedAssignee es el valor de assignee que no se proyecta en el Gantt.
const UnassignedAssignee = "unassigned"

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
		// libre son los minutos sin ocupar del día que está en cursor. Cruza
		// de una tarea a otra a propósito: dos tareas de medio día se empaquetan
		// en el mismo día, que es justo de lo que va el gantt.
		libre := minutosPorDia

		for _, t := range queues[assignee] {
			est := t.Estimate
			def := false
			if est <= 0 {
				est = defaultEstimate
				def = true
			}

			// La aritmética va en MINUTOS, no en fracciones de día. Con coma
			// flotante, "queda un día entero" y "no queda nada" sólo se
			// distinguían comparando contra epsilon, y como ningún estimate
			// caía nunca exactamente en epsilon esas dos comparaciones no las
			// podía matar ningún test: eran la clase de línea que parece decidir
			// algo y no decide. En enteros el mismo borde es un `== 0` de
			// verdad, que se alcanza con cualquier estimate redondo y que un test
			// sí alcanza por los dos lados.
			//
			// El cambio no altera ningún resultado: 1440 minutos son un día, así
			// que el reparto sale igual. Lo que cambia es que ahora se puede
			// comprobar.
			total := minutosDe(est)
			var entryStart time.Time
			if total > 0 {
				// El salto por día lleno va ANTES de fijar el inicio: si el día
				// que estaba en cursor ya lo consumió la tarea anterior, esta
				// empieza en el siguiente laborable, no en uno ya lleno. Por eso
				// está aquí y no dentro del bucle, que ya sólo avanza cuando le
				// queda trabajo a media tarea.
				if libre == 0 {
					cursor = siguienteDia(cursor, assignee, lookup)
					libre = minutosPorDia
				}
				entryStart = cursor

				// El reparto es un `for range` sobre el número de DÍAS COMPLETOS,
				// no un bucle que descuenta un saldo. La cuenta no depende de que
				// el saldo llegue a cero, así que ninguna mutación de la condición
				// puede dejarla dando vueltas: con `restante >= 0` el saldo se
				// quedaba en cero, libre en cero, y siguienteDia avanzaba el cursor
				// para siempre. Un `for range` sobre un entero que no se decrementa
				// dentro no puede colgar.
				//
				// El resto del día se consume a mano, después de los días completos.
				// Es lo que quedaba como última vuelta del bucle anterior, y queda
				// aquí porque así el bucle no tiene salida: siempre se sale por el
				// final.
				resto := libre
				for range divRound(total, minutosPorDia) {
					consumo := min(resto, total)
					resto -= consumo
					total -= consumo
					if total > 0 {
						cursor = siguienteDia(cursor, assignee, lookup)
						resto = minutosPorDia
					}
				}
				libre = resto
			}

			s.Entries = append(s.Entries, ScheduleEntry{
				Task:              t,
				Start:             FormatDate(entryStart),
				End:               FormatDate(cursor),
				Estimate:          est,
				EstimateDefaulted: def,
			})
			// End del assignee = fin de su última entrada. Se asigna dentro
			// del bucle en vez de con `if n := len(s.Entries); n > 0`: la
			// cola nunca está vacía (todo assignee viene de >= 1 tarea), así
			// que ese `> 0` era siempre cierto y su mutant no se distinguía.
			s.End = s.Entries[len(s.Entries)-1].End
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
// minutosPorDia es la capacidad de un día laborable, en minutos.
const minutosPorDia = 24 * 60

// divRound divide redondeando al entero más cercano. Se usa para el número de
// días completos de una estimación: un estimate de medio día son 0 días y el resto
// se consume aparte, y uno de un día y medio es 1 día completo más medio minuto.
//
// El divisor es siempre minutosPorDia, así que la guarda contra cero no hace
// falta: con ella, el `b < 1` tenía un borde (b == 1) que ningún test alcanza, y
// por eso su mutante era indistinguible. El divisor es un parámetro para que los
// tests puedan pasar otros, y se documenta que en producción es siempre 1440.
func divRound(a, b int) int {
	return (a + b/2) / b
}

// minutosDe convierte una estimación en días a minutos, redondeando.
//
// El redondeo es lo que hace el suelo noticeable: por debajo de medio minuto
// -- y por debajo de cero -- no se reserva nada, y la entrada se queda sin día de
// inicio, que es lo que el calendario usa para no pintar una barra que no
// representa nada. Antes ese suelo era epsilon en días, y comparar coma flotante
// contra epsilon no se puede probar por los dos lados.
func minutosDe(est float64) int {
	return int(math.Round(est * minutosPorDia))
}

// siguienteDia avanza un día natural y lo pasa a laborable, saltando fines de
// semana y días no laborables de esa persona.
func siguienteDia(day time.Time, assignee string, lookup map[string][]offRange) time.Time {
	return nextWorkingDay(day.AddDate(0, 0, 1), assignee, lookup)
}

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
