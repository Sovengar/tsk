package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/ai"
	"tsk/internal/aiprompt"
	"tsk/internal/config"
)

// stubAI replaces the process boundaries of the AI feature for one test. The
// globals are installed once, as closures over the struct, so a test may set a
// field at any point before the command runs.
type stubAI struct {
	run     func(context.Context, ai.Request) (ai.Spec, string, error)
	notify  func(bool, string, string) error
	spawn   func(string, string, string) bool
	state   func() (string, error)
	prompt  func(string) string
	resolve func(string, string) (string, string)
	focus   func() error
	path    func() (string, error)
	lastReq ai.Request
	notifs  []string
	// spawned records (logPath, jobPath, container) per background run.
	spawned [][3]string
}

func stub(t *testing.T) *stubAI {
	t.Helper()
	s := &stubAI{}
	restore := func() {
		aiRun = ai.RunWithRepair
		aiNotify = ai.Notify
		spawnDetached = defaultSpawnDetached
		aiStateDir = ai.StateDir
		aiPromptFor = aiprompt.Load
		aiPromptPath = aiprompt.Path
		aiResolveModel = ai.ResolveModel
		aiLookPath = exec.LookPath
		aiSpawnWorker = spawnAIWorker
		aiFocus = ai.Focus
	}
	restore()
	t.Cleanup(restore)

	aiRun = func(ctx context.Context, r ai.Request) (ai.Spec, string, error) {
		if s.run == nil {
			t.Fatal("unexpected harness run")
		}
		s.lastReq = r
		return s.run(ctx, r)
	}
	aiNotify = func(enabled bool, summary, body string) error {
		if enabled {
			s.notifs = append(s.notifs, summary+"|"+body)
		}
		if s.notify == nil {
			return nil
		}
		return s.notify(enabled, summary, body)
	}
	aiFocus = func() error {
		if s.focus == nil {
			return nil
		}
		return s.focus()
	}
	aiSpawnWorker = func(jobPath, logPath, container string) bool {
		s.spawned = append(s.spawned, [3]string{logPath, jobPath, container})
		if s.spawn == nil {
			return true
		}
		return s.spawn(jobPath, logPath, container)
	}
	aiStateDir = func() (string, error) {
		if s.state == nil {
			return ai.StateDir()
		}
		return s.state()
	}
	aiPromptFor = func(path string) string {
		if s.prompt == nil {
			return aiprompt.Load(path)
		}
		return s.prompt(path)
	}
	aiPromptPath = func() (string, error) {
		if s.path == nil {
			return aiprompt.Path()
		}
		return s.path()
	}
	aiResolveModel = func(model, thinking string) (string, string) {
		if s.resolve == nil {
			return model, thinking
		}
		return s.resolve(model, thinking)
	}

	return s
}

// specRun returns a fixed spec.
func specRun(spec ai.Spec) func(context.Context, ai.Request) (ai.Spec, string, error) {
	return func(context.Context, ai.Request) (ai.Spec, string, error) {
		return spec, "raw", nil
	}
}

// runErr is run() with the stderr buffer exposed: the AI commands report their
// failures through outputError, which writes JSON to stderr.
func runAIErr(t *testing.T, args ...string) (stdoutText, stderrText string, code int) {
	t.Helper()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	origOut, origErr, origExit := stdout, stderr, exit
	stdout, stderr = out, errOut
	exit = func(code int) { panic(exitPanic{code: code}) }
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
	return out.String(), errOut.String(), code
}

// withAIConfig writes a config.toml carrying the [ai] section over the temp
// database the test already points at.
func withAIConfig(t *testing.T, aiSection string) {
	t.Helper()
	path := os.Getenv("TSK_CONFIG")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\n[ai]\n"+aiSection+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddAIWaitCreatesTheTask(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	s := stub(t)
	s.run = specRun(ai.Spec{
		Title:       "Resume voice-gate",
		Project:     "voice-gate",
		Assignee:    "@sovengar",
		Description: "Read the recovery notes.",
		Tags:        []string{"voice"},
	})

	out, code := run(t, "add", "--ai", "--wait", "--project", "voice-gate", "revisar", "recovery")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, out)
	}
	if payload["ok"] != true || payload["pending"] != nil {
		t.Fatalf("payload = %v", payload)
	}
	task, ok := payload["task"].(map[string]any)
	if !ok || task["title"] != "Resume voice-gate" {
		t.Fatalf("task = %v", payload["task"])
	}
	if s.lastReq.Prompt == "" || s.lastReq.Harness != config.DefaultHarness {
		t.Fatalf("request = %+v", s.lastReq)
	}
	if !strings.Contains(s.lastReq.Prompt, "revisar recovery") {
		t.Fatalf("prompt did not carry the request: %q", s.lastReq.Prompt)
	}
	if !strings.Contains(s.lastReq.Prompt, "voice-gate") {
		t.Fatalf("prompt did not carry the project list: %q", s.lastReq.Prompt)
	}
}

