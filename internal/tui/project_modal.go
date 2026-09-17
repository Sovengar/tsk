package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// projectSavedMsg se emite al terminar una operación de proyecto (crear, editar,
// archivar o restaurar). err != nil indica fallo.
type projectSavedMsg struct {
	err    error
	name   string
	action string // "create" | "edit" | "archive" | "unarchive"
}

// dashProjectList devuelve los proyectos visibles en el Dashboard según el
// toggle de archivados.
func (m *Model) dashProjectList() []model.Project {
	if m.showArchived {
		return m.archivedProjects
	}
	return m.projects
}

// selectedDashProject devuelve el proyecto seleccionado en el Dashboard, o nil.
func (m *Model) selectedDashProject() *model.Project {
	list := m.dashProjectList()
	if m.dashProjectIdx >= 0 && m.dashProjectIdx < len(list) {
		return &list[m.dashProjectIdx]
	}
	return nil
}

// clampDashProjectIdx mantiene el índice de selección dentro de la lista visible.
func (m *Model) clampDashProjectIdx() {
	n := len(m.dashProjectList())
	if n == 0 {
		m.dashProjectIdx = 0
		return
	}
	if m.dashProjectIdx >= n {
		m.dashProjectIdx = n - 1
	}
	if m.dashProjectIdx < 0 {
		m.dashProjectIdx = 0
	}
}

// selectProjectByName selecciona un proyecto y ajusta la vista (activos o
// archivados) según dónde aparezca.
func (m *Model) selectProjectByName(name string) {
	for i, p := range m.projects {
		if p.Name == name {
			m.showArchived = false
			m.dashProjectIdx = i
			return
		}
	}
	for i, p := range m.archivedProjects {
		if p.Name == name {
			m.showArchived = true
			m.dashProjectIdx = i
			return
		}
	}
}

// openProjectModal abre el modal de proyecto en modo creación (edit=false) o
// edición (edit=true). En creación limpia los campos.
func (m *Model) openProjectModal(edit bool) tea.Cmd {
	m.projectModalOpen = true
	m.projectModalEdit = edit
	m.projectModalField = 0
	if !edit {
		m.projectNameInput = ""
		m.projectWorkflowInput = strings.Join(model.DefaultWorkflow, ",")
		m.projectListOrderInput = strings.Join(model.DefaultListOrder, ",")
		m.projectEditingName = ""
	}
	return nil
}

// openProjectModalForEdit precarga los campos con el proyecto dado.
func (m *Model) openProjectModalForEdit(p *model.Project) tea.Cmd {
	m.openProjectModal(true)
	m.projectEditingName = p.Name
	m.projectNameInput = p.Name
	m.projectWorkflowInput = strings.Join(p.Workflow, ",")
	m.projectListOrderInput = strings.Join(p.ListOrder, ",")
	return nil
}

// editTextInput aplica una tecla de edición a un buffer de texto simple.
func editTextInput(target *string, key string) {
	if target == nil {
		return
	}
	switch {
	case key == "backspace":
		if len(*target) > 0 {
			*target = (*target)[:len(*target)-1]
		}
	case key == "space" || key == " ":
		*target += " "
	case len(key) == 1 && key[0] >= 33:
		*target += key
	}
}

// handleProjectModalKey procesa teclas del modal de proyecto.
func (m Model) handleProjectModalKey(key string) (tea.Model, tea.Cmd) {
	const numFields = 3

	switch key {
	case "esc":
		m.projectModalOpen = false
		return m, nil
	case "tab":
		m.projectModalField = (m.projectModalField + 1) % numFields
		return m, nil
	case "shift+tab":
		m.projectModalField = (m.projectModalField + numFields - 1) % numFields
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.projectNameInput)
		if name == "" {
			return m, m.setToast("Project name is required", "error")
		}
		original := m.projectEditingName
		m.projectModalOpen = false
		return m, m.saveProjectCmd(m.projectModalEdit, original, name,
			strings.TrimSpace(m.projectWorkflowInput),
			strings.TrimSpace(m.projectListOrderInput))
	}

	var target *string
	switch m.projectModalField {
	case 0:
		target = &m.projectNameInput
	case 1:
		target = &m.projectWorkflowInput
	case 2:
		target = &m.projectListOrderInput
	}
	editTextInput(target, key)
	return m, nil
}

