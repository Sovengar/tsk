package model

import (
	"reflect"
	"testing"
)

func TestStatusChoices(t *testing.T) {
	defaultWF := []string{"backlog", "todo", "doing", "delivered", "reviewing", "done", "cancelled"}

	tests := []struct {
		name     string
		workflow []string
		current  string
		want     []string
	}{
		{
			name:     "backlog: everything that follows",
			workflow: defaultWF,
			current:  "backlog",
			want:     []string{"todo", "doing", "delivered", "reviewing", "done", "cancelled"},
		},
		{
			name:     "doing: forward then the passed ones reversed",
			workflow: defaultWF,
			current:  "doing",
			want:     []string{"delivered", "reviewing", "done", "todo", "backlog", "cancelled"},
		},
		{
			name:     "done: only the already-passed ones",
			workflow: defaultWF,
			current:  "done",
			want:     []string{"reviewing", "delivered", "doing", "todo", "backlog", "cancelled"},
		},
		{
			name:     "cancelled: never offers current",
			workflow: defaultWF,
			current:  "cancelled",
			want:     []string{"backlog", "todo", "doing", "delivered", "reviewing", "done"},
		},
		{
			name:     "unknown current: the whole workflow in order",
			workflow: defaultWF,
			current:  "unknown",
			want:     []string{"backlog", "todo", "doing", "delivered", "reviewing", "done", "cancelled"},
		},
		{
			name:     "single status workflow",
			workflow: []string{"done"},
			current:  "done",
			want:     []string{"cancelled"},
		},
		{
			name:     "workflow without cancelled appends it",
			workflow: []string{"backlog", "todo", "done"},
			current:  "todo",
			want:     []string{"done", "backlog", "cancelled"},
		},
		{
			name:     "workflow without cancelled and unknown current",
			workflow: []string{"backlog", "todo", "done"},
			current:  "ghost",
			want:     []string{"backlog", "todo", "done", "cancelled"},
		},
		{
			name:     "cancelled in the middle is skipped when locating current",
			workflow: []string{"todo", "cancelled", "done"},
			current:  "todo",
			want:     []string{"done", "cancelled"},
		},
		{
			name:     "empty workflow yields nothing",
			workflow: nil,
			current:  "todo",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusChoices(tt.workflow, tt.current)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("StatusChoices(%v, %q) = %v, want %v", tt.workflow, tt.current, got, tt.want)
			}
		})
	}
}

// The contract also holds independently of the exact rows: no current, no
// duplicates and a non-empty result for a non-empty workflow.
func TestStatusChoicesInvariants(t *testing.T) {
	workflows := [][]string{
		{"backlog", "todo", "doing", "done", "cancelled"},
		{"todo", "doing", "done"},
		{"done"},
		{"a", "b", "c"},
	}
	currents := []string{"backlog", "todo", "doing", "done", "cancelled", "ghost"}

	for _, wf := range workflows {
		for _, current := range currents {
			got := StatusChoices(wf, current)
			if len(got) == 0 {
				t.Errorf("StatusChoices(%v, %q) is empty", wf, current)
				continue
			}
			seen := map[string]bool{}
			for _, s := range got {
				if s == current {
					t.Errorf("StatusChoices(%v, %q) contains current %q", wf, current, s)
				}
				if seen[s] {
					t.Errorf("StatusChoices(%v, %q) repeats %q: %v", wf, current, s, got)
				}
				seen[s] = true
			}
		}
	}
}
