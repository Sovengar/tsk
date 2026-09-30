package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// renderOverlay es la barra de teclas del modal activo. Cada overlay tiene su
// propia lista y varias son literales en línea aquí dentro: si una se pierde al
// tocar el switch, el modal aparece sin ninguna ayuda sin que nada falle.
func TestKeybindsBarRenderOverlay(t *testing.T) {
	tests := []struct {
		overlay overlayKind
		want    []string // al menos una de estas keys debe aparecer
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
		{overlayNone, nil, []string{"confirm", "toggle tag"}},
	}
	for _, tt := range tests {
		t.Run(overlayName(tt.overlay), func(t *testing.T) {
			s := KeybindsBar{view: viewList, overlay: tt.overlay, width: 200}
			out := ansi.Strip(strings.Join(s.renderOverlay(), "\n"))

			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("falta %q en el overlay:\n%s", want, out)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(out, absent) {
					t.Errorf("%q no debería aparecer en este overlay:\n%s", absent, out)
				}
			}
			if len(tt.want) == 0 && out != "" {
				t.Errorf("sin overlay no debería haber barra, got %q", out)
			}
		})
	}
}

// Toda overlay conocida debe devolver ALGO: un overlay sin keybinds deja al
// usuario sin pistas y es el fallo silencioso típico.
func TestEveryOverlayHasKeybinds(t *testing.T) {
	all := []overlayKind{
		overlayDetail, overlayTag, overlayNewTask, overlayFilter,
		overlayProject, overlayConfirm, overlayDescEdit, overlayAssignee,
		overlayAssigneeDetail, overlayOffdayForm,
	}
	for _, o := range all {
		if o == overlayNone {
			continue
		}
		s := KeybindsBar{view: viewList, overlay: o, width: 200}
		if rows := s.renderOverlay(); len(rows) == 0 {
			t.Errorf("overlay %s sin keybinds", overlayName(o))
		}
	}
}

// El reparto en filas agrupa de keybindsPerRow en keybindsPerRow: una fila
// nunca puede llevar más de ese número de acciones.
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
			t.Errorf("n=%d: filas = %d, want %d", n, len(rows), wantRows)
		}
		for _, row := range rows {
			sep := styleStatusSep.Render(" · ")
			if got := strings.Count(row, sep) + 1; got > keybindsPerRow {
				t.Errorf("n=%d: fila con %d acciones, want <= %d: %q", n, got, keybindsPerRow, ansi.Strip(row))
			}
		}
	}
}

// keysOf extrae las keys de una lista de keybinds.
func keysOf(kbs []keybind) []string {
	out := make([]string, 0, len(kbs))
	for _, kb := range kbs {
		out = append(out, kb.key)
	}
	return out
}

// overlayName da un nombre legible al overlay para los subtests.
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
	}
	return "?"
}

// ---- clamps: comparten la misma forma (vacío→0, alto→n-1, negativo→0) y son
// lo que evita que un cursor quede apuntando fuera de la lista. ----

// Los clamps comparten la misma forma: lista vacía -> 0, índice por encima ->
// último, índice negativo -> 0. El valor esperado se deriva del modelo en vez de
// estar escrito a mano, porque el tamaño del roster depende de las fixtures.
func TestClampIndexes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		clamp func(m *Model)
		get   func(m *Model) int
		want  func(m *Model) int
	}{
		{
			name:  "assignee dentro de rango",
			setup: func(m *Model) { m.assigneeIdx = 1 },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return 1 },
		},
		{
			name:  "assignee demasiado alto cae al último",
			setup: func(m *Model) { m.assigneeIdx = 99 },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return len(m.assigneeRoster()) - 1 },
		},
		{
			name:  "assignee negativo",
			setup: func(m *Model) { m.assigneeIdx = -3 },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "assignee sin roster",
			setup: func(m *Model) { m.assigneeIdx = 5; m.tasks = nil },
			clamp: func(m *Model) { m.clampAssigneeIdx() },
			get:   func(m *Model) int { return m.assigneeIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "offday dentro de rango",
			setup: func(m *Model) { m.assigneeIdx = 0; m.assigneeOffdayIdx = 0 },
			clamp: func(m *Model) { m.clampOffdayIdx() },
			get:   func(m *Model) int { return m.assigneeOffdayIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "offday demasiado alto",
			setup: func(m *Model) { m.assigneeIdx = 0; m.assigneeOffdayIdx = 99 },
			clamp: func(m *Model) { m.clampOffdayIdx() },
			get:   func(m *Model) int { return m.assigneeOffdayIdx },
			want:  func(m *Model) int { return max(0, len(m.assigneeOffDays(m.currentAssignee()))-1) },
		},
		{
			name:  "offday negativo",
			setup: func(m *Model) { m.assigneeIdx = 0; m.assigneeOffdayIdx = -2 },
			clamp: func(m *Model) { m.clampOffdayIdx() },
			get:   func(m *Model) int { return m.assigneeOffdayIdx },
			want:  func(m *Model) int { return 0 },
		},
		{
			name:  "kanban columna fuera de rango cae a la última",
			setup: func(m *Model) { m.kanbanCol = 99; m.kanbanRow = 0 },
			clamp: func(m *Model) { m.clampKanbanCursor() },
			get:   func(m *Model) int { return m.kanbanCol },
			want:  func(m *Model) int { return len(m.kanbanColumns()) - 1 },
		},
		{
			name:  "kanban columna negativa",
			setup: func(m *Model) { m.kanbanCol = -1 },
			clamp: func(m *Model) { m.clampKanbanCursor() },
			get:   func(m *Model) int { return m.kanbanCol },
			want:  func(m *Model) int { return 0 },
		},
		{
			// Un proyecto con workflow vacío no produce columnas: es el único
			// camino a la rama defensiva del clamp.
			name: "kanban sin columnas",
			setup: func(m *Model) {
				m.currentView = viewKanban
				m.projects = []model.Project{{Name: "vacio", Workflow: []string{}}}
				m.filterProject = "vacio"
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
				t.Errorf("índice = %d, want %d", got, want)
			}
		})
	}
}

// El gantt no sólo recorta: además nunca deja el cursor en una fila de cabecera,
// salta a la tarea más cercana. Con un índice alto, el recorte y el salto se
// combinan.
func TestSnapGanttCursorBoundsAndHeaderSkip(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewGantt

	m.ganttCursor = 99
	m.snapGanttCursor()
	if got := m.ganttCursor; got < 0 || got >= len(m.ganttRows()) {
		t.Errorf("ganttCursor = %d, fuera de [0,%d)", got, len(m.ganttRows()))
	}

	m.ganttCursor = -4
	m.snapGanttCursor()
	if m.ganttCursor < 0 {
		t.Errorf("ganttCursor = %d, want >= 0", m.ganttCursor)
	}
}

// El clamp del gantt tiene una regla extra: nunca debe dejar el cursor en una
// fila de cabecera, salta a la tarea más cercana.
func TestSnapGanttCursorSkipsHeaders(t *testing.T) {
	m := newTestModel(t)
	m.currentView = viewGantt
	rows := m.ganttRows()
	if len(rows) == 0 {
		t.Skip("sin filas de gantt en las fixtures")
	}

	for i := range rows {
		m.ganttCursor = i
		m.snapGanttCursor()
		if got := m.ganttRows()[m.ganttCursor]; got.kind != ganttTaskRow {
			t.Errorf("cursor en %d sigue en una cabecera (%v), want tarea", i, got.kind)
		}
	}
}
