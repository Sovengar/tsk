// Package aiprompt loads the prompt used by `tsk add --ai` and injects the live
// tsk state into it.
//
// The prompt is a plain markdown file: ai.md, next to config.toml. Its settings
// (harness, model, thinking, notify) live in the [ai] section of config.toml, so
// there is exactly one configuration file for the whole command.
//
// A missing, unreadable or empty ai.md is not an error: the embedded default
// below is used, so the command works on a machine that never wrote the file.
package aiprompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the name of the prompt file inside the tsk config directory.
const FileName = "ai.md"

// Default is the embedded prompt, used when ai.md does not exist. It keeps the
// rules of the interactive /tsk prompt (short title, project by best fit,
// @sovengar fallback) and adds the JSON contract the native path needs.
const Default = `You create exactly one task for the tsk task manager and reply with JSON.

# Request

{{text}}

# tsk state (already fetched, do not run tsk to read it)

Projects (name: workflow): {{projects}}
Assignees in use: {{assignees}}
Tags in use: {{tags}}
Working directory: {{workdir}}

# Rules

1. Pick the project that fits best. Use {{project_hint}} when it is given and
   valid, otherwise choose from the list. Never invent a project.
2. Pick the assignee of the request when it is named and known, otherwise use
   {{default_assignee}}.
3. Title: imperative, at most 50 columns. No project, no assignee, no prefixes
   (WIP, TODO, dates), no trailing period. The detail belongs to the
   description.
4. Description: investigate the working directory before you write it. Read the
   files, git log and any note that explains the request. Write concrete steps,
   paths and acceptance criteria, in the same language as the request. An empty
   description is a failure.
5. Tags: reuse tags from the list; add a new one only when none fits.
6. Reply with ONE JSON object and nothing else:

{"title":"...","project":"...","assignee":"@name","description":"...","tags":["..."],"priority":0,"estimate":0}

priority is 0..3 (none, low, medium, high). estimate is days, 0 when unknown.
`

// Path resolves ai.md: TSK_AI_PROMPT, then the XDG config directory.
func Path() (string, error) {
	if p := os.Getenv("TSK_AI_PROMPT"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "tsk", FileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tsk", FileName), nil
}

// Load returns the prompt body at path. A missing, unreadable or blank file
// falls back to Default: the command must stay usable without ai.md.
func Load(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return Default
	}
	body := strings.TrimSpace(string(data))
	if body == "" {
		return Default
	}
	return body
}

// Context is the live tsk state injected into the prompt, so the headless
// harness never has to run tsk to read it.
type Context struct {
	// Text is the free-form request of the user.
	Text string
	// Projects maps every active project name to its workflow.
	Projects map[string][]string
	// Assignees and Tags are the values already in use.
	Assignees []string
	Tags      []string
	// Workdir is where the harness explores before writing the description.
	Workdir string
	// ProjectHint is the validated `--project` value, empty when absent.
	ProjectHint string
	// DefaultAssignee is used when the request names nobody.
	DefaultAssignee string
}

// Placeholders understood by the prompt body.
const (
	placeholderText            = "{{text}}"
	placeholderProjects        = "{{projects}}"
	placeholderAssignees       = "{{assignees}}"
	placeholderTags            = "{{tags}}"
	placeholderWorkdir         = "{{workdir}}"
	placeholderProjectHint     = "{{project_hint}}"
	placeholderDefaultAssignee = "{{default_assignee}}"
)

// DefaultAssignee is the fallback assignee of the interactive /tsk prompt.
const DefaultAssignee = "@sovengar"

// Render substitutes the tsk state into the prompt body. Unknown placeholders
// stay untouched: a user prompt may carry its own {{...}} syntax.
func Render(body string, ctx Context) string {
	projectHint := ctx.ProjectHint
	if projectHint == "" {
		projectHint = "(none given: choose it)"
	}
	defaultAssignee := ctx.DefaultAssignee
	if defaultAssignee == "" {
		defaultAssignee = DefaultAssignee
	}
	workdir := ctx.Workdir
	if workdir == "" {
		workdir = "."
	}
	return strings.NewReplacer(
		placeholderText, ctx.Text,
		placeholderProjects, formatProjects(ctx.Projects),
		placeholderAssignees, orNone(strings.Join(ctx.Assignees, ", ")),
		placeholderTags, orNone(strings.Join(ctx.Tags, ", ")),
		placeholderWorkdir, workdir,
		placeholderProjectHint, projectHint,
		placeholderDefaultAssignee, defaultAssignee,
	).Replace(body)
}

// formatProjects renders "name (a, b, c)" per project, sorted by name so the
// prompt is stable between runs.
func formatProjects(projects map[string][]string) string {
	if len(projects) == 0 {
		return "none"
	}
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sortStrings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s (%s)", name, strings.Join(projects[name], ", ")))
	}
	return strings.Join(parts, "; ")
}

func orNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// sortStrings is a small insertion sort: the project list is short and this
// keeps the package free of a sort import that a mutation run would chase.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
