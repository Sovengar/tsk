package tui

import (
	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

func (m *Model) openPriority(task *model.Task) tea.Cmd {
	if task == nil {
		return nil
	}
	m.priorityOpen = true
	m.priorityTaskID = task.ID
	m.priorityList = []int{model.PriorityNone, model.PriorityLow, model.PriorityMedium, model.PriorityHigh}
	m.priorityIdx = 0
	for i, p := range m.priorityList {
		if p == task.Priority {
			m.priorityIdx = i
		}
	}
	return nil
}

func (m *Model) closePriority() {
	m.priorityOpen = false
	m.priorityTaskID = 0
	m.priorityList = nil
	m.priorityIdx = 0
}

func (m Model) handlePriorityKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.closePriority()
		return m, nil
	case "j", "down":
		m.priorityIdx = cycleIndex(m.priorityIdx, len(m.priorityList), 1)
		return m, nil
	case "k", "up":
		m.priorityIdx = cycleIndex(m.priorityIdx, len(m.priorityList), -1)
		return m, nil
	case "enter":
		return m.applyPriority()
	}
	return m, nil
}

func (m Model) applyPriority() (tea.Model, tea.Cmd) {
	if len(m.priorityList) == 0 {
		return m, nil
	}
	idx := m.priorityIdx
	if idx < 0 || idx >= len(m.priorityList) {
		return m, nil
	}
	prio := m.priorityList[idx]
	taskID := m.priorityTaskID
	m.closePriority()
	m.detailOpen = false
	m.detailTask = nil
	m.detailComments = nil
	m.detailCommentSel = -1
	return m, m.taskActionCmd(taskID, func(id int64) (*model.Task, error) {
		return m.database.UpdateTask(id, map[string]any{"priority": prio})
	})
}

func (m *Model) renderPriorityModal(content string) string {
	m.priorityIdx = clampTo(m.priorityIdx, len(m.priorityList))
	lines := []string{""}
	for i, p := range m.priorityList {
		line := selectionPrefix(i == m.priorityIdx) + model.PriorityLabel(p)
		if i == m.priorityIdx {
			line = styleSelected.Render(line)
		}
		lines = append(lines, line)
	}
	totalWidth := modalWidthFor(40, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Priority ", lines, totalWidth), totalWidth, m.width)
}
