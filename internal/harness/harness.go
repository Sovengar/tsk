// Package harness detects the AI coding harnesses available on this machine,
// composes the handoff prompt for a task and launches the configured handoff
// command detached. It is shared by the TUI and the CLI; neither imports the
// other, and config/model stay below it.
package harness

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/sovengar/ai"

	"tsk/internal/config"
	"tsk/internal/model"
)

// Harness is one entry of the picker: a display name and the executable that
// runs it. It is the shared AI agent contract, not a tsk-only type.
type Harness = ai.Agent

// ModelOptions is the model/effort picker data for one harness.
type ModelOptions = ai.Options

// NoHandoffMessage is the pinned refusal shown when handoff.command is unset.
const NoHandoffMessage = "No handoff configured: set handoff.command in config.toml"

// NoHarnessesMessage is the pinned empty state of the picker, shared by the TUI
// overlay and the CLI.
const NoHarnessesMessage = "No harnesses found"

// ErrNoHandoff is returned when the handoff command is not configured.
//
//nolint:staticcheck // the message is pinned verbatim by behavior.feature
var ErrNoHandoff = errors.New(NoHandoffMessage)

// ErrMissingPromptPlaceholder is returned when the handoff command does not
// carry the required {{prompt_file}} placeholder.
var ErrMissingPromptPlaceholder = errors.New("handoff.command must contain {{prompt_file}}")

// Template placeholders understood by the handoff command. {{harness}} is the
// display Name (wrappers key on it, e.g. herdr --kind); {{harness_binary}} is
// the executable to run, so a config `binary` override is usable.
const (
	placeholderHarness       = "{{harness}}"
	placeholderHarnessBinary = "{{harness_binary}}"
	placeholderWorkingDir    = "{{working_dir}}"
	placeholderContainer     = "{{container}}"
	placeholderPrompt        = "{{prompt_file}}"
	placeholderModel         = "{{model}}"
	placeholderThinking      = "{{thinking}}"
	placeholderEffort        = "{{effort}}"
	placeholderSessionID     = "{{session_id}}"
	placeholderModelArgs     = "{{model_args}}"
)

// Template placeholders understood by the handoff prompt. Unlike the command
// template, the substituted values reach a file, not a shell, so they are
// inserted bare.
const (
	placeholderID       = "{{id}}"
	placeholderTitle    = "{{title}}"
	placeholderProject  = "{{project}}"
	placeholderStatuses = "{{statuses}}"
)

// DefaultPrompt is the built-in handoff prompt, used when [handoff] prompt is
// empty. It identifies the task and gives the concrete tsk CLI commands to read
// it and update its state; the description is not embedded so the harness
// always reads the current task.
const DefaultPrompt = "Run this task: Task #{{id}}: {{title}} ({{project}}). " +
	"Check it with `tsk show {{id}}`. " +
	"Update its state as you go with `tsk move {{id}} <status>`; valid statuses: {{statuses}}."

// lookPath probes the executable on PATH. It is a variable so tests can make
// detection deterministic without touching the real PATH.
var lookPath = exec.LookPath

// Detect returns the harnesses available on this machine: the built-in
// registry entries whose binary is on PATH, merged with the config-declared
// ones (additive). A config entry with the same display name replaces the
// detected one; declared entries are listed even when not on PATH. The result
// is sorted by display name.
func Detect(_ config.Config) []Harness {
	return ai.Detect(nil, lookPath)
}

// Configured returns the detected harnesses when the single handoff command is
// usable. It is the list shown by Ask AI and used by `tsk ask`.
func Configured(cfg config.Config) []Harness {
	if ValidateCommand(cfg.Handoff.Command) != nil {
		return nil
	}
	return Detect(cfg)
}

// ComposePrompt is the prompt handed to the harness. prompt is the [handoff]
// prompt template; empty selects DefaultPrompt. It identifies the task and
// gives the concrete tsk CLI commands to read it and update its state; the
// description is not embedded so the harness always reads the current task. The
// statuses are the task project's workflow. An empty workflow drops the state
// clause, so the prompt never ends with a dangling "valid statuses:".
func ComposePrompt(t model.Task, statuses []string, prompt string) string {
	if prompt == "" {
		prompt = DefaultPrompt
	}
	if len(statuses) == 0 {
		prompt = dropStatusesClause(prompt)
	} else {
		prompt = strings.ReplaceAll(prompt, placeholderStatuses, strings.Join(statuses, ", "))
	}
	return strings.NewReplacer(
		placeholderID, strconv.FormatInt(t.ID, 10),
		placeholderTitle, t.Title,
		placeholderProject, t.ProjectName,
	).Replace(prompt)
}

