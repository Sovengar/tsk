package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// ntKey sends a key to the form modal. It does not reuse press() because this
// handler needs keys press() does not build (arrows, shift+tab, ctrl+s) and
// because several assertions need the returned cmd intact.
func ntKey(m *Model, key string) (*Model, tea.Cmd) {
	var km tea.KeyPressMsg
	switch key {
	case "enter":
		km = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		km = tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		km = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "esc":
		km = tea.KeyPressMsg{Code: tea.KeyEsc}
	case "up":
		km = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		km = tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		km = tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		km = tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		km = tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+s":
		km = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case ",":
		km = tea.KeyPressMsg{Code: ',', Text: ","}
	default:
		km = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	next, cmd := m.Update(km)
	return asModel(next), cmd
}

// modelWithPeopleAndTags creates a model with the people and tags that the
// autocomplete has to offer. The default fixtures only bring @john and
// @margo and no tag, so the dropdown caps are never reached.
func modelWithPeopleAndTags(t *testing.T, people []string, tags []string) *Model {
	t.Helper()
	m := newTestModel(t)
	m.tasks = nil
	if len(people) == 0 {
		people = []string{"@ann"} // at least one task is needed for tags to exist
	}
	for i, p := range people {
		m.tasks = append(m.tasks, model.Task{
			ID:       int64(i + 1),
			Title:    "t",
			Status:   "todo",
			Assignee: p,
			Tags:     tags,
		})
	}
	m.invalidateFilterCache()
	return m
}

func openNewTask(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m, _ = ntKey(m, "i")
	if !m.newTaskOpen {
		t.Fatal("the creation modal did not open")
	}
	return m
}

// --- Caps of the suggestions dropdown -----------------------------------

// The assignees dropdown is cut at newTaskMaxSuggestions. The cut matters:
// without it the modal grows without limit and the inverted
// `len(out) == max` (which cuts as soon as len(out) != max, that is, on the first one) would return a single suggestion.
func TestNewTaskAssigneeSuggestionsCap(t *testing.T) {
	people := []string{"@ann", "@carol", "@david", "@helen", "@margo", "@fiona"}
	m := modelWithPeopleAndTags(t, people, nil)
	m.newTaskAssignee = "a" // all six match, so only the cap decides

	got := m.assigneeSuggestions()
	if len(got) != newTaskMaxSuggestions {
		t.Errorf("suggestions = %d, want the cap %d (%v)", len(got), newTaskMaxSuggestions, got)
	}
}

func TestNewTaskTagSuggestionsCap(t *testing.T) {
	tags := []string{"alpha", "beta", "gamma", "delta", "zeta", "gamma2"}
	m := modelWithPeopleAndTags(t, nil, tags)
	m.newTaskTagInput = "a" // all match, so only the cap decides

	got := m.tagFieldSuggestions()
	if len(got) != newTaskMaxSuggestions {
		t.Errorf("tag suggestions = %d, want the cap %d (%v)", len(got), newTaskMaxSuggestions, got)
	}
}

// The exact match does not suggest itself: it is what is already written.
func TestNewTaskSuggestionsExcludeExactMatch(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@bruce"}, []string{"urgent", "backend"})

	m.newTaskAssignee = "@ann"
	if got := m.assigneeSuggestions(); contains(got, "@ann") {
		t.Errorf("the exact assignee must not be suggested: %v", got)
	}

	m.newTaskTagInput = "urgent"
	if got := m.tagFieldSuggestions(); contains(got, "urgent") {
		t.Errorf("the exact tag must not be suggested: %v", got)
	}
}

func contains(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}

// --- Textarea width ------------------------------------------------------

func TestNewTaskTextareaWidth(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantMin   int
		wantExact int // > 0 when the preferred width fits entirely
	}{
		{"wide screen uses the preferred", 120, 10, newTaskModalWidth - 6},
		{"medium screen clamps to the available", 40, 10, 0},
		{"tiny screen never drops below 10", 12, 10, 0},
		{"1-column screen", 1, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.width = tt.width

			got := m.newTaskTextareaWidth()
			if got < tt.wantMin {
				t.Errorf("newTaskTextareaWidth() = %d, want >= %d", got, tt.wantMin)
			}
			if tt.wantExact > 0 && got != tt.wantExact {
				t.Errorf("newTaskTextareaWidth() = %d, want %d", got, tt.wantExact)
			}
		})
	}
}

// --- Suggestion completion on tab or submit -------------------------------

func TestNewTaskMoveFieldCompletesAssigneeSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = 0
	if len(m.assigneeSuggestions()) == 0 {
		t.Fatal("fixture: with no suggestions there is nothing to complete")
	}

	m, _ = ntKey(m, "tab")
	if m.newTaskAssignee == "" {
		t.Error("tabbing with an active suggestion must complete the assignee")
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx after completing = %d, want -1", m.newTaskAssigneeSuggIdx)
	}
	if m.newTaskFieldIdx != newTaskFieldTags {
		t.Errorf("focus must advance anyway, field = %d", m.newTaskFieldIdx)
	}
}

