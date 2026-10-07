package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/model"
)

// modelWithTasks builds a model with n tasks in the "api" project and an
// explicit page size, so the cursor can be placed at will.
func modelWithTasks(t *testing.T, n, pageSize int) *Model {
	t.Helper()
	m := newTestModel(t)
	m.pageSize = pageSize

	tasks := make([]model.Task, 0, n)
	for i := range n {
		tasks = append(tasks, model.Task{
			ID:     int64(i + 1),
			Title:  fmt.Sprintf("task %02d", i),
			Status: "todo",
		})
	}
	m.tasks = tasks
	m.invalidateFilterCache()
	return m
}

// listWindow is the single calculation of pagination. These cases pin its
// exact invariants, including the edges that used to live spread across five
// different call sites (and that is why each copy was a test hole).
func TestListWindow(t *testing.T) {
	tests := []struct {
		name                string
		total, size, cursor int
		wantPage, wantPages int
		wantStart, wantEnd  int
	}{
		{"empty list", 0, 5, 0, 0, 1, 0, 0},
		{"an exact page", 10, 5, 0, 0, 2, 0, 5},
		{"cursor on the first", 12, 5, 0, 0, 3, 0, 5},
		{"cursor mid-page", 12, 5, 3, 0, 3, 0, 5},
		{"last of the first page", 12, 5, 4, 0, 3, 0, 5},
		{"first of the second", 12, 5, 5, 1, 3, 5, 10},
		{"last of the second", 12, 5, 11, 2, 3, 10, 12},
		{"cursor beyond the end", 12, 5, 99, 2, 3, 10, 12},
		{"negative cursor", 12, 5, -3, 0, 3, 0, 5},
		{"page of size 1", 3, 1, 2, 2, 3, 2, 3},
		{"more tasks than exact pages", 20, 5, 19, 3, 4, 15, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithTasks(t, tt.total, tt.size)
			m.cursor = tt.cursor

			w := m.listWindow()
			if w.page != tt.wantPage || w.pages != tt.wantPages ||
				w.start != tt.wantStart || w.end != tt.wantEnd {
				t.Errorf("listWindow = {page:%d pages:%d start:%d end:%d}, want {page:%d pages:%d start:%d end:%d}",
					w.page, w.pages, w.start, w.end, tt.wantPage, tt.wantPages, tt.wantStart, tt.wantEnd)
			}
			if w.total != tt.total || w.size != tt.size {
				t.Errorf("total/size = %d/%d, want %d/%d", w.total, w.size, tt.total, tt.size)
			}
			// Invariant the view depends on: the window never exceeds the total
			// and never inverts.
			if w.start > w.end || w.end > w.total {
				t.Errorf("invalid window: start=%d end=%d total=%d", w.start, w.end, w.total)
			}
		})
	}
}

func TestListWindowClampCursor(t *testing.T) {
	// `cursor` sets the page (and with it the window); `probe` is the value put
	// in clampCursor, which is what j does going down and p going up.
	tests := []struct {
		name                string
		total, size, cursor int
		probe               int
		want                int
	}{
		{"empty list always at 0", 0, 5, 0, 7, 0},
		{"empty list with negative probe", 0, 5, 0, -2, 0},
		{"inside the window", 12, 5, 0, 3, 3},
		{"going down does not pass the end of the page", 12, 5, 0, 5, 4},
		{"going up does not pass the start of the page", 12, 5, 5, 4, 5},
		{"last of the last page", 12, 5, 11, 11, 11},
		{"going down on the last page stays", 12, 5, 11, 12, 11},
		{"far above gets clamped", 12, 5, 99, 99, 11},
		{"negative gets clamped to the start", 12, 5, 0, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithTasks(t, tt.total, tt.size)
			m.cursor = tt.cursor

			w := m.listWindow()
			if got := w.clampCursor(tt.probe); got != tt.want {
				t.Errorf("clampCursor(%d) = %d, want %d", tt.probe, got, tt.want)
			}
			// Invariant: never outside the visible window.
			if tt.total > 0 {
				if got := w.clampCursor(tt.probe); got < w.start || got > w.end-1 {
					t.Errorf("clampCursor returned %d, outside [%d,%d]", got, w.start, w.end)
				}
			}
		})
	}
}

