package tui

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

func TestDashboardMOpensAssigneeModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4") // dashboard
	m, _ = press(m, "m")

	if !m.assigneeModalOpen {
		t.Fatal("m en Dashboard debe abrir el modal de assignees")
	}
	if m.assigneeDetail {
		t.Error("debe abrir en el nivel de lista")
	}
}

func TestAssigneeRosterIncludesOffdayOnlyPerson(t *testing.T) {
	m := newTestModel(t)
	if _, err := m.database.AddOffDay("@vacation", "2026-09-01", "", ""); err != nil {
		t.Fatal(err)
	}
	offdays, _ := m.database.ListOffDays("")
	m.offdays = offdays

	var found bool
	for _, s := range m.assigneeRoster() {
		if s.Name != "@vacation" {
			continue
		}
		found = true
		if s.OffDayCount != 1 || s.Total != 0 {
			t.Errorf("resumen = %+v, want 0 tasks / 1 off-day", s)
		}
	}
	if !found {
		t.Fatalf("roster no incluye a la persona sólo con off-days: %v", m.assigneeRoster())
	}
}

func TestAssigneeRosterAlwaysHasMe(t *testing.T) {
	m := newTestModel(t)
	for _, s := range m.assigneeRoster() {
		if s.Name == "Me" {
			return
		}
	}
	t.Error("el roster siempre debe incluir a Me")
}

func TestAddOffdayFromModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m, _ = press(m, "m")
	m, _ = press(m, "a") // formulario para la persona seleccionada

	if !m.offdayFormOpen {
		t.Fatal("a debe abrir el formulario de off-day")
	}
	name := m.currentAssignee()
	if name == "" {
		t.Fatal("debe haber una persona seleccionada")
	}
	m.offdayStartInput = "2026-09-01"
	m.offdayEndInput = "2026-09-05"

	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter debe emitir el comando de alta")
	}
	saved, ok := cmd().(offdaySavedMsg)
	if !ok || saved.err != nil {
		t.Fatalf("alta msg = %#v", cmd())
	}

	next, _ := m.Update(saved)
	m = asModel(next)
	if m.toast == "" || m.toastKind != "info" {
		t.Errorf("toast = %q/%q, want info", m.toast, m.toastKind)
	}
	if !m.assigneeDetail {
		t.Error("tras el alta se debe entrar al detalle de la persona")
	}

	m = applyMsg(m, m.loadOffDays()())

	offs := m.assigneeOffDays(name)
	if len(offs) != 1 {
		t.Fatalf("off-days de %s = %d, want 1", name, len(offs))
	}
	if offs[0].StartDate != "2026-09-01" || offs[0].EndDate != "2026-09-05" {
		t.Errorf("rango = %s→%s, want 2026-09-01→2026-09-05", offs[0].StartDate, offs[0].EndDate)
	}
}

func TestAddOffdayInvalidDateKeepsFormOpen(t *testing.T) {
	m := newTestModel(t)
	m.assigneeModalOpen = true
	m.openOffdayForm()
	m.offdayStartInput = "not-a-date"

	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter debe emitir el comando de alta")
	}
	saved, ok := cmd().(offdaySavedMsg)
	if !ok || saved.err == nil {
		t.Fatalf("se esperaba error de validación, msg = %#v", cmd())
	}

	next, _ := m.Update(saved)
	m = asModel(next)
	if !m.offdayFormOpen {
		t.Error("ante error el formulario debe seguir abierto")
	}
	if m.toastKind != "error" {
		t.Errorf("toast kind = %q, want error", m.toastKind)
	}
}

func TestDeleteOffdayFromModal(t *testing.T) {
	m := newTestModel(t)
	if _, err := m.database.AddOffDay("@juan", "2026-09-01", "", "vacation"); err != nil {
		t.Fatal(err)
	}
	offdays, _ := m.database.ListOffDays("")
	m.offdays = offdays

	m.assigneeModalOpen = true
	roster := m.assigneeRoster()
	for i, s := range roster {
		if s.Name == "@juan" {
			m.assigneeIdx = i
		}
	}

	m, _ = press(m, "enter") // entrar al detalle
	if !m.assigneeDetail {
		t.Fatal("enter debe entrar al detalle de la persona")
	}

	m, _ = press(m, "d")
	if !m.confirmOpen || m.confirmAction != "delete-offday" {
		t.Fatalf("d debe pedir confirmación de borrado, action = %q", m.confirmAction)
	}
	if m.confirmOffday.Assignee != "@juan" {
		t.Errorf("confirmOffday = %+v, want @juan", m.confirmOffday)
	}

	m, cmd := press(m, "y")
	if cmd == nil {
		t.Fatal("y debe emitir el comando de borrado")
	}
	saved, ok := cmd().(offdaySavedMsg)
	if !ok || saved.err != nil {
		t.Fatalf("baja msg = %#v", cmd())
	}

	next, _ := m.Update(saved)
	m = asModel(next)
	m = applyMsg(m, m.loadOffDays()())

	if len(m.assigneeOffDays("@juan")) != 0 {
		t.Error("el off-day debía borrarse")
	}
}

func TestRenderAssigneeModal(t *testing.T) {
	m := newTestModel(t)
	m.assigneeModalOpen = true

	if out := m.renderAssigneeModal("base"); !strings.Contains(out, "Assignees") {
		t.Errorf("modal no contiene título: %q", out)
	}

	m.assigneeDetail = true
	if out := m.renderAssigneeDetail("base"); !strings.Contains(out, "Off-days") {
		t.Errorf("detalle no contiene la sección Off-days: %q", out)
	}
}

func TestConfirmModalDeleteOffday(t *testing.T) {
	m := newTestModel(t)
	m.confirmOpen = true
	m.confirmAction = "delete-offday"
	m.confirmOffday = model.OffDay{ID: 1, Assignee: "@juan", StartDate: "2026-09-01", EndDate: "2026-09-05"}

	out := m.renderConfirmModal("base")
	if !strings.Contains(out, "@juan") || !strings.Contains(out, "Delete off-day") {
		t.Errorf("confirmación de borrado inesperada: %q", out)
	}
}
