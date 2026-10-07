package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// The tag modal mixes three things: which suggestions come out, which one
// is selected and which are already applied to the task. The tests that
// cover it looked for the text of a concrete tag, so a marking condition
// set backwards -- marking all, or none -- still showed the same words.

// tagRender returns the tag modal without colors.
func tagRender(t *testing.T, m *Model) string {
	t.Helper()
	m.width = 120
	return ansi.Strip(m.renderTagModal(""))
}

// newTagModel opens the tag modal on the first task, with different tags in
// the database so that there are suggestions.
func newTagModel(t *testing.T, tags ...string) *Model {
	t.Helper()
	m := newTestModel(t)
	for _, tag := range tags {
		if _, err := m.database.CreateTaskFull("api", "with "+tag, "", "@john", 1, "todo", 0, []string{tag}); err != nil {
			t.Fatalf("CreateTaskFull(%q): %v", tag, err)
		}
	}
	m.tasks, _ = m.database.ListTasks("", "", "")
	m.filteredT = nil

	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &task
	m.tagOpen = true
	m.tagInput = ""
	m.tagSuggestIdx = -1
	return m
}

// With no typed text all existing tags come out; typing a prefix, only those
// that start with it. The prefix is searched lowercased and without spaces.
func TestTagSuggestionsFilterByPrefix(t *testing.T) {
	m := newTagModel(t, "bug", "build", "chore")

	if got := m.tagSuggestions(); len(got) != 3 {
		t.Errorf("without typing, %d suggestions come out, want 3: %v", len(got), got)
	}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty shows everything", "", []string{"bug", "build", "chore"}},
		{"only spaces is equivalent to empty", "   ", []string{"bug", "build", "chore"}},
		{"prefix", "bu", []string{"bug", "build"}},
		{"lowercase prefix against uppercase", "BU", []string{"bug", "build"}},
		{"with surrounding spaces", "  bu  ", []string{"bug", "build"}},
		{"no matches", "zzz", nil},
		{"prefix in the middle does not count", "ug", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.tagInput = tt.input
			got := m.tagSuggestions()
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("tagSuggestions with %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// A tag that exists in no task does not come out as a suggestion, but the
// modal offers it as a new creation.
func TestTagModalOffersNewTag(t *testing.T) {
	m := newTagModel(t, "bug")
	m.tagInput = "new"

	out := tagRender(t, m)
	if !strings.Contains(out, "(new tag)") {
		t.Errorf("a tag with no suggestions does not say it:\\n%s", out)
	}
	if strings.Contains(out, "bug") {
		t.Errorf("a suggestion came out that does not start with the prefix:\\n%s", out)
	}
}

// With suggestions, the "(new tag)" does not come out: there is where to choose from.
func TestTagModalNoNewTagWhenThereAreSuggestions(t *testing.T) {
	m := newTagModel(t, "bug")
	m.tagInput = "bu"

	if out := tagRender(t, m); strings.Contains(out, "(new tag)") {
		t.Errorf("with suggestions the (new tag) still comes out:\\n%s", out)
	}
}

// The "Current:" line says (none) with no tags and the tags joined by
// commas with them. It is what tells whoever is in the modal what is already set.
func TestTagModalCurrentLine(t *testing.T) {
	m := newTagModel(t)
	m.detailTask.Tags = nil
	if out := tagRender(t, m); !strings.Contains(out, "Current: (none)") {
		t.Errorf("without tags it does not say (none):\\n%s", out)
	}

	m.detailTask.Tags = []string{"one", "two"}
	if out := tagRender(t, m); !strings.Contains(out, "Current: one, two") {
		t.Errorf("with tags it does not list them:\\n%s", out)
	}
}

// The suggestions are truncated to the cap, and what is seen is the
// beginning of the list, not a scattered subset.
func TestTagModalClipsSuggestions(t *testing.T) {
	tags := make([]string, tagMaxSuggestions+5)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%02d", i)
	}
	m := newTagModel(t, tags...)

	suggs := m.tagSuggestions()
	if len(suggs) != len(tags) {
		t.Fatalf("the model has %d suggestions, want %d", len(suggs), len(tags))
	}

	out := tagRender(t, m)
	for _, tag := range tags[:tagMaxSuggestions] {
		if !strings.Contains(out, tag) {
			t.Errorf("%q is not visible, it should be among the first %d:\\n%s", tag, tagMaxSuggestions, out)
		}
	}
	for _, tag := range tags[tagMaxSuggestions:] {
		if strings.Contains(out, tag) {
			t.Errorf("%q is visible, it is beyond the cap:\\n%s", tag, out)
		}
	}
}

// The tags the task already has come out with a tick and the others
// without it. It is the difference between "you can remove this" and "you can add this".
func TestTagModalMarksAppliedTags(t *testing.T) {
	m := newTagModel(t, "applied", "free")
	m.detailTask.Tags = []string{"applied"}
	m.tagInput = ""

	out := tagRender(t, m)
	if !strings.Contains(out, "✓ applied") {
		t.Errorf("the applied tag does not carry the tick:\\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "free") && strings.Contains(line, "✓") {
			t.Errorf("an unapplied tag carries the tick: %q", line)
		}
	}
}

// Typing resets the selection: otherwise the cursor would stay pointing at
// a suggestion that is no longer in the list.
func TestTagTypingResetsSuggestionIndex(t *testing.T) {
	m := newTagModel(t, "bug", "build")
	m.tagSuggestIdx = 1

	for _, key := range []string{"b", "u"} {
		next, _ := press(m, key)
		if next.tagSuggestIdx != -1 {
			t.Errorf("after typing %q the suggestion index is %d, want -1", key, next.tagSuggestIdx)
		}
	}

	// Backspace too: the same input is still being edited.
	m2 := newTagModel(t, "bug")
	m2.tagSuggestIdx = 0
	m2.tagInput = "bu"
	next, _ := press(m2, "backspace")
	if next.tagSuggestIdx != -1 {
		t.Errorf("after backspace the index is %d, want -1", next.tagSuggestIdx)
	}
	if next.tagInput != "b" {
		t.Errorf("after backspace the input is %q, want %q", next.tagInput, "b")
	}
}

// BUG: the space cannot be typed in the tag input.
//
// Bubbletea does not report the space as a lone character but with the
// string "space", so the handler's `len(key) == 1` condition discards it and
// the key is lost entirely. A tag with a space -- "in progress", "waiting for QA"
// -- cannot be typed by hand: it has to be chosen from the suggestions or
// the file edited.
//
// The test pins the current behavior, not the one it should have. When it
// is fixed, this expectation has to change.
func TestTagSpaceIsNotTypedIntoTheInput(t *testing.T) {
	m := newTagModel(t)
	m.tagInput = "co"

	next, _ := press(m, "space")
	if next.tagInput != "co" {
		t.Errorf("after space the input is %q, want %q unchanged (known bug)", next.tagInput, "co")
	}

	// The rest of the printable ones do pass: they are the ones the condition does see.
	m2 := newTagModel(t)
	m2.tagInput = ""
	for _, key := range []string{"a", "Z", "1", "9", "-"} {
		next, _ := press(m2, key)
		if next.tagInput == "" {
			t.Errorf("the key %q did not reach the input:\n%s", key, ansi.Strip(m2.renderTagModal("")))
		}
		m2.tagInput = next.tagInput
	}
}

// Esc closes the modal and clears the state, including the suggestion index.
func TestTagEscResetsState(t *testing.T) {
	m := newTagModel(t, "bug")
	m.tagSuggestIdx = 2
	m.tagInput = "bu"

	next, _ := press(m, "esc")
	if next.tagOpen {
		t.Error("esc did not close the modal")
	}
	if next.tagInput != "" {
		t.Errorf("the input ended up as %q", next.tagInput)
	}
	if next.tagSuggestIdx != -1 {
		t.Errorf("the index ended up as %d, want -1", next.tagSuggestIdx)
	}
}

// The tick of an applied tag depends on the detail's task: with no open task
// there is nothing to compare against.
func TestTagModalWithoutOpenTask(t *testing.T) {
	m := newTagModel(t, "bug")
	m.detailTask = nil
	m.detailOpen = false

	out := tagRender(t, m)
	if strings.Contains(out, "✓") {
		t.Errorf("with no open task a tick comes out:\\n%s", out)
	}
	if !strings.Contains(out, "Current: (none)") {
		t.Errorf("with no open task:\\n%s", out)
	}
}

// hasTag is what decides the render's tick, so the render and the business
// rule cannot disagree.
func TestTagAppliedFlagMatchesHasTag(t *testing.T) {
	m := newTagModel(t, "applied", "free")
	m.detailTask.Tags = []string{"applied"}
	m.tagInput = ""

	out := tagRender(t, m)
	for _, tag := range []string{"applied", "free"} {
		lineWithTag := ""
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, tag) && !strings.Contains(line, "Current:") {
				lineWithTag = line
				break
			}
		}
		if lineWithTag == "" {
			t.Errorf("I did not find the row of %q:\n%s", tag, out)
			continue
		}
		hasTick := strings.Contains(lineWithTag, "✓")
		if hasTick != model.HasTag(m.detailTask.Tags, tag) {
			t.Errorf("%q: the render says tick=%v and HasTag=%v", tag, hasTick, model.HasTag(m.detailTask.Tags, tag))
		}
	}
}

