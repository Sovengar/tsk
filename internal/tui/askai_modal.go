package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tsk/internal/config"
	"tsk/internal/harness"
	"tsk/internal/model"
)

const (
	askAISeparator        = " ⟡ "
	askAIHarnessSeparator = "   ⟡   "
)

// detectHarnesses and executeHandoff are variables so tests can make the picker
// deterministic and assert the launch without spawning anything.
var (
	detectHarnesses           = harness.Detect
	loadAskAIOptions          = harness.Options
	executeHandoff            = harness.Execute
	executeHandoffWithOptions = harness.ExecuteWithOptions
)

// openAskAI opens the Ask AI picker for a task. It refuses with the pinned
// message when no handoff is configured (never opening a picker that can do
// nothing) and does nothing when there is no task.
func (m *Model) openAskAI(task *model.Task) tea.Cmd {
	if task == nil {
		return nil
	}
	// Validate the legacy global form here so its diagnostics remain stable.
	if m.config.Handoff.Command != "" {
		if err := harness.ValidateCommand(m.config.Handoff.Command); err != nil {
			return m.setToast(err.Error(), "error")
		}
	}
	m.askAIOpen = true
	m.askAIIndex = 0
	m.askAIFocus = 0
	m.askAISearchOpen = false
	m.askAISearch = ""
	m.askAIList = configuredAskAIHarnesses(m.config)
	if m.config.Handoff.Command == "" && len(m.askAIList) == 0 {
		m.askAIOpen = false
		return m.setToast(harness.NoHandoffMessage, "error")
	}
	m.askAITask = task
	m.refreshAskAIOptions()
	return nil
}

func configuredAskAIHarnesses(cfg config.Config) []harness.Harness {
	if harness.ValidateCommand(cfg.Handoff.Command) != nil {
		return nil
	}
	return detectHarnesses(cfg)
}

func (m *Model) refreshAskAIOptions() {
	if !inRange(m.askAIIndex, len(m.askAIList)) {
		m.askAIModels = nil
		m.askAIThinkingLevels = nil
		m.askAIModel = m.config.Handoff.Model
		m.askAIThinking = m.config.Handoff.Thinking
		return
	}
	handoff := m.config.Handoff
	options := loadAskAIOptions(m.askAIList[m.askAIIndex].Name)
	options.Models = includeConfiguredOption(options.Models, handoff.Model)
	options.Efforts = includeConfiguredOption(options.Efforts, handoff.Thinking)
	m.askAIModels = options.Models
	m.askAIThinkingLevels = options.Efforts
	m.askAIModelIdx = optionIndex(options.Models, handoff.Model)
	m.askAIThinkingIdx = optionIndex(options.Efforts, handoff.Thinking)
	if len(options.Models) > 0 {
		m.askAIModel = options.Models[m.askAIModelIdx]
	} else {
		m.askAIModel = handoff.Model
	}
	if len(options.Efforts) > 0 {
		m.askAIThinking = options.Efforts[m.askAIThinkingIdx]
	} else {
		m.askAIThinking = handoff.Thinking
	}
}

func optionIndex(options []string, selected string) int {
	for i, option := range options {
		if option == selected {
			return i
		}
	}
	return 0
}

func includeConfiguredOption(options []string, configured string) []string {
	if configured == "" || slices.Contains(options, configured) {
		return options
	}
	return append([]string{configured}, options...)
}

// The fields of the picker: harness, model and effort.
const (
	askAIFieldHarness = iota
	askAIFieldModel
	askAIFieldThinking
	askAIFieldCount
)

