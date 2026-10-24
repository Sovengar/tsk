// Package harness detects the AI coding harnesses available on this machine,
// composes the handoff prompt for a task and launches the configured handoff
// command detached. It is shared by the TUI and the CLI; neither imports the
// other, and config/model stay below it.
package harness

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"tsk/internal/config"
	"tsk/internal/model"
)

// Harness is one entry of the picker: a display name and the executable that
// runs it.
type Harness struct {
	Name   string
	Binary string
}

// NoHandoffMessage is the pinned refusal shown when ai.ask.handoff.command is unset.
const NoHandoffMessage = "No handoff configured: set ai.ask.handoff.command in config.toml"

// NoHarnessesMessage is the pinned empty state of the picker, shared by the TUI
// overlay and the CLI.
const NoHarnessesMessage = "No harnesses found"

// ErrNoHandoff is returned when the handoff command is not configured.
//
//nolint:staticcheck // the message is pinned verbatim by behavior.feature
var ErrNoHandoff = errors.New(NoHandoffMessage)

// ErrMissingPromptPlaceholder is returned when the handoff command does not
// carry the required {{prompt_file}} placeholder.
var ErrMissingPromptPlaceholder = errors.New("ai.ask.handoff.command must contain {{prompt_file}}")

// Template placeholders understood by the handoff command. {{harness}} is the
// display Name (wrappers key on it, e.g. herdr --kind); {{harness_binary}} is
// the executable to run, so a config `binary` override is usable.
const (
	placeholderHarness       = "{{harness}}"
	placeholderHarnessBinary = "{{harness_binary}}"
	placeholderCwd           = "{{cwd}}"
	placeholderPrompt        = "{{prompt_file}}"
	placeholderModel         = "{{model}}"
	placeholderThinking      = "{{thinking}}"
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

// DefaultPrompt is the built-in handoff prompt, used when [ai.ask.handoff] prompt is
// empty. It identifies the task and gives the concrete tsk CLI commands to read
// it and update its state; the description is not embedded so the harness
// always reads the current task.
const DefaultPrompt = "Run this task: Task #{{id}}: {{title}} ({{project}}). " +
	"Check it with `tsk show {{id}}`. " +
	"Update its state as you go with `tsk move {{id}} <status>`; valid statuses: {{statuses}}."

// builtinRegistry maps best-known harness display names to their executable.
// Members of this list are listed only when the binary is on PATH.
var builtinRegistry = []Harness{
	{Name: "aider", Binary: "aider"},
	{Name: "claude", Binary: "claude"},
	{Name: "codex", Binary: "codex"},
	{Name: "gemini", Binary: "gemini"},
	{Name: "opencode", Binary: "opencode"},
	{Name: "pi", Binary: "pi"},
}

// lookPath probes the executable on PATH. It is a variable so tests can make
// detection deterministic without touching the real PATH.
var lookPath = exec.LookPath

// Detect returns the harnesses available on this machine: the built-in
// registry entries whose binary is on PATH, merged with the config-declared
// ones (additive). A config entry with the same display name replaces the
// detected one; declared entries are listed even when not on PATH. The result
// is sorted by display name.
func Detect(cfg config.Config) []Harness {
	return merge(detectedBuiltins(), declaredHarnesses(cfg.Harnesses))
}

// Configured returns detected/declared harnesses with a usable per-harness
// handoff command. It is the list shown by Ask AI and used by `tsk ask`.
func Configured(cfg config.Config) []Harness {
	all := Detect(cfg)
	out := make([]Harness, 0, len(all))
	for _, h := range all {
		if ValidateCommand(cfg.HandoffFor(h.Name).Command) == nil {
			out = append(out, h)
		}
	}
	return out
}

// detectedBuiltins is the registry filtered by PATH availability.
func detectedBuiltins() []Harness {
	out := make([]Harness, 0, len(builtinRegistry))
	for _, h := range builtinRegistry {
		if _, err := lookPath(h.Binary); err == nil {
			out = append(out, h)
		}
	}
	return out
}

// declaredHarnesses turns the config entries into harnesses, defaulting a
// missing binary to the display name.
func declaredHarnesses(entries []config.HarnessConfig) []Harness {
	out := make([]Harness, 0, len(entries))
	for _, e := range entries {
		binary := e.Binary
		if binary == "" {
			binary = e.Name
		}
		out = append(out, Harness{Name: e.Name, Binary: binary})
	}
	return out
}

// merge dedupes by display name (declared wins) and sorts alphabetically.
func merge(detected, declared []Harness) []Harness {
	// No capacity hint: len(detected)+len(declared) is a plain size hint whose
	// only mutant (lengths subtracted) cannot be observed by any test, so it
	// was an equivalent survivor.
	byName := make(map[string]Harness)
	for _, h := range detected {
		byName[h.Name] = h
	}
	for _, h := range declared {
		byName[h.Name] = h
	}
	out := make([]Harness, 0, len(byName))
	for _, h := range byName {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b Harness) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// ComposePrompt is the prompt handed to the harness. prompt is the [ai.ask.handoff]
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
// template uses them bare (e.g. --cwd {{cwd}}). {{harness}} is the display name
// and {{harness_binary}} the executable. It refuses a command without the
// required {{prompt_file}} placeholder.
func ExpandTemplate(command string, h Harness, cwd, promptFile string) (string, error) {
	return ExpandTemplateWithOptions(command, h, cwd, promptFile, "", "")
}

// ExpandTemplateWithOptions expands command placeholders including model
// selection controls.
func ExpandTemplateWithOptions(command string, h Harness, cwd, promptFile, modelName, thinking string) (string, error) {
	if err := ValidateCommand(command); err != nil {
		return "", err
	}
	modelArgs := ModelArgs(h, modelName, thinking)
	// Keep the old Herdr-style template working. New configurations should put
	// {{model_args}} exactly where the selected harness expects its arguments.
	if modelArgs != "" && !strings.Contains(command, placeholderModelArgs) &&
		!strings.Contains(command, placeholderModel) && !strings.Contains(command, placeholderThinking) {
		command = injectModelArgs(command, modelArgs)
	}
	r := strings.NewReplacer(
		placeholderHarness, shellQuote(h.Name),
		placeholderHarnessBinary, shellQuote(h.Binary),
		placeholderCwd, shellQuote(cwd),
		placeholderPrompt, shellQuote(promptFile),
		placeholderModel, shellQuote(modelName),
		placeholderThinking, shellQuote(thinking),
		placeholderModelArgs, modelArgs,
	)
	return r.Replace(command), nil
}

// ModelArgs returns shell-safe arguments understood by the selected harness.
// It is intentionally a string because handoff commands are POSIX shell
// templates and may run the harness inside a wrapper such as herdr.
func ModelArgs(h Harness, modelName, thinking string) string {
	if modelName == "" && thinking == "" {
		return ""
	}
	var args []string
	switch h.Name {
	case "pi":
		if modelName != "" {
			args = append(args, "--model", shellQuote(modelName))
		}
		if thinking != "" {
			args = append(args, "--thinking", shellQuote(thinking))
		}
	case "codex":
		if modelName != "" {
			args = append(args, "--model", shellQuote(modelName))
		}
		if thinking != "" {
			args = append(args, "-c", shellQuote("model_reasoning_effort="+thinking))
		}
	case "opencode":
		if modelName != "" {
			args = append(args, "--model", shellQuote(modelName))
		}
		if thinking != "" {
			args = append(args, "--variant", shellQuote(thinking))
		}
	}
	return strings.Join(args, " ")
}

func injectModelArgs(command, modelArgs string) string {
	// Herdr and similar wrappers forward arguments after `--`; put the
	// harness flags immediately after that delimiter, before the prompt.
	if i := strings.Index(command, " -- "); i >= 0 {
		return command[:i+4] + modelArgs + " " + command[i+4:]
	}
	return command
}

// handoffFor keeps the old global config working while allowing each harness
// to own a completely independent command and set of defaults.
func handoffFor(cfg config.Config, h Harness) config.HandoffConfig {
	return cfg.HandoffFor(h.Name)
}

// shellQuote wraps a value in single quotes, escaping embedded single quotes,
// so the value reaches the shell as one literal argument. It is byte-safe for
// non-ASCII user text.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// HandoffDir resolves the handoff working directory: ai.ask.handoff.cwd when set,
// otherwise the directory where tsk was launched.
func HandoffDir(cfg config.Config) string {
	if cfg.Handoff.CWD != "" {
		return cfg.Handoff.CWD
	}
	wd, err := getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ResolveCWD selects the project path for a handoff, falling back to the
// configured directory and finally the user's home shorthand.
func ResolveCWD(cfg config.Config, projectPath string) string {
	return resolveCWD(cfg, cfg.Handoff, projectPath)
}

func resolveCWD(cfg config.Config, handoff config.HandoffConfig, projectPath string) string {
	if projectPath != "" {
		return projectPath
	}
	if handoff.CWD != "" {
		return handoff.CWD
	}
	return "~"
}

// getwd is the working directory probe; a variable so the error branch of
// HandoffDir is reachable in a test.
var getwd = os.Getwd

// Execute is the whole handoff: it validates the command, composes the prompt
// with the task project's statuses, writes it to a private temp file and
// launches the command detached. It returns as soon as the process starts
// (fire-and-forget).
func Execute(cfg config.Config, task model.Task, statuses []string, h Harness) error {
	handoff := handoffFor(cfg, h)
	return ExecuteWithOptions(cfg, task, statuses, h, handoff.Model, handoff.Thinking)
}

// ExecuteWithOptions performs a handoff with optional per-invocation model and
// thinking values. Empty values inherit the configured defaults.
func ExecuteWithOptions(cfg config.Config, task model.Task, statuses []string, h Harness, modelName, thinking string) error {
	handoff := handoffFor(cfg, h)
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
	if err := LaunchWithOptions(handoff.Command, h, resolveCWD(cfg, handoff, task.ProjectPath), promptPath, modelName, thinking); err != nil {
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
	expanded, err := ExpandTemplateWithOptions(command, h, cwd, promptFile, modelName, thinking)
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
