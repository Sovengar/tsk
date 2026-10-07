package tui

import (
	"testing"

	"tsk/internal/config"
	"tsk/internal/model"
)

// These functions were extracted from the navigation of List, Kanban and
// Dashboard so they could be checked exhaustively. Before, every call site
// carried its own copy of the count with its own edge, and that was at the
// same time the source of the mutants and the reason there was no way to pin
// them down without building a whole view and a database.

// cycleIndex wraps around at the end: that is what j/k do in a cycle, where
// going down from the last row returns to the first.
func TestCycleIndex(t *testing.T) {
	tests := []struct {
		name      string
		idx, n, d int
		want      int
	}{
		{"down in the middle", 1, 3, 1, 2},
		{"down on the last wraps around", 2, 3, 1, 0},
		{"up on the first wraps around", 0, 3, -1, 2},
		{"up in the middle", 1, 3, -1, 0},
		{"single-item list is always zero", 0, 1, 1, 0},
		{"single-item list up as well", 0, 1, -1, 0},
		{"empty list does not move", 5, 0, 1, 5},
		{"empty list does not move backward", 5, 0, -1, 5},
		{"negative n does not move", 3, -2, 1, 3},
		{"out-of-range index is normalized", 7, 3, 1, 2},
		{"step bigger than the list", 0, 3, 5, 2},
		{"negative step bigger than the list", 0, 3, -5, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cycleIndex(tt.idx, tt.n, tt.d); got != tt.want {
				t.Errorf("cycleIndex(%d, %d, %d) = %d, want %d", tt.idx, tt.n, tt.d, got, tt.want)
			}
		})
	}
}

// The property that makes cycleIndex usable in navigation: for any delta,
// the result is always inside the list.
func TestCycleIndexAlwaysInRange(t *testing.T) {
	for n := 1; n <= 8; n++ {
		for idx := -2; idx <= n+2; idx++ {
			for d := -9; d <= 9; d++ {
				got := cycleIndex(idx, n, d)
				if got < 0 || got >= n {
					t.Fatalf("cycleIndex(%d, %d, %d) = %d, out of [0,%d)", idx, n, d, got, n)
				}
			}
		}
	}
}

// And walking with delta +1 n times returns to the starting point, for any
// initial index.
func TestCycleIndexIsAPermutation(t *testing.T) {
	for n := 1; n <= 8; n++ {
		for start := range n {
			seen := map[int]bool{}
			idx := start
			for range n {
				if seen[idx] {
					t.Fatalf("n=%d: the cycle repeated %d before completing %d turns", n, idx, n)
				}
				seen[idx] = true
				idx = cycleIndex(idx, n, 1)
			}
			if idx != start {
				t.Errorf("n=%d: after %d steps from %d it is back at %d", n, n, start, idx)
			}
		}
	}
}

// shiftIndex stays at the extreme instead of wrapping: that is what the
// arrows do when moving between Kanban columns.
func TestShiftIndex(t *testing.T) {
	tests := []struct {
		name      string
		idx, n, d int
		want      int
	}{
		{"right in the middle", 1, 3, 1, 2},
		{"right on the last stays", 2, 3, 1, 2},
		{"left in the middle", 1, 3, -1, 0},
		{"left on the first stays", 0, 3, -1, 0},
		{"empty list returns zero", 4, 0, 1, 0},
		{"negative list returns zero", 4, -1, -1, 0},
		{"out-of-range index is clamped", 9, 3, 1, 2},
		{"negative index is clamped", -9, 3, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shiftIndex(tt.idx, tt.n, tt.d); got != tt.want {
				t.Errorf("shiftIndex(%d, %d, %d) = %d, want %d", tt.idx, tt.n, tt.d, got, tt.want)
			}
		})
	}
}

func TestShiftIndexAlwaysInRange(t *testing.T) {
	for n := 1; n <= 8; n++ {
		for idx := -3; idx <= n+3; idx++ {
			for d := -5; d <= 5; d++ {
				got := shiftIndex(idx, n, d)
				if got < 0 || got >= n {
					t.Fatalf("shiftIndex(%d, %d, %d) = %d, out of [0,%d)", idx, n, d, got, n)
				}
			}
		}
	}
}

