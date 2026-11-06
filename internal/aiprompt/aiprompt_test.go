package aiprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileUsesDefault(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "absent.md"))
	if got != Default {
		t.Fatal("want the embedded default")
	}
}

func TestLoadReadsTheBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("\n  Create it.  \n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := Load(path); got != "Create it." {
		t.Fatalf("body = %q", got)
	}
}

func TestLoadBlankFileUsesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("   \n\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := Load(path); got != Default {
		t.Fatalf("body = %q, want the embedded default", got)
	}
}

func TestLoadAcceptsMarkdownAsIs(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	body := "# Task\n\nUse {{text}} and {{projects}}.\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := Load(path); !strings.Contains(got, "{{text}}") {
		t.Fatalf("body = %q", got)
	}
}

func TestRenderSubstitutesEveryPlaceholder(t *testing.T) {
	body := "text={{text}} projects={{projects}} assignees={{assignees}} tags={{tags}} " +
		"workdir={{workdir}} hint={{project_hint}} assignee={{default_assignee}} keep={{unknown}}"
	got := Render(body, Context{
		Text:            "revisar recovery",
		Projects:        map[string][]string{"voice-gate": {"backlog", "doing", "done"}},
		Assignees:       []string{"@sovengar"},
		Tags:            []string{"voice", "infra"},
		Workdir:         "/work/voice-gate",
		ProjectHint:     "voice-gate",
		DefaultAssignee: "@ana",
	})
	want := "text=revisar recovery projects=voice-gate (backlog, doing, done) " +
		"assignees=@sovengar tags=voice, infra workdir=/work/voice-gate " +
		"hint=voice-gate assignee=@ana keep={{unknown}}"
	if got != want {
		t.Fatalf("rendered = %q\nwant      = %q", got, want)
	}
}

func TestRenderDefaultsEmptyContext(t *testing.T) {
	body := "{{projects}}|{{assignees}}|{{tags}}|{{workdir}}|{{project_hint}}|{{default_assignee}}"
	got := Render(body, Context{})
	want := "none|none|none|.|(none given: choose it)|" + DefaultAssignee
	if got != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func TestFormatProjectsSortsAThreeEntryMap(t *testing.T) {
	got := formatProjects(map[string][]string{
		"zeta":  {"todo"},
		"mu":    {"todo"},
		"alpha": {"todo"},
	})
	want := "alpha (todo); mu (todo); zeta (todo)"
	if got != want {
		t.Fatalf("projects = %q, want %q", got, want)
	}
}

func TestFormatProjectsSortsByName(t *testing.T) {
	got := formatProjects(map[string][]string{
		"zeta":  {"todo"},
		"alpha": {"backlog", "done"},
	})
	want := "alpha (backlog, done); zeta (todo)"
	if got != want {
		t.Fatalf("projects = %q, want %q", got, want)
	}
}

func TestPathHonoursEnvironment(t *testing.T) {
	t.Setenv("TSK_AI_PROMPT", "/custom/ai.md")
	got, err := Path()
	if err != nil || got != "/custom/ai.md" {
		t.Fatalf("path = %q, err = %v", got, err)
	}

	t.Setenv("TSK_AI_PROMPT", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	got, err = Path()
	if err != nil || got != "/xdg/tsk/ai.md" {
		t.Fatalf("path = %q, err = %v", got, err)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got, err = Path()
	if err != nil || !strings.HasSuffix(got, "/.config/tsk/ai.md") {
		t.Fatalf("path = %q, err = %v", got, err)
	}

	t.Setenv("HOME", "")
	if _, err := Path(); err == nil {
		t.Fatal("an unresolved home directory must be an error")
	}
}

func TestDefaultPromptCarriesTheContract(t *testing.T) {
	for _, needle := range []string{
		placeholderText, placeholderProjects, placeholderAssignees,
		placeholderTags, placeholderWorkdir, placeholderProjectHint,
		placeholderDefaultAssignee, "50 columns", "ONE JSON object",
	} {
		if !strings.Contains(Default, needle) {
			t.Fatalf("default prompt is missing %q", needle)
		}
	}
}
