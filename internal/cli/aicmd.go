package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tsk/internal/ai"
	"tsk/internal/aiprompt"
	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// aiJob is the whole state of one AI task creation. It is written to a file and
// handed to the background worker, so the worker needs no re-parsing of the
// command line and no access to the parent's memory.
type aiJob struct {
	Prompt   string `json:"prompt"`
	Harness  string `json:"harness"`
	Model    string `json:"model"`
	Thinking string `json:"thinking"`
	Notify   bool   `json:"notify"`
	// Container is the mux workspace that hosts the run, so it survives the
	// shell that started it.
	Container string `json:"container,omitempty"`
	// Focus moves the focus to the run's pane when it finishes.
	Focus           bool     `json:"focus,omitempty"`
	Workdir         string   `json:"workdir"`
	Path            string   `json:"path,omitempty"`
	Status          string   `json:"status,omitempty"`
	Projects        []string `json:"projects"`
	Assignees       []string `json:"assignees"`
	DefaultAssignee string   `json:"default_assignee"`
}

// aiResult is what `tsk add --ai` prints. A background run reports the log and
// the job file; a --wait run also reports the spec and the created task.
type aiResult struct {
	OK         bool        `json:"ok"`
	Pending    bool        `json:"pending,omitempty"`
	Log        string      `json:"log,omitempty"`
	Job        string      `json:"job,omitempty"`
	Harness    string      `json:"harness"`
	Container  string      `json:"container,omitempty"`
	Pane       bool        `json:"pane,omitempty"`
	DurationMS int64       `json:"duration_ms,omitempty"`
	Spec       *ai.Spec    `json:"spec,omitempty"`
	Task       *model.Task `json:"task,omitempty"`
}

// aiTimeout bounds one run: a harness that hangs must not keep a worker alive.
const aiTimeout = 10 * time.Minute

// Process boundaries of the AI feature; variables so tests never spawn a
// harness, a detached worker or a notification daemon.
var (
	aiRun          = ai.RunWithRepair
	aiNotify       = ai.Notify
	aiStateDir     = ai.StateDir
	aiPromptFor    = aiprompt.Load
	aiPromptPath   = aiprompt.Path
	aiResolveModel = ai.ResolveModel
	aiLookPath     = exec.LookPath
	aiSpawnWorker  = spawnAIWorker
	aiFocus        = ai.Focus
)

// cmdAddAI implements `tsk add --ai`: it loads ai.md, injects the live tsk
// state into the prompt and either runs the harness now (--wait) or hands the
// job to a detached worker that notifies when it finishes.
func cmdAddAI(args []string, load func() *db.DB) {
	var project, assignee, path, status, text string
	var words []string
	wait := false

	s := newFlagScanner(args, 0)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--ai", "--wait":
			wait = wait || flag == "--wait"
		case "--project":
			project, _ = s.next()
		case "--assignee":
			assignee, _ = s.next()
		case "--path":
			path, _ = s.next()
		case "--status":
			status, _ = s.next()
		default:
			words = append(words, flag)
		}
	}
	text = strings.TrimSpace(strings.Join(words, " "))
	if text == "" {
		outputError("usage: tsk add --ai [--wait] [--project X] [--assignee @name] [--path DIR] \"free-form request\"")
	}

	cfg := config.Load()
	promptPath := cfg.AI.Prompt
	if promptPath == "" {
		resolved, err := aiPromptPath()
		if err != nil {
			outputError(err.Error())
		}
		promptPath = resolved
	}
	body := aiPromptFor(promptPath)

	database := load()
	defer closeDB(database)

	// An explicit --assignee becomes the fallback the spec inherits when the
	// harness names nobody; it is still validated against the roster later.
	defaultAssignee := aiprompt.DefaultAssignee
	if assignee != "" {
		defaultAssignee = assignee
		if !strings.HasPrefix(defaultAssignee, "@") {
			defaultAssignee = "@" + defaultAssignee
		}
	}

	// ai.model may be a category name (`dumb`, `free`, ...) from the operator's
	// Pi catalog; resolve it to the physical model before the harness sees it.
	model, thinking := aiResolveModel(cfg.AI.Model, cfg.AI.Thinking)

	job := aiJob{
		Harness:         cfg.AI.HarnessName(),
		Model:           model,
		Thinking:        thinking,
		Notify:          cfg.AI.NotifyEnabled(),
		Path:            path,
		Status:          status,
		DefaultAssignee: defaultAssignee,
	}
	job.Workdir, job.Projects, job.Assignees = aiState(database, project)
	job.Container = cfg.AI.ContainerName()
	job.Focus = cfg.AI.FocusEnabled()
	job.Prompt = aiprompt.Render(body, aiprompt.Context{
		Text:            text,
		Projects:        aiProjects(database),
		Assignees:       job.Assignees,
		Workdir:         job.Workdir,
		ProjectHint:     project,
		DefaultAssignee: defaultAssignee,
	})

	if wait {
		result := runAIJob(job)
		if result.Err != nil {
			outputError(result.Err.Error())
		}
		outputJSON(aiResult{
			OK:         true,
			Harness:    job.Harness,
			DurationMS: result.DurationMS,
			Spec:       result.Spec,
			Task:       result.Task,
		})
		return
	}

	jobPath, logPath, err := writeAIJob(job)
	if err != nil {
		outputError(err.Error())
	}
	pane := aiSpawnWorker(jobPath, logPath, job.Container)
	outputJSON(aiResult{
		OK:        true,
		Pending:   true,
		Job:       jobPath,
		Log:       logPath,
		Harness:   job.Harness,
		Container: job.Container,
		Pane:      pane,
	})
}

