package tui

import (
	"strings"
	"testing"
)

func TestViewSwitching4(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	if m.currentView != viewGantt {
		t.Errorf("after 4: view = %v, want gantt", m.currentView)
	}
}

func TestRenderGantt(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	out := m.renderGantt(m.height)
	if out == "" {
		t.Fatal("gantt should not be empty")
	}
	if !strings.Contains(out, "Gantt") {
		t.Errorf("gantt output missing title:\n%s", out)
	}
}

func TestGanttNavigation(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	rows := m.ganttRows()
	if len(rows) == 0 {
		t.Fatal("expected gantt rows")
	}

	// j baja, k sube (clamp en los extremos).
	m, _ = press(m, "j")
	if m.ganttCursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", m.ganttCursor)
	}
	m, _ = press(m, "k")
	if m.ganttCursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", m.ganttCursor)
	}
	m, _ = press(m, "k")
	if m.ganttCursor != 0 {
		t.Errorf("k at top: cursor = %d, want 0", m.ganttCursor)
	}

	// G va al final.
	m, _ = press(m, "G")
	if m.ganttCursor != len(rows)-1 {
		t.Errorf("after G: cursor = %d, want %d", m.ganttCursor, len(rows)-1)
	}

	// h/l desplazan la ventana de días.
	m, _ = press(m, "l")
	if m.ganttOffsetDays != 1 {
		t.Errorf("after l: offset = %d, want 1", m.ganttOffsetDays)
	}
	m, _ = press(m, "h")
	if m.ganttOffsetDays != 0 {
		t.Errorf("after h: offset = %d, want 0", m.ganttOffsetDays)
	}
	m, _ = press(m, "h")
	if m.ganttOffsetDays != 0 {
		t.Errorf("h at 0: offset = %d, want 0", m.ganttOffsetDays)
	}
}

func TestGanttEnterOpensDetailOnTaskRow(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")

	// Posicionar el cursor en la primera fila de tarea.
	rows := m.ganttRows()
	for i, r := range rows {
		if r.kind == ganttTaskRow {
			m.ganttCursor = i
			break
		}
	}

	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Error("enter on a task row should open the detail")
	}
	if m.detailTask == nil {
		t.Error("detail task should be set")
	}
}