func TestAddAIWaitReportsAnInvalidSpec(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	s := stub(t)
	s.run = specRun(ai.Spec{Title: "T", Project: "nope", Description: "d"})

	_, errOut, code := runAIErr(t, "add", "--ai", "--wait", "texto")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "does not exist") {
		t.Fatalf("stderr = %q", errOut)
	}
	if len(s.notifs) != 1 || !strings.Contains(s.notifs[0], "rejected the spec") {
		t.Fatalf("notifications = %v", s.notifs)
	}
}

func TestAddAIWaitReportsAHarnessFailure(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	s := stub(t)
	s.run = func(context.Context, ai.Request) (ai.Spec, string, error) {
		return ai.Spec{}, "", ai.ErrHarnessNotStarted
	}

	_, errOut, code := runAIErr(t, "add", "--ai", "--wait", "texto")
	if code != 1 || !strings.Contains(errOut, "could not be started") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestAddAIWaitReportsADatabaseFailure(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	s := stub(t)
	s.run = specRun(ai.Spec{
		Title:       "T",
		Project:     "voice-gate",
		Description: "d",
		Tags:        []string{"tag"},
	})

	// A status outside the project workflow makes the insert fail after a
	// valid spec, which is the only way to reach the error branch.
	_, errOut, code := runAIErr(t, "add", "--ai", "--wait", "--status", "nope", "texto")
	if code != 1 || errOut == "" {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestAddAIBackgroundSpawnsAWorker(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	dir := t.TempDir()
	s := stub(t)
	s.state = func() (string, error) { return dir, nil }
	s.notify = func(bool, string, string) error {
		t.Fatal("the parent must not notify: the worker does")
		return nil
	}
	s.run = func(context.Context, ai.Request) (ai.Spec, string, error) {
		t.Fatal("a background run must not call the harness in the parent")
		return ai.Spec{}, "", nil
	}

	out, code := run(t, "add", "--ai", "revisar recovery")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, out)
	}
	if payload["pending"] != true {
		t.Fatalf("payload = %v", payload)
	}
	if len(s.spawned) != 1 {
		t.Fatalf("spawned = %v", s.spawned)
	}
	jobPath := s.spawned[0][1]
	job, err := readAIJob(jobPath)
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if !strings.Contains(job.Prompt, "revisar recovery") || job.Harness != config.DefaultHarness {
		t.Fatalf("job = %+v", job)
	}
	if !job.Notify {
		t.Fatal("notify must default to on")
	}
	if _, err := os.Stat(s.spawned[0][0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the parent must not create the log: %v", err)
	}
}

func TestAddAIRequiresText(t *testing.T) {
	withTempDB(t)
	_, code := run(t, "add", "--ai")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestAddAIReportsThePromptPathFailure(t *testing.T) {
	withTempDB(t)
	s := stub(t)
	s.path = func() (string, error) { return "", errors.New("no home") }
	_, errOut, code := runAIErr(t, "add", "--ai", "texto")
	if code != 1 || !strings.Contains(errOut, "no home") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestAddAIReportsTheStateDirFailure(t *testing.T) {
	withTempDB(t)
	s := stub(t)
	s.state = func() (string, error) { return "", errors.New("no state") }
	_, errOut, code := runAIErr(t, "add", "--ai", "texto")
	if code != 1 || !strings.Contains(errOut, "no state") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestAddAIReportsAJobWriteFailure(t *testing.T) {
	withTempDB(t)
	// A regular file where the directory should be: MkdirAll fails.
	dir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := stub(t)
	s.state = func() (string, error) { return dir, nil }
	_, errOut, code := runAIErr(t, "add", "--ai", "texto")
	if code != 1 || errOut == "" {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestAddAIReportsWhenNoPaneWasOpened(t *testing.T) {
	withTempDB(t)
	s := stub(t)
	s.state = func() (string, error) { return t.TempDir(), nil }
	s.spawn = func(string, string, string) bool { return false }

	out, code := run(t, "add", "--ai", "texto")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, out)
	}
	if payload["pane"] != nil {
		t.Fatalf("pane = %v, want it absent on the fallback", payload["pane"])
	}
	if payload["container"] != config.DefaultContainer {
		t.Fatalf("container = %v, want %q", payload["container"], config.DefaultContainer)
	}
}

func TestAddAIFocusesThePaneOnSuccess(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	focused := 0
	s := stub(t)
	s.focus = func() error { focused++; return nil }
	s.run = specRun(ai.Spec{Title: "T", Project: "voice-gate", Description: "d"})

	if out, code := run(t, "add", "--ai", "--wait", "texto"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if focused != 1 {
		t.Fatalf("focus calls = %d, want 1", focused)
	}
}

func TestAddAISkipsFocusWhenDisabled(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	withAIConfig(t, "focus = false\n")
	focused := 0
	s := stub(t)
	s.focus = func() error { focused++; return nil }
	s.run = specRun(ai.Spec{Title: "T", Project: "voice-gate", Description: "d"})

	if out, code := run(t, "add", "--ai", "--wait", "texto"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if focused != 0 {
		t.Fatalf("focus calls = %d, want 0", focused)
	}
}

func TestAddAIUsesTheConfiguredSettings(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	withAIConfig(t, "harness = \"codex\"\nmodel = \"m\"\nthinking = \"high\"\nnotify = false\n")
	dir := t.TempDir()
	s := stub(t)
	s.state = func() (string, error) { return dir, nil }
	s.prompt = func(string) string { return "custom {{text}} {{project_hint}}" }

	out, code := run(t, "add", "--ai", "--project", "x", "hola")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	job, err := readAIJob(s.spawned[0][1])
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if job.Harness != "codex" || job.Model != "m" || job.Thinking != "high" || job.Notify {
		t.Fatalf("job = %+v", job)
	}
	if job.Prompt != "custom hola x" {
		t.Fatalf("prompt = %q", job.Prompt)
	}
}

func TestAddAIResolvesTheModelCategory(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	withAIConfig(t, "model = \"dumb\"\nthinking = \"\"\n")
	var got [2]string
	s := stub(t)
	s.state = func() (string, error) { return t.TempDir(), nil }
	s.resolve = func(model, thinking string) (string, string) {
		got = [2]string{model, thinking}
		return "opencode-go/beam-free", "medium"
	}

	if out, code := run(t, "add", "--ai", "hola"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if got != [2]string{"dumb", ""} {
		t.Fatalf("resolver input = %v", got)
	}
	job, err := readAIJob(s.spawned[0][1])
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if job.Model != "opencode-go/beam-free" || job.Thinking != "medium" {
		t.Fatalf("job model = %q, thinking = %q", job.Model, job.Thinking)
	}
}

func TestAddAIUsesTheConfiguredPromptPath(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	promptPath := filepath.Join(t.TempDir(), "custom.md")
	if err := os.WriteFile(promptPath, []byte("from the file: {{text}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	withAIConfig(t, "prompt = \""+promptPath+"\"\n")
	s := stub(t)
	s.state = func() (string, error) { return t.TempDir(), nil }
	s.path = func() (string, error) {
		t.Fatal("an explicit ai.prompt must win over the default path")
		return "", nil
	}

	if out, code := run(t, "add", "--ai", "hola"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	job, err := readAIJob(s.spawned[0][1])
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if job.Prompt != "from the file: hola" {
		t.Fatalf("prompt = %q", job.Prompt)
	}
}

func TestAddAINormalisesTheAssigneeFlag(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	dir := t.TempDir()
	s := stub(t)
	s.state = func() (string, error) { return dir, nil }

	if out, code := run(t, "add", "--ai", "--assignee", "ana", "texto"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	job, err := readAIJob(s.spawned[0][1])
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if job.DefaultAssignee != "@ana" {
		t.Fatalf("assignee = %q", job.DefaultAssignee)
	}

	if out, code := run(t, "add", "--ai", "--assignee", "@ana", "texto"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	job, err = readAIJob(s.spawned[1][1])
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if job.DefaultAssignee != "@ana" {
		t.Fatalf("assignee = %q", job.DefaultAssignee)
	}
}

func TestWorkerCreatesTheTask(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	s := stub(t)
	s.run = specRun(ai.Spec{
		Title:       "Resume voice-gate",
		Project:     "voice-gate",
		Description: "d",
	})

	jobPath := writeJobFixture(t, aiJob{
		Prompt:          "p",
		Harness:         "pi",
		Notify:          true,
		Projects:        []string{"voice-gate"},
		Assignees:       []string{"@sovengar"},
		DefaultAssignee: "@sovengar",
	})

	out, code := run(t, "__ai-worker", "--job", jobPath)
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, out)
	}
	if payload["ok"] != true {
		t.Fatalf("payload = %v", payload)
	}
}

func TestWorkerRequiresTheJobFlag(t *testing.T) {
	_, errOut, code := runAIErr(t, "__ai-worker")
	if code != 1 || !strings.Contains(errOut, "usage") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestWorkerReportsAReadFailure(t *testing.T) {
	_, errOut, code := runAIErr(t, "__ai-worker", "--job", filepath.Join(t.TempDir(), "absent.json"))
	if code != 1 || errOut == "" {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestWorkerReportsABadJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runAIErr(t, "__ai-worker", "--job", path)
	if code != 1 || errOut == "" {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestWorkerReportsAFailedRun(t *testing.T) {
	s := stub(t)
	s.run = func(context.Context, ai.Request) (ai.Spec, string, error) {
		return ai.Spec{}, "", ai.ErrHarnessNotStarted
	}
	jobPath := writeJobFixture(t, aiJob{Prompt: "p", Harness: "pi"})
	_, errOut, code := runAIErr(t, "__ai-worker", "--job", jobPath)
	if code != 1 || !strings.Contains(errOut, "could not be started") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestSpawnDetachedStartsTheWorker(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")
	jobPath := filepath.Join(dir, "job.json")

	var got []string
	restore := startProcess
	startProcess = func(cmd *exec.Cmd) error {
		got = cmd.Args
		return nil
	}
	t.Cleanup(func() { startProcess = restore })

	if err := realSpawnDetached(logPath, jobPath); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if len(got) != 4 || got[1] != "__ai-worker" || got[2] != "--job" || got[3] != jobPath {
		t.Fatalf("args = %v", got)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log not created: %v", err)
	}
}

func TestDefaultStartProcessStartsACommand(t *testing.T) {
	cmd := exec.Command("true")
	if err := startProcess(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

func TestDefaultSpawnDetachedDelegates(t *testing.T) {
	restoreStart := startProcess
	startProcess = func(*exec.Cmd) error { return nil }
	t.Cleanup(func() { startProcess = restoreStart })

	if err := defaultSpawnDetached(filepath.Join(t.TempDir(), "log"), "job"); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestSpawnDetachedReportsTheExecutableFailure(t *testing.T) {
	restore := executablePath
	executablePath = func() (string, error) { return "", errors.New("no exe") }
	t.Cleanup(func() { executablePath = restore })

	if err := realSpawnDetached(filepath.Join(t.TempDir(), "log"), "job"); err == nil {
		t.Fatal("want the executable error")
	}
}

func TestSpawnDetachedReportsTheLogFailure(t *testing.T) {
	dir := t.TempDir()
	if err := realSpawnDetached(filepath.Join(dir, "missing", "log"), "job"); err == nil {
		t.Fatal("want the log error")
	}
}

func TestSpawnDetachedReportsTheStartFailure(t *testing.T) {
	restore := startProcess
	startProcess = func(*exec.Cmd) error { return errors.New("no fork") }
	t.Cleanup(func() { startProcess = restore })

	err := realSpawnDetached(filepath.Join(t.TempDir(), "log"), "job")
	if err == nil || !strings.Contains(err.Error(), "no fork") {
		t.Fatalf("err = %v", err)
	}
}

func TestAIStateUsesTheProjectPath(t *testing.T) {
	withTempDB(t)
	projectDir := t.TempDir()
	if _, code := run(t, "project", "add", "voice-gate", "--path", projectDir); code != 0 {
		t.Fatalf("project add failed")
	}
	if _, code := run(t, "add", "existing", "--project", "voice-gate", "--assignee", "@ana"); code != 0 {
		t.Fatalf("task add failed")
	}
	database := openDB()
	defer closeDB(database)

	workdir, projects, assignees := aiState(database, "voice-gate")
	if workdir != projectDir {
		t.Fatalf("workdir = %q, want %q", workdir, projectDir)
	}
	if len(projects) != 1 || projects[0] != "voice-gate" {
		t.Fatalf("projects = %v", projects)
	}
	if !containsString(assignees, "@ana") || !containsString(assignees, aiprompt.DefaultAssignee) {
		t.Fatalf("assignees = %v", assignees)
	}
}

func TestAIStateFallsBackToTheProcessDirectory(t *testing.T) {
	withTempDB(t)
	database := openDB()
	defer closeDB(database)

	workdir, _, _ := aiState(database, "unknown")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if workdir != wd {
		t.Fatalf("workdir = %q, want %q", workdir, wd)
	}
}

func TestAIStateSkipsEmptyAndDuplicateAssignees(t *testing.T) {
	withTempDB(t)
	if _, code := run(t, "project", "add", "voice-gate"); code != 0 {
		t.Fatalf("project add failed")
	}
	if _, code := run(t, "add", "one", "--project", "voice-gate", "--assignee", "@ana"); code != 0 {
		t.Fatalf("task add failed")
	}
	if _, code := run(t, "add", "two", "--project", "voice-gate", "--assignee", "@ana"); code != 0 {
		t.Fatalf("task add failed")
	}
	if _, code := run(t, "add", "three", "--project", "voice-gate"); code != 0 {
		t.Fatalf("task add failed")
	}
	database := openDB()
	defer closeDB(database)

	_, _, assignees := aiState(database, "voice-gate")
	count := 0
	for _, name := range assignees {
		if name == "@ana" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("assignees = %v, want @ana once", assignees)
	}
}

func TestAIStateFallsBackWhenGetwdFails(t *testing.T) {
	withTempDB(t)
	database := openDB()
	defer closeDB(database)
	restore := getwd
	getwd = func() (string, error) { return "", errors.New("no cwd") }
	t.Cleanup(func() { getwd = restore })

	if workdir, _, _ := aiState(database, "unknown"); workdir != "." {
		t.Fatalf("workdir = %q, want .", workdir)
	}
}

func TestAddAIForwardsPathAndStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "voice-gate")
	s := stub(t)
	s.state = func() (string, error) { return t.TempDir(), nil }

	if out, code := run(t, "add", "--ai", "--path", "/work/dir", "--status", "todo", "texto"); code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	job, err := readAIJob(s.spawned[0][1])
	if err != nil {
		t.Fatalf("job: %v", err)
	}
	if job.Path != "/work/dir" || job.Status != "todo" {
		t.Fatalf("job = %+v", job)
	}
}

func TestWriteAIJobReportsTheWriteFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	s := stub(t)
	s.state = func() (string, error) { return dir, nil }

	_, _, err := writeAIJob(aiJob{Prompt: "p"})
	if err == nil {
		t.Fatal("want the write failure")
	}
}

func TestSpawnAIWorkerUsesMuxWhenAvailable(t *testing.T) {
	restoreLook := aiLookPath
	aiLookPath = func(string) (string, error) { return "/usr/bin/mux", nil }
	t.Cleanup(func() { aiLookPath = restoreLook })
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")

	var got [3]string
	restoreStart := startProcess
	startProcess = func(cmd *exec.Cmd) error {
		got = [3]string{cmd.Path, strings.Join(cmd.Args[1:], " "), cmd.Dir}
		return nil
	}
	t.Cleanup(func() { startProcess = restoreStart })

	if !spawnAIWorker("job", "log", "Background") {
		t.Fatal("want a pane when mux and a multiplexer are present")
	}
	if !strings.HasSuffix(got[0], "mux") {
		t.Fatalf("spawned = %v, want mux", got)
	}
	for _, needle := range []string{"--container Background", "--layout tab", "--no-focus", "__ai-worker --job job"} {
		if !strings.Contains(got[1], needle) {
			t.Fatalf("argv = %q, missing %q", got[1], needle)
		}
	}
}

func TestSpawnInMuxReportsTheExecutableFailure(t *testing.T) {
	restore := executablePath
	executablePath = func() (string, error) { return "", errors.New("no exe") }
	t.Cleanup(func() { executablePath = restore })

	if err := spawnInMux("Background", "job"); err == nil {
		t.Fatal("want the executable error")
	}
}

func TestSpawnAIWorkerFallsBackWithoutMux(t *testing.T) {
	restoreLook := aiLookPath
	aiLookPath = func(string) (string, error) { return "", errors.New("no mux") }
	t.Cleanup(func() { aiLookPath = restoreLook })

	var got [3]string
	restoreSpawn := spawnDetached
	spawnDetached = func(logPath, jobPath string) error {
		got = [3]string{logPath, jobPath, "detached"}
		return nil
	}
	t.Cleanup(func() { spawnDetached = restoreSpawn })

	if !spawnAIWorker("job", "log", "Background") {
		t.Fatal("want the detached fallback to succeed")
	}
	if got != [3]string{"log", "job", "detached"} {
		t.Fatalf("spawn = %v", got)
	}
}

func TestSpawnAIWorkerFallsBackWhenMuxFails(t *testing.T) {
	restoreLook := aiLookPath
	aiLookPath = func(string) (string, error) { return "/usr/bin/mux", nil }
	t.Cleanup(func() { aiLookPath = restoreLook })
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")

	restoreStart := startProcess
	startProcess = func(*exec.Cmd) error { return errors.New("no mux socket") }
	t.Cleanup(func() { startProcess = restoreStart })

	called := false
	restoreSpawn := spawnDetached
	spawnDetached = func(string, string) error { called = true; return nil }
	t.Cleanup(func() { spawnDetached = restoreSpawn })

	if !spawnAIWorker("job", "log", "Background") {
		t.Fatal("want the detached fallback to succeed")
	}
	if !called {
		t.Fatal("the fallback must run when mux fails")
	}
}

func TestSpawnAIWorkerReportsAFailingFallback(t *testing.T) {
	restoreLook := aiLookPath
	aiLookPath = func(string) (string, error) { return "", errors.New("no mux") }
	t.Cleanup(func() { aiLookPath = restoreLook })

	restoreSpawn := spawnDetached
	spawnDetached = func(string, string) error { return errors.New("no fork") }
	t.Cleanup(func() { spawnDetached = restoreSpawn })

	if spawnAIWorker("job", "log", "Background") {
		t.Fatal("want no pane when both paths fail")
	}
}

func TestHasMultiplexerDetectsEveryBackend(t *testing.T) {
	restoreLook := aiLookPath
	aiLookPath = func(string) (string, error) { return "/usr/bin/mux", nil }
	t.Cleanup(func() { aiLookPath = restoreLook })

	for _, key := range []string{"HERDR_ENV", "TUIOS_ENV", "TMUX", "KITTY_LISTEN_ON", "WEZTERM_PANE"} {
		for _, other := range []string{"HERDR_ENV", "TUIOS_ENV", "TMUX", "KITTY_LISTEN_ON", "WEZTERM_PANE"} {
			t.Setenv(other, "")
		}
		t.Setenv(key, "1")
		if !hasMultiplexer() {
			t.Fatalf("%s must count as a multiplexer", key)
		}
	}

	for _, other := range []string{"HERDR_ENV", "TUIOS_ENV", "TMUX", "KITTY_LISTEN_ON", "WEZTERM_PANE"} {
		t.Setenv(other, "")
	}
	if hasMultiplexer() {
		t.Fatal("no backend must mean no pane")
	}
}

func TestAIStateSurvivesAClosedDatabase(t *testing.T) {
	withTempDB(t)
	database := openDB()
	closeDB(database)

	workdir, projects, assignees := aiState(database, "voice-gate")
	if workdir == "" {
		t.Fatal("workdir must never be empty")
	}
	if len(projects) != 0 {
		t.Fatalf("projects = %v", projects)
	}
	if len(assignees) != 1 {
		t.Fatalf("assignees = %v, want only the default", assignees)
	}
}

func writeJobFixture(t *testing.T, job aiJob) string {
	t.Helper()
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "job.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
