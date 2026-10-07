package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// TestNewTaskOpenDefaults verifies the initial state of the form: focus on
// Priority, assignee "Me" and active overlay.
func TestNewTaskOpenDefaults(t *testing.T) {
	m := newTestModel(t)

	m, _ = press(m, "i")
	if !m.newTaskOpen {
		t.Fatal("i must open the new task modal")
	}
	if m.newTaskFieldIdx != newTaskFieldPriority {
		t.Errorf("initial focus = %d, want priority", m.newTaskFieldIdx)
	}
	if m.newTaskAssignee != "Me" {
		t.Errorf("initial assignee = %q, want Me", m.newTaskAssignee)
	}
	if m.newTaskPriority != model.PriorityLow {
		t.Errorf("initial priority = %d, want low", m.newTaskPriority)
	}
	if m.newTaskProject != "api" {
		t.Errorf("project = %q, want api", m.newTaskProject)
	}
	if m.overlayKind() != overlayNewTask {
		t.Errorf("overlay = %v, want overlayNewTask", m.overlayKind())
	}
	// "Me" matches exactly: it must not suggest itself.
	if got := m.assigneeSuggestions(); len(got) != 0 {
		t.Errorf("with assignee Me there must be no suggestions, got %v", got)
	}
}

// TestNewTaskTabNavigation verifies that Tab/Shift+Tab walk the fields in
// order and wrap around without getting lost.
func TestNewTaskTabNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	order := []int{
		newTaskFieldTitle,
		newTaskFieldDescription,
		newTaskFieldAssignee,
		newTaskFieldTags,
		newTaskFieldPriority,
	}
	for _, want := range order {
		m, _ = press(m, "tab")
		if m.newTaskFieldIdx != want {
			t.Fatalf("after tab: field = %d, want %d", m.newTaskFieldIdx, want)
		}
	}

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.newTaskFieldIdx != newTaskFieldTags {
		t.Errorf("shift+tab from priority: field = %d, want tags", m.newTaskFieldIdx)
	}
}

// TestNewTaskPriorityKeys verifies priority selection with 1-4 and arrows.
func TestNewTaskPriorityKeys(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	m, _ = press(m, "3")
	if m.newTaskPriority != 3 {
		t.Errorf("priority = %d, want 3", m.newTaskPriority)
	}
	m, _ = press(m, "left")
	if m.newTaskPriority != 2 {
		t.Errorf("after left: priority = %d, want 2", m.newTaskPriority)
	}
	m, _ = press(m, "right")
	if m.newTaskPriority != 3 {
		t.Errorf("after right: priority = %d, want 3", m.newTaskPriority)
	}
	m, _ = press(m, "1")
	if m.newTaskPriority != 1 {
		t.Errorf("after 1: priority = %d, want 1", m.newTaskPriority)
	}
}

// TestNewTaskTypingOnPriorityJumpsToTitle verifies that typing a letter over
// the priority selector jumps to the title and inserts it.
func TestNewTaskTypingOnPriorityJumpsToTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	m, _ = press(m, "H")
	if m.newTaskFieldIdx != newTaskFieldTitle {
		t.Errorf("typing over priority must focus Title, got %d", m.newTaskFieldIdx)
	}
	if m.newTaskTitle != "H" {
		t.Errorf("title = %q, want H", m.newTaskTitle)
	}
}

// TestNewTaskRequiresTitle verifies the inline validation without closing the modal.
func TestNewTaskRequiresTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	m, _ = press(m, "enter") // priority -> title
	if m.newTaskFieldIdx != newTaskFieldTitle {
		t.Fatalf("enter on priority must advance to Title, got %d", m.newTaskFieldIdx)
	}
	m, _ = press(m, "enter") // submit without a title
	if !m.newTaskOpen {
		t.Fatal("the modal must not close without a title")
	}
	if m.newTaskErr == "" {
		t.Error("the inline required-title error was expected")
	}
}

