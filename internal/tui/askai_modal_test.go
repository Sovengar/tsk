package tui

import (
	"errors"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/harness"
	"tsk/internal/model"
)

// askTestHarnesses is the harness list the next askModel will expose. Tests
// are not parallel, so the askConfig -> askModel pairing is deterministic.
var askTestHarnesses []string

// askConfig builds a config with a usable handoff command and the given test
// harnesses. Harnesses are no longer configured in tsk; the test stubs
// detection instead.
func askConfig(harnesses ...string) config.Config {
	askTestHarnesses = append([]string(nil), harnesses...)
	cfg := config.Defaults()
	cfg.Handoff.Command = "true {{prompt_file}}"
	return cfg
}

// askModel is a test model with a custom config and a deterministic harness
// list, independent of the binaries installed on PATH.
func askModel(t *testing.T, cfg config.Config) *Model {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	names := append([]string(nil), askTestHarnesses...)
	orig := detectHarnesses
	detectHarnesses = func(config.Config) []harness.Harness {
		sort.Strings(names)
		out := make([]harness.Harness, 0, len(names))
		for _, name := range names {
			out = append(out, harness.Harness{Name: name, Binary: name})
		}
		return out
	}
	t.Cleanup(func() { detectHarnesses = orig })
	m := newTestModel(t)
	m.config = cfg
	return m
}

// stubExecute replaces the real handoff execution and records the call.
func stubExecute(t *testing.T) *handoffCall {
	t.Helper()
	call := &handoffCall{}
	orig := executeHandoff
	origOptions := executeHandoffWithOptions
	executeHandoff = func(_ config.Config, task model.Task, statuses []string, h harness.Harness) error {
		call.task = task
		call.statuses = statuses
		call.name = h.Name
		call.binary = h.Binary
		return call.err
	}
	executeHandoffWithOptions = func(_ config.Config, task model.Task, statuses []string, h harness.Harness, modelName, thinking string) error {
		call.task = task
		call.statuses = statuses
		call.name = h.Name
		call.binary = h.Binary
		call.model = modelName
		call.thinking = thinking
		return call.err
	}
	t.Cleanup(func() { executeHandoff, executeHandoffWithOptions = orig, origOptions })
	return call
}

type handoffCall struct {
	name     string
	binary   string
	task     model.Task
	statuses []string
	err      error
	model    string
	thinking string
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
	if !strings.Contains(out, "> opencode"+askAIHarnessSeparator) {
		t.Errorf("picker does not render the selected harness/model/effort row:\n%s", out)
	}
	for _, hint := range []string{"(j/k)", "(/: search)", "Tab: cycle effort", "Enter: launch"} {
		if strings.Contains(out, hint) {
			t.Errorf("picker must not render the %q keybind hint:\n%s", hint, out)
		}
	}
}

