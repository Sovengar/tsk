package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// The detail modal splits its height between comments and description,
// truncates both to the box width and marks the selected comment. The tests
// that covered it looked at whether some text appeared; what has to be looked
// at is how many people there are and where.
//
// The asserts are about width and position: the text comes out whole as soon
// as it fits, and a truncated line shows in its length.

// newDetailWithTags opens the detail of a task with the given tags.
func newDetailWithTags(t *testing.T, tags ...string) *Model {
	t.Helper()
	m := newTestModel(t)
	task := m.tasks[0]
	task.Tags = tags
	m.detailOpen = true
	m.detailTask = &task
	m.detailComments = nil
	m.detailCommentSel = -1
	m.width = 100
	return m
}

// detailRender returns the detail without colors.
func detailRender(t *testing.T, m *Model, h int) string {
	t.Helper()
	return ansi.Strip(m.renderDetail(m.detailTask, h))
}

// With no tags, the metadata puts a long dash instead of an empty list, which
// would look like a hole.
func TestDetailTagsDashWhenNone(t *testing.T) {
	m := newDetailWithTags(t)
	out := detailRender(t, m, 40)
	if !strings.Contains(out, "Tags:") {
		t.Fatalf("the tags line does not show:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "Tags:") {
			continue
		}
		if !strings.Contains(line, "—") {
			t.Errorf("without tags the line does not carry the long dash: %q", line)
		}
		return
	}
}

func TestDetailTagsJoined(t *testing.T) {
	m := newDetailWithTags(t, "one", "two", "three")
	out := detailRender(t, m, 40)
	if !strings.Contains(out, "one, two, three") {
		t.Errorf("the tags do not come out comma-joined:\n%s", out)
	}
}

// A long description is truncated, and the truncated line reaches the box's
// inner width. With height to spare it comes out whole.
func TestDetailDescriptionIsClippedToBox(t *testing.T) {
	longDesc := strings.Repeat("word ", 200)
	m := newDetailWithTags(t)
	m.detailTask.Description = longDesc

	width := modalInnerWidth(m.width)

	// With height to spare the description is not truncated vertically, but
	// each line is truncated horizontally to the box width.
	out := detailRender(t, m, 200)
	widest := 0
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > widest {
			widest = w
		}
	}
	if widest > width+2 {
		t.Errorf("a line measures %d, more than the box's inner width %d", widest, width)
	}
}

// With little height, the description is truncated vertically: fewer lines fit
// than words it has.
func TestDetailDescriptionBudgetGrowsWithHeight(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailTask.Description = strings.Repeat("word\n", 100)

	few := strings.Count(detailRender(t, m, 20), "word")
	many := strings.Count(detailRender(t, m, 80), "word")
	if few >= many {
		t.Errorf("with height 20, %d description lines come out and with 80, %d: the height does not bound",
			few, many)
	}
	if few == 0 {
		t.Error("with height 20 no description line comes out")
	}
}

// The selected comment carries a "> " in front and the rest "  ". It is what
// tells the selection apart from the rest of the modal's marks.
func TestDetailCommentMarkerFollowsSelection(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{
		{ID: 1, Body: "first", CreatedAt: "2026-01-01T10:00:00Z"},
		{ID: 2, Body: "second", CreatedAt: "2026-01-02T11:00:00Z"},
		{ID: 3, Body: "third", CreatedAt: "2026-01-03T12:00:00Z"},
	}
	m.width = 120
	m.clampOffdayIdx()

	for sel := 0; sel < 3; sel++ {
		m.detailCommentSel = sel
		out := detailRender(t, m, 60)
		expected := "> " + formatCommentTime(m.detailComments[sel].CreatedAt)
		if !strings.Contains(out, expected) {
			t.Errorf("with the selection at %d %q does not come out:\n%s", sel, expected, out)
		}
		// Only one comment "> ": the box's is "│", not "> ".
		if n := strings.Count(out, "> 20"); n != 1 {
			t.Errorf("with the selection at %d there are %d markers, want 1:\n%s", sel, n, out)
		}
	}
}

