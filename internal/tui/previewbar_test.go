package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

func TestPreviewBarRendersDescription(t *testing.T) {
	p := NewPreviewBar(60)
	p.SetTask(&model.Task{ID: 1, Title: "t", Description: "hello there"})

	out := ansi.Strip(p.View())
	if !strings.Contains(out, "hello there") {
		t.Errorf("the preview should show the description:\n%s", out)
	}
	if !strings.Contains(out, "Description") {
		t.Errorf("the preview should carry the title Description:\n%s", out)
	}
}

func TestPreviewBarEmptyDescriptionShowsPlaceholder(t *testing.T) {
	p := NewPreviewBar(60)
	p.SetTask(&model.Task{ID: 1, Title: "t"})

	if out := ansi.Strip(p.View()); !strings.Contains(out, "(no description)") {
		t.Errorf("without a description it should show the placeholder:\n%s", out)
	}
}

func TestPreviewBarWithoutTaskIsEmpty(t *testing.T) {
	p := NewPreviewBar(60)
	if out := p.View(); out != "" {
		t.Errorf("with no task selected the preview must be empty, got %q", out)
	}
}

// TestPreviewBarTruncatesToMaxLines verifies the height cap and that the
// truncated line never exceeds the width (if it did, bordered would re-wrap it and
// add an extra row).
func TestPreviewBarTruncatesToMaxLines(t *testing.T) {
	const width = 40
	p := NewPreviewBar(width)
	p.SetMaxLines(3)
	p.SetTask(&model.Task{Description: strings.Repeat("word ", 60)})

	out := p.View()
	lines := strings.Split(out, "\n")
	if len(lines) != 3+2 { // maxLines + top and bottom borders
		t.Fatalf("height = %d rows, want %d:\n%s", len(lines), 3+2, out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("a long description should be truncated with …:\n%s", out)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d measures %d, exceeds the width %d", i, w, width)
		}
	}
}

func TestPreviewBarRespectsWidth(t *testing.T) {
	const width = 30
	p := NewPreviewBar(width)
	p.SetTask(&model.Task{Description: strings.Repeat("a fairly long word ", 20)})

	for i, line := range strings.Split(p.View(), "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d measures %d, exceeds the width %d", i, w, width)
		}
	}
}

func TestSelectedTaskNilOnDashboard(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	if got := m.selectedTask(); got != nil {
		t.Errorf("on the dashboard there is no selected task, got %+v", got)
	}
}

func TestSelectedTaskFollowsListCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m.cursor = 1

	got := m.selectedTask()
	if got == nil {
		t.Fatal("there should be a selected task in the list")
	}
	if want := m.filteredTasks()[1].ID; got.ID != want {
		t.Errorf("selectedTask = %d, want %d", got.ID, want)
	}
}

func TestSelectedTaskFollowsKanbanCursor(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "2")

	cols := m.kanbanColumns()
	idx := -1
	for i, c := range cols {
		if len(c.tasks) > 0 {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("the fixture should have some column with tasks")
	}
	m.kanbanCol = idx
	m.kanbanRow = 0

	got := m.selectedTask()
	if got == nil {
		t.Fatal("there should be a selected task in kanban")
	}
	if want := cols[idx].tasks[0].ID; got.ID != want {
		t.Errorf("selectedTask = %d, want %d", got.ID, want)
	}
}

// The preview wraps the description to the width and, if it does not fit in the
// given height, truncates the leftover lines. The last visible one carries an
// ellipsis and still measures the full width: that is why the cut is limit-1
// and not limit, and that is why the ellipsis fits exactly.
func TestPreviewTruncatedLastLineKeepsWidth(t *testing.T) {
	p := NewPreviewBar(40)
	p.SetTask(&model.Task{Description: strings.Repeat("word ", 40)})
	p.SetMaxLines(2)

	lines := strings.Split(ansi.Strip(p.View()), "\n")
	if len(lines) < 2 {
		t.Fatalf("the preview has %d lines:\n%q", len(lines), p.View())
	}
	// What is checked is that the last line carries an ellipsis: it is what says
	// that the description goes on beyond the lines that fit. The width of the
	// line is useless here -- the box pads up to its border and they all measure
	// the same.
	if !strings.Contains(strings.Join(lines, "\n"), "…") {
		t.Errorf("a truncated description does not carry an ellipsis:\n%q", p.View())
	}
}

// If the description fits whole, there is neither truncation nor ellipsis.
func TestPreviewNoEllipsisWhenItFits(t *testing.T) {
	p := NewPreviewBar(80)
	p.SetTask(&model.Task{Description: "short"})
	p.SetMaxLines(20)

	if out := ansi.Strip(p.View()); strings.Contains(out, "…") {
		t.Errorf("a short description carries an ellipsis:\n%q", out)
	}
}

// maxLines never drops below one: a zero-height preview cannot show nothing,
// because then the task row would disappear.
func TestPreviewMaxLinesFloor(t *testing.T) {
	p := NewPreviewBar(40)
	p.SetTask(&model.Task{Description: strings.Repeat("word ", 50)})
	p.SetMaxLines(0)

	if lineCount(ansi.Strip(p.View())) < 1 {
		t.Errorf("with maxLines 0 the preview shows nothing:\n%q", p.View())
	}
}

// With the description exactly at the line limit there is no ellipsis: it fits
// whole. The edge is where `len(wrapped) > maxLines` stops truncating.
func TestPreviewEllipsisOnlyWhenClipped(t *testing.T) {
	desc := strings.Repeat("word ", 40)
	for _, maxLines := range []int{1, 2, 5, 20} {
		p := NewPreviewBar(40)
		p.SetTask(&model.Task{Description: desc})
		p.SetMaxLines(maxLines)

		// The box adds borders and a header, so only the lines of the
		// description are counted.
		desc := 0
		for _, line := range strings.Split(ansi.Strip(p.View()), "\n") {
			if strings.Contains(line, "word") {
				desc++
			}
		}
		if desc > maxLines {
			t.Errorf("with maxLines %d, %d description lines come out", maxLines, desc)
		}
		if desc == 0 {
			t.Errorf("with maxLines %d no description line comes out", maxLines)
		}
	}
}
