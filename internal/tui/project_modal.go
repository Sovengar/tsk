package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// projectSavedMsg is emitted when a project operation finishes (create, edit,
// archive or restore). err != nil indicates failure.
type projectSavedMsg struct {
	err    error
	name   string
	action string // "create" | "edit" | "archive" | "unarchive"
}

// dashProjectList returns the projects visible in the Dashboard according to
// the archived toggle.
func (m *Model) dashProjectList() []model.Project {
	if m.showArchived {
		return m.archivedProjects
	}
	return m.projects
}

// selectedDashProject returns the project selected in the Dashboard, or nil.
func (m *Model) selectedDashProject() *model.Project {
	list := m.dashProjectList()
	if inRange(m.dashProjectIdx, len(list)) {
		return &list[m.dashProjectIdx]
	}
	return nil
}

// clampDashProjectIdx keeps the selection index inside the visible list.
func (m *Model) clampDashProjectIdx() {
	m.dashProjectIdx = clampTo(m.dashProjectIdx, len(m.dashProjectList()))
}

// selectProjectByName selects a project and adjusts the view (active or
// archived) according to where it appears.
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

// openProjectModal opens the project modal in creation mode (edit=false) or
// edit (edit=true). In creation it clears the fields.
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

// openProjectModalForEdit preloads the fields with the given project.
func (m *Model) openProjectModalForEdit(p *model.Project) tea.Cmd {
	m.openProjectModal(true)
	m.projectEditingName = p.Name
	m.projectNameInput = p.Name
	m.projectWorkflowInput = strings.Join(p.Workflow, ",")
	m.projectListOrderInput = strings.Join(p.ListOrder, ",")
	return nil
}

// editTextInput applies an editing key to a simple text buffer.
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
	// The 33 leaves the space out on purpose: the fields of this modal are
	// comma-separated lists, and a space inside would break the name. The
	// condition goes unnoticed because Bubbletea delivers the space as a named
	// key ("space"), not as a lone character.
	case len(key) == 1 && key[0] >= 33:
		*target += key
	}
}

// handleProjectModalKey processes project modal keys.
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

// saveProjectCmd persists the creation or editing of a project.
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

// projectActionCmd archives or restores a project.
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

// handleConfirmKey resolves the confirmation modal for destructive actions:
// archive/restore project or delete an off-day.
func (m Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y", "enter":
		action := m.confirmAction
		project := m.confirmProject
		offday := m.confirmOffday
		m.confirmOpen = false
		m.confirmAction = ""
		m.confirmProject = ""
		m.confirmOffday = model.OffDay{}
		switch action {
		case "delete-offday":
			if offday.ID == 0 {
				return m, nil
			}
			return m, m.deleteOffDayCmd(offday.ID, offday.Assignee)
		case "archive", "unarchive":
			if project == "" {
				return m, nil
			}
			return m, m.projectActionCmd(action, project)
		}
		return m, nil
	case "n", "N", "esc", "q":
		m.confirmOpen = false
		m.confirmAction = ""
		m.confirmProject = ""
		m.confirmOffday = model.OffDay{}
	}
	return m, nil
}

// handleProjectSaved applies the result of a project operation.
func (m Model) handleProjectSaved(msg projectSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Reopen the modal so the written text in create/edit is not lost.
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

// renderProjectModal renders the project create/edit modal inside
// a box with a border and the title embedded at the top. The keybinds go in
// the bottom bar, not duplicated.
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

// renderConfirmModal overlays a yes/no confirmation on the content.
func (m *Model) renderConfirmModal(content string) string {
	w := m.width

	var title string
	var lines []string
	if m.confirmAction == "delete-offday" {
		o := m.confirmOffday
		title = " Delete off-day "
		lines = []string{
			fmt.Sprintf("  Person: %s", o.Assignee),
			fmt.Sprintf("  From:   %s", o.StartDate),
			fmt.Sprintf("  To:     %s", o.EndDate),
			styleDim.Render("  This only removes the absence; tasks are unaffected."),
		}
	} else {
		title = " Archive project "
		detail := "Tasks will be hidden. You can restore it later."
		if m.confirmAction == "unarchive" {
			title = " Restore project "
			detail = "The project and its tasks will be visible again."
		}
		lines = []string{
			fmt.Sprintf("  Project: %s", m.confirmProject),
			styleDim.Render("  " + detail),
		}
	}

	totalWidth := modalWidthFor(54, w)
	return overlayModal(content, renderModalBox(title, lines, totalWidth), totalWidth, w)
}
