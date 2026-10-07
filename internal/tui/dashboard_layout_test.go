package tui

import (
	"testing"

	"tsk/internal/model"
)

// These functions were extracted from the Dashboard render, which repeated the
// same count over m.tasks five times with its own project filter inside. Since
// they are pure, they are tested without a model, without a database and without rendering anything.

func dashTasks(fixture ...model.Task) []model.Task { return fixture }

var dashFixture = dashTasks(
	model.Task{ID: 1, ProjectName: "api", Assignee: "@john", Status: "todo"},
	model.Task{ID: 2, ProjectName: "api", Assignee: "@john", Status: "doing"},
	model.Task{ID: 3, ProjectName: "api", Assignee: "@margo", Status: "done"},
	model.Task{ID: 4, ProjectName: "web", Assignee: "@john", Status: "todo"},
	model.Task{ID: 5, ProjectName: "web", Assignee: "@ann", Status: "cancelled"},
	model.Task{ID: 6, ProjectName: "web", Assignee: "@ann", Status: "reviewing"},
)

// dashProjectTasks counts the active ones of a project; "" counts the whole thing.
func TestDashProjectTasks(t *testing.T) {
	tests := []struct {
		name    string
		project string
		want    int
	}{
		{"api has 2 active of 3", "api", 2},
		{"web has 2 active of 3", "web", 2},
		{"no project counts all the active ones", "", 4},
		{"nonexistent project", "nope", 0},
		{"no tasks", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := dashFixture
			if tt.name == "no tasks" {
				tasks = nil
			}
			if got := dashProjectTasks(tasks, tt.project); got != tt.want {
				t.Errorf("dashProjectTasks(%q) = %d, want %d", tt.project, got, tt.want)
			}
		})
	}
}

// It only counts the active ones: done and cancelled stay out even if they
// belong to the project. That is what sets this count apart from the status one.
func TestDashProjectTasksOnlyActive(t *testing.T) {
	all := dashProjectTasks(dashFixture, "")
	_, done, cancelled, _ := dashStatusCounts(dashFixture, "")
	if all+done+cancelled != len(dashFixture) {
		t.Errorf("active(%d) + done(%d) + cancelled(%d) = %d, want %d tasks",
			all, done, cancelled, all+done+cancelled, len(dashFixture))
	}
}

// dashStatusCounts: done and cancelled apart, everything else active.
func TestDashStatusCounts(t *testing.T) {
	tests := []struct {
		name                             string
		project                          string
		wantActive, wantDone, wantCancel int
	}{
		{"the whole set", "", 4, 1, 1},
		{"only api", "api", 2, 1, 0},
		{"only web", "web", 2, 0, 1},
		{"nonexistent project", "nope", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, done, cancelled, byStatus := dashStatusCounts(dashFixture, tt.project)
			if active != tt.wantActive || done != tt.wantDone || cancelled != tt.wantCancel {
				t.Errorf("dashStatusCounts(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tt.project, active, done, cancelled, tt.wantActive, tt.wantDone, tt.wantCancel)
			}
			// The byStatus map carries ALL statuses, not only active ones:
			// they are the Overview bars, one per workflow status.
			sum := 0
			for _, n := range byStatus {
				sum += n
			}
			var want int
			switch tt.project {
			case "api", "web":
				want = 3
			case "nope":
				want = 0
			default:
				want = 6
			}
			if sum != want {
				t.Errorf("byStatus sums %d, want %d (%v)", sum, want, byStatus)
			}
		})
	}
}

// The byStatus map includes the done and cancelled statuses, not only the
// active ones: the Overview bars are painted for the whole workflow.
func TestDashStatusCountsMapIncludesDoneAndCancelled(t *testing.T) {
	_, _, _, byStatus := dashStatusCounts(dashFixture, "")
	for _, want := range []string{"todo", "doing", "done", "cancelled", "reviewing"} {
		if _, ok := byStatus[want]; !ok {
			t.Errorf("byStatus does not have %q: %v", want, byStatus)
		}
	}
	if byStatus["done"] != 1 || byStatus["cancelled"] != 1 {
		t.Errorf("done/cancelled wrongly counted: %v", byStatus)
	}
}

// An unknown status counts as active: it is neither in done nor in cancelled.
func TestDashStatusCountsUnknownStatusIsActive(t *testing.T) {
	tasks := dashTasks(model.Task{ProjectName: "api", Status: "backlog"})
	active, done, cancelled, byStatus := dashStatusCounts(tasks, "")
	if active != 1 || done != 0 || cancelled != 0 {
		t.Errorf("a status of the workflow itself must count as active, it gave (%d,%d,%d)", active, done, cancelled)
	}
	if byStatus["backlog"] != 1 {
		t.Errorf("byStatus[backlog] = %d, want 1", byStatus["backlog"])
	}
}

