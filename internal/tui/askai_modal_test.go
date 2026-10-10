package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/harness"
	"tsk/internal/model"
)

// askConfig builds a config with a usable handoff command and the given
// declared harnesses.
func askConfig(harnesses ...string) config.Config {
	cfg := config.Defaults()
	cfg.Handoff.Command = "true {{prompt_file}}"
	for _, name := range harnesses {
		cfg.Harnesses = append(cfg.Harnesses, config.HarnessConfig{Name: name})
	}
	return cfg
}

// askModel is a test model with a custom config and an empty PATH, so only the
// config-declared harnesses are detected.
func askModel(t *testing.T, cfg config.Config) *Model {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	m := newTestModel(t)
	m.config = cfg
	return m
}

// stubExecute replaces the real handoff execution and records the call.
func stubExecute(t *testing.T) *handoffCall {
	t.Helper()
	call := &handoffCall{}
	orig := executeHandoff
	executeHandoff = func(_ config.Config, task model.Task, statuses []string, h harness.Harness) error {
		call.task = task
		call.statuses = statuses
		call.name = h.Name
		call.binary = h.Binary
		return call.err
	}
	t.Cleanup(func() { executeHandoff = orig })
	return call
}

type handoffCall struct {
	name     string
	binary   string
	task     model.Task
	statuses []string
	err      error
}

func askNames(list []harness.Harness) []string {
	out := make([]string, 0, len(list))
	for _, h := range list {
		out = append(out, h.Name)
	}
	return out
}

func TestAskAIOpensSortedOnFocusedTask(t *testing.T) {
	m := askModel(t, askConfig("zzz", "opencode", "aaa"))
	m, _ = press(m, "a")

	if !m.askAIOpen {
		t.Fatal("pressing a on a focused task must open the picker")
	}
	want := []string{"aaa", "opencode", "zzz"}
	if strings.Join(askNames(m.askAIList), ",") != strings.Join(want, ",") {
		t.Errorf("picker list = %v, want %v (sorted)", askNames(m.askAIList), want)
	}
}

func TestAskAIRendersHarnessNames(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")

	out := ansi.Strip(m.renderAskAIModal("base"))
	if !strings.Contains(out, "opencode") {
		t.Errorf("picker does not render opencode:\n%s", out)
	}
}

func TestAskAIListExcludesUnavailable(t *testing.T) {
	// Only opencode is declared/detected; claude is neither.
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")

	out := ansi.Strip(m.renderAskAIModal("base"))
	if strings.Contains(out, "claude") {
		t.Errorf("picker shows claude though it is not available:\n%s", out)
	}
}

func TestAskAIConfigDeclaredUnknownListed(t *testing.T) {
	m := askModel(t, askConfig("my-tool"))
	m, _ = press(m, "a")

	if got := askNames(m.askAIList); len(got) != 1 || got[0] != "my-tool" {
		t.Errorf("picker list = %v, want the config-declared my-tool", got)
	}
}

func TestAskAIConfigEntryOverridesDetection(t *testing.T) {
	cfg := config.Defaults()
	cfg.Handoff.Command = "true {{prompt_file}}"
	// A declared entry that is also in the built-in registry: with an empty
	// PATH detection is silent, so declare it and a second registry name to
	// prove dedupe is by display name.
	cfg.Harnesses = []config.HarnessConfig{{Name: "opencode", Binary: "opencode-custom"}}
	m := askModel(t, cfg)
	m, _ = press(m, "a")

	if got := askNames(m.askAIList); len(got) != 1 || got[0] != "opencode" {
		t.Errorf("picker list = %v, want a single opencode entry", got)
	}
}

func TestAskAIEmptyState(t *testing.T) {
	m := askModel(t, askConfig())
	m, _ = press(m, "a")

	if !m.askAIOpen {
		t.Fatal("the picker must open even with no harnesses")
	}
	out := ansi.Strip(m.renderAskAIModal("base"))
	if !strings.Contains(out, "No harnesses found") {
		t.Errorf("empty picker must show exactly 'No harnesses found':\n%s", out)
	}

	// Enter cannot launch anything.
	if _, cmd := press(m, "enter"); cmd != nil {
		t.Error("enter on an empty picker must not launch anything")
	}
}

