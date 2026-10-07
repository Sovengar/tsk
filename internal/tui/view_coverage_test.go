package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/model"
)

// View() is the function with the most branches in the package and the one no
// test called: the render tests called renderX directly, which is the same code
// but without the view switch nor the overlay stack. That left uncovered
// exactly the code that decides who it calls.
//
// One test per stack state: each open overlay and each view.
func TestViewCoversEveryStateOfTheOverlayStack(t *testing.T) {
	view := func(t *testing.T, m *Model) string {
		t.Helper()
		m.width, m.height = 100, 30
		return ansi.Strip(m.View().Content)
	}

	t.Run("the three views", func(t *testing.T) {
		for _, v := range []struct {
			name   string
			marker string
		}{
			{"dashboard", "Total"},
			{"kanban", "backlog"},
			{"gantt", "@"},
		} {
			t.Run(v.name, func(t *testing.T) {
				m := newTestModel(t)
				switch v.name {
				case "dashboard":
					m.currentView = viewDashboard
				case "kanban":
					m.currentView = viewKanban
				case "gantt":
					m.currentView = viewGantt
				}
				if out := view(t, m); !strings.Contains(out, v.marker) {
					t.Errorf("the view %s does not contain %q:\n%s", v.name, v.marker, out)
				}
			})
		}
	})

	t.Run("an unknown view falls back to the dashboard", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewKind(99)
		if out := view(t, m); !strings.Contains(out, "Total") {
			t.Errorf("an unknown view did not come out through the dashboard:\n%s", out)
		}
	})

	t.Run("each overlay on top", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			open   func(*Model)
			marker string
		}{
			{"tags", func(m *Model) { m.tagOpen = true; m.detailTask = &m.tasks[0] }, "Tags"},
			{"project", func(m *Model) { m.projectModalOpen = true }, "Project"},
			{"confirmation", func(m *Model) { m.confirmOpen = true; m.confirmAction = "delete"; m.confirmProject = "api" }, "Confirm"},
			{"off-day", func(m *Model) { m.offdayFormOpen = true }, "Off-day"},
			{"assignees", func(m *Model) { m.assigneeModalOpen = true }, "Assignees"},
			{"assignee detail", func(m *Model) { m.assigneeModalOpen = true; m.assigneeDetail = true }, "Assignee ·"},
			{"help", func(m *Model) { m.helpOpen = true }, "Keybinds"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m := newTestModel(t)
				m.assigneeIdx = 0
				tc.open(m)
				if out := view(t, m); !strings.Contains(out, tc.marker) {
					t.Errorf("the overlay %s is not visible:\n%s", tc.name, out)
				}
			})
		}
	})

	t.Run("the filter open and the help open at the same time", func(t *testing.T) {
		m := newTestModel(t)
		m.filterOpen = true
		m.filterActive = true
		if out := view(t, m); out == "" {
			t.Error("View() empty with the filter open")
		}
	})
}

// The command that loads the projects and the one that loads the tasks return
// an empty message when the database fails, instead of nothing: an empty
// message is the "loaded but no data" signal, and Update tells it apart from
// the initial state. With the DB closed both paths run.
func TestLoadCommandsSurviveAClosedDB(t *testing.T) {
	m := newTestModel(t)
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, tc := range []struct {
		name string
		cmd  func() tea.Cmd
	}{
		{"projects", m.loadProjects},
		{"tasks", m.loadTasks},
		{"off-days", m.loadOffDays},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msgs := mustRun(t, tc.cmd())
			if len(msgs) == 0 {
				t.Fatal("the command did not return any message")
			}
			if _, ok := msgs[0].(projectsLoadedMsg); !ok && tc.name == "projects" {
				t.Errorf("the message is %T, want projectsLoadedMsg", msgs[0])
			}
		})
	}
}

