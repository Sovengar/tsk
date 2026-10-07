package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// typeTag types each character into the tag modal.
func typeTag(m Model, text string) Model {
	for _, ch := range text {
		next, _ := m.handleTagModalKey(string(ch))
		m = next.(Model)
	}
	return m
}

// openTagModal opens the detail of the first task and its tag modal.
func openTagModal(t *testing.T, m *Model) Model {
	t.Helper()
	task := (*m).tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	next, _ := m.handleDetailKey("t")
	got := next.(Model)
	if !got.tagOpen {
		t.Fatal("\"t\" did not open the tag modal")
	}
	return got
}

// TestTagModalTogglesTag verifies that Enter adds the typed tag and that, if it is
// already applied, removes it; the modal stays open after each toggle.
func TestTagModalTogglesTag(t *testing.T) {
	m := newTestModel(t)
	taskID := m.tasks[0].ID
	m2 := openTagModal(t, m)

	m2 = typeTag(m2, "blocked")
	next, cmd := m2.handleTagModalKey("enter")
	m2 = next.(Model)
	if cmd == nil {
		t.Fatal("Enter did not return a toggle command")
	}
	if m2.tagOpen != true {
		t.Error("the modal should stay open after the toggle")
	}
	if m2.tagInput != "" {
		t.Errorf("the input should be cleared after the toggle: %q", m2.tagInput)
	}
	next, _ = m2.Update(mustMsg(t, cmd))
	m2 = next.(Model)

	got, err := m2.database.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if !model.HasTag(got.Tags, "blocked") {
		t.Fatalf("the tag was not added: %v", got.Tags)
	}
	if m2.detailTask == nil || !model.HasTag(m2.detailTask.Tags, "blocked") {
		t.Errorf("the detail does not reflect the new tag: %+v", m2.detailTask)
	}

	// Second toggle: the same tag is removed.
	m2 = typeTag(m2, "blocked")
	next, cmd = m2.handleTagModalKey("enter")
	m2 = next.(Model)
	next, _ = m2.Update(mustMsg(t, cmd))
	m2 = next.(Model)

	got, _ = m2.database.GetTask(taskID)
	if model.HasTag(got.Tags, "blocked") {
		t.Errorf("the tag was not removed: %v", got.Tags)
	}
}

// TestTagModalSuggestionsCompleteAndClose covers suggestions, Tab and Esc.
func TestTagModalSuggestionsCompleteAndClose(t *testing.T) {
	m := newTestModel(t)
	taskID := m.tasks[0].ID
	if _, err := m.database.SetTaskTags(taskID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks

	m2 := openTagModal(t, m)

	if suggs := m2.tagSuggestions(); !hasOption(suggs, "blocked") {
		t.Fatalf("the suggestions do not include the existing tag: %v", suggs)
	}

	// Filter by prefix and complete with Tab.
	m2 = typeTag(m2, "block")
	if suggs := m2.tagSuggestions(); !hasOption(suggs, "blocked") {
		t.Fatalf("the prefix filter did not find blocked: %v", suggs)
	}
	next, _ := m2.handleTagModalKey("tab")
	m2 = next.(Model)
	if m2.tagInput != "blocked" {
		t.Errorf("Tab did not complete the suggestion: %q", m2.tagInput)
	}

	next, _ = m2.handleTagModalKey("esc")
	m2 = next.(Model)
	if m2.tagOpen {
		t.Error("Esc did not close the tag modal")
	}
	if !m2.detailOpen {
		t.Error("closing the tag modal should not close the detail")
	}
}

// TestRenderTagModalShowsCurrent and suggestions.
func TestRenderTagModalShowsCurrent(t *testing.T) {
	m := newTestModel(t)
	taskID := m.tasks[0].ID
	if _, err := m.database.SetTaskTags(taskID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := m.database.ListTasks("", "", "")
	m.tasks = tasks

	m2 := openTagModal(t, m)
	m2.width = 100
	out := ansi.Strip(m2.renderTagModal("base"))
	if !strings.Contains(out, "Tags") {
		t.Errorf("the modal does not show the Tags title:\n%s", out)
	}
	if !strings.Contains(out, "Current: blocked") {
		t.Errorf("the modal does not show the current tags:\n%s", out)
	}
	if !strings.Contains(out, "✓ blocked") {
		t.Errorf("the applied suggestion should be marked with ✓:\n%s", out)
	}
}