// saveProjectCmd persiste la creación o edición de un proyecto.
func (m Model) saveProjectCmd(edit bool, original, name, workflow, listOrder string) tea.Cmd {
	return func() tea.Msg {
		if edit {
			action := "edit"
			updates := map[string]any{"name": name}
			if workflow != "" {
				wf, perr := model.ParseWorkflow(workflow)
				if perr != nil {
					return projectSavedMsg{err: perr, action: action}
				}
				updates["workflow"] = wf
			}
			if listOrder != "" {
				lo, perr := model.ParseWorkflow(listOrder)
				if perr != nil {
					return projectSavedMsg{err: perr, action: action}
				}
				updates["list_order"] = lo
			}
			if err := m.database.UpdateProject(original, updates); err != nil {
				return projectSavedMsg{err: err, action: action}
			}
			return projectSavedMsg{name: name, action: action}
		}

		var wf []string
		if workflow != "" {
			var perr error
			wf, perr = model.ParseWorkflow(workflow)
			if perr != nil {
				return projectSavedMsg{err: perr, action: "create"}
			}
		}
		var lo []string
		if listOrder != "" {
			var perr error
			lo, perr = model.ParseWorkflow(listOrder)
			if perr != nil {
				return projectSavedMsg{err: perr, action: "create"}
			}
		}
		if _, err := m.database.CreateProjectWithListOrder(name, wf, lo); err != nil {
			return projectSavedMsg{err: err, action: "create"}
		}
		return projectSavedMsg{name: name, action: "create"}
	}
}

// projectActionCmd archiva o restaura un proyecto.
func (m Model) projectActionCmd(action, name string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch action {
		case "archive":
			err = m.database.ArchiveProject(name)
		case "unarchive":
			err = m.database.UnarchiveProject(name)
		}
		if err != nil {
			return projectSavedMsg{err: err, action: action}
		}
		return projectSavedMsg{name: name, action: action}
	}
}

// handleConfirmKey resuelve el modal de confirmación de archivar/restaurar.
func (m Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y", "enter":
		action := m.confirmAction
		name := m.confirmProject
		m.confirmOpen = false
		m.confirmAction = ""
		m.confirmProject = ""
		if action == "" || name == "" {
			return m, nil
		}
		return m, m.projectActionCmd(action, name)
	case "n", "N", "esc", "q":
		m.confirmOpen = false
		m.confirmAction = ""
		m.confirmProject = ""
	}
	return m, nil
}

// handleProjectSaved aplica el resultado de una operación de proyecto.
func (m Model) handleProjectSaved(msg projectSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Reabrir el modal para no perder lo escrito en crear/editar.
		if msg.action == "create" || msg.action == "edit" {
			m.projectModalOpen = true
		}
		return m, m.setToast(msg.err.Error(), "error")
	}

	var text string
	switch msg.action {
	case "create":
		text = fmt.Sprintf("Project %q created", msg.name)
		m.pendingSelectName = msg.name
	case "edit":
		text = fmt.Sprintf("Project %q updated", msg.name)
		m.pendingSelectName = msg.name
	case "archive":
		text = fmt.Sprintf("Project %q archived", msg.name)
	case "unarchive":
		text = fmt.Sprintf("Project %q restored", msg.name)
		m.pendingSelectName = msg.name
	default:
		text = "Done"
	}
	cmd := m.setToast(text, "info")
	return m, tea.Batch(cmd, m.loadProjects(), m.loadTasks())
}

// renderProjectModal renderiza el modal de creación/edición de proyecto dentro
// de una caja con borde y el título embebido arriba. Los keybinds van en la
// barra inferior, no duplicados.
func (m *Model) renderProjectModal(content string) string {
	w := m.width

	title := " New Project "
	hint := "(comma-separated; empty = default)"
	if m.projectModalEdit {
		title = " Edit Project "
		hint = "(comma-separated; empty = keep current)"
	}

	name := m.projectNameInput
	if m.projectModalField == 0 {
		name += "_"
	}
	workflow := m.projectWorkflowInput
	if m.projectModalField == 1 {
		workflow += "_"
	}
	listOrder := m.projectListOrderInput
	if m.projectModalField == 2 {
		listOrder += "_"
	}

	lines := []string{
		fmt.Sprintf("  Name:       %s", name),
		fmt.Sprintf("  Workflow:   %s", workflow),
		fmt.Sprintf("  List order: %s", listOrder),
		styleDim.Render("              " + hint),
	}

	totalWidth := modalWidthFor(64, w)
	return overlayModal(content, renderModalBox(title, lines, totalWidth), totalWidth, w)
}

// renderConfirmModal superpone una confirmación y/n sobre el contenido.
func (m *Model) renderConfirmModal(content string) string {
	w := m.width

	title := " Archive project "
	detail := "Tasks will be hidden. You can restore it later."
	if m.confirmAction == "unarchive" {
		title = " Restore project "
		detail = "The project and its tasks will be visible again."
	}

	lines := []string{
		fmt.Sprintf("  Project: %s", m.confirmProject),
		styleDim.Render("  " + detail),
	}

	totalWidth := modalWidthFor(54, w)
	return overlayModal(content, renderModalBox(title, lines, totalWidth), totalWidth, w)
}
