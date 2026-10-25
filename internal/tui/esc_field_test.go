package tui

import "testing"

// Esc inside a field goes one level back: it leaves the field and keeps the
// overlay open. Only Esc with no field entered closes the overlay.
func TestEscLeavesFieldBeforeClosing(t *testing.T) {
	t.Run("filter", func(t *testing.T) {
		m := newTestModel(t)
		m, _ = pressKeys(t, m, "f", "j", "enter")
		if !m.filterFieldActive {
			t.Fatal("the fixture did not enter the field")
		}
		m, _ = press(m, "esc")
		if !m.filterOpen {
			t.Error("esc inside the field closed the filter overlay")
		}
		if m.filterFieldActive {
			t.Error("esc inside the field did not leave it")
		}
		if m.filterFieldIdx != filterFieldStatus {
			t.Errorf("esc moved the field cursor to %d, want the same field", m.filterFieldIdx)
		}
		m, _ = press(m, "esc")
		if m.filterOpen {
			t.Error("esc with no field entered must close the overlay")
		}
	})

	t.Run("new task", func(t *testing.T) {
		m := newTestModel(t)
		m, _ = pressKeys(t, m, "i", "j", "enter", "h", "i")
		if !m.newTaskFieldActive {
			t.Fatal("the fixture did not enter the title")
		}
		m, _ = press(m, "esc")
		if !m.newTaskOpen {
			t.Error("esc inside the field closed the task form")
		}
		if m.newTaskFieldActive {
			t.Error("esc inside the field did not leave it")
		}
		if m.newTaskTitle != "hi" {
			t.Errorf("esc lost the typed text: %q", m.newTaskTitle)
		}
		m, _ = press(m, "esc")
		if m.newTaskOpen {
			t.Error("esc with no field entered must close the form")
		}
	})

	t.Run("project", func(t *testing.T) {
		m := newTestModel(t)
		m, _ = pressKeys(t, m, "4", "i", "enter", "x")
		if !m.projectModalActive {
			t.Fatal("the fixture did not enter the name field")
		}
		m, _ = press(m, "esc")
		if !m.projectModalOpen {
			t.Error("esc inside the field closed the project modal")
		}
		if m.projectModalActive {
			t.Error("esc inside the field did not leave it")
		}
		if m.projectNameInput != "x" {
			t.Errorf("esc lost the typed name: %q", m.projectNameInput)
		}
	})

	t.Run("off-day", func(t *testing.T) {
		m := newTestModel(t)
		m.assigneeModalOpen = true
		m.openOffdayForm()
		m, _ = pressKeys(t, m, "enter", "2")
		if !m.offdayFormActive {
			t.Fatal("the fixture did not enter the start field")
		}
		m, _ = press(m, "esc")
		if !m.offdayFormOpen {
			t.Error("esc inside the field closed the off-day form")
		}
		if m.offdayFormActive {
			t.Error("esc inside the field did not leave it")
		}
		if m.offdayStartInput != "2" {
			t.Errorf("esc lost the typed date: %q", m.offdayStartInput)
		}
	})
}
