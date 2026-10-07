package tui

import (
	"testing"

	"tsk/internal/model"
)

// The people modal's roster is built with a pure function so that it can be
// checked without a model or a database. These are the rules that matter:
// who appears, what is counted and in which order.

func assigneeTasks(fixture ...model.Task) []model.Task { return fixture }
func offdays(fixture ...model.OffDay) []model.OffDay   { return fixture }

var rosterTasks = assigneeTasks(
	model.Task{ID: 1, Assignee: "@john", Status: "todo"},
	model.Task{ID: 2, Assignee: "@john", Status: "doing"},
	model.Task{ID: 3, Assignee: "@john", Status: "done"},
	model.Task{ID: 4, Assignee: "@margo", Status: "todo"},
	model.Task{ID: 5, Assignee: "", Status: "todo"},
	model.Task{ID: 6, Assignee: model.UnassignedAssignee, Status: "todo"},
)

var rosterOffDays = offdays(
	model.OffDay{ID: 1, Assignee: "@john", StartDate: "2026-07-01", EndDate: "2026-07-02"},
	model.OffDay{ID: 2, Assignee: "@john", StartDate: "2026-08-01", EndDate: "2026-08-01"},
	model.OffDay{ID: 3, Assignee: "@ann", StartDate: "2026-09-01", EndDate: "2026-09-02"},
)

func rosterByName(roster []assigneeSummary, name string) *assigneeSummary {
	for i := range roster {
		if roster[i].Name == name {
			return &roster[i]
		}
	}
	return nil
}

// "Me" is always in the roster even if nobody has anything assigned: it is
// the value new tasks are attributed to.
func TestBuildAssigneeRosterAlwaysHasMe(t *testing.T) {
	roster := buildAssigneeRoster(nil, nil)
	if len(roster) != 1 {
		t.Fatalf("with nothing assigned the roster has %d entries, want 1 (only Me)", len(roster))
	}
	if roster[0].Name != "Me" {
		t.Errorf("name = %q, want Me", roster[0].Name)
	}
	if roster[0].Total != 0 || roster[0].Active != 0 || roster[0].OffDayCount != 0 {
		t.Errorf("Me = %+v, want all zeros", roster[0])
	}
}

// With no assignee there is nobody to attribute anything to: empty and
// "unassigned" are skipped, in tasks and in off-days.
func TestBuildAssigneeRosterSkipsUnassigned(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	if s := rosterByName(roster, ""); s != nil {
		t.Errorf("a task with no assignee must not create an entry: %+v", s)
	}
	if s := rosterByName(roster, model.UnassignedAssignee); s != nil {
		t.Errorf("unassigned must not create an entry: %+v", s)
	}
	// @ann only has one off-day and no task: she still appears.
	if s := rosterByName(roster, "@ann"); s == nil {
		t.Error("whoever has an off-day must appear even without tasks")
	} else if s.Total != 0 || s.OffDayCount != 1 {
		t.Errorf("@ann = %+v, want 0 tasks and 1 off-day", *s)
	}
}

// The totals and the active ones are counted separately.
func TestBuildAssigneeRosterCounts(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)

	john := rosterByName(roster, "@john")
	if john == nil {
		t.Fatal("@john does not appear")
	}
	if john.Total != 3 {
		t.Errorf("@john.Total = %d, want 3", john.Total)
	}
	if john.Active != 2 {
		t.Errorf("@john.Active = %d, want 2 (the done one does not count)", john.Active)
	}
	if john.OffDayCount != 2 {
		t.Errorf("@john.OffDayCount = %d, want 2", john.OffDayCount)
	}

	margo := rosterByName(roster, "@margo")
	if margo == nil {
		t.Fatal("@margo does not appear")
	}
	if margo.Total != 1 || margo.Active != 1 || margo.OffDayCount != 0 {
		t.Errorf("@margo = %+v, want 1/1/0", *margo)
	}
}

// The roster comes out sorted by name: the index is the position and it has
// to be stable between renders even if the map behind it changes order.
func TestBuildAssigneeRosterSorted(t *testing.T) {
	for range 20 {
		roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
		for i := 1; i < len(roster); i++ {
			if roster[i-1].Name >= roster[i].Name {
				t.Fatalf("roster out of order at %d: %q before %q (%v)",
					i, roster[i-1].Name, roster[i].Name, roster)
			}
		}
	}
}

