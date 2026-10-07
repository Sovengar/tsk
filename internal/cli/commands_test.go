package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// runJSON runs a command that must succeed and returns its JSON payload.
func runJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	out, code := run(t, args...)
	if code != 0 {
		t.Fatalf("Run(%v) exited with %d: %s", args, code, out)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("Run(%v) did not return JSON: %v (%q)", args, err, out)
	}
	return v
}

// wantError runs a command that must fail and checks the exit code.
func wantError(t *testing.T, args ...string) {
	t.Helper()
	if _, code := run(t, args...); code != 1 {
		t.Errorf("Run(%v) = %d, want 1", args, code)
	}
}

// seed creates a project with the default workflow.
func seed(t *testing.T, name string) {
	t.Helper()
	if _, code := run(t, "project", "add", name); code != 0 {
		t.Fatalf("project add %s failed", name)
	}
}

// addTask creates a task and returns its id.
func addTask(t *testing.T, project, title string, extra ...string) int64 {
	t.Helper()
	args := append([]string{"add", title, "--project", project}, extra...)
	payload := runJSON(t, args...)
	task, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("add %q without a task payload: %v", title, payload)
	}
	id, ok := task["id"].(float64)
	if !ok {
		t.Fatalf("task without an id: %v", task)
	}
	return int64(id)
}

func taskField(t *testing.T, payload map[string]any, field string) any {
	t.Helper()
	task, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("payload without a task: %v", payload)
	}
	return task[field]
}

// strList normalizes a JSON list field: the API returns null (not []) when the
// Go slice is nil, and both mean "empty" for whoever consumes the output.
func strList(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, item.(string))
		}
		return out
	default:
		return nil
	}
}

func str(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("value %v (%T) is not a string", v, v)
	}
	return s
}

// ---- project ----

func TestProjectAddAndList(t *testing.T) {
	withTempDB(t)

	seed(t, "api")
	payload := runJSON(t, "project", "list", "--json")
	projects, ok := payload["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("project list --json = %v", payload)
	}
	if name := str(t, projects[0].(map[string]any)["name"]); name != "api" {
		t.Errorf("name = %q, want api", name)
	}

	// Human output: header and total.
	out, _ := run(t, "project", "list")
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "api") {
		t.Errorf("human project list = %q", out)
	}
}

func TestProjectListEmpty(t *testing.T) {
	withTempDB(t)
	if out, _ := run(t, "project", "list"); !strings.Contains(out, "No projects registered.") {
		t.Errorf("empty list = %q", out)
	}
	if out, _ := run(t, "project", "list", "--archived"); !strings.Contains(out, "No archived projects.") {
		t.Errorf("empty archived list = %q", out)
	}
}

func TestProjectAddCustomWorkflow(t *testing.T) {
	withTempDB(t)
	runJSON(t, "project", "add", "api", "--workflow", "backlog,todo,reviewing,done")
	runJSON(t, "project", "add", "web", "--workflow", "backlog,todo,reviewing,done",
		"--list-order", "reviewing,todo,backlog,done")

	api := runJSON(t, "project", "show", "api", "--json")
	p := api["project"].(map[string]any)
	if p["name"] != "api" {
		t.Errorf("name = %v", p["name"])
	}
	if got := p["workflow"].([]any); len(got) != 4 || got[0] != "backlog" {
		t.Errorf("workflow = %v", got)
	}
	// Without an explicit --list-order the field comes out null, not []: CreateProject
	// receives a nil slice and json.Marshal serializes it as "null", which on
	// re-read leaves the slice as nil. That is the current API contract; a
	// consumer that assumes .list_order.length breaks with that null.
	if got := p["list_order"]; got != nil {
		t.Errorf("implicit list_order = %v, want null", got)
	}

	web := runJSON(t, "project", "show", "web", "--json")
	wp := web["project"].(map[string]any)
	if got := wp["list_order"].([]any); len(got) != 4 || got[0] != "reviewing" {
		t.Errorf("list_order = %v", got)
	}
}