// A stale suggestion index (the filter changed underneath) must not index
// out of range nor complete with garbage.
func TestNewTaskMoveFieldIgnoresStaleSuggestionIndex(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "typed"
	m.newTaskAssigneeSuggIdx = 99 // beyond the suggestion list

	m, _ = ntKey(m, "tab")
	if m.newTaskAssignee != "typed" {
		t.Errorf("assignee = %q, want the typed value intact", m.newTaskAssignee)
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 (cleared anyway)", m.newTaskAssigneeSuggIdx)
	}
}

// The suggestion is only completed on the Assignee field: on another field
// the same index must not touch anything.
func TestNewTaskSuggestionOnlyCompletesOnAssigneeField(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskAssignee = "intact"
	m.newTaskAssigneeSuggIdx = 0

	m, _ = ntKey(m, "tab")
	if m.newTaskAssignee != "intact" {
		t.Errorf("assignee = %q, want intact outside the Assignee field", m.newTaskAssignee)
	}
}

// newTaskSubmit completes the suggestion the same as tabbing, so submitting
// while navigating the dropdown does not lose the selection.
func TestNewTaskSubmitCompletesAssigneeSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskTitle = "task"
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = 1

	if len(m.assigneeSuggestions()) < 2 {
		t.Fatalf("fixture: 2 suggestions needed, got %d", len(m.assigneeSuggestions()))
	}

	_, cmd := ntKey(m, "ctrl+s")
	msgs := mustRun(t, cmd)

	tasks, err := m.database.ListTasks("api", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var created *model.Task
	for i := range tasks {
		if tasks[i].Title == "task" {
			created = &tasks[i]
		}
	}
	if created == nil {
		t.Fatal("the task was not created")
	}
	if created.Assignee == "" {
		t.Error("the active suggestion should have completed the assignee on submit")
	}
	if !containsMsgType(msgs, tasksLoadedMsg{}) {
		t.Errorf("msgs = %v, want a tasksLoadedMsg", msgTypes(msgs))
	}
}

// containsMsgType and msgTypes make the assertion about the type of the
// messages a batch produced readable.
func containsMsgType(msgs []tea.Msg, want tea.Msg) bool {
	for _, msg := range msgs {
		if reflect.TypeOf(msg) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}

func msgTypes(msgs []tea.Msg) []string {
	var out []string
	for _, msg := range msgs {
		out = append(out, fmt.Sprintf("%T", msg))
	}
	return out
}

// enter on the Assignee field with an active suggestion completes the
// selection and does NOT submit the form: the dropdown beats the validation.
func TestNewTaskAssigneeEnterCompletesSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = 0
	suggs := m.assigneeSuggestions()
	if len(suggs) == 0 {
		t.Fatal("fixture: no suggestions")
	}

	m, _ = ntKey(m, "enter")

	if !m.newTaskOpen {
		t.Error("enter with an active suggestion must not submit the form")
	}
	if m.newTaskAssignee != suggs[0] {
		t.Errorf("assignee = %q, want the suggestion %q", m.newTaskAssignee, suggs[0])
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 after completing", m.newTaskAssigneeSuggIdx)
	}
}

// enter with an index exactly equal to the list's size falls into the submit,
// not into an out-of-range indexing. It is the exact edge of `< len(suggs)`.
func TestNewTaskAssigneeEnterAtExactListLength(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskTitle = "exact edge"
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = len(m.assigneeSuggestions()) // one past the last

	_, cmd := ntKey(m, "enter")
	mustRun(t, cmd)

	tasks, _ := m.database.ListTasks("api", "", "")
	found := false
	for _, task := range tasks {
		if task.Title == "exact edge" {
			found = true
		}
	}
	if !found {
		t.Error("with the index out of range, enter must submit the form")
	}
}

// Submitting from another field with an active suggestion index does NOT
// complete the assignee: the condition demands being on the Assignee field, not just having an index.
func TestNewTaskSubmitFromTitleIgnoresAssigneeSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldTitle
	m.newTaskTitle = "from title"
	m.newTaskAssignee = ""
	// Index 1 on purpose: suggestion 0 is "Me", which is exactly what the
	// submit's default expects, so with 0 the mutant would go
	// unnoticed.
	m.newTaskAssigneeSuggIdx = 1
	if suggs := m.assigneeSuggestions(); len(suggs) < 2 || suggs[1] == "Me" {
		t.Fatalf("fixture: the second suggestion must be another person, got %v", suggs)
	}

	_, cmd := ntKey(m, "ctrl+s")
	mustRun(t, cmd)

	tasks, _ := m.database.ListTasks("api", "", "")
	for _, task := range tasks {
		if task.Title == "from title" {
			if task.Assignee != "Me" {
				t.Errorf("assignee = %q, want Me (not completed outside the field)", task.Assignee)
			}
			return
		}
	}
	t.Fatal("the task was not created")
}

