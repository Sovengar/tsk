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
	// Handoff configures how a task is handed to an AI harness. An empty
	// Command disables the handoff. One command serves every harness; the
	// agent-specific argv is produced by the shared ai binary.
	Handoff HandoffConfig `toml:"handoff"`
	// AI configures `tsk add --ai`: the harness that turns a free-form request
	// into a task, its optional model/effort, and the notification on finish.
	AI AIConfig `toml:"ai"`
}

// AIConfig is the [ai] section used by `tsk add --ai`. The prompt itself lives
// in ai.md next to this file; only its path is configurable here.
type AIConfig struct {
	// Harness is the agent to run: pi, codex, opencode, hermes, jcode.
	Harness string `toml:"harness"`
	// Model and Thinking are optional overrides for the harness.
	Model    string `toml:"model"`
	Thinking string `toml:"thinking"`
	// Notify sends a desktop notification when a background run finishes.
	// Nil means on: the command is fire-and-forget, so the notification is the
	// only completion signal.
	Notify *bool `toml:"notify"`
	// Prompt is the prompt file; empty selects ai.md next to config.toml.
	Prompt string `toml:"prompt"`
	// Container is the mux workspace that hosts every AI run, so the shell
	// that started one can be closed without losing the work.
	Container string `toml:"container"`
	// Focus moves the focus to a run's pane when it finishes. Nil means on.
	Focus *bool `toml:"focus"`
}

// DefaultContainer is the mux workspace used when [ai] container is empty.
const DefaultContainer = "Background"

// DefaultHarness is the agent used when [ai] harness is empty.
const DefaultHarness = "pi"

// NotifyEnabled reports whether the desktop notification is on.
func (a AIConfig) NotifyEnabled() bool {
	return a.Notify == nil || *a.Notify
}

// ContainerName returns the shared mux workspace of AI runs.
func (a AIConfig) ContainerName() string {
	if a.Container == "" {
		return DefaultContainer
	}
	return a.Container
}

// FocusEnabled reports whether a finished run steals the focus.
func (a AIConfig) FocusEnabled() bool {
	return a.Focus == nil || *a.Focus
}

// HarnessName returns the configured harness, or DefaultHarness.
func (a AIConfig) HarnessName() string {
	if a.Harness == "" {
		return DefaultHarness
	}
	return a.Harness
}

// HandoffConfig is the single shell template used to launch any harness.
// Command carries {{harness}} (display name), {{working_dir}}, {{container}},
// {{model}}, {{effort}}, {{session_id}} and {{prompt_file}} placeholders. Model, Thinking and CWD are
// optional global defaults. Prompt is the prompt template handed to the
// harness, with {{id}}, {{title}}, {{project}} and {{statuses}} placeholders;
// empty selects the built-in default.
type HandoffConfig struct {
	Command    string `toml:"command"`
	WorkingDir string `toml:"working_dir"`
	Prompt     string `toml:"prompt"`
	Model      string `toml:"model"`
	Thinking   string `toml:"thinking"`
}

// scalarConfig is the view of the config file used for the scalar decode.
type scalarConfig struct {
	Database            DatabaseConfig `toml:"database"`
	Editor              EditorConfig   `toml:"editor"`
	ListPageSize        int            `toml:"list_page_size"`
	DefaultEstimateDays float64        `toml:"default_estimate_days"`
	GanttWeeks          int            `toml:"gantt_weeks"`
	Handoff             HandoffConfig  `toml:"handoff"`
	AI                  AIConfig       `toml:"ai"`
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
	cfg.AI = scalars.AI

	// The handoff section is salvaged from the raw tree so the legacy
	// [ai.ask.handoff] spelling keeps working and a wrong-typed key never
	// wipes the typed decode above.
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err == nil {
		cfg.Handoff = handoffFromRaw(raw, scalars.Handoff)
	}

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

// handoffFromRaw salvages [handoff], falling back to the legacy nested
// [ai.ask.handoff] spelling. A partial section keeps the typed value; a key
// with a wrong type is skipped (config never fails).
func handoffFromRaw(raw map[string]any, current HandoffConfig) HandoffConfig {
	if table, ok := raw["handoff"].(map[string]any); ok {
		return handoffFromTable(table, current)
	}
	if table, ok := raw["ai.ask.handoff"].(map[string]any); ok {
		return handoffFromTable(table, current)
	}
	section, ok := raw["ai"].(map[string]any)
	if !ok {
		return current
	}
	section, ok = section["ask"].(map[string]any)
	if !ok {
		return current
	}
	table, ok := section["handoff"].(map[string]any)
	if !ok {
		return current
	}
	return handoffFromTable(table, current)
}

func handoffFromTable(table map[string]any, current HandoffConfig) HandoffConfig {
	if v, ok := table["command"].(string); ok {
		current.Command = v
	}
	if v, ok := table["working_dir"].(string); ok {
		current.WorkingDir = v
	}
	if v, ok := table["prompt"].(string); ok {
		current.Prompt = v
	}
	if v, ok := table["model"].(string); ok {
		current.Model = v
	}
	if v, ok := table["thinking"].(string); ok {
		current.Thinking = v
	}
	return current
}