// TestNewTaskSubmitsFromTitle verifies that Enter on Title creates and closes,
// leaving the toast's feedback.
func TestNewTaskSubmitsFromTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")
	m, _ = press(m, "tab") // title

	for _, ch := range []string{"B", "u", "g"} {
		m, _ = press(m, ch)
	}
	if m.newTaskTitle != "Bug" {
		t.Fatalf("title = %q, want Bug", m.newTaskTitle)
	}

	m, cmd := press(m, "enter")
	if m.newTaskOpen {
		t.Error("Enter on Title must close the modal")
	}
	if cmd == nil {
		t.Error("the creation cmd was expected")
	}
	if m.toast != "Task created" {
		t.Errorf("toast = %q, want 'Task created'", m.toast)
	}
}

// TestNewTaskDescriptionUsesInlineEditor verifies that the description is
// edited with the embedded textarea and that Enter inserts a break instead of creating.
func TestNewTaskDescriptionUsesInlineEditor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")
	m, _ = press(m, "tab") // title
	m, _ = press(m, "tab") // description

	if m.newTaskFieldIdx != newTaskFieldDescription {
		t.Fatalf("field = %d, want description", m.newTaskFieldIdx)
	}

	m, _ = press(m, "h")
	if got := m.newTaskTextarea.Value(); got != "h" {
		t.Fatalf("textarea = %q, want h", got)
	}

	m, _ = press(m, "enter")
	if !m.newTaskOpen {
		t.Fatal("Enter on Description must not create")
	}
	if !strings.Contains(m.newTaskTextarea.Value(), "\n") {
		t.Errorf("Enter must insert a line break, got %q", m.newTaskTextarea.Value())
	}
}

// TestNewTaskAssigneeAutocomplete verifies the fuzzy filtering, selection with
// ↓ and completion with Enter without creating the task.
func TestNewTaskAssigneeAutocomplete(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")
	m, _ = press(m, "tab") // title
	m, _ = press(m, "tab") // description
	m, _ = press(m, "tab") // assignee

	// Clear the default "Me".
	m, _ = press(m, "backspace")
	m, _ = press(m, "backspace")
	if m.newTaskAssignee != "" {
		t.Fatalf("assignee = %q, want empty", m.newTaskAssignee)
	}

	m, _ = press(m, "@")
	m, _ = press(m, "m")
	suggs := m.assigneeSuggestions()
	if len(suggs) == 0 || suggs[0] != "@margo" {
		t.Fatalf("suggestions = %v, want @margo first", suggs)
	}

	m, _ = press(m, "down")
	if m.newTaskAssigneeSuggIdx != 0 {
		t.Fatalf("suggIdx = %d, want 0", m.newTaskAssigneeSuggIdx)
	}
	m, _ = press(m, "enter")
	if m.newTaskAssignee != "@margo" {
		t.Errorf("assignee = %q, want @margo", m.newTaskAssignee)
	}
	if !m.newTaskOpen {
		t.Error("Enter on a suggestion must complete, not create")
	}
	// With the full name left, it must not repeat itself as a suggestion.
	if got := m.assigneeSuggestions(); len(got) != 0 {
		t.Errorf("an exact match must not be suggested: %v", got)
	}
}

// goNewTaskTags opens the form and focuses the Tags field.
func goNewTaskTags(t *testing.T, m *Model) *Model {
	t.Helper()
	m, _ = press(m, "i")
	for i := 0; i < 4; i++ {
		m, _ = press(m, "tab")
	}
	if m.newTaskFieldIdx != newTaskFieldTags {
		t.Fatalf("field = %d, want tags", m.newTaskFieldIdx)
	}
	return m
}