// --- Form defaults -------------------------------------------------------

// With no explicit assignee the task is attributed to "Me": that default is
// lost with the inverted `== ""`.
func TestNewTaskSubmitDefaultsAssigneeToMe(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTitle = "no assignee"
	m.newTaskAssignee = "   " // only spaces, trimmed to empty

	_, cmd := ntKey(m, "ctrl+s")
	mustRun(t, cmd)

	tasks, _ := m.database.ListTasks("api", "", "")
	found := false
	for _, task := range tasks {
		if task.Title == "no assignee" {
			found = true
			if task.Assignee != "Me" {
				t.Errorf("assignee = %q, want Me", task.Assignee)
			}
		}
	}
	if !found {
		t.Fatal("the task was not created")
	}
}

func TestNewTaskCloseResetsState(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTitle = "halfway"
	m.newTaskAssignee = "@ann"
	m.newTaskErr = "something"
	m.newTaskAssigneeSuggIdx = 2
	m.newTaskTags = []string{"x"}

	m, _ = ntKey(m, "esc")

	if m.newTaskOpen {
		t.Error("esc must close the modal")
	}
	if m.newTaskTitle != "" || m.newTaskAssignee != "" || m.newTaskErr != "" {
		t.Errorf("transient state must be cleared: title=%q assignee=%q err=%q",
			m.newTaskTitle, m.newTaskAssignee, m.newTaskErr)
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 after closing", m.newTaskAssigneeSuggIdx)
	}
	// newTaskClose does NOT clear the tags: the full reset happens in newTask(),
	// which is the one that opens. Checked here so that the contract is explicit.
	if len(m.newTaskTags) != 1 {
		t.Errorf("tags = %v, want the tag intact (close leaves them alone)", m.newTaskTags)
	}
}

// Reopening the modal starts from a clean state, even if close did not clean everything.
func TestNewTaskReopenResetsTagsAndPriority(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTags = []string{"garbage"}
	m.newTaskTagInput = "medium"
	m.newTaskPriority = model.PriorityHigh
	m.newTaskErr = "old error"
	m, _ = ntKey(m, "esc")
	m, _ = ntKey(m, "i")

	if m.newTaskTags != nil {
		t.Errorf("tags = %v, want nil after reopening", m.newTaskTags)
	}
	if m.newTaskTagInput != "" {
		t.Errorf("input = %q, want empty after reopening", m.newTaskTagInput)
	}
	if m.newTaskPriority != model.PriorityLow {
		t.Errorf("priority = %d, want Low (the default on open)", m.newTaskPriority)
	}
	if m.newTaskErr != "" {
		t.Errorf("err = %q, want empty after reopening", m.newTaskErr)
	}
}

// --- Priority ------------------------------------------------------------

// left/right stop at the extremes. The condition is `> PriorityNone` and
// `< PriorityHigh`; without the edge cases the equivalent `>=`/`<=` goes
// unnoticed.
func TestNewTaskPriorityArrowsStopAtEdges(t *testing.T) {
	tests := []struct {
		name     string
		from     int
		key      string
		wantFrom int
	}{
		{"left at none does not go down", model.PriorityNone, "left", model.PriorityNone},
		{"left at low goes down to none", model.PriorityLow, "left", model.PriorityNone},
		{"right at high does not go up", model.PriorityHigh, "right", model.PriorityHigh},
		{"right at med goes up to high", model.PriorityMedium, "right", model.PriorityHigh},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := openNewTask(t)
			m.newTaskFieldIdx = newTaskFieldPriority
			m.newTaskPriority = tt.from

			m, _ = ntKey(m, tt.key)
			if m.newTaskPriority != tt.wantFrom {
				t.Errorf("priority = %d after %s from %d, want %d",
					m.newTaskPriority, tt.key, tt.from, tt.wantFrom)
			}
		})
	}
}

func TestNewTaskPriorityDigits(t *testing.T) {
	// The 1-4 keys assign the digit as-is, without offset: "1" leaves Low and
	// "4" leaves 4, which is outside the valid range 0-3. It is a bug (key 4
	// produces a priority that does not exist and the task is saved with it),
	// but it is pinned here as current behavior: fixing it is a deliberate
	// behavior change, not a missing test. See TestNewTaskPriorityDigitFourIsOutOfRange.
	for digit, want := range map[string]int{
		"1": 1,
		"2": 2,
		"3": 3,
		"4": 4,
	} {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldPriority

		m, _ = ntKey(m, digit)
		if m.newTaskPriority != want {
			t.Errorf("key %s: priority = %d, want %d", digit, m.newTaskPriority, want)
		}
	}
}

