package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// renderOverlay is the key bar of the active modal. Each overlay has its own
// list and several are inline literals in here: if one is lost when touching
// the switch, the modal appears with no help at all and nothing fails.
func TestKeybindsBarRenderOverlay(t *testing.T) {
	tests := []struct {
		overlay overlayKind
		want    []string // at least one of these keys must appear
		absent  []string
	}{
		{overlayDetail, []string{"select comment", "close"}, nil},
		{overlayTag, []string{"toggle tag", "suggestion", "close"}, nil},
		{overlayNewTask, keysOf(newTaskKeybinds()), []string{"toggle tag"}},
		{overlayFilter, keysOf(filterKeybinds()), []string{"toggle tag"}},
		{overlayProject, []string{"save", "cancel"}, []string{"toggle tag"}},
		{overlayConfirm, []string{"confirm", "cancel"}, nil},
		{overlayDescEdit, []string{"save", "copy", "paste", "cancel"}, nil},
		{overlayAssignee, []string{"open", "add off-day", "close"}, nil},
		{overlayAssigneeDetail, []string{"add off-day", "delete off-day", "back"}, []string{"toggle tag"}},
		{overlayOffdayForm, []string{"save", "next field", "cancel"}, []string{"toggle tag"}},
		{overlayAskAI, []string{"move", "handoff", "close"}, []string{"toggle tag"}},
		{overlayNone, nil, []string{"confirm", "toggle tag"}},
	}
	for _, tt := range tests {
		t.Run(overlayName(tt.overlay), func(t *testing.T) {
			s := KeybindsBar{view: viewList, overlay: tt.overlay, width: 200}
			out := ansi.Strip(strings.Join(s.renderOverlay(), "\n"))

			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("%q is missing in the overlay:\n%s", want, out)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(out, absent) {
					t.Errorf("%q should not appear in this overlay:\n%s", absent, out)
				}
			}
			if len(tt.want) == 0 && out != "" {
				t.Errorf("with no overlay there should be no bar, got %q", out)
			}
		})
	}
}

// Every known overlay must return SOMETHING: an overlay with no keybinds
// leaves the user without hints and it is the typical silent failure.
func TestEveryOverlayHasKeybinds(t *testing.T) {
	all := []overlayKind{
		overlayDetail, overlayTag, overlayNewTask, overlayFilter,
		overlayProject, overlayConfirm, overlayDescEdit, overlayAssignee,
		overlayAssigneeDetail, overlayOffdayForm, overlayAskAI,
	}
	for _, o := range all {
		if o == overlayNone {
			continue
		}
		s := KeybindsBar{view: viewList, overlay: o, width: 200}
		if rows := s.renderOverlay(); len(rows) == 0 {
			t.Errorf("overlay %s without keybinds", overlayName(o))
		}
	}
}

// The split into rows groups in chunks of keybindsPerRow: a row can never
// carry more than that number of actions.
func TestKeybindsRowsRespectPerRow(t *testing.T) {
	for _, n := range []int{0, 1, keybindsPerRow - 1, keybindsPerRow, keybindsPerRow + 1, keybindsPerRow * 3} {
		kbs := make([]keybind, n)
		for i := range kbs {
			kbs[i] = keybind{"k" + strings.Repeat("x", i%5+1), "d" + strings.Repeat("y", i%3+1)}
		}
		s := KeybindsBar{view: viewList, width: 400}
		rows := s.renderRows(kbs)

		wantRows := 0
		if n > 0 {
			wantRows = (n + keybindsPerRow - 1) / keybindsPerRow
		}
		if len(rows) != wantRows {
			t.Errorf("n=%d: rows = %d, want %d", n, len(rows), wantRows)
		}
		for _, row := range rows {
			sep := styleStatusSep.Render(" · ")
			if got := strings.Count(row, sep) + 1; got > keybindsPerRow {
				t.Errorf("n=%d: row with %d actions, want <= %d: %q", n, got, keybindsPerRow, ansi.Strip(row))
			}
		}
	}
}

// keysOf extracts the keys of a keybinds list.
func keysOf(kbs []keybind) []string {
	out := make([]string, 0, len(kbs))
	for _, kb := range kbs {
		out = append(out, kb.key)
	}
	return out
}

// overlayName gives a readable name to the overlay for the subtests.
func overlayName(o overlayKind) string {
	switch o {
	case overlayNone:
		return "none"
	case overlayDetail:
		return "detail"
	case overlayTag:
		return "tag"
	case overlayNewTask:
		return "new-task"
	case overlayFilter:
		return "filter"
	case overlayProject:
		return "project"
	case overlayConfirm:
		return "confirm"
	case overlayDescEdit:
		return "desc-edit"
	case overlayAssignee:
		return "assignee"
	case overlayAssigneeDetail:
		return "assignee-detail"
	case overlayOffdayForm:
		return "offday-form"
	case overlayAskAI:
		return "ask-ai"
	}
	return "?"
}

