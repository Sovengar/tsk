package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// assigneeSuggestions devuelve los assignees que coinciden con el prefix (case-insensitive).
func (m *Model) assigneeSuggestions(prefix string) []string {
	if prefix == "" {
		return nil
	}
	lower := strings.ToLower(prefix)
	var result []string
	for _, a := range m.uniqueAssignees() {
		if strings.HasPrefix(strings.ToLower(a), lower) {
			result = append(result, a)
		}
	}
	return result
}

// handleNewTaskKey procesa teclas del modal de nueva tarea.
func (m Model) handleNewTaskKey(key string) (tea.Model, tea.Cmd) {
	numFields := 3

	switch key {
	case "esc":
		if m.newTaskFieldIdx == 2 && m.newTaskAssignee != "" {
			m.newTaskAssignee = ""
			m.newTaskAssigneeSuggIdx = -1
			return m, nil
		}
		m.newTaskOpen = false
		return m, nil

	case "down":
		if m.newTaskFieldIdx == 2 && len(m.assigneeSuggestions(m.newTaskAssignee)) > 0 {
			suggs := m.assigneeSuggestions(m.newTaskAssignee)
			m.newTaskAssigneeSuggIdx = (m.newTaskAssigneeSuggIdx + 1) % len(suggs)
			return m, nil
		}
	case "up":
		if m.newTaskFieldIdx == 2 && len(m.assigneeSuggestions(m.newTaskAssignee)) > 0 {
			suggs := m.assigneeSuggestions(m.newTaskAssignee)
			if m.newTaskAssigneeSuggIdx <= 0 {
				m.newTaskAssigneeSuggIdx = len(suggs) - 1
			} else {
				m.newTaskAssigneeSuggIdx--
			}
			return m, nil
		}
	case "tab":
		// Autocompletar si hay sugerencia seleccionada, luego avanzar campo
		if m.newTaskFieldIdx == 2 && m.newTaskAssigneeSuggIdx >= 0 {
			suggs := m.assigneeSuggestions(m.newTaskAssignee)
			if m.newTaskAssigneeSuggIdx < len(suggs) {
				m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
			}
		}
		m.newTaskAssigneeSuggIdx = -1
		m.newTaskFieldIdx = (m.newTaskFieldIdx + 1) % numFields
	case "shift+tab":
		m.newTaskAssigneeSuggIdx = -1
		m.newTaskFieldIdx = (m.newTaskFieldIdx + numFields - 1) % numFields

	case "left", "right":
		if m.newTaskFieldIdx == 1 {
			if key == "left" && m.newTaskPriority > 0 {
				m.newTaskPriority--
			} else if key == "right" && m.newTaskPriority < 3 {
				m.newTaskPriority++
			}
		}

	case "1", "2", "3", "4":
		if m.newTaskFieldIdx == 1 {
			m.newTaskPriority = int(key[0] - '0')
		}

	case "enter":
		if m.newTaskFieldIdx == 0 && m.newTaskTitle == "" {
			return m, nil
		}
		// En assignee: si hay sugerencia seleccionada, autocompletar
		if m.newTaskFieldIdx == 2 && m.newTaskAssigneeSuggIdx >= 0 {
			suggs := m.assigneeSuggestions(m.newTaskAssignee)
			if m.newTaskAssigneeSuggIdx < len(suggs) {
				m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
				m.newTaskAssigneeSuggIdx = -1
				return m, nil
			}
		}
		// Confirmar creación
		assignee := m.newTaskAssignee
		if assignee == "" {
			assignee = "unassigned"
		}
		project := m.newTaskProject
		title := m.newTaskTitle
		priority := m.newTaskPriority
		m.newTaskOpen = false
		return m, m.openEditorForNewTask(project, title, priority, assignee)

	default:
		if m.newTaskFieldIdx == 0 {
			// Title input
			if key == "backspace" {
				if len(m.newTaskTitle) > 0 {
					m.newTaskTitle = m.newTaskTitle[:len(m.newTaskTitle)-1]
				}
			} else if key == "space" || key == " " {
				m.newTaskTitle += " "
			} else if len(key) == 1 && key[0] >= 33 {
				m.newTaskTitle += key
			}
		} else if m.newTaskFieldIdx == 2 {
			// Assignee input con autocomplete
			m.newTaskAssigneeSuggIdx = -1
			if key == "backspace" {
				if len(m.newTaskAssignee) > 0 {
					m.newTaskAssignee = m.newTaskAssignee[:len(m.newTaskAssignee)-1]
				}
			} else if key == "space" || key == " " {
				m.newTaskAssignee += " "
			} else if len(key) == 1 && key[0] >= 33 {
				m.newTaskAssignee += key
			}
		}
	}

	return m, nil
}

// renderNewTaskModal renderiza el modal de creación de tarea dentro de una caja
// con borde. Los keybinds van en la barra inferior, no duplicados dentro.
func (m *Model) renderNewTaskModal(content string) string {
	w := m.width

	titleInput := m.newTaskTitle
	if m.newTaskFieldIdx == 0 {
		titleInput += "_"
	}

	prioLabels := []string{"none", "low", "med", "high"}
	prioDisplay := prioLabels[m.newTaskPriority]
	if m.newTaskFieldIdx == 1 {
		prioDisplay = "> " + prioDisplay + " <"
	} else {
		prioDisplay = "  " + prioDisplay
	}

	assigneeInput := m.newTaskAssignee
	if m.newTaskFieldIdx == 2 {
		assigneeInput += "_"
	}

	lines := []string{
		fmt.Sprintf("  Title:     %s", titleInput),
		fmt.Sprintf("  Priority:  %s    (← → or 1-4)", prioDisplay),
		fmt.Sprintf("  Assignee:  %s", assigneeInput),
	}

	// Mostrar sugerencias si estamos en el campo assignee
	if m.newTaskFieldIdx == 2 {
		suggs := m.assigneeSuggestions(m.newTaskAssignee)
		if len(suggs) > 0 {
			maxShow := 5
			if len(suggs) < maxShow {
				maxShow = len(suggs)
			}
			for i := 0; i < maxShow; i++ {
				prefix := "    "
				if i == m.newTaskAssigneeSuggIdx {
					prefix = styleSelected.Render("  > ")
				}
				lines = append(lines, fmt.Sprintf("%s%s", prefix, suggs[i]))
			}
		} else if m.newTaskAssignee != "" {
			lines = append(lines, fmt.Sprintf("    %s (new)", styleDim.Render(m.newTaskAssignee)))
		}
	}

	totalWidth := modalWidthFor(50, w)
	return overlayModal(content, renderModalBox(" New Task ", lines, totalWidth), totalWidth, w)
}

// openEditorForNewTask abre nvim con template prellenado para la nueva tarea.
func (m *Model) openEditorForNewTask(projectName, title string, priority int, assignee string) tea.Cmd {
	editorCmd := m.config.Editor.Command
	if editorCmd == "" {
		editorCmd = "nvim"
	}

	content := fmt.Sprintf("# %s\n\n\n\n---\nassignee: %s\npriority: %d\ntags: \n",
		title, assignee, priority)

	return newTaskCmdWithContent(projectName, editorCmd, content, 4)
}
