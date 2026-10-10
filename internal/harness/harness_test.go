package harness

import (
	"errors"
	"os"
	"strings"
	"testing"

	"tsk/internal/config"
	"tsk/internal/model"
)

// withLookPath replaces the PATH probe so detection is deterministic: only the
// binaries passed are "available".
func withLookPath(t *testing.T, onPath ...string) {
	t.Helper()
	orig := lookPath
	set := make(map[string]bool, len(onPath))
	for _, b := range onPath {
		set[b] = true
	}
	lookPath = func(binary string) (string, error) {
		if set[binary] {
			return "/usr/bin/" + binary, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = orig })
}

func names(harnesses []Harness) []string {
	out := make([]string, 0, len(harnesses))
	for _, h := range harnesses {
		out = append(out, h.Name)
	}
	return out
}

func TestDetectOnlyInstalledKnownHarnesses(t *testing.T) {
	withLookPath(t, "opencode")
	got := names(Detect(config.Defaults()))
	if len(got) != 1 || got[0] != "opencode" {
		t.Errorf("Detect = %v, want exactly [opencode]", got)
	}
}

func TestDetectIncludesConfigDeclaredUnknown(t *testing.T) {
	withLookPath(t)
	cfg := config.Defaults()
	cfg.Harnesses = []config.HarnessConfig{{Name: "my-tool", Binary: "my-tool-bin"}}
	got := Detect(cfg)
	if len(got) != 1 || got[0].Name != "my-tool" || got[0].Binary != "my-tool-bin" {
		t.Errorf("Detect = %+v, want the declared my-tool entry", got)
	}
}

func TestDetectConfigDeclaredWithoutBinaryDefaultsToName(t *testing.T) {
	withLookPath(t)
	cfg := config.Defaults()
	cfg.Harnesses = []config.HarnessConfig{{Name: "my-tool"}}
	got := Detect(cfg)
	if len(got) != 1 || got[0].Binary != "my-tool" {
		t.Errorf("Detect = %+v, want binary defaulting to the name", got)
	}
}

func TestDetectConfigOverridesDetectedSameName(t *testing.T) {
	withLookPath(t, "opencode", "claude")
	cfg := config.Defaults()
	cfg.Harnesses = []config.HarnessConfig{{Name: "opencode", Binary: "opencode-custom"}}
	got := Detect(cfg)
	var opencodes []Harness
	for _, h := range got {
		if h.Name == "opencode" {
			opencodes = append(opencodes, h)
		}
	}
	if len(opencodes) != 1 {
		t.Fatalf("Detect = %+v, want exactly one opencode entry", got)
	}
	if opencodes[0].Binary != "opencode-custom" {
		t.Errorf("opencode binary = %q, want the config override", opencodes[0].Binary)
	}
}

func TestDetectSortedAlphabetically(t *testing.T) {
	withLookPath(t, "opencode")
	cfg := config.Defaults()
	cfg.Harnesses = []config.HarnessConfig{{Name: "zzz"}, {Name: "aaa"}}
	got := names(Detect(cfg))
	want := []string{"aaa", "opencode", "zzz"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Detect order = %v, want %v", got, want)
	}
}

func TestComposePrompt(t *testing.T) {
	task := model.Task{ID: 7, Title: "Add dark mode", ProjectName: "tsk", Description: "implement the toggle"}
	want := "Run this task: Task #7: Add dark mode (tsk). Check it with `tsk show 7`. " +
		"Update its state as you go with `tsk move 7 <status>`; valid statuses: backlog, todo, doing, delivered, reviewing, done, cancelled."
	if got := ComposePrompt(task, model.DefaultWorkflow, ""); got != want {
		t.Errorf("ComposePrompt = %q, want %q", got, want)
	}
}

func TestComposePromptEmptyWorkflowDropsStatusClause(t *testing.T) {
	task := model.Task{ID: 3, Title: "Tidy", ProjectName: "tsk"}
	want := "Run this task: Task #3: Tidy (tsk). Check it with `tsk show 3`."
	got := ComposePrompt(task, nil, "")
	if got != want {
		t.Errorf("ComposePrompt = %q, want %q", got, want)
	}
	if strings.Contains(got, "valid statuses") {
		t.Errorf("ComposePrompt = %q, want no dangling status clause", got)
	}
}

func TestComposePromptCustomTemplate(t *testing.T) {
	task := model.Task{ID: 7, Title: "Add dark mode", ProjectName: "tsk"}
	got := ComposePrompt(task, []string{"todo", "doing"}, "Do task {{id}}: {{title}} ({{project}}). Valid statuses: {{statuses}}.")
	want := "Do task 7: Add dark mode (tsk). Valid statuses: todo, doing."
	if got != want {
		t.Errorf("ComposePrompt = %q, want %q", got, want)
	}
}

