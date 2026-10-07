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

// The Dashboard puts together three blocks with row budgets and a project filter.
// Its arithmetic is in the render and comparing "the word shows up" does not
// see it: a moved sign in the total changes the number, and a badly placed cap
// condition changes how many rows fit without removing any word from the screen.

// dashRender returns the rendered dashboard, without colors.
func dashRender(t *testing.T, m *Model) string {
	t.Helper()
	m.currentView = viewDashboard
	m.width = 140
	m.height = 40
	return ansi.Strip(m.renderDashboard(40))
}

// newDashModel starts from the test model, with the project filter pointing
// at project and no status filter, so that the truncation is observable.
//
// The Dashboard's project truncation is NOT the general filter: it comes out
// of its own project picker, the "[api(3)]" of the line above. It is chosen
// with dashProjectIdx, and -1 is "all". By default New leaves it on the first
// project, so without touching it the Overview would always count a single
// project and the general filter would have no effect at all on this view.
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
	t.Fatalf("the fixture does not have the project %q", project)
	return m
}

// The Overview total is the sum of active, completed and cancelled. The
// cancelled ones go with a plus sign: the total is "all tasks", and with only
// active and done ones a cancelled task would be counted by its absence.
func TestDashTotalCountsEveryTask(t *testing.T) {
	m := newDashModel(t, "")
	// The three addends have to be there at once: with one at zero, changing a
	// sign or an operator in the sum leaves the same number and the test does not see it.
	mustCreateTask(t, m.database, "api", "cancelled", "", "@john", 3, model.CancelledStatus)
	mustCreateTask(t, m.database, "api", "finished", "", "@john", 2, model.DoneStatus)
	reloadTasks(t, m)
	active, done, cancelled, _ := dashStatusCounts(m.tasks, "")
	total := active + done + cancelled
	if active == 0 || done == 0 || cancelled == 0 {
		t.Fatalf("the fixture needs all three addends at once, it has %d/%d/%d",
			active, done, cancelled)
	}

	out := dashRender(t, m)
	if want := "  Total         " + strconv.Itoa(total); !strings.Contains(out, want) {
		t.Errorf("the total does not say %q:\n%s", want, out)
	}

	// The projects line lists the projects with their count. Without this, a
	// condition that always fired -- of the kind "if there are no items, show
	// (none)" badly placed -- would pass the total test.
	if !strings.Contains(out, "api(") || !strings.Contains(out, "web(") {
		t.Errorf("the projects line does not list the projects:\n%s", out)
	}
	if strings.Contains(out, "(none)") {
		t.Errorf("(none) came out with projects in the list:\n%s", out)
	}
}

