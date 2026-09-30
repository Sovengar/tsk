package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsPageSize(t *testing.T) {
	if got := Defaults().ListPageSize; got != DefaultPageSize {
		t.Errorf("ListPageSize = %d, want %d", got, DefaultPageSize)
	}
}

// TestLoadNumericDefaults cubre el triple contrato de los knobs numéricos que
// Load sanea: 0 y negativos caen al default (límite exacto del `<= 0`, que un
// `< 0` no distingue), y un valor positivo distinto del default se respeta
// (si no, el saneo se aplicaría siempre).
func TestLoadNumericDefaults(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantEst   float64
		wantWeeks int
	}{
		{"valores válidos", "default_estimate_days = 3.5\ngantt_weeks = 12\n", 3.5, 12},
		{"cero usa default", "default_estimate_days = 0\ngantt_weeks = 0\n", DefaultEstimateDays, DefaultGanttWeeks},
		{"negativo usa default", "default_estimate_days = -2\ngantt_weeks = -1\n", DefaultEstimateDays, DefaultGanttWeeks},
		{"sin fichero usa default", "", DefaultEstimateDays, DefaultGanttWeeks},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TSK_CONFIG", path)

			cfg := Load()
			if cfg.DefaultEstimateDays != tt.wantEst {
				t.Errorf("Load().DefaultEstimateDays = %v, want %v", cfg.DefaultEstimateDays, tt.wantEst)
			}
			if cfg.GanttWeeks != tt.wantWeeks {
				t.Errorf("Load().GanttWeeks = %d, want %d", cfg.GanttWeeks, tt.wantWeeks)
			}
		})
	}
}

func TestLoadPageSize(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"valor válido", "list_page_size = 25\n", 25},
		{"cero usa default", "list_page_size = 0\n", DefaultPageSize},
		{"negativo usa default", "list_page_size = -3\n", DefaultPageSize},
		{"sin fichero usa default", "", DefaultPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TSK_CONFIG", path)

			if got := Load().ListPageSize; got != tt.want {
				t.Errorf("Load().ListPageSize = %d, want %d", got, tt.want)
			}
		})
	}
}