// focusPane brings the run's pane to the front when it is configured to. It is
// best effort: a closed multiplexer must not fail the command.
func focusPane(job aiJob) {
	if !job.Focus {
		return
	}
	_ = aiFocus()
}

// aiJobResult is the in-process outcome of a job: either a created task or the
// error that stopped it.
type aiJobResult struct {
	Spec       *ai.Spec
	Task       *model.Task
	Err        error
	DurationMS int64
}

// runAIJob runs the harness, validates the spec and inserts the task. The
// harness proposes, tsk writes: no harness answer reaches SQLite unchecked.
func runAIJob(job aiJob) aiJobResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	defer cancel()

	spec, _, err := aiRun(ctx, ai.Request{
		Harness:  job.Harness,
		Model:    job.Model,
		Thinking: job.Thinking,
		Prompt:   job.Prompt,
		Cwd:      job.Workdir,
	})
	if err != nil {
		focusPane(job)
		_ = aiNotify(job.Notify, "tsk add --ai failed", err.Error())
		return aiJobResult{Err: err}
	}
	if err := spec.Validate(ai.Validation{
		Projects:        job.Projects,
		Assignees:       job.Assignees,
		DefaultAssignee: job.DefaultAssignee,
	}); err != nil {
		focusPane(job)
		_ = aiNotify(job.Notify, "tsk add --ai rejected the spec", err.Error())
		return aiJobResult{Spec: &spec, Err: err}
	}

	database := openDB()
	defer closeDB(database)
	task, err := database.CreateTaskWithPath(
		spec.Project, spec.Title, spec.Description, spec.Assignee,
		spec.Priority, job.Status, spec.Estimate, spec.Tags, job.Path,
	)
	if err != nil {
		_ = aiNotify(job.Notify, "tsk add --ai failed", err.Error())
		return aiJobResult{Spec: &spec, Err: err}
	}
	focusPane(job)
	_ = aiNotify(job.Notify, "tsk: "+spec.Title, "#"+strconv.FormatInt(task.ID, 10)+" "+task.ProjectName)
	return aiJobResult{Spec: &spec, Task: task, DurationMS: time.Since(start).Milliseconds()}
}

// cmdAIWorker is the hidden worker behind a background run. It reads the job
// file, runs it and prints the result JSON, which the parent redirected to the
// log.
func cmdAIWorker(args []string) {
	var jobPath string
	s := newFlagScanner(args, 0)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		if flag == "--job" {
			jobPath, _ = s.next()
		}
	}
	if jobPath == "" {
		outputError("usage: tsk __ai-worker --job FILE")
	}
	job, err := readAIJob(jobPath)
	if err != nil {
		outputError(err.Error())
	}
	result := runAIJob(job)
	if result.Err != nil {
		outputError(result.Err.Error())
	}
	outputJSON(aiResult{
		OK:         true,
		Job:        jobPath,
		Harness:    job.Harness,
		DurationMS: result.DurationMS,
		Spec:       result.Spec,
		Task:       result.Task,
	})
}

// readAIJob decodes the job file written by cmdAddAI.
func readAIJob(path string) (aiJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return aiJob{}, err
	}
	var job aiJob
	if err := json.Unmarshal(data, &job); err != nil {
		return aiJob{}, err
	}
	return job, nil
}