// inRange is the guard before every tasks[idx]: its lower edge matters
// (a -1 would index backwards) and so does the upper one (out of range blows up).
func TestInRange(t *testing.T) {
	tests := []struct {
		name   string
		idx, n int
		want   bool
	}{
		{"inside", 0, 1, true},
		{"last valid", 2, 3, true},
		{"one past", 3, 3, false},
		{"negative", -1, 3, false},
		{"empty list", 0, 0, false},
		{"empty list and negative", -1, 0, false},
		{"negative list", 0, -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inRange(tt.idx, tt.n); got != tt.want {
				t.Errorf("inRange(%d, %d) = %v, want %v", tt.idx, tt.n, got, tt.want)
			}
		})
	}
}

// nextPriority: the cycle depends on the status. In backlog there are four
// steps (it includes none) and out of backlog only three, so the same
// priority advances differently depending on where the task is.
func TestNextPriority(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		priority int
		want     int
	}{
		// backlog: none → low → med → high → none
		{"backlog from none", "backlog", model.PriorityNone, model.PriorityLow},
		{"backlog from low", "backlog", model.PriorityLow, model.PriorityMedium},
		{"backlog from med", "backlog", model.PriorityMedium, model.PriorityHigh},
		{"backlog from high wraps around", "backlog", model.PriorityHigh, model.PriorityNone},

		// out of backlog: low → med → high → low
		{"doing from low", "doing", model.PriorityLow, model.PriorityMedium},
		{"doing from med", "doing", model.PriorityMedium, model.PriorityHigh},
		{"doing from high wraps around", "doing", model.PriorityHigh, model.PriorityLow},
		// none is NOT in the out-of-backlog cycle: it jumps straight to low.
		{"doing from none enters the cycle", "doing", model.PriorityNone, model.PriorityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPriority(tt.status, tt.priority); got != tt.want {
				t.Errorf("nextPriority(%q, %d) = %d, want %d", tt.status, tt.priority, got, tt.want)
			}
		})
	}
}

// The priority is truncated to 0..3 before cycling: it arrives from the
// database and there is no range guarantee. Without the truncation a 7 with
// the three-step cycle would stay at 7 (= 1 = low) and "going up" would not move anything.
func TestNextPriorityClampsOutOfRange(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		priority int
		want     int
	}{
		{"backlog from 7 clamps to high and returns to none", "backlog", 7, model.PriorityNone},
		{"backlog from 99", "backlog", 99, model.PriorityNone},
		{"backlog from -5 clamps to none", "backlog", -5, model.PriorityLow},
		{"doing from 7 clamps to high and returns to low", "doing", 7, model.PriorityLow},
		{"doing from -5 clamps to none", "doing", -5, model.PriorityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPriority(tt.status, tt.priority); got != tt.want {
				t.Errorf("nextPriority(%q, %d) = %d, want %d", tt.status, tt.priority, got, tt.want)
			}
		})
	}
}

// The output always falls in the valid priority range.
func TestNextPriorityAlwaysValid(t *testing.T) {
	statuses := []string{"backlog", "todo", "doing", "reviewing", "done"}
	for _, status := range statuses {
		for p := -3; p <= 9; p++ {
			got := nextPriority(status, p)
			if got < model.PriorityNone || got > model.PriorityHigh {
				t.Errorf("nextPriority(%q, %d) = %d, outside the 0-3 range", status, p, got)
			}
		}
	}
}

// Cycling many times in backlog walks the four states and returns to the
// starting point; out of backlog it walks the three.
func TestNextPriorityCycles(t *testing.T) {
	t.Run("backlog walks 4 states", func(t *testing.T) {
		p := model.PriorityNone
		for range 4 {
			p = nextPriority("backlog", p)
		}
		if p != model.PriorityNone {
			t.Errorf("4 steps in backlog = %d, want %d (back to the start)", p, model.PriorityNone)
		}
	})

	t.Run("outside backlog walks 3 states", func(t *testing.T) {
		p := model.PriorityLow
		for range 3 {
			p = nextPriority("doing", p)
		}
		if p != model.PriorityLow {
			t.Errorf("3 steps outside backlog = %d, want %d (back to the start)", p, model.PriorityLow)
		}
	})
}