// It documents the bug: key 4 leaves the priority outside the range 0-3, so
// no radio is marked and the label falls back to the default "-".
func TestNewTaskPriorityDigitFourIsOutOfRange(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldPriority
	m, _ = ntKey(m, "4")

	if m.newTaskPriority <= model.PriorityHigh {
		t.Fatalf("priority = %d, expected outside the 0-3 range", m.newTaskPriority)
	}
	if got := ansi.Strip(m.renderNewTaskPriority()); strings.Contains(got, "●") {
		t.Errorf("render = %q, no radio should be marked", got)
	}
	if got := model.PriorityShortLabel(m.newTaskPriority); got != "-" {
		t.Errorf("PriorityShortLabel(%d) = %q, want - (out of range)", m.newTaskPriority, got)
	}
}

// The limit of the printable characters filter is 33 ('!'). With `> 33` the
// exclamation mark would stop being typed, so the edge is pinned down.
func TestNewTaskPrintableKeyJumpsToTitle(t *testing.T) {
	// The threshold is 33 ('!'). The space is 32, so it does NOT count as
	// printable and must stay in the field without touching the title.
	for _, ch := range []string{"a", "Z", "!", "9", "-"} {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldPriority

		m, _ = ntKey(m, ch)
		if m.newTaskFieldIdx != newTaskFieldTitle {
			t.Errorf("key %q: field = %d, want title", ch, m.newTaskFieldIdx)
		}
		if !strings.Contains(m.newTaskTitle, ch) {
			t.Errorf("key %q: title = %q, want it to contain the key", ch, m.newTaskTitle)
		}
	}
}

// ctrl+s is resolved before looking at the active field: it submits the form,
// it does not jump to the title. With an empty title it returns the inline
// error and gives focus back to Title without closing the modal.
func TestNewTaskCtrlSSubmitsFromPriorityField(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldPriority

	m, _ = ntKey(m, "ctrl+s")

	if !m.newTaskOpen {
		t.Error("the modal must not close without a title")
	}
	if m.newTaskFieldIdx != newTaskFieldTitle {
		t.Errorf("field = %d, want title after the validation error", m.newTaskFieldIdx)
	}
	if m.newTaskErr == "" {
		t.Error("want the inline error for the missing title")
	}
}

// --- Assignees dropdown navigation ---------------------------------------

func TestNewTaskAssigneeSuggestionWrapAround(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol", "@david"}, nil)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = -1

	if got := len(m.assigneeSuggestions()); got != 3 {
		t.Fatalf("fixture: 3 suggestions expected, got %d", got)
	}

	// down walks and wraps to the beginning.
	wantDown := []int{0, 1, 2, 0}
	for i, want := range wantDown {
		m, _ = ntKey(m, "down")
		if m.newTaskAssigneeSuggIdx != want {
			t.Fatalf("down #%d: suggIdx = %d, want %d", i+1, m.newTaskAssigneeSuggIdx, want)
		}
	}

	// up from 0 jumps to the last; from the last it goes down to 0.
	wantUp := []int{2, 1, 0}
	for i, want := range wantUp {
		m, _ = ntKey(m, "up")
		if m.newTaskAssigneeSuggIdx != want {
			t.Fatalf("up #%d: suggIdx = %d, want %d", i+1, m.newTaskAssigneeSuggIdx, want)
		}
	}
}

// With no suggestions, up/down do not touch the index. With the inverted
// `>= 0` the modulo by zero blows up the test.
func TestNewTaskAssigneeSuggestionArrowsWithNoSuggestions(t *testing.T) {
	m := openNewTask(t) // assignee "Me" suggests nothing
	m.newTaskFieldIdx = newTaskFieldAssignee
	if got := m.assigneeSuggestions(); len(got) != 0 {
		t.Fatalf("fixture: 0 suggestions expected, got %d (%v)", len(got), got)
	}
	m.newTaskAssigneeSuggIdx = -1

	for _, key := range []string{"up", "down", "up", "down"} {
		m, _ = ntKey(m, key)
		if m.newTaskAssigneeSuggIdx != -1 {
			t.Errorf("after %q with no suggestions suggIdx = %d, want -1", key, m.newTaskAssigneeSuggIdx)
		}
	}
}

// --- Tags -----------------------------------------------------------------

func TestNewTaskTagSuggestionWrapAround(t *testing.T) {
	m := modelWithPeopleAndTags(t, nil, []string{"alpha", "beta", "gamma"})
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "a"
	m.newTaskTagSuggIdx = -1

	if got := len(m.tagFieldSuggestions()); got != 3 {
		t.Fatalf("fixture: 3 suggestions expected, got %d", got)
	}

	for i, want := range []int{0, 1, 2, 0} {
		m, _ = ntKey(m, "down")
		if m.newTaskTagSuggIdx != want {
			t.Fatalf("down #%d: suggIdx = %d, want %d", i+1, m.newTaskTagSuggIdx, want)
		}
	}
	for i, want := range []int{2, 1, 0} {
		m, _ = ntKey(m, "up")
		if m.newTaskTagSuggIdx != want {
			t.Fatalf("up #%d: suggIdx = %d, want %d", i+1, m.newTaskTagSuggIdx, want)
		}
	}
}

