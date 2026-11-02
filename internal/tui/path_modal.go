package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/harness"
	"tsk/internal/model"
)

// pathSavedMsg results from saving a task's path. err != nil indicates failure.
type pathSavedMsg struct {
	err    error
	taskID int64
	task   *model.Task
	tasks  []model.Task
}

// openPath opens the path editor for a task. The input starts with the task's
// own path, empty when the task inherits the project path.
func (m *Model) openPath(task *model.Task) tea.Cmd {
	if task == nil {
		return nil
	}
	m.pathOpen = true
	m.pathTaskID = task.ID
	m.pathInput = task.Path
	return nil
}

func (m *Model) closePath() {
	m.pathOpen = false
	m.pathTaskID = 0
	m.pathInput = ""
}

// handlePathKey processes the path modal's keys: typing edits the path,
// Enter saves it and Esc cancels.
func (m Model) handlePathKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.closePath()
		return m, nil
	case "enter":
		return m.applyPath()
	}
	editTextInput(&m.pathInput, key)
	return m, nil
}

// applyPath persists the typed path (empty clears the override) and keeps the
// detail open with the refreshed task.
func (m Model) applyPath() (tea.Model, tea.Cmd) {
	taskID := m.pathTaskID
	path := strings.TrimSpace(m.pathInput)
	m.closePath()
	return m, m.savePathCmd(taskID, path)
}

// savePathCmd persists the path and reloads the listing.
func (m *Model) savePathCmd(taskID int64, path string) tea.Cmd {
	return func() tea.Msg {
		task, err := m.database.UpdateTask(taskID, map[string]any{"path": path})
		if err != nil {
			return pathSavedMsg{err: err, taskID: taskID}
		}
		// The save already landed; a listing failure only leaves the views stale.
		tasks, _ := m.database.ListTasks("", "", "")
		return pathSavedMsg{taskID: taskID, task: task, tasks: tasks}
	}
}

// renderPathModal draws the path editor overlaid on the detail.
func (m *Model) renderPathModal(content string) string {
	w := m.width
	width := modalWidthFor(60, w)

	lines := []string{}
	if m.detailTask != nil {
		lines = append(lines, styleDim.Render("  "+m.detailTask.Title))
		inherited := harness.ResolveWorkingDir(m.config, "", m.detailTask.ProjectPath)
		lines = append(lines, "  Project:   "+inherited)
	}
	lines = append(lines, "")
	lines = append(lines, "  > "+m.pathInput+styleTitle.Render("▏"))
	lines = append(lines, "")
	lines = append(lines, styleDim.Render("  empty = inherit the project path"))

	return overlayModal(content, renderModalBox(" Task path ", lines, width), width, w)
}
