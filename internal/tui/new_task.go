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
		{"Ctrl+S", "create"},
		{"Tab", "next field"},
		{"Enter", "next / add tag"},
		{",", "add tag"},
		{"↑↓", "suggestions"},
		{"←→/1-4", "priority"},
		{"Esc", "cancel"},
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

// newTaskMoveField completes the active suggestion (if we are on Assignee) and
// moves the focus forward/backward between fields, respecting the textarea's focus.
func (m Model) newTaskMoveField(delta int) (tea.Model, tea.Cmd) {
	if m.newTaskFieldIdx == newTaskFieldAssignee && m.newTaskAssigneeSuggIdx >= 0 {
		suggs := m.assigneeSuggestions()
		if inRange(m.newTaskAssigneeSuggIdx, len(suggs)) {
			m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
		}
		m.newTaskAssigneeSuggIdx = -1
	}
	// On leaving Tags, the half-typed tag is not lost.
	if m.newTaskFieldIdx == newTaskFieldTags {
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
		m.newTaskTextarea.Blur()
		return m, nil
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

	m.newTaskClose()
	return m, tea.Batch(
		m.createTaskCmd(project, title, description, assignee, priority, tags),
		m.setToast("Task created", "info"),
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

// handleNewTaskKey processes the task form's keys.
//
// Navigation: Tab/Shift+Tab move the focus. Ctrl+S creates from any field.
// Enter advances on Priority, creates on Title, inserts a break in Description
// and complete-or-create on Assignee; on Assignee, ↑↓ walk the suggestions.
func (m Model) handleNewTaskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.newTaskClose()
		return m, nil
	case "ctrl+s":
		return m.newTaskSubmit()
	case "tab":
		return m.newTaskMoveField(1)
	case "shift+tab":
		return m.newTaskMoveField(-1)
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
		case "enter":
			return m.newTaskMoveField(1)
		default:
			// Typing a letter over the selector jumps to the title and inserts it.
			if len(key) == 1 && key[0] >= 33 {
				m.newTaskFieldIdx = newTaskFieldTitle
				editTextInput(&m.newTaskTitle, key)
				m.newTaskErr = ""
			}
		}

	case newTaskFieldTitle:
		if key == "enter" {
			return m.newTaskSubmit()
		}
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
		case "enter":
			if m.newTaskAssigneeSuggIdx >= 0 {
				suggs := m.assigneeSuggestions()
				if inRange(m.newTaskAssigneeSuggIdx, len(suggs)) {
					m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
					m.newTaskAssigneeSuggIdx = -1
					return m, nil
				}
			}
			return m.newTaskSubmit()
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
		case "enter", ",":
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

	lines = append(lines, newTaskRow("Priority", m.renderNewTaskPriority(), m.newTaskFieldIdx == newTaskFieldPriority))

	titleInput := m.newTaskTitle
	if m.newTaskFieldIdx == newTaskFieldTitle {
		titleInput += styleTitle.Render(cursorGlyph)
	}
	lines = append(lines, newTaskRow("Title", titleInput, m.newTaskFieldIdx == newTaskFieldTitle))
	if m.newTaskErr != "" {
		lines = append(lines, styleError.Render("    ⚠ "+m.newTaskErr))
	}

	lines = append(lines, newTaskSection("Description", m.newTaskFieldIdx == newTaskFieldDescription))
	if strings.TrimSpace(m.newTaskTextarea.Value()) == "" && m.newTaskFieldIdx != newTaskFieldDescription {
		lines = append(lines, styleDim.Render("    (empty · Tab to edit)"))
	} else {
		for _, dl := range strings.Split(m.newTaskTextarea.View(), "\n") {
			lines = append(lines, "    "+dl)
		}
	}

	assigneeInput := m.newTaskAssignee
	if m.newTaskFieldIdx == newTaskFieldAssignee {
		assigneeInput += styleTitle.Render(cursorGlyph)
	}
	lines = append(lines, newTaskRow("Assignee", assigneeInput, m.newTaskFieldIdx == newTaskFieldAssignee))
	if m.newTaskFieldIdx == newTaskFieldAssignee {
		lines = append(lines, m.renderNewTaskAssigneeSuggestions()...)
	}

	tagsFocused := m.newTaskFieldIdx == newTaskFieldTags
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
