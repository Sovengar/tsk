package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// newTaskFieldIdx identifies the active field of the task form, in the order
// in which the user walks them with Tab.
const (
	newTaskFieldPriority = iota
	newTaskFieldTitle
	newTaskFieldDescription
	newTaskFieldAssignee
	newTaskFieldTags
	newTaskFieldCount
)

const (
	// newTaskModalWidth is the modal's preferred width (it is truncated in
	// narrow terminals).
	newTaskModalWidth = 64
	// newTaskDescHeight is the height of the embedded description textarea.
	newTaskDescHeight = 4
	// newTaskMaxSuggestions bounds the assignees dropdown.
	newTaskMaxSuggestions = 5
)

// cursorGlyph is the text cursor of the simple fields.
const cursorGlyph = "▏"

// newTaskKeybinds lists the task form's keys. It is the single source for
// the keybinds bar and the help modal.
func newTaskKeybinds() []keybind {
	return []keybind{
		{"↑↓/jk", "field"},
		{"Enter", "enter / next"},
		{"Ctrl+S", "create"},
		{"←→/1-4", "priority"},
		{"↑↓", "suggestions"},
		{",", "add tag"},
		{"Esc", "back"},
	}
}

// assigneeSuggestions returns the known assignees that match (fuzzy) what is
// typed, prioritizing early matches. With an empty input it lists all.
// It excludes the exact match: if you already typed the full name there is
// nothing to complete, so no duplicate is shown and no phantom row is
// selected with ↑↓.
func (m Model) assigneeSuggestions() []string {
	typed := strings.TrimSpace(m.newTaskAssignee)
	if typed == "" {
		return fuzzyFilter(m.uniqueAssignees(), "", newTaskMaxSuggestions)
	}
	var out []string
	for _, s := range fuzzyFilter(m.uniqueAssignees(), typed, 0) {
		if strings.EqualFold(s, typed) {
			continue
		}
		out = append(out, s)
		if len(out) == newTaskMaxSuggestions {
			break
		}
	}
	return out
}

// tagFieldSuggestions returns the known tags that match (fuzzy) what is typed,
// excluding the already added ones and the exact match. With an empty input
// it lists all.
func (m Model) tagFieldSuggestions() []string {
	typed := strings.TrimSpace(m.newTaskTagInput)
	var out []string
	for _, t := range fuzzyFilter(m.uniqueTags(), typed, 0) {
		if containsFold(m.newTaskTags, t) {
			continue
		}
		if typed != "" && strings.EqualFold(t, typed) {
			continue
		}
		out = append(out, t)
		if len(out) == newTaskMaxSuggestions {
			break
		}
	}
	return out
}

// newTaskCommitTag adds the pending tag (what was typed, whether a suggestion
// or a new value), normalized and without duplicates, and clears the input.
func (m *Model) newTaskCommitTag() {
	for _, t := range model.ParseTags(m.newTaskTagInput) {
		if !containsFold(m.newTaskTags, t) {
			m.newTaskTags = append(m.newTaskTags, t)
		}
	}
	m.newTaskTagInput = ""
	m.newTaskTagSuggIdx = -1
}

// newTaskTextareaWidth is the width of the embedded textarea: the modal's
// inner width minus the borders and the 4-column indentation of the rows.
func (m Model) newTaskTextareaWidth() int {
	w := modalWidthFor(newTaskModalWidth, m.width) - 6
	w = max(w, 10)
	return w
}

// buildNewTaskTextarea creates the form's description editor, reusing the
// same textarea as the task detail.
func (m Model) buildNewTaskTextarea(value string) textarea.Model {
	return newDescTextarea(value, m.newTaskTextareaWidth(), newTaskDescHeight)
}

// newTaskSyncFocus focuses or unfocuses the textarea according to the active field.
func (m *Model) newTaskSyncFocus() tea.Cmd {
	if m.newTaskFieldIdx == newTaskFieldDescription {
		return m.newTaskTextarea.Focus()
	}
	m.newTaskTextarea.Blur()
	return nil
}