func TestProjectShowHumanOutput(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	out, _ := run(t, "project", "show", "api")
	for _, want := range []string{"Project:  api", "Workflow:", "(workflow order)", "Archived: false"} {
		if !strings.Contains(out, want) {
			t.Errorf("project show = %q, missing %q", out, want)
		}
	}
	wantError(t, "project", "show", "nope")
}

func TestProjectUpdate(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")

	runJSON(t, "project", "update", "api", "--name", "api-v2")
	if payload := runJSON(t, "project", "show", "api-v2", "--json"); payload["project"].(map[string]any)["name"] != "api-v2" {
		t.Errorf("rename failed: %v", payload)
	}

	runJSON(t, "project", "update", "api-v2", "--workflow", "backlog,todo,done")
	runJSON(t, "project", "update", "api-v2", "--list-order", "done,todo,backlog")
	p := runJSON(t, "project", "show", "api-v2", "--json")["project"].(map[string]any)
	if got := p["list_order"].([]any); len(got) != 3 || got[0] != "done" {
		t.Errorf("list_order after update = %v", got)
	}

	// Duplicate name and invalid workflow are errors.
	wantError(t, "project", "update", "api-v2", "--name", "web")
	wantError(t, "project", "update", "api-v2", "--workflow", "no,duplicates,no")
}

func TestProjectArchiveLifecycle(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	runJSON(t, "project", "archive", "api")

	if out, _ := run(t, "project", "list"); strings.Contains(out, "api") {
		t.Errorf("the archived project is still in the list: %q", out)
	}
	archived := runJSON(t, "project", "list", "--archived", "--json")
	if got := archived["projects"].([]any); len(got) != 1 {
		t.Errorf("archived list = %v", got)
	}

	runJSON(t, "project", "unarchive", "api")
	if out, _ := run(t, "project", "list"); !strings.Contains(out, "api") {
		t.Errorf("after unarchiving it should come back: %q", out)
	}
	wantError(t, "project", "archive", "nope")
	wantError(t, "project", "unarchive", "nope")
}

func TestProjectRemove(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	runJSON(t, "project", "remove", "api")
	wantError(t, "project", "remove", "api")
	wantError(t, "project", "add")
}

// ---- add / list / show ----

func TestAddRequiresProject(t *testing.T) {
	withTempDB(t)
	wantError(t, "add")
	wantError(t, "add", "no project")
}

func TestAddAppliesEveryFlag(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	payload := runJSON(t, "add", "with everything", "--project", "api",
		"--priority", "2", "--assignee", "@ann", "--status", "todo",
		"--estimate", "2.5", "--tag", "bug,urgent")
	task := payload["task"].(map[string]any)

	if task["priority"].(float64) != 2 {
		t.Errorf("priority = %v, want 2", task["priority"])
	}
	if task["assignee"] != "@ann" {
		t.Errorf("assignee = %v", task["assignee"])
	}
	if task["status"] != "todo" {
		t.Errorf("status = %v, want todo", task["status"])
	}
	if task["estimate"].(float64) != 2.5 {
		t.Errorf("estimate = %v, want 2.5", task["estimate"])
	}
	tags := strList(task["tags"])
	if len(tags) != 2 || tags[0] != "bug" || tags[1] != "urgent" {
		t.Errorf("tags = %v, want [bug urgent]", tags)
	}
}

func TestAddIgnoresMalformedNumericFlags(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	// A non-numeric --priority is ignored (priority 0), just like a negative
	// estimate: the command must not fail, it simply does not apply it.
	task := runJSON(t, "add", "weird", "--project", "api",
		"--priority", "abc", "--estimate", "-3")["task"].(map[string]any)
	if task["priority"].(float64) != 0 {
		t.Errorf("priority = %v, want 0", task["priority"])
	}
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0 (negative ignored)", task["estimate"])
	}
}

func TestAddRejectsBadInput(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	wantError(t, "add", "x", "--project", "nope")
	wantError(t, "add", "x", "--project", "api", "--status", "nope")
}

