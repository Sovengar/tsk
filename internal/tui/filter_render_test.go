package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/db"
)

// The filter modal mixes three things in the same render: what value each
// field has, how the applied one and the cursor one are marked, and the
// truncation of the options list. The tests that cover it looked for loose
// words, so a marking condition set backwards -- marking all instead of the
// chosen one, or none -- still showed the same words.

// filterRender returns the filter modal without colors.
func filterRender(t *testing.T, m *Model) string {
	t.Helper()
	m.width = 120
	return ansi.Strip(m.renderFilterModal(""))
}

// The value shown of each field: empty means "all", and the filter's empty
// priority is -1, not 0. The 0 is "no priority", which is a real filter.
func TestFilterCurrentValueLabels(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
		field int
		want  string
	}{
		{"empty project", nil, filterFieldProject, "all"},
		{"project with a value", func(m *Model) { m.filterProject = "api" }, filterFieldProject, "api"},
		{"empty status", nil, filterFieldStatus, "all"},
		{"status with a value", func(m *Model) { m.filterStatus = "doing" }, filterFieldStatus, "doing"},
		{"default status", func(m *Model) { m.filterStatus = statusFilterAllActive }, filterFieldStatus, statusFilterAllActive},
		{"empty assignee", nil, filterFieldAssignee, "all"},
		{"assignee with a value", func(m *Model) { m.filterAssignee = "@john" }, filterFieldAssignee, "@john"},
		{"priority unfiltered", func(m *Model) { m.filterPriority = -1 }, filterFieldPriority, "all"},
		{"priority zero", func(m *Model) { m.filterPriority = 0 }, filterFieldPriority, "none"},
		{"low priority", func(m *Model) { m.filterPriority = 1 }, filterFieldPriority, "low"},
		{"medium priority", func(m *Model) { m.filterPriority = 2 }, filterFieldPriority, "med"},
		{"high priority", func(m *Model) { m.filterPriority = 3 }, filterFieldPriority, "high"},
		{"empty tag", nil, filterFieldTag, "all"},
		{"tag with a value", func(m *Model) { m.filterTag = "bug" }, filterFieldTag, "bug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			if tt.setup != nil {
				tt.setup(m)
			}
			if got := m.filterCurrentValue(tt.field); got != tt.want {
				t.Errorf("field %d value = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

// A field without a recognised value falls on "all": the priority switch has
// no case for a -2, and even so the modal has to show something.
func TestFilterCurrentValueUnknownFallsBack(t *testing.T) {
	for _, p := range []int{-2, 4, 99} {
		m := &Model{filterPriority: p}
		if got := m.filterCurrentValue(filterFieldPriority); got != "all" {
			t.Errorf("priority %d = %q, want all", p, got)
		}
	}
	if got := (&Model{}).filterCurrentValue(filterFieldCount); got != "all" {
		t.Errorf("out-of-range field = %q, want all", got)
	}
}

// With the project field focused and no search, the input line shows the
// current value dimmed. With a search, it shows what was typed.
func TestFilterInputShowsCurrentValueOrSearch(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = filterFieldProject
	m.filterProject = "api"

	if !strings.Contains(filterRender(t, m), "api") {
		t.Errorf("without a search the current value is not visible:\n%s", filterRender(t, m))
	}

	m.filterSearch = "we"
	if !strings.Contains(filterRender(t, m), "we") {
		t.Errorf("with a search the typed text is not visible:\n%s", filterRender(t, m))
	}
	if strings.Contains(filterRender(t, m), "api") {
		t.Errorf("with a search the current value is still visible:\n%s", filterRender(t, m))
	}
}

// With more options than fit, the "n/total" footer shows; with exactly enough, not.
// The cap also counts the "all" option, so the edge is one label before
// what it looks like. The cases are placed on both sides.
func TestFilterOptionsFooter(t *testing.T) {
	tests := []struct {
		name string
		tags int
	}{
		{"two below", filterMaxVisibleOptions - 2},
		{"exactly at the cap", filterMaxVisibleOptions - 1},
		{"one too many", filterMaxVisibleOptions},
		{"many too many", filterMaxVisibleOptions + 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.filterOpen = true
			m.filterFieldIdx = filterFieldTag
			for i := range tt.tags {
				mustCreateTaskWithTags(t, m.database, "api", "task", "", "@john", 1, "todo",
					[]string{fmt.Sprintf("tag%02d", i)})
			}
			m.tasks, _ = m.database.ListTasks("", "", "")
			m.filteredT = nil

			total := len(m.filterFieldOptions(filterFieldTag))
			want := total > filterMaxVisibleOptions

			out := filterRender(t, m)
			if got := strings.Contains(out, "/"+strconv.Itoa(total)); got != want {
				t.Errorf("with %d options (%d tags) the footer is %v, want %v\n%s",
					total, tt.tags, got, want, out)
			}

			// The number is the cursor position plus one, and with two tags the
			// cursor can move. It is what distinguishes a "+1" from a "+0": only
			// by searching the full prefix, because "4/12" is also inside
			// "14/12".
			if total > filterMaxVisibleOptions {
				for idx := range min(3, total) {
					m.filterOptionIdx = idx
					footer := fmt.Sprintf("%d/%d", idx+1, total)
					if out := filterRender(t, m); !strings.Contains(out, footer) {
						t.Errorf("with the cursor at %d the footer does not say %q:\n%s", idx, footer, out)
					}
				}
			}
		})
	}
}

// The applied value carries a dot and the cursor one a triangle, and they are
// not the same row unless they match. Here they are separated on purpose: the
// applied one is "all" and the cursor is on the second option.
func TestFilterMarksAppliedAndCursorSeparately(t *testing.T) {
	m := newTestModel(t)
	m.filterOpen = true
	m.filterFieldIdx = filterFieldProject
	m.filterProject = "" // applied: "all", which is the first option
	m.filterOptionIdx = 1

	out := filterRender(t, m)
	// "all" is the first one and it is applied: only the dot.
	if !strings.Contains(out, "● all") {
		t.Errorf("the applied option does not carry the dot:\n%s", out)
	}
	// The second one is the cursor's: only the triangle.
	if !strings.Contains(out, "▸ ") {
		t.Errorf("the cursor is not visible:\n%s", out)
	}
	// No row carries both: that would be the "applied and selected" case.
	if strings.Contains(out, "● ▸") || strings.Contains(out, "▸ ●") {
		t.Errorf("a row carries both marks, it should not:\\n%s", out)
	}
}

// mustCreateTaskWithTags creates a task with tags, which is what makes the
// modal's tag field have options to list.
func mustCreateTaskWithTags(t *testing.T, database *db.DB, projectName, title, description, assignee string, priority int, status string, tags []string) {
	t.Helper()
	if _, err := database.CreateTaskFull(projectName, title, description, assignee, priority, status, 0, tags); err != nil {
		t.Fatalf("CreateTaskFull(%q): %v", title, err)
	}
}
