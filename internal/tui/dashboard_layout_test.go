package tui

import (
	"testing"

	"tsk/internal/model"
)

// Estas funciones se extrajeron del render del Dashboard, que repetía la misma
// cuenta sobre m.tasks cinco veces con su propio filtro de proyecto dentro. Como
// son puras, se prueban sin modelo, sin base de datos y sin renderizar nada.

func dashTasks(fixture ...model.Task) []model.Task { return fixture }

var dashFixture = dashTasks(
	model.Task{ID: 1, ProjectName: "api", Assignee: "@juan", Status: "todo"},
	model.Task{ID: 2, ProjectName: "api", Assignee: "@juan", Status: "doing"},
	model.Task{ID: 3, ProjectName: "api", Assignee: "@maria", Status: "done"},
	model.Task{ID: 4, ProjectName: "web", Assignee: "@juan", Status: "todo"},
	model.Task{ID: 5, ProjectName: "web", Assignee: "@ana", Status: "cancelled"},
	model.Task{ID: 6, ProjectName: "web", Assignee: "@ana", Status: "reviewing"},
)

// dashProjectTasks cuenta las activas de un proyecto; "" cuenta el entero.
func TestDashProjectTasks(t *testing.T) {
	tests := []struct {
		name    string
		project string
		want    int
	}{
		{"api tiene 2 activas de 3", "api", 2},
		{"web tiene 2 activas de 3", "web", 2},
		{"sin proyecto cuenta todas las activas", "", 4},
		{"proyecto inexistente", "nope", 0},
		{"sin tareas", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := dashFixture
			if tt.name == "sin tareas" {
				tasks = nil
			}
			if got := dashProjectTasks(tasks, tt.project); got != tt.want {
				t.Errorf("dashProjectTasks(%q) = %d, want %d", tt.project, got, tt.want)
			}
		})
	}
}

// Sólo cuenta las activas: done y cancelled quedan fuera aunque pertenezcan al
// proyecto. Es lo que distingue esta cuenta de la de estados.
func TestDashProjectTasksOnlyActive(t *testing.T) {
	all := dashProjectTasks(dashFixture, "")
	_, done, cancelled, _ := dashStatusCounts(dashFixture, "")
	if all+done+cancelled != len(dashFixture) {
		t.Errorf("activas(%d) + done(%d) + cancelled(%d) = %d, want %d tareas",
			all, done, cancelled, all+done+cancelled, len(dashFixture))
	}
}

// dashStatusCounts: done y cancelled aparte, todo lo demás activa.
func TestDashStatusCounts(t *testing.T) {
	tests := []struct {
		name                             string
		project                          string
		wantActive, wantDone, wantCancel int
	}{
		{"todo el conjunto", "", 4, 1, 1},
		{"sólo api", "api", 2, 1, 0},
		{"sólo web", "web", 2, 0, 1},
		{"proyecto inexistente", "nope", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, done, cancelled, byStatus := dashStatusCounts(dashFixture, tt.project)
			if active != tt.wantActive || done != tt.wantDone || cancelled != tt.wantCancel {
				t.Errorf("dashStatusCounts(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tt.project, active, done, cancelled, tt.wantActive, tt.wantDone, tt.wantCancel)
			}
			// El mapa porStatus lleva TODOS los estados, no sólo activas:
			// son las barras del Overview, una por estado del workflow.
			sum := 0
			for _, n := range byStatus {
				sum += n
			}
			var want int
			switch tt.project {
			case "api", "web":
				want = 3
			case "nope":
				want = 0
			default:
				want = 6
			}
			if sum != want {
				t.Errorf("byStatus suma %d, want %d (%v)", sum, want, byStatus)
			}
		})
	}
}

// El mapa porStatus incluye los estados done y cancelled, no sólo los activos:
// las barras del Overview se pintan para todo el workflow.
func TestDashStatusCountsMapIncludesDoneAndCancelled(t *testing.T) {
	_, _, _, byStatus := dashStatusCounts(dashFixture, "")
	for _, want := range []string{"todo", "doing", "done", "cancelled", "reviewing"} {
		if _, ok := byStatus[want]; !ok {
			t.Errorf("byStatus no tiene %q: %v", want, byStatus)
		}
	}
	if byStatus["done"] != 1 || byStatus["cancelled"] != 1 {
		t.Errorf("done/cancelled mal contados: %v", byStatus)
	}
}

// Un estado desconocido cuenta como activa: no está en done ni en cancelled.
func TestDashStatusCountsUnknownStatusIsActive(t *testing.T) {
	tasks := dashTasks(model.Task{ProjectName: "api", Status: "backlog"})
	active, done, cancelled, byStatus := dashStatusCounts(tasks, "")
	if active != 1 || done != 0 || cancelled != 0 {
		t.Errorf("un estado propio del workflow debe contar como activa, dio (%d,%d,%d)", active, done, cancelled)
	}
	if byStatus["backlog"] != 1 {
		t.Errorf("byStatus[backlog] = %d, want 1", byStatus["backlog"])
	}
}

