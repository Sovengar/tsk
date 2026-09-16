package model

import (
	"encoding/json"
	"fmt"
)

// Project representa un proyecto registrado en taskd.
type Project struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Workflow  []string `json:"workflow"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// DefaultWorkflow es el flujo por defecto si no se especifica.
var DefaultWorkflow = []string{"backlog", "todo", "in_progress", "review", "done"}

// ParseWorkflow convierte un string "a,b,c" en []string.
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

// HasStatus verifica si el workflow contiene el estado dado.
func HasStatus(workflow []string, status string) bool {
	for _, s := range workflow {
		if s == status {
			return true
		}
	}
	return false
}

// FindStatus containing realiza búsqueda parcial (para "review").
func FindStatusContaining(workflow []string, substr string) (string, bool) {
	for _, s := range workflow {
		if contains(s, substr) {
			return s, true
		}
	}
	return "", false
}

// TerminalStatus devuelve el último estado del workflow (done).
func TerminalStatus(workflow []string) string {
	if len(workflow) == 0 {
		return "done"
	}
	return workflow[len(workflow)-1]
}

// StartStatus devuelve el segundo estado del workflow (después de backlog si existe).
func StartStatus(workflow []string) string {
	if len(workflow) <= 1 {
		return TerminalStatus(workflow)
	}
	if workflow[0] == "backlog" && len(workflow) >= 2 {
		return workflow[1]
	}
	return workflow[0]
}

// CancelledStatus es el estado cancelado implícito.
const CancelledStatus = "cancelled"

// NextStatus devuelve el siguiente estado en el workflow.
func NextStatus(workflow []string, current string) (string, bool) {
	for i, s := range workflow {
		if s == current && i+1 < len(workflow) {
			return workflow[i+1], true
		}
	}
	return "", false
}

// PrevStatus devuelve el estado anterior en el workflow.
func PrevStatus(workflow []string, current string) (string, bool) {
	for i, s := range workflow {
		if s == current && i > 0 {
			return workflow[i-1], true
		}
	}
	return "", false
}

// ValidateWorkflow verifica que no haya duplicados.
func ValidateWorkflow(workflow []string) error {
	seen := make(map[string]bool, len(workflow))
	for _, s := range workflow {
		if seen[s] {
			return fmt.Errorf("duplicate status %q in workflow", s)
		}
		seen[s] = true
	}
	return nil
}

// WorkflowJSON serializa el workflow a JSON string para storage.
func WorkflowJSON(workflow []string) string {
	b, _ := json.Marshal(workflow)
	return string(b)
}

// ParseWorkflowJSON deserializa un JSON string a []string.
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
