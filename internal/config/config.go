package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const FileName = "config.toml"

// DefaultPageSize is the default number of tasks per page in the List view.
const DefaultPageSize = 10

// DefaultEstimateDays is the default estimate (in days) for tasks without
// an estimate, used by the Gantt projection.
const DefaultEstimateDays = 1.0

// DefaultGanttWeeks is the default visible horizon of the Gantt, in weeks.
const DefaultGanttWeeks = 6

// Config is the global configuration of tsk.
type Config struct {
	Database            DatabaseConfig `toml:"database"`
	Editor              EditorConfig   `toml:"editor"`
	ListPageSize        int            `toml:"list_page_size"`
	DefaultEstimateDays float64        `toml:"default_estimate_days"`
	GanttWeeks          int            `toml:"gantt_weeks"`
	// Harnesses declares additional AI harnesses, additive to PATH detection.
	// It is decoded apart from the scalars because it is the only list-typed
	// key and its typed decode is the one that can fail on a single bad entry.
	Harnesses []HarnessConfig `toml:"harness"`
	// Handoff configures how a task is handed to a harness. An empty Command
	// disables the handoff.
	Handoff HandoffConfig `toml:"handoff"`
}

// HarnessConfig declares one AI harness. Name is the display name and the
// default binary; Binary overrides the executable to look up.
type HarnessConfig struct {
	Name   string `toml:"name"`
	Binary string `toml:"binary"`
}

// HandoffConfig is the shell template used to launch a harness. Command is a
// shell command template with {{harness}} (display name), {{harness_binary}}
// (executable), {{cwd}} and {{prompt_file}} placeholders; CWD overrides the
// working directory (default: tsk's launch dir).
type HandoffConfig struct {
	Command string `toml:"command"`
	CWD     string `toml:"cwd"`
}

// scalarConfig is the view of the config file used for the scalar decode. It
// deliberately omits the [[harness]] list: BurntSushi returns an error for a
// typed slice with one malformed entry, and decoding through the full Config
// would turn that single entry into a whole-config fallback to Defaults(). With
// this view the file's scalars always decode, and the harness list is salvaged
// entry by entry afterwards.
type scalarConfig struct {
	Database            DatabaseConfig `toml:"database"`
	Editor              EditorConfig   `toml:"editor"`
	ListPageSize        int            `toml:"list_page_size"`
	DefaultEstimateDays float64        `toml:"default_estimate_days"`
	GanttWeeks          int            `toml:"gantt_weeks"`
	Handoff             HandoffConfig  `toml:"handoff"`
}

// DatabaseConfig configures the database.
type DatabaseConfig struct {
	Path string `toml:"path"`
}

// EditorConfig configures the external editor.
type EditorConfig struct {
	Command string `toml:"command"`
}

// Defaults returns the default configuration.
func Defaults() Config {
	return Config{
		Database: DatabaseConfig{
			Path: "", // resolved dynamically via DefaultPath()
		},
		Editor: EditorConfig{
			Command: "nvim",
		},
		ListPageSize:        DefaultPageSize,
		DefaultEstimateDays: DefaultEstimateDays,
		GanttWeeks:          DefaultGanttWeeks,
	}
}

// Path resolves the configuration file path.
func Path() (string, error) {
	if p := os.Getenv("TSK_CONFIG"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "tsk", FileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tsk", FileName), nil
}

// Load reads the configuration file (if it exists) over the defaults.
func Load() Config {
	cfg := Defaults()
	path, err := Path()
	if err != nil {
		cfg.Database.Path = ""
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg // no file: defaults
	}

	// Scalars first, through a view that excludes [[harness]]: an unknown key is
	// ignored, so even a malformed harness entry leaves the scalars intact.
	var scalars scalarConfig
	if _, err := toml.Decode(string(data), &scalars); err != nil {
		return Defaults() // malformed file: defaults
	}
	cfg.Database = scalars.Database
	cfg.Editor = scalars.Editor
	cfg.ListPageSize = scalars.ListPageSize
	cfg.DefaultEstimateDays = scalars.DefaultEstimateDays
	cfg.GanttWeeks = scalars.GanttWeeks
	cfg.Handoff = scalars.Handoff

	// The harness list is decoded apart, entry by entry, so a single bad entry
	// is dropped instead of wiping the whole config.
	cfg.Harnesses = decodeHarnesses(data)

	if cfg.ListPageSize <= 0 {
		cfg.ListPageSize = DefaultPageSize
	}
	if cfg.DefaultEstimateDays <= 0 {
		cfg.DefaultEstimateDays = DefaultEstimateDays
	}
	if cfg.GanttWeeks <= 0 {
		cfg.GanttWeeks = DefaultGanttWeeks
	}
	return cfg
}

// decodeHarnesses salvages the [[harness]] entries from the file's raw tree.
// An entry is dropped when it is not a table, when its name is not a string or
// when it is empty; the rest survive. The raw decode is used instead of the
// typed one because the raw decode succeeds even when an entry has a wrong type
// (the typed slice decode returns an error and would discard everything).
func decodeHarnesses(data []byte) []HarnessConfig {
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil
	}
	entries, ok := raw["harness"].([]map[string]any)
	if !ok {
		return nil
	}
	out := make([]HarnessConfig, 0, len(entries))
	for _, e := range entries {
		name, ok := e["name"].(string)
		if !ok || name == "" {
			continue
		}
		binary, _ := e["binary"].(string)
		out = append(out, HarnessConfig{Name: name, Binary: binary})
	}
	return out
}