func TestNewTaskTagSuggestionArrowsWithNoSuggestions(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "qqqq" // matches no tag
	if got := m.tagFieldSuggestions(); len(got) != 0 {
		t.Fatalf("fixture: 0 suggestions expected, got %d (%v)", len(got), got)
	}
	m.newTaskTagSuggIdx = -1

	for _, key := range []string{"up", "down"} {
		m, _ = ntKey(m, key)
		if m.newTaskTagSuggIdx != -1 {
			t.Errorf("after %q with no suggestions suggIdx = %d, want -1", key, m.newTaskTagSuggIdx)
		}
	}
}

func TestNewTaskTagEnterCompletesSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, nil, []string{"backend", "frontend"})
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "e"
	m.newTaskTagSuggIdx = 0
	if len(m.tagFieldSuggestions()) == 0 {
		t.Fatal("fixture: with no suggestions there is nothing to confirm")
	}

	m, _ = ntKey(m, "enter")

	if len(m.newTaskTags) != 1 {
		t.Fatalf("tags = %v, want one tag added", m.newTaskTags)
	}
	if m.newTaskTagInput != "" {
		t.Errorf("input = %q, want empty after confirming", m.newTaskTagInput)
	}
	if m.newTaskTagSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 after confirming", m.newTaskTagSuggIdx)
	}
}

// enter with no active suggestion commits what was typed as-is.
func TestNewTaskTagEnterCommitsTypedText(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "new"
	m.newTaskTagSuggIdx = -1

	m, _ = ntKey(m, "enter")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "new" {
		t.Errorf("tags = %v, want [new]", m.newTaskTags)
	}
}

func TestNewTaskTagCommaCommitsTypedText(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "comma"

	m, _ = ntKey(m, ",")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "comma" {
		t.Errorf("tags = %v, want [comma]", m.newTaskTags)
	}
}

// A stale suggestion index on confirm must not index out of range.
func TestNewTaskTagEnterIgnoresStaleSuggestionIndex(t *testing.T) {
	m := modelWithPeopleAndTags(t, nil, []string{"alpha", "beta"})
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "zzz"
	m.newTaskTagSuggIdx = 99

	m, _ = ntKey(m, "enter")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "zzz" {
		t.Errorf("tags = %v, want [zzz] (the stale index is ignored)", m.newTaskTags)
	}
}

func TestNewTaskTagBackspaceRemovesLastTagWhenInputEmpty(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTags = []string{"one", "two"}
	m.newTaskTagInput = ""

	m, _ = ntKey(m, "backspace")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "one" {
		t.Errorf("tags = %v, want [one]", m.newTaskTags)
	}
}

func TestNewTaskTagBackspaceEditsNonEmptyInput(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTags = []string{"one"}
	m.newTaskTagInput = "abc"

	m, _ = ntKey(m, "backspace")

	if m.newTaskTagInput != "ab" {
		t.Errorf("input = %q, want ab", m.newTaskTagInput)
	}
	if len(m.newTaskTags) != 1 {
		t.Errorf("tags = %v, want the stack intact", m.newTaskTags)
	}
}

// Backspace with an empty input and no tags must not try to delete from an
// empty slice.
func TestNewTaskTagBackspaceWithNoTagsIsSafe(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTags = nil
	m.newTaskTagInput = ""

	m, _ = ntKey(m, "backspace")

	if len(m.newTaskTags) != 0 {
		t.Errorf("tags = %v, want none", m.newTaskTags)
	}
}

// Typing clears the selection: the suggested list no longer corresponds to
// what is typed.
func TestNewTaskTypingResetsSuggestionIndex(t *testing.T) {
	t.Run("assignee", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldAssignee
		m.newTaskAssignee = "a"
		m.newTaskAssigneeSuggIdx = 1

		m, _ = ntKey(m, "n")
		if m.newTaskAssigneeSuggIdx != -1 {
			t.Errorf("suggIdx = %d, want -1 when typing", m.newTaskAssigneeSuggIdx)
		}
		if m.newTaskAssignee != "an" {
			t.Errorf("assignee = %q, want an", m.newTaskAssignee)
		}
	})

	t.Run("tags", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldTags
		m.newTaskTagInput = "a"
		m.newTaskTagSuggIdx = 0
		if len(m.tagFieldSuggestions()) == 0 {
			t.Fatal("fixture: with no suggestions the index is not observable")
		}

		m, _ = ntKey(m, "l")
		if m.newTaskTagSuggIdx != -1 {
			t.Errorf("suggIdx = %d, want -1 when typing", m.newTaskTagSuggIdx)
		}
		if m.newTaskTagInput != "al" {
			t.Errorf("input = %q, want al", m.newTaskTagInput)
		}
	})
}

// --- Render: tags ---------------------------------------------------------

