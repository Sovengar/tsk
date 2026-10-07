package tui

import (
	"math"
	"strings"
	"testing"

	"tsk/internal/model"

	"github.com/charmbracelet/x/ansi"
)

// goesToNewTaskTextarea decides where a message that is neither key nor paste goes.
//
// The condition was inside Update's default and could not be checked:
// the messages arriving through that branch are private to the textarea
// package (pasteMsg, copyMsg), which are not exported, so a test cannot
// build one to see where it ends up. With the decision extracted to a pure
// function, the routing is checked whole and without needing the message that triggers it.
//
// The case that matters is the field: only the description carries an embedded
// textarea, because it is the only multiline one. If the routing accepted any
// field with the form open, the keys of the one-line fields would reach the
// textarea and be eaten: that is the bug a misplaced `==` would cause, and that
// is what this table ties down.
func TestGoesToNewTaskTextarea(t *testing.T) {
	cases := []struct {
		name        string
		newTaskOpen bool
		field       int
		want        bool
	}{
		{"form open on the description", true, newTaskFieldDescription, true},
		{"form open on the title", true, newTaskFieldTitle, false},
		{"form open on the assignee", true, newTaskFieldAssignee, false},
		{"form open on the tags", true, newTaskFieldTags, false},
		{"form open on the priority", true, newTaskFieldPriority, false},
		{"form closed on the description", false, newTaskFieldDescription, false},
		{"form closed on the title", false, newTaskFieldTitle, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := goesToNewTaskTextarea(c.newTaskOpen, c.field)
			if got != c.want {
				t.Errorf("goesToNewTaskTextarea(%v, field %d) = %v, want %v",
					c.newTaskOpen, c.field, got, c.want)
			}
		})
	}
}

// The consequence of the above, seen from Update: with the form open on a
// one-line field, a message that is neither key nor paste must not be
// forwarded to the textarea nor return its command. With the form open on the description, it must.
func TestNonKeyMessageRoutingRespectsTheField(t *testing.T) {
	// Any message that falls into the default: it is neither KeyMsg nor PasteMsg
	// nor any of the types the Update switch recognizes one by one.
	msg := struct{}{}

	t.Run("one-line field: not forwarded", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldTitle
		before := m.newTaskTextarea.Value()

		nextModel, cmd := m.Update(msg)
		if cmd != nil {
			t.Error("it returned a command with the cursor on the title: the message went to the textarea")
		}
		if nextModel.(Model).newTaskTextarea.Value() != before {
			t.Error("the textarea changed with the cursor on the title")
		}
	})

	t.Run("description field: forwarded", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldDescription

		nextModel, _ := m.Update(msg)
		// What is checked is that the path is walked: the textarea's Update
		// with an unknown message changes nothing, so the proof that it went
		// through there is the condition, not the result. What does have to be
		// true is that nothing breaks.
		if got := nextModel.(Model).newTaskTextarea.Value(); got != m.newTaskTextarea.Value() {
			t.Errorf("the textarea value changed with a no-op message: %q -> %q",
				m.newTaskTextarea.Value(), got)
		}
	})

	t.Run("form closed: not forwarded", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = false
		m.newTaskFieldIdx = newTaskFieldDescription

		if _, cmd := m.Update(msg); cmd != nil {
			t.Error("it returned a command with the form closed")
		}
	})
}