// taskAt is the guard that was written in front of every tasks[m.cursor].
// Receiving the index as a parameter is what allows killing its BOUNDARY
// mutant: idx == len(tasks) never happens from the keyboard because the
// cursor arrives bounded, but here that exact case can be asked for.
func TestTaskAt(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Title: "first"},
		{ID: 2, Title: "second"},
		{ID: 3, Title: "third"},
	}
	tests := []struct {
		name   string
		tasks  []model.Task
		idx    int
		wantID int64
		wantOK bool
	}{
		{"first", tasks, 0, 1, true},
		{"the middle one", tasks, 1, 2, true},
		{"last", tasks, 2, 3, true},
		{"one past", tasks, 3, 0, false},
		{"way past", tasks, 99, 0, false},
		{"negative", tasks, -1, 0, false},
		{"empty list", nil, 0, 0, false},
		{"empty list with negative index", nil, -5, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskAt(tt.tasks, tt.idx)
			if !tt.wantOK {
				if got != nil {
					t.Fatalf("taskAt(%d) = %+v, want nil", tt.idx, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("taskAt(%d) = nil, want task %d", tt.idx, tt.wantID)
			}
			if got.ID != tt.wantID {
				t.Errorf("taskAt(%d).ID = %d, want %d", tt.idx, got.ID, tt.wantID)
			}
		})
	}
}

// With any list and any index, taskAt returns something or nil, it never
// panics: that is what makes it safe to use in front of every tasks[idx].
func TestTaskAtNeverPanics(t *testing.T) {
	sizes := []int{0, 1, 2, 5}
	for _, n := range sizes {
		tasks := make([]model.Task, n)
		for i := range tasks {
			tasks[i] = model.Task{ID: int64(i + 1)}
		}
		for idx := -3; idx <= n+3; idx++ {
			got := taskAt(tasks, idx)
			if inRange(idx, n) {
				if got == nil || got.ID != int64(idx+1) {
					t.Fatalf("taskAt(n=%d, %d) = %+v, want task %d", n, idx, got, idx+1)
				}
			} else if got != nil {
				t.Fatalf("taskAt(n=%d, %d) = %+v, want nil", n, idx, got)
			}
		}
	}
}

// previewBudgetFor: what fits between the minimum of 1 line and the cap,
// without letting the preview push the content off the screen.
func TestPreviewBudgetFor(t *testing.T) {
	const keybinds = 2
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{"normal terminal hits the cap", 60, previewMaxLines},
		{"exactly at the cap", previewMaxLines + keybinds + minContentHeight + 2, previewMaxLines},
		{"a bit under the cap", previewMaxLines + keybinds + minContentHeight + 1, previewMaxLines - 1},
		{"small terminal gives less", 20, 20 - keybinds - minContentHeight - 2},
		{"minimum terminal gives 1", 10, 1},
		{"zero height gives 1", 0, 1},
		{"negative height gives 1", -10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := previewBudgetFor(tt.height, keybinds)
			if got != tt.want {
				t.Errorf("previewBudgetFor(%d, %d) = %d, want %d", tt.height, keybinds, got, tt.want)
			}
			if got < 1 || got > previewMaxLines {
				t.Errorf("previewBudgetFor(%d, %d) = %d, out of [1,%d]", tt.height, keybinds, got, previewMaxLines)
			}
		})
	}
}

// More keybinds leave less budget, never more.
func TestPreviewBudgetShrinksWithKeybinds(t *testing.T) {
	for height := 12; height <= 60; height++ {
		prev := previewBudgetFor(height, 0)
		for k := 1; k <= 8; k++ {
			got := previewBudgetFor(height, k)
			if got > prev {
				t.Errorf("height=%d: more keybinds (%d) gave more budget: %d > %d", height, k, got, prev)
			}
			prev = got
		}
	}
}