// With no selection there is no "> " in the comments.
func TestDetailNoMarkerWithoutSelection(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "one", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 120

	out := detailRender(t, m, 60)
	if strings.Contains(out, "> "+formatCommentTime("2026-01-01T10:00:00Z")) {
		t.Errorf("without a selection a marker comes out:\n%s", out)
	}
}

// A multi-line comment collapses to one line: the whole body cannot take
// three rows of the box.
func TestDetailCommentBodyCollapsedToOneLine(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "one\ntwo\nthree", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 120

	out := detailRender(t, m, 60)
	if !strings.Contains(out, "one two three") {
		t.Errorf("the body did not collapse to one line:\n%s", out)
	}
}

// A very long comment is truncated to the box width: the whole modal measures
// what it measures, without overflowing because of a text body.
func TestDetailLongCommentIsClipped(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{
		ID:        1,
		Body:      strings.Repeat("x", 400),
		CreatedAt: "2026-01-01T10:00:00Z",
	}}
	m.width = 80

	out := detailRender(t, m, 60)
	if !strings.Contains(out, "xxxx") {
		t.Fatalf("the comment does not show:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "xxxx") {
			continue
		}
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("the comment's line measures %d, more than the modal (%d)", w, m.width)
		}
	}
}

// The content is off-centered upwards: the fill goes only in front, in half
// of the leftover height. Below there is nothing -- that gap is put by the
// layout that wraps the modal --, so what is measured is the number of lines
// on top, and it has to be half the remainder and not the whole remainder.
func TestDetailPaddedDownByHalfTheSlack(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "one", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 100

	const height = 60
	out := detailRender(t, m, height)
	lines := strings.Split(out, "\n")

	top := 0
	for top < len(lines) && strings.TrimSpace(lines[top]) == "" {
		top++
	}
	if top == 0 {
		t.Fatalf("there is no padding:\\n%q", out)
	}

	content := len(lines) - top
	if want := (height - content) / 2; top != want {
		t.Errorf("padding %d lines, want %d (half of the %d left over)",
			top, want, height-content)
	}
}

// With the height exactly at the content size there is not even one margin line.
func TestDetailNotCentredWhenItFills(t *testing.T) {
	m := newDetailWithTags(t)
	m.width = 100

	out := detailRender(t, m, 3)
	if strings.HasPrefix(out, "\\n") {
		t.Errorf("with height 3 there is a top margin:\\n%q", out)
	}
}

// The detail's boxes are drawn at the modal's full width, borders
// included. The inner width is two less, and that is what is used to truncate
// the content.
func TestDetailBoxesShareWidth(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "one", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 100

	out := detailRender(t, m, 60)
	width := 0
	for _, line := range strings.Split(out, "\n") {
		i := strings.Index(line, "╭")
		if i < 0 {
			continue
		}
		runes := []rune(line[i:])
		for j, r := range runes {
			if r == '╮' && j+1 > width {
				width = j + 1
			}
		}
	}
	if want := m.width; width != want {
		t.Errorf("the widest box measures %d, want %d (the modal's width with its borders)", width, want)
	}
}

// The content truncation limit is the box's inner width, two less than the
// modal's width.
//
// The row count is what tells it apart, not the line width: the box pads with
// spaces up to its border, so all lines measure the same wherever they are
// truncated. With two extra columns -- a limit above the inner width --
// lipgloss wraps the line and the modal grows one row.
//
// And it is worth saying what this test does NOT tell apart: a narrower limit
// is not visible either, because the content fits with room to spare in the
// box and leftover space does not matter. Only the upper side of the inner
// width is observable, and it is the side that matters -- the one that makes the modal grow.
func TestDetailContentWrapRowCount(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
	}{
		{"long description", func(m *Model) {
			m.detailTask.Description = strings.Repeat("word ", 200)
		}},
		{"long comment", func(m *Model) {
			m.detailComments = []model.Comment{{
				ID:        1,
				Body:      strings.Repeat("y", 400),
				CreatedAt: "2026-01-01T10:00:00Z",
			}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newDetailWithTags(t)
			tt.setup(m)

			out := detailRender(t, m, 200)
			if got := len(strings.Split(out, "\n")); got != 107 {
				t.Errorf("the modal has %d rows, want 107", got)
			}
		})
	}
}