// aiState resolves the working directory, the project names and the assignees
// in use. The prompt carries them, so the harness never runs tsk to read state.
func aiState(database *db.DB, project string) (workdir string, projects, assignees []string) {
	all := aiProjects(database)
	projects = make([]string, 0, len(all))
	for name := range all {
		projects = append(projects, name)
	}
	workdir = aiWorkdir(database, project)

	seen := map[string]bool{aiprompt.DefaultAssignee: true}
	assignees = []string{aiprompt.DefaultAssignee}
	for _, task := range aiTasks(database) {
		if task.Assignee == "" || seen[task.Assignee] {
			continue
		}
		seen[task.Assignee] = true
		assignees = append(assignees, task.Assignee)
	}
	sort.Strings(assignees)
	return workdir, projects, assignees
}

// aiProjects maps every active project to its workflow.
func aiProjects(database *db.DB) map[string][]string {
	projects := map[string][]string{}
	list, err := database.ListProjects()
	if err != nil {
		return projects
	}
	for _, p := range list {
		projects[p.Name] = p.Workflow
	}
	return projects
}

// aiTasks is the task set used to discover the assignees in use.
func aiTasks(database *db.DB) []model.Task {
	tasks, err := database.ListTasks("", "", "")
	if err != nil {
		return nil
	}
	return tasks
}

// getwd is the working-directory probe; a variable so its failure branch is
// reachable in a test.
var getwd = os.Getwd

// aiWorkdir is the directory the harness explores: the project path, then the
// launch directory. The task path is not known before the task exists.
func aiWorkdir(database *db.DB, project string) string {
	if project != "" {
		if p, err := database.GetProject(project); err == nil {
			if dir := model.EffectiveDir("", p.Path); dir != "" {
				return dir
			}
		}
	}
	wd, err := getwd()
	if err != nil {
		return "."
	}
	return wd
}

// writeAIJob writes the job next to the log, both under the tsk state
// directory, so a background run leaves a readable trace.
func writeAIJob(job aiJob) (jobPath, logPath string, err error) {
	dir, err := aiStateDir()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	stamp := time.Now().Format("20060102-150405")
	jobPath = filepath.Join(dir, stamp+"-job.json")
	logPath = filepath.Join(dir, stamp+".log")
	// The job struct is JSON-safe by construction: the marshal cannot fail.
	data, _ := json.MarshalIndent(job, "", "  ")
	if err := os.WriteFile(jobPath, data, 0o600); err != nil {
		return "", "", err
	}
	return jobPath, logPath, nil
}

// spawnAIWorker starts the worker where the run stays observable after the
// shell dies: a new mux tab in the project's workspace when a multiplexer is
// present, otherwise a plain detached process with a log file. It reports
// whether a pane was opened.
func spawnAIWorker(jobPath, logPath, container string) bool {
	if hasMultiplexer() {
		if err := spawnInMux(container, jobPath); err == nil {
			return true
		}
	}
	return spawnDetached(logPath, jobPath) == nil
}

// hasMultiplexer reports whether mux can present a pane: the binary must exist
// and one of its backends must be live in this environment.
func hasMultiplexer() bool {
	if _, err := aiLookPath("mux"); err != nil {
		return false
	}
	for _, key := range []string{"HERDR_ENV", "TUIOS_ENV", "TMUX", "KITTY_LISTEN_ON", "WEZTERM_PANE"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// spawnInMux opens a new tab in the mux workspace and runs the worker there
// without focusing it. The pane keeps the worker's output, so reattaching shows
// the run and its result.
func spawnInMux(container, jobPath string) error {
	exe, err := executablePath()
	if err != nil {
		return err
	}
	argv := []string{
		"--container", container,
		"--layout", "tab",
		"--no-focus",
		"--label", "tsk-ai",
		"--", exe, "__ai-worker", "--job", jobPath,
	}
	return startProcess(exec.Command("mux", argv...))
}

// spawnDetached is the process boundary; a variable so tests assert the
// arguments without starting a real tsk.
var spawnDetached = defaultSpawnDetached

// defaultSpawnDetached is the production spawn the variable starts as.
func defaultSpawnDetached(logPath, jobPath string) error {
	return realSpawnDetached(logPath, jobPath)
}

// executablePath and startProcess are the two OS calls of a detached spawn.
// They are variables so the failure branches are reachable in a test.
var (
	executablePath = os.Executable
	startProcess   = func(cmd *exec.Cmd) error { return cmd.Start() }
)

func realSpawnDetached(logPath, jobPath string) error {
	exe, err := executablePath()
	if err != nil {
		return err
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(exe, "__ai-worker", "--job", jobPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := startProcess(cmd); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