// Me appears sorted among the others, not always first.
func TestBuildAssigneeRosterMeIsSorted(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	names := make([]string, len(roster))
	for i, s := range roster {
		names[i] = s.Name
	}
	want := []string{"@ann", "@john", "@margo", "Me"}
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

// Off-days without an assignee do not create new entries either.
func TestBuildAssigneeRosterOffDayWithoutAssignee(t *testing.T) {
	ods := offdays(
		model.OffDay{ID: 1, Assignee: "", StartDate: "2026-07-01", EndDate: "2026-07-01"},
		model.OffDay{ID: 2, Assignee: model.UnassignedAssignee, StartDate: "2026-07-01", EndDate: "2026-07-01"},
	)
	roster := buildAssigneeRoster(nil, ods)
	if len(roster) != 1 {
		t.Errorf("roster = %+v, want only Me", roster)
	}
}

// tasksForAssignee only returns the non-closed ones of that person.
func TestTasksForAssignee(t *testing.T) {
	got := tasksForAssignee(rosterTasks, "@john")
	if len(got) != 2 {
		t.Fatalf("got %d tasks, want 2 (the done one stays out)", len(got))
	}
	for _, task := range got {
		if task.Assignee != "@john" {
			t.Errorf("a task of %q slipped in", task.Assignee)
		}
		if !task.IsActive() {
			t.Errorf("a closed task slipped in: %+v", task)
		}
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("the insertion order was not respected: %d, %d", got[0].ID, got[1].ID)
	}
}

func TestTasksForAssigneeNoMatch(t *testing.T) {
	if got := tasksForAssignee(rosterTasks, "@nobody"); len(got) != 0 {
		t.Errorf("got %d, want 0 for a person with no tasks", len(got))
	}
	if got := tasksForAssignee(nil, "@john"); len(got) != 0 {
		t.Errorf("got %d, want 0 with no tasks", len(got))
	}
}

// tasksForAssignee filters by name AND ONLY skips those with no
// assignee, unlike buildAssigneeRoster. The asymmetry is real and is
// pinned on purpose: the only caller passes the name selected in the modal,
// which comes from the roster, and there is always at least "Me" in it.
// Adding the filter would be one more condition with no case exercising it.
func TestTasksForAssigneeMatchesEmptyAssignee(t *testing.T) {
	if got := tasksForAssignee(rosterTasks, ""); len(got) != 1 {
		t.Errorf("name filter with the empty one = %d tasks, want 1 (the fixture's)", len(got))
	}
	if got := tasksForAssignee(rosterTasks, model.UnassignedAssignee); len(got) != 1 {
		t.Errorf("name filter with unassigned = %d tasks, want 1", len(got))
	}
	// But those people are not in the roster, so they never become the
	// selected name.
	if s := rosterByName(buildAssigneeRoster(rosterTasks, nil), ""); s != nil {
		t.Errorf("the empty one must not appear in the roster: %+v", s)
	}
}

// offDaysForAssignee keeps the load order, which is how the DB gives them.
func TestOffDaysForAssignee(t *testing.T) {
	got := offDaysForAssignee(rosterOffDays, "@john")
	if len(got) != 2 {
		t.Fatalf("got %d off-days, want 2", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("the order changed: %d, %d", got[0].ID, got[1].ID)
	}
	for _, o := range got {
		if o.Assignee != "@john" {
			t.Errorf("an off-day of %q slipped in", o.Assignee)
		}
	}
}

func TestOffDaysForAssigneeNoMatch(t *testing.T) {
	if got := offDaysForAssignee(rosterOffDays, "@nobody"); len(got) != 0 {
		t.Errorf("got %d, want 0", len(got))
	}
	if got := offDaysForAssignee(nil, "@john"); len(got) != 0 {
		t.Errorf("got %d, want 0 with no off-days", len(got))
	}
}

// nameAt is the guard before roster[idx]: an out-of-range index gives ""
// instead of blowing up, and that is the value the callers treat as "nobody".
func TestNameAt(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{"first", 0, "@ann"},
		{"the middle one", 2, "@margo"},
		{"last", len(roster) - 1, "Me"},
		{"one past", len(roster), ""},
		{"way past", 99, ""},
		{"negative", -1, ""},
		{"empty roster", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := roster
			if tt.name == "empty roster" {
				r = nil
			}
			if got := nameAt(r, tt.idx); got != tt.want {
				t.Errorf("nameAt(%d) = %q, want %q", tt.idx, got, tt.want)
			}
		})
	}
}

// nameAt and the roster do not get out of sync: the name at position i is
// the roster's at that position, for the whole range.
func TestNameAtMatchesRoster(t *testing.T) {
	roster := buildAssigneeRoster(rosterTasks, rosterOffDays)
	for i := range roster {
		if got := nameAt(roster, i); got != roster[i].Name {
			t.Errorf("nameAt(%d) = %q, want %q", i, got, roster[i].Name)
		}
	}
}
