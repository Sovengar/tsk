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
	if err := ValidateCommand(command); err != nil {
		return "", err
	}
	r := strings.NewReplacer(
		placeholderHarness, shellQuote(h.Name),
		placeholderHarnessBinary, shellQuote(h.Binary),
		placeholderCwd, shellQuote(cwd),
		placeholderPrompt, shellQuote(promptFile),
	)
	return r.Replace(command), nil
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

// getwd is the working directory probe; a variable so the error branch of
// HandoffDir is reachable in a test.
var getwd = os.Getwd

// Execute is the whole handoff: it validates the command, composes the prompt
// with the task project's statuses, writes it to a private temp file and
// launches the command detached. It returns as soon as the process starts
// (fire-and-forget).
func Execute(cfg config.Config, task model.Task, statuses []string, h Harness) error {
	if err := ValidateCommand(cfg.Handoff.Command); err != nil {
		return err
	}
	promptPath, err := PromptFile(ComposePrompt(task, statuses, cfg.Handoff.Prompt))
	if err != nil {
		return err
	}
	if err := Launch(cfg.Handoff.Command, h, HandoffDir(cfg), promptPath); err != nil {
		// The harness never started, so the prompt file would be orphaned.
		_ = os.Remove(promptPath)
		return err
	}
	return nil
}

// Launch expands the template and starts /bin/sh -c <expanded> detached.
func Launch(command string, h Harness, cwd, promptFile string) error {
	expanded, err := ExpandTemplate(command, h, cwd, promptFile)
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
