package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"tsk/internal/db"
	"tsk/internal/harness"
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

// withAskEnv points TSK_CONFIG at a fresh config with the given handoff command
// (empty string = no [handoff] section) plus extra harness TOML, and empties
// PATH so built-in detection finds nothing and the declared harnesses are the
// only variable. Returns the database path.
func withAskEnv(t *testing.T, handoffCommand, extraHarnessTOML string) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tsk.db")
	cfgPath := filepath.Join(dir, "config.toml")
	body := "[database]\npath = \"" + dbPath + "\"\n"
	if handoffCommand != "" {
		body += "\n[handoff]\ncommand = '" + handoffCommand + "'\n"
	}
	body += extraHarnessTOML
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", cfgPath)
	// No binary is on PATH: detection contributes nothing.
	t.Setenv("PATH", t.TempDir())
	return dbPath
}

const harnessOpencode = "\n[[harness]]\nname = \"opencode\"\n"

// seedTask creates one project and task and returns the task id.
func seedTask(t *testing.T, dbPath string) int64 {
	t.Helper()
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	if _, err := database.CreateProject("api", nil); err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask("api", "Add dark mode", "implement the toggle", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func TestAskStatusesReturnsProjectWorkflow(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "tsk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	if _, err := database.CreateProject("web", []string{"todo", "doing", "done"}); err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask("web", "T", "", "", 0, "todo")
	if err != nil {
		t.Fatal(err)
	}

	got := askStatuses(database, task)
	want := []string{"todo", "doing", "done"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("askStatuses = %v, want %v", got, want)
	}

	if got := askStatuses(database, &model.Task{ProjectName: "missing"}); strings.Join(got, ",") != strings.Join(model.DefaultWorkflow, ",") {
		t.Errorf("askStatuses(unknown project) = %v, want the default workflow", got)
	}
}

func TestCmdAskLaunches(t *testing.T) {
	dbPath := withAskEnv(t, "true {{prompt_file}}", harnessOpencode)
	id := seedTask(t, dbPath)

	out, code := run(t, "ask", strconv.FormatInt(id, 10), "--harness", "opencode")
	if code != 0 {
		t.Fatalf("tsk ask exited with %d: %s", code, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, out)
	}
	if got["ok"] != true || got["launched"] != true {
		t.Errorf("payload = %v, want ok and launched true", got)
	}
	if got["harness"] != "opencode" {
		t.Errorf("harness = %v, want opencode", got["harness"])
	}
}

func TestCmdAskAutoPicksOnlyHarness(t *testing.T) {
	dbPath := withAskEnv(t, "true {{prompt_file}}", harnessOpencode)
	id := seedTask(t, dbPath)

	out, code := run(t, "ask", strconv.FormatInt(id, 10))
	if code != 0 {
		t.Fatalf("tsk ask exited with %d: %s", code, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["harness"] != "opencode" {
		t.Errorf("harness = %v, want the only available opencode", got["harness"])
	}
}

func TestCmdAskRefusesToGuess(t *testing.T) {
	extra := "\n[[harness]]\nname = \"opencode\"\n\n[[harness]]\nname = \"claude\"\n"
	dbPath := withAskEnv(t, "true {{prompt_file}}", extra)
	id := seedTask(t, dbPath)

	errOut, code := runErr(t, "ask", strconv.FormatInt(id, 10))
	if code == 0 {
		t.Fatal("tsk ask without --harness and several harnesses should fail")
	}
	for _, want := range []string{"opencode", "claude"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("error without %q: %s", want, errOut)
		}
	}
}

func TestCmdAskUnknownTask(t *testing.T) {
	withAskEnv(t, "true {{prompt_file}}", harnessOpencode)
	if _, code := runErr(t, "ask", "999", "--harness", "opencode"); code == 0 {
		t.Fatal("tsk ask with an unknown task should fail")
	}
}

func TestCmdAskUnknownHarness(t *testing.T) {
	dbPath := withAskEnv(t, "true {{prompt_file}}", harnessOpencode)
	id := seedTask(t, dbPath)
	errOut, code := runErr(t, "ask", strconv.FormatInt(id, 10), "--harness", "nope")
	if code == 0 {
		t.Fatal("tsk ask with an unknown harness should fail")
	}
	if !strings.Contains(errOut, "nope") {
		t.Errorf("error without the requested name: %s", errOut)
	}
}

func TestCmdAskNoHandoffConfigured(t *testing.T) {
	dbPath := withAskEnv(t, "", harnessOpencode)
	id := seedTask(t, dbPath)
	errOut, code := runErr(t, "ask", strconv.FormatInt(id, 10), "--harness", "opencode")
	if code == 0 {
		t.Fatal("tsk ask without handoff.command should fail")
	}
	if !strings.Contains(errOut, "handoff.command") {
		t.Errorf("error must name handoff.command: %s", errOut)
	}
}

func TestCmdAskMissingPlaceholder(t *testing.T) {
	dbPath := withAskEnv(t, "true", harnessOpencode)
	id := seedTask(t, dbPath)
	errOut, code := runErr(t, "ask", strconv.FormatInt(id, 10), "--harness", "opencode")
	if code == 0 {
		t.Fatal("tsk ask with a command lacking {{prompt_file}} should fail")
	}
	if !strings.Contains(errOut, "{{prompt_file}}") {
		t.Errorf("error must name {{prompt_file}}: %s", errOut)
	}
}

func TestCmdAskNoHarnessesFound(t *testing.T) {
	dbPath := withAskEnv(t, "true {{prompt_file}}", "")
	id := seedTask(t, dbPath)
	errOut, code := runErr(t, "ask", strconv.FormatInt(id, 10))
	if code == 0 {
		t.Fatal("tsk ask with no harnesses should fail")
	}
	if !strings.Contains(errOut, "No harnesses found") {
		t.Errorf("error = %s, want No harnesses found", errOut)
	}
}

func TestCmdAskUsage(t *testing.T) {
	withAskEnv(t, "true {{prompt_file}}", harnessOpencode)
	if _, code := runErr(t, "ask"); code == 0 {
		t.Fatal("tsk ask without a task id should fail with usage")
	}
}

// A spawn that cannot start (nonexistent working directory) is surfaced as a
// JSON error instead of being swallowed.
func TestCmdAskSpawnError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tsk.db")
	cfgPath := filepath.Join(dir, "config.toml")
	body := "[database]\npath = \"" + dbPath + "\"\n\n[handoff]\ncommand = 'true {{prompt_file}}'\ncwd = \"/no/such/tsk-dir-xyz\"\n" + harnessOpencode
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", cfgPath)
	t.Setenv("PATH", t.TempDir())
	id := seedTask(t, dbPath)

	errOut, code := runErr(t, "ask", strconv.FormatInt(id, 10), "--harness", "opencode")
	if code == 0 {
		t.Fatal("tsk ask with an unusable cwd should fail")
	}
	if !strings.Contains(errOut, "no such file") && !strings.Contains(errOut, "chdir") {
		t.Errorf("error = %s, want the spawn failure", errOut)
	}
}

func TestSelectHarness(t *testing.T) {
	list := []harness.Harness{{Name: "opencode", Binary: "/opt/oc"}, {Name: "claude"}}

	got, err := selectHarness(list, "opencode")
	if err != nil || got.Name != "opencode" || got.Binary != "/opt/oc" {
		t.Errorf("selectHarness(opencode) = %+v, %v; want the full entry incl. binary", got, err)
	}
	if _, err := selectHarness(list, "nope"); err == nil || !strings.Contains(err.Error(), "opencode") {
		t.Errorf("unknown harness error = %v, want it to list available names", err)
	}
	// A requested name with nothing detected must not print a dangling "(available: )".
	if _, err := selectHarness(nil, "nope"); err == nil || !strings.Contains(err.Error(), "No harnesses found") {
		t.Errorf("selectHarness(unknown, none available) = %v, want No harnesses found", err)
	}
	if got, err := selectHarness([]harness.Harness{{Name: "only"}}, ""); err != nil || got.Name != "only" {
		t.Errorf("selectHarness with a single harness = %q, %v", got, err)
	}
	if _, err := selectHarness(list, ""); err == nil {
		t.Error("selectHarness with several harnesses and no pick should fail")
	}
	if _, err := selectHarness(nil, ""); err == nil || !strings.Contains(err.Error(), "No harnesses found") {
		t.Errorf("selectHarness with none = %v, want No harnesses found", err)
	}
}
