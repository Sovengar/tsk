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
	if _, err := os.Stat(path); err != nil {
		return cfg // no file: defaults
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Defaults() // malformed file: defaults
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