// TestNewTaskTagsAddAndRemove verifies adding with Enter and comma, and
// deleting the last tag with backspace on the empty input.
func TestNewTaskTagsAddAndRemove(t *testing.T) {
	m := newTestModel(t)
	m = goNewTaskTags(t, m)

	for _, ch := range []string{"b", "a", "c", "k"} {
		m, _ = press(m, ch)
	}
	m, _ = press(m, "enter")
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "back" {
		t.Fatalf("tags = %v, want [back]", m.newTaskTags)
	}
	if m.newTaskTagInput != "" {
		t.Errorf("input = %q, want empty after commit", m.newTaskTagInput)
	}

	m, _ = press(m, "f")
	m, _ = press(m, ",")
	if len(m.newTaskTags) != 2 || m.newTaskTags[1] != "f" {
		t.Fatalf("tags = %v, want [back f]", m.newTaskTags)
	}

	m, _ = press(m, "backspace")
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "back" {
		t.Errorf("after backspace: tags = %v, want [back]", m.newTaskTags)
	}
}

// TestNewTaskTagsCommitOnTab verifies that leaving the field does not lose the
// half-typed tag.
func TestNewTaskTagsCommitOnTab(t *testing.T) {
	m := newTestModel(t)
	m = goNewTaskTags(t, m)

	m, _ = press(m, "x")
	m, _ = press(m, "tab") // tags -> priority
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "x" {
		t.Errorf("tags = %v, want [x] after leaving the field", m.newTaskTags)
	}
}

// TestNewTaskTagsAutocomplete verifies the fuzzy filtering and completion
// with Enter without creating the task.
func TestNewTaskTagsAutocomplete(t *testing.T) {
	m := newTestModel(t)
	m.tasks[0].Tags = []string{"backend", "frontend"}
	m = goNewTaskTags(t, m)

	for _, ch := range []string{"b", "a", "c", "k"} {
		m, _ = press(m, ch)
	}
	suggs := m.tagFieldSuggestions()
	if len(suggs) == 0 || suggs[0] != "backend" {
		t.Fatalf("suggestions = %v, want backend first", suggs)
	}

	m, _ = press(m, "down")
	m, _ = press(m, "enter")
	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "backend" {
		t.Errorf("tags = %v, want [backend]", m.newTaskTags)
	}
	if !m.newTaskOpen {
		t.Error("Enter on a tag suggestion must not create the task")
	}
	// The tag already chosen is not suggested again (even if the dropdown lists the rest).
	if got := m.tagFieldSuggestions(); containsFold(got, "backend") {
		t.Errorf("already added tags must not be suggested: %v", got)
	}
}

// TestNewTaskModalRenders verifies that the modal shows all the fields and
// does not overflow the terminal width.
func TestNewTaskModalRenders(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "i")

	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"New Task", "Priority", "Title", "Description", "Assignee", "Me", "Tags", "optional"} {
		if !strings.Contains(out, want) {
			t.Errorf("the modal does not show %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("line of width %d exceeds the width %d: %q", w, m.width, line)
		}
	}
}

// TestFuzzyFilter covers the shared matcher of the autocomplete.
func TestFuzzyFilter(t *testing.T) {
	items := []string{"@margo", "@john", "Me"}

	if got := fuzzyFilter(items, "mar", 5); len(got) != 1 || got[0] != "@margo" {
		t.Errorf("fuzzyFilter(mar) = %v, want [@margo]", got)
	}

	// Substring with an earlier match wins.
	if got := fuzzyFilter([]string{"@john", "@margo"}, "a", 5); got[0] != "@margo" {
		t.Errorf("order by score = %v, want @margo first", got)
	}

	// Empty query respects the original order and the cap.
	if got := fuzzyFilter(items, "", 2); len(got) != 2 || got[0] != "@margo" {
		t.Errorf("fuzzyFilter('') = %v, want the first two in order", got)
	}

	// Subsequence and discard.
	if _, ok := fuzzyScore("jn", "@john"); !ok {
		t.Error("jn should match @john as a subsequence")
	}
	if _, ok := fuzzyScore("zz", "@john"); ok {
		t.Error("zz should not match @john")
	}
}
