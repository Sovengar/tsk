package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// taskByTitle looks up a test task by title.
func taskByTitle(t *testing.T, m *Model, title string) *model.Task {
	t.Helper()
	for i := range m.tasks {
		if m.tasks[i].Title == title {
			return &m.tasks[i]
		}
	}
	t.Fatalf("task %q not found", title)
	return nil
}

// openDetail opens the detail modal for the given task.
func openDetail(m *Model, task *model.Task) {
	cp := *task
	m.detailOpen = true
	m.detailTask = &cp
	comments, _ := m.database.ListComments(task.ID)
	m.detailComments = comments
	m.detailCommentSel = -1
}

// applyMsg applies a message to the model, runs the resulting command and
// also applies its message (one level), so that async flows get
// resolved in tests.
func applyMsg(t *testing.T, m *Model, msg tea.Msg) *Model {
	next, cmd := m.Update(msg)
	m = asModel(next)
	if cmd != nil {
		if out := mustMsg(t, cmd); out != nil {
			next2, _ := m.Update(out)
			m = asModel(next2)
		}
	}
	return m
}

func asModel(v tea.Model) *Model {
	switch t := v.(type) {
	case *Model:
		return t
	case Model:
		return &t
	default:
		return nil
	}
}

func TestDetailCommentSelection(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "one")
	mustAddComment(t, m.database, task.ID, "two")
	openDetail(m, task)

	if m.detailCommentSel != -1 {
		t.Fatalf("initial sel = %d, want -1", m.detailCommentSel)
	}

	// j enters the list and selects the first one
	m, _ = press(m, "j")
	if m.detailCommentSel != 0 {
		t.Errorf("after j: sel = %d, want 0", m.detailCommentSel)
	}
	m, _ = press(m, "j")
	if m.detailCommentSel != 1 {
		t.Errorf("after 2x j: sel = %d, want 1", m.detailCommentSel)
	}
	// j on the last one does not go past it
	m, _ = press(m, "j")
	if m.detailCommentSel != 1 {
		t.Errorf("j at bottom: sel = %d, want 1", m.detailCommentSel)
	}
	// k goes up
	m, _ = press(m, "k")
	if m.detailCommentSel != 0 {
		t.Errorf("after k: sel = %d, want 0", m.detailCommentSel)
	}
	// k on the first one deselects
	m, _ = press(m, "k")
	if m.detailCommentSel != -1 {
		t.Errorf("k at top: sel = %d, want -1", m.detailCommentSel)
	}
}

func TestDetailDeleteComment(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "one")
	mustAddComment(t, m.database, task.ID, "two")
	openDetail(m, task)

	// Selects the first one and deletes it with d
	m, _ = press(m, "j")
	m, cmd := press(m, "d")
	if !m.detailOpen {
		t.Error("the detail must stay open when deleting a comment")
	}
	if cmd == nil {
		t.Fatal("d with a comment selected must produce a command")
	}

	m = applyMsg(t, m, mustMsg(t, cmd))
	if len(m.detailComments) != 1 {
		t.Fatalf("comments = %d, want 1", len(m.detailComments))
	}
	if m.detailComments[0].Body != "two" {
		t.Errorf("left %q, want two", m.detailComments[0].Body)
	}
	if m.detailCommentSel != 0 {
		t.Errorf("sel after deleting = %d, want 0", m.detailCommentSel)
	}

	stored, _ := m.database.ListComments(task.ID)
	if len(stored) != 1 {
		t.Errorf("comments in DB = %d, want 1", len(stored))
	}
}

func TestDetailDeleteLastCommentClearsSelection(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "only")
	openDetail(m, task)

	m, _ = press(m, "j")
	m, cmd := press(m, "d")
	m = applyMsg(t, m, mustMsg(t, cmd))

	if len(m.detailComments) != 0 {
		t.Fatalf("comments = %d, want 0", len(m.detailComments))
	}
	if m.detailCommentSel != -1 {
		t.Errorf("sel = %d, want -1", m.detailCommentSel)
	}
}

func TestDetailDoneWithoutCommentSelection(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "note")
	openDetail(m, task)

	// With no selection, d marks done and closes the modal.
	m, cmd := press(m, "d")
	if m.detailOpen {
		t.Error("d with no selection must close the detail")
	}
	if cmd == nil {
		t.Fatal("d with no selection must produce the Done command")
	}
	applyMsg(t, m, mustMsg(t, cmd))

	stored, _ := m.database.GetTask(task.ID)
	if stored.Status != "done" {
		t.Errorf("status = %q, want done", stored.Status)
	}
}

func TestDetailEscDeselectsThenCloses(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "note")
	openDetail(m, task)

	m, _ = press(m, "j")
	if m.detailCommentSel != 0 {
		t.Fatalf("sel = %d, want 0", m.detailCommentSel)
	}

	// First Esc deselects
	m, _ = press(m, "esc")
	if m.detailCommentSel != -1 {
		t.Errorf("esc 1: sel = %d, want -1", m.detailCommentSel)
	}
	if !m.detailOpen {
		t.Error("esc 1 must not close the modal")
	}

	// Second Esc closes
	m, _ = press(m, "esc")
	if m.detailOpen {
		t.Error("esc 2 must close the modal")
	}
}

func TestRenderDetailCommentWindowRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	for i := 0; i < 20; i++ {
		mustAddComment(t, m.database, task.ID, "comment "+string(rune('A'+i)))
	}
	openDetail(m, task)
	m.detailCommentSel = 15

	const budget = 20
	out := m.renderDetail(task, budget)

	if n := lineCount(out); n > budget {
		t.Errorf("height = %d, exceeds the budget %d", n, budget)
	}
	// The window must follow the selection.
	if !strings.Contains(ansi.Strip(out), "comment P") {
		t.Errorf("the selected comment (#15) is not visible:\n%s", ansi.Strip(out))
	}
}

func TestRenderDetailNoComments(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Add caching")
	openDetail(m, task)

	out := ansi.Strip(m.renderDetail(task, 24))
	if !strings.Contains(out, "Comments (0)") {
		t.Errorf("the comments box title is missing:\n%s", out)
	}
	if !strings.Contains(out, "(no comments)") {
		t.Errorf("the comments placeholder is missing:\n%s", out)
	}
}

// TestRenderDetailTwoBoxes verifies that the detail draws two boxes with a
// rounded border: task+description and comments.
func TestRenderDetailTwoBoxes(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	openDetail(m, task)

	out := ansi.Strip(m.renderDetail(task, 24))
	if n := strings.Count(out, "╭"); n != 2 {
		t.Errorf("top borders = %d, want 2:\n%s", n, out)
	}
	if n := strings.Count(out, "╰"); n != 2 {
		t.Errorf("bottom borders = %d, want 2:\n%s", n, out)
	}
	if !strings.Contains(out, "Description:") {
		t.Errorf("the task box must contain the description:\n%s", out)
	}
}

// TestRenderDetailNoActionsLine verifies that the detail no longer repeats the
// keybinds on an actions line (they live in the KeybindsBar).
func TestRenderDetailNoActionsLine(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	openDetail(m, task)

	out := ansi.Strip(m.renderDetail(task, 24))
	if strings.Contains(out, "New comment") {
		t.Errorf("the detail must not repeat the keybinds:\n%s", out)
	}
}

// TestRenderDetailIndentsDescription verifies that the description ends up
// indented the same as the metadata.
func TestRenderDetailIndentsDescription(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	taskCopy := *task
	taskCopy.Description = "I don't understand"
	openDetail(m, &taskCopy)

	out := ansi.Strip(m.renderDetail(&taskCopy, 24))
	if !strings.Contains(out, "  I don't understand") {
		t.Errorf("the description must be indented by 2 spaces:\n%s", out)
	}
}

// TestDetailHidesPreview verifies that when the detail opens the preview box
// disappears (the description is now shown inside the modal).
func TestDetailHidesPreview(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	withPreview := ansi.Strip(m.View().Content)
	if !strings.Contains(withPreview, " Description ") {
		t.Fatalf("without the detail the preview should be visible:\n%s", withPreview)
	}

	m, _ = press(m, "enter")
	withDetail := ansi.Strip(m.View().Content)
	if strings.Contains(withDetail, " Description ") {
		t.Errorf("with the detail open the preview must not be visible:\n%s", withDetail)
	}
	if !strings.Contains(withDetail, "Description:") {
		t.Errorf("the detail must show its own description:\n%s", withDetail)
	}
}

// TestDetailViewHasThreeBoxes verifies that the detail view shows three boxes
// with a rounded border: task+description, comments and keybinds.
func TestDetailViewHasThreeBoxes(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m, _ = press(m, "enter")

	out := ansi.Strip(m.View().Content)
	if n := strings.Count(out, "╭"); n != 3 {
		t.Errorf("bordered boxes = %d, want 3 (task, comments, keybinds):\n%s", n, out)
	}
	if !strings.Contains(out, "Keybinds · Detail") {
		t.Errorf("the detail keybinds box is missing:\n%s", out)
	}
}

func TestCommentCmdEmptyBodyAddsNothing(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	openDetail(m, task)

	// An empty comment must not create anything.
	m = applyMsg(t, m, commentFinishedMsg{taskID: task.ID, body: ""})
	comments, _ := m.database.ListComments(task.ID)
	if len(comments) != 0 {
		t.Errorf("comments = %d, want 0", len(comments))
	}
}

func TestCommentAddedSelectsNewest(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "old")
	openDetail(m, task)

	m = applyMsg(t, m, commentFinishedMsg{taskID: task.ID, body: "new"})
	if len(m.detailComments) != 2 {
		t.Fatalf("comments = %d, want 2", len(m.detailComments))
	}
	if m.detailCommentSel != 1 {
		t.Errorf("sel = %d, want 1 (the new one)", m.detailCommentSel)
	}
	if m.detailComments[1].Body != "new" {
		t.Errorf("last = %q, want new", m.detailComments[1].Body)
	}
}
