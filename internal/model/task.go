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

// Task represents a task in a project.
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

// PriorityLabel returns the readable label of the priority.
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

// PriorityShortLabel returns the abbreviated label (H/M/L).
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

// PriorityBar returns a visual priority character.
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

// IsActive tells whether the task is not in a terminal state.
func (t *Task) IsActive() bool {
	return t.Status != CancelledStatus && t.Status != "done"
}

// NormalizeTags cleans a tag list: trims spaces, lowercases,
// drops empties and duplicates, preserving the order of appearance.
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

// ParseTags converts an "a, b,c" string into a normalized list.
func ParseTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return NormalizeTags(strings.Split(s, ","))
}

// HasTag tells whether the list contains the given tag (case-insensitive comparison).
func HasTag(tags []string, tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	for _, t := range tags {
		if strings.ToLower(strings.TrimSpace(t)) == tag {
			return true
		}
	}
	return false
}

// TagsJSON serializes tags for storage; always returns a JSON array.
func TagsJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// ParseTagsJSON deserializes tags from storage; tolerates empty and null.
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

// TaskListResponse is the JSON listing response.
type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
}

// TaskResponse is the JSON response of a single task.
type TaskResponse struct {
	OK   bool `json:"ok"`
	Task Task `json:"task"`
}

// TaskActionResult is the response of start/done/cancel/move.
type TaskActionResult struct {
	OK       bool   `json:"ok"`
	Task     Task   `json:"task"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// ErrorResult is the standard error format.
type ErrorResult struct {
	Error string `json:"error"`
}
