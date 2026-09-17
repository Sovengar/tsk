package model

import (
	"encoding/json"
	"strings"
)

// Priority levels.
const (
	PriorityNone   = 0
	PriorityLow    = 1
	PriorityMedium = 2
	PriorityHigh   = 3
)

// Task representa una tarea en un proyecto.
type Task struct {
	ID          int64    `json:"id"`
	ProjectID   int64    `json:"project_id"`
	ProjectName string   `json:"project,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Assignee    string   `json:"assignee"`
	Estimate    float64  `json:"estimate"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

// PriorityLabel devuelve la etiqueta legible de la prioridad.
func PriorityLabel(p int) string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityMedium:
		return "MED"
	case PriorityHigh:
		return "HIGH"
	default:
		return "none"
	}
}

// PriorityShortLabel devuelve la etiqueta abreviada (H/M/L).
func PriorityShortLabel(p int) string {
	switch p {
	case PriorityLow:
		return "L"
	case PriorityMedium:
		return "M"
	case PriorityHigh:
		return "H"
	default:
		return "-"
	}
}

// PriorityBar devuelve un caracter visual de prioridad.
func PriorityBar(p int) string {
	switch p {
	case PriorityHigh:
		return "●"
	case PriorityMedium:
		return "●"
	case PriorityLow:
		return "●"
	default:
		return " "
	}
}

// IsActive indica si la tarea no está en estado terminal.
func (t *Task) IsActive() bool {
	return t.Status != CancelledStatus && t.Status != "done"
}

// NormalizeTags limpia una lista de tags: recorta espacios, pasa a minúsculas,
// descarta vacíos y duplicados, preservando el orden de aparición.
func NormalizeTags(tags []string) []string {
	var out []string
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

// ParseTags convierte un string "a, b,c" en una lista normalizada.
func ParseTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return NormalizeTags(strings.Split(s, ","))
}

// HasTag indica si la lista contiene el tag dado (comparación case-insensitive).
func HasTag(tags []string, tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	for _, t := range tags {
		if strings.ToLower(strings.TrimSpace(t)) == tag {
			return true
		}
	}
	return false
}

// TagsJSON serializa tags para storage; siempre devuelve un array JSON.
func TagsJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// ParseTagsJSON deserializa tags desde storage; tolera vacío y null.
func ParseTagsJSON(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return nil
	}
	return NormalizeTags(tags)
}

// TaskListResponse es la respuesta JSON de listado.
type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
}

// TaskResponse es la respuesta JSON de una sola tarea.
type TaskResponse struct {
	OK   bool `json:"ok"`
	Task Task `json:"task"`
}

// TaskActionResult es la respuesta de start/done/cancel/move.
type TaskActionResult struct {
	OK       bool   `json:"ok"`
	Task     Task   `json:"task"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// ErrorResult es el formato estándar de error.
type ErrorResult struct {
	Error string `json:"error"`
}
