package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"taskd/internal/model"
)

// editorFinishedMsg se envía cuando el editor termina (edición).
type editorFinishedMsg struct {
	err    error
	taskID int64
	file   string
}

// newTaskFinishedMsg se envía cuando el editor termina (creación).
type newTaskFinishedMsg struct {
	err         error
	projectName string
	file        string
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
	content := fmt.Sprintf("# %s\n\n%s\n\n---\nassignee: %s\npriority: %d\n",
		task.Title,
		task.Description,
		task.Assignee,
		task.Priority,
	)
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return editorFinishedMsg{err: err, taskID: task.ID}
		}
	}
	tmpFile.Close()

	// Construir comando del editor
	args := strings.Fields(editorCmd)
	cmd := exec.Command(args[0], append(args[1:], tmpFile.Name())...)

	// Devolver tea.ExecProcess directamente — es un tea.Cmd
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		if execErr != nil {
			os.Remove(tmpFile.Name())
			return editorFinishedMsg{err: execErr, taskID: task.ID}
		}
		// Leer el archivo modificado
		data, readErr := os.ReadFile(tmpFile.Name())
		os.Remove(tmpFile.Name())
		if readErr != nil {
			return editorFinishedMsg{err: readErr, taskID: task.ID}
		}
		return editorFinishedMsg{
			taskID: task.ID,
			file:   string(data),
		}
	})
}

// newTaskCmd lanza el editor con un template vacío para crear una tarea nueva.
func newTaskCmd(projectName, editorCmd string) tea.Cmd {
	content := "# \n\nDescripción aquí\n\n---\nassignee: unassigned\npriority: 0\n"
	return newTaskCmdWithContent(projectName, editorCmd, content, 0)
}

// newTaskCmdWithContent lanza el editor con contenido prellenado.
// cursorLine indica la línea donde posicionar el cursor (0 = sin posición).
func newTaskCmdWithContent(projectName, editorCmd, content string, cursorLine int) tea.Cmd {
	tmpFile, err := os.CreateTemp("", "tsk-new-*.md")
	if err != nil {
		return func() tea.Msg {
			return newTaskFinishedMsg{err: err, projectName: projectName}
		}
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return newTaskFinishedMsg{err: err, projectName: projectName}
		}
	}
	tmpFile.Close()

	args := strings.Fields(editorCmd)
	cmdArgs := append(args[1:], tmpFile.Name())
	if cursorLine > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("+%d", cursorLine))
	}
	cmd := exec.Command(args[0], cmdArgs...)

	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		if execErr != nil {
			os.Remove(tmpFile.Name())
			return newTaskFinishedMsg{err: execErr, projectName: projectName}
		}
		data, readErr := os.ReadFile(tmpFile.Name())
		os.Remove(tmpFile.Name())
		if readErr != nil {
			return newTaskFinishedMsg{err: readErr, projectName: projectName}
		}
		return newTaskFinishedMsg{
			projectName: projectName,
			file:        string(data),
		}
	})
}

// parseEditFile parsea el contenido del archivo editado.
func parseEditFile(content string) (title, description, assignee string, priority int) {
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
	if metadataPart != "" {
		for _, line := range strings.Split(metadataPart, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "assignee:") {
				assignee = strings.TrimSpace(strings.TrimPrefix(line, "assignee:"))
			} else if strings.HasPrefix(line, "priority:") {
				if p, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "priority:"))); err == nil {
					priority = p
				}
			}
		}
	}

	return
}

// updateTaskFromEdit actualiza la tarea con los datos editados.
func (m *Model) updateTaskFromEdit(taskID int64, content string) tea.Cmd {
	return func() tea.Msg {
		title, description, assignee, priority := parseEditFile(content)

		updates := map[string]any{
			"title":       title,
			"description": description,
			"assignee":    assignee,
			"priority":    priority,
		}

		_, err := m.database.UpdateTask(taskID, updates)
		if err != nil {
			return nil
		}
		return m.loadTasks()
	}
}

// createTaskFromEdit crea una tarea nueva con los datos del editor.
func (m *Model) createTaskFromEdit(projectName, content string) tea.Cmd {
	return func() tea.Msg {
		title, description, assignee, priority := parseEditFile(content)

		title = strings.TrimSpace(title)
		if title == "" {
			title = "(untitled)"
		}

		// Position: al final de las tasks del proyecto
		position := 0
		for _, t := range m.tasks {
			if t.ProjectName == projectName && t.Position >= position {
				position = t.Position + 1
			}
		}

		_, err := m.database.CreateTask(projectName, title, description, assignee, priority, position, "")
		if err != nil {
			return nil
		}
		return m.loadTasks()
	}
}

// editSelectedTask edita la tarea seleccionada en la vista actual.
func (m *Model) editSelectedTask() tea.Cmd {
	var t *model.Task

	switch m.currentView {
	case viewList:
		tasks := m.filteredTasks()
		if len(tasks) > 0 && m.cursor < len(tasks) {
			t = &tasks[m.cursor]
		}
	case viewKanban:
		workflow := m.mergedWorkflow()
		if m.kanbanCol < len(workflow) {
			colTasks := m.tasksInColumn(workflow[m.kanbanCol])
			if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
				t = &colTasks[m.kanbanRow]
			}
		}
	case viewDashboard:
		if m.detailOpen && m.detailTask != nil {
			t = m.detailTask
		}
	}

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
	m.newTaskTitle = ""
	m.newTaskPriority = 0
	m.newTaskAssignee = "unassigned"
	m.newTaskAssigneeSuggIdx = -1
	m.newTaskProject = projectName
	m.newTaskFieldIdx = 0
	return nil
}

// currentProjectName devuelve el nombre del proyecto según la vista activa.
func (m *Model) currentProjectName() string {
	switch m.currentView {
	case viewDashboard:
		if m.dashProjectIdx >= 0 && m.dashProjectIdx < len(m.projects) {
			return m.projects[m.dashProjectIdx].Name
		}
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
