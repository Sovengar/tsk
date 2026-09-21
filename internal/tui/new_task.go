package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// newTaskFieldIdx identifica el campo activo del alta de tarea, en el orden en
// que el usuario los recorre con Tab.
const (
	newTaskFieldPriority = iota
	newTaskFieldTitle
	newTaskFieldDescription
	newTaskFieldAssignee
	newTaskFieldTags
	newTaskFieldCount
)

const (
	// newTaskModalWidth es el ancho preferido del modal (se recorta en
	// terminales angostas).
	newTaskModalWidth = 64
	// newTaskDescHeight es el alto del textarea de descripción embebido.
	newTaskDescHeight = 4
	// newTaskMaxSuggestions acota el dropdown de assignees.
	newTaskMaxSuggestions = 5
)

// cursorGlyph es el cursor de texto de los campos simples.
const cursorGlyph = "▏"

// newTaskKeybinds lista las teclas del alta de tarea. Es la fuente única para la
// barra de keybinds y el modal de ayuda.
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

// assigneeSuggestions devuelve los assignees conocidos que matchean (fuzzy) con
// lo escrito, priorizando coincidencias tempranas. Con el input vacío lista
// todos. Excluye el match exacto: si ya escribiste el nombre completo no hay
// nada que completar, así no se muestra duplicado ni se selecciona una fila
// fantasma con ↑↓.
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

// tagFieldSuggestions devuelve las tags conocidas que matchean (fuzzy) con lo
// escrito, excluyendo las ya agregadas y el match exacto. Con el input vacío
// lista todas.
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

// newTaskCommitTag agrega la tag pendiente (lo tipeado, ya sea una sugerencia o
// un valor nuevo), normalizada y sin duplicar, y limpia el input.
func (m *Model) newTaskCommitTag() {
	for _, t := range model.ParseTags(m.newTaskTagInput) {
		if !containsFold(m.newTaskTags, t) {
			m.newTaskTags = append(m.newTaskTags, t)
		}
	}
	m.newTaskTagInput = ""
	m.newTaskTagSuggIdx = -1
}

// newTaskTextareaWidth es el ancho del textarea embebido: ancho interior del
// modal menos los bordes y la indentación de 4 columnas de las filas.
func (m Model) newTaskTextareaWidth() int {
	w := modalWidthFor(newTaskModalWidth, m.width) - 6
	if w < 10 {
		w = 10
	}
	return w
}

// buildNewTaskTextarea crea el editor de descripción del alta, reutilizando el
// mismo textarea que el detalle de tarea.
func (m Model) buildNewTaskTextarea(value string) textarea.Model {
	return newDescTextarea(value, m.newTaskTextareaWidth(), newTaskDescHeight)
}

// newTaskSyncFocus enfoca o desenfoca el textarea según el campo activo.
func (m *Model) newTaskSyncFocus() tea.Cmd {
	if m.newTaskFieldIdx == newTaskFieldDescription {
		return m.newTaskTextarea.Focus()
	}
	m.newTaskTextarea.Blur()
	return nil
}

// newTaskMoveField completa la sugerencia activa (si estamos en Assignee) y
// avanza/retrocede el foco entre campos, respetando el foco del textarea.
func (m Model) newTaskMoveField(delta int) (tea.Model, tea.Cmd) {
	if m.newTaskFieldIdx == newTaskFieldAssignee && m.newTaskAssigneeSuggIdx >= 0 {
		suggs := m.assigneeSuggestions()
		if m.newTaskAssigneeSuggIdx < len(suggs) {
			m.newTaskAssignee = suggs[m.newTaskAssigneeSuggIdx]
		}
		m.newTaskAssigneeSuggIdx = -1
	}
	// Al salir de Tags, la tag a medio tipear no se pierde.
	if m.newTaskFieldIdx == newTaskFieldTags {
		m.newTaskCommitTag()
	}
	m.newTaskFieldIdx = (m.newTaskFieldIdx + delta + newTaskFieldCount) % newTaskFieldCount
	cmd := m.newTaskSyncFocus()
	return m, cmd
}

