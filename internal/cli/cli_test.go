package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/model"
)

// captureOutput redirects stdout/stderr/exit during the test and returns the stdout
// buffer. exit does not kill the process: it panics a sentinel that run() lets escape,
// so the command stops exactly where the real process would stop.
type exitPanic struct{ code int }

func captureOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	origOut, origErr, origExit := stdout, stderr, exit
	stdout, stderr = out, &bytes.Buffer{}
	exit = func(code int) { panic(exitPanic{code: code}) }
	t.Cleanup(func() { stdout, stderr, exit = origOut, origErr, origExit })
	return out
}

// run runs Run(args) capturing the output. Returns the exit code: 0
// if the command finished without calling outputError, 1 if it did.
func run(t *testing.T, args ...string) (stdoutText string, code int) {
	t.Helper()
	out := captureOutput(t)
	code = 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(exitPanic)
				if !ok {
					panic(r)
				}
				code = e.code
			}
		}()
		Run(args)
	}()
	return out.String(), code
}

// withTempDB points the config at a temporary database and returns its path.
func withTempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tsk.db")
	cfg := filepath.Join(dir, "config.toml")
	body := "[database]\npath = \"" + dbPath + "\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", cfg)
	return dbPath
}

func TestRunUnknownCommandFallsBackToTUI(t *testing.T) {
	// Without arguments, or with something that is not a subcommand, Run returns false
	// so that main launches the TUI.
	for _, args := range [][]string{
		{},
		{"tsk"},
		{"nope"},
		{"--unknown"},
		{"Project"}, // the dispatch is case-sensitive
	} {
		if got := Run(args); got {
			t.Errorf("Run(%v) = true, want false (it must fall back to the TUI)", args)
		}
	}
}

func TestRunHandlesKnownCommands(t *testing.T) {
	withTempDB(t)
	tests := []struct {
		name string
		args []string
	}{
		{"help", []string{"help"}},
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
		{"project add", []string{"project", "add", "api"}},
		{"add", []string{"add", "first", "--project", "api"}},
		{"list", []string{"list"}},
		{"stats", []string{"stats"}},
		{"migrate", []string{"migrate"}},
		{"completion bash", []string{"completion", "bash"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, code := run(t, tt.args...); code != 0 {
				t.Errorf("Run(%v) exited with %d", tt.args, code)
			}
		})
	}
}

func TestRunMissingArgsIsAnError(t *testing.T) {
	tests := [][]string{
		{"show"},
		{"update"},
		{"move"},
		{"start"},
		{"review"},
		{"done"},
		{"cancel"},
	}
	for _, args := range tests {
		if _, code := run(t, args...); code != 1 {
			t.Errorf("Run(%v) = %d, want 1 (usage error)", args, code)
		}
	}
}

func TestParseIDRejectsGarbage(t *testing.T) {
	captureOutput(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("parseID(\"abc\") should have called outputError")
		}
	}()
	parseID("abc")
}

func TestParseIDAcceptsNumbers(t *testing.T) {
	captureOutput(t)
	for in, want := range map[string]int64{"1": 1, "42": 42, "-7": -7} {
		if got := parseID(in); got != want {
			t.Errorf("parseID(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTruncateLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		w    int
		want string
	}{
		{"fits whole", "abc", 5, "abc"},
		{"fits exactly", "abc", 3, "abc"},
		{"truncates with dots", "abcdefgh", 5, "abc.."},
		{"width 1", "abc", 1, "a"},
		{"width 2", "abc", 2, "ab"},
		{"width 0", "abc", 0, ""},
		{"utf-8 by runes", "áéíóú", 3, "á.."},
		{"empty", "", 3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateLabel(tt.in, tt.w); got != tt.want {
				t.Errorf("truncateLabel(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		in   string
		w    int
		want string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"abc", 0, "abc"},
		{"", 2, "  "},
		{"áé", 4, "áé  "},
	}
	for _, tt := range tests {
		if got := padRight(tt.in, tt.w); got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
		}
	}
}

func TestRenderGanttTextInvalidStart(t *testing.T) {
	if got := renderGanttText(&model.Schedule{Start: "not-a-date"}, 4); got != "" {
		t.Errorf("renderGanttText with an invalid start = %q, want \"\"", got)
	}
}

func TestRenderGanttText(t *testing.T) {
	s := &model.Schedule{
		Start: "2026-09-14",
		Assignees: []model.AssigneeSchedule{{
			Assignee: "@a",
			End:      "2026-09-16",
			Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 1, Title: "first"}, Start: "2026-09-14", End: "2026-09-15", Estimate: 2},
				{Task: model.Task{ID: 2, Title: "second"}, Start: "2026-09-16", End: "2026-09-16", Estimate: 1, EstimateDefaulted: true},
			},
		}},
		Unassigned: []model.Task{{ID: 9, Title: "no owner"}},
	}
	out := renderGanttText(s, 3)

	for _, want := range []string{
		"Gantt · start 2026-09-14 · 3 weeks",
		"@a  ends 2026-09-16",
		"#1 first",
		"#2 second",
		"2026-09-14→2026-09-15 [2d]",
		"2026-09-16→2026-09-16 [1d] ~", // the tilde marks the default estimate
		"Unassigned (1):",
		"#9 no owner",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output without %q:\n%s", want, out)
		}
	}

	// The 2-day bar takes two cells and the 1-day one takes one.
	if !strings.Contains(out, "██") {
		t.Errorf("no 2-day bar:\n%s", out)
	}
	// The ruler carries the week label of the starting Monday.
	if !strings.Contains(out, "3SEP") {
		t.Errorf("the week label is missing:\n%s", out)
	}
}

func TestRenderGanttTextNoUnassignedHidesSection(t *testing.T) {
	out := renderGanttText(&model.Schedule{Start: "2026-09-14"}, 1)
	if strings.Contains(out, "Unassigned") {
		t.Errorf("with no unassigned tasks it must not print the section:\n%s", out)
	}
}

func TestOutputJSONShape(t *testing.T) {
	out := captureOutput(t)
	outputJSON(map[string]any{"ok": true})
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (%q)", err, out.String())
	}
	if got["ok"] != true {
		t.Errorf("payload = %v", got)
	}
}

func TestCompletionShells(t *testing.T) {
	withTempDB(t)
	for _, sh := range []string{"bash", "zsh", "fish"} {
		out, code := run(t, "completion", sh)
		if code != 0 {
			t.Errorf("completion %s exited with %d", sh, code)
		}
		if !strings.Contains(out, "tsk") {
			t.Errorf("completion %s does not mention tsk: %q", sh, out)
		}
	}
	if out, _ := run(t, "completion"); !strings.Contains(out, "bash") {
		t.Errorf("completion without a shell should list the shells: %q", out)
	}
}

// runErr is run but returning stderr, which is where outputError writes. Write
// errors are read there; normal output, on stdout.
func runErr(t *testing.T, args ...string) (stderrText string, code int) {
	t.Helper()
	out := &bytes.Buffer{}
	origOut, origErr, origExit := stdout, stderr, exit
	stdout, stderr = &bytes.Buffer{}, out
	exit = func(c int) { panic(exitPanic{c}) }
	t.Cleanup(func() { stdout, stderr, exit = origOut, origErr, origExit })

	code = 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(exitPanic)
				if !ok {
					panic(r)
				}
				code = e.code
			}
		}()
		Run(args)
	}()
	return out.String(), code
}