func TestListFilters(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")

	addTask(t, "api", "a1", "--assignee", "@ann", "--tag", "bug")
	addTask(t, "api", "a2", "--assignee", "@bob")
	addTask(t, "web", "w1", "--status", "todo")

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"todo", []string{"list", "--json"}, []string{"a1", "a2", "w1"}},
		{"by project", []string{"list", "--project", "api", "--json"}, []string{"a1", "a2"}},
		{"by status", []string{"list", "--status", "todo", "--json"}, []string{"w1"}},
		{"by assignee", []string{"list", "--assignee", "@ann", "--json"}, []string{"a1"}},
		{"by tag", []string{"list", "--tag", "bug", "--json"}, []string{"a1"}},
		{"nonexistent tag", []string{"list", "--tag", "nope", "--json"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			titles := listTitles(t, tt.args...)
			if len(titles) != len(tt.want) {
				t.Fatalf("titles = %v, want %v", titles, tt.want)
			}
			for i, want := range tt.want {
				if titles[i] != want {
					t.Errorf("titles[%d] = %q, want %q (workflow order: %v)", i, titles[i], want, titles)
				}
			}
		})
	}
}

func listTitles(t *testing.T, args ...string) []string {
	t.Helper()
	tasks, ok := runJSON(t, args...)["tasks"].([]any)
	if !ok {
		t.Fatalf("list %v without tasks", args)
	}
	titles := make([]string, 0, len(tasks))
	for _, item := range tasks {
		titles = append(titles, str(t, item.(map[string]any)["title"]))
	}
	return titles
}

func TestListHumanOutput(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	addTask(t, "api", "visible")

	out, _ := run(t, "list")
	if !strings.Contains(out, "ID") || !strings.Contains(out, "visible") {
		t.Errorf("human list = %q", out)
	}
	if !strings.Contains(out, "Total: 1 tasks") {
		t.Errorf("the total is missing: %q", out)
	}

	if out, _ := run(t, "list", "--project", "empty"); !strings.Contains(out, "No tasks found.") {
		t.Errorf("empty list = %q", out)
	}
}

func TestShowIncludesComments(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "with comment")
	runJSON(t, "comment", "add", strconv.FormatInt(id, 10), "hello", "world")

	payload := runJSON(t, "show", strconv.FormatInt(id, 10))
	if str(t, payload["task"].(map[string]any)["title"]) != "with comment" {
		t.Errorf("task = %v", payload["task"])
	}
	// The body is joined with spaces: several args are a single comment.
	comments := payload["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments = %v", comments)
	}
	if body := str(t, comments[0].(map[string]any)["body"]); body != "hello world" {
		t.Errorf("body = %q, want %q", body, "hello world")
	}

	wantError(t, "show", "9999")
	wantError(t, "show", "abc")
}

// ---- transitions ----

func TestMoveAndShorthands(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	tests := []struct {
		name       string
		args       []string
		wantStatus string
	}{
		{"move", []string{"move", "", "todo"}, "todo"},
		{"done", []string{"done", ""}, "done"},
		{"cancel", []string{"cancel", ""}, "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := addTask(t, "api", tt.name)
			args := append([]string{}, tt.args...)
			args[1] = strconv.FormatInt(id, 10)
			payload := runJSON(t, args...)
			if got := str(t, taskField(t, payload, "status")); got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestStartReportsNextStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "startable")

	payload := runJSON(t, "start", strconv.FormatInt(id, 10))
	if got := str(t, taskField(t, payload, "status")); got != "todo" {
		t.Errorf("start → %q, want todo", got)
	}
	// The default workflow still is backlog,todo,doing,..., so propose doing.
	if next := str(t, payload["next"]); next != "doing" {
		t.Errorf("next = %q, want doing", next)
	}
}

func TestReviewRequiresReviewStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "with review")
	if got := str(t, taskField(t, runJSON(t, "review", strconv.FormatInt(id, 10)), "status")); got != "reviewing" {
		t.Errorf("review → %q, want reviewing", got)
	}

	// A project without a review status cannot go through review.
	runJSON(t, "project", "add", "web", "--workflow", "backlog,todo,done")
	wid := addTask(t, "web", "without review")
	wantError(t, "review", strconv.FormatInt(wid, 10))
}

func TestMoveRejectsUnknownStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "x")
	wantError(t, "move", strconv.FormatInt(id, 10), "nope")
	wantError(t, "move", "9999", "todo")
}

// ---- update ----

func TestUpdateFieldsAndTags(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "before", "--tag", "old")
	arg := strconv.FormatInt(id, 10)

	payload := runJSON(t, "update", arg, "--title", "after",
		"--description", "notes", "--priority", "3",
		"--assignee", "@bob", "--estimate", "1.5")
	task := payload["task"].(map[string]any)
	if task["title"] != "after" || task["description"] != "notes" {
		t.Errorf("title/description = %v/%v", task["title"], task["description"])
	}
	if task["priority"].(float64) != 3 || task["assignee"] != "@bob" || task["estimate"].(float64) != 1.5 {
		t.Errorf("priority/assignee/estimate = %v/%v/%v",
			task["priority"], task["assignee"], task["estimate"])
	}

	// --tags replaces, --tag adds, --untag removes.
	task = runJSON(t, "update", arg, "--tag", "new")["task"].(map[string]any)
	if tags := strList(task["tags"]); len(tags) != 2 {
		t.Errorf("after --tag = %v, want 2 tags", tags)
	}
	task = runJSON(t, "update", arg, "--untag", "old")["task"].(map[string]any)
	if tags := strList(task["tags"]); len(tags) != 1 || tags[0] != "new" {
		t.Errorf("after --untag = %v", tags)
	}
	task = runJSON(t, "update", arg, "--tags", "a,b")["task"].(map[string]any)
	if tags := strList(task["tags"]); len(tags) != 2 || tags[0] != "a" {
		t.Errorf("after --tags = %v, want [a b]", tags)
	}
}

func TestUpdateNoFlagsIsANoop(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "same")
	task := runJSON(t, "update", strconv.FormatInt(id, 10))["task"].(map[string]any)
	if task["title"] != "same" {
		t.Errorf("title = %v, want same", task["title"])
	}
	wantError(t, "update", "9999", "--title", "x")
}

// ---- comments ----

func TestCommentLifecycle(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "task")
	arg := strconv.FormatInt(id, 10)

	created := runJSON(t, "comment", "add", arg, "first")
	commentID := int64(created["comment"].(map[string]any)["id"].(float64))

	if got := runJSON(t, "comment", "list", arg)["comments"].([]any); len(got) != 1 {
		t.Errorf("list = %v", got)
	}
	runJSON(t, "comment", "remove", strconv.FormatInt(commentID, 10))
	if got := runJSON(t, "comment", "list", arg)["comments"].([]any); len(got) != 0 {
		t.Errorf("after remove = %v, want empty", got)
	}

	wantError(t, "comment")
	wantError(t, "comment", "nope")
	wantError(t, "comment", "add")
	wantError(t, "comment", "add", arg)
	wantError(t, "comment", "list")
	wantError(t, "comment", "remove")
	wantError(t, "comment", "remove", "9999")
}

func TestCommentListEmpty(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "without comments")
	// The empty slice must come as [] and not as null: it is consumed from JSON.
	got := runJSON(t, "comment", "list", strconv.FormatInt(id, 10))["comments"]
	if list, ok := got.([]any); !ok || len(list) != 0 {
		t.Errorf("comments = %v, want empty list", got)
	}
}

// ---- offday ----