// dashAssigneeCounts: total and active subcount per person.
func TestDashAssigneeCounts(t *testing.T) {
	total, active := dashAssigneeCounts(dashFixture, "")
	if total["@john"] != 3 {
		t.Errorf("total[@john] = %d, want 3", total["@john"])
	}
	// The three of @john are active; the only done one in the fixture is @margo's.
	if active["@john"] != 3 {
		t.Errorf("active[@john] = %d, want 3 (none of theirs is closed)", active["@john"])
	}
	if total["@margo"] != 1 || active["@margo"] != 0 {
		t.Errorf("@margo = %d total / %d active, want 1/0", total["@margo"], active["@margo"])
	}
	if total["@ann"] != 2 || active["@ann"] != 1 {
		t.Errorf("@ann = %d total / %d active, want 2/1 (one is cancelled)", total["@ann"], active["@ann"])
	}
}

// The project filter trims both counts at once, not only the total.
func TestDashAssigneeCountsFilterByProject(t *testing.T) {
	total, active := dashAssigneeCounts(dashFixture, "api")
	if total["@john"] != 2 || active["@john"] != 2 {
		t.Errorf("in api @john = %d/%d, want 2/2", total["@john"], active["@john"])
	}
	if _, ok := total["@ann"]; ok {
		t.Errorf("@ann has no tasks in api, but appears: %v", total)
	}
}

// No person can have more active ones than totals.
func TestDashAssigneeCountsActiveNeverExceedsTotal(t *testing.T) {
	total, active := dashAssigneeCounts(dashFixture, "")
	for person, n := range active {
		if n > total[person] {
			t.Errorf("%s: %d active over %d total", person, n, total[person])
		}
	}
}

// dashRowsAvailable: what is left after the used ones and the header. The floor is 0.
func TestDashRowsAvailable(t *testing.T) {
	tests := []struct {
		name                       string
		colLines, used, headerRows int
		want                       int
	}{
		{"with plenty of room", 20, 5, 3, 12},
		{"exact", 10, 5, 3, 2},
		{"exactly one row fits", 9, 5, 3, 1},
		{"no rows", 8, 5, 3, 0},
		{"overshooting gives 0", 3, 5, 3, 0},
		{"heavily overshooting gives 0", 0, 50, 3, 0},
		// With negative heights the subtraction gives a positive number: the
		// floor protects against the real case (the column ran out of space),
		// not against impossible entries.
		{"subtractions that give a positive", -5, -8, -2, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashRowsAvailable(tt.colLines, tt.used, tt.headerRows)
			if got != tt.want {
				t.Errorf("dashRowsAvailable(%d, %d, %d) = %d, want %d",
					tt.colLines, tt.used, tt.headerRows, got, tt.want)
			}
			if got < 0 {
				t.Errorf("it can never be negative, it gave %d", got)
			}
		})
	}
}

// A budget of 0 means "nothing else fits", and that is why the floor is 0
// and not 1: with 1 the block would grow one line beyond the calculated height.
func TestDashRowsAvailableZeroMeansNothing(t *testing.T) {
	if got := dashRowsAvailable(8, 5, 3); got != 0 {
		t.Fatalf("budget 0 expected, it gave %d", got)
	}
	if got := dashRowsAvailable(9, 5, 3); got != 1 {
		t.Fatalf("budget 1 expected, it gave %d", got)
	}
}

// dashColumnWidths: two equal columns with a gap of 1 in between.
func TestDashColumnWidths(t *testing.T) {
	tests := []struct {
		name                string
		innerW              int
		wantLeft, wantRight int
	}{
		{"normal width", 78, 38, 38},
		{"odd", 79, 38, 38},
		{"even", 80, 39, 39},
		{"narrow", 10, 4, 4},
		{"minimally useful", 6, 2, 2},
		{"insufficient", 4, 1, 1},
		{"zero", 0, -1, -1},
		{"negative", -10, -6, -6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, right := dashColumnWidths(tt.innerW)
			if left != tt.wantLeft || right != tt.wantRight {
				t.Errorf("dashColumnWidths(%d) = (%d,%d), want (%d,%d)",
					tt.innerW, left, right, tt.wantLeft, tt.wantRight)
			}
			if left != right {
				t.Errorf("the columns must be equal, it gave %d and %d", left, right)
			}
		})
	}
}