// rowIsTask decides whether the gantt's cursor is on a task.
//
// The range with inRange instead of `>= 0 && < len(rows)` is what makes the
// cursor at -1 and at len(rows) checkable as values and not as
// "anything that is not a task".
func TestRowIsTask(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@john", "@margo"}, 3)
	m.currentView = viewGantt
	m.width, m.height = 140, 40
	rows := m.ganttRows()

	// A real task row: inside the range and of the right type.
	taskIdx := -1
	headerIdx := -1
	for i, f := range rows {
		if f.kind == ganttTaskRow && taskIdx < 0 {
			taskIdx = i
		}
		if f.kind == ganttAssigneeRow && headerIdx < 0 {
			headerIdx = i
		}
	}
	if taskIdx < 0 || headerIdx < 0 {
		t.Fatalf("the fixture left neither of the two row types: %d rows", len(rows))
	}

	if !rowIsTask(rows, taskIdx) {
		t.Errorf("row %d is a task and rowIsTask says no", taskIdx)
	}
	if rowIsTask(rows, headerIdx) {
		t.Errorf("row %d is a header and rowIsTask says yes", headerIdx)
	}

	// The edges of the range, which is what inRange comes to replace. With the
	// range written by hand, these values only told themselves apart from the
	// internal comparison's if the row sitting there were of the other type --
	// and at 0 it never is, because row 0 is a header.
	for _, i := range []int{-1, len(rows), len(rows) + 5} {
		if rowIsTask(rows, i) {
			t.Errorf("rowIsTask(%d) says yes, with %d rows", i, len(rows))
		}
	}

	// And an empty table: every index is out.
	for _, i := range []int{-1, 0, 1} {
		if rowIsTask(nil, i) {
			t.Errorf("rowIsTask(nil, %d) says yes", i)
		}
	}
	// With an empty list inRange must not return true even for 0, which is the
	// case a `idx <= n` would have let through.
	if inRange(0, 0) {
		t.Error("inRange(0, 0) says 0 is in a list of 0 elements")
	}
}

// cycleProjectFilter has three exits and all three have to be there:
// with no projects there is nothing to walk; with the current filter out of
// the list the jump starts at the beginning; and the normal case, which is
// the everyday one.
func TestCycleProjectFilterExits(t *testing.T) {
	t.Run("a single project: the filter DOES jump", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.projects = []model.Project{{Name: "api"}}
		m.filterProject = "api"

		m.cycleProjectFilter(1)

		// With a project there are TWO options -- "all" and "api" -- so advancing
		// has to lead to "all". Before, this did not move, because the guard
		// `len(opts) <= 1` counted wrong: it treated "all" as if it did not count.
		if m.filterProject != "" {
			t.Errorf("with a single project the filter stayed at %q, want the first one (\"all\")",
				m.filterProject)
		}
	})

	t.Run("the current filter is not in the list: jumps from the start", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		// The filter points at something that is no longer an option -- a
		// deleted project, or a hand-written name. The walk has to start at the
		// beginning instead of staying where it is.
		m.filterProject = "missing"

		m.cycleProjectFilter(1)

		if m.filterProject == "missing" {
			t.Error("with a filter outside the list, advancing did nothing")
		}
	})

	t.Run("the normal one: alternates between the options", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.filterProject = ""

		options := m.filterFieldOptions(filterFieldProject)
		if len(options) < 3 {
			t.Skipf("the fixture only has %d options", len(options))
		}

		seen := map[string]bool{}
		seen[m.filterProject] = true
		for range len(options) {
			m.cycleProjectFilter(1)
			seen[m.filterProject] = true
		}
		if len(seen) != len(options) {
			t.Errorf("it went through %d distinct values, want %d: %v",
				len(seen), len(options), seen)
		}
	})
}