// handleAskAIKey processes the picker's keys: jk/↑↓ move the value of the
// focused field, h/l and ←→ move the focus between harness, model and effort,
// Tab cycles the effort, / opens the model picker (also Enter on the model),
// Enter hands off and Esc closes. The picker is a value list, not a two-step
// form: the arrows never move the focus on their own.
func (m Model) handleAskAIKey(key string) (tea.Model, tea.Cmd) {
	if m.askAISearchOpen {
		return m.handleAskAISearchKey(key)
	}
	switch key {
	case "esc":
		return m.askAIClose()
	case "ctrl+s":
		return m.askAIHandoff()
	case "h", "left":
		m.askAIFocus = cycleIndex(m.askAIFocus, askAIFieldCount, -1)
		return m, nil
	case "l":
		m.askAIFocus = cycleIndex(m.askAIFocus, askAIFieldCount, 1)
		return m, nil
	case "tab":
		m.moveAskAIThinking(1)
		return m, nil
	case "shift+tab":
		m.moveAskAIThinking(-1)
		return m, nil
	case "/":
		m.openAskAIModels()
		return m, nil
	case "j", "down":
		m.moveAskAIValue(1)
		return m, nil
	case "k", "up":
		m.moveAskAIValue(-1)
		return m, nil
	case "enter":
		return m.askAIHandoff()
	}
	return m, nil
}

// openAskAIModels opens the model picker when there is a model to choose.
func (m *Model) openAskAIModels() {
	if len(m.askAIModels) == 0 {
		return
	}
	m.askAISearchOpen = true
	m.askAISearch = ""
}

// handleAskAISearchKey edits the model picker: typing filters the models,
// jk/↑↓ move the choice and Enter or Esc closes the picker.
func (m Model) handleAskAISearchKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "enter":
		m.askAISearchOpen = false
		m.askAISearch = ""
		return m, nil
	case "backspace":
		if len(m.askAISearch) > 0 {
			m.askAISearch = m.askAISearch[:len(m.askAISearch)-1]
		}
		return m, nil
	case "j", "down":
		m.moveAskAIModel(1)
		return m, nil
	case "k", "up":
		m.moveAskAIModel(-1)
		return m, nil
	}
	if len(key) == 1 && key[0] >= 33 {
		m.askAISearch += key
	}
	return m, nil
}

// moveAskAIValue moves the value of the focused field.
func (m *Model) moveAskAIValue(delta int) {
	switch m.askAIFocus {
	case askAIFieldHarness:
		m.askAIIndex = cycleIndex(m.askAIIndex, len(m.askAIList), delta)
		m.refreshAskAIOptions()
	case askAIFieldModel:
		// Model changes happen only in the / picker.
	case askAIFieldThinking:
		m.moveAskAIThinking(delta)
	}
}

// askAIClose closes the picker.
func (m Model) askAIClose() (tea.Model, tea.Cmd) {
	m.askAIOpen = false
	m.askAITask = nil
	return m, nil
}

// askAIHandoff starts the handoff with the chosen harness, model and effort.
func (m Model) askAIHandoff() (tea.Model, tea.Cmd) {
	if !inRange(m.askAIIndex, len(m.askAIList)) || m.askAITask == nil {
		return m, nil
	}
	selected := m.askAIList[m.askAIIndex]
	task := *m.askAITask
	m.askAIOpen = false
	m.askAITask = nil
	return m, m.askLaunchCmd(selected, task, m.askAIModel, m.askAIThinking)
}

func (m *Model) moveAskAIModel(delta int) {
	indices := m.filteredAskAIModelIndices()
	if len(indices) == 0 {
		return
	}
	current := 0
	for i, index := range indices {
		if index == m.askAIModelIdx {
			current = i
			break
		}
	}
	selected := indices[cycleIndex(current, len(indices), delta)]
	m.askAIModelIdx = selected
	m.askAIModel = m.askAIModels[selected]
}

func (m *Model) moveAskAIThinking(delta int) {
	if len(m.askAIThinkingLevels) == 0 {
		return
	}
	m.askAIThinkingIdx = cycleIndex(m.askAIThinkingIdx, len(m.askAIThinkingLevels), delta)
	m.askAIThinking = m.askAIThinkingLevels[m.askAIThinkingIdx]
}

func (m *Model) filteredAskAIModelIndices() []int {
	needle := strings.ToLower(m.askAISearch)
	var indices []int
	for i, option := range m.askAIModels {
		if needle == "" || strings.Contains(strings.ToLower(option), needle) {
			indices = append(indices, i)
		}
	}
	return indices
}

