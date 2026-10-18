package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// cursorToTask leaves the List cursor on the given task.
func cursorToTask(t *testing.T, m *Model, task *model.Task) {
	t.Helper()
	for i, tk := range m.filteredTasks() {
		if tk.ID == task.ID {
			m.cursor = i
			return
		}
	}
	t.Fatalf("task %d is not in the filtered list", task.ID)
}

// ---- Opening the overlay with "m" ----

func TestOpenStatusOverlayByView(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, m *Model) *model.Task
	}{
		{
			name: "list",
			setup: func(t *testing.T, m *Model) *model.Task {
				task := taskByStatus(t, m, "doing")
				cursorToTask(t, m, task)
				m.currentView = viewList
				return task
			},
		},
		{
			name: "kanban",
			setup: func(t *testing.T, m *Model) *model.Task {
				m.currentView = viewKanban
				m.filterProject = "api"
				m.clampKanbanCursor()
				task := m.kanbanColumns()[m.kanbanCol].tasks[m.kanbanRow]
				return &task
			},
		},
		{
			name: "detail",
			setup: func(t *testing.T, m *Model) *model.Task {
				task := taskByStatus(t, m, "doing")
				m.detailOpen = true
				m.detailTask = task
				return task
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			task := tt.setup(t, m)

			next, _ := press(m, "m")
			if !next.statusOpen {
				t.Fatal("m did not open the status overlay")
			}
			if next.statusTaskID != task.ID {
				t.Errorf("statusTaskID = %d, want %d", next.statusTaskID, task.ID)
			}
			if next.statusIdx != 0 {
				t.Errorf("statusIdx = %d, want 0", next.statusIdx)
			}
			if next.statusProjectID != 0 {
				t.Errorf("statusProjectID = %d, want 0 (keep the project)", next.statusProjectID)
			}
		})
	}
}

// From doing, the first choice is the next status and the already passed ones
// come after, with cancelled last.
func TestStatusOverlayListOrder(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	cursorToTask(t, m, task)
	m.currentView = viewList

	next, _ := press(m, "m")
	want := []string{"delivered", "reviewing", "done", "todo", "backlog", "cancelled"}
	if strings.Join(next.statusList, ",") != strings.Join(want, ",") {
		t.Errorf("statusList = %v, want %v", next.statusList, want)
	}
}

// The overlay renders the choices in order, with cancelled separated by a
// blank line, over the content.
func TestStatusOverlayRender(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	cursorToTask(t, m, task)
	m.currentView = viewList

	next, _ := press(m, "m")
	out := ansi.Strip(next.View().Content)

	if !strings.Contains(out, "Status") {
		t.Errorf("the status modal title is missing:\n%s", out)
	}
	for _, want := range next.statusList {
		if !strings.Contains(out, want) {
			t.Errorf("the choice %q is missing from the render:\n%s", want, out)
		}
	}
	// cancelled is separated: a blank line sits right before it.
	lines := statusOverlayLines(next.statusList, next.statusIdx)
	cancelledAt := -1
	for i, line := range lines {
		if strings.Contains(line, "cancelled") {
			cancelledAt = i
		}
	}
	if cancelledAt <= 0 || strings.TrimSpace(ansi.Strip(lines[cancelledAt-1])) != "" {
		t.Errorf("cancelled is not separated by a blank line: %v", lines)
	}
}

// ---- Keys inside the status overlay ----

func TestStatusOverlayMoveWraps(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	m.detailTask = task
	m.detailOpen = true

	opened, _ := press(m, "m")
	n := len(opened.statusList)

	down, _ := press(opened, "j")
	if down.statusIdx != 1 {
		t.Errorf("j left the index at %d, want 1", down.statusIdx)
	}
	up, _ := press(down, "k")
	if up.statusIdx != 0 {
		t.Errorf("k left the index at %d, want 0", up.statusIdx)
	}
	fromFirst, _ := press(opened, "k")
	if fromFirst.statusIdx != n-1 {
		t.Errorf("k from 0 wrapped to %d, want the last (%d)", fromFirst.statusIdx, n-1)
	}
	fromLast, _ := press(fromFirst, "j")
	if fromLast.statusIdx != 0 {
		t.Errorf("j from the last wrapped to %d, want 0", fromLast.statusIdx)
	}
}

