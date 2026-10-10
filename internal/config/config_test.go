package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
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

// loadFrom writes content to a temp config file, points TSK_CONFIG at it and
// returns Load().
func loadFrom(t *testing.T, content string) Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", path)
	return Load()
}

func TestLoadHarnesses(t *testing.T) {
	cfg := loadFrom(t, `
[[harness]]
name = "opencode"

[[harness]]
name = "my-tool"
binary = "my-tool-bin"
`)
	if len(cfg.Harnesses) != 2 {
		t.Fatalf("Harnesses = %+v, want 2 entries", cfg.Harnesses)
	}
	if cfg.Harnesses[0].Name != "opencode" || cfg.Harnesses[0].Binary != "" {
		t.Errorf("first entry = %+v, want name opencode and empty binary", cfg.Harnesses[0])
	}
	if cfg.Harnesses[1].Name != "my-tool" || cfg.Harnesses[1].Binary != "my-tool-bin" {
		t.Errorf("second entry = %+v", cfg.Harnesses[1])
	}
}

// A single malformed harness entry must not wipe the whole config: the scalars
// survive and the bad entry is dropped without taking the valid ones with it.
func TestLoadMalformedHarnessDoesNotWipeConfig(t *testing.T) {
	cfg := loadFrom(t, `
list_page_size = 25
gantt_weeks = 9

[ai.ask.handoff]
command = "launch {{prompt_file}}"

[[harness]]
name = "good"

[[harness]]
name = 5

[[harness]]
binary = "nameless"
`)
	if cfg.ListPageSize != 25 {
		t.Errorf("ListPageSize = %d, want 25 (scalars must survive a bad harness entry)", cfg.ListPageSize)
	}
	if cfg.GanttWeeks != 9 {
		t.Errorf("GanttWeeks = %d, want 9", cfg.GanttWeeks)
	}
	if cfg.Handoff.Command != "launch {{prompt_file}}" {
		t.Errorf("Handoff.Command = %q, want it preserved", cfg.Handoff.Command)
	}
	if len(cfg.Harnesses) != 1 {
		t.Fatalf("Harnesses = %+v, want only the valid entry", cfg.Harnesses)
	}
	if cfg.Harnesses[0].Name != "good" {
		t.Errorf("Harnesses[0] = %+v, want name good", cfg.Harnesses[0])
	}
}

func TestLoadHandoff(t *testing.T) {
	cfg := loadFrom(t, `
[ai.ask.handoff]
command = "herdr agent start {{harness}}"
cwd = "/tmp/work"
prompt = "Run this task: Task #{{id}}: {{title}} ({{project}}). Statuses: {{statuses}}."
`)
	if cfg.Handoff.Command != "herdr agent start {{harness}}" {
		t.Errorf("Handoff.Command = %q", cfg.Handoff.Command)
	}
	if cfg.Handoff.CWD != "/tmp/work" {
		t.Errorf("Handoff.CWD = %q, want /tmp/work", cfg.Handoff.CWD)
	}
	if cfg.Handoff.Prompt != "Run this task: Task #{{id}}: {{title}} ({{project}}). Statuses: {{statuses}}." {
		t.Errorf("Handoff.Prompt = %q", cfg.Handoff.Prompt)
	}
}

// A partial ai/ask section (no ask table, or no handoff table under it) must
// not wipe the tag-decoded value: the nested spelling is only used when it is
// complete.
func TestLoadHandoffPartialSectionsIgnored(t *testing.T) {
	for _, doc := range []string{"[ai]\nother = 1\n", "[ai.ask]\nother = 2\n"} {
		cfg := loadFrom(t, doc)
		if cfg.Handoff.Command != "" || cfg.Handoff.CWD != "" || cfg.Handoff.Prompt != "" {
			t.Errorf("Handoff = %+v, want zero value for %q", cfg.Handoff, doc)
		}
	}
}

// The dotted toml tag matches a quoted literal section too, so both spellings
// of the handoff section work.
func TestLoadHandoffQuotedLiteralSection(t *testing.T) {
	cfg := loadFrom(t, `
["ai.ask.handoff"]
command = "launch {{prompt_file}}"
`)
	if cfg.Handoff.Command != "launch {{prompt_file}}" {
		t.Errorf("Handoff.Command = %q", cfg.Handoff.Command)
	}
}

func TestLoadNoHarnessesOrHandoff(t *testing.T) {
	cfg := loadFrom(t, "list_page_size = 10\n")
	if len(cfg.Harnesses) != 0 {
		t.Errorf("Harnesses = %+v, want none", cfg.Harnesses)
	}
	if cfg.Handoff.Command != "" || cfg.Handoff.CWD != "" {
		t.Errorf("Handoff = %+v, want zero value", cfg.Handoff)
	}
}

// A malformed whole file still falls back to defaults, as before.
func TestLoadMalformedFileFallsBack(t *testing.T) {
	cfg := loadFrom(t, "list_page_size = = =\n")
	if cfg.ListPageSize != DefaultPageSize {
		t.Errorf("ListPageSize = %d, want default %d", cfg.ListPageSize, DefaultPageSize)
	}
}

// decodeHarnesses is total: a non-array harness table is dropped instead of
// blowing up.
func TestDecodeHarnessesDefensive(t *testing.T) {
	if got := decodeHarnesses(map[string]any{"harness": 5}); got != nil {
		t.Errorf("non-array harness = %+v, want nil", got)
	}
	var raw map[string]any
	if _, err := toml.Decode("[[harness]]\nname = \"ok\"\n", &raw); err != nil {
		t.Fatal(err)
	}
	if got := decodeHarnesses(raw); len(got) != 1 || got[0].Name != "ok" {
		t.Errorf("valid entry = %+v, want one entry named ok", got)
	}
}