// askLaunchCmd hands the task off in a command: the TUI never blocks on the
// harness, it only reports asynchronous launch success or failure. The prompt
// carries the task project's workflow so the harness knows its valid statuses.
func (m *Model) askLaunchCmd(h harness.Harness, task model.Task, modelName, thinking string) tea.Cmd {
	cfg := m.config
	handoff := cfg.Handoff
	statuses := m.workflowForProject(task.ProjectName)
	return func() tea.Msg {
		var err error
		if modelName == handoff.Model && thinking == handoff.Thinking {
			err = executeHandoff(cfg, task, statuses, h)
		} else {
			err = executeHandoffWithOptions(cfg, task, statuses, h, modelName, thinking)
		}
		if err != nil {
			return askFailedMsg{err: err}
		}
		return askLaunchedMsg{name: h.Name}
	}
}

// workflowForProject returns the workflow of the named project, or the merged
// workflow when it is not loaded (so the prompt still lists a usable set).
func (m *Model) workflowForProject(name string) []string {
	if p := m.projectByName(name); p != nil {
		return p.Workflow
	}
	return m.mergedWorkflow()
}

// renderAskAIModal draws the picker over content: the available harnesses with
// the cursor, or exactly "No harnesses found" when there are none.
func (m *Model) renderAskAIModal(content string) string {
	lines := []string{""}
	if len(m.askAIList) == 0 {
		lines = append(lines, "  "+harness.NoHarnessesMessage)
	} else {
		m.askAIIndex = clampTo(m.askAIIndex, len(m.askAIList))
		start, end := visibleRange(m.askAIIndex, len(m.askAIList), filterMaxVisibleOptions)
		for i := start; i < end; i++ {
			line := selectionPrefix(i == m.askAIIndex) + m.askAIList[i].Name
			if i == m.askAIIndex {
				line = m.askAISelectedRow()
			}
			lines = append(lines, line)
		}
		if len(m.askAIList) > filterMaxVisibleOptions {
			lines = append(lines, styleDim.Render(fmt.Sprintf("  %d/%d", m.askAIIndex+1, len(m.askAIList))))
		}
	}
	if m.askAISearchOpen {
		lines = append(lines, "")
		lines = append(lines, styleSelected.Render("  Model search: "+m.askAISearch+"_"))
		for _, i := range m.filteredAskAIModelIndices() {
			line := selectionPrefix(i == m.askAIModelIdx) + m.askAIModels[i]
			if i == m.askAIModelIdx {
				line = styleSelected.Render(line)
			}
			lines = append(lines, line)
		}
	}

	totalWidth := modalWidthFor(m.askAIModalWidth(), m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Ask AI ", lines, totalWidth), totalWidth, m.width)
}

// askAISelectedRow draws the selected harness with its model and effort. The
// focused field is highlighted and the others stay plain.
func (m *Model) askAISelectedRow() string {
	name := ""
	if inRange(m.askAIIndex, len(m.askAIList)) {
		name = m.askAIList[m.askAIIndex].Name
	}
	field := func(text string, index int) string {
		if m.askAIFocus != index {
			return text
		}
		return styleSelected.Render(text)
	}
	return selectionPrefix(true) + field(name, askAIFieldHarness) +
		askAIHarnessSeparator + field(m.askAIModel, askAIFieldModel) +
		askAISeparator + field(m.askAIThinking, askAIFieldThinking)
}

// askAIModalWidth leaves enough room for the selected harness, model and
// effort. Long model IDs get their full width on any terminal that permits it.
func (m *Model) askAIModalWidth() int {
	preferred := 72
	if inRange(m.askAIIndex, len(m.askAIList)) {
		row := selectionPrefix(true) + m.askAIList[m.askAIIndex].Name +
			askAIHarnessSeparator + m.askAIModel + askAISeparator + m.askAIThinking
		preferred = max(preferred, len(row)+4)
	}
	return preferred
}
