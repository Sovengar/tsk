package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// editorFinishedMsg se envía cuando el editor termina (edición).
type editorFinishedMsg struct {
	err    error
	taskID int64
	file   string
}

// editTaskCmd lanza el editor con los datos de la tarea.
// Prepara el archivo temporal y devuelve tea.ExecProcess directamente.
func editTaskCmd(task model.Task, editorCmd string) tea.Cmd {
	// Crear archivo temporal con los campos editables
	tmpFile, err := os.CreateTemp("", "tsk-edit-*.md")
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err, taskID: task.ID}
		}
	}

	// Escribir formato editable
	content := fmt.Sprintf("# %s\n\n%s\n\n---\nassignee: %s\npriority: %d\nestimate: %g\ntags: %s\n",
		task.Title,
		task.Description,
		task.Assignee,
		task.Priority,
		task.Estimate,
		strings.Join(task.Tags, ","),
	)
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return editorFinishedMsg{err: err, taskID: task.ID}
		}
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return editorFinishedMsg{err: err, taskID: task.ID}
		}
	}

	// Construir comando del editor
	args := strings.Fields(editorCmd)
	cmd := exec.Command(args[0], append(args[1:], tmpFile.Name())...)

	// Devolver tea.ExecProcess directamente — es un tea.Cmd
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		if execErr != nil {
			_ = os.Remove(tmpFile.Name())
			return editorFinishedMsg{err: execErr, taskID: task.ID}
		}
		// Leer el archivo modificado
		data, readErr := os.ReadFile(tmpFile.Name())
		_ = os.Remove(tmpFile.Name())
		if readErr != nil {
			return editorFinishedMsg{err: readErr, taskID: task.ID}
		}
		return editorFinishedMsg{
			taskID: task.ID,
			file:   string(data),
		}
	})
}

// parseEditFile parsea el contenido del archivo editado.
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

	descLines := strings.Split(mainPart, "\n")
	if len(descLines) > 1 {
		start := 1
		for start < len(descLines) && strings.TrimSpace(descLines[start]) == "" {
			start++
		}
		description = strings.TrimSpace(strings.Join(descLines[start:], "\n"))
	}

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
				if e, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "estimate:")), 64); err == nil && e >= 0 {
					estimate = e
				}
			} else if strings.HasPrefix(line, "tags:") {
				tags = model.ParseTags(strings.TrimPrefix(line, "tags:"))
			}
		}
	}

	return
}

// updateTaskFromEdit actualiza la tarea con los datos editados.
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

// selectedEditableTask devuelve la tarea seleccionada en la vista actual, o
// nil si no hay ninguna. Fuente única para el editor externo y el inline.
func (m *Model) selectedEditableTask() *model.Task {
	switch m.currentView {
	case viewList:
		tasks := m.filteredTasks()
		if len(tasks) > 0 && m.cursor < len(tasks) {
			return &tasks[m.cursor]
		}
	case viewKanban:
		cols := m.kanbanColumns()
		if m.kanbanCol < len(cols) {
			colTasks := cols[m.kanbanCol].tasks
			if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
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

// editSelectedTask abre el editor externo con la tarea seleccionada.
func (m *Model) editSelectedTask() tea.Cmd {
	t := m.selectedEditableTask()
	if t == nil {
		return nil
	}

	editorCmd := m.config.Editor.Command
	if editorCmd == "" {
		editorCmd = "nvim"
	}

	return editTaskCmd(*t, editorCmd)
}

// newTask abre el modal para crear una tarea nueva.
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

// currentProjectName devuelve el nombre del proyecto según la vista activa.
func (m *Model) currentProjectName() string {
	switch m.currentView {
	case viewList, viewKanban:
		if m.filterProject != "" {
			return m.filterProject
		}
	}
	if len(m.projects) > 0 {
		return m.projects[0].Name
	}
	return ""
}
