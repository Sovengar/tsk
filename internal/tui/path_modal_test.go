package tui

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

func TestDetailPathKeyOpensTheEditor(t *testing.T) {
	m := newTestModel(t)
	m.detailOpen = true
	m.detailTask = &m.tasks[0]
	m.detailTask.Path = "/old/dir"

	next, cmd := m.handleDetailKey("P")
	got := next.(Model)
	if cmd != nil {
		t.Error("opening the path editor must not run a command")
	}
	if !got.pathOpen || got.pathTaskID != m.detailTask.ID {
		t.Fatalf("path editor not open for task %d: open=%v id=%d", m.detailTask.ID, got.pathOpen, got.pathTaskID)
	}
	if got.pathInput != "/old/dir" {
		t.Errorf("path input = %q, want the task path", got.pathInput)
	}
}

func TestOpenPathNeedsATask(t *testing.T) {
	m := newTestModel(t)
	if cmd := m.openPath(nil); cmd != nil {
		t.Error("openPath(nil) = a command, want nil")
	}
	if m.pathOpen {
		t.Error("openPath(nil) opened the editor")
	}
}

func TestHandlePathKeyEditsAndCancels(t *testing.T) {
	m := newTestModel(t)
	m.pathOpen = true
	m.pathTaskID = 1
	m.pathInput = "ab"

	next, _ := m.handlePathKey("backspace")
	mm := next.(Model)
	m = &mm
	if m.pathInput != "a" {
		t.Errorf("after backspace = %q, want a", m.pathInput)
	}

	next, _ = m.handlePathKey("c")
	mm = next.(Model)
	m = &mm
	if m.pathInput != "ac" {
		t.Errorf("after typing = %q, want ac", m.pathInput)
	}

	next, _ = m.handlePathKey("esc")
	mm = next.(Model)
	m = &mm
	if m.pathOpen || m.pathInput != "" || m.pathTaskID != 0 {
		t.Errorf("esc did not close the editor: %+v", m)
	}
}

func TestApplyPathSavesAndKeepsTheDetail(t *testing.T) {
	m := newTestModel(t)
	task := m.tasks[0]
	m.detailOpen = true
	m.detailTask = &m.tasks[0]
	m.pathOpen = true
	m.pathTaskID = task.ID
	m.pathInput = "  /work/dir  "

	next, cmd := m.handlePathKey("enter")
	mm := next.(Model)
	m = &mm
	if m.pathOpen {
		t.Error("enter did not close the editor")
	}
	if cmd == nil {
		t.Fatal("enter returned no command")
	}

	msg, ok := cmd().(pathSavedMsg)
	if !ok {
		t.Fatal("the saved command did not return pathSavedMsg")
	}
	if msg.err != nil {
		t.Fatalf("saving the path: %v", msg.err)
	}
	if msg.task == nil || msg.task.Path != "/work/dir" {
		t.Fatalf("saved task path = %+v, want /work/dir", msg.task)
	}

	next, _ = m.Update(msg)
	updated := next.(Model)
	if !updated.detailOpen || updated.detailTask == nil || updated.detailTask.Path != "/work/dir" {
		t.Errorf("the detail did not receive the saved task: %+v", updated.detailTask)
	}
}

func TestSavePathCmdReportsErrors(t *testing.T) {
	m := newTestModel(t)
	if err := m.database.Close(); err != nil {
		t.Fatal(err)
	}
	msg := m.savePathCmd(1, "/work/dir")().(pathSavedMsg)
	if msg.err == nil {
		t.Fatal("savePathCmd on a closed DB = nil error")
	}

	next, cmd := m.Update(msg)
	updated := next.(Model)
	if cmd == nil {
		t.Error("a failed save must raise a toast")
	}
	if updated.toast == "" {
		t.Error("a failed save set no toast")
	}
}

func TestRenderPathModal(t *testing.T) {
	m := newTestModel(t)

	withTask := m
	withTask.detailTask = &m.tasks[0]
	withTask.pathInput = "/typed"
	if out := withTask.renderPathModal("base"); !strings.Contains(out, "Task path") || !strings.Contains(out, "/typed") {
		t.Errorf("the modal does not show the title and the input:\n%s", out)
	}

	withoutTask := m
	withoutTask.detailTask = nil
	if out := withoutTask.renderPathModal("base"); !strings.Contains(out, "Task path") {
		t.Errorf("the modal without a task does not render:\n%s", out)
	}
}

func TestPathOverlayIsShownAndListed(t *testing.T) {
	m := newTestModel(t)
	m.detailOpen = true
	m.detailTask = &m.tasks[0]
	m.pathOpen = true
	m.pathInput = "/typed"

	if got := m.overlayKind(); got != overlayPath {
		t.Errorf("overlayKind = %v, want overlayPath", got)
	}
	m.width, m.height = 120, 40
	if out := m.View().Content; !strings.Contains(out, "Task path") {
		t.Errorf("View without the path modal:\n%s", out)
	}

	bar := NewKeybindsBar(120)
	bar.SetOverlay(overlayPath)
	if out := bar.View(); !strings.Contains(out, "Task path") || !strings.Contains(out, "save") {
		t.Errorf("the keybinds bar does not describe the path modal:\n%s", out)
	}
}

func TestDetailShowsTheEffectivePathAndItsSource(t *testing.T) {
	m := newTestModel(t)
	task := model.Task{ID: 7, Title: "T", Status: "todo", ProjectName: "api"}

	if out := m.renderDetail(&task, 30); !strings.Contains(out, "Path:") || !strings.Contains(out, "(default)") {
		t.Errorf("the detail does not show the default path:\n%s", out)
	}

	task.ProjectPath = "/project"
	if out := m.renderDetail(&task, 30); !strings.Contains(out, "/project (project)") {
		t.Errorf("the detail does not show the project path:\n%s", out)
	}

	task.Path = "/task"
	if out := m.renderDetail(&task, 30); !strings.Contains(out, "/task (task)") {
		t.Errorf("the detail does not show the task path:\n%s", out)
	}
}

func TestDetailHelpListsThePathKey(t *testing.T) {
	found := false
	for _, kb := range detailKeybinds() {
		if kb.key == "P" && kb.desc == "task path" {
			found = true
		}
	}
	if !found {
		t.Error("the detail help does not list the P key")
	}
}

func TestPathModalCapturesKeysInTheApp(t *testing.T) {
	m := newTestModel(t)
	m.detailOpen = true
	m.detailTask = &m.tasks[0]
	m.pathOpen = true
	m.pathTaskID = m.tasks[0].ID

	got, _ := press(m, "esc")
	if got.pathOpen {
		t.Error("esc did not close the path modal routed through the app")
	}
}

func TestDetailPathKeyWithoutTaskDoesNothing(t *testing.T) {
	m := newTestModel(t)
	m.detailOpen = true
	m.detailTask = nil

	next, _ := m.handleDetailKey("P")
	got := next.(Model)
	if got.pathOpen {
		t.Error("P without a task opened the path modal")
	}
}

func TestPathSavedWithoutAnOpenDetail(t *testing.T) {
	m := newTestModel(t)
	msg := pathSavedMsg{taskID: 1, task: &model.Task{ID: 1, Path: "/x"}, tasks: m.tasks}

	next, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("a successful save without a detail returned a command")
	}
	got := next.(Model)
	if got.detailTask != nil {
		t.Errorf("a save repopulated a closed detail: %+v", got.detailTask)
	}
}