func TestStatusOverlayEnterMovesAndCloses(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	cursorToTask(t, m, task)
	m.currentView = viewList

	opened, _ := press(m, "m")
	// The first choice from doing is delivered.
	if len(opened.statusList) == 0 || opened.statusList[0] != "delivered" {
		t.Fatalf("choices = %v, want delivered first", opened.statusList)
	}

	applied, cmd := press(opened, "enter")
	if applied.statusOpen {
		t.Error("enter did not close the overlay")
	}
	if cmd == nil {
		t.Fatal("enter did not emit the move command")
	}
	// Feed the reload back, as the Bubbletea runtime would.
	for _, msg := range mustRun(t, cmd) {
		applied, _ = updateMsg(t, applied, msg)
	}
	if len(applied.tasks) == 0 {
		t.Error("the listing did not reload after the move")
	}

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", task.ID, err)
	}
	if got.Status != "delivered" {
		t.Errorf("status = %q, want delivered", got.Status)
	}
}

// Enter closes the detail when the overlay was opened from it.
func TestStatusOverlayEnterClosesDetail(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	m.detailTask = task
	m.detailOpen = true

	opened, _ := press(m, "m")
	applied, cmd := press(opened, "enter")
	if applied.detailOpen {
		t.Error("enter did not close the detail")
	}
	if applied.detailTask != nil {
		t.Error("enter did not clear the detail's task")
	}
	mustRun(t, cmd)
}

// Esc closes the overlay and leaves the detail open behind it.
func TestStatusOverlayEscKeepsDetail(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	m.detailTask = task
	m.detailOpen = true

	opened, _ := press(m, "m")
	closed, cmd := press(opened, "esc")
	if closed.statusOpen {
		t.Error("esc did not close the overlay")
	}
	if !closed.detailOpen {
		t.Error("esc closed the detail, want it open")
	}
	if cmd != nil {
		t.Errorf("esc produced a command %T", cmd)
	}
}

func TestStatusOverlayIgnoresOtherKeys(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	m.detailTask = task
	m.detailOpen = true

	opened, _ := press(m, "m")
	next, cmd := press(opened, "z")
	if !next.statusOpen {
		t.Error("an unrelated key closed the overlay")
	}
	if next.statusIdx != 0 {
		t.Errorf("an unrelated key moved the cursor to %d", next.statusIdx)
	}
	if cmd != nil {
		t.Errorf("an unrelated key produced a command %T", cmd)
	}
}

// An empty list is a defensive edge: apply goes nowhere.
func TestStatusOverlayEnterWithEmptyList(t *testing.T) {
	m := newTestModel(t)
	m.statusOpen = true
	m.statusList = nil

	next, cmd := press(m, "enter")
	if cmd != nil {
		t.Errorf("enter with no choices produced a command %T", cmd)
	}
	if !next.statusOpen {
		t.Error("enter with no choices closed the overlay")
	}
}

// ---- Opening and applying the project picker with "M" ----

func TestMoveProjectExcludesCurrentProject(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	cursorToTask(t, m, task)
	m.currentView = viewList

	next, _ := press(m, "M")
	if !next.moveProjectOpen {
		t.Fatal("M did not open the project picker")
	}
	for _, p := range next.moveProjectList {
		if p.ID == task.ProjectID {
			t.Errorf("the picker lists the task's own project %q", p.Name)
		}
	}
	if len(next.moveProjectList) != len(m.projects)-1 {
		t.Errorf("picker has %d projects, want %d", len(next.moveProjectList), len(m.projects)-1)
	}
}

