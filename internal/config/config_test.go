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

func TestLoadHandoff(t *testing.T) {
	cfg := loadFrom(t, `
[handoff]
command = "mux -- agentbridge exec --harness {{harness}} --prompt-file {{prompt_file}}"
working_dir = "/tmp/work"
prompt = "Run this task: Task #{{id}}: {{title}} ({{project}}). Statuses: {{statuses}}."
model = "free"
thinking = "high"
`)
	if cfg.Handoff.Command != "mux -- agentbridge exec --harness {{harness}} --prompt-file {{prompt_file}}" {
		t.Errorf("Handoff.Command = %q", cfg.Handoff.Command)
	}
	if cfg.Handoff.WorkingDir != "/tmp/work" {
		t.Errorf("Handoff.WorkingDir = %q, want /tmp/work", cfg.Handoff.WorkingDir)
	}
	if cfg.Handoff.Prompt != "Run this task: Task #{{id}}: {{title}} ({{project}}). Statuses: {{statuses}}." {
		t.Errorf("Handoff.Prompt = %q", cfg.Handoff.Prompt)
	}
	if cfg.Handoff.Model != "free" || cfg.Handoff.Thinking != "high" {
		t.Errorf("Handoff defaults = %+v, want free/high", cfg.Handoff)
	}
}

// The legacy [ai.ask.handoff] spelling still loads, but the new single
// [handoff] section is canonical.
func TestLoadLegacyHandoff(t *testing.T) {
	cfg := loadFrom(t, `
[ai.ask.handoff]
command = "launch {{prompt_file}}"
working_dir = "/tmp/legacy"
`)
	if cfg.Handoff.Command != "launch {{prompt_file}}" || cfg.Handoff.WorkingDir != "/tmp/legacy" {
		t.Errorf("Handoff = %+v, want legacy command and cwd", cfg.Handoff)
	}
}

// A partial ai/ask section (no ask table, or no handoff table under it) must
// not wipe the tag-decoded value.
func TestLoadHandoffPartialSectionsIgnored(t *testing.T) {
	for _, doc := range []string{"[ai]\nother = 1\n", "[ai.ask]\nother = 2\n"} {
		cfg := loadFrom(t, doc)
		if cfg.Handoff.Command != "" || cfg.Handoff.WorkingDir != "" || cfg.Handoff.Prompt != "" {
			t.Errorf("Handoff = %+v, want zero value for %q", cfg.Handoff, doc)
		}
	}
}

// The dotted toml tag matches a quoted literal section too.
func TestLoadHandoffQuotedLiteralSection(t *testing.T) {
	cfg := loadFrom(t, `
["ai.ask.handoff"]
command = "launch {{prompt_file}}"
`)
	if cfg.Handoff.Command != "launch {{prompt_file}}" {
		t.Errorf("Handoff.Command = %q", cfg.Handoff.Command)
	}
}

func TestLoadNoHandoff(t *testing.T) {
	cfg := loadFrom(t, "list_page_size = 10\n")
	if cfg.Handoff.Command != "" || cfg.Handoff.WorkingDir != "" {
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

// Unknown/legacy [[harness]] tables are ignored without wiping the handoff.
func TestLoadIgnoresHarnessTables(t *testing.T) {
	cfg := loadFrom(t, `
list_page_size = 25

[handoff]
command = "launch {{prompt_file}}"

[[harness]]
name = "pi"
binary = "pi"
`)
	if cfg.ListPageSize != 25 || cfg.Handoff.Command != "launch {{prompt_file}}" {
		t.Errorf("cfg = %+v, want scalars and handoff intact", cfg)
	}
}

// TestLoadAISection covers the [ai] block of `tsk add --ai`: the typed decode,
// the harness default and the notify tri-state (absent = on, false = off).
func TestLoadAISection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := "[ai]\nharness = \"codex\"\nmodel = \"m\"\nthinking = \"high\"\nnotify = false\nprompt = \"/p/ai.md\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", path)

	cfg := Load()
	if cfg.AI.Harness != "codex" || cfg.AI.Model != "m" || cfg.AI.Thinking != "high" {
		t.Fatalf("ai = %+v", cfg.AI)
	}
	if cfg.AI.Prompt != "/p/ai.md" {
		t.Fatalf("prompt = %q", cfg.AI.Prompt)
	}
	if cfg.AI.NotifyEnabled() {
		t.Fatal("notify = false must disable it")
	}
	if cfg.AI.HarnessName() != "codex" {
		t.Fatalf("harness = %q", cfg.AI.HarnessName())
	}
}

func TestAIDefaults(t *testing.T) {
	var ai AIConfig
	if !ai.NotifyEnabled() {
		t.Fatal("notify must default to on")
	}
	if ai.HarnessName() != DefaultHarness {
		t.Fatalf("harness = %q, want %q", ai.HarnessName(), DefaultHarness)
	}
	on := true
	ai.Notify = &on
	if !ai.NotifyEnabled() {
		t.Fatal("notify = true must enable it")
	}
}

func TestAIContainerAndFocus(t *testing.T) {
	var ai AIConfig
	if ai.ContainerName() != DefaultContainer {
		t.Fatalf("container = %q, want %q", ai.ContainerName(), DefaultContainer)
	}
	if !ai.FocusEnabled() {
		t.Fatal("focus must default to on")
	}

	ai.Container = "AI"
	if ai.ContainerName() != "AI" {
		t.Fatalf("container = %q, want the configured one", ai.ContainerName())
	}
	off := false
	ai.Focus = &off
	if ai.FocusEnabled() {
		t.Fatal("focus = false must disable it")
	}
}
