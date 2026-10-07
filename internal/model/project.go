package model

import (
	"encoding/json"
	"fmt"
)

// Project represents a project registered in tsk.
type Project struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Workflow   []string `json:"workflow"`
	ListOrder  []string `json:"list_order"`
	Archived   bool     `json:"archived"`
	ArchivedAt string   `json:"archived_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// DefaultWorkflow is the default flow when none is specified. It defines the
// progression of the actions; it must include "done".
var DefaultWorkflow = []string{"backlog", "todo", "doing", "delivered", "reviewing", "done", "cancelled"}

// DefaultListOrder is the default presentation order of the List view.
// It contains the same states as DefaultWorkflow, in a different order.
var DefaultListOrder = []string{"reviewing", "delivered", "doing", "todo", "backlog", "done", "cancelled"}

// ParseWorkflow converts an "a,b,c" string into []string.
func ParseWorkflow(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var parts []string
	for _, p := range splitComma(s) {
		p = trimSpace(p)
		if p == "" {
			continue
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty workflow")
	}
	return parts, nil
}

// HasStatus checks whether the workflow contains the given state.
func HasStatus(workflow []string, status string) bool {
	for _, s := range workflow {
		if s == status {
			return true
		}
	}
	return false
}

// FindStatus containing does a partial search (for "review").
func FindStatusContaining(workflow []string, substr string) (string, bool) {
	for _, s := range workflow {
		if contains(s, substr) {
			return s, true
		}
	}
	return "", false
}

// DoneStatus is the final state that the "done" action moves to. It must be
// present in the project's workflow (validated on create/edit).
const DoneStatus = "done"

// TerminalStatus returns the terminal state of the closing actions: always
// "done", regardless of its position in the workflow.
func TerminalStatus(workflow []string) string {
	return DoneStatus
}

// StartStatus returns the second state of the workflow (after backlog if it exists).
func StartStatus(workflow []string) string {
	if len(workflow) <= 1 {
		return TerminalStatus(workflow)
	}
	if workflow[0] == "backlog" && len(workflow) >= 2 {
		return workflow[1]
	}
	return workflow[0]
}

// CancelledStatus is the implicit cancelled state.
const CancelledStatus = "cancelled"

// NextStatus returns the next state in the workflow.
func NextStatus(workflow []string, current string) (string, bool) {
	for i, s := range workflow {
		if s == current && i+1 < len(workflow) {
			return workflow[i+1], true
		}
	}
	return "", false
}

// PrevStatus returns the previous state in the workflow.
func PrevStatus(workflow []string, current string) (string, bool) {
	for i, s := range workflow {
		if s == current && i > 0 {
			return workflow[i-1], true
		}
	}
	return "", false
}

// ValidateWorkflow checks that there are no duplicates and that it includes the state
// "done", required by the closing actions.
func ValidateWorkflow(workflow []string) error {
	seen := make(map[string]bool, len(workflow))
	for _, s := range workflow {
		if seen[s] {
			return fmt.Errorf("duplicate status %q in workflow", s)
		}
		seen[s] = true
	}
	if !HasStatus(workflow, DoneStatus) {
		return fmt.Errorf("workflow must include %q", DoneStatus)
	}
	return nil
}

// ValidateListOrder checks that list_order has no duplicates and that every
// state belongs to the project's workflow (cancelled is always allowed).
// An empty list_order is valid: it means "use the workflow order".
func ValidateListOrder(workflow, listOrder []string) error {
	seen := make(map[string]bool, len(listOrder))
	for _, s := range listOrder {
		if s == "" {
			return fmt.Errorf("empty status in list_order")
		}
		if seen[s] {
			return fmt.Errorf("duplicate status %q in list_order", s)
		}
		seen[s] = true
		if s == CancelledStatus {
			continue
		}
		if !HasStatus(workflow, s) {
			return fmt.Errorf("status %q in list_order not in workflow %v", s, workflow)
		}
	}
	return nil
}

// WorkflowJSON serializes the workflow to a JSON string for storage.
func WorkflowJSON(workflow []string) string {
	b, _ := json.Marshal(workflow)
	return string(b)
}

// ParseWorkflowJSON deserializes a JSON string into []string.
func ParseWorkflowJSON(s string) ([]string, error) {
	var w []string
	if err := json.Unmarshal([]byte(s), &w); err != nil {
		return nil, err
	}
	return w, nil
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}

// contains tells whether substr appears inside s. An empty substr never
// matches: without the filter, containsAt would compare against the empty string and
// find a match at index 0.
func contains(s, substr string) bool {
	if substr == "" || len(substr) > len(s) {
		return false
	}
	return s == substr || containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