// newTaskSubmit valida y crea la tarea. Si falta el título, deja el error inline
// y devuelve el foco a Title sin cerrar el modal.
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
		if m.newTaskAssigneeSuggIdx < len(suggs) {
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

// newTaskClose cierra el modal y limpia el estado transitorio.
func (m *Model) newTaskClose() {
	m.newTaskOpen = false
	m.newTaskErr = ""
	m.newTaskTitle = ""
	m.newTaskAssignee = ""
	m.newTaskAssigneeSuggIdx = -1
	m.newTaskTextarea.Blur()
}

// createTaskCmd persiste la tarea y recarga el listado.
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

// handleNewTaskPaste inserta texto pegado en el campo activo.
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

// handleNewTaskKey procesa las teclas del alta de tarea.
//
// Navegación: Tab/Shift+Tab mueven el foco. Ctrl+S crea desde cualquier campo.
// Enter avanza en Priority, crea en Title, inserta salto en Description y
// completa-o-crea en Assignee; en Assignee, ↑↓ recorren las sugerencias.
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
			// Tipear una letra sobre el selector salta al título y la inserta.
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
			suggs := m.assigneeSuggestions()
			if len(suggs) > 0 {
				if m.newTaskAssigneeSuggIdx <= 0 {
					m.newTaskAssigneeSuggIdx = len(suggs) - 1
				} else {
					m.newTaskAssigneeSuggIdx--
				}
			}
			return m, nil
		case "down":
			suggs := m.assigneeSuggestions()
			if len(suggs) > 0 {
				m.newTaskAssigneeSuggIdx = (m.newTaskAssigneeSuggIdx + 1) % len(suggs)
			}
			return m, nil
		case "enter":
			if m.newTaskAssigneeSuggIdx >= 0 {
				suggs := m.assigneeSuggestions()
				if m.newTaskAssigneeSuggIdx < len(suggs) {
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
			suggs := m.tagFieldSuggestions()
			if len(suggs) > 0 {
				if m.newTaskTagSuggIdx <= 0 {
					m.newTaskTagSuggIdx = len(suggs) - 1
				} else {
					m.newTaskTagSuggIdx--
				}
			}
			return m, nil
		case "down":
			suggs := m.tagFieldSuggestions()
			if len(suggs) > 0 {
				m.newTaskTagSuggIdx = (m.newTaskTagSuggIdx + 1) % len(suggs)
			}
			return m, nil
		case "enter", ",":
			if m.newTaskTagSuggIdx >= 0 {
				suggs := m.tagFieldSuggestions()
				if m.newTaskTagSuggIdx < len(suggs) {
					m.newTaskTagInput = suggs[m.newTaskTagSuggIdx]
				}
			}
			m.newTaskCommitTag()
			return m, nil
		case "backspace":
			// Con el input vacío, backspace borra la última tag agregada.
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

// newTaskRow dibuja una fila etiqueta/valor con el foco resaltado. El prefijo
// mide lo mismo con o sin foco para que las filas queden alineadas.
func newTaskRow(label, value string, focused bool) string {
	lbl := cellWidth(label, 11)
	if focused {
		return "  ▸ " + styleTitle.Render(lbl) + " " + value
	}
	return "    " + styleStatusDesc.Render(lbl) + " " + value
}

// newTaskSection dibuja una etiqueta de sección multilínea (Description).
func newTaskSection(label string, focused bool) string {
	if focused {
		return "  ▸ " + styleTitle.Render(label)
	}
	return "    " + styleStatusDesc.Render(label)
}

// newTaskOptionalRow dibuja un campo opcional: etiqueta en color de tags y un
// badge "optional" que lo distingue visualmente de los campos obligatorios.
func newTaskOptionalRow(label, value string, focused bool) string {
	lbl := cellWidth(label, 11)
	badge := styleWarn.Render("optional")
	prefix := "    "
	if focused {
		prefix = "  ▸ "
	}
	return prefix + styleProjectTag.Render(lbl) + " " + badge + "  " + value
}

// renderNewTaskTags dibuja las tags elegidas como chips más el input pendiente.
func (m Model) renderNewTaskTags() string {
	parts := make([]string, 0, len(m.newTaskTags))
	for _, t := range m.newTaskTags {
		parts = append(parts, styleProjectTag.Render("["+t+"]"))
	}

	input := m.newTaskTagInput
	if m.newTaskFieldIdx == newTaskFieldTags {
		input += styleTitle.Render(cursorGlyph)
	}

	if len(parts) == 0 && input == "" {
		if m.newTaskFieldIdx == newTaskFieldTags {
			return styleDim.Render("type to add…")
		}
		return styleDim.Render("—")
	}
	if input != "" {
		parts = append(parts, input)
	}
	return strings.Join(parts, " ")
}

// renderTagFieldSuggestions lista el dropdown de tags disponibles y, si lo
// escrito no existe todavía, insinúa que se creará una nueva.
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

// renderNewTaskPriority muestra la prioridad como radios: la elegida sólida.
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

// renderNewTaskAssigneeSuggestions lista el dropdown de assignees y, si el texto
// escrito no existe todavía, insinúa que se creará uno nuevo.
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

// renderNewTaskModal renderiza el modal de creación: prioridad, título,
// descripción (textarea reutilizado del detalle) y assignee con autocompletado.
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
	innerWidth := totalWidth - 2
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	boxTitle := " New Task · " + m.newTaskProject + " "
	return overlayModal(content, renderModalBox(boxTitle, lines, totalWidth), totalWidth, w)
}