// newTaskMoveField completes the active suggestion (if we are on Assignee),
// commits the pending tag (if we are on Tags) and moves the field cursor
// forward/backward, blurring the textarea when it is left behind.
func (m Model) newTaskMoveField(delta int) (tea.Model, tea.Cmd) {
	if m.newTaskFieldIdx == newTaskFieldAssignee && m.newTaskAssigneeSuggIdx >= 0 {
		suggs := m.assigneeSuggestions()
		if inRange(m.newTaskAssigneeSuggIdx, len(suggs)) {
			m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
		}
		m.newTaskAssigneeSuggIdx = -1
	}
	// On leaving Tags the highlighted suggestion wins, and the half-typed
	// tag is not lost.
	if m.newTaskFieldIdx == newTaskFieldTags {
		if m.newTaskTagSuggIdx >= 0 {
			suggs := m.tagFieldSuggestions()
			if inRange(m.newTaskTagSuggIdx, len(suggs)) {
				m.newTaskTagInput = suggs[m.newTaskTagSuggIdx]
			}
		}
		m.newTaskCommitTag()
	}
	m.newTaskFieldIdx = (m.newTaskFieldIdx + delta + newTaskFieldCount) % newTaskFieldCount
	cmd := m.newTaskSyncFocus()
	return m, cmd
}

// newTaskSubmit validates and creates the task. If the title is missing, it
// leaves the inline error and gives focus back to Title without closing the modal.
func (m Model) newTaskSubmit() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.newTaskTitle)
	if title == "" {
		m.newTaskErr = "Title is required"
		m.newTaskFieldIdx = newTaskFieldTitle
		m.newTaskFieldActive = true
		cmd := m.newTaskSyncFocus()
		return m, cmd
	}

	if m.newTaskFieldIdx == newTaskFieldAssignee && m.newTaskAssigneeSuggIdx >= 0 {
		suggs := m.assigneeSuggestions()
		if inRange(m.newTaskAssigneeSuggIdx, len(suggs)) {
			m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
		}
	}
	assignee := strings.TrimSpace(m.newTaskAssignee)
	if assignee == "" {
		assignee = "Me"
	}
	m.newTaskCommitTag()
	description := strings.TrimSpace(m.newTaskTextarea.Value())
	tags := append([]string(nil), m.newTaskTags...)
	project := m.newTaskProject
	priority := m.newTaskPriority

	// Creating a task revives an archived project (the DB layer does it in the
	// same transaction as the insert), so the toast says so and the project
	// lists reload to drop it from the archived view.
	revived := false
	for _, p := range m.archivedProjects {
		if p.Name == project {
			revived = true
			break
		}
	}
	toast := "Task created"
	if revived {
		toast = "Task created · project revived"
	}

	m.newTaskClose()
	return m, tea.Batch(
		m.createTaskCmd(project, title, description, assignee, priority, tags),
		m.setToast(toast, "info"),
		m.loadProjects(),
	)
}

// newTaskClose closes the modal and clears the transient state.
func (m *Model) newTaskClose() {
	m.newTaskOpen = false
	m.newTaskErr = ""
	m.newTaskTitle = ""
	m.newTaskAssignee = ""
	m.newTaskAssigneeSuggIdx = -1
	m.newTaskTextarea.Blur()
}

// createTaskCmd persists the task and reloads the listing.
func (m Model) createTaskCmd(project, title, description, assignee string, priority int, tags []string) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.database.CreateTaskFull(project, title, description, assignee, priority, "", 0, tags); err != nil {
			return taskCreateFailedMsg{err: err}
		}
		tasks, err := m.database.ListTasks("", "", "")
		if err != nil {
			return nil
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

// handleNewTaskPaste inserts pasted text into the active field.
func (m Model) handleNewTaskPaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	if m.newTaskFieldIdx == newTaskFieldDescription {
		var cmd tea.Cmd
		m.newTaskTextarea, cmd = m.newTaskTextarea.Update(msg)
		return m, cmd
	}
	text := strings.NewReplacer("\n", " ", "\r", " ").Replace(msg.Content)
	switch m.newTaskFieldIdx {
	case newTaskFieldTitle:
		m.newTaskTitle += text
		m.newTaskErr = ""
	case newTaskFieldAssignee:
		m.newTaskAssignee += text
		m.newTaskAssigneeSuggIdx = -1
	case newTaskFieldTags:
		m.newTaskTagInput += text
		m.newTaskTagSuggIdx = -1
	}
	return m, nil
}