func TestListWindowPageNavigation(t *testing.T) {
	tests := []struct {
		name                string
		total, size, cursor int
		wantNext, wantPrev  int
		nextOK, prevOK      bool
	}{
		{"first page has no previous", 12, 5, 0, 5, 0, true, false},
		{"middle page has both", 12, 5, 6, 10, 0, true, true},
		{"last page has no next", 12, 5, 11, 0, 5, false, true},
		{"empty list does not navigate", 0, 5, 0, 0, 0, false, false},
		{"a single page does not navigate", 3, 5, 1, 0, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := modelWithTasks(t, tt.total, tt.size)
			m.cursor = tt.cursor
			w := m.listWindow()

			if got, ok := w.nextPageStart(); got != tt.wantNext || ok != tt.nextOK {
				t.Errorf("nextPageStart() = %d, %v; want %d, %v", got, ok, tt.wantNext, tt.nextOK)
			}
			if got, ok := w.prevPageStart(); got != tt.wantPrev || ok != tt.prevOK {
				t.Errorf("prevPageStart() = %d, %v; want %d, %v", got, ok, tt.wantPrev, tt.prevOK)
			}
		})
	}
}

// pageLegend cases the existing test does not cover: cursor beyond the end
// (the page is truncated) and negative cursor (the window does not invert).
func TestPageLegendOutOfRangeCursor(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		pageSize int
		cursor   int
		want     string
	}{
		{"cursor beyond the end clamps to the last page", 4, 3, 99, "4-4 of 4 · Page 2/2"},
		{"negative cursor clamps to the first", 4, 3, -2, "1-3 of 4 · Page 1/2"},
		{"exactly one page", 3, 3, 2, "1-3 of 3 · Page 1/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.tasks = make([]model.Task, tt.total)
			m.invalidateFilterCache()
			m.pageSize = tt.pageSize
			m.cursor = tt.cursor

			if got := m.pageLegend(); got != tt.want {
				t.Errorf("pageLegend() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Real keyboard navigation. The behavior is the usual one (j/k stop at the
// page's edge, n/p jump pages, N/P go to the ends); what changes is that
// there is no longer any increment inside an if, so the mutant of that
// increment cannot hang.
func TestListNavigationByKey(t *testing.T) {
	t.Run("j stops at the end of the page", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 0

		for range 10 {
			m, _ = press(m, "j")
		}
		if m.cursor != 4 {
			t.Errorf("cursor = %d after going down too much, want 4 (end of the first page)", m.cursor)
		}
	})

	t.Run("k stops at the start of the page", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 7

		for range 10 {
			m, _ = press(m, "k")
		}
		if m.cursor != 5 {
			t.Errorf("cursor = %d after going up too much, want 5 (start of the second page)", m.cursor)
		}
	})

	t.Run("n jumps page and p comes back", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 0

		m, _ = press(m, "n")
		if m.cursor != 5 {
			t.Errorf("after n cursor = %d, want 5", m.cursor)
		}
		m, _ = press(m, "p")
		if m.cursor != 0 {
			t.Errorf("after p cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("n on the last page does nothing", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 11

		m, _ = press(m, "n")
		if m.cursor != 11 {
			t.Errorf("cursor = %d, want 11 (n on the last page does not move)", m.cursor)
		}
	})

	t.Run("p on the first page does nothing", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList
		m.cursor = 3

		m, _ = press(m, "p")
		if m.cursor != 3 {
			t.Errorf("cursor = %d, want 3", m.cursor)
		}
	})

	t.Run("N goes to the last task and P to the first", func(t *testing.T) {
		m := modelWithTasks(t, 12, 5)
		m.currentView = viewList

		m, _ = press(m, "N")
		if m.cursor != 11 {
			t.Errorf("after N cursor = %d, want 11", m.cursor)
		}
		m, _ = press(m, "P")
		if m.cursor != 0 {
			t.Errorf("after P cursor = %d, want 0", m.cursor)
		}
	})

	t.Run("navigating with an empty list does not move the cursor", func(t *testing.T) {
		m := modelWithTasks(t, 0, 5)
		m.currentView = viewList

		for _, k := range []string{"j", "k", "n", "p", "N", "P"} {
			m, _ = press(m, k)
			if m.cursor != 0 {
				t.Errorf("after %q with an empty list cursor = %d, want 0", k, m.cursor)
			}
		}
	})

	t.Run("j with a single task does not go out", func(t *testing.T) {
		m := modelWithTasks(t, 1, 5)
		m.currentView = viewList
		m.cursor = 0

		m, _ = press(m, "j")
		if m.cursor != 0 {
			t.Errorf("cursor = %d, want 0", m.cursor)
		}
	})
}

// The effective page size: 0 or negative falls back to the config default.
func TestListPageSizeFallsBackToDefault(t *testing.T) {
	m := newTestModel(t)
	for _, size := range []int{0, -1, -100} {
		m.pageSize = size
		if got, want := m.listPageSize(), config.DefaultPageSize; got != want {
			t.Errorf("listPageSize() with pageSize=%d = %d, want %d", size, got, want)
		}
	}
	m.pageSize = 7
	if got := m.listPageSize(); got != 7 {
		t.Errorf("listPageSize() = %d, want 7", got)
	}
}

// The filter bar's separator leaves two columns for the box, and the floor
// at zero keeps strings.Repeat from blowing up with a negative.
func TestSeparatorWidth(t *testing.T) {
	tests := []struct {
		name   string
		innerW int
		want   int
	}{
		{"roomy", 118, 116},
		{"normal", 78, 76},
		{"exact", 2, 0},
		{"one short", 1, 0},
		{"zero", 0, 0},
		{"negative", -10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := separatorWidth(tt.innerW); got != tt.want {
				t.Errorf("separatorWidth(%d) = %d, want %d", tt.innerW, got, tt.want)
			}
		})
	}
}

// Never negative: that is what prevents the strings.Repeat panic.
func TestSeparatorWidthNeverNegative(t *testing.T) {
	for innerW := -100; innerW <= 300; innerW++ {
		if got := separatorWidth(innerW); got < 0 {
			t.Fatalf("separatorWidth(%d) = %d, want >= 0", innerW, got)
		}
	}
}

// The rendered separator measures the inner width minus two, and that is
// checked on the plain text, which is where a moved "- 2" shows.
func TestRenderFilterHeaderSeparatorWidth(t *testing.T) {
	for _, width := range []int{120, 80, 40} {
		m := newTestModel(t)
		m.width = width

		header := ansi.Strip(m.renderFilterHeader(width - 2))
		lines := strings.Split(header, "\n")
		if len(lines) < 2 {
			t.Fatalf("the header has no separator:\n%s", header)
		}
		// The dashes are counted, not the line's width: the filter bar has a
		// natural width and JoinVertical pads all lines up to the widest, so
		// the last one always measures the same.
		got := strings.Count(lines[len(lines)-1], "─")
		if want := width - 4; got != want {
			t.Errorf("with %d of window the separator has %d dashes, want %d", width, got, want)
		}
	}
}

// The page is truncated to the available height with the window following
// the cursor. Without enough height the whole page is painted: it is better
// for the box to overflow than to come out empty.
func TestListWindowForHeight(t *testing.T) {
	tests := []struct {
		name                                  string
		pageStart, pageEnd, cursor, maxHeight int
		wantStart, wantEnd                    int
	}{
		{"fits whole", 0, 10, 5, 40, 0, 10},
		{"barely fits", 0, 10, 5, 10 + listFixedRows, 0, 10},
		// With five rows for tasks and ten in the page, the five-row window
		// scrolls following the cursor.
		{"does not fit, cursor at the start", 0, 10, 0, 5 + listFixedRows, 0, 5},
		{"does not fit, cursor at the end", 0, 10, 9, 5 + listFixedRows, 5, 10},
		{"does not fit, cursor in the middle", 0, 10, 5, 5 + listFixedRows, 3, 8},
		{"separate page", 20, 30, 22, 5 + listFixedRows, 20, 25},
		{"minimum height", 0, 10, 3, listFixedRows, 0, 10},
		{"zero height", 0, 10, 3, 0, 0, 10},
		{"negative height", 0, 10, 3, -50, 0, 10},
		{"empty page", 0, 0, 0, 40, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := listWindowForHeight(tt.pageStart, tt.pageEnd, tt.cursor, tt.maxHeight)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("listWindowForHeight(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.pageStart, tt.pageEnd, tt.cursor, tt.maxHeight, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// What it guarantees: the window never leaves the page, and when it has to
// truncate, the cursor stays inside.
func TestListWindowForHeightProperties(t *testing.T) {
	for pageStart := 0; pageStart <= 20; pageStart += 10 {
		for pageEnd := pageStart; pageEnd <= pageStart+12; pageEnd++ {
			for cursor := pageStart - 3; cursor <= pageEnd+3; cursor++ {
				for _, maxHeight := range []int{-10, 0, 1, listFixedRows, listFixedRows + 3, listFixedRows + 7, 40} {
					start, end := listWindowForHeight(pageStart, pageEnd, cursor, maxHeight)

					if start < pageStart || end > pageEnd || end < start {
						t.Fatalf("page=[%d,%d) cursor=%d height=%d -> [%d,%d): outside the page",
							pageStart, pageEnd, cursor, maxHeight, start, end)
					}
					visible := maxHeight - listFixedRows
					if visible <= 0 || pageEnd-pageStart <= visible {
						if start != pageStart || end != pageEnd {
							t.Fatalf("page=[%d,%d) cursor=%d height=%d -> [%d,%d), want the whole page",
								pageStart, pageEnd, cursor, maxHeight, start, end)
						}
						continue
					}
					if end-start != visible {
						t.Fatalf("page=[%d,%d) cursor=%d height=%d -> [%d,%d): window of %d, want %d",
							pageStart, pageEnd, cursor, maxHeight, start, end, end-start, visible)
					}
					// The cursor is bounded to the page before looking at it. Outside
					// the page it is not a position -- the list's general clamp
					// already left it inside --, and what has to stay inside the
					// window is its equivalent on this page.
					acotado := pageStart + clampTo(cursor-pageStart, pageEnd-pageStart)
					if acotado < start || acotado >= end {
						t.Fatalf("page=[%d,%d) cursor=%d height=%d -> [%d,%d): the cursor ended up outside",
							pageStart, pageEnd, cursor, maxHeight, start, end)
					}
				}
			}
		}
	}
}

// The filter bar shows "Priority: all" when there is no filter, and the
// number when there is one. -1 is not a filter: it is the absence of a
// filter, and 0 is a real filter (no priority). That is why zero comes out as "0" and not as "all".
func TestFilterBarPriorityValue(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		want     string
	}{
		{"no filter", -1, "Priority: all"},
		{"filter of zero", 0, "Priority: 0"},
		{"filter of one", 1, "Priority: 1"},
		{"filter of three", 3, "Priority: 3"},
		{"below the range", -7, "Priority: all"},
		{"above the range", 9, "Priority: 9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.filterPriority = tt.priority
			m.width = 120

			out := ansi.Strip(m.renderFilterBar())
			if !strings.Contains(out, tt.want) {
				t.Errorf("with priority %d the bar does not say %q:\n%s", tt.priority, tt.want, out)
			}
		})
	}
}

// The status filter does not show in the Gantt, because there each column is
// a status and the bar would repeat what is already seen.
func TestFilterBarHidesStatusOnGantt(t *testing.T) {
	for _, view := range []viewKind{viewList, viewKanban, viewDashboard} {
		m := newTestModel(t)
		m.currentView = view
		m.width = 120
		if out := ansi.Strip(m.renderFilterBar()); !strings.Contains(out, "Status:") {
			t.Errorf("in view %v the status filter does not show:\n%s", view, out)
		}
	}

	m := newTestModel(t)
	m.currentView = viewGantt
	m.width = 120
	if out := ansi.Strip(m.renderFilterBar()); strings.Contains(out, "Status:") {
		t.Errorf("in the Gantt the status filter shows:\n%s", out)
	}
}
