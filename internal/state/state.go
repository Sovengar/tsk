// Package state persists small, non-configurable UI preferences.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const fileName = "state.json"

// State is the persisted UI state. Empty fields intentionally mean their
// default value, so a missing or malformed file can be ignored safely.
type State struct {
	Project string `json:"project,omitempty"`
}

// Path resolves tsk's XDG state file. TSK_STATE is a test and integration
// override; normal users should use XDG_STATE_HOME instead.
func Path() (string, error) {
	if p := os.Getenv("TSK_STATE"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "tsk", fileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "tsk", fileName), nil
}

// Load returns the saved state. Missing, unreadable, and malformed files
// return the zero state so the TUI can always start normally.
func Load() State {
	path, err := Path()
	if err != nil {
		return State{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	var saved State
	if err := json.Unmarshal(data, &saved); err != nil {
		return State{}
	}
	return saved
}

// Save atomically replaces the state file.
func Save(saved State) error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