// dropStatusesClause removes the sentence carrying the {{statuses}}
// placeholder, so an empty workflow leaves no dangling "valid statuses:".
// Sentences are split on ". " (period+space), the template's own separator;
// dropping the last sentence would lose the prompt's final period, so it is
// restored when the prompt had one.
func dropStatusesClause(prompt string) string {
	if !strings.Contains(prompt, placeholderStatuses) {
		return prompt
	}
	endsPeriod := strings.HasSuffix(prompt, ".")
	sentences := strings.Split(prompt, ". ")
	kept := make([]string, 0, len(sentences))
	for _, s := range sentences {
		if strings.Contains(s, placeholderStatuses) {
			continue
		}
		kept = append(kept, s)
	}
	out := strings.Join(kept, ". ")
	if endsPeriod && !strings.HasSuffix(out, ".") {
		out += "."
	}
	return out
}

// ValidateCommand reports whether the handoff command can be used: it must be
// set and must carry the required {{prompt_file}} placeholder.
func ValidateCommand(command string) error {
	if command == "" {
		return ErrNoHandoff
	}
	if !strings.Contains(command, placeholderPrompt) {
		return ErrMissingPromptPlaceholder
	}
	return nil
}

// ExpandTemplate replaces the placeholders with shell-quoted values, so a
// template uses them bare (e.g. --working-dir {{working_dir}}). {{harness}} is the display name
// and {{harness_binary}} the executable. It refuses a command without the
// required {{prompt_file}} placeholder.
func ExpandTemplate(command string, h Harness, cwd, promptFile string) (string, error) {
	return ExpandTemplateWithOptions(command, h, cwd, promptFile, "", "")
}

// ExpandTemplateWithOptions expands command placeholders including model and
// effort controls. It has no session id; Execute uses ExpandTemplateWithSession.
func ExpandTemplateWithOptions(command string, h Harness, cwd, promptFile, modelName, thinking string) (string, error) {
	return ExpandTemplateWithSession(command, h, cwd, promptFile, modelName, thinking, "", "")
}

// ExpandTemplateWithSession expands the single handoff template. {{session_id}}
// is stable per task and used by mux to label a tab; {{effort}} is an alias for
// the existing {{thinking}} placeholder.
func ExpandTemplateWithSession(command string, h Harness, cwd, promptFile, modelName, thinking, sessionID, container string) (string, error) {
	if err := ValidateCommand(command); err != nil {
		return "", err
	}
	modelArgs := ModelArgs(h, modelName, thinking)
	// Keep old templates working. New configurations use {{model}}/{{effort}}
	// directly and let the ai binary build agent-specific flags.
	if modelArgs != "" && !strings.Contains(command, placeholderModelArgs) &&
		!strings.Contains(command, placeholderModel) && !strings.Contains(command, placeholderThinking) &&
		!strings.Contains(command, placeholderEffort) {
		command = injectModelArgs(command, modelArgs)
	}
	r := strings.NewReplacer(
		placeholderHarness, shellQuote(h.Name),
		placeholderHarnessBinary, shellQuote(h.Binary),
		placeholderWorkingDir, shellQuote(cwd),
		placeholderContainer, shellQuote(container),
		placeholderPrompt, shellQuote(promptFile),
		placeholderModel, shellQuote(modelName),
		placeholderThinking, shellQuote(thinking),
		placeholderEffort, shellQuote(thinking),
		placeholderSessionID, shellQuote(sessionID),
		placeholderModelArgs, modelArgs,
	)
	return r.Replace(command), nil
}

// ModelArgs returns shell-safe arguments understood by the selected harness.
// It is intentionally a string because handoff commands are POSIX shell
// templates and may run the harness inside a wrapper such as herdr.
func ModelArgs(h Harness, modelName, thinking string) string {
	return ai.ModelArgs(h, modelName, thinking)
}

// Options loads the local model registry used by the selected harness. A
// missing or malformed registry returns empty options so Ask AI remains usable
// with configured command templates and custom harnesses.
func Options(name string) ModelOptions {
	return ai.OptionsFor(name)
}

func injectModelArgs(command, modelArgs string) string {
	// Herdr and similar wrappers forward arguments after `--`; put the
	// harness flags immediately after that delimiter, before the prompt.
	if i := strings.Index(command, " -- "); i >= 0 {
		return command[:i+4] + modelArgs + " " + command[i+4:]
	}
	return command
}

