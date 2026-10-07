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

// TestLoadNumericDefaults covers the triple contract of the numeric knobs that
// Load sanitizes: 0 and negatives fall back to the default (exact boundary of `<= 0`, which a
// `< 0` does not distinguish), and a positive value different from the default is respected
// (otherwise the sanitizing would always apply).
func TestLoadNumericDefaults(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantEst   float64
		wantWeeks int
	}{
		{"valid values", "default_estimate_days = 3.5\ngantt_weeks = 12\n", 3.5, 12},
		{"zero uses default", "default_estimate_days = 0\ngantt_weeks = 0\n", DefaultEstimateDays, DefaultGanttWeeks},
		{"negative uses default", "default_estimate_days = -2\ngantt_weeks = -1\n", DefaultEstimateDays, DefaultGanttWeeks},
		{"no file uses default", "", DefaultEstimateDays, DefaultGanttWeeks},
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
		{"valid value", "list_page_size = 25\n", 25},
		{"zero uses default", "list_page_size = 0\n", DefaultPageSize},
		{"negative uses default", "list_page_size = -3\n", DefaultPageSize},
		{"no file uses default", "", DefaultPageSize},
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
