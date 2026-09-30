package cli

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

// Estas aserciones son exactas a propósito, no "contains". El render del Gantt
// es una rejilla: si un offset se mueve una columna, una etiqueta se desplaza o
// el eje pierde una celda, el resultado sigue "pareciendo" un Gantt y por eso
// sólo un assert literal lo detecta. Cubre los mutants de los offsets
// (labelW+1, +d), del ancho de la rejilla y del recorte de la etiqueta.

const (
	ganttLabelW  = 30
	ganttHeadOff = 2 // cabecera y línea en blanco antes de la regla
)

func ganttLines(t *testing.T, s *model.Schedule, weeks int) []string {
	t.Helper()
	return strings.Split(renderGanttText(s, weeks), "\n")
}

func TestRenderGanttTextRejilla(t *testing.T) {
	// Arranca en lunes: los lunes caen en d=0 y d=7, así que la regla queda
	// exactamente en 31+4+3+4 = 42 columnas (recortada por TrimRight) y el eje
	// en 31+14 = 45.
	lines := ganttLines(t, &model.Schedule{Start: "2026-09-14"}, 2)

	wantRuler := strings.Repeat(" ", ganttLabelW+1) + "3SEP" + "   " + "4SEP"
	if lines[ganttHeadOff] != wantRuler {
		t.Errorf("regla = %q\nwant   %q", lines[ganttHeadOff], wantRuler)
	}

	wantAxis := strings.Repeat(" ", ganttLabelW+1) + "|------|------"
	if lines[ganttHeadOff+1] != wantAxis {
		t.Errorf("eje = %q\nwant  %q", lines[ganttHeadOff+1], wantAxis)
	}
	if got := len([]rune(lines[ganttHeadOff+1])); got != ganttLabelW+1+14 {
		t.Errorf("eje = %d celdas, want %d", got, ganttLabelW+1+14)
	}
}

func TestRenderGanttTextEjeSigueElLunes(t *testing.T) {
	tests := []struct {
		start  string
		weeks  int
		pipes  int
		weekNb int
	}{
		{"2026-09-14", 2, 2, 14}, // lunes
		{"2026-09-15", 2, 2, 14}, // martes: los lunes caen en d=5 y d=12
		{"2026-09-16", 1, 1, 7},  // miércoles con una sola semana: lunes en d=4
		{"2026-09-14", 4, 4, 28},
	}
	for _, tt := range tests {
		t.Run(tt.start, func(t *testing.T) {
			axis := ganttLines(t, &model.Schedule{Start: tt.start}, tt.weeks)[ganttHeadOff+1]
			if got := strings.Count(axis, "|"); got != tt.pipes {
				t.Errorf("eje con %d lunes, want %d (%q)", got, tt.pipes, axis)
			}
			if got := len([]rune(axis)) - ganttLabelW - 1; got != tt.weekNb {
				t.Errorf("eje = %d celdas, want %d", got, tt.weekNb)
			}
		})
	}
}

// TestRenderGanttTextRecortaLaUltimaEtiqueta: sólo cuando el calendario no
// arranca en lunes la última etiqueta de semana cae tan a la derecha que no
// cabe en la regla y se trunca. Es el único caso donde el `col+i < len(ruler)`
// decide algo; sin esta aserción el guard es código muerto a ojos del test.
func TestRenderGanttTextRecortaLaUltimaEtiqueta(t *testing.T) {
	ruler := ganttLines(t, &model.Schedule{Start: "2026-09-15"}, 2)[ganttHeadOff]
	if !strings.HasSuffix(ruler, "1") {
		t.Errorf("esperaba la etiqueta recortada a 1OCT→1: %q", ruler)
	}
	if strings.Contains(ruler, "1OCT") {
		t.Errorf("la etiqueta no debería caber entera: %q", ruler)
	}

	// Con arranque en lunes la última sí cabe: nada se recorta.
	full := ganttLines(t, &model.Schedule{Start: "2026-09-14"}, 2)[ganttHeadOff]
	if !strings.HasSuffix(full, "4SEP") {
		t.Errorf("con arranque en lunes la etiqueta va entera: %q", full)
	}
}