// handleNewTaskKey processes the task form's keys. With no field entered,
// ↑↓/jk move the field cursor and Enter enters the field under it. Inside a
// field, Enter leaves it and parks the cursor on the next one. Ctrl+S creates
// the task from any state; on Assignee, ↑↓ walk the suggestions.
func (m Model) handleNewTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.newTaskFieldActive {
		return m.handleNewTaskCursorKey(msg.String())
	}
	return m.handleNewTaskFieldKey(msg)
}

// handleNewTaskCursorKey navigates the field list without entering any field.
func (m Model) handleNewTaskCursorKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.newTaskClose()
		return m, nil
	case "ctrl+s":
		return m.newTaskSubmit()
	case "j", "down":
		return m.newTaskMoveCursor(1)
	case "k", "up":
		return m.newTaskMoveCursor(-1)
	case "enter":
		m.newTaskFieldActive = true
		cmd := m.newTaskSyncFocus()
		return m, cmd
	}
	return m, nil
}

// newTaskMoveCursor moves the field cursor, keeping every field closed.
func (m Model) newTaskMoveCursor(delta int) (tea.Model, tea.Cmd) {
	m.newTaskFieldIdx = (m.newTaskFieldIdx + delta + newTaskFieldCount) % newTaskFieldCount
	m.newTaskAssigneeSuggIdx = -1
	m.newTaskTagSuggIdx = -1
	m.newTaskTextarea.Blur()
	return m, nil
}

// newTaskLeaveField commits the pending value of the entered field, closes it
// and parks the cursor on the next one.
func (m Model) newTaskLeaveField() (tea.Model, tea.Cmd) {
	m.newTaskFieldActive = false
	return m.newTaskMoveField(1)
}

// handleNewTaskFieldKey edits the field that was entered. Enter leaves the
// field and advances; Ctrl+S creates the task.
func (m Model) handleNewTaskFieldKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		// Esc leaves the field without closing the form. The pending tag is
		// committed so the typed text is not lost; the suggestion highlight
		// is discarded.
		m.newTaskFieldActive = false
		m.newTaskAssigneeSuggIdx = -1
		m.newTaskTagSuggIdx = -1
		if m.newTaskFieldIdx == newTaskFieldTags {
			m.newTaskCommitTag()
		}
		m.newTaskTextarea.Blur()
		return m, nil
	case "ctrl+s":
		return m.newTaskSubmit()
	case "tab", "shift+tab":
		// Tabs no longer move fields: Enter enters and leaves them.
		return m, nil
	case "enter":
		return m.newTaskLeaveField()
	}

	switch m.newTaskFieldIdx {
	case newTaskFieldPriority:
		switch key {
		case "left":
			if m.newTaskPriority > model.PriorityNone {
				m.newTaskPriority--
			}
		case "right":
			if m.newTaskPriority < model.PriorityHigh {
				m.newTaskPriority++
			}
		case "1", "2", "3", "4":
			m.newTaskPriority = int(key[0] - '0')
		default:
			// Typing a letter over the selector jumps to the title and inserts it.
			if len(key) == 1 && key[0] >= 33 {
				m.newTaskFieldIdx = newTaskFieldTitle
				editTextInput(&m.newTaskTitle, key)
				m.newTaskErr = ""
			}
		}

	case newTaskFieldTitle:
		editTextInput(&m.newTaskTitle, key)
		m.newTaskErr = ""

	case newTaskFieldDescription:
		var cmd tea.Cmd
		m.newTaskTextarea, cmd = m.newTaskTextarea.Update(msg)
		return m, cmd

	case newTaskFieldAssignee:
		switch key {
		case "up":
			m.newTaskAssigneeSuggIdx = cycleIndex(m.newTaskAssigneeSuggIdx, len(m.assigneeSuggestions()), -1)
			return m, nil
		case "down":
			m.newTaskAssigneeSuggIdx = cycleIndex(m.newTaskAssigneeSuggIdx, len(m.assigneeSuggestions()), 1)
			return m, nil
		default:
			editTextInput(&m.newTaskAssignee, key)
			m.newTaskAssigneeSuggIdx = -1
		}

	case newTaskFieldTags:
		switch key {
		case "up":
			m.newTaskTagSuggIdx = cycleIndex(m.newTaskTagSuggIdx, len(m.tagFieldSuggestions()), -1)
			return m, nil
		case "down":
			m.newTaskTagSuggIdx = cycleIndex(m.newTaskTagSuggIdx, len(m.tagFieldSuggestions()), 1)
			return m, nil
		case ",":
			if m.newTaskTagSuggIdx >= 0 {
				suggs := m.tagFieldSuggestions()
				if inRange(m.newTaskTagSuggIdx, len(suggs)) {
					m.newTaskTagInput = suggs[m.newTaskTagSuggIdx]
				}
			}
			m.newTaskCommitTag()
			return m, nil
		case "backspace":
			// With an empty input, backspace deletes the last added tag.
			if m.newTaskTagInput == "" && len(m.newTaskTags) > 0 {
				m.newTaskTags = m.newTaskTags[:len(m.newTaskTags)-1]
				return m, nil
			}
			editTextInput(&m.newTaskTagInput, key)
			m.newTaskTagSuggIdx = -1
		default:
			editTextInput(&m.newTaskTagInput, key)
			m.newTaskTagSuggIdx = -1
		}
	}

	return m, nil
}