// The three states of the tags row distinguish a focused field, a field
// with content and an empty field. They are distinct branches of the render, not variants.
func TestRenderNewTaskTagsStates(t *testing.T) {
	tests := []struct {
		name     string
		field    int
		tags     []string
		input    string
		wantHas  []string
		wantMiss []string
	}{
		{
			// The hint "type to add…" came out with the field focused and empty,
			// and the cursor glued behind. Before it never came out: the cursor
			// was added to input before the if, so input was never empty there
			// and the branch was unreachable. It is a fixed UI bug, not a
			// pinned expectation.
			name:     "empty and focused shows the hint and the cursor",
			field:    newTaskFieldTags,
			wantHas:  []string{"type to add", cursorGlyph},
			wantMiss: []string{"—"},
		},
		{
			name:     "empty and unfocused shows the dash",
			field:    newTaskFieldTitle,
			wantHas:  []string{"—"},
			wantMiss: []string{"type to add", cursorGlyph},
		},
		{
			name:     "with tags shows the chips",
			field:    newTaskFieldTags,
			tags:     []string{"api"},
			wantHas:  []string{"[api]"},
			wantMiss: []string{"type to add", "—"},
		},
		{
			name:     "half-typed input is shown with the chips",
			field:    newTaskFieldTags,
			tags:     []string{"api"},
			input:    "med",
			wantHas:  []string{"[api]", "med"},
			wantMiss: []string{"type to add", "—"},
		},
		{
			name:     "several tags",
			field:    newTaskFieldTags,
			tags:     []string{"api", "web"},
			wantHas:  []string{"[api]", "[web]"},
			wantMiss: []string{"type to add", "—"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := openNewTask(t)
			m.newTaskFieldIdx = tt.field
			m.newTaskTags = tt.tags
			m.newTaskTagInput = tt.input

			got := ansi.Strip(m.renderNewTaskTags())
			for _, want := range tt.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("render = %q, want it to contain %q", got, want)
				}
			}
			for _, bad := range tt.wantMiss {
				if strings.Contains(got, bad) {
					t.Errorf("render = %q, want it NOT to contain %q", got, bad)
				}
			}
		})
	}
}

// The cursor only appears in the focused field.
func TestRenderNewTaskTagsCursorOnlyWhenFocused(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTags = []string{"api"}

	m.newTaskFieldIdx = newTaskFieldTags
	withCursor := ansi.Strip(m.renderNewTaskTags())

	m.newTaskFieldIdx = newTaskFieldTitle
	withoutCursor := ansi.Strip(m.renderNewTaskTags())

	if withCursor == withoutCursor {
		t.Errorf("the cursor must depend on focus:\n focused: %q\n unfocused: %q", withCursor, withoutCursor)
	}
	if !strings.Contains(withCursor, cursorGlyph) {
		t.Errorf("focused = %q, want the cursor %q", withCursor, cursorGlyph)
	}
	if strings.Contains(withoutCursor, cursorGlyph) {
		t.Errorf("unfocused = %q, want no cursor", withoutCursor)
	}
}

func TestRenderNewTaskPriority(t *testing.T) {
	for p := model.PriorityNone; p <= model.PriorityHigh; p++ {
		m := openNewTask(t)
		m.newTaskPriority = p

		got := ansi.Strip(m.renderNewTaskPriority())
		labels := []string{"none", "low", "med", "high"}
		for i, l := range labels {
			marker := "○"
			if i == p {
				marker = "●"
			}
			want := marker + " " + l
			if !strings.Contains(got, want) {
				t.Errorf("priority %d: render = %q, want %q", p, got, want)
			}
		}
		if n := strings.Count(got, "●"); n != 1 {
			t.Errorf("priority %d: %d filled radios, want exactly 1", p, n)
		}
	}
}

// --- Render: "new" suggestions --------------------------------------------

// What is typed that does not exist yet is hinted as a new creation, in both
// tags and in assignee.
func TestRenderSuggestionsHintAtNewItem(t *testing.T) {
	t.Run("tags", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskTagInput = "invented"

		got := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
		if !strings.Contains(got, "+ new: invented") {
			t.Errorf("render = %q, want the new-tag hint", got)
		}
	})

	t.Run("existing tags are not hinted", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskTagInput = "alpha"

		got := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
		if strings.Contains(got, "+ new:") {
			t.Errorf("render = %q, must not hint a tag that already exists", got)
		}
	})

	t.Run("already-added tags are not hinted", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskTagInput = "alpha"
		m.newTaskTags = []string{"alpha"}

		got := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
		if strings.Contains(got, "+ new:") {
			t.Errorf("render = %q, must not hint an already-added tag", got)
		}
	})

	t.Run("assignee", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ann"}, nil)
		m.newTaskAssignee = "newbie"

		got := ansi.Strip(strings.Join(m.renderNewTaskAssigneeSuggestions(), "\n"))
		if !strings.Contains(got, "✎ new: newbie") {
			t.Errorf("render = %q, want the new-person hint", got)
		}
	})

	t.Run("existing assignee is not hinted", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ann"}, nil)
		m.newTaskAssignee = "@ann"

		got := ansi.Strip(strings.Join(m.renderNewTaskAssigneeSuggestions(), "\n"))
		if strings.Contains(got, "✎ new:") {
			t.Errorf("render = %q, must not hint a person that already exists", got)
		}
	})
}

