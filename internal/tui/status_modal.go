package tui

import (
	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// openStatus opens the status overlay for a task, offering the statuses of the
// task's own project: the next workflow status first. It is the "m" key.
func (m *Model) openStatus(task *model.Task) tea.Cmd {
	if task == nil {
		return nil
	}
	p := m.projectByName(task.ProjectName)
	if p == nil {
		return nil
	}
	return m.openStatusForProject(task, 0, "", p.Workflow)
}

// openStatusForProject opens the status overlay against another project's
// workflow. projectID > 0 marks a cross-project move coming from "M"; 0 keeps
// the task in its current project.
func (m *Model) openStatusForProject(task *model.Task, projectID int64, projectName string, workflow []string) tea.Cmd {
	if task == nil {
		return nil
	}
	m.statusOpen = true
	m.statusTaskID = task.ID
	m.statusList = model.StatusChoices(workflow, task.Status)
	m.statusIdx = 0
	m.statusProjectID = projectID
	m.statusProjectName = projectName
	return nil
}

// closeStatus resets the overlay's state.
func (m *Model) closeStatus() {
	m.statusOpen = false
	m.statusTaskID = 0
	m.statusList = nil
	m.statusIdx = 0
	m.statusProjectID = 0
	m.statusProjectName = ""
}

// handleStatusKey processes the strict status overlay: move, apply, close.
func (m Model) handleStatusKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// The detail underneath (if any) stays open.
		m.closeStatus()
		return m, nil
	case "j", "down":
		m.statusIdx = cycleIndex(m.statusIdx, len(m.statusList), 1)
		return m, nil
	case "k", "up":
		m.statusIdx = cycleIndex(m.statusIdx, len(m.statusList), -1)
		return m, nil
	case "enter":
		return m.applyStatus()
	}
	return m, nil
}

// applyStatus applies the highlighted choice and closes the overlay and, if it
// was open, the detail. The listing reloads through taskActionCmd.
func (m Model) applyStatus() (tea.Model, tea.Cmd) {
	choice := taskAt2(m.statusList, m.statusIdx)
	if choice == "" {
		return m, nil
	}
	taskID := m.statusTaskID
	projectID := m.statusProjectID
	projectName := m.statusProjectName
	m.closeStatus()
	m.detailOpen = false
	m.detailTask = nil
	m.detailComments = nil
	m.detailCommentSel = -1

	if projectID > 0 {
		toast := "→ " + projectName + " · " + choice
		return m, tea.Batch(m.setToast(toast, "info"), m.taskActionCmd(taskID, func(id int64) (*model.Task, error) {
			return m.database.MoveTaskToProject(id, projectID, choice)
		}))
	}

	toast := "status → " + choice
	return m, tea.Batch(m.setToast(toast, "info"), m.taskActionCmd(taskID, func(id int64) (*model.Task, error) {
		return m.database.MoveTask(id, choice)
	}))
}

// openMoveProject opens the project picker with every project except the
// task's current one. With no other project it refuses with an error toast.
func (m *Model) openMoveProject(task *model.Task) tea.Cmd {
	if task == nil {
		return nil
	}
	var list []model.Project
	for _, p := range m.projects {
		if p.ID != task.ProjectID {
			list = append(list, p)
		}
	}
	if len(list) == 0 {
		return m.setToast("no other project", "error")
	}
	m.moveProjectOpen = true
	m.moveProjectIdx = 0
	m.moveProjectTaskID = task.ID
	m.moveProjectList = list
	return nil
}

// handleMoveProjectKey processes the project picker: move, pick, close.
func (m Model) handleMoveProjectKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.moveProjectOpen = false
		m.moveProjectList = nil
		return m, nil
	case "j", "down":
		m.moveProjectIdx = cycleIndex(m.moveProjectIdx, len(m.moveProjectList), 1)
		return m, nil
	case "k", "up":
		m.moveProjectIdx = cycleIndex(m.moveProjectIdx, len(m.moveProjectList), -1)
		return m, nil
	case "enter":
		if !inRange(m.moveProjectIdx, len(m.moveProjectList)) {
			return m, nil
		}
		p := m.moveProjectList[m.moveProjectIdx]
		task := m.taskByID(m.moveProjectTaskID)
		m.moveProjectOpen = false
		m.moveProjectList = nil
		if task == nil {
			return m, nil
		}
		return m, m.openStatusForProject(task, p.ID, p.Name, p.Workflow)
	}
	return m, nil
}

// taskByID returns the loaded task with that ID, or nil. The detail's copy
// wins so that a move opened from the detail uses the freshest status.
func (m *Model) taskByID(id int64) *model.Task {
	if m.detailTask != nil && m.detailTask.ID == id {
		return m.detailTask
	}
	for i := range m.tasks {
		if m.tasks[i].ID == id {
			return &m.tasks[i]
		}
	}
	return nil
}

// statusOverlayLines builds the rows of the status overlay: the choices in
// order, with a blank line before cancelled when it is not the first row.
func statusOverlayLines(choices []string, idx int) []string {
	lines := []string{""}
	for i, status := range choices {
		if status == model.CancelledStatus && i > 0 {
			lines = append(lines, "")
		}
		line := selectionPrefix(i == idx) + status
		if i == idx {
			line = styleSelected.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

// renderStatusModal draws the status overlay over content.
func (m *Model) renderStatusModal(content string) string {
	m.statusIdx = clampTo(m.statusIdx, len(m.statusList))

	lines := statusOverlayLines(m.statusList, m.statusIdx)
	totalWidth := modalWidthFor(40, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Status ", lines, totalWidth), totalWidth, m.width)
}

// renderMoveProjectModal draws the project picker over content.
func (m *Model) renderMoveProjectModal(content string) string {
	m.moveProjectIdx = clampTo(m.moveProjectIdx, len(m.moveProjectList))

	lines := []string{""}
	for i, p := range m.moveProjectList {
		line := selectionPrefix(i == m.moveProjectIdx) + p.Name
		if i == m.moveProjectIdx {
			line = styleSelected.Render(line)
		}
		lines = append(lines, line)
	}

	totalWidth := modalWidthFor(40, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Move to project ", lines, totalWidth), totalWidth, m.width)
}
