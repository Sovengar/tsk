package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tsk/internal/model"
)

// descEditorHeight es el alto (en filas) del textarea del editor inline.
const descEditorHeight = 8

// descEditTarget devuelve la tarea a editar: la del detalle si está abierto, si
// no la seleccionada en la vista activa.
func (m *Model) descEditTarget() *model.Task {
	if m.detailOpen && m.detailTask != nil {
		return m.detailTask
	}
	return m.selectedEditableTask()
}

// descEditorWidth devuelve el ancho del textarea integrado en la caja del
// detalle: descuenta los 2 bordes y la indentación de 2 columnas.
func (m Model) descEditorWidth() int {
	if w := m.width - 4; w > 0 {
		return w
	}
	return 1
}

// resizeDescEditor reajusta el textarea tras un cambio de terminal.
func (m *Model) resizeDescEditor() {
	m.descEditTextarea.SetWidth(m.descEditorWidth())
}

// openDescEditor abre el detalle en modo edición de descripción. Si el detalle
// no estaba abierto, lo abre (y carga sus comentarios) para poder integrar el
// textarea dentro de la misma caja, sin superponer modales.
func (m *Model) openDescEditor() tea.Cmd {
	src := m.descEditTarget()
	if src == nil {
		return nil
	}
	t := *src // copia: no aliasar el cache de tareas filtradas

	firstOpen := !m.detailOpen
	m.detailOpen = true
	m.detailTask = &t
	m.detailCommentSel = -1
	m.descEditHadDetail = !firstOpen

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "Escribe la descripción…"
	ta.CharLimit = 0

	// Estilos sobrios: sin números de línea ni fondo en la línea del cursor,
	// para que el textarea combine con la caja del detalle.
	st := textarea.DefaultDarkStyles()
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.CursorLineNumber = lipgloss.NewStyle()
	st.Focused.LineNumber = lipgloss.NewStyle()
	st.Focused.Prompt = lipgloss.NewStyle()
	st.Focused.Placeholder = styleDim
	st.Focused.Text = lipgloss.NewStyle()
	ta.SetStyles(st)

	ta.SetValue(t.Description)
	ta.SetWidth(m.descEditorWidth())
	ta.SetHeight(descEditorHeight)
	ta.CursorEnd()

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

// closeDescEditor sale del modo edición. Si el detalle no estaba abierto antes
// de editar, lo cierra para volver a la vista de origen.
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

// handleDescEditKey procesa las teclas del editor inline. Esc cancela y Ctrl+S
// guarda; el resto se delega al textarea, así que Enter inserta un salto.
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
			// Solo reflejar el cambio si el detalle sigue visible.
			if m.detailOpen && m.detailTask != nil && m.detailTask.ID == id {
				m.detailTask.Description = desc
			}
			return m, m.saveDescriptionCmd(id, desc)
		case "ctrl+c":
			// Copiar la selección; sin selección no hace nada (no cierra la TUI).
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

// descEditorLines arma las líneas del textarea indentadas para encajar en la
// caja del detalle (2 columnas, igual que la metadata y la descripción normal).
func (m *Model) descEditorLines() []string {
	lines := strings.Split(m.descEditTextarea.View(), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return lines
}

// saveDescriptionCmd persiste la descripción y recarga el listado.
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