// newTaskRow draws a label/value row with the focus highlighted. The prefix
// measures the same with or without focus so that the rows stay aligned.
func newTaskRow(label, value string, focused bool) string {
	lbl := cellWidth(label, 11)
	if focused {
		return "  ▸ " + styleTitle.Render(lbl) + " " + value
	}
	return "    " + styleStatusDesc.Render(lbl) + " " + value
}

// newTaskSection draws a multi-line section label (Description).
func newTaskSection(label string, focused bool) string {
	if focused {
		return "  ▸ " + styleTitle.Render(label)
	}
	return "    " + styleStatusDesc.Render(label)
}

// newTaskOptionalRow draws an optional field: label in tag color and an
// "optional" badge that tells it apart visually from the required fields.
func newTaskOptionalRow(label, value string, focused bool) string {
	lbl := cellWidth(label, 11)
	badge := styleWarn.Render("optional")
	prefix := "    "
	if focused {
		prefix = "  ▸ "
	}
	return prefix + styleProjectTag.Render(lbl) + " " + badge + "  " + value
}

// renderNewTaskTags draws the chosen tags as chips plus the pending input.
func (m Model) renderNewTaskTags() string {
	parts := make([]string, 0, len(m.newTaskTags))
	for _, t := range m.newTaskTags {
		parts = append(parts, styleProjectTag.Render("["+t+"]"))
	}

	input := m.newTaskTagInput
	if m.newTaskFieldIdx == newTaskFieldTags {
		input += styleTitle.Render(cursorGlyph)
	}

	// The hint is decided on what was typed, not on input: input already carries
	// the cursor when the field is focused, so comparing it to "" was always
	// false and the "type to add…" was never seen.
	if len(parts) == 0 && strings.TrimSpace(m.newTaskTagInput) == "" {
		if m.newTaskFieldIdx == newTaskFieldTags {
			return styleDim.Render("type to add…") + input
		}
		return styleDim.Render("—")
	}
	if input != "" {
		parts = append(parts, input)
	}
	return strings.Join(parts, " ")
}