// The dropdown's active item is marked; the others are not.
func TestRenderSuggestionsMarkActiveItem(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, []string{"alpha", "beta"})
	m.newTaskAssignee = "a"
	m.newTaskTagInput = "a"

	m.newTaskAssigneeSuggIdx = 1
	assignee := ansi.Strip(strings.Join(m.renderNewTaskAssigneeSuggestions(), "\n"))
	if !strings.Contains(assignee, "▸ @carol") {
		t.Errorf("render assignee = %q, want the second one marked", assignee)
	}

	m.newTaskTagSuggIdx = 0
	tags := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
	if !strings.Contains(tags, "▸ alpha") {
		t.Errorf("render tags = %q, want the first one marked", tags)
	}
}

// --- Render: full modal ---------------------------------------------------

// The modal always draws the sections, but the conditional blocks (error,
// suggestions) only when it applies. Each branch is a render path.
func TestRenderNewTaskModalSections(t *testing.T) {
	t.Run("priority focused", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldPriority
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "▸ Priority") {
			t.Errorf("want the focused Priority row:\n%s", got)
		}
		if !strings.Contains(got, "○ none") {
			t.Errorf("want the priority radios:\n%s", got)
		}
	})

	t.Run("empty description without focus", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldTitle
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "(empty · Tab to edit)") {
			t.Errorf("want the description placeholder:\n%s", got)
		}
	})

	t.Run("focused empty description does not show the placeholder", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldDescription
		got := ansi.Strip(m.renderNewTaskModal(""))

		if strings.Contains(got, "(empty · Tab to edit)") {
			t.Errorf("with focus on the description it must not ask to tab:\n%s", got)
		}
		if !strings.Contains(got, "▸ Description") {
			t.Errorf("want the focused Description section:\n%s", got)
		}
	})

	t.Run("inline error visible", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldTitle
		m.newTaskErr = "Title is required"
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "⚠ Title is required") {
			t.Errorf("want the inline error:\n%s", got)
		}
	})

	t.Run("no error means no error line", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskErr = ""
		got := ansi.Strip(m.renderNewTaskModal(""))

		if strings.Contains(got, "⚠") {
			t.Errorf("without an error there must be no warning line:\n%s", got)
		}
	})

	t.Run("title cursor only in its field", func(t *testing.T) {
		m := openNewTask(t)

		m.newTaskFieldIdx = newTaskFieldTitle
		withCursor := ansi.Strip(m.renderNewTaskModal(""))

		m.newTaskFieldIdx = newTaskFieldPriority
		withoutCursor := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(withCursor, cursorGlyph) {
			t.Errorf("with focus on the title the cursor must be visible:\n%s", withCursor)
		}
		if strings.Contains(withoutCursor, cursorGlyph) {
			t.Errorf("without focus on the title the cursor must not be visible:\n%s", withoutCursor)
		}
	})

	t.Run("assignee cursor only in its field", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ann"}, nil)
		m.newTaskOpen = true
		m.newTaskProject = "api"
		m.newTaskFieldIdx = newTaskFieldAssignee
		m.newTaskAssignee = ""

		got := ansi.Strip(m.renderNewTaskModal(""))
		_ = got
		if !strings.Contains(got, "▸ Assignee") {
			t.Errorf("want the focused Assignee row:\n%s", got)
		}
		if !strings.Contains(got, "@ann") {
			t.Errorf("with focus on Assignee the suggestions must be listed:\n%s", got)
		}

		m.newTaskFieldIdx = newTaskFieldTitle
		got = ansi.Strip(m.renderNewTaskModal(""))
		if strings.Contains(got, "@ann") {
			t.Errorf("without focus the dropdown must not be listed:\n%s", got)
		}
	})

	t.Run("tag suggestions only in their field", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha", "beta"})
		m.newTaskOpen = true
		m.newTaskProject = "api"
		m.newTaskTagInput = "a" // matches, so the dropdown has content

		m.newTaskFieldIdx = newTaskFieldTags
		got := ansi.Strip(m.renderNewTaskModal(""))
		if !strings.Contains(got, "▸ Tags") {
			t.Errorf("want the focused Tags row:\n%s", got)
		}
		if !strings.Contains(got, "alpha") {
			t.Errorf("with focus on Tags the suggestions must be listed:\n%s", got)
		}

		m.newTaskFieldIdx = newTaskFieldTitle
		got = ansi.Strip(m.renderNewTaskModal(""))
		if strings.Contains(got, "alpha") {
			t.Errorf("without focus the tag dropdown must not be listed:\n%s", got)
		}
	})

	t.Run("the optional badge appears in Tags", func(t *testing.T) {
		m := openNewTask(t)
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "optional") {
			t.Errorf("want the optional badge:\n%s", got)
		}
	})
}