// The budget never leaves the bounded range, for any input.
func TestPreviewBudgetAlwaysBounded(t *testing.T) {
	for height := -5; height <= 80; height++ {
		for keybinds := -2; keybinds <= 20; keybinds++ {
			got := previewBudgetFor(height, keybinds)
			if got < 1 || got > previewMaxLines {
				t.Fatalf("previewBudgetFor(%d, %d) = %d, out of [1,%d]", height, keybinds, got, previewMaxLines)
			}
		}
	}
}

// matchesStatus has three modes: active, all, or an exact status.
func TestMatchesStatus(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		status string
		active bool
		want   bool
	}{
		{"all-active accepts an active one", statusFilterAllActive, "doing", true, true},
		{"all-active rejects an inactive one", statusFilterAllActive, "done", false, false},
		{"all does not restrict", "", "done", false, true},
		{"all does not restrict with active", "", "todo", true, true},
		{"exact status matches", "doing", "doing", false, true},
		{"exact status does not match", "doing", "todo", false, false},
		{"exact status with active does not filter either", "todo", "todo", true, true},
		{"an unknown status only matches itself", "new", "new", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesStatus(tt.filter, tt.status, tt.active); got != tt.want {
				t.Errorf("matchesStatus(%q, %q, %v) = %v, want %v", tt.filter, tt.status, tt.active, got, tt.want)
			}
		})
	}
}

// The "all active" status filter does NOT look at the status field, only the
// active flag: that is what tells that mode apart from exact match.
func TestMatchesStatusAllActiveIgnoresStatusName(t *testing.T) {
	for _, status := range []string{"todo", "doing", "done", "cancelled", "whatever"} {
		if !matchesStatus(statusFilterAllActive, status, true) {
			t.Errorf("with active=true the status %q should pass", status)
		}
		if matchesStatus(statusFilterAllActive, status, false) {
			t.Errorf("with active=false the status %q should not pass", status)
		}
	}
}

// matchesPriority: -1 is "any", any other value demands a match.
func TestMatchesPriority(t *testing.T) {
	tests := []struct {
		name           string
		filter, actual int
		want           bool
	}{
		{"no filter accepts anything", -1, 0, true},
		{"no filter accepts the highest", -1, 3, true},
		{"none filter matches none", 0, 0, true},
		{"none filter does not match high", 0, 3, false},
		{"high filter matches high", 3, 3, true},
		{"high filter does not match none", 3, 0, false},
		// Any negative disables the filter, not only -1: that is what the
		// original `if m.filterPriority >= 0 && ...` did and it is kept.
		{"another negative also disables the filter", -2, 1, true},
		{"another negative with the same priority", -2, -2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPriority(tt.filter, tt.actual); got != tt.want {
				t.Errorf("matchesPriority(%d, %d) = %v, want %v", tt.filter, tt.actual, got, tt.want)
			}
		})
	}
}

// clampTo is the edge that four different clamps of the program repeated.
// With an empty list it returns 0, which is what all of them showed.
func TestClampTo(t *testing.T) {
	tests := []struct {
		name   string
		idx, n int
		want   int
	}{
		{"inside", 2, 5, 2},
		{"last valid", 4, 5, 4},
		{"one past", 5, 5, 4},
		{"way past", 99, 5, 4},
		{"negative", -3, 5, 0},
		{"single-item list", 7, 1, 0},
		{"empty list", 4, 0, 0},
		{"empty list with negative", -4, 0, 0},
		{"negative n", 4, -2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampTo(tt.idx, tt.n); got != tt.want {
				t.Errorf("clampTo(%d, %d) = %d, want %d", tt.idx, tt.n, got, tt.want)
			}
		})
	}
}

func TestClampToAlwaysInRange(t *testing.T) {
	for n := 0; n <= 8; n++ {
		for idx := -5; idx <= 13; idx++ {
			got := clampTo(idx, n)
			if n <= 0 {
				if got != 0 {
					t.Fatalf("clampTo(%d, %d) = %d, want 0 with no list", idx, n, got)
				}
				continue
			}
			if got < 0 || got >= n {
				t.Fatalf("clampTo(%d, %d) = %d, out of [0,%d)", idx, n, got, n)
			}
		}
	}
}