func TestComposePromptCustomTemplateEmptyWorkflowDropsStatusesSentence(t *testing.T) {
	task := model.Task{ID: 3, Title: "Tidy", ProjectName: "tsk"}
	got := ComposePrompt(task, nil, "Read `tsk show {{id}}`. Then update via `tsk move {{id}} <status>`; valid statuses: {{statuses}}.")
	want := "Read `tsk show 3`."
	if got != want {
		t.Errorf("ComposePrompt = %q, want %q", got, want)
	}
}

func TestComposePromptCustomTemplateWithoutStatusesPlaceholder(t *testing.T) {
	task := model.Task{ID: 3, Title: "Tidy", ProjectName: "tsk"}
	got := ComposePrompt(task, nil, "Just do {{id}}: {{title}}.")
	want := "Just do 3: Tidy."
	if got != want {
		t.Errorf("ComposePrompt = %q, want %q", got, want)
	}
}

func TestComposePromptDoesNotEmbedDescription(t *testing.T) {
	task := model.Task{ID: 1, Title: "Fix crash on empty list", ProjectName: "tsk", Description: "secret detail"}
	if got := ComposePrompt(task, model.DefaultWorkflow, ""); strings.Contains(got, "secret detail") {
		t.Errorf("ComposePrompt = %q, want the description omitted", got)
	}
}