// TestRenderGanttTextPosicionDeLasBarras fija en qué celda cae cada barra: la
// primera empieza en la columna 31 (etiqueta de 30 + espacio) y una tarea que
// empieza el día 5 lleva 5 celdas en blanco delante. Un redondeo de días
// distinto (Hours()/24) mueve la barra sin cambiar nada visible a simple vista.
func TestRenderGanttTextPosicionDeLasBarras(t *testing.T) {
	s := &model.Schedule{
		Start: "2026-09-14",
		Assignees: []model.AssigneeSchedule{{
			Assignee: "@a",
			Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 1, Title: "dos dias"}, Start: "2026-09-14", End: "2026-09-15", Estimate: 2},
				{Task: model.Task{ID: 2, Title: "dia cinco"}, Start: "2026-09-19", End: "2026-09-19", Estimate: 1},
			},
		}},
	}
	lines := ganttLines(t, s, 2)

	var firstCells []int
	for _, line := range lines {
		if i := strings.Index(line, "█"); i >= 0 {
			firstCells = append(firstCells, i-ganttLabelW-1)
		}
	}
	if len(firstCells) != 2 {
		t.Fatalf("barras = %v, want 2 (lineas: %q)", firstCells, lines)
	}
	if firstCells[0] != 0 {
		t.Errorf("primera barra en la celda %d, want 0", firstCells[0])
	}
	if firstCells[1] != 5 {
		t.Errorf("barra del 19-sep en la celda %d, want 5", firstCells[1])
	}
}

// TestRenderGanttTextRecortaFueraDeVentana: una entrada fuera del rango
// visible no pinta barra ni desborda la fila. Cubre el recorte por la derecha
// y por la izquierda (una fecha no parseable devuelve -1 en dayIndex).
func TestRenderGanttTextRecortaFueraDeVentana(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
	}{
		{"se sale por la derecha", "2026-10-01", "2026-10-05"},
		{"enteramente antes", "2020-01-01", "2020-01-02"},
		{"fecha no parseable", "no-es-fecha", "tampoco"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &model.Schedule{
				Start: "2026-09-14",
				Assignees: []model.AssigneeSchedule{{
					Assignee: "@a",
					Entries: []model.ScheduleEntry{
						{Task: model.Task{ID: 1, Title: "fuera"}, Start: tt.start, End: tt.end, Estimate: 1},
					},
				}},
			}
			for _, line := range ganttLines(t, s, 2) {
				if strings.Contains(line, "█") {
					t.Errorf("una entrada fuera de la ventana no debe pintar barra: %q", line)
				}
			}
		})
	}
}

// TestRenderGanttTextMarcaElEstimatePorDefecto: la tilde "~" sólo aparece cuando
// el estimate vino del default, no cuando la tarea lo traía explícito.
func TestRenderGanttTextMarcaElEstimatePorDefecto(t *testing.T) {
	entries := []model.ScheduleEntry{
		{Task: model.Task{ID: 1, Title: "por defecto"}, Start: "2026-09-14", End: "2026-09-14", Estimate: 1, EstimateDefaulted: true},
		{Task: model.Task{ID: 2, Title: "explicito"}, Start: "2026-09-14", End: "2026-09-14", Estimate: 1},
	}
	out := renderGanttText(&model.Schedule{
		Start:     "2026-09-14",
		Assignees: []model.AssigneeSchedule{{Assignee: "@a", End: "2026-09-14", Entries: entries}},
	}, 1)

	if strings.Count(out, "~") != 1 {
		t.Errorf("esperaba 1 marca de default, got %d:\n%s", strings.Count(out, "~"), out)
	}
}

func TestRenderGanttTextTareasSinDueno(t *testing.T) {
	s := &model.Schedule{
		Start:      "2026-09-14",
		Unassigned: []model.Task{{ID: 7, Title: "sin dueño"}, {ID: 8, Title: "otra"}},
	}
	out := renderGanttText(s, 1)
	if !strings.Contains(out, "Unassigned (2):") {
		t.Errorf("falta el recuento: %q", out)
	}
	// Sin dueño no se agenda: no debe haber ninguna barra ni línea por persona.
	if strings.Contains(out, "█") || strings.Contains(out, "ends") {
		t.Errorf("una tarea sin dueño no debería agendarse: %q", out)
	}
}

// TestRenderGanttTextEntrySpanningTheRightEdge: una entrada que termina el día
// siguiente al final de la ventana debe recortarse al último día, sin escribir
// fuera de la fila ni reventar. Cubre el `totalDays-1` del recorte: si el
// borde fuese totalDays+1, la escritura se saldría de la fila.
func TestRenderGanttTextEntrySpanningTheRightEdge(t *testing.T) {
	const weeks = 2
	const totalDays = weeks * 7

	tests := []struct {
		name     string
		end      string
		wantBars int
	}{
		{"termina en el último día visible", "2026-09-27", totalDays},
		{"termina un día después", "2026-09-28", totalDays},
		{"termina dos días después", "2026-09-29", totalDays},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &model.Schedule{
				Start: "2026-09-14",
				Assignees: []model.AssigneeSchedule{{
					Assignee: "@a",
					Entries: []model.ScheduleEntry{
						{Task: model.Task{ID: 1, Title: "al borde"}, Start: "2026-09-14", End: tt.end, Estimate: 99},
					},
				}},
			}
			line := ganttLines(t, s, weeks)[ganttHeadOff+3]
			if got := strings.Count(line, "█"); got != tt.wantBars {
				t.Errorf("%d celdas pintadas, want %d (%q)", got, tt.wantBars, line)
			}
		})
	}
}