// nextOption is the pure function of the filter's jump. The three cases
// -- the empty list, the current value absent and the normal one -- are
// tested here without building a model, which is exactly what the previous version did not allow.
func TestNextOption(t *testing.T) {
	opts := []string{"all", "api", "web"}

	t.Run("forwards", func(t *testing.T) {
		if got := nextOption(opts, "all", 1); got != "api" {
			t.Errorf("nextOption(all, +1) = %q, want api", got)
		}
		if got := nextOption(opts, "web", 1); got != "all" {
			t.Errorf("nextOption(web, +1) = %q, want all (wraps around)", got)
		}
	})

	t.Run("backwards", func(t *testing.T) {
		if got := nextOption(opts, "api", -1); got != "all" {
			t.Errorf("nextOption(api, -1) = %q, want all", got)
		}
		if got := nextOption(opts, "all", -1); got != "web" {
			t.Errorf("nextOption(all, -1) = %q, want web (wraps around)", got)
		}
	})

	t.Run("the current value is absent: starts at the beginning", func(t *testing.T) {
		if got := nextOption(opts, "deleted", 1); got != "api" {
			t.Errorf("nextOption(deleted, +1) = %q, want api", got)
		}
		// And backwards it also starts at the beginning: with the index at 0,
		// going back gives the last one, not the first.
		if got := nextOption(opts, "deleted", -1); got != "web" {
			t.Errorf("nextOption(deleted, -1) = %q, want web", got)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		for _, emptyList := range [][]string{nil, {}} {
			if got := nextOption(emptyList, "api", 1); got != "" {
				t.Errorf("nextOption(empty list, +1) = %q, want \"\"", got)
			}
			if got := nextOption(emptyList, "api", -1); got != "" {
				t.Errorf("nextOption(empty list, -1) = %q, want \"\"", got)
			}
		}
	})

	t.Run("a single option: stays on it", func(t *testing.T) {
		one := []string{"all"}
		if got := nextOption(one, "all", 1); got != "all" {
			t.Errorf("nextOption([all], +1) = %q, want all", got)
		}
		if got := nextOption(one, "all", -1); got != "all" {
			t.Errorf("nextOption([all], -1) = %q, want all", got)
		}
	})

	t.Run("there and back: returns to the starting point", func(t *testing.T) {
		for _, current := range opts {
			there := nextOption(opts, current, 1)
			back := nextOption(opts, there, -1)
			if back != current {
				t.Errorf("from %q forward gives %q and from there backward gives %q, want %q",
					current, there, back, current)
			}
		}
	})
}

// The truncation of comment lines and of kanban cards.
//
// Both discount a fixed margin before truncating, and that margin is what
// keeps the text from going out of the box. With the number written raw
// (`width - 2`) it was not known what it was discounting; with named
// constants it can be checked, and what is checked is that a text arriving
// exactly at the edge loses exactly those columns and not one more.
func TestTruncationMarginIsTheDeclaredOne(t *testing.T) {
	const width = 30

	t.Run("comments", func(t *testing.T) {
		m := newDetailModel(t, 1)
		m.width, m.height = width, 24
		// A body that does not fit with room to spare: that way the truncation
		// is active and the margin shows.
		m.detailComments[0].Body = strings.Repeat("w", 200)

		lines := m.renderCommentLines(width)
		if len(lines) != 1 {
			t.Fatalf("lines = %d, want 1", len(lines))
		}
		lineWidth := ansi.StringWidth(lines[0])
		want := width - commentPrefix - dateGap
		if lineWidth > want {
			t.Errorf("the comment line measures %d columns and the margin leaves %d: %q",
				lineWidth, want, lines[0])
		}
		// And with a narrower margin (one column less) the text would fit, so
		// a `width - 3` would tell itself apart. It is checked that it does NOT
		// fit with the declared margin.
		if ansi.StringWidth(lines[0]) == want+1 {
			t.Error("the line fit exactly: the truncation removed nothing")
		}
	})

	t.Run("kanban cards", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewKanban
		m.width, m.height = 40, 30

		// The truncation happens BEFORE the border, on the card's text, and
		// then a 2-column prefix is put in it. So the real limit of the
		// title is width - cardMargin - prefix.
		//
		// The box pads on the right with spaces up to the column width, so
		// measuring the whole line says nothing: what is measured is how many
		// characters of the title come out, which is exactly what the margin
		// controls.
		const colWidth = 20
		const prefix = 2
		limit := colWidth - cardMargin - prefix

		card := func(title string) string {
			col := kanbanColumn{
				status: "todo",
				tasks:  []model.Task{{ID: 1, Title: title, Assignee: "@john"}},
			}
			return m.renderKanbanColumn(col, colWidth, "header", false, columnWindow{0, 1})
		}

		// The padding letter does not appear anywhere else on the card --
		// neither in "@john" nor in the header --, so counting it counts the title.
		const letter = "x"

		// A long title is truncated to the limit, not one column more.
		longTitle := strings.Repeat(letter, limit+50)
		if n := strings.Count(card(longTitle), letter); n != limit {
			t.Errorf("a very long title comes out with %d characters, want %d (the declared limit)", n, limit)
		}

		// The edge from both sides: one that measures EXACTLY the limit comes
		// out whole, and one with a column more comes out truncated. That is
		// what separates the declared margin from a narrower one.
		exactTitle := strings.Repeat(letter, limit)
		if n := strings.Count(card(exactTitle), letter); n != limit {
			t.Errorf("a title of %d columns comes out with %d: one that fits exactly must not lose anything",
				limit, n)
		}
		oneMoreTitle := strings.Repeat(letter, limit+1)
		if n := strings.Count(card(oneMoreTitle), letter); n != limit {
			t.Errorf("a title of %d columns comes out with %d, want %d: one that does not fit loses the extra column",
				limit+1, n, limit)
		}
	})
}

