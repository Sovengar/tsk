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
		t.Fatal("m in the Dashboard must open the assignees modal")
	}
	if m.assigneeDetail {
		t.Error("it must open at the list level")
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
			t.Errorf("summary = %+v, want 0 tasks / 1 off-day", s)
		}
	}
	if !found {
		t.Fatalf("the roster does not include the person with only off-days: %v", m.assigneeRoster())
	}
}

func TestAssigneeRosterAlwaysHasMe(t *testing.T) {
	m := newTestModel(t)
	for _, s := range m.assigneeRoster() {
		if s.Name == "Me" {
			return
		}
	}
	t.Error("the roster must always include Me")
}

func TestAddOffdayFromModal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	m, _ = press(m, "m")
	m, _ = press(m, "a") // form for the selected person

	if !m.offdayFormOpen {
		t.Fatal("a must open the off-day form")
	}
	name := m.currentAssignee()
	if name == "" {
		t.Fatal("there must be a selected person")
	}
	m.offdayStartInput = "2026-09-01"
	m.offdayEndInput = "2026-09-05"

	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter must emit the create command")
	}
	saved, ok := mustMsg(t, cmd).(offdaySavedMsg)
	if !ok || saved.err != nil {
		t.Fatalf("create msg = %#v", mustMsg(t, cmd))
	}

	next, _ := m.Update(saved)
	m = asModel(next)
	if m.toast == "" || m.toastKind != "info" {
		t.Errorf("toast = %q/%q, want info", m.toast, m.toastKind)
	}
	if !m.assigneeDetail {
		t.Error("after creating, it must enter the person's detail")
	}

	m = applyMsg(t, m, m.loadOffDays()())

	offs := m.assigneeOffDays(name)
	if len(offs) != 1 {
		t.Fatalf("off-days for %s = %d, want 1", name, len(offs))
	}
	if offs[0].StartDate != "2026-09-01" || offs[0].EndDate != "2026-09-05" {
		t.Errorf("range = %s→%s, want 2026-09-01→2026-09-05", offs[0].StartDate, offs[0].EndDate)
	}
}

func TestAddOffdayInvalidDateKeepsFormOpen(t *testing.T) {
	m := newTestModel(t)
	m.assigneeModalOpen = true
	m.openOffdayForm()
	m.offdayStartInput = "not-a-date"

	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter must emit the create command")
	}
	saved, ok := mustMsg(t, cmd).(offdaySavedMsg)
	if !ok || saved.err == nil {
		t.Fatalf("a validation error was expected, msg = %#v", mustMsg(t, cmd))
	}

	next, _ := m.Update(saved)
	m = asModel(next)
	if !m.offdayFormOpen {
		t.Error("on error the form must stay open")
	}
	if m.toastKind != "error" {
		t.Errorf("toast kind = %q, want error", m.toastKind)
	}
}

func TestDeleteOffdayFromModal(t *testing.T) {
	m := newTestModel(t)
	if _, err := m.database.AddOffDay("@john", "2026-09-01", "", "vacation"); err != nil {
		t.Fatal(err)
	}
	offdays, _ := m.database.ListOffDays("")
	m.offdays = offdays

	m.assigneeModalOpen = true
	roster := m.assigneeRoster()
	for i, s := range roster {
		if s.Name == "@john" {
			m.assigneeIdx = i
		}
	}

	m, _ = press(m, "enter") // enter the detail
	if !m.assigneeDetail {
		t.Fatal("enter must go into the person's detail")
	}

	m, _ = press(m, "d")
	if !m.confirmOpen || m.confirmAction != "delete-offday" {
		t.Fatalf("d must ask for delete confirmation, action = %q", m.confirmAction)
	}
	if m.confirmOffday.Assignee != "@john" {
		t.Errorf("confirmOffday = %+v, want @john", m.confirmOffday)
	}

	m, cmd := press(m, "y")
	if cmd == nil {
		t.Fatal("y must emit the delete command")
	}
	saved, ok := mustMsg(t, cmd).(offdaySavedMsg)
	if !ok || saved.err != nil {
		t.Fatalf("delete msg = %#v", mustMsg(t, cmd))
	}

	next, _ := m.Update(saved)
	m = asModel(next)
	m = applyMsg(t, m, m.loadOffDays()())

	if len(m.assigneeOffDays("@john")) != 0 {
		t.Error("the off-day should have been deleted")
	}
}

func TestRenderAssigneeModal(t *testing.T) {
	m := newTestModel(t)
	m.assigneeModalOpen = true

	if out := m.renderAssigneeModal("base"); !strings.Contains(out, "Assignees") {
		t.Errorf("the modal has no title: %q", out)
	}

	m.assigneeDetail = true
	if out := m.renderAssigneeDetail("base"); !strings.Contains(out, "Off-days") {
		t.Errorf("the detail has no Off-days section: %q", out)
	}
}

func TestConfirmModalDeleteOffday(t *testing.T) {
	m := newTestModel(t)
	m.confirmOpen = true
	m.confirmAction = "delete-offday"
	m.confirmOffday = model.OffDay{ID: 1, Assignee: "@john", StartDate: "2026-09-01", EndDate: "2026-09-05"}

	out := m.renderConfirmModal("base")
	if !strings.Contains(out, "@john") || !strings.Contains(out, "Delete off-day") {
		t.Errorf("unexpected delete confirmation: %q", out)
	}
}
