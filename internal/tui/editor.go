package tui

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// editorFinishedMsg is sent when the editor finishes (editing).
type editorFinishedMsg struct {
	err    error
	taskID int64
	file   string
}

// editTaskCmd launches the editor with the task's data.
// It prepares the temp file and returns tea.ExecProcess directly.
func editTaskCmd(task model.Task, editorCmd string) tea.Cmd {
	// Create the temp file with the editable fields. It goes through createTemp,
	// the same indirection the comment uses, so that its failed write
	// branch has a test.
	tmpFile, err := createTemp("", "tsk-edit-*.md")
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err, taskID: task.ID}
		}
	}

	// writeAndClose lives in comment.go and is the same function the comment
	// creation uses: the task editor writes a template, the comment one writes
	// nothing, and what fails after creating the temp file fails the same in both.
	if err := writeAndClose(tmpFile, editTemplate(task)); err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err, taskID: task.ID}
		}
	}

	// Build the editor command
	args := strings.Fields(editorCmd)
	cmd := exec.Command(args[0], append(args[1:], tmpFile.Name())...)

	// Return tea.ExecProcess directly — it is a tea.Cmd
	return tea.ExecProcess(cmd, editorCallback(task.ID, tmpFile.Name()))
}

// editorCallback is the tail of editTaskCmd, outside of it for the same
// reason as commentCallback: tea.ExecProcess hides the closure inside a
// private message and a test cannot reach it.
func editorCallback(taskID int64, path string) tea.ExecCallback {
	return func(execErr error) tea.Msg {
		file, err := readEditedFile(path, execErr)
		if err != nil {
			return editorFinishedMsg{err: err, taskID: taskID}
		}
		return editorFinishedMsg{
			taskID: taskID,
			file:   file,
		}
	}
}

// editTemplate is the file opened in the external editor: the readable body
// on top, the metadata below separated by "---".
//
// It was extracted from editTaskCmd because the full cycle is writing this
// text and reading it back with parseEditFile. With the two halves exposed the
// whole trip can be checked without launching an editor: if one of the two
// changes without the other, the task is saved with a lost field and nobody notices.
func editTemplate(task model.Task) string {
	return fmt.Sprintf("# %s\n\n%s\n\n---\nassignee: %s\npriority: %d\nestimate: %g\ntags: %s\n",
		task.Title,
		task.Description,
		task.Assignee,
		task.Priority,
		task.Estimate,
		strings.Join(task.Tags, ","),
	)
}

// parseEditFile parses the content of the edited file.
func parseEditFile(content string) (title, description, assignee string, priority int, estimate float64, tags []string) {
	mainPart := content
	metadataPart := ""
	if idx := strings.Index(content, "---\n"); idx >= 0 {
		mainPart = content[:idx]
		metadataPart = content[idx+4:]
	}

	trimmed := strings.TrimSpace(mainPart)
	if strings.HasPrefix(trimmed, "# ") {
		title = strings.TrimPrefix(trimmed, "# ")
		if nl := strings.Index(title, "\n"); nl >= 0 {
			title = title[:nl]
		}
	}

	// Line 0 is always the title's and it is discarded by POSITION, not by
	// starting with "# ". The `if len(descLines) > 1` that was here did
	// nothing: with a single line, descLines[1:] is also empty, so the
	// description came out the same. It was a branch with three mutants that
	// nobody could tell apart, because all three give the same result.
	descLines := strings.Split(mainPart, "\n")
	start := 1
	for start < len(descLines) && strings.TrimSpace(descLines[start]) == "" {
		start++
	}
	description = strings.TrimSpace(strings.Join(descLines[start:], "\n"))

	assignee = ""
	priority = 0
	estimate = 0
	if metadataPart != "" {
		for _, line := range strings.Split(metadataPart, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "assignee:") {
				assignee = strings.TrimSpace(strings.TrimPrefix(line, "assignee:"))
			} else if strings.HasPrefix(line, "priority:") {
				if p, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "priority:"))); err == nil {
					priority = p
				}
			} else if strings.HasPrefix(line, "estimate:") {
				// The "non-negative" filter goes INSIDE the parsing, with a
				// sentinel instead of a comparison on the assignment line.
				//
				// `err == nil && e >= 0` versus `e > 0` only differed when
				// e == 0, and since the line assigned over a return value that
				// was already 0, putting a zero on top of a zero changed nothing:
				// the two forms were indistinguishable. Now the "not valid" is an
				// explicit -1, which is a value the estimate can never have, so
				// rejection and acceptance are two distinct facts and can be
				// checked separately.
				if e := nonNegativeEstimate(strings.TrimSpace(strings.TrimPrefix(line, "estimate:"))); e != estimateAbsent {
					estimate = e
				}
			} else if strings.HasPrefix(line, "tags:") {
				tags = model.ParseTags(strings.TrimPrefix(line, "tags:"))
			}
		}
	}

	return
}