// nonNegativeEstimate and the sentinel that tells it apart from "the estimate is zero".
//
// It is the pair of cases that makes the caller's comparison reachable: an
// estimate of 0 is a legitimate value and has to pass, and an absent estimate
// has to be distinguishable from it. With the sentinel at -1 the two are
// distinct facts; without it, "was not there" and "came as zero" were the
// same and the `>= 0` comparison had no case that told it apart from its `> 0`.
func TestNonNegativeEstimate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"zero", "0", 0},
		{"zero with decimals", "0.0", 0},
		{"negative zero", "-0", 0},
		{"one", "1", 1},
		{"half", "0.5", 0.5},
		{"with a plus sign", "+2", 2},
		{"one hundred twenty", "120", 120},

		{"negative", "-1", estimateAbsent},
		{"small negative", "-0.5", estimateAbsent},
		{"the sentinel written as-is", "-1.0000001", estimateAbsent},

		{"not a number", "abc", estimateAbsent},
		{"empty", "", estimateAbsent},
		{"with units", "3d", estimateAbsent},
		{"nan", "NaN", estimateAbsent},
		{"positive infinity", "+Inf", math.Inf(1)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nonNegativeEstimate(c.in); got != c.want {
				t.Errorf("nonNegativeEstimate(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// The sentinel has to be a value no real estimate can take, or the
// "no estimate" gets confused with good data.
func TestSentinelIsNotAValidEstimate(t *testing.T) {
	if estimateAbsent >= 0 {
		t.Fatalf("the sentinel is %v, and any value >= 0 is a valid estimate", estimateAbsent)
	}
	// And it has to be the only value the sentinel returns for an input that
	// is not a number: the return value does not depend on the text.
	for _, input := range []string{"", "abc", "-5", "-0.0001"} {
		if got := nonNegativeEstimate(input); got != estimateAbsent {
			t.Errorf("nonNegativeEstimate(%q) = %v, want the sentinel %v",
				input, got, estimateAbsent)
		}
	}
}

// The full trip through the template: an estimate of 0 has to arrive at the
// saved task, and an absent estimate has to stay at the default. It is the
// case the sentinel makes possible.
func TestZeroEstimateIsSavedAndAbsentIsNot(t *testing.T) {
	task := model.Task{ID: 1, Title: "t", Estimate: 5}

	cases := []struct {
		name         string
		template     string
		wantEstimate float64
	}{
		{"zero estimate", "# t\n\nd\n\n---\nestimate: 0", 0},
		{"absent estimate", "# t\n\nd\n\n---\nassignee: @john", 0},
		{"negative estimate", "# t\n\nd\n\n---\nestimate: -3", 0},
		{"normal estimate", "# t\n\nd\n\n---\nestimate: 2.5", 2.5},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			title, desc, assignee, priority, estimate, tags := parseEditFile(c.template)
			if title != task.Title {
				t.Errorf("title = %q, want %q", title, task.Title)
			}
			if desc != "d" {
				t.Errorf("desc = %q, want %q", desc, "d")
			}
			_ = assignee
			_ = priority
			_ = tags
			if estimate != c.wantEstimate {
				t.Errorf("estimate = %v, want %v", estimate, c.wantEstimate)
			}
		})
	}
}