// dashAssigneeCounts: total y subcuenta de activas por persona.
func TestDashAssigneeCounts(t *testing.T) {
	total, active := dashAssigneeCounts(dashFixture, "")
	if total["@juan"] != 3 {
		t.Errorf("total[@juan] = %d, want 3", total["@juan"])
	}
	// Las tres de @juan están activas; la única done del fixture es de @maria.
	if active["@juan"] != 3 {
		t.Errorf("activas[@juan] = %d, want 3 (ninguna suya está cerrada)", active["@juan"])
	}
	if total["@maria"] != 1 || active["@maria"] != 0 {
		t.Errorf("@maria = %d total / %d activas, want 1/0", total["@maria"], active["@maria"])
	}
	if total["@ana"] != 2 || active["@ana"] != 1 {
		t.Errorf("@ana = %d total / %d activas, want 2/1 (una está cancelled)", total["@ana"], active["@ana"])
	}
}

// El filtro de proyecto recorta las dos cuentas a la vez, no sólo el total.
func TestDashAssigneeCountsFilterByProject(t *testing.T) {
	total, active := dashAssigneeCounts(dashFixture, "api")
	if total["@juan"] != 2 || active["@juan"] != 2 {
		t.Errorf("en api @juan = %d/%d, want 2/2", total["@juan"], active["@juan"])
	}
	if _, ok := total["@ana"]; ok {
		t.Errorf("@ana no tiene tareas en api, pero aparece: %v", total)
	}
}

// Ninguna persona puede tener más activas que totales.
func TestDashAssigneeCountsActiveNeverExceedsTotal(t *testing.T) {
	total, active := dashAssigneeCounts(dashFixture, "")
	for person, n := range active {
		if n > total[person] {
			t.Errorf("%s: %d activas sobre %d totales", person, n, total[person])
		}
	}
}

// dashRowsAvailable: lo que queda tras lo usado y la cabecera. El suelo es 0.
func TestDashRowsAvailable(t *testing.T) {
	tests := []struct {
		name                       string
		colLines, used, headerRows int
		want                       int
	}{
		{"con sitio de sobra", 20, 5, 3, 12},
		{"justo", 10, 5, 3, 2},
		{"cabe justo una fila", 9, 5, 3, 1},
		{"sin filas", 8, 5, 3, 0},
		{"sobrepasado da 0", 3, 5, 3, 0},
		{"muy sobrepasado da 0", 0, 50, 3, 0},
		// Con alturas negativas la resta da un número positivo: el suelo protege
		// del caso real (la columna se quedó sin espacio), no de entradas
		// imposibles.
		{"restas que dan positivo", -5, -8, -2, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashRowsAvailable(tt.colLines, tt.used, tt.headerRows)
			if got != tt.want {
				t.Errorf("dashRowsAvailable(%d, %d, %d) = %d, want %d",
					tt.colLines, tt.used, tt.headerRows, got, tt.want)
			}
			if got < 0 {
				t.Errorf("nunca puede ser negativo, dio %d", got)
			}
		})
	}
}

// Un presupuesto de 0 significa "no cabe nada más", y por eso el suelo es 0 y no
// 1: con 1 el bloque crecería una línea sobre el alto calculado.
func TestDashRowsAvailableZeroMeansNothing(t *testing.T) {
	if got := dashRowsAvailable(8, 5, 3); got != 0 {
		t.Fatalf("presupuesto 0 esperado, dio %d", got)
	}
	if got := dashRowsAvailable(9, 5, 3); got != 1 {
		t.Fatalf("presupuesto 1 esperado, dio %d", got)
	}
}

// dashColumnWidths: dos columnas iguales con un hueco de 1 en medio.
func TestDashColumnWidths(t *testing.T) {
	tests := []struct {
		name                string
		innerW              int
		wantLeft, wantRight int
	}{
		{"ancho normal", 78, 38, 38},
		{"impar", 79, 38, 38},
		{"muy par", 80, 39, 39},
		{"estrecho", 10, 4, 4},
		{"mínimamente útil", 6, 2, 2},
		{"insuficiente", 4, 1, 1},
		{"cero", 0, -1, -1},
		{"negativo", -10, -6, -6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, right := dashColumnWidths(tt.innerW)
			if left != tt.wantLeft || right != tt.wantRight {
				t.Errorf("dashColumnWidths(%d) = (%d,%d), want (%d,%d)",
					tt.innerW, left, right, tt.wantLeft, tt.wantRight)
			}
			if left != right {
				t.Errorf("las columnas deben ser iguales, dio %d y %d", left, right)
			}
		})
	}
}