func TestMoveProjectSingleProjectToasts(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	cursorToTask(t, m, task)
	m.currentView = viewList
	// Only the task's own project remains.
	m.projects = []model.Project{{ID: task.ProjectID, Name: "solo"}}

	next, _ := press(m, "M")
	if next.moveProjectOpen {
		t.Error("M opened the picker with a single project")
	}
	if next.toast == "" {
		t.Error("M with a single project did not toast")
	}
}

func TestMoveProjectKeys(t *testing.T) {
	m := newTestModel(t)
	// Two extra projects so the picker holds three choices and the wrap from
	// the first to the last is distinguishable from a plain step.
	mustCreateProject(t, m.database, "extra1", model.DefaultWorkflow)
	mustCreateProject(t, m.database, "extra2", model.DefaultWorkflow)
	reloadProjects(t, m)

	task := taskByStatus(t, m, "doing")
	if cmd := m.openMoveProject(task); cmd != nil {
		t.Fatalf("openMoveProject returned a command %T", cmd)
	}
	n := len(m.moveProjectList)

	down, _ := press(m, "j")
	if down.moveProjectIdx != 1 {
		t.Errorf("j left the index at %d, want 1", down.moveProjectIdx)
	}
	up, _ := press(down, "k")
	if up.moveProjectIdx != 0 {
		t.Errorf("k left the index at %d, want 0", up.moveProjectIdx)
	}
	fromFirst, _ := press(m, "k")
	if fromFirst.moveProjectIdx != n-1 {
		t.Errorf("k from 0 wrapped to %d, want %d", fromFirst.moveProjectIdx, n-1)
	}

	closed, cmd := press(up, "esc")
	if closed.moveProjectOpen {
		t.Error("esc did not close the picker")
	}
	if cmd != nil {
		t.Errorf("esc produced a command %T", cmd)
	}
}

func TestMoveProjectIgnoresOtherKeys(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	if cmd := m.openMoveProject(task); cmd != nil {
		t.Fatalf("openMoveProject returned a command %T", cmd)
	}

	next, cmd := press(m, "z")
	if !next.moveProjectOpen {
		t.Error("an unrelated key closed the picker")
	}
	if next.moveProjectIdx != 0 {
		t.Errorf("an unrelated key moved the cursor to %d", next.moveProjectIdx)
	}
	if cmd != nil {
		t.Errorf("an unrelated key produced a command %T", cmd)
	}
}

func TestMoveProjectEnterWithInvalidIndex(t *testing.T) {
	m := newTestModel(t)
	m.moveProjectOpen = true
	m.moveProjectList = []model.Project{{ID: 1, Name: "web"}}
	m.moveProjectIdx = 99

	next, cmd := press(m, "enter")
	if cmd != nil {
		t.Errorf("enter with an out-of-range index produced a command %T", cmd)
	}
	if !next.moveProjectOpen {
		t.Error("enter with an out-of-range index changed the state")
	}
}

// Enter on the picker opens the status overlay for the chosen project.
func TestMoveProjectEnterOpensStatusOverlay(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	if cmd := m.openMoveProject(task); cmd != nil {
		t.Fatalf("openMoveProject returned a command %T", cmd)
	}
	chosen := m.moveProjectList[0]

	next, _ := press(m, "enter")
	if next.moveProjectOpen {
		t.Error("enter did not close the picker")
	}
	if !next.statusOpen {
		t.Fatal("enter did not open the status overlay")
	}
	if next.statusProjectID != chosen.ID || next.statusProjectName != chosen.Name {
		t.Errorf("target = %d/%q, want %d/%q", next.statusProjectID, next.statusProjectName, chosen.ID, chosen.Name)
	}
}