// nonNegativeEstimate parses the template's estimate field and returns the
// value, or -1 if it cannot be parsed or is negative.
//
// The -1 is a sentinel and not an error: a negative estimate is not something
// that can be written to the database, so it is discarded. And since no real
// estimate is -1, the returned value says unambiguously whether there was an
// estimate or not -- which is what the caller's comparison needs to tell "was
// not there" from "came as zero".
func nonNegativeEstimate(s string) float64 {
	e, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return estimateAbsent
	}
	if e < 0 {
		return estimateAbsent
	}
	// NaN parses without error and compares false against everything, so an `e < 0`
	// or an `e >= 0` would let it through as if it were a good estimate. It is
	// rejected separately: it is the only value that is not a number despite parsing.
	if math.IsNaN(e) {
		return estimateAbsent
	}
	return e
}

// estimateAbsent is the sentinel nonNegativeEstimate returns when the field
// brings no usable estimate. It is -1 because no valid estimate is one, so
// the sentinel and "the estimate is zero" never get confused.
const estimateAbsent = -1

// updateTaskFromEdit updates the task with the edited data.
func (m *Model) updateTaskFromEdit(taskID int64, content string) tea.Cmd {
	return func() tea.Msg {
		title, description, assignee, priority, estimate, tags := parseEditFile(content)

		updates := map[string]any{
			"title":       title,
			"description": description,
			"assignee":    assignee,
			"priority":    priority,
			"estimate":    estimate,
			"tags":        model.TagsJSON(tags),
		}

		_, err := m.database.UpdateTask(taskID, updates)
		if err != nil {
			return nil
		}
		tasks, err := m.database.ListTasks("", "", "")
		if err != nil {
			return nil
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

// selectedEditableTask returns the task selected in the current view, or
// nil if there is none. Single source for the external and the inline editor.
func (m *Model) selectedEditableTask() *model.Task {
	switch m.currentView {
	case viewList:
		tasks := m.filteredTasks()
		// inRange covers both halves: with an empty list no index is valid, so
		// the `len(tasks) > 0` that did the first half was redundant.
		if inRange(m.cursor, len(tasks)) {
			return &tasks[m.cursor]
		}
	case viewKanban:
		cols := m.kanbanColumns()
		if inRange(m.kanbanCol, len(cols)) {
			colTasks := cols[m.kanbanCol].tasks
			if inRange(m.kanbanRow, len(colTasks)) {
				return &colTasks[m.kanbanRow]
			}
		}
	case viewDashboard:
		if m.detailOpen && m.detailTask != nil {
			return m.detailTask
		}
	}
	return nil
}

// editSelectedTask opens the external editor with the selected task.
func (m *Model) editSelectedTask() tea.Cmd {
	t := m.selectedEditableTask()
	if t == nil {
		return nil
	}

	editorCmd := editorCommand(m.config.Editor.Command)

	return editTaskCmd(*t, editorCmd)
}

// newTask opens the modal to create a new task.
func (m *Model) newTask() tea.Cmd {
	projectName := m.currentProjectName()
	if projectName == "" {
		return nil
	}

	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldPriority
	m.newTaskPriority = model.PriorityLow
	m.newTaskTitle = ""
	m.newTaskAssignee = "Me"
	m.newTaskAssigneeSuggIdx = -1
	m.newTaskTags = nil
	m.newTaskTagInput = ""
	m.newTaskTagSuggIdx = -1
	m.newTaskProject = projectName
	m.newTaskErr = ""
	m.newTaskTextarea = m.buildNewTaskTextarea("")
	return nil
}

// currentProjectName returns the project name according to the active view.
func (m *Model) currentProjectName() string {
	return currentProjectName(m.currentView, m.filterProject, m.projects)
}