// ---- clamps: they share the same shape (empty→0, high→n-1, negative→0)
// and they are what keeps a cursor from pointing outside the list. ----

// The clamps share the same shape: empty list -> 0, index above -> last,
// negative index -> 0. The expected value is derived from the model instead
// of being written by hand, because the roster size depends on the fixtures.
func TestClampIndexes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		clamp func(m *Model)
		get   func(m *Model) int
		want  func(m *Model) int
	}{
		{
			name:  "assignee within range",
			setup: func(m *Model) { m.assigneeIdx = 1 },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return 1 },
		},
		{
			name:  "assignee too high falls to the last",
			setup: func(m *Model) { m.assigneeIdx = 99 },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return len(m.assigneeRoster()) - 1 },
		},
		{
			name:  "negative assignee",
			setup: func(m *Model) { m.assigneeIdx = -3 },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "assignee with no roster",
			setup: func(m *Model) { m.assigneeIdx = 5; m.tasks = nil },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "offday within range",
			setup: func(m *Model) { m.assigneeIdx = 0; m.assigneeOffdayIdx = 0 },
			clamp: func(m *Model) { m.clampOffdayIdx() },
			get:   func(m *Model) int { return m.assigneeOffdayIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "offday too high",
			setup: func(m *Model) { m.assigneeIdx = 0; m.assigneeOffdayIdx = 99 },
			clamp: func(m *Model) { m.clampOffdayIdx() },
			get:   func(m *Model) int { return m.assigneeOffdayIdx },
			want:  func(m *Model) int { return max(0, len(m.assigneeOffDays(m.currentAssignee()))-1) },
		},
		{
			name:  "negative offday",
			setup: func(m *Model) { m.assigneeIdx = 0; m.assigneeOffdayIdx = -2 },
			clamp: func(m *Model) { m.clampOffdayIdx() },
			get:   func(m *Model) int { return m.assigneeOffdayIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "kanban column out of range falls to the last",
			setup: func(m *Model) { m.kanbanCol = 99; m.kanbanRow = 0 },
			clamp: func(m *Model) { m.clampKanbanCursor() },
			get:   func(m *Model) int { return m.kanbanCol },
			want:  func(m *Model) int { return len(m.kanbanColumns()) - 1 },
		},
		{
			name:  "negative kanban column",
			setup: func(m *Model) { m.kanbanCol = -1 },
			clamp: func(m *Model) { m.clampKanbanCursor() },
			get:   func(m *Model) int { return m.kanbanCol },
			want:  func(m *Model) int { return 0 },
		},
		{
			// A project with an empty workflow produces no columns: it is the only
			// path to the clamp's defensive branch.
			name: "kanban with no columns",
			setup: func(m *Model) {
				m.currentView = viewKanban
				m.projects = []model.Project{{Name: "empty", Workflow: []string{}}}
				m.filterProject = "empty"
				m.kanbanCol, m.kanbanRow = 3, 3
			},
			clamp: func(m *Model) { m.clampKanbanCursor() },
			get:   func(m *Model) int { return m.kanbanCol },
			want:  func(m *Model) int { return 0 },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			tt.setup(m)
			tt.clamp(m)
			if got, want := tt.get(m), tt.want(m); got != want {
				t.Errorf("index = %d, want %d", got, want)
			}
		})
	}
}

// The gantt does not only truncate: it also never leaves the cursor on a
// header row, it jumps to the nearest task. With a high index, the
// truncation and the jump combine.
func TestSnapGanttCursorBoundsAndHeaderSkip(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewGantt

	m.ganttCursor = 99
	m.snapGanttCursor()
	if got := m.ganttCursor; got < 0 || got >= len(m.ganttRows()) {
		t.Errorf("ganttCursor = %d, outside [0,%d)", got, len(m.ganttRows()))
	}

	m.ganttCursor = -4
	m.snapGanttCursor()
	if m.ganttCursor < 0 {
		t.Errorf("ganttCursor = %d, want >= 0", m.ganttCursor)
	}
}

// The gantt's clamp has an extra rule: it must never leave the cursor on a
// header row; it jumps to the nearest task.
func TestSnapGanttCursorSkipsHeaders(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewGantt
	rows := m.ganttRows()
	if len(rows) == 0 {
		t.Skip("no gantt rows in the fixtures")
	}

	for i := range rows {
		m.ganttCursor = i
		m.snapGanttCursor()
		if got := m.ganttRows()[m.ganttCursor]; got.kind != ganttTaskRow {
			t.Errorf("cursor at %d is still on a header (%v), want a task", i, got.kind)
		}
	}
}