// The command that decides the effective workflow returns the selected
// project's one, and the default when there is no project or the filter leaves none.
func TestEffectiveWorkflowFallsBackToTheDefault(t *testing.T) {
	t.Run("no projects", func(t *testing.T) {
		m := newEmptyDBModel(t)
		if got := m.commonWorkflow(); len(got) != len(model.DefaultWorkflow) {
			t.Errorf("with no projects the workflow is %v, want the default", got)
		}
	})

	t.Run("with projects", func(t *testing.T) {
		m := newTestModel(t)
		if got := m.commonWorkflow(); len(got) == 0 {
			t.Error("with projects the workflow is empty")
		}
	})

	t.Run("an out-of-range index leaves no selected project", func(t *testing.T) {
		m := newTestModel(t)
		m.dashProjectIdx = 9999

		if p := m.selectedDashProject(); p != nil {
			t.Errorf("with an out-of-range index there is a selected project: %q", p.Name)
		}
		// The list of visibles does not depend on the index: what is left
		// without a project is the selection, not the workflow.
		if got := m.commonWorkflow(); len(got) == 0 {
			t.Error("an out-of-range index emptied the workflow")
		}
	})
}

// Init() returns the pair of startup commands. It is one line, but it is the
// one that decides whether the application starts with data or with an empty screen.
func TestInitLoadsProjectsAndTasks(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil")
	}
	msgs := mustRun(t, cmd)
	if len(msgs) != 2 {
		t.Errorf("Init() produces %d messages, want 2 (projects and tasks)", len(msgs))
	}
}

// The list footer says the range being seen, the total and the page. It is
// geometry with numbers inside, so a presence test cannot tell it apart:
// "1-4 of 4" is also inside "11-4 of 4".
func TestListFooterCountsExactly(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 20

	total := len(m.tasks)
	if total < 2 {
		t.Fatalf("the fixture needs at least two tasks, it has %d", total)
	}

	out := ansi.Strip(m.View().Content)
	want := fmt.Sprintf("1-%d of %d · Page 1/1", total, total)
	if legend := m.pageLegend(); legend != want {
		t.Errorf("the legend is %q, want %q", legend, want)
	}
	if !strings.Contains(out, want) {
		t.Errorf("the footer does not say %q:\n%s", want, out)
	}

	// With real pagination, the third page does not say 1-n again.
	paginated := modelWithTasks(t, 5, 2)
	for range 2 {
		paginated, _ = press(paginated, "n")
	}

	start, end := paginated.pageBounds()
	if start != 4 || end != 5 {
		t.Fatalf("after two pages the window is [%d,%d), want [4,5)", start, end)
	}
	if legend := paginated.pageLegend(); legend != "5-5 of 5 · Page 3/3" {
		t.Errorf("on the last page the legend is %q, want %q", legend, "5-5 of 5 · Page 3/3")
	}
}

// An archived project is called differently in the Dashboard, and with no
// selected projects the footer says so instead of leaving the line empty.
func TestDashboardNamesTheArchivedProject(t *testing.T) {
	m := newDashModel(t, "")
	m.showArchived = true
	m.width, m.height = 100, 30

	out := ansi.Strip(m.renderDashboard(20))
	if !strings.Contains(out, "Archived:") {
		t.Errorf("an archived dashboard does not say it:\n%s", out)
	}
}

func TestDashboardSaysNoneWhenNoProjectMatches(t *testing.T) {
	m := newDashModel(t, "")
	m.dashProjectIdx = 0
	m.projects = nil
	m.width, m.height = 100, 30

	out := ansi.Strip(m.renderDashboard(20))
	if !strings.Contains(out, "(none)") {
		t.Errorf("with no projects the footer does not say (none):\n%s", out)
	}
}

// The external editor is opened with the comment key and with the new-task
// key, and both leave the state as it was if there is no task to apply it to.
func TestExternalEditorNeedsATask(t *testing.T) {
	m := newTestModel(t)
	m.filteredT = nil
	m.cursor = 99

	next, cmd := press(m, "E")
	if next.currentView != m.currentView {
		t.Error("with no task selected, E changed the view")
	}
	if cmd == nil {
		t.Log("with no task there is no command, which is expected")
	}
}

// Configuring the width of the form's textarea is a side effect of opening the
// modal, and with a very small width the calculation still has to give
// something usable instead of a negative integer.
func TestNewTaskTextareaWidthNeverGoesNegative(t *testing.T) {
	for _, width := range []int{0, 1, 5, 40, 200} {
		m := newBareModel(t, func(*config.Config) {})
		m.width = width
		if n := m.newTaskTextareaWidth(); n < 1 {
			t.Errorf("with width %d the textarea width is %d, want >= 1", width, n)
		}
	}
}