func TestExpandTemplateSubstitutesAndQuotes(t *testing.T) {
	got, err := ExpandTemplate(
		"run --kind {{harness}} --bin {{harness_binary}} --cwd {{cwd}} --prompt {{prompt_file}}",
		Harness{Name: "my tool", Binary: "/o'pt/oc"}, "/a b", "/tmp/p'q",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := `run --kind 'my tool' --bin '/o'\''pt/oc' --cwd '/a b' --prompt '/tmp/p'\''q'`
	if got != want {
		t.Errorf("ExpandTemplate = %q, want %q", got, want)
	}
}

func TestExpandTemplateMissingPromptPlaceholder(t *testing.T) {
	_, err := ExpandTemplate("run {{harness}}", Harness{Name: "opencode", Binary: "opencode"}, "/tmp", "/tmp/p")
	if !errors.Is(err, ErrMissingPromptPlaceholder) {
		t.Fatalf("err = %v, want ErrMissingPromptPlaceholder", err)
	}
	if !strings.Contains(err.Error(), "ai.ask.handoff.command") || !strings.Contains(err.Error(), "{{prompt_file}}") {
		t.Errorf("err = %q, want it to name ai.ask.handoff.command and {{prompt_file}}", err)
	}
}

func TestValidateCommand(t *testing.T) {
	if err := ValidateCommand(""); !errors.Is(err, ErrNoHandoff) {
		t.Errorf("empty command err = %v, want ErrNoHandoff", err)
	}
	if err := ValidateCommand("run {{harness}}"); !errors.Is(err, ErrMissingPromptPlaceholder) {
		t.Errorf("missing placeholder err = %v, want ErrMissingPromptPlaceholder", err)
	}
	if err := ValidateCommand("run {{prompt_file}}"); err != nil {
		t.Errorf("valid command err = %v, want nil", err)
	}
}

func TestNoHandoffMessage(t *testing.T) {
	if NoHandoffMessage != "No handoff configured: set ai.ask.handoff.command in config.toml" {
		t.Errorf("NoHandoffMessage = %q", NoHandoffMessage)
	}
	if ErrNoHandoff.Error() != NoHandoffMessage {
		t.Errorf("ErrNoHandoff = %q, want %q", ErrNoHandoff, NoHandoffMessage)
	}
}

func withGetwd(t *testing.T, dir string, err error) {
	t.Helper()
	orig := getwd
	getwd = func() (string, error) { return dir, err }
	t.Cleanup(func() { getwd = orig })
}

func TestHandoffDirDefaultsToWorkingDir(t *testing.T) {
	withGetwd(t, "/launch/dir", nil)
	if got := HandoffDir(config.Defaults()); got != "/launch/dir" {
		t.Errorf("HandoffDir = %q, want the working dir", got)
	}
}

func TestHandoffDirOverride(t *testing.T) {
	withGetwd(t, "/launch/dir", nil)
	cfg := config.Defaults()
	cfg.Handoff.CWD = "/configured"
	if got := HandoffDir(cfg); got != "/configured" {
		t.Errorf("HandoffDir = %q, want the configured override", got)
	}
}

func TestHandoffDirFallsBackOnGetwdError(t *testing.T) {
	withGetwd(t, "", errors.New("no cwd"))
	if got := HandoffDir(config.Defaults()); got != "." {
		t.Errorf("HandoffDir = %q, want . on error", got)
	}
}

// captureSpawn replaces the detached spawn with a recorder.
func captureSpawn(t *testing.T) *spawnCall {
	t.Helper()
	call := &spawnCall{}
	orig := runDetached
	runDetached = func(name string, args []string, dir string) error {
		call.name, call.args, call.dir = name, args, dir
		return call.err
	}
	t.Cleanup(func() { runDetached = orig })
	return call
}

type spawnCall struct {
	name string
	args []string
	dir  string
	err  error
}

func TestLaunchExpandsAndSpawnsDetached(t *testing.T) {
	call := captureSpawn(t)
	h := Harness{Name: "opencode", Binary: "/opt/oc"}
	err := Launch("run {{harness}} --bin {{harness_binary}} -- {{prompt_file}}", h, "/work", "/tmp/p")
	if err != nil {
		t.Fatal(err)
	}
	if call.name != "/bin/sh" {
		t.Errorf("spawn name = %q, want /bin/sh", call.name)
	}
	if len(call.args) != 2 || call.args[0] != "-c" {
		t.Fatalf("spawn args = %v, want [-c <expanded>]", call.args)
	}
	want := `run 'opencode' --bin '/opt/oc' -- '/tmp/p'`
	if call.args[1] != want {
		t.Errorf("expanded = %q, want %q", call.args[1], want)
	}
	if call.dir != "/work" {
		t.Errorf("spawn dir = %q, want /work", call.dir)
	}
}

func TestLaunchTemplateErrorDoesNotSpawn(t *testing.T) {
	call := captureSpawn(t)
	err := Launch("run {{harness}}", Harness{Name: "opencode"}, "/work", "/tmp/p")
	if !errors.Is(err, ErrMissingPromptPlaceholder) {
		t.Fatalf("err = %v, want ErrMissingPromptPlaceholder", err)
	}
	if call.name != "" {
		t.Errorf("spawn was called (%q), want none", call.name)
	}
}

func TestLaunchPropagatesSpawnError(t *testing.T) {
	call := captureSpawn(t)
	call.err = errors.New("spawn boom")
	if err := Launch("run {{prompt_file}}", Harness{Name: "h"}, "/w", "/p"); err == nil {
		t.Fatal("Launch = nil, want the spawn error")
	}
}

func TestExecuteWritesPromptFileAndSpawns(t *testing.T) {
	call := captureSpawn(t)
	cfg := config.Defaults()
	cfg.Handoff.Command = "{{prompt_file}}"
	task := model.Task{ID: 7, Title: "Add dark mode", ProjectName: "tsk", Description: "implement the toggle"}

	if err := Execute(cfg, task, model.DefaultWorkflow, Harness{Name: "opencode", Binary: "opencode"}); err != nil {
		t.Fatal(err)
	}
	if call.name != "/bin/sh" {
		t.Fatalf("no spawn happened: %+v", call)
	}
	expanded := call.args[1]
	if len(expanded) < 2 || expanded[0] != '\'' || expanded[len(expanded)-1] != '\'' {
		t.Fatalf("expanded = %q, want a single-quoted prompt path", expanded)
	}
	path := expanded[1 : len(expanded)-1]
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("prompt file %q: %v", path, err)
	}
	want := "Run this task: Task #7: Add dark mode (tsk). Check it with `tsk show 7`. " +
		"Update its state as you go with `tsk move 7 <status>`; valid statuses: backlog, todo, doing, delivered, reviewing, done, cancelled."
	if string(data) != want {
		t.Errorf("prompt content = %q, want %q", data, want)
	}
	_ = os.Remove(path)
}

func TestExecuteUsesConfiguredPromptTemplate(t *testing.T) {
	call := captureSpawn(t)
	cfg := config.Defaults()
	cfg.Handoff.Command = "{{prompt_file}}"
	cfg.Handoff.Prompt = "Pick up {{id}}: {{title}} ({{project}}). Statuses: {{statuses}}."
	task := model.Task{ID: 9, Title: "Write tests", ProjectName: "tsk"}

	if err := Execute(cfg, task, []string{"todo", "doing"}, Harness{Name: "opencode", Binary: "opencode"}); err != nil {
		t.Fatal(err)
	}
	path := strings.Trim(call.args[1], "'")
	defer func() { _ = os.Remove(path) }()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("prompt file %q: %v", path, err)
	}
	want := "Pick up 9: Write tests (tsk). Statuses: todo, doing."
	if string(data) != want {
		t.Errorf("prompt content = %q, want %q", data, want)
	}
}

