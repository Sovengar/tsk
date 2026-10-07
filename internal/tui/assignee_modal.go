package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// Row caps of the assignee modals: they bound the height so it does not grow
// out of control with lots of data and does not push the keybinds off screen.
const (
	assigneeModalMaxRows  = 10
	assigneeModalTaskRows = 5
	assigneeModalOffRows  = 6
)

// assigneeSummary is a row of the roster: one person and their counts.
type assigneeSummary struct {
	Name        string
	Total       int
	Active      int
	OffDayCount int
}

// assigneeRoster builds the modal's people list: the union of the assignees
// of tasks, assignees with off-days and "Me" (always present). It excludes
// the "no assignee" placeholder. Stable alphabetical order.
// assigneeRoster delegates to the pure function: the rule does not depend on the model.
func (m Model) assigneeRoster() []assigneeSummary {
	return buildAssigneeRoster(m.tasks, m.offdays)
}

// currentAssignee returns the person selected in the modal, or "".
func (m *Model) currentAssignee() string {
	return nameAt(m.assigneeRoster(), m.assigneeIdx)
}

// assigneeActiveTasks are the non-closed tasks of a person.
func (m Model) assigneeActiveTasks(name string) []model.Task {
	return tasksForAssignee(m.tasks, name)
}

// assigneeOffDays are the off-days of a person, in load order (ascending
// start date, which is how the DB returns them).
func (m Model) assigneeOffDays(name string) []model.OffDay {
	return offDaysForAssignee(m.offdays, name)
}

// openAssigneeModal opens the modal at the list level.
func (m *Model) openAssigneeModal() {
	m.assigneeModalOpen = true
	m.assigneeDetail = false
	m.assigneeIdx = 0
	m.assigneeOffdayIdx = 0
}

// openOffdayForm opens the empty creation form for the selected person.
func (m *Model) openOffdayForm() tea.Cmd {
	m.offdayFormOpen = true
	m.offdayFormField = 0
	m.offdayStartInput = ""
	m.offdayEndInput = ""
	m.offdayNoteInput = ""
	return nil
}

// clampAssigneeIdx keeps the roster index in range.
func (m *Model) clampAssigneeIdx() {
	m.assigneeIdx = clampTo(m.assigneeIdx, len(m.assigneeRoster()))
}

// clampOffdayIdx keeps the off-days index in range.
func (m *Model) clampOffdayIdx() {
	m.assigneeOffdayIdx = clampTo(m.assigneeOffdayIdx, len(m.assigneeOffDays(m.currentAssignee())))
}

// selectionPrefix returns the selected-row indicator.
func selectionPrefix(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}

// handleAssigneeModalKey processes the modal's keys, both at the level of the
// people list and at the detail one.
func (m Model) handleAssigneeModalKey(key string) (tea.Model, tea.Cmd) {
	m.clampAssigneeIdx()

	if m.assigneeDetail {
		offs := m.assigneeOffDays(m.currentAssignee())
		switch key {
		case "esc":
			m.assigneeDetail = false
			return m, nil
		case "j", "down":
			m.assigneeOffdayIdx = shiftIndex(m.assigneeOffdayIdx, len(offs), 1)
		case "k", "up":
			m.assigneeOffdayIdx = shiftIndex(m.assigneeOffdayIdx, len(offs), -1)
		case "a":
			if m.currentAssignee() != "" {
				return m, m.openOffdayForm()
			}
		case "d", "x":
			if inRange(m.assigneeOffdayIdx, len(offs)) {
				m.confirmOpen = true
				m.confirmAction = "delete-offday"
				m.confirmOffday = offs[m.assigneeOffdayIdx]
			}
		}
		return m, nil
	}

	roster := m.assigneeRoster()
	switch key {
	case "esc":
		m.assigneeModalOpen = false
	case "j", "down":
		m.assigneeIdx = shiftIndex(m.assigneeIdx, len(roster), 1)
	case "k", "up":
		m.assigneeIdx = shiftIndex(m.assigneeIdx, len(roster), -1)
	case "enter":
		// The roster is never empty, so there is always somebody to enter.
		m.assigneeDetail = true
		m.assigneeOffdayIdx = 0
	case "a":
		return m, m.openOffdayForm()
	}
	return m, nil
}

// handleOffdayFormKey processes the keys of the off-day creation form.
func (m Model) handleOffdayFormKey(key string) (tea.Model, tea.Cmd) {
	const numFields = 3

	switch key {
	case "esc":
		m.offdayFormOpen = false
		return m, nil
	case "tab":
		m.offdayFormField = (m.offdayFormField + 1) % numFields
		return m, nil
	case "shift+tab":
		m.offdayFormField = (m.offdayFormField + numFields - 1) % numFields
		return m, nil
	case "enter":
		name := m.currentAssignee()
		if name == "" {
			return m, m.setToast("No assignee selected", "error")
		}
		if m.offdayStartInput == "" {
			return m, m.setToast("Start date is required (YYYY-MM-DD)", "error")
		}
		m.offdayFormOpen = false
		return m, m.addOffDayCmd(name, m.offdayStartInput, m.offdayEndInput, m.offdayNoteInput)
	}

	var target *string
	switch m.offdayFormField {
	case 0:
		target = &m.offdayStartInput
	case 1:
		target = &m.offdayEndInput
	case 2:
		target = &m.offdayNoteInput
	}
	editTextInput(target, key)
	return m, nil
}

// addOffDayCmd persists an off-day and emits offdaySavedMsg with the result.
func (m Model) addOffDayCmd(name, start, end, note string) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.database.AddOffDay(name, start, end, note); err != nil {
			return offdaySavedMsg{err: err, action: "add", name: name}
		}
		return offdaySavedMsg{action: "add", name: name}
	}
}