func TestAskAINoTaskFocusedIsNoop(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m.tasks = nil
	m.filteredT = nil
	m.filterStatus = "nonexistent-status"

	next, cmd := press(m, "a")
	if next.askAIOpen {
		t.Error("with no task focused the picker must not open")
	}
	if cmd != nil {
		t.Error("with no task focused nothing must run")
	}
}

func TestAskAICancel(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")
	m, cmd := press(m, "esc")

	if m.askAIOpen {
		t.Error("esc must close the picker")
	}
	if cmd != nil {
		t.Error("cancel must not launch anything")
	}
}

func TestAskAINoHandoffConfigured(t *testing.T) {
	cfg := askConfig("opencode")
	cfg.Handoff.Command = ""
	m := askModel(t, cfg)

	next, _ := press(m, "a")
	if next.askAIOpen {
		t.Error("with no handoff configured the picker must not open")
	}
	if next.toast != harness.NoHandoffMessage {
		t.Errorf("toast = %q, want %q", next.toast, harness.NoHandoffMessage)
	}
}

func TestAskAIMissingPlaceholder(t *testing.T) {
	cfg := askConfig("opencode")
	cfg.Handoff.Command = "true"
	m := askModel(t, cfg)

	next, _ := press(m, "a")
	if next.askAIOpen {
		t.Error("with a command lacking the placeholder the picker must not open")
	}
	if !strings.Contains(next.toast, "{{prompt_file}}") {
		t.Errorf("toast = %q, want it to name {{prompt_file}}", next.toast)
	}
}

func TestAskAISelectLaunches(t *testing.T) {
	call := stubExecute(t)
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")

	// Capture the focused task before it is consumed.
	wantTaskID := m.askAITask.ID

	m, cmd := press(m, "enter")
	if m.askAIOpen {
		t.Error("selecting a harness must close the picker")
	}
	msg := mustMsg(t, cmd)
	if _, ok := msg.(askLaunchedMsg); !ok {
		t.Fatalf("msg = %T, want askLaunchedMsg", msg)
	}
	if call.name != "opencode" {
		t.Errorf("handoff harness = %q, want opencode", call.name)
	}
	if call.binary != "opencode" {
		t.Errorf("handoff harness binary = %q, want opencode (defaulted from the name)", call.binary)
	}
	if call.task.ID != wantTaskID {
		t.Errorf("handoff task = %d, want %d", call.task.ID, wantTaskID)
	}
	if len(call.statuses) == 0 {
		t.Error("handoff must carry the task project's workflow statuses")
	}
}

func TestWorkflowForProject(t *testing.T) {
	m := askModel(t, askConfig("opencode"))

	got := m.workflowForProject("web")
	want := []string{"todo", "doing", "done"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("workflowForProject(web) = %v, want %v", got, want)
	}
	if got := m.workflowForProject("unknown"); len(got) == 0 {
		t.Error("an unknown project must fall back to a merged workflow")
	}
}

func TestAskAILaunchErrorToast(t *testing.T) {
	call := stubExecute(t)
	call.err = errors.New("spawn boom")
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")

	m, cmd := press(m, "enter")
	msg := mustMsg(t, cmd)
	updated, toastCmd := m.Update(msg)

	next := modelFrom(t, updated)
	if next.toast != "spawn boom" {
		t.Errorf("toast = %q, want the launch error", next.toast)
	}
	if toastCmd == nil {
		t.Error("the error toast must schedule its expiry")
	}
}

// modelFrom normalizes the tea.Model Update returns into *Model.
func modelFrom(t *testing.T, m tea.Model) *Model {
	t.Helper()
	switch v := m.(type) {
	case *Model:
		return v
	case Model:
		return &v
	}
	t.Fatalf("unexpected model type %T", m)
	return nil
}

func TestAskAINavigation(t *testing.T) {
	// Three harnesses so the two directions differ at the same position
	// (with two, +1 and -1 from index 1 both land on 0 and a mutant survives).
	m := askModel(t, askConfig("aaa", "bbb", "ccc"))
	m, _ = press(m, "a")

	m, _ = press(m, "j")
	m, _ = press(m, "j")
	if m.askAIIndex != 2 {
		t.Fatalf("j j left index at %d, want 2", m.askAIIndex)
	}
	m, _ = press(m, "k")
	if m.askAIIndex != 1 {
		t.Errorf("k from 2 left index at %d, want 1", m.askAIIndex)
	}
}