func TestOffDayLifecycle(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	created := runJSON(t, "offday", "add", "@ann", "2026-07-28", "2026-08-01", "--note", "vacation")
	off := created["offday"].(map[string]any)
	if off["start_date"] != "2026-07-28" || off["end_date"] != "2026-08-01" {
		t.Errorf("range = %v→%v", off["start_date"], off["end_date"])
	}
	if off["note"] != "vacation" {
		t.Errorf("note = %v", off["note"])
	}

	// No end nor note: a single day.
	one := runJSON(t, "offday", "add", "@bob", "2026-07-28")["offday"].(map[string]any)
	if one["start_date"] != "2026-07-28" || one["end_date"] != "2026-07-28" {
		t.Errorf("a single day = %v→%v", one["start_date"], one["end_date"])
	}

	if got := runJSON(t, "offday", "list", "--json")["offdays"].([]any); len(got) != 2 {
		t.Errorf("list = %v, want 2", got)
	}
	if got := runJSON(t, "offday", "list", "--assignee", "@ann", "--json")["offdays"].([]any); len(got) != 1 {
		t.Errorf("filtered list = %v, want 1", got)
	}

	out, _ := run(t, "offday", "list")
	if !strings.Contains(out, "ASSIGNEE") || !strings.Contains(out, "@ann") {
		t.Errorf("human offday list = %q", out)
	}

	id := int64(off["id"].(float64))
	runJSON(t, "offday", "remove", strconv.FormatInt(id, 10))
	if got := runJSON(t, "offday", "list", "--json")["offdays"].([]any); len(got) != 1 {
		t.Errorf("after remove = %v, want 1", got)
	}
}

func TestOffDayValidations(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	wantError(t, "offday")
	wantError(t, "offday", "nope")
	wantError(t, "offday", "add")
	wantError(t, "offday", "add", "unassigned", "2026-07-28")
	wantError(t, "offday", "add", "@ann", "not-a-date")
	wantError(t, "offday", "remove")
	wantError(t, "offday", "remove", "9999")

	if out, _ := run(t, "offday", "list"); !strings.Contains(out, "No off-days registered.") {
		t.Errorf("empty offday list = %q", out)
	}
}

// ---- gantt / stats / help ----

func TestGanttOutputs(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")
	addTask(t, "api", "by ann", "--assignee", "@ann", "--estimate", "2")
	addTask(t, "web", "by bob", "--assignee", "@bob")

	// The payload IS the schedule, with no wrapper.
	sched := runJSON(t, "gantt", "--json", "--weeks", "2")
	if str(t, sched["start"]) == "" {
		t.Errorf("schedule without start: %v", sched)
	}

	// The filters are VIEW-only: they hide the person but the queue is not recalculated.
	tests := []struct {
		name       string
		args       []string
		wantPeople []string
	}{
		{"no filter", []string{"gantt", "--json", "--weeks", "2"}, []string{"@ann", "@bob"}},
		{"by project", []string{"gantt", "--json", "--weeks", "2", "--project", "api"}, []string{"@ann"}},
		{"by assignee", []string{"gantt", "--json", "--weeks", "2", "--assignee", "@bob"}, []string{"@bob"}},
		{"project and assignee at once", []string{"gantt", "--json", "--weeks", "2", "--project", "web", "--assignee", "@ann"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, a := range runJSON(t, tt.args...)["assignees"].([]any) {
				got = append(got, str(t, a.(map[string]any)["assignee"]))
			}
			if len(got) != len(tt.wantPeople) {
				t.Fatalf("assignees = %v, want %v", got, tt.wantPeople)
			}
			for i := range got {
				if got[i] != tt.wantPeople[i] {
					t.Errorf("assignee[%d] = %q, want %q", i, got[i], tt.wantPeople[i])
				}
			}
		})
	}

	out, _ := run(t, "gantt", "--weeks", "2")
	if !strings.Contains(out, "Gantt") {
		t.Errorf("gantt text = %q", out)
	}
}

