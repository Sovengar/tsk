package model

import "testing"

func TestParseWorkflow(t *testing.T) {
	wf, err := ParseWorkflow("a,b,c")
	if err != nil {
		t.Fatal(err)
	}
	if len(wf) != 3 || wf[0] != "a" || wf[2] != "c" {
		t.Errorf("result = %v", wf)
	}
}

func TestParseWorkflowEmpty(t *testing.T) {
	_, err := ParseWorkflow("")
	if err != nil {
		t.Errorf("empty string should not error, got %v", err)
	}
}

func TestParseWorkflowWithSpaces(t *testing.T) {
	wf, _ := ParseWorkflow(" a , b , c ")
	if len(wf) != 3 || wf[0] != "a" || wf[1] != "b" || wf[2] != "c" {
		t.Errorf("result = %v", wf)
	}
}

func TestHasStatus(t *testing.T) {
	wf := []string{"backlog", "todo", "done"}
	if !HasStatus(wf, "todo") {
		t.Error("HasStatus(todo) = false, want true")
	}
	if HasStatus(wf, "missing") {
		t.Error("HasStatus(missing) = true, want false")
	}
}

func TestFindStatusContaining(t *testing.T) {
	wf := []string{"backlog", "todo", "in_progress", "review", "done"}
	s, ok := FindStatusContaining(wf, "review")
	if !ok || s != "review" {
		t.Errorf("FindStatusContaining(review) = %q, %v", s, ok)
	}
	s, ok = FindStatusContaining(wf, "prog")
	if !ok || s != "in_progress" {
		t.Errorf("FindStatusContaining(prog) = %q, %v", s, ok)
	}
	_, ok = FindStatusContaining(wf, "xyz")
	if ok {
		t.Error("expected not found")
	}
}

func TestTerminalStatus(t *testing.T) {
	wf := []string{"backlog", "todo", "done"}
	if got := TerminalStatus(wf); got != "done" {
		t.Errorf("TerminalStatus = %q, want done", got)
	}
}

func TestTerminalStatusEmpty(t *testing.T) {
	if got := TerminalStatus(nil); got != "done" {
		t.Errorf("TerminalStatus(empty) = %q, want done", got)
	}
}

func TestStartStatus(t *testing.T) {
	wf := []string{"backlog", "todo", "in_progress", "done"}
	if got := StartStatus(wf); got != "todo" {
		t.Errorf("StartStatus = %q, want todo", got)
	}

	// Sin backlog
	wf2 := []string{"todo", "done"}
	if got := StartStatus(wf2); got != "todo" {
		t.Errorf("StartStatus (no backlog) = %q, want todo", got)
	}

	// Solo un elemento
	wf3 := []string{"done"}
	if got := StartStatus(wf3); got != "done" {
		t.Errorf("StartStatus (single) = %q, want done", got)
	}
}

func TestNextStatus(t *testing.T) {
	wf := []string{"backlog", "todo", "in_progress", "done"}
	next, ok := NextStatus(wf, "todo")
	if !ok || next != "in_progress" {
		t.Errorf("NextStatus(todo) = %q, %v", next, ok)
	}
	_, ok = NextStatus(wf, "done")
	if ok {
		t.Error("NextStatus(done) should return false")
	}
}

func TestPrevStatus(t *testing.T) {
	wf := []string{"backlog", "todo", "in_progress", "done"}
	prev, ok := PrevStatus(wf, "in_progress")
	if !ok || prev != "todo" {
		t.Errorf("PrevStatus(in_progress) = %q, %v", prev, ok)
	}
	_, ok = PrevStatus(wf, "backlog")
	if ok {
		t.Error("PrevStatus(backlog) should return false")
	}
}

func TestValidateWorkflow(t *testing.T) {
	if err := ValidateWorkflow([]string{"a", "b", "c"}); err != nil {
		t.Errorf("valid workflow errored: %v", err)
	}
	if err := ValidateWorkflow([]string{"a", "a"}); err == nil {
		t.Error("duplicate workflow should error")
	}
}

func TestWorkflowJSONRoundtrip(t *testing.T) {
	wf := []string{"backlog", "todo", "done"}
	json := WorkflowJSON(wf)
	got, err := ParseWorkflowJSON(json)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "backlog" || got[2] != "done" {
		t.Errorf("roundtrip = %v", got)
	}
}

func TestPriorityHelpers(t *testing.T) {
	if l := PriorityLabel(PriorityHigh); l != "HIGH" {
		t.Errorf("PriorityLabel(HIGH) = %q", l)
	}
	if l := PriorityLabel(99); l != "none" {
		t.Errorf("PriorityLabel(99) = %q", l)
	}
	if b := PriorityBar(PriorityHigh); b != "●" {
		t.Errorf("PriorityBar(HIGH) = %q", b)
	}
}

func TestTaskIsActive(t *testing.T) {
	t1 := &Task{Status: "in_progress"}
	if !t1.IsActive() {
		t.Error("in_progress should be active")
	}
	t2 := &Task{Status: "done"}
	if t2.IsActive() {
		t.Error("done should not be active")
	}
	t3 := &Task{Status: "cancelled"}
	if t3.IsActive() {
		t.Error("cancelled should not be active")
	}
}
