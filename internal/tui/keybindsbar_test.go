package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestKeybindsBarNormalViewShowsCommonAndViewKeys(t *testing.T) {
	var kb KeybindsBar
	// Wide enough so no keybind row splits when wrapping.
	kb.SetWidth(200)
	kb.SetView(viewList)

	out := ansi.Strip(kb.View())
	// Common keys, now part of the view's list.
	if !strings.Contains(out, "quit") {
		t.Errorf("the normal view must show the common keys:\n%s", out)
	}
	if !strings.Contains(out, "insert task") {
		t.Errorf("the List view must show its keybinds:\n%s", out)
	}
	// List uses Tab to cycle projects.
	if !strings.Contains(out, "cycle project") {
		t.Errorf("List must show Tab as cycle projects:\n%s", out)
	}
	if !strings.Contains(out, "Keybinds") {
		t.Errorf("the pane's title is missing:\n%s", out)
	}
}

// TestKeybindsForViewCommonFirst verifies that the formerly global keys
// head the list (they are the most repeated ones) in every view.
func TestKeybindsForViewCommonFirst(t *testing.T) {
	for _, v := range []viewKind{viewDashboard, viewList, viewKanban, viewGantt} {
		want := []string{"1/2/3/4", "hjkl", "?", "q"}
		kbs := keybindsForView(v)
		for i, key := range want {
			if i >= len(kbs) || kbs[i].key != key {
				t.Fatalf("view %s: keybind[%d].key = %q, want %q", v, i, kbs[i].key, key)
			}
		}
	}
}

// TestKeybindsBarMaxSevenPerRow verifies that no row exceeds 7 actions.
func TestKeybindsBarMaxSevenPerRow(t *testing.T) {
	for _, v := range []viewKind{viewDashboard, viewList, viewKanban, viewGantt} {
		var kb KeybindsBar
		kb.SetWidth(200)
		kb.SetView(v)

		out := ansi.Strip(kb.View())
		for _, line := range strings.Split(out, "\n") {
			// 7 actions => 6 "·" separators.
			if n := strings.Count(line, "·"); n > keybindsPerRow-1 {
				t.Errorf("view %s: row with more than %d actions:\n%s", v, keybindsPerRow, line)
			}
		}
	}
}

// TestKeybindsBarDetailAllActions verifies that the detail shows all of its
// actions spread over rows of at most keybindsPerRow.
func TestKeybindsBarDetailAllActions(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(200)
	kb.SetView(viewList)
	kb.SetOverlay(overlayDetail)

	out := ansi.Strip(kb.View())
	for _, line := range strings.Split(out, "\n") {
		if n := strings.Count(line, "·"); n > keybindsPerRow-1 {
			t.Errorf("detail row with more than %d actions:\n%s", keybindsPerRow, line)
		}
	}
	for _, want := range []string{"j/k", "select comment", "new comment", "tags", "delete/done", "edit/editor", "start", "cancel", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is missing in the detail:\n%s", want, out)
		}
	}
}

func TestKeybindsBarDetailOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetView(viewList)
	kb.SetOverlay(overlayDetail)

	out := ansi.Strip(kb.View())
	// Only detail modal keys.
	for _, want := range []string{"comment", "select", "delete/done", "edit", "start", "cancel", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q is missing in the detail overlay:\n%s", want, out)
		}
	}
	// The common and view ones no longer apply.
	for _, unwanted := range []string{"quit", "insert task", "filter"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("%q should not be shown in the detail:\n%s", unwanted, out)
		}
	}
	if !strings.Contains(out, "Keybinds · Detail") {
		t.Errorf("the title must indicate the Detail context:\n%s", out)
	}
}

func TestKeybindsBarNewTaskOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetOverlay(overlayNewTask)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "create") || strings.Contains(out, "quit") {
		t.Errorf("wrong new task overlay:\n%s", out)
	}
}

func TestKeybindsBarFilterOverlay(t *testing.T) {
	var kb KeybindsBar
	kb.SetWidth(120)
	kb.SetOverlay(overlayFilter)

	out := ansi.Strip(kb.View())
	if !strings.Contains(out, "cycle") || strings.Contains(out, "quit") {
		t.Errorf("wrong filter overlay:\n%s", out)
	}
}

func TestOverlayKindPriority(t *testing.T) {
	m := newTestModel(t)

	if m.overlayKind() != overlayNone {
		t.Errorf("with no modals: %v, want overlayNone", m.overlayKind())
	}

	m.detailOpen = true
	if m.overlayKind() != overlayDetail {
		t.Errorf("with detail: %v, want overlayDetail", m.overlayKind())
	}

	// The new task takes priority over the detail (same as handleKey).
	m.newTaskOpen = true
	m.filterOpen = true
	if m.overlayKind() != overlayNewTask {
		t.Errorf("with new task: %v, want overlayNewTask", m.overlayKind())
	}

	m.newTaskOpen = false
	if m.overlayKind() != overlayDetail {
		t.Errorf("detail before filters: %v, want overlayDetail", m.overlayKind())
	}

	m.detailOpen = false
	if m.overlayKind() != overlayFilter {
		t.Errorf("with filters: %v, want overlayFilter", m.overlayKind())
	}
}

// TestDetailOpenSwitchesKeybinds verifies that opening the detail changes the
// context of the keybinds bar.
func TestDetailOpenSwitchesKeybinds(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	if m.overlayKind() != overlayNone {
		t.Fatalf("before opening: %v, want overlayNone", m.overlayKind())
	}

	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Fatal("enter must open the detail")
	}
	if m.overlayKind() != overlayDetail {
		t.Errorf("with the detail open: %v, want overlayDetail", m.overlayKind())
	}
}