func TestExecuteRefusesWithoutCommand(t *testing.T) {
	call := captureSpawn(t)
	err := Execute(config.Defaults(), model.Task{ID: 1}, model.DefaultWorkflow, Harness{Name: "opencode"})
	if !errors.Is(err, ErrNoHandoff) {
		t.Fatalf("err = %v, want ErrNoHandoff", err)
	}
	if call.name != "" {
		t.Error("spawn was called, want none")
	}
}

func TestExecutePropagatesPromptFileError(t *testing.T) {
	call := captureSpawn(t)
	orig := createTemp
	createTemp = func(pattern string) (promptFile, error) {
		return nil, errors.New("temp boom")
	}
	t.Cleanup(func() { createTemp = orig })

	cfg := config.Defaults()
	cfg.Handoff.Command = "{{prompt_file}}"
	if err := Execute(cfg, model.Task{ID: 1}, model.DefaultWorkflow, Harness{Name: "opencode"}); err == nil {
		t.Fatal("Execute = nil, want the temp file error")
	}
	if call.name != "" {
		t.Error("spawn was called, want none")
	}
}

func TestExecuteRemovesPromptFileOnLaunchError(t *testing.T) {
	call := captureSpawn(t)
	call.err = errors.New("spawn boom")

	cfg := config.Defaults()
	cfg.Handoff.Command = "{{prompt_file}}"
	if err := Execute(cfg, model.Task{ID: 1}, model.DefaultWorkflow, Harness{Name: "opencode"}); err == nil {
		t.Fatal("Execute = nil, want the launch error")
	}
	if call.name != "/bin/sh" {
		t.Fatalf("no spawn happened: %+v", call)
	}
	path := strings.Trim(call.args[1], "'")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("prompt file %q must be removed when the launch fails (stat err = %v)", path, statErr)
	}
}

func TestPromptFileWritesContent0600(t *testing.T) {
	path, err := PromptFile("hello prompt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello prompt" {
		t.Errorf("content = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 0600", perm)
	}
}

func TestPromptFileCreateError(t *testing.T) {
	orig := createTemp
	createTemp = func(pattern string) (promptFile, error) {
		return nil, errors.New("no temp")
	}
	t.Cleanup(func() { createTemp = orig })

	if _, err := PromptFile("x"); err == nil {
		t.Fatal("PromptFile = nil, want the create error")
	}
}

func TestPromptFileWriteErrorPropagates(t *testing.T) {
	orig := createTemp
	createTemp = func(pattern string) (promptFile, error) {
		return &fakePromptFile{writeErr: errors.New("write boom")}, nil
	}
	t.Cleanup(func() { createTemp = orig })

	if _, err := PromptFile("x"); err == nil {
		t.Fatal("PromptFile = nil, want the write error")
	}
}

type fakePromptFile struct {
	written  string
	writeErr error
	closeErr error
	name     string
}

func (f *fakePromptFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.written += string(p)
	return len(p), nil
}

func (f *fakePromptFile) Close() error { return f.closeErr }
func (f *fakePromptFile) Name() string { return f.name }

func TestPromptFileWriteError(t *testing.T) {
	f := &fakePromptFile{writeErr: errors.New("write boom")}
	if err := writePromptFile(f, "x"); err == nil {
		t.Fatal("writePromptFile = nil, want the write error")
	}
}

func TestPromptFileCloseError(t *testing.T) {
	f := &fakePromptFile{closeErr: errors.New("close boom")}
	if err := writePromptFile(f, "x"); err == nil {
		t.Fatal("writePromptFile = nil, want the close error")
	}
	if f.written != "x" {
		t.Errorf("written = %q, want x", f.written)
	}
}

func TestPromptFileSuccessWithFake(t *testing.T) {
	f := &fakePromptFile{name: "/tmp/fake"}
	if err := writePromptFile(f, "body"); err != nil {
		t.Fatal(err)
	}
	if f.written != "body" {
		t.Errorf("written = %q", f.written)
	}
}

// realRunDetached is exercised for real: it is the only place that touches the
// OS, and both its success and its spawn-error branch must be reachable.
func TestRealRunDetached(t *testing.T) {
	if err := realRunDetached("/bin/sh", []string{"-c", "exit 0"}, t.TempDir()); err != nil {
		t.Fatalf("realRunDetached = %v, want nil", err)
	}
	if err := realRunDetached("tsk-no-such-binary-xyz", nil, t.TempDir()); err == nil {
		t.Fatal("realRunDetached with a missing binary = nil, want an error")
	}
}
