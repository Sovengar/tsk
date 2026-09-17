package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const FileName = "config.toml"

// DefaultPageSize es el número de tareas por página por defecto en la vista List.
const DefaultPageSize = 10

// DefaultEstimateDays es la estimación por defecto (en días) para tareas sin
// estimate, usada por la proyección del Gantt.
const DefaultEstimateDays = 1.0

// DefaultGanttWeeks es el horizonte visible por defecto del Gantt, en semanas.
const DefaultGanttWeeks = 6

// Config es la configuración global de tsk.
type Config struct {
	Database            DatabaseConfig `toml:"database"`
	Editor              EditorConfig   `toml:"editor"`
	ListPageSize        int            `toml:"list_page_size"`
	DefaultEstimateDays float64        `toml:"default_estimate_days"`
	GanttWeeks          int            `toml:"gantt_weeks"`
}

// DatabaseConfig configura la base de datos.
type DatabaseConfig struct {
	Path string `toml:"path"`
}

// EditorConfig configura el editor externo.
type EditorConfig struct {
	Command string `toml:"command"`
}

// Defaults devuelve la configuración por defecto.
func Defaults() Config {
	return Config{
		Database: DatabaseConfig{
			Path: "", // se resuelve dinámicamente vía DefaultPath()
		},
		Editor: EditorConfig{
			Command: "nvim",
		},
		ListPageSize:        DefaultPageSize,
		DefaultEstimateDays: DefaultEstimateDays,
		GanttWeeks:          DefaultGanttWeeks,
	}
}

// Path resuelve la ruta del fichero de configuración.
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

// Load lee el fichero de configuración (si existe) sobre los defaults.
func Load() Config {
	cfg := Defaults()
	path, err := Path()
	if err != nil {
		cfg.Database.Path = ""
		return cfg
	}
	if _, err := os.Stat(path); err != nil {
		return cfg // sin fichero: defaults
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Defaults() // fichero malformado: defaults
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