// The modal is truncated to the available width; the inner width is the total
// minus the two borders, and on that depends that nothing overflows.
func TestRenderNewTaskModalFitsWidth(t *testing.T) {
	for _, width := range []int{120, 80, 70, 66, 40, 20} {
		m := openNewTask(t)
		m.width = width
		m.newTaskFieldIdx = newTaskFieldTags
		m.newTaskTags = []string{"a-very-long-tag-that-really-does-not-fit"}

		rendered := m.renderNewTaskModal("")
		for i, line := range strings.Split(ansi.Strip(rendered), "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width=%d: line %d of %d columns exceeds the terminal:\n%q", width, i, w, line)
			}
		}
	}
}

// Content that does not fit is TRUNCATED, not wrapped. That is what makes the
// modal's height not depend on the content: with a truncate width larger than
// the interior, the tags row splits into two lines and the modal grows.
func TestRenderNewTaskModalTruncatesInsteadOfWrapping(t *testing.T) {
	for _, width := range []int{120, 80, 64, 50, 40} {
		short := openNewTask(t)
		short.width = width
		short.newTaskFieldIdx = newTaskFieldTags
		short.newTaskTags = []string{"api"}

		long := openNewTask(t)
		long.width = width
		long.newTaskFieldIdx = newTaskFieldTags
		long.newTaskTags = []string{"a-very-long-tag-that-really-does-not-fit-at-all"}

		shortLines := len(strings.Split(short.renderNewTaskModal(""), "\n"))
		longLines := len(strings.Split(long.renderNewTaskModal(""), "\n"))
		if shortLines != longLines {
			t.Errorf("width=%d: the modal goes from %d to %d lines with a long tag; it must truncate, not wrap",
				width, shortLines, longLines)
		}
	}
}

// --- Focus sync with the textarea -----------------------------------------

// The textarea only receives focus when it is the active field; otherwise it
// is unfocused so it does not capture keys.
func TestNewTaskSyncFocusFollowsActiveField(t *testing.T) {
	m := openNewTask(t)

	m.newTaskFieldIdx = newTaskFieldDescription
	if cmd := m.newTaskSyncFocus(); cmd == nil {
		t.Error("focusing the description must emit a focus command")
	}

	m.newTaskFieldIdx = newTaskFieldTitle
	if cmd := m.newTaskSyncFocus(); cmd != nil {
		t.Error("on leaving the description the textarea must stay unfocused (no cmd)")
	}
}

// Tab from Tags confirms the half-typed tag instead of losing it.
func TestNewTaskMoveFieldCommitsPendingTag(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "pending"

	m, _ = ntKey(m, "tab")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "pending" {
		t.Errorf("tags = %v, want [pending]", m.newTaskTags)
	}
	if m.newTaskFieldIdx != newTaskFieldPriority {
		t.Errorf("field = %d, want priority after the wrap", m.newTaskFieldIdx)
	}
}

// The case that separates `>= 0` from `> 0` is index zero: with the first
// suggestion selected, a "> 0" would leave the text half-done and create the
// task with "@j" instead of "@john". Index 1 was already covered by the test
// above; this one puts the missing one.
func TestNewTaskSubmitCompletesFirstSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskTitle = "task with the first suggestion"
	m.newTaskAssignee = "@"
	m.newTaskAssigneeSuggIdx = 0

	suggs := m.assigneeSuggestions()
	if len(suggs) == 0 {
		t.Fatal("fixture: no suggestions")
	}

	_, cmd := ntKey(m, "ctrl+s")
	mustRun(t, cmd)

	tasks, err := m.database.ListTasks("api", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var created *model.Task
	for i := range tasks {
		if tasks[i].Title == "task with the first suggestion" {
			created = &tasks[i]
		}
	}
	if created == nil {
		t.Fatal("the task was not created")
	}
	if created.Assignee != suggs[0] {
		t.Errorf("the task was created with assignee %q, want %q (the first suggestion)", created.Assignee, suggs[0])
	}
}

// And with no selected suggestion the text stays as it is: the condition
// demands the index, not just being on the field.
func TestNewTaskSubmitKeepsTypedAssigneeWithoutSelection(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ann", "@carol"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskTitle = "task without suggestion"
	m.newTaskAssignee = "@typed"
	m.newTaskAssigneeSuggIdx = -1

	_, cmd := ntKey(m, "ctrl+s")
	mustRun(t, cmd)

	tasks, err := m.database.ListTasks("api", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range tasks {
		if tasks[i].Title == "task without suggestion" {
			if tasks[i].Assignee != "@typed" {
				t.Errorf("the assignee is %q, want @typed as-is", tasks[i].Assignee)
			}
			return
		}
	}
	t.Fatal("the task was not created")
}
