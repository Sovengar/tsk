package model

import "testing"

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Blocked ", "bug", "BUG", "", "  ", "urgent"})
	want := []string{"blocked", "bug", "urgent"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseTags(t *testing.T) {
	if got := ParseTags(""); got != nil {
		t.Errorf("ParseTags(\"\") = %v, want nil", got)
	}
	got := ParseTags(" Blocked, bug ,,urgent ")
	want := []string{"blocked", "bug", "urgent"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHasTag(t *testing.T) {
	tags := []string{"blocked", "bug"}
	if !HasTag(tags, "BLOCKED") {
		t.Error("HasTag debería ser case-insensitive")
	}
	if HasTag(tags, "urgent") {
		t.Error("HasTag no debería encontrar un tag ausente")
	}
}

func TestTagsJSONRoundtrip(t *testing.T) {
	if got := TagsJSON(nil); got != "[]" {
		t.Errorf("TagsJSON(nil) = %q, want []", got)
	}
	got := ParseTagsJSON(TagsJSON([]string{"Blocked", "bug"}))
	want := []string{"blocked", "bug"}
	if len(got) != len(want) {
		t.Fatalf("roundtrip = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if got := ParseTagsJSON(""); got != nil {
		t.Errorf("ParseTagsJSON(\"\") = %v, want nil", got)
	}
}

// TestDefaultWorkflowIncludesDelivered asegura que delivered exista entre
// doing y reviewing en los defaults.
func TestDefaultWorkflowIncludesDelivered(t *testing.T) {
	idx := func(list []string, s string) int {
		for i, v := range list {
			if v == s {
				return i
			}
		}
		return -1
	}
	doing, delivered, reviewing := idx(DefaultWorkflow, "doing"), idx(DefaultWorkflow, "delivered"), idx(DefaultWorkflow, "reviewing")
	if delivered < 0 {
		t.Fatalf("DefaultWorkflow no incluye delivered: %v", DefaultWorkflow)
	}
	if !(doing < delivered && delivered < reviewing) {
		t.Errorf("delivered debe ir entre doing y reviewing: %v", DefaultWorkflow)
	}
	if !HasStatus(DefaultListOrder, "delivered") {
		t.Errorf("DefaultListOrder no incluye delivered: %v", DefaultListOrder)
	}
}