// firstValidIndex is the "if the index does not apply, the first one" without looking at elements.
func TestFirstValidIndex(t *testing.T) {
	tests := []struct {
		name   string
		idx, n int
		want   int
	}{
		{"inside", 2, 5, 2},
		{"negative", -1, 5, 0},
		{"out at the top", 9, 5, 4},
		{"single-item list", 3, 1, 0},
		{"empty list", 3, 0, 0},
		{"negative list", 3, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstValidIndex(tt.idx, tt.n); got != tt.want {
				t.Errorf("firstValidIndex(%d, %d) = %d, want %d", tt.idx, tt.n, got, tt.want)
			}
		})
	}
}

// firstOrAt returns the first element when the index does not apply. That is
// what tells this function apart from taskAt2, which returns "".
func TestFirstOrAt(t *testing.T) {
	items := []string{"a", "b", "c"}
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{"first", 0, "a"},
		{"the middle one", 1, "b"},
		{"last", 2, "c"},
		{"negative falls on the first", -1, "a"},
		{"out at the top falls on the last", 9, "c"},
		{"one past falls on the last", 3, "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstOrAt(items, tt.idx); got != tt.want {
				t.Errorf("firstOrAt(%d) = %q, want %q", tt.idx, got, tt.want)
			}
		})
	}
}

// And on an empty list it returns "", like taskAt2: there is no first element.
func TestFirstOrAtEmpty(t *testing.T) {
	for _, idx := range []int{-1, 0, 5} {
		if got := firstOrAt(nil, idx); got != "" {
			t.Errorf("firstOrAt(nil, %d) = %q, want empty", idx, got)
		}
	}
}

// taskAt2 returns "" out of range, unlike firstOrAt.
func TestTaskAt2OutOfRange(t *testing.T) {
	items := []string{"a", "b", "c"}
	for _, idx := range []int{-1, 3, 99} {
		if got := taskAt2(items, idx); got != "" {
			t.Errorf("taskAt2(%d) = %q, want empty out of range", idx, got)
		}
	}
	if got := taskAt2(items, 1); got != "b" {
		t.Errorf("taskAt2(1) = %q, want b", got)
	}
	if got := taskAt2(nil, 0); got != "" {
		t.Errorf("taskAt2(nil, 0) = %q, want empty", got)
	}
}

// The detail's selection does not wrap upwards: -1 means "nothing
// selected", and from there "j" goes to the first comment, not the second.
func TestNextCommentSel(t *testing.T) {
	tests := []struct {
		name   string
		sel, n int
		want   int
	}{
		{"nothing selected goes to the first", -1, 3, 0},
		{"from the first to the second", 0, 3, 1},
		{"from the second to the third", 1, 3, 2},
		{"on the last it stays", 2, 3, 2},
		{"past the last it stays", 9, 3, 2},
		{"a single comment", -1, 1, 0},
		{"a single comment already on it", 0, 1, 0},
		{"no comments", -1, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextCommentSel(tt.sel, tt.n); got != tt.want {
				t.Errorf("nextCommentSel(%d, %d) = %d, want %d", tt.sel, tt.n, got, tt.want)
			}
		})
	}
}

// It never leaves [0, n-1] for n >= 1, and it is monotonic until it saturates at the last.
func TestNextCommentSelStaysInRange(t *testing.T) {
	for n := 1; n <= 6; n++ {
		prev := -1
		for sel := -1; sel <= 8; sel++ {
			got := nextCommentSel(sel, n)
			if got < 0 || got > n-1 {
				t.Fatalf("nextCommentSel(%d, %d) = %d, out of [0,%d]", sel, n, got, n-1)
			}
			if got < prev {
				t.Fatalf("nextCommentSel(%d, %d) = %d went backwards from %d", sel, n, got, prev)
			}
			prev = got
		}
	}
}

// Backwards, -1 is a real bound: from the first comment it goes back to
// "nothing selected" and from there it does not go out to -2.
func TestPrevCommentSel(t *testing.T) {
	tests := []struct {
		name string
		sel  int
		want int
	}{
		{"from the last", 3, 2},
		{"from the second", 1, 0},
		{"from the first back to nothing", 0, -1},
		{"from nothing it stays", -1, -1},
		{"very negative", -7, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prevCommentSel(tt.sel); got != tt.want {
				t.Errorf("prevCommentSel(%d) = %d, want %d", tt.sel, got, tt.want)
			}
		})
	}
}

