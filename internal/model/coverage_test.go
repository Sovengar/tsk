package model

import (
	"testing"
	"time"
)

// PriorityLabel and PriorityBar have a `default` that the existing tests did not
// touch, and the three non-priority cases returned different labels.
func TestPriorityLabelsCoverEveryPriority(t *testing.T) {
	for _, tc := range []struct {
		prio   int
		label  string
		short  string
		bar    string
		isLast bool
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

// ParseTagsJSON never fails: a JSON that is not a list of strings is the
// same as having no tags. Losing a task's tags is bad, but losing the whole
// task from the view would be worse.
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
		{in: `["one"]`, want: []string{"one"}},
		{in: `["  one  ","TWO","two"]`, want: []string{"one", "two"}},
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

// ParseWorkflowJSON does return an error, unlike ParseTagsJSON: an unreadable
// workflow is corrupt data that has to be reported, not something that can be
// replaced by a default without anyone noticing.
func TestParseWorkflowJSONReportsBadInput(t *testing.T) {
	if _, err := ParseWorkflowJSON(`["backlog"`); err == nil {
		t.Error("ParseWorkflowJSON with truncated JSON: want error")
	}
	got, err := ParseWorkflowJSON(`["backlog","done"]`)
	if err != nil {
		t.Fatalf("ParseWorkflowJSON: %v", err)
	}
	if len(got) != 2 || got[0] != "backlog" || got[1] != "done" {
		t.Errorf("ParseWorkflowJSON = %v, want [backlog done]", got)
	}
}

// Off-days with unreadable dates are dropped instead of taking down the whole
// calendar, and a range written backwards is normalized. Both are things a
// user can type by accident.
func TestOffRangesSkipBadDatesAndNormalizeReversedRanges(t *testing.T) {
	offdays := []OffDay{
		{Assignee: "@john", StartDate: "2026-03-02", EndDate: "2026-03-01", Note: "backwards"},
		{Assignee: "@john", StartDate: "not-a-date", EndDate: "2026-03-05", Note: "bad start"},
		{Assignee: "@john", StartDate: "2026-03-06", EndDate: "not-a-date", Note: "bad end"},
		{Assignee: "@margo", StartDate: "2026-03-03", EndDate: "2026-03-04", Note: "good"},
	}

	got := offRangesByAssignee(offdays)

	john, ok := got["@john"]
	if !ok {
		t.Fatalf("no ranges for @john: %v", got)
	}
	if len(john) != 1 {
		t.Fatalf("@john has %d ranges, want 1 (the two unreadable ones are dropped)", len(john))
	}
	if want := (time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)); !john[0].start.Equal(want) {
		t.Errorf("the range starts on %s, want %s: a reversed range is normalized",
			john[0].start.Format("2006-01-02"), want.Format("2006-01-02"))
	}

	if len(got["@margo"]) != 1 {
		t.Errorf("@margo has %d ranges, want 1", len(got["@margo"]))
	}
}

// The calendar filter has to let unassigned tasks through:
// they are the ones that do not appear in any person's queue, so if the filter
// removed them they would vanish from the view, plain and simple.
func TestFilterScheduleKeepsUnassignedTasks(t *testing.T) {
	newTask := func(assignee string) Task {
		return Task{Assignee: assignee, Priority: PriorityHigh}
	}
	schedule := &Schedule{
		Start: "2026-03-01",
		Assignees: []AssigneeSchedule{{
			Assignee: "@john",
			Entries:  []ScheduleEntry{{Task: newTask("@john")}},
		}},
		Unassigned: []Task{newTask(""), newTask(""), newTask("")},
	}

	filtered := FilterSchedule(schedule, func(Task) bool { return true })

	if len(filtered.Unassigned) != 3 {
		t.Errorf("%d tasks left unassigned, want 3", len(filtered.Unassigned))
	}
	if len(filtered.Assignees) != 1 || len(filtered.Assignees[0].Entries) != 1 {
		t.Errorf("@john's queue does not survive intact: %+v", filtered.Assignees)
	}

	// And a filter that rejects everything empties both lists, leaving no people
	// with an empty queue hanging around.
	filtered = FilterSchedule(schedule, func(Task) bool { return false })
	if len(filtered.Unassigned) != 0 || len(filtered.Assignees) != 0 {
		t.Errorf("with a filter that rejects everything, %d unassigned remain and %d queues",
			len(filtered.Unassigned), len(filtered.Assignees))
	}

	// The original is untouched: it is a copy, not an in-place truncation.
	if len(schedule.Unassigned) != 3 || len(schedule.Assignees) != 1 {
		t.Error("FilterSchedule modified the input schedule")
	}
}