// deleteOffDayCmd deletes an off-day and emits offdaySavedMsg with the result.
func (m Model) deleteOffDayCmd(id int64, name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.database.DeleteOffDay(id); err != nil {
			return offdaySavedMsg{err: err, action: "delete", name: name}
		}
		return offdaySavedMsg{action: "delete", name: name}
	}
}

// handleOffdaySaved applies the result of an off-day create or delete. On
// error it reopens the form so the written text is not lost; on success it
// reloads the off-days so the Gantt stays consistent.
func (m Model) handleOffdaySaved(msg offdaySavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if msg.action == "add" {
			m.offdayFormOpen = true
		}
		return m, m.setToast(msg.err.Error(), "error")
	}

	var text string
	switch msg.action {
	case "add":
		text = fmt.Sprintf("Off-day added for %s", msg.name)
		m.assigneeDetail = true
	case "delete":
		text = fmt.Sprintf("Off-day deleted for %s", msg.name)
	default:
		text = "Done"
	}
	return m, tea.Batch(m.setToast(text, "info"), m.loadOffDays())
}

// renderAssigneeModal draws the assignee modal: the people list or, if you
// entered one, their detail with active tasks and off-days.
func (m *Model) renderAssigneeModal(content string) string {
	// The clamp goes before the fork: the detail uses currentAssignee, which
	// is the roster element at assigneeIdx, and without bounding the index an
	// old assigneeIdx would paint a detail without a name. It used to live
	// after, and the detail path depended on somebody else having run it.
	m.clampAssigneeIdx()

	if m.assigneeDetail {
		return m.renderAssigneeDetail(content)
	}

	roster := m.assigneeRoster()

	// The roster is never empty: buildAssigneeRoster always puts "Me" in.
	// That is why there is no branch for the no-people case; the
	// "(no assignees)" that was here was unreachable.
	lines := []string{""}
	start, end := visibleRange(m.assigneeIdx, len(roster), assigneeModalMaxRows)
	for i, s := range roster[start:end] {
		i += start
		line := fmt.Sprintf("%s%s %2d tasks (%d active)  %d off-days",
			selectionPrefix(i == m.assigneeIdx), cellWidth(s.Name, 16), s.Total, s.Active, s.OffDayCount)
		if i == m.assigneeIdx {
			line = styleSelected.Render(line)
		}
		lines = append(lines, line)
	}
	if len(roster) > assigneeModalMaxRows {
		lines = append(lines, styleDim.Render(fmt.Sprintf("  %d/%d", m.assigneeIdx+1, len(roster))))
	}

	totalWidth := modalWidthFor(58, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Assignees ", lines, totalWidth), totalWidth, m.width)
}

// renderAssigneeDetail draws the detail of the selected person.
func (m *Model) renderAssigneeDetail(content string) string {
	// The roster is never empty and the index is already bounded, so there is
	// a person. The branch that went back to the list when the name came out
	// empty was unreachable; what replaced it, the clamp above, is needed.
	name := m.currentAssignee()
	m.clampOffdayIdx()

	tasks := m.assigneeActiveTasks(name)
	offs := m.assigneeOffDays(name)

	lines := []string{"", "  " + styleSelected.Render(name), ""}

	lines = append(lines, styleColumnHeader.Render("  Active tasks"))
	if len(tasks) == 0 {
		lines = append(lines, styleDim.Render("  (none)"))
	}
	for i, t := range tasks {
		if i >= assigneeModalTaskRows {
			lines = append(lines, styleDim.Render(fmt.Sprintf("  ... %d more", len(tasks)-assigneeModalTaskRows)))
			break
		}
		lines = append(lines, fmt.Sprintf("  #%d %s", t.ID, truncate(t.Title, 34)))
	}

	lines = append(lines, "", styleColumnHeader.Render("  Off-days"))
	if len(offs) == 0 {
		lines = append(lines, styleDim.Render("  (none)"))
	}
	start, end := visibleRange(m.assigneeOffdayIdx, len(offs), assigneeModalOffRows)
	for i, o := range offs[start:end] {
		i += start
		note := ""
		if o.Note != "" {
			note = "  " + o.Note
		}
		line := fmt.Sprintf("%s%s → %s%s", selectionPrefix(i == m.assigneeOffdayIdx), o.StartDate, o.EndDate, note)
		if i == m.assigneeOffdayIdx {
			line = styleSelected.Render(line)
		}
		lines = append(lines, line)
	}
	if len(offs) > assigneeModalOffRows {
		lines = append(lines, styleDim.Render(fmt.Sprintf("  %d/%d", m.assigneeOffdayIdx+1, len(offs))))
	}

	totalWidth := modalWidthFor(58, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Assignee · "+name+" ", lines, totalWidth), totalWidth, m.width)
}

// renderOffdayForm draws the creation form over the assignee modal.
func (m *Model) renderOffdayForm(content string) string {
	start := m.offdayStartInput
	if m.offdayFormField == 0 {
		start += "_"
	}
	end := m.offdayEndInput
	if m.offdayFormField == 1 {
		end += "_"
	}
	note := m.offdayNoteInput
	if m.offdayFormField == 2 {
		note += "_"
	}

	lines := []string{
		fmt.Sprintf("  Person: %s", m.currentAssignee()),
		fmt.Sprintf("  Start:  %s", start),
		fmt.Sprintf("  End:    %s", end),
		fmt.Sprintf("  Note:   %s", note),
		styleDim.Render("  End/Note optional · dates YYYY-MM-DD"),
	}

	totalWidth := modalWidthFor(58, m.width)
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" New off-day ", lines, totalWidth), totalWidth, m.width)
}
