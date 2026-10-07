package model

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// dateLayout is the canonical Gantt date format (YYYY-MM-DD).
const dateLayout = "2006-01-02"

// UnassignedAssignee is the assignee value that is not projected into the Gantt.
const UnassignedAssignee = "unassigned"

// OffDay is a non-working day of a person: vacation, holiday or
// absence. The [StartDate, EndDate] range is inclusive at both ends; a
// single day uses the same date in both fields.
type OffDay struct {
	ID        int64  `json:"id"`
	Assignee  string `json:"assignee"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Note      string `json:"note,omitempty"`
}

// ScheduleEntry is a task projected onto the calendar.
type ScheduleEntry struct {
	Task              Task    `json:"task"`
	Start             string  `json:"start"`
	End               string  `json:"end"`
	Estimate          float64 `json:"estimate"`
	EstimateDefaulted bool    `json:"estimate_defaulted"`
}

// AssigneeSchedule is the projected sequential queue of one person.
type AssigneeSchedule struct {
	Assignee string          `json:"assignee"`
	Entries  []ScheduleEntry `json:"entries"`
	End      string          `json:"end"`
}

// Schedule is the full projection: one queue per person plus the tasks without
// an assignee (which cannot be scheduled).
type Schedule struct {
	Start      string             `json:"start"`
	Assignees  []AssigneeSchedule `json:"assignees"`
	Unassigned []Task             `json:"unassigned"`
}

// ParseDate converts "YYYY-MM-DD" to a time.Time in UTC.
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, s, time.UTC)
}

// FormatDate formats a time.Time as "YYYY-MM-DD".
func FormatDate(t time.Time) string {
	return t.Format(dateLayout)
}

// IsUnassigned tells whether a task has no assignee and therefore is not scheduled.
func IsUnassigned(assignee string) bool {
	return assignee == "" || assignee == UnassignedAssignee
}

// IsOffDay tells whether day is a non-working day for assignee: a weekend or a
// registered off-day. It is used to paint the calendar, not to schedule.
func IsOffDay(offdays []OffDay, assignee string, day time.Time) bool {
	day = truncateDay(day)
	if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return true
	}
	return isOff(day, assignee, offRangesByAssignee(offdays))
}

// FormatEstimate formats an estimate in days: integer with no decimals
// ("2d") and a fraction with its exact value ("0.5d").
func FormatEstimate(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10) + "d"
	}
	return strconv.FormatFloat(v, 'f', -1, 64) + "d"
}

// WeekOfMonthLabel labels the week starting on Monday t by its number
// within the month, glued to the month in uppercase, e.g. "1SEP" or "3OCT". The
// week and the month are determined by that week's Thursday (the week that
// contains day 1 is week 1), so the month lines up with where it really falls.
func WeekOfMonthLabel(t time.Time) string {
	thu := t.AddDate(0, 0, 3) // Monday + 3 = Thursday
	return fmt.Sprintf("%d%s", (thu.Day()-1)/7+1, strings.ToUpper(thu.Format("Jan")))
}

// offRange is a range of non-working days already parsed to time.Time.
type offRange struct {
	start, end time.Time
}

// BuildSchedule projects a sequential queue per person: each person does
// one task at a time, in the order they arrive (priority -> status -> id), and
// consumes working days (Monday to Friday, except their off-days). There are
// no dependencies between tasks and no parallelism within one person.
//
// Done/cancelled tasks are ignored; tasks without an assignee (empty or
// "unassigned") are returned in Unassigned without scheduling. A task with no estimate
// uses defaultEstimate and is flagged with EstimateDefaulted.
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
		// free is the unoccupied minutes of the day at the cursor. It carries
		// over from one task to the next on purpose: two half-day tasks are packed
		// into the same day, which is exactly what the gantt is about.
		free := minutesPerDay

		for _, t := range queues[assignee] {
			est := t.Estimate
			def := false
			if est <= 0 {
				est = defaultEstimate
				def = true
			}

			// The arithmetic runs in MINUTES, not in day fractions. With floating
			// point, "a whole day remains" and "nothing remains" could only be
			// told apart by comparing against epsilon, and since no estimate
			// ever landed exactly on epsilon, no test could kill those two
			// comparisons: they were the kind of line that seems to decide
			// something and decides nothing. In integers the same edge is a real
			// `== 0`, hit by any round estimate and that a test
			// does hit from both sides.
			//
			// The change alters no result: 1440 minutes are one day, so
			// the split comes out the same. What changes is that now it can be
			// verified.
			total := estimateMinutes(est)
			var entryStart time.Time
			if total > 0 {
				// The jump for a full day happens BEFORE setting the start: if the day
				// at the cursor was already consumed by the previous task, this one
				// starts on the next working day, not on a full one. That is why
				// it is here and not inside the loop, which only advances when
				// there is work left for a half task.
				if free == 0 {
					cursor = nextDay(cursor, assignee, lookup)
					free = minutesPerDay
				}
				entryStart = cursor

				// The split is a `for range` over the number of FULL DAYS,
				// not a loop that decrements a balance. The count does not depend on
				// the balance reaching zero, so no mutation of the condition
				// can leave it spinning: with `remaining >= 0` the balance stayed
				// at zero, free at zero, and nextDay advanced the cursor
				// forever. A `for range` over an integer that does not decrement
				// inside cannot hang.
				//
				// The rest of the day is consumed by hand, after the full days.
				// It was what remained as the last lap of the previous loop, and it stays
				// here because that way the loop has no exit: you always leave through
				// the end.
				remaining := free
				for range divRound(total, minutesPerDay) {
					consumed := min(remaining, total)
					remaining -= consumed
					total -= consumed
					if total > 0 {
						cursor = nextDay(cursor, assignee, lookup)
						remaining = minutesPerDay
					}
				}
				free = remaining
			}

			s.Entries = append(s.Entries, ScheduleEntry{
				Task:              t,
				Start:             FormatDate(entryStart),
				End:               FormatDate(cursor),
				Estimate:          est,
				EstimateDefaulted: def,
			})
			// End of the assignee = end of their last entry. It is assigned inside
			// the loop instead of with `if n := len(s.Entries); n > 0`: the
			// queue is never empty (every assignee comes from >= 1 task), so
			// that `> 0` was always true and its mutant was indistinguishable.
			s.End = s.Entries[len(s.Entries)-1].End
		}
		sched.Assignees = append(sched.Assignees, s)
	}
	return sched
}

// FilterSchedule returns a copy of s with only the tasks that satisfy keep.
// It is a VIEW filter: it recalculates neither dates nor queues, so a hidden task
// still holds its place in the person's timeline (there may be
// gaps between the visible tasks). People left without tasks are
// dropped.
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

// nextWorkingDay advances day to the first working day for assignee:
// Saturday and Sunday are always non-working, plus their off-days.
// minutesPerDay is the capacity of a working day, in minutes.
const minutesPerDay = 24 * 60

// divRound divides rounding to the nearest integer. It is used for the number of
// full days of an estimate: a half-day estimate is 0 days and the rest
// is consumed separately, and one of a day and a half is 1 full day plus half a minute.
//
// The divisor is always minutesPerDay, so the zero guard is not
// needed: with it, `b < 1` had an edge (b == 1) that no test reaches, and
// that is why its mutant was indistinguishable. The divisor is a parameter so that
// tests can pass others, and it is documented that in production it is always 1440.
func divRound(a, b int) int {
	return (a + b/2) / b
}

// estimateMinutes converts an estimate in days to minutes, rounding.
//
// The rounding is what makes the floor noticeable: below half a minute
// -- and below zero -- nothing is reserved, and the entry keeps no start
// day, which is what the calendar uses so as not to paint a bar that
// represents nothing. Before, that floor was epsilon in days, and comparing floating
// point against epsilon cannot be tested from both sides.
func estimateMinutes(est float64) int {
	return int(math.Round(est * minutesPerDay))
}

// nextDay advances one calendar day and turns it into a working one, skipping
// weekends and that person's non-working days.
func nextDay(day time.Time, assignee string, lookup map[string][]offRange) time.Time {
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

// isOff tells whether day falls within any off-day of assignee.
func isOff(day time.Time, assignee string, lookup map[string][]offRange) bool {
	for _, r := range lookup[assignee] {
		if !day.Before(r.start) && !day.After(r.end) {
			return true
		}
	}
	return false
}

// offRangesByAssignee groups off-days per person, ignoring ranges with
// invalid dates.
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

// truncateDay normalizes a time.Time to UTC midnight while keeping the local
// calendar day of the input.
func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