// On the projects line, the selected one goes between brackets and
// highlighted. With "all" selected none is between brackets, and with a
// project chosen, exactly that one.
//
// The brackets are what distinguish the condition from its negated version:
// marking all or none leaves the names on screen the same, so a test that
// looked for "api(" would not notice it.
func TestDashSelectedProjectIsBracketed(t *testing.T) {
	allSelected := newDashModel(t, "")
	if n := countBracketed(dashRender(t, allSelected)); n != 0 {
		t.Errorf("with \"all\" selected there are %d bracketed projects, want 0", n)
	}

	m := newDashModel(t, "api")
	out := dashRender(t, m)
	if n := countBracketed(out); n != 1 {
		t.Errorf("with api selected there are %d bracketed projects, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "[api(") {
		t.Errorf("the bracketed project is not the selected one:\n%s", out)
	}
	if strings.Contains(out, "[web(") {
		t.Errorf("a project that was not selected appeared bracketed:\n%s", out)
	}
}

// countBracketed counts the bracket pairs of the projects line.
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

// With a project selected in the Dashboard, the total counts only that
// project. The fixture spreads four tasks between "api" and "web", so the
// filtered number and the global one do not match, and a total that forgot
// the project would show.
func TestDashTotalRespectsSelectedProject(t *testing.T) {
	m := newDashModel(t, "api")

	a, d, c, _ := dashStatusCounts(m.tasks, "api")
	api := a + d + c
	a, d, c, _ = dashStatusCounts(m.tasks, "")
	total := a + d + c
	if api == total {
		t.Fatalf("the fixture does not distinguish: api=%d, all=%d", api, total)
	}

	out := dashRender(t, m)
	if !strings.Contains(out, "  Total         "+strconv.Itoa(api)) {
		t.Errorf("with api selected the total is not %d:\n%s", api, out)
	}
	if strings.Contains(out, "  Total         "+strconv.Itoa(total)) {
		t.Errorf("the total is the one of all projects (%d), not api's (%d):\n%s", total, api, out)
	}
}

// A status with no tasks does not show: the Overview lists the workflow's
// ones with a bar, and an empty bar says nothing.
func TestDashOverviewSkipsEmptyStatuses(t *testing.T) {
	m := newDashModel(t, "")

	out := dashRender(t, m)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "todo" || fields[0] == "backlog" {
			continue
		}
		if len(fields) >= 2 && fields[1] == "0" {
			t.Errorf("a status with zero tasks came out: %q", line)
		}
	}
	if !strings.Contains(out, "backlog") {
		t.Errorf("the status with tasks did not come out:\n%s", out)
	}
}

// A status name longer than 14 columns is truncated, and the count goes
// behind in its place: without the truncation the bar would throw the table off.
func TestDashOverviewClipsLongStatusNames(t *testing.T) {
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	longStatus := "awaiting-review"
	mustCreateProject(t, database, "long", []string{longStatus, model.DoneStatus})
	mustCreateTask(t, database, "long", "task", "", "@john", 1, longStatus)

	m := New(database, config.Defaults())
	m.width, m.height = 140, 40
	reloadTasks(t, &m)
	reloadProjects(t, &m)
	m.filterStatus = ""

	out := dashRender(t, &m)
	// The truncation is 14 bytes of the whole name: "awaiting-revie" out of 15.
	if !strings.Contains(out, "awaiting-revie 1") {
		t.Errorf("the status clipped to 14 was not seen:\n%s", out)
	}
	if strings.Contains(out, longStatus) {
		t.Errorf("the %d-character status came out unclipped:\n%s", len(longStatus), out)
	}
}

// The Active panel comes out sorted and with the header; with the filter on,
// only the project's tasks.
func TestDashActiveRespectsSelectedProject(t *testing.T) {
	m := newDashModel(t, "api")

	out := dashRender(t, m)
	if !strings.Contains(out, "Active") {
		t.Fatalf("the Active panel is not visible:\n%s", out)
	}
	if strings.Contains(out, "Fix checkout") {
		t.Errorf("a task of the filtered-out project came out:\n%s", out)
	}
	if !strings.Contains(out, "N+1") {
		t.Errorf("no task of the filtered project came out:\n%s", out)
	}
}

// The Active panel is bounded by the available height: with many tasks and
// little height it cuts at the bottom instead of overflowing the box.
func TestDashActiveTruncatesToBudget(t *testing.T) {
	m := newTestModel(t)
	for range 20 {
		mustCreateTask(t, m.database, "api", "extra task", "", "@john", 1, "todo")
	}
	reloadTasks(t, m)
	m.filterProject = "api"
	m.filterStatus = ""
	m.currentView = viewDashboard
	m.width, m.height = 140, 40

	wide := strings.Count(ansi.Strip(m.renderDashboard(40)), "\n")
	narrow := strings.Count(ansi.Strip(m.renderDashboard(14)), "\n")
	if narrow >= wide {
		t.Errorf("at height 14 there are %d lines and at 40 there are %d: the height does not bound",
			narrow, wide)
	}
	if narrow == 0 {
		t.Error("at height 14 nothing comes out")
	}
}

// The team comes out sorted alphabetically, which is what makes two renders
// in a row look the same.
func TestDashTeamIsSorted(t *testing.T) {
	m := newTestModel(t)
	for _, name := range []string{"@zeta", "@alpha"} {
		mustCreateTask(t, m.database, "api", "task of "+name, "", name, 1, "todo")
	}
	reloadTasks(t, m)
	m.filterProject = "api"
	m.filterStatus = ""

	out := dashRender(t, m)
	iAlpha := strings.Index(out, "@alpha")
	iZeta := strings.Index(out, "@zeta")
	if iAlpha < 0 || iZeta < 0 {
		t.Fatalf("people are missing from the team panel:\n%s", out)
	}
	if iAlpha > iZeta {
		t.Errorf("@zeta comes before @alpha: the team is not sorted\n%s", out)
	}
}