// TestGanttWeeksInvalidFallsBackToConfig: a non-numeric or non-positive --weeks
// leaves weeks at 0 and the config value is used. If the fallback did not happen,
// the calendar would come out with 0 days and an empty Gantt that the command would report
// as if it were a filter with no results.
func TestGanttWeeksInvalidFallsBackToConfig(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	addTask(t, "api", "task", "--assignee", "@ann")

	for _, weeks := range []string{"zero", "0", "-2", "not-a-number"} {
		t.Run(weeks, func(t *testing.T) {
			if got := runJSON(t, "gantt", "--json", "--weeks", weeks)["assignees"].([]any); len(got) != 1 {
				t.Errorf("assignees = %v, want 1", got)
			}
			// The text axis must have days: 0 weeks = 0 cells.
			out, _ := run(t, "gantt", "--weeks", weeks)
			axis := strings.Split(out, "\n")[ganttHeadOff+1]
			if got := len([]rune(axis)) - ganttLabelW - 1; got <= 0 {
				t.Errorf("weeks=%q fell back to 0 days instead of using the config: %q", weeks, axis)
			}
		})
	}
}

func TestStats(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")
	addTask(t, "api", "a1", "--assignee", "@ann")
	addTask(t, "api", "a2", "--assignee", "@ann", "--status", "todo")
	addTask(t, "web", "w1", "--assignee", "@bob")

	stats := runJSON(t, "stats", "--json")["stats"].(map[string]any)
	if stats["total"].(float64) != 3 {
		t.Errorf("total = %v, want 3", stats["total"])
	}

	byProject := runJSON(t, "stats", "--json", "--project", "api")["stats"].(map[string]any)
	if byProject["total"].(float64) != 2 {
		t.Errorf("api's total = %v, want 2", byProject["total"])
	}

	out, _ := run(t, "stats")
	for _, want := range []string{"Total: 3 tasks", "By Status:", "By Assignee:", "@ann"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats = %q, missing %q", out, want)
		}
	}
}

func TestHelpListsCommands(t *testing.T) {
	withTempDB(t)
	out, _ := run(t, "help")
	for _, want := range []string{"Projects:", "Tasks:", "add", "list", "gantt", "stats"} {
		if !strings.Contains(out, want) {
			t.Errorf("help = %q, missing %q", out, want)
		}
	}
}

func TestMigrateIsANoop(t *testing.T) {
	withTempDB(t)
	if _, code := run(t, "migrate"); code != 0 {
		t.Errorf("migrate exited with %d", code)
	}
}

// TestProjectSubcommandUsageAtExactlyTwoArgs: the args guard of the
// `project` subcommands is `len(args) < 2`, so the edge that separates it
// from `<= 2` is a subcommand with exactly 2 tokens. With 1 it must fail and with
// 2 it must not.
func TestProjectSubcommandUsageAtExactlyTwoArgs(t *testing.T) {
	withTempDB(t)
	for _, sub := range []string{"show", "update"} {
		wantError(t, "project", sub)
	}
	seed(t, "api")
	// Exactly 3 tokens (subcommand + name): it is the other side of the edge of
	// `len(args) < 2` and it has to remain a valid command.
	if _, code := run(t, "project", "show", "api"); code != 0 {
		t.Errorf("project show api exited with %d", code)
	}
	if _, code := run(t, "project", "update", "api"); code != 0 {
		t.Errorf("project update api exited with %d", code)
	}
	if _, code := run(t, "project", "show", "api", "--json"); code != 0 {
		t.Errorf("project show api --json exited with %d", code)
	}
}

// TestUpdateEstimateZeroOverwrites pins the `e >= 0` edge of the --estimate flag:
// with 0 the value is NOT "unset", it is an explicit estimate of zero days. If
// the flag were ignored, the task would keep the previous estimate.
func TestUpdateEstimateZeroOverwrites(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "with days", "--estimate", "3")
	arg := strconv.FormatInt(id, 10)

	task := runJSON(t, "update", arg, "--estimate", "0")["task"].(map[string]any)
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0 (an explicit 0 overwrites the previous one)", task["estimate"])
	}
	// And it is still zero after re-reading, not an unapplied estimate.
	again := runJSON(t, "show", arg)["task"].(map[string]any)
	if again["estimate"].(float64) != 0 {
		t.Errorf("persisted estimate = %v, want 0", again["estimate"])
	}
}

