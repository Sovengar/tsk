// Package ai turns a free-form request into a tsk task: it runs the configured
// agent harness headless, extracts the JSON spec from its answer and validates
// it against the live database before the CLI writes anything.
//
// The agent only proposes; tsk validates and inserts. That split is what makes
// the feature safe: a harness that hallucinates a project name never reaches
// SQLite.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxTitleRunes is the hard limit of a title. The interactive prompt targets
// 50 columns; above this the TUI truncates with "..", so it is a rejection.
const MaxTitleRunes = 70

// Spec is the JSON contract the harness must answer with. It mirrors the
// fields of `tsk add`, so the CLI creates the task with the same call the user
// would have typed.
type Spec struct {
	Title       string   `json:"title"`
	Project     string   `json:"project"`
	Assignee    string   `json:"assignee"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Priority    int      `json:"priority"`
	Estimate    float64  `json:"estimate"`
	Path        string   `json:"path"`
	Status      string   `json:"status"`
}

// Validation is the live state a spec is checked against. A spec is never
// trusted: every name must already exist, exactly like `tsk add --project`.
type Validation struct {
	// Projects are the active project names.
	Projects []string
	// Assignees are the values in use, plus DefaultAssignee.
	Assignees []string
	// DefaultAssignee is the fallback the spec may omit.
	DefaultAssignee string
}

// Errors returned by Validate. They are pinned as sentinels so tests and the
// CLI can tell a bad spec apart from a broken database.
var (
	ErrEmptyTitle        = errors.New("the task title is empty")
	ErrTitleTooLong      = fmt.Errorf("the task title is longer than %d columns", MaxTitleRunes)
	ErrUnknownProject    = errors.New("the task project does not exist")
	ErrUnknownAssignee   = errors.New("the task assignee does not exist")
	ErrEmptyDescription  = errors.New("the task description is empty")
	ErrNoJSONObject      = errors.New("the harness answer carries no JSON object")
	ErrHarnessNotStarted = errors.New("the harness could not be started")
)

// ParseSpec extracts the task spec from a harness answer. An answer may carry
// prose, a fenced code block or several JSON objects: the last complete object
// that decodes and has a title wins, because a harness that thinks out loud
// ends with its conclusion.
func ParseSpec(output string) (Spec, error) {
	for _, candidate := range jsonObjects(output) {
		var spec Spec
		if err := json.Unmarshal([]byte(candidate), &spec); err != nil {
			continue
		}
		if strings.TrimSpace(spec.Title) == "" {
			continue
		}
		return spec, nil
	}
	return Spec{}, ErrNoJSONObject
}

// jsonObjects returns every top-level {...} substring, last first. The scan is
// JSON-aware: braces inside strings and escaped quotes do not open or close an
// object, so a description carrying "}" still parses.
func jsonObjects(output string) []string {
	var found []string
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i, r := range output {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// Braces inside a string are literal.
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				found = append(found, output[start:i+1])
				start = -1
			}
		}
	}
	for i, j := 0, len(found)-1; i < j; i, j = i+1, j-1 {
		found[i], found[j] = found[j], found[i]
	}
	return found
}

// Validate checks a spec against the live state. It normalises the assignee
// with a leading "@" and trims the title, so a harness that answers
// "sovengar" or " title." still produces a usable task.
func (s *Spec) Validate(v Validation) error {
	s.Title = strings.TrimSpace(s.Title)
	s.Project = strings.TrimSpace(s.Project)
	s.Assignee = normalizeAssignee(strings.TrimSpace(s.Assignee), v.DefaultAssignee)
	s.Description = strings.TrimSpace(s.Description)

	if s.Title == "" {
		return ErrEmptyTitle
	}
	if utf8.RuneCountInString(s.Title) > MaxTitleRunes {
		return ErrTitleTooLong
	}
	if s.Description == "" {
		return ErrEmptyDescription
	}
	if !contains(v.Projects, s.Project) {
		return fmt.Errorf("%w: %s", ErrUnknownProject, s.Project)
	}
	if s.Assignee != "" && !contains(v.Assignees, s.Assignee) {
		return fmt.Errorf("%w: %s", ErrUnknownAssignee, s.Assignee)
	}
	return nil
}

// normalizeAssignee applies the "@" prefix and the default assignee, mirroring
// the interactive prompt: no name means DefaultAssignee.
func normalizeAssignee(assignee, fallback string) string {
	if assignee == "" {
		return fallback
	}
	if strings.HasPrefix(assignee, "@") {
		return assignee
	}
	return "@" + assignee
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// Request is one headless run: the harness, its optional model/effort and the
// prompt.
type Request struct {
	Harness  string
	Model    string
	Thinking string
	Prompt   string
	// Cwd is the directory the harness explores before it writes the
	// description. Empty inherits the process directory.
	Cwd string
}

// Run executes the harness headless through agentbridge and returns its
// combined output. agentbridge owns the agent-specific argv; tsk never builds
// an agent command line itself.
func Run(ctx context.Context, r Request) (string, error) {
	argv := []string{"exec", "--harness", r.Harness}
	if r.Model != "" {
		argv = append(argv, "--model", r.Model)
	}
	if r.Thinking != "" {
		argv = append(argv, "--effort", r.Thinking)
	}
	if r.Cwd != "" {
		argv = append(argv, "--cwd", r.Cwd)
	}
	argv = append(argv, "--prompt", r.Prompt)
	return runAgent(ctx, "agentbridge", argv)
}

// runAgent is the process boundary; a variable so tests assert the argv
// without spawning a real harness.
var runAgent = func(ctx context.Context, name string, argv []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, argv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %w: %s", ErrHarnessNotStarted, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// RunWithRepair runs the harness once and, when the answer carries no usable
// spec, retries once with the parse error appended. A harness that wrote prose
// instead of JSON gets one chance to correct itself; more than one makes the
// command slow for no gain.
func RunWithRepair(ctx context.Context, r Request) (Spec, string, error) {
	out, err := Run(ctx, r)
	if err != nil {
		return Spec{}, out, err
	}
	spec, parseErr := ParseSpec(out)
	if parseErr == nil {
		return spec, out, nil
	}
	repair := Request{
		Harness:  r.Harness,
		Model:    r.Model,
		Thinking: r.Thinking,
		Cwd:      r.Cwd,
		Prompt: r.Prompt + "\n\n# Repair\n\nYour previous answer was rejected: " +
			parseErr.Error() + ". Reply with ONE JSON object only, no prose.",
	}
	second, err := Run(ctx, repair)
	if err != nil {
		return Spec{}, second, err
	}
	spec, err = ParseSpec(second)
	return spec, second, err
}

// Notify sends the desktop notification that closes the background run. It is
// best effort: a machine without a notification daemon must not fail a task
// that was already created.
func Notify(enabled bool, summary, body string) error {
	if !enabled {
		return nil
	}
	return sendNotification("notify-send", "-a", "tsk", summary, body)
}

// sendNotification is the process boundary of Notify; a variable for tests.
var sendNotification = func(name string, argv ...string) error {
	cmd := exec.Command(name, argv...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// StateDir is where a background run writes its log, under XDG state.
func StateDir() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "tsk", "ai"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "tsk", "ai"), nil
}
