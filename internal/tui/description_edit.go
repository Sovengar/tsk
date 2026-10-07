package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tsk/internal/model"
)

// descEditorHeight is the height (in rows) of the inline editor's textarea.
const descEditorHeight = 8

// descEditTarget returns the task to edit: the detail's one if it is open,
// otherwise the one selected in the active view.
func (m *Model) descEditTarget() *model.Task {
	if m.detailOpen && m.detailTask != nil {
		return m.detailTask
	}
	return m.selectedEditableTask()
}

// descEditorWidth returns the width of the textarea embedded in the
// detail's box: it subtracts the 2 borders and the 2-column indentation.
func (m Model) descEditorWidth() int {
	// The floor of 1 avoids a width of 0 or negative for the textarea, which
	// would be left with no room to type. With a terminal of fewer than 4
	// columns there is nowhere to put it either, but it is not a crash.
	return max(m.width-4, 1)
}

// resizeDescEditor readjusts the textarea after a terminal change.
func (m *Model) resizeDescEditor() {
	m.descEditTextarea.SetWidth(m.descEditorWidth())
}

// newDescTextarea builds the description editor's textarea with the
// sober styles shared by the detail and the new-task form. It is the only
// source of configuration of the inline editor.
func newDescTextarea(value string, width, height int) textarea.Model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "Type the description…"
	ta.CharLimit = 0

	// Sober styles: no line numbers and no background on the cursor line,
	// so the textarea matches the detail's box.
	st := textarea.DefaultDarkStyles()
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.CursorLineNumber = lipgloss.NewStyle()
	st.Focused.LineNumber = lipgloss.NewStyle()
	st.Focused.Prompt = lipgloss.NewStyle()
	st.Focused.Placeholder = styleDim
	st.Focused.Text = lipgloss.NewStyle()
	ta.SetStyles(st)

	ta.SetValue(value)
	ta.SetWidth(width)
	ta.SetHeight(height)
	ta.CursorEnd()
	return ta
}

// openDescEditor opens the detail in description-editing mode. If the detail
// was not open, it opens it (and loads its comments) so the textarea can be
// integrated inside the same box, without stacking modals.
func (m *Model) openDescEditor() tea.Cmd {
	src := m.descEditTarget()
	if src == nil {
		return nil
	}
	t := *src // copy: do not alias the filtered task cache

	firstOpen := !m.detailOpen
	m.detailOpen = true
	m.detailTask = &t
	m.detailCommentSel = -1
	m.descEditHadDetail = !firstOpen

	ta := newDescTextarea(t.Description, m.descEditorWidth(), descEditorHeight)

	m.descEditOpen = true
	m.descEditTaskID = t.ID
	m.descEditTextarea = ta

	cmds := []tea.Cmd{m.descEditTextarea.Focus()}
	if firstOpen {
		m.detailComments = nil
		cmds = append(cmds, m.loadCommentsCmd(t.ID))
	}
	return tea.Batch(cmds...)
}

// closeDescEditor leaves the editing mode. If the detail was not open before
// editing, it closes it to return to the source view.
func (m *Model) closeDescEditor() {
	m.descEditOpen = false
	m.descEditTextarea.Blur()
	if !m.descEditHadDetail {
		m.detailOpen = false
		m.detailTask = nil
		m.detailComments = nil
		m.detailCommentSel = -1
	}
}

// handleDescEditKey processes the keys of the inline editor. Esc cancels and
// Ctrl+S saves; the rest is delegated to the textarea, so Enter inserts a line break.
func (m Model) handleDescEditKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.closeDescEditor()
			return m, nil
		case "ctrl+s":
			desc := strings.TrimSpace(m.descEditTextarea.Value())
			id := m.descEditTaskID
			m.closeDescEditor()
			// Only reflect the change if the detail is still visible.
			if m.detailOpen && m.detailTask != nil && m.detailTask.ID == id {
				m.detailTask.Description = desc
			}
			return m, m.saveDescriptionCmd(id, desc)
		case "ctrl+c":
			// Copy the selection; with no selection it does nothing (it does not close the TUI).
			if m.descEditTextarea.HasSelection() {
				return m, m.descEditTextarea.CopySelection()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.descEditTextarea, cmd = m.descEditTextarea.Update(msg)
	return m, cmd
}

// descEditorLines builds the textarea lines indented to fit into the
// detail's box (2 columns, same as the metadata and the normal description).
func (m *Model) descEditorLines() []string {
	lines := strings.Split(m.descEditTextarea.View(), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return lines
}

// saveDescriptionCmd persists the description and reloads the listing.
func (m *Model) saveDescriptionCmd(taskID int64, description string) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.database.UpdateTask(taskID, map[string]any{"description": description}); err != nil {
			return nil
		}
		tasks, err := m.database.ListTasks("", "", "")
		if err != nil {
			return nil
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}