// "M" then status applies both the project and the status.
func TestMoveProjectThenStatusAppliesBoth(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	if cmd := m.openMoveProject(task); cmd != nil {
		t.Fatalf("openMoveProject returned a command %T", cmd)
	}
	chosen := m.moveProjectList[0]

	opened, _ := press(m, "enter")
	if len(opened.statusList) == 0 {
		t.Fatal("the status overlay has no choices")
	}
	choice := opened.statusList[0]

	applied, cmd := press(opened, "enter")
	if applied.statusOpen || applied.moveProjectOpen {
		t.Error("applying did not close the overlays")
	}
	if cmd == nil {
		t.Fatal("applying emitted no command")
	}
	mustRun(t, cmd)

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", task.ID, err)
	}
	if got.ProjectID != chosen.ID || got.ProjectName != chosen.Name {
		t.Errorf("project = %d/%q, want %d/%q", got.ProjectID, got.ProjectName, chosen.ID, chosen.Name)
	}
	if got.Status != choice {
		t.Errorf("status = %q, want %q", got.Status, choice)
	}
}

// The overlay from the detail applies the move too.
func TestMoveProjectFromDetailApplies(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	m.detailTask = task
	m.detailOpen = true

	opened, _ := press(m, "M")
	if !opened.moveProjectOpen {
		t.Fatal("M did not open the picker")
	}
	chosen := opened.moveProjectList[0]
	statusOpen, _ := press(opened, "enter")
	choice := statusOpen.statusList[0]

	applied, cmd := press(statusOpen, "enter")
	if applied.detailOpen {
		t.Error("applying did not close the detail")
	}
	mustRun(t, cmd)

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", task.ID, err)
	}
	if got.ProjectID != chosen.ID || got.Status != choice {
		t.Errorf("project/status = %d/%q, want %d/%q", got.ProjectID, got.Status, chosen.ID, choice)
	}
}

// ---- Defensive edges of the open helpers ----

func TestOpenStatusWithNilTask(t *testing.T) {
	m := newTestModel(t)
	if cmd := m.openStatus(nil); cmd != nil {
		t.Errorf("openStatus(nil) returned a command %T", cmd)
	}
	if m.statusOpen {
		t.Error("openStatus(nil) opened the overlay")
	}
}

func TestOpenStatusWithUnknownProject(t *testing.T) {
	m := newTestModel(t)
	ghost := model.Task{ID: 99, ProjectName: "ghost"}

	if cmd := m.openStatus(&ghost); cmd != nil {
		t.Errorf("openStatus with an unknown project returned a command %T", cmd)
	}
	if m.statusOpen {
		t.Error("openStatus with an unknown project opened the overlay")
	}
}

func TestOpenStatusForProjectWithNilTask(t *testing.T) {
	m := newTestModel(t)
	if cmd := m.openStatusForProject(nil, 1, "web", model.DefaultWorkflow); cmd != nil {
		t.Errorf("openStatusForProject(nil) returned a command %T", cmd)
	}
	if m.statusOpen {
		t.Error("openStatusForProject(nil) opened the overlay")
	}
}

func TestOpenMoveProjectWithNilTask(t *testing.T) {
	m := newTestModel(t)
	if cmd := m.openMoveProject(nil); cmd != nil {
		t.Errorf("openMoveProject(nil) returned a command %T", cmd)
	}
	if m.moveProjectOpen {
		t.Error("openMoveProject(nil) opened the picker")
	}
}

func TestMoveProjectEnterWithUnknownTask(t *testing.T) {
	m := newTestModel(t)
	m.moveProjectOpen = true
	m.moveProjectTaskID = 9999
	m.moveProjectList = []model.Project{{ID: 1, Name: "web"}}

	next, cmd := press(m, "enter")
	if cmd != nil {
		t.Errorf("enter with an unknown task produced a command %T", cmd)
	}
	if next.moveProjectOpen {
		t.Error("enter with an unknown task left the picker open")
	}
}

func TestTaskByID(t *testing.T) {
	m := newTestModel(t)

	if got := m.taskByID(-1); got != nil {
		t.Errorf("taskByID(-1) = %v, want nil", got)
	}
	if got := m.taskByID(m.tasks[0].ID); got == nil || got.ID != m.tasks[0].ID {
		t.Errorf("taskByID did not find a loaded task")
	}

	// The detail's copy wins.
	task := taskByStatus(t, m, "doing")
	copyOfTask := *task
	copyOfTask.Title = "from detail"
	m.detailTask = &copyOfTask
	if got := m.taskByID(copyOfTask.ID); got == nil || got.Title != "from detail" {
		t.Errorf("taskByID did not prefer the detail's copy: %v", got)
	}
}