// TestAddEstimateZeroIsValid: on creation, --estimate 0 leaves the estimate at 0
// (which is the column's default value), with no error.
func TestAddEstimateZeroIsValid(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	task := runJSON(t, "add", "without days", "--project", "api", "--estimate", "0")["task"].(map[string]any)
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0", task["estimate"])
	}
}

// TestUpdateTagsFlagsAreGreedy documents the current semantics of the flag
// parser: a flag with a value eats the next token NO MATTER WHAT, even if it
// looks like another flag. `update --tag --untag` ends up adding a tag named
// "--untag". It is not ideal (a real parser would error on a missing value),
// but it is pinned here on purpose: if it ever changes, this test says so.
func TestUpdateTagsFlagsAreGreedy(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "tags", "--tag", "one")
	arg := strconv.FormatInt(id, 10)

	task := runJSON(t, "update", arg, "--tag", "--untag")["task"].(map[string]any)
	tags := strList(task["tags"])
	if len(tags) != 2 || tags[0] != "one" || tags[1] != "--untag" {
		t.Errorf("tags = %v, want [one --untag] (parser greedy)", tags)
	}

	// An orphan flag at the end IS ignored: there is no token to consume.
	task = runJSON(t, "update", arg, "--untag")["task"].(map[string]any)
	if got := strList(task["tags"]); len(got) != 2 {
		t.Errorf("tags = %v, want the 2 intact", got)
	}

	// --tags with an empty list DOES empty the tags (replacement semantics).
	task = runJSON(t, "update", arg, "--tags", "")["task"].(map[string]any)
	if got := strList(task["tags"]); len(got) != 0 {
		t.Errorf("--tags empty = %v, want no tags", got)
	}
}

// TestGanttWeeksExactDayCount pins the exact number of days per --weeks. With a
// valid `--weeks`, the grid has 7 days per week; if the flag were ignored
// (the mutant of `err == nil`) or read backwards, it would fall back to the config.
func TestGanttWeeksExactDayCount(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	addTask(t, "api", "task", "--assignee", "@ann")

	for weeks, days := range map[string]int{"1": 7, "2": 14, "4": 28} {
		t.Run(weeks, func(t *testing.T) {
			out, _ := run(t, "gantt", "--weeks", weeks)
			axis := strings.Split(out, "\n")[ganttHeadOff+1]
			if got := len([]rune(axis)) - ganttLabelW - 1; got != days {
				t.Errorf("--weeks %s → %d days, want %d (%q)", weeks, got, days, axis)
			}
		})
	}
}

// TestAddLastFlagWinsAndZeroIsExplicit: with two --estimate, the last one wins. And an
// explicit 0 has to overwrite a previous value: `e >= 0` distinguishes 0 from "unset"
// and that's why the edge matters.
func TestAddLastFlagWinsAndZeroIsExplicit(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	task := runJSON(t, "add", "x", "--project", "api",
		"--estimate", "2", "--estimate", "0")["task"].(map[string]any)
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0 (the last flag wins)", task["estimate"])
	}

	task = runJSON(t, "add", "y", "--project", "api",
		"--priority", "3", "--priority", "0")["task"].(map[string]any)
	if task["priority"].(float64) != 0 {
		t.Errorf("priority = %v, want 0", task["priority"])
	}
}

// TestUpdateWithoutFlagsIsAnExactNoop documents that `update <id>` without flags
// writes nothing. The guard `if len(updates) > 0` exists just for that: without it the
// call would open an UPDATE that only moves updated_at.
func TestUpdateWithoutFlagsIsAnExactNoop(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "still", "--tag", "kept")
	arg := strconv.FormatInt(id, 10)

	before := runJSON(t, "show", arg)["task"].(map[string]any)
	runJSON(t, "update", arg)
	after := runJSON(t, "show", arg)["task"].(map[string]any)

	if before["title"] != after["title"] || before["updated_at"] != after["updated_at"] {
		t.Errorf("update without flags changed something: %v → %v", before["updated_at"], after["updated_at"])
	}
	if got := strList(after["tags"]); len(got) != 1 || got[0] != "kept" {
		t.Errorf("tags = %v, want [kept]", got)
	}
}