// The selected row goes in bold, and only one: with the index at -1 (nothing
// selected) none is in bold, and with a valid index exactly that one.
// The tick and the bold are different marks and do not replace each other.
func TestTagModalHighlightsExactlyOneSuggestion(t *testing.T) {
	tests := []struct {
		name  string
		idx   int
		wantN int
	}{
		{"nothing selected", -1, 0},
		{"first", 0, 1},
		{"the middle one", 1, 1},
		{"the last one", 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTagModel(t, "aaa", "bbb", "ccc")
			m.tagInput = ""
			m.tagSuggestIdx = tt.idx

			raw := m.renderTagModal("")
			if got := countBoldLines(raw); got != tt.wantN {
				t.Errorf("with the index at %d there are %d bold rows, want %d", tt.idx, got, tt.wantN)
			}
		})
	}
}

// countBoldLines counts the modal's lines carrying the selection style,
// which is bold. It is counted on the render without removing the codes: if
// they were removed, the three rows would be indistinguishable.
func countBoldLines(rendered string) int {
	n := 0
	for _, line := range strings.Split(rendered, "\n") {
		// The selection style opens with lipgloss's bold sequence.
		if strings.Contains(line, "\x1b[1m") {
			n++
		}
	}
	return n
}

// Tab and Enter complete and consume: both clear the suggestion selection,
// because the input becomes the full value and the suggestion list is
// going to change.
func TestTagTabAndEnterClearSuggestionIndex(t *testing.T) {
	tests := []struct {
		key       string
		wantInput string
	}{
		// Tab completes: the input becomes the whole suggestion.
		{"tab", "build"},
		// Enter adds it and leaves the input empty for the next one.
		{"enter", ""},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := newTagModel(t, "bug", "build")
			m.tagInput = "bu"
			m.tagSuggestIdx = 1

			next, _ := press(m, tt.key)
			got := next

			if got.tagSuggestIdx != -1 {
				t.Errorf("after %q the index is %d, want -1", tt.key, got.tagSuggestIdx)
			}
			if got.tagInput != tt.wantInput {
				t.Errorf("after %q the input is %q, want %q", tt.key, got.tagInput, tt.wantInput)
			}
		})
	}
}

// countBoldLinesWith counts the render's lines carrying the selection bold
// and also contain marker. The second filter is needed when the modal will
// highlight more than one thing -- the person's name, for example -- and
// what matters is which of the lists is marked.
func countBoldLinesWith(rendered, marker string) int {
	n := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "\x1b[1m") && strings.Contains(line, marker) {
			n++
		}
	}
	return n
}