func TestAskAIModalExpandsForLongModelName(t *testing.T) {
	m := askModel(t, askConfig("codex"))
	m.width = 140
	m, _ = press(m, "a")
	m.askAIModel = "openai/gpt-5.6-super-long-model-name-for-regression-testing"
	m.askAIThinking = "medium"

	out := ansi.Strip(m.renderAskAIModal("base"))
	if !strings.Contains(out, m.askAIModel) {
		t.Errorf("long model name was truncated despite available width:\n%s", out)
	}
	if m.askAIModalWidth() <= 72 {
		t.Errorf("modal width = %d, want it to expand for the long model", m.askAIModalWidth())
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
		t.Error("enter on the harness must close the picker and hand off")
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

func TestAskAISelectsModelAndThinkingPerTask(t *testing.T) {
	call := stubExecute(t)
	origOptions := loadAskAIOptions
	loadAskAIOptions = func(string) harness.ModelOptions {
		return harness.ModelOptions{Models: []string{"model-a", "model-b"}, Efforts: []string{"low", "high"}}
	}
	t.Cleanup(func() { loadAskAIOptions = origOptions })
	m := askModel(t, askConfig("opencode"))
	m, _ = press(m, "a")
	// Focus the model (l), open its picker (/), move it (j), close it
	// (enter), cycle the effort (Tab) and hand off (ctrl+s).
	for _, key := range []string{"l", "/", "j", "enter", "tab"} {
		m, _ = press(m, key)
	}
	_, cmd := press(m, "ctrl+s")
	if _, ok := mustMsg(t, cmd).(askLaunchedMsg); !ok {
		t.Fatal("model/thinking selection must launch the handoff")
	}
	if call.model != "model-b" || call.thinking != "high" {
		t.Errorf("handoff options = model %q, thinking %q; want model-b/high", call.model, call.thinking)
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

	m, cmd := press(m, "ctrl+s")
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

	out := ansi.Strip(m.renderAskAIModal("base"))
	if !strings.Contains(out, "> aaa") {
		t.Errorf("the cursor row must carry the selection prefix:\n%s", out)
	}
	if strings.Contains(out, "> bbb") {
		t.Errorf("a non-selected row must not carry the selection prefix:\n%s", out)
	}
	// Only the focused field carries the cursor style.
	row := m.askAISelectedRow()
	if !strings.Contains(row, styleSelected.Render("aaa")) {
		t.Error("the focused field must be highlighted")
	}
	if m.askAIModel != "" && strings.Contains(row, styleSelected.Render(m.askAIModel)) {
		t.Error("a field without the focus must not be highlighted")
	}

	// Moving the focus highlights the model instead of the harness.
	m.askAIFocus = askAIFieldModel
	row = m.askAISelectedRow()
	if strings.Contains(row, styleSelected.Render("aaa")) {
		t.Error("the harness must lose the highlight when the focus leaves it")
	}
	if m.askAIModel != "" && !strings.Contains(row, styleSelected.Render(m.askAIModel)) {
		t.Error("the model must take the highlight when it gets the focus")
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
	for _, want := range []string{"value", "field", "effort", "models", "handoff", "close"} {
		if !strings.Contains(out, want) {
			t.Errorf("Ask AI keybinds are missing %q:\n%s", want, out)
		}
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

// stubAskAIOptions makes the model and effort options deterministic.
func stubAskAIOptions(t *testing.T, models, thinking []string) {
	t.Helper()
	orig := loadAskAIOptions
	loadAskAIOptions = func(string) harness.ModelOptions {
		return harness.ModelOptions{Models: models, Efforts: thinking}
	}
	t.Cleanup(func() { loadAskAIOptions = orig })
}

// The arrows move the value of the focused field, never the focus itself.
func TestAskAIArrowsMoveTheFocusedValue(t *testing.T) {
	stubAskAIOptions(t, []string{"model-a", "model-b"}, []string{"low", "high"})
	m := askModel(t, askConfig("aaa", "bbb"))

	m, _ = pressKeys(t, m, "a", "j")
	if m.askAIIndex != 1 {
		t.Fatalf("j left the harness index at %d, want 1", m.askAIIndex)
	}
	if m.askAIFocus != askAIFieldHarness {
		t.Fatalf("j moved the focus away from the harness: %d", m.askAIFocus)
	}
	m, _ = press(m, "k")
	if m.askAIIndex != 0 {
		t.Errorf("k left the harness index at %d, want 0", m.askAIIndex)
	}

	m.askAIFocus = askAIFieldModel
	m.askAIModel = "model-b"
	m, _ = pressKeys(t, m, "down", "up")
	if m.askAIModel != "model-b" {
		t.Errorf("down/up changed the model to %q; only / may change it", m.askAIModel)
	}

	m.askAIFocus = askAIFieldThinking
	m, _ = pressKeys(t, m, "down", "up")
	if m.askAIThinking != "low" {
		t.Errorf("down/up left the effort at %q, want low", m.askAIThinking)
	}
}

// h/l and ←/→ move the focus between harness, model and effort, wrapping.
func TestAskAIFocusMovesWithHL(t *testing.T) {
	m := askModel(t, askConfig("aaa"))
	m, _ = press(m, "a")

	m, _ = press(m, "l")
	if m.askAIFocus != askAIFieldModel {
		t.Fatalf("l left the focus at %d, want the model", m.askAIFocus)
	}
	m, _ = press(m, "right")
	if m.askAIFocus != askAIFieldModel {
		t.Errorf("right must be ignored, focus = %d", m.askAIFocus)
	}
	m, _ = press(m, "l")
	if m.askAIFocus != askAIFieldThinking {
		t.Errorf("l from the model must move to the effort, got %d", m.askAIFocus)
	}
	m, _ = press(m, "l")
	if m.askAIFocus != askAIFieldHarness {
		t.Errorf("l from the effort must wrap to the harness, got %d", m.askAIFocus)
	}
	m, _ = press(m, "left")
	if m.askAIFocus != askAIFieldThinking {
		t.Errorf("left from the harness must wrap to the effort, got %d", m.askAIFocus)
	}
	m, _ = press(m, "h")
	if m.askAIFocus != askAIFieldModel {
		t.Errorf("h left the focus at %d, want the model", m.askAIFocus)
	}
}

// Enter no longer opens the model picker; only / does.
func TestAskAIEnterOnModelDoesNotOpenModelPicker(t *testing.T) {
	stubAskAIOptions(t, []string{"model-a"}, nil)
	m := askModel(t, askConfig("aaa"))

	m, _ = pressKeys(t, m, "a", "l", "enter")
	if m.askAISearchOpen {
		t.Fatal("enter on the model must not open the model picker")
	}
	if m.askAIOpen {
		t.Error("enter on the model must hand off, closing the picker")
	}
}

// The model picker opens with /, filters by typing and closes with Esc.
func TestAskAIModelPickerKeys(t *testing.T) {
	stubAskAIOptions(t, []string{"model-a", "model-b"}, nil)
	m := askModel(t, askConfig("aaa"))

	m, _ = pressKeys(t, m, "a", "/")
	if !m.askAISearchOpen {
		t.Fatal("/ must open the model picker")
	}
	m, _ = pressKeys(t, m, "b", "j")
	if m.askAISearch != "b" {
		t.Errorf("typing did not filter the picker: %q", m.askAISearch)
	}
	if m.askAIModel != "model-b" {
		t.Errorf("the arrow on the filtered picker left model %q, want model-b", m.askAIModel)
	}
	m, _ = press(m, "backspace")
	if m.askAISearch != "" {
		t.Errorf("backspace left the filter at %q", m.askAISearch)
	}
	m, _ = press(m, "up")
	if m.askAIModel != "model-a" {
		t.Errorf("up left model %q, want model-a", m.askAIModel)
	}
	m, _ = press(m, "esc")
	if m.askAISearchOpen {
		t.Error("esc must close the model picker")
	}
}

// With no models the picker stays closed instead of opening on an empty list.
func TestAskAIModelPickerStaysClosedWithoutModels(t *testing.T) {
	stubAskAIOptions(t, nil, nil)
	m := askModel(t, askConfig("aaa"))

	m, _ = pressKeys(t, m, "a", "/")
	if m.askAISearchOpen {
		t.Error("/ must not open an empty model picker")
	}
	m, _ = pressKeys(t, m, "l", "enter")
	if m.askAISearchOpen {
		t.Error("enter on the model must not open an empty picker")
	}
	if m.askAIOpen {
		t.Error("enter on the model must hand off, closing the picker")
	}
}

// Tab cycles the effort forward and Shift+Tab backwards.
func TestAskAITabCyclesEffort(t *testing.T) {
	stubAskAIOptions(t, nil, []string{"low", "high"})
	m := askModel(t, askConfig("aaa"))

	m, _ = pressKeys(t, m, "a", "tab")
	if m.askAIThinking != "high" {
		t.Fatalf("Tab left the effort at %q, want high", m.askAIThinking)
	}
	m, _ = press(m, "shift+tab")
	if m.askAIThinking != "low" {
		t.Errorf("Shift+Tab left the effort at %q, want low", m.askAIThinking)
	}
}

// Enter hands off from the effort field too.
func TestAskAIEnterOnEffortHandsOff(t *testing.T) {
	call := stubExecute(t)
	m := askModel(t, askConfig("opencode"))
	m.askAIFocus = askAIFieldThinking

	m, cmd := pressKeys(t, m, "a")
	if cmd != nil {
		t.Fatal("opening the picker must not run anything")
	}
	_, cmd = press(m, "enter")
	if _, ok := mustMsg(t, cmd).(askLaunchedMsg); !ok {
		t.Fatal("enter on the effort must hand off")
	}
	if call.name != "opencode" {
		t.Errorf("handoff harness = %q, want opencode", call.name)
	}
}
