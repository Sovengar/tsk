package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"tsk/internal/harness"
	"tsk/internal/model"
)

// detectHarnesses and executeHandoff are variables so tests can make the picker
// deterministic and assert the launch without spawning anything.
var (
	detectHarnesses = harness.Detect
	executeHandoff  = harness.Execute
)

// openAskAI opens the Ask AI picker for a task. It refuses with the pinned
// message when no handoff is configured (never opening a picker that can do
// nothing) and does nothing when there is no task.
func (m *Model) openAskAI(task *model.Task) tea.Cmd {
	if task == nil {
		return nil
	}
	if err := harness.ValidateCommand(m.config.Handoff.Command); err != nil {
		return m.setToast(err.Error(), "error")
	}
	m.askAIOpen = true
	m.askAIIndex = 0
	m.askAIList = detectHarnesses(m.config)
	m.askAITask = task
	return nil
}

// handleAskAIKey processes the picker's keys: move, hand off, close.
func (m Model) handleAskAIKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.askAIOpen = false
		m.askAITask = nil
		return m, nil
	case "j", "down":
		m.askAIIndex = cycleIndex(m.askAIIndex, len(m.askAIList), 1)
		return m, nil
	case "k", "up":
		m.askAIIndex = cycleIndex(m.askAIIndex, len(m.askAIList), -1)
		return m, nil
	case "enter":
		if !inRange(m.askAIIndex, len(m.askAIList)) || m.askAITask == nil {
			return m, nil
		}
		selected := m.askAIList[m.askAIIndex]
		task := *m.askAITask
		m.askAIOpen = false
		m.askAITask = nil
		return m, m.askLaunchCmd(selected, task)
	}
	return m, nil
}

// askLaunchCmd hands the task off in a command: the TUI never blocks on the
// harness, it only reports asynchronous launch success or failure. The prompt
// carries the task project's workflow so the harness knows its valid statuses.
func (m *Model) askLaunchCmd(h harness.Harness, task model.Task) tea.Cmd {
	cfg := m.config
	statuses := m.workflowForProject(task.ProjectName)
	return func() tea.Msg {
		if err := executeHandoff(cfg, task, statuses, h); err != nil {
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
				line = styleSelected.Render(line)
			}
			lines = append(lines, line)
		}
		if len(m.askAIList) > filterMaxVisibleOptions {
			lines = append(lines, styleDim.Render(fmt.Sprintf("  %d/%d", m.askAIIndex+1, len(m.askAIList))))
		}
	}

	totalWidth := modalWidthFor(40, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Ask AI ", lines, totalWidth), totalWidth, m.width)
}
