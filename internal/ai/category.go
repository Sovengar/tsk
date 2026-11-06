// Category resolution for the [ai] model.
//
// The operator's Pi setup keeps a catalog of named model categories (`dumb`,
// `free`, `smart`, ...) in <agentDir>/extensions-extras/models-by-category.json.
// Those names are aliases resolved by the Pi extensions, not models Pi accepts:
// `pi --model dumb` fails with "Model dumb not found". tsk resolves the name
// through the same catalog before it hands the model to agentbridge, so
// `ai.model = "dumb"` runs the physical model the operator configured.
//
// A reference carrying "/" is already a physical id and is never looked up. An
// unknown name is passed through unchanged, so the harness reports the usual
// "model not found" instead of tsk inventing a fallback.
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Category store and seed, relative to the Pi agent directory.
const (
	categoryStoreFile = "models-by-category.json"
	categorySeedFile  = "models-by-category.seed.json"
	categoryDir       = "extensions-extras"
)

// categoryEntry is one alias: the physical model and its optional thinking.
type categoryEntry struct {
	Model    string `json:"model"`
	Thinking string `json:"thinking"`
	Effort   string `json:"effort"`
}

// categoryDoc is the catalog document shape.
type categoryDoc struct {
	Categories map[string]categoryEntry `json:"categories"`
}

// Thinking returns the entry's effort, accepting the legacy `effort` spelling.
func (e categoryEntry) level() string {
	if e.Thinking != "" {
		return e.Thinking
	}
	return e.Effort
}

// ResolveModel maps a category name to its physical model. thinking is the
// operator's explicit override: when it is set it wins over the category's own
// level. A physical id, an empty model and an unknown name are returned as-is.
func ResolveModel(model, thinking string) (string, string) {
	if model == "" || strings.Contains(model, "/") {
		return model, thinking
	}
	entry, ok := lookupCategory(model)
	if !ok || !isModelID(entry.Model) {
		return model, thinking
	}
	if thinking == "" {
		thinking = entry.level()
	}
	return entry.Model, thinking
}

// AgentDir resolves the Pi agent directory: PI_CODING_AGENT_DIR, then
// ~/.pi/agent. A leading "~" in the override is expanded.
func AgentDir() string {
	if value := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); value != "" {
		if strings.HasPrefix(value, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, strings.TrimPrefix(value, "~"))
			}
		}
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".pi", "agent")
}

// lookupCategory reads the store first and the tracked seed second: the store
// is what `/categories set` writes, the seed is the shipped default.
func lookupCategory(name string) (categoryEntry, bool) {
	dir := AgentDir()
	if dir == "" {
		return categoryEntry{}, false
	}
	for _, file := range []string{categoryStoreFile, categorySeedFile} {
		entries := readCategories(filepath.Join(dir, categoryDir, file))
		if entry, ok := entries[name]; ok {
			return entry, true
		}
	}
	return categoryEntry{}, false
}

// readCategories decodes one catalog file. A missing, unreadable or malformed
// file yields no categories: resolution then leaves the reference untouched.
func readCategories(path string) map[string]categoryEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc categoryDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return doc.Categories
}

// isModelID reports whether value is a usable provider/model id, the same rule
// the catalog's own seed validation applies.
func isModelID(value string) bool {
	slash := strings.Index(value, "/")
	if slash <= 0 || slash == len(value)-1 {
		return false
	}
	return strings.TrimSpace(value[:slash]) != "" && strings.TrimSpace(value[slash+1:]) != ""
}