// ---- Old keys are inert ----

func TestOldStatusKeysInertInListAndKanban(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		for _, key := range []string{"s", "d", "x"} {
			m := newTestModel(t)
			m.currentView = viewList
			before := m.filteredTasks()[0].Status

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("list key %q produced a command %T", key, cmd)
			}
			if next.statusOpen || next.moveProjectOpen {
				t.Errorf("list key %q opened an overlay", key)
			}
			if next.filteredTasks()[0].Status != before {
				t.Errorf("list key %q changed the status", key)
			}
		}
	})

	t.Run("kanban", func(t *testing.T) {
		for _, key := range []string{"s", "S", "d", "x"} {
			m := newKanbanModel(t, 3)
			m.currentView = viewKanban

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("kanban key %q produced a command %T", key, cmd)
			}
			if next.statusOpen || next.moveProjectOpen {
				t.Errorf("kanban key %q opened an overlay", key)
			}
		}
	})

	t.Run("detail", func(t *testing.T) {
		for _, key := range []string{"s", "x"} {
			m := newDetailModel(t, 0)
			m.detailOpen = true

			next, cmd := press(m, key)
			if cmd != nil {
				t.Errorf("detail key %q produced a command %T", key, cmd)
			}
			if !next.detailOpen || next.statusOpen {
				t.Errorf("detail key %q changed the open modal", key)
			}
		}
	})
}

// ---- Routing and keybinds ----

func TestOverlayKindStatusAndMoveProject(t *testing.T) {
	m := newTestModel(t)

	m.statusOpen = true
	if got := m.overlayKind(); got != overlayStatus {
		t.Errorf("with the status overlay: %v, want overlayStatus", got)
	}

	m.statusOpen = false
	m.moveProjectOpen = true
	if got := m.overlayKind(); got != overlayMoveProject {
		t.Errorf("with the move picker: %v, want overlayMoveProject", got)
	}

	// The status overlay wins over the move picker, matching handleKey.
	m.statusOpen = true
	if got := m.overlayKind(); got != overlayStatus {
		t.Errorf("with both overlays: %v, want overlayStatus", got)
	}
}

func TestKeybindsBarStatusOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetOverlay(overlayStatus)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "Keybinds · Status") {
		t.Errorf("the title must indicate the Status context:\n%s", out)
	}
	for _, want := range []string{"move", "select", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is missing in the status keybinds:\n%s", want, out)
		}
	}
}

func TestKeybindsBarMoveProjectOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetOverlay(overlayMoveProject)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "Keybinds · Move to project") {
		t.Errorf("the title must indicate the Move to project context:\n%s", out)
	}
	for _, want := range []string{"move", "select", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is missing in the move keybinds:\n%s", want, out)
		}
	}
}

func TestViewRendersMoveProjectModal(t *testing.T) {
	m := newTestModel(t)
	task := taskByStatus(t, m, "doing")
	if cmd := m.openMoveProject(task); cmd != nil {
		t.Fatalf("openMoveProject returned a command %T", cmd)
	}

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Move to project") {
		t.Errorf("the move modal is missing:\n%s", out)
	}
	for _, p := range m.moveProjectList {
		if !strings.Contains(out, p.Name) {
			t.Errorf("project %q is missing from the render:\n%s", p.Name, out)
		}
	}
}

func TestStatusOverlayEnterAppliesMoveTask(t *testing.T) {
	// A direct tea.Msg path: the same-project move uses MoveTask, not the
	// cross-project one. It is pinned so a wrong branch is caught.
	m := newTestModel(t)
	task := taskByStatus(t, m, "backlog")
	m.statusOpen = true
	m.statusTaskID = task.ID
	m.statusList = []string{"todo"}
	m.statusProjectID = 0

	_, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter emitted no command")
	}
	mustRun(t, cmd)

	got, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("status = %q, want todo", got.Status)
	}
}