// shellQuote wraps a value in single quotes, escaping embedded single quotes,
// so the value reaches the shell as one literal argument. It is byte-safe for
// non-ASCII user text.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// HandoffDir resolves the handoff working directory: handoff.cwd when set,
// otherwise the directory where tsk was launched.
func HandoffDir(cfg config.Config) string {
	if cfg.Handoff.WorkingDir != "" {
		return cfg.Handoff.WorkingDir
	}
	wd, err := getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ResolveWorkingDir selects the working directory for a handoff: the task's
// own path first, then the project path, then the configured handoff directory
// and finally ~/Tasks.
func ResolveWorkingDir(cfg config.Config, taskPath, projectPath string) string {
	return resolveWorkingDir(cfg, cfg.Handoff, taskPath, projectPath)
}

// ContainerName is the multiplexer workspace for a task. It is the project
// directory's base name (tsk, gitdash, ...), so tasks from the same repository
// share one workspace; it falls back to the project name when no path is set.
func ContainerName(task model.Task, workingDir string) string {
	if workingDir != "" {
		base := filepath.Base(filepath.Clean(workingDir))
		if base != "." && base != string(filepath.Separator) && base != "~" {
			return base
		}
	}
	return task.ProjectName
}

func resolveWorkingDir(cfg config.Config, handoff config.HandoffConfig, taskPath, projectPath string) string {
	if dir := model.ResolveDir(taskPath, projectPath); dir != "" {
		return dir
	}
	if handoff.WorkingDir != "" {
		return handoff.WorkingDir
	}
	return model.EffectiveDir("", "")
}

// getwd is the working directory probe; a variable so the error branch of
// HandoffDir is reachable in a test.
var getwd = os.Getwd

// Execute is the whole handoff: it validates the command, composes the prompt
// with the task project's statuses, writes it to a private temp file and
// launches the command detached. It returns as soon as the process starts
// (fire-and-forget).
func Execute(cfg config.Config, task model.Task, statuses []string, h Harness) error {
	return ExecuteWithOptions(cfg, task, statuses, h, cfg.Handoff.Model, cfg.Handoff.Thinking)
}

// ExecuteWithOptions performs a handoff with optional per-invocation model and
// thinking values. Empty values inherit the configured defaults.
func ExecuteWithOptions(cfg config.Config, task model.Task, statuses []string, h Harness, modelName, thinking string) error {
	handoff := cfg.Handoff
	if err := ValidateCommand(handoff.Command); err != nil {
		return err
	}
	promptPath, err := PromptFile(ComposePrompt(task, statuses, handoff.Prompt))
	if err != nil {
		return err
	}
	if modelName == "" {
		modelName = handoff.Model
	}
	if thinking == "" {
		thinking = handoff.Thinking
	}
	sessionID := fmt.Sprintf("tsk-%s-%d", task.ProjectName, task.ID)
	workingDir := resolveWorkingDir(cfg, handoff, task.Path, task.ProjectPath)
	container := ContainerName(task, workingDir)
	if err := LaunchWithSession(handoff.Command, h, workingDir, promptPath, modelName, thinking, sessionID, container); err != nil {
		// The harness never started, so the prompt file would be orphaned.
		_ = os.Remove(promptPath)
		return err
	}
	return nil
}

// Launch expands the template and starts /bin/sh -c <expanded> detached.
func Launch(command string, h Harness, cwd, promptFile string) error {
	return LaunchWithOptions(command, h, cwd, promptFile, "", "")
}

// LaunchWithOptions expands and starts a handoff command with model controls.
func LaunchWithOptions(command string, h Harness, cwd, promptFile, modelName, thinking string) error {
	return LaunchWithSession(command, h, cwd, promptFile, modelName, thinking, "", "")
}

// LaunchWithSession expands and starts the single handoff command with a
// session id available to the template.
func LaunchWithSession(command string, h Harness, cwd, promptFile, modelName, thinking, sessionID, container string) error {
	expanded, err := ExpandTemplateWithSession(command, h, cwd, promptFile, modelName, thinking, sessionID, container)
	if err != nil {
		return err
	}
	return runDetached("/bin/sh", []string{"-c", expanded}, cwd)
}

// runDetached starts the process. A variable so tests can assert the command,
// the arguments and the working directory without spawning anything.
var runDetached = realRunDetached

// realRunDetached runs the command in its own session with the null device as
// its standard streams, so it does not interfere with the TUI, does not receive
// Ctrl+C, and is reaped in the background (no zombie).
func realRunDetached(name string, args []string, dir string) error {
	if dir == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = home
		}
	}
	// The default directory may not exist yet: a handoff to ~/Tasks must not
	// fail on a fresh machine.
	if dir == model.EffectiveDir("", "") {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// createTemp is the temp-file factory; a variable so a test can force its error.
var createTemp = func(pattern string) (promptFile, error) {
	return os.CreateTemp("", pattern)
}

// promptFile is the minimum writePromptFile needs from a file. It is an
// interface so a test can pass a file whose write or close fails: those branches
// have no other path to be reached.
type promptFile interface {
	Write(p []byte) (int, error)
	Close() error
	Name() string
}

// PromptFile writes the prompt to a fresh temp file and returns its path. The
// file is deliberately never deleted by tsk: the harness may read it after tsk
// exits and the OS temp reaper owns it. os.CreateTemp already creates the file
// with mode 0600.
func PromptFile(content string) (string, error) {
	f, err := createTemp("tsk-prompt-*")
	if err != nil {
		return "", err
	}
	if err := writePromptFile(f, content); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// writePromptFile writes the content and closes the file, removing the file if
// anything fails (a half-written temp file is trash that stays forever).
func writePromptFile(f promptFile, content string) error {
	if _, err := f.Write([]byte(content)); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return nil
}