func TestAskAISelectionPrefixMarksCursor(t *testing.T) {
	m := askModel(t, askConfig("aaa", "bbb"))
	m, _ = press(m, "a")

	raw := m.renderAskAIModal("base")
	out := ansi.Strip(raw)
	if !strings.Contains(out, "> aaa") {
		t.Errorf("the cursor row must carry the selection prefix:\n%s", out)
	}
	if strings.Contains(out, "> bbb") {
		t.Errorf("a non-selected row must not carry the selection prefix:\n%s", out)
	}
	// The cursor row is the styled one: without this the style branch's
	// negation is invisible once the ANSI codes are stripped.
	if !strings.Contains(raw, styleSelected.Render("> aaa")) {
		t.Error("the cursor row must be rendered with the selected style")
	}
}

func TestAskAIPaginationHiddenAtCap(t *testing.T) {
	// Exactly the visible cap: no counter (the `>` boundary, not `>=`).
	m := askModel(t, askConfig("h1", "h2", "h3", "h4", "h5", "h6"))
	m, _ = press(m, "a")

	out := ansi.Strip(m.renderAskAIModal("base"))
	if strings.Contains(out, "/6") {
		t.Errorf("at the visible cap there must be no counter:\n%s", out)
	}
	// All six are visible.
	for _, name := range []string{"h1", "h6"} {
		if !strings.Contains(out, name) {
			t.Errorf("%s is missing from the picker:\n%s", name, out)
		}
	}
}

func TestAskAIFromDetail(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	// Open the detail of the first task.
	m, _ = press(m, "enter")
	if !m.detailOpen {
		t.Fatal("detail should be open")
	}

	m, _ = press(m, "a")
	if !m.askAIOpen {
		t.Fatal("a from the detail must open the picker")
	}
	if !m.detailOpen {
		t.Error("the detail must stay open behind the picker")
	}

	// esc closes the picker, not the detail.
	m, _ = press(m, "esc")
	if m.askAIOpen {
		t.Error("esc must close the picker")
	}
	if !m.detailOpen {
		t.Error("the detail must remain open after closing the picker")
	}
}

func TestAskAIOverlayKind(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")
	if got := m.overlayKind(); got != overlayAskAI {
		t.Errorf("overlayKind = %s, want ask-ai", overlayName(got))
	}
}

func TestAskAIViewRendersOverlayAndTitle(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "opencode") {
		t.Errorf("view does not paint the picker:\n%s", out)
	}
	if !strings.Contains(out, "Keybinds · Ask AI") {
		t.Errorf("view does not switch the keybinds bar to Ask AI:\n%s", out)
	}
}

func TestAskAIUnrelatedKeyIsIgnored(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")

	next, cmd := press(m, "x")
	if !next.askAIOpen {
		t.Error("an unrelated key must not close the picker")
	}
	if cmd != nil {
		t.Error("an unrelated key must not run anything")
	}
}

func TestAskAIPaginationShowsCounter(t *testing.T) {
	m := askModel(t, askConfig("h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8"))
	m, _ = press(m, "a")

	out := ansi.Strip(m.renderAskAIModal("base"))
	if !strings.Contains(out, "  1/8") {
		t.Errorf("picker with more than the visible cap must show the counter:\n%s", out)
	}
}

func TestAskAILaunchedToast(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	updated, cmd := m.Update(askLaunchedMsg{name: "opencode"})

	next := modelFrom(t, updated)
	if !strings.Contains(next.toast, "opencode") {
		t.Errorf("toast = %q, want it to mention the harness", next.toast)
	}
	if cmd == nil {
		t.Error("the info toast must schedule its expiry")
	}
}

func TestAskAIFromKanban(t *testing.T) {
	m := askModel(t, askConfig("opencode"))
	m.currentView = viewKanban
	m.kanbanCol, m.kanbanRow = 0, 0
	if m.selectedTask() == nil {
		t.Skip("no task in the first kanban column")
	}

	m, _ = press(m, "a")
	if !m.askAIOpen {
		t.Error("a from the kanban must open the picker")
	}
}