func TestPrevCommentSelNeverBelowMinusOne(t *testing.T) {
	for sel := -20; sel <= 20; sel++ {
		if got := prevCommentSel(sel); got < -1 {
			t.Fatalf("prevCommentSel(%d) = %d, want >= -1", sel, got)
		}
	}
}

// A pageSize of 0 or negative is not "a page": it is missing config, and
// it falls back to the default like the rest of the config.
func TestResolvePageSize(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero", 0, config.DefaultPageSize},
		{"negative", -1, config.DefaultPageSize},
		{"very negative", -100, config.DefaultPageSize},
		{"one", 1, 1},
		{"ten", 10, 10},
		{"very large", 100000, 100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePageSize(tt.in); got != tt.want {
				t.Errorf("resolvePageSize(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// The default editor is the three paths of "c" (comment), "E" (full
// editing) and the description's. They share a function so they do not diverge.
func TestEditorCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"unset", "", "nvim"},
		{"vim", "vim", "vim"},
		{"with arguments", "code --wait", "code --wait"},
		{"a space is not empty", " ", " "},
		{"dash", "-", "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editorCommand(tt.in); got != tt.want {
				t.Errorf("editorCommand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The Gantt split has two regimes and two floors. As a pure function it is
// checked over a sweep of widths, which is what the box padded with spaces
// did not let you see.
func TestGanttLabelAndDays(t *testing.T) {
	tests := []struct {
		name        string
		innerW      int
		wantLabel   int
		wantDayCols int
	}{
		{"roomy", 118, 30, 87},
		{"exactly at the threshold", 70, 30, 39},
		{"one less than the threshold", 69, 23, 45},
		{"an exact third", 60, 20, 39},
		{"with label floor", 45, 15, 29},
		{"label at the minimum", 43, 14, 28},
		{"label floor from the one-third rule", 30, 14, 15},
		{"days at the minimum", 20, 14, 7},
		{"below the days floor", 18, 14, 7},
		{"zero", 0, 14, 7},
		{"negative", -50, 14, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, days := ganttLabelAndDays(tt.innerW)
			if label != tt.wantLabel || days != tt.wantDayCols {
				t.Errorf("ganttLabelAndDays(%d) = (%d, %d), want (%d, %d)",
					tt.innerW, label, days, tt.wantLabel, tt.wantDayCols)
			}
		})
	}
}

// The two halves plus the separator fill the inner width while there is
// room for the day columns; when there is not, the label rules and the days
// stay at their minimum.
func TestGanttLabelAndDaysProperties(t *testing.T) {
	for innerW := -20; innerW <= 300; innerW++ {
		label, days := ganttLabelAndDays(innerW)
		if label < ganttMinLabelWidth {
			t.Fatalf("innerW=%d: label %d, want >= %d", innerW, label, ganttMinLabelWidth)
		}
		if days < ganttMinDayCols {
			t.Fatalf("innerW=%d: days %d, want >= %d", innerW, days, ganttMinDayCols)
		}
		if label+1+days > innerW {
			// It can only happen when the minimums do not fit, which is what
			// makes the floor: we prefer overflowing to being left without a visible day.
			if label != ganttMinLabelWidth || days != ganttMinDayCols {
				t.Fatalf("innerW=%d: (%d + 1 + %d) overflows and is not at the minimums (%d, %d)",
					innerW, label, days, label, days)
			}
		}
	}
}

// The regime changes right at the threshold: 70 inner columns keep the
// label fixed, 69 divide it. That pair is what tells the ">=" from the "<".
func TestGanttLabelAndDaysThresholdIsExact(t *testing.T) {
	if l, _ := ganttLabelAndDays(ganttLabelThreshold); l != ganttFixedLabelWidth {
		t.Errorf("at the threshold the label is %d, want %d", l, ganttFixedLabelWidth)
	}
	if l, _ := ganttLabelAndDays(ganttLabelThreshold - 1); l == ganttFixedLabelWidth {
		t.Errorf("one column below the threshold the label is still %d, want the third",
			ganttFixedLabelWidth)
	}
}