// renderTagFieldSuggestions lists the available tags dropdown and, if what is
// typed does not exist yet, hints that a new one will be created.
func (m Model) renderTagFieldSuggestions() []string {
	var lines []string
	for i, s := range m.tagFieldSuggestions() {
		if i == m.newTaskTagSuggIdx {
			lines = append(lines, styleSelected.Render("      ▸ "+s))
		} else {
			lines = append(lines, "        "+s)
		}
	}
	typed := strings.TrimSpace(m.newTaskTagInput)
	if typed != "" && !containsFold(m.uniqueTags(), typed) && !containsFold(m.newTaskTags, typed) {
		lines = append(lines, styleDim.Render("      + new: "+typed))
	}
	return lines
}

// renderNewTaskPriority shows the priority as radios: the chosen one solid.
func (m Model) renderNewTaskPriority() string {
	labels := []string{"none", "low", "med", "high"}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == m.newTaskPriority {
			parts[i] = styleSelected.Render("● " + l)
		} else {
			parts[i] = styleDim.Render("○ " + l)
		}
	}
	return strings.Join(parts, "  ")
}

// renderNewTaskAssigneeSuggestions lists the assignees dropdown and, if the
// typed text does not exist yet, hints that a new one will be created.
func (m Model) renderNewTaskAssigneeSuggestions() []string {
	var lines []string
	for i, s := range m.assigneeSuggestions() {
		if i == m.newTaskAssigneeSuggIdx {
			lines = append(lines, styleSelected.Render("      ▸ "+s))
		} else {
			lines = append(lines, "        "+s)
		}
	}
	typed := strings.TrimSpace(m.newTaskAssignee)
	if typed != "" && !containsFold(m.uniqueAssignees(), typed) {
		lines = append(lines, styleDim.Render("      ✎ new: "+typed))
	}
	return lines
}

// renderNewTaskModal renders the creation modal: priority, title,
// description (textarea reused from the detail) and assignee with autocomplete.
func (m *Model) renderNewTaskModal(content string) string {
	w := m.width
	lines := []string{""}

	priorityFocused := m.newTaskFieldActive && m.newTaskFieldIdx == newTaskFieldPriority
	lines = append(lines, newTaskRow("Priority", m.renderNewTaskPriority(), priorityFocused))

	titleFocused := m.newTaskFieldActive && m.newTaskFieldIdx == newTaskFieldTitle
	titleInput := m.newTaskTitle
	if titleFocused {
		titleInput += styleTitle.Render(cursorGlyph)
	}
	lines = append(lines, newTaskRow("Title", titleInput, titleFocused))
	if m.newTaskErr != "" {
		lines = append(lines, styleError.Render("    ⚠ "+m.newTaskErr))
	}

	descriptionFocused := m.newTaskFieldActive && m.newTaskFieldIdx == newTaskFieldDescription
	lines = append(lines, newTaskSection("Description", descriptionFocused))
	if strings.TrimSpace(m.newTaskTextarea.Value()) == "" && !descriptionFocused {
		lines = append(lines, styleDim.Render("    (empty · Enter to edit)"))
	} else {
		for _, dl := range strings.Split(m.newTaskTextarea.View(), "\n") {
			lines = append(lines, "    "+dl)
		}
	}

	assigneeFocused := m.newTaskFieldActive && m.newTaskFieldIdx == newTaskFieldAssignee
	assigneeInput := m.newTaskAssignee
	if assigneeFocused {
		assigneeInput += styleTitle.Render(cursorGlyph)
	}
	lines = append(lines, newTaskRow("Assignee", assigneeInput, assigneeFocused))
	if assigneeFocused {
		lines = append(lines, m.renderNewTaskAssigneeSuggestions()...)
	}

	tagsFocused := m.newTaskFieldActive && m.newTaskFieldIdx == newTaskFieldTags
	lines = append(lines, newTaskOptionalRow("Tags", m.renderNewTaskTags(), tagsFocused))
	if tagsFocused {
		lines = append(lines, m.renderTagFieldSuggestions()...)
	}

	totalWidth := modalWidthFor(newTaskModalWidth, w)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	boxTitle := " New Task · " + m.newTaskProject + " "
	return overlayModal(content, renderModalBox(boxTitle, lines, totalWidth), totalWidth, w)
}
