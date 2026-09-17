package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// Límites de filas de los modales de assignees: acotan el alto para no crecer
// sin control con muchos datos y no empujar los keybinds fuera de pantalla.
const (
	assigneeModalMaxRows  = 10
	assigneeModalTaskRows = 5
	assigneeModalOffRows  = 6
)

// assigneeSummary es una fila del roster: una persona y sus conteos.
type assigneeSummary struct {
	Name        string
	Total       int
	Active      int
	OffDayCount int
}

// assigneeRoster arma la lista de personas del modal: la unión de assignees de
// tareas, assignees con off-days y "Me" (siempre presente). Excluye el
// placeholder de "sin responsable". Orden alfabético estable.
func (m Model) assigneeRoster() []assigneeSummary {
	byName := map[string]*assigneeSummary{}
	ensure := func(name string) *assigneeSummary {
		if s, ok := byName[name]; ok {
			return s
		}
		s := &assigneeSummary{Name: name}
		byName[name] = s
		return s
	}

	ensure("Me")
	for _, t := range m.tasks {
		if model.IsUnassigned(t.Assignee) {
			continue
		}
		s := ensure(t.Assignee)
		s.Total++
		if t.IsActive() {
			s.Active++
		}
	}
	for _, o := range m.offdays {
		if model.IsUnassigned(o.Assignee) {
			continue
		}
		ensure(o.Assignee).OffDayCount++
	}

	result := make([]assigneeSummary, 0, len(byName))
	for _, s := range byName {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// currentAssignee devuelve la persona seleccionada en el modal, o "".
func (m *Model) currentAssignee() string {
	roster := m.assigneeRoster()
	if m.assigneeIdx >= 0 && m.assigneeIdx < len(roster) {
		return roster[m.assigneeIdx].Name
	}
	return ""
}

// assigneeActiveTasks son las tareas no cerradas de una persona.
func (m Model) assigneeActiveTasks(name string) []model.Task {
	var result []model.Task
	for _, t := range m.tasks {
		if t.Assignee == name && t.IsActive() {
			result = append(result, t)
		}
	}
	return result
}

// assigneeOffDays son los off-days de una persona, en el orden de carga (fecha
// de inicio ascendente, que es como los devuelve la DB).
func (m Model) assigneeOffDays(name string) []model.OffDay {
	var result []model.OffDay
	for _, o := range m.offdays {
		if o.Assignee == name {
			result = append(result, o)
		}
	}
	return result
}

// openAssigneeModal abre el modal en el nivel de lista.
func (m *Model) openAssigneeModal() {
	m.assigneeModalOpen = true
	m.assigneeDetail = false
	m.assigneeIdx = 0
	m.assigneeOffdayIdx = 0
}

// openOffdayForm abre el formulario de alta vacío para la persona seleccionada.
func (m *Model) openOffdayForm() tea.Cmd {
	m.offdayFormOpen = true
	m.offdayFormField = 0
	m.offdayStartInput = ""
	m.offdayEndInput = ""
	m.offdayNoteInput = ""
	return nil
}

// clampAssigneeIdx mantiene el índice del roster dentro de rango.
func (m *Model) clampAssigneeIdx() {
	n := len(m.assigneeRoster())
	if n == 0 {
		m.assigneeIdx = 0
		return
	}
	if m.assigneeIdx >= n {
		m.assigneeIdx = n - 1
	}
	if m.assigneeIdx < 0 {
		m.assigneeIdx = 0
	}
}

// clampOffdayIdx mantiene el índice de off-days dentro de rango.
func (m *Model) clampOffdayIdx() {
	n := len(m.assigneeOffDays(m.currentAssignee()))
	if n == 0 {
		m.assigneeOffdayIdx = 0
		return
	}
	if m.assigneeOffdayIdx >= n {
		m.assigneeOffdayIdx = n - 1
	}
	if m.assigneeOffdayIdx < 0 {
		m.assigneeOffdayIdx = 0
	}
}

// selectionPrefix devuelve el indicador de fila seleccionada.
func selectionPrefix(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}

// handleAssigneeModalKey procesa las teclas del modal, tanto en el nivel de
// lista de personas como en el de detalle.
func (m Model) handleAssigneeModalKey(key string) (tea.Model, tea.Cmd) {
	m.clampAssigneeIdx()

	if m.assigneeDetail {
		offs := m.assigneeOffDays(m.currentAssignee())
		switch key {
		case "esc":
			m.assigneeDetail = false
			return m, nil
		case "j", "down":
			if m.assigneeOffdayIdx < len(offs)-1 {
				m.assigneeOffdayIdx++
			}
		case "k", "up":
			if m.assigneeOffdayIdx > 0 {
				m.assigneeOffdayIdx--
			}
		case "a":
			if m.currentAssignee() != "" {
				return m, m.openOffdayForm()
			}
		case "d", "x":
			if m.assigneeOffdayIdx < len(offs) {
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
		if m.assigneeIdx < len(roster)-1 {
			m.assigneeIdx++
		}
	case "k", "up":
		if m.assigneeIdx > 0 {
			m.assigneeIdx--
		}
	case "enter":
		if len(roster) > 0 {
			m.assigneeDetail = true
			m.assigneeOffdayIdx = 0
		}
	case "a":
		if len(roster) > 0 {
			return m, m.openOffdayForm()
		}
	}
	return m, nil
}

// handleOffdayFormKey procesa las teclas del formulario de alta de off-day.
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

// addOffDayCmd persiste un off-day y emite offdaySavedMsg con el resultado.
func (m Model) addOffDayCmd(name, start, end, note string) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.database.AddOffDay(name, start, end, note); err != nil {
			return offdaySavedMsg{err: err, action: "add", name: name}
		}
		return offdaySavedMsg{action: "add", name: name}
	}
}

// deleteOffDayCmd elimina un off-day y emite offdaySavedMsg con el resultado.
func (m Model) deleteOffDayCmd(id int64, name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.database.DeleteOffDay(id); err != nil {
			return offdaySavedMsg{err: err, action: "delete", name: name}
		}
		return offdaySavedMsg{action: "delete", name: name}
	}
}

// handleOffdaySaved aplica el resultado de un alta o baja de off-day. Ante error
// reabre el formulario para no perder lo escrito; ante éxito recarga los
// off-days para que el Gantt quede consistente.
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

// renderAssigneeModal dibuja el modal de assignees: lista de personas o, si se
// entró a una, su detalle con tareas activas y off-days.
func (m *Model) renderAssigneeModal(content string) string {
	if m.assigneeDetail {
		return m.renderAssigneeDetail(content)
	}

	m.clampAssigneeIdx()
	roster := m.assigneeRoster()

	lines := []string{""}
	if len(roster) == 0 {
		lines = append(lines, styleDim.Render("  (no assignees)"))
	}
	start, end := visibleRange(m.assigneeIdx, len(roster), assigneeModalMaxRows)
	for i := start; i < end; i++ {
		s := roster[i]
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
	innerWidth := totalWidth - 2
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Assignees ", lines, totalWidth), totalWidth, m.width)
}

// renderAssigneeDetail dibuja el detalle de la persona seleccionada.
func (m *Model) renderAssigneeDetail(content string) string {
	name := m.currentAssignee()
	if name == "" {
		// El roster quedó vacío (p. ej. se borró todo): volver a la lista.
		m.assigneeDetail = false
		return m.renderAssigneeModal(content)
	}
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
	for i := start; i < end; i++ {
		o := offs[i]
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
	innerWidth := totalWidth - 2
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" Assignee · "+name+" ", lines, totalWidth), totalWidth, m.width)
}

// renderOffdayForm dibuja el formulario de alta sobre el modal de assignees.
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
	innerWidth := totalWidth - 2
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	return overlayModal(content, renderModalBox(" New off-day ", lines, totalWidth), totalWidth, m.width)
}
