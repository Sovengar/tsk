package tui

import "testing"

// Shift+Tab cycles the projects backwards in every view: the opposite
// direction of Tab.
func TestShiftTabCyclesProjectsBackwards(t *testing.T) {
	for _, view := range []string{"1", "2", "3"} {
		t.Run("view "+view, func(t *testing.T) {
			m := newTestModel(t)
			m, _ = press(m, view)

			// From "all projects", backwards lands on the last project.
			m, _ = press(m, "shift+tab")
			if m.filterProject != "web" {
				t.Errorf("shift+tab from all = %q, want web", m.filterProject)
			}
			m, _ = press(m, "shift+tab")
			if m.filterProject != "api" {
				t.Errorf("second shift+tab = %q, want api", m.filterProject)
			}
			// And Tab undoes it.
			m, _ = press(m, "tab")
			if m.filterProject != "web" {
				t.Errorf("tab after two back = %q, want web", m.filterProject)
			}
		})
	}
}

// In the Dashboard, Shift+Tab moves the project selection backwards.
func TestShiftTabCyclesDashboardProjectsBackwards(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "4")
	if len(m.dashProjectList()) < 2 {
		t.Fatalf("the fixture left %d projects", len(m.dashProjectList()))
	}

	m, _ = press(m, "shift+tab")
	if m.dashProjectIdx != len(m.dashProjectList())-1 {
		t.Errorf("shift+tab from 0 = %d, want the last one", m.dashProjectIdx)
	}
	m, _ = press(m, "tab")
	if m.dashProjectIdx != 0 {
		t.Errorf("tab after the wrap = %d, want 0", m.dashProjectIdx)
	}
}
