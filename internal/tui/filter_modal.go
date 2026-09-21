package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// filterField indices
const (
	filterFieldProject  = 0
	filterFieldStatus   = 1
	filterFieldAssignee = 2
	filterFieldPriority = 3
	filterFieldTag      = 4
)

// filterFieldCount es la cantidad de campos del modal de filtros.
const filterFieldCount = 5

// filterMaxVisibleOptions acota las opciones listadas del campo activo para que
// el modal no crezca sin control con muchas tags o assignees.
const filterMaxVisibleOptions = 6

// statusFilterAllActive es el valor por defecto del filtro de estado: muestra
// todas las tareas salvo las terminales (done/cancelled). El valor "" (mostrado
// como "all") es el que no restringe el estado.
const statusFilterAllActive = "all active"

// filterFieldOptions retorna las opciones disponibles para un campo.
func (m *Model) filterFieldOptions(field int) []string {
	switch field {
	case filterFieldProject:
		opts := []string{"all"}
		for _, p := range m.projects {
			opts = append(opts, p.Name)
		}
		return opts
	case filterFieldStatus:
		opts := []string{statusFilterAllActive, "all"}
		// El estado es un concepto por-proyecto: si hay uno seleccionado, sólo
		// se ofrecen sus estados; en "all projects", sólo los comunes a todos.
		if m.filterProject != "" {
			if p := m.projectByName(m.filterProject); p != nil {
				return append(opts, p.Workflow...)
			}
			return opts
		}
		return append(opts, m.commonWorkflow()...)
	case filterFieldAssignee:
		opts := []string{"all"}
		for _, a := range m.uniqueAssignees() {
			if a != "" {
				opts = append(opts, a)
			}
		}
		return opts
	case filterFieldPriority:
		return []string{"all", "none", "low", "med", "high"}
	case filterFieldTag:
		return append([]string{"all"}, m.uniqueTags()...)
	}
	return nil
}

// filterFieldLabel devuelve el nombre del campo.
func filterFieldLabel(field int) string {
	switch field {
	case filterFieldProject:
		return "Project"
	case filterFieldStatus:
		return "Status"
	case filterFieldAssignee:
		return "Assignee"
	case filterFieldPriority:
		return "Priority"
	case filterFieldTag:
		return "Tag"
	}
	return "?"
}

// filterCurrentValue retorna el valor actual del filtro para un campo.
func (m *Model) filterCurrentValue(field int) string {
	switch field {
	case filterFieldProject:
		if m.filterProject == "" {
			return "all"
		}
		return m.filterProject
	case filterFieldStatus:
		if m.filterStatus == "" {
			return "all"
		}
		return m.filterStatus
	case filterFieldAssignee:
		if m.filterAssignee == "" {
			return "all"
		}
		return m.filterAssignee
	case filterFieldPriority:
		switch m.filterPriority {
		case -1:
			return "all"
		case 0:
			return "none"
		case 1:
			return "low"
		case 2:
			return "med"
		case 3:
			return "high"
		}
	case filterFieldTag:
		if m.filterTag == "" {
			return "all"
		}
		return m.filterTag
	}
	return "all"
}

// filterApplySelection aplica el valor seleccionado al filtro.
func (m *Model) filterApplySelection(field int, value string) {
	switch field {
	case filterFieldProject:
		if value == "all" {
			m.filterProject = ""
		} else {
			m.filterProject = value
		}
		// El estado es por-proyecto: si el filtro activo ya no existe en el
		// nuevo contexto (otro proyecto, o la intersección en "all"), se limpia.
		m.clearInvalidStatusFilter()
	case filterFieldStatus:
		if value == "all" {
			m.filterStatus = ""
		} else {
			m.filterStatus = value
		}
	case filterFieldAssignee:
		if value == "all" {
			m.filterAssignee = ""
		} else {
			m.filterAssignee = value
		}
	case filterFieldPriority:
		switch value {
		case "all":
			m.filterPriority = -1
		case "none":
			m.filterPriority = 0
		case "low":
			m.filterPriority = 1
		case "med":
			m.filterPriority = 2
		case "high":
			m.filterPriority = 3
		}
	case filterFieldTag:
		if value == "all" {
			m.filterTag = ""
		} else {
			m.filterTag = value
		}
	}
	m.invalidateFilterCache()
}

// cycleProjectFilter avanza (dir=+1) o retrocede (dir=-1) el filtro Project a
// través de sus opciones ("all" + proyectos activos). Reutiliza las mismas
// opciones y validaciones del modal de filtros, de modo que Tab se comporta
// igual en List, Kanban y Gantt. Con 0 o 1 opción no hace nada.
func (m *Model) cycleProjectFilter(dir int) {
	opts := m.filterFieldOptions(filterFieldProject)
	if len(opts) <= 1 {
		return
	}
	current := m.filterCurrentValue(filterFieldProject)
	idx := 0
	for i, o := range opts {
		if o == current {
			idx = i
			break
		}
	}
	m.filterApplySelection(filterFieldProject, opts[(idx+dir+len(opts))%len(opts)])
}

// clearInvalidStatusFilter limpia el filtro de estado cuando dejó de existir en
// el contexto del proyecto actual. "" (all) siempre es válido.
func (m *Model) clearInvalidStatusFilter() {
	if m.filterStatus == "" {
		return
	}
	for _, opt := range m.filterFieldOptions(filterFieldStatus) {
		if opt == m.filterStatus {
			return
		}
	}
	m.filterStatus = ""
}

// filterKeybinds lista las teclas del modal de filtros. Es la fuente única para
// la barra de keybinds y el modal de ayuda.
func filterKeybinds() []keybind {
	return []keybind{
		{"Tab", "field"},
		{"↑↓", "option"},
		{"←→", "cycle"},
		{"Enter", "apply / next"},
		{"Ctrl+R", "reset"},
		{"Esc", "close"},
	}
}

// openFilterModal abre el modal con el foco en el primer campo y el cursor de
// opciones sincronizado con el valor aplicado.
func (m *Model) openFilterModal() tea.Cmd {
	m.filterOpen = true
	m.filterFieldIdx = filterFieldProject
	m.filterSyncOption()
	return nil
}

// filterVisibleOptions devuelve las opciones del campo activo, filtradas por la
// búsqueda fuzzy. Los modos agregados ("all active"/"all") quedan siempre
// disponibles para poder volver atrás sin borrar la búsqueda.
func (m *Model) filterVisibleOptions() []string {
	opts := m.filterFieldOptions(m.filterFieldIdx)
	if m.filterSearch == "" {
		return opts
	}
	var matches, aggregators []string
	for _, o := range opts {
		if o == statusFilterAllActive || o == "all" {
			aggregators = append(aggregators, o)
			continue
		}
		if _, ok := fuzzyScore(m.filterSearch, o); ok {
			matches = append(matches, o)
		}
	}
	// Los matches van primero para que el cursor quede sobre el mejor candidato;
	// los modos agregados quedan al final, siempre accesibles.
	return append(matches, aggregators...)
}

// filterSyncOption limpia la búsqueda y posiciona el cursor sobre el valor
// aplicado del campo activo.
func (m *Model) filterSyncOption() {
	m.filterSearch = ""
	opts := m.filterFieldOptions(m.filterFieldIdx)
	current := m.filterCurrentValue(m.filterFieldIdx)
	m.filterOptionIdx = 0
	for i, o := range opts {
		if o == current {
			m.filterOptionIdx = i
			break
		}
	}
}

// clampFilterOption mantiene el cursor de opciones dentro de la lista visible.
func (m *Model) clampFilterOption() {
	opts := m.filterVisibleOptions()
	if len(opts) == 0 {
		m.filterOptionIdx = 0
		return
	}
	if m.filterOptionIdx >= len(opts) {
		m.filterOptionIdx = len(opts) - 1
	}
	if m.filterOptionIdx < 0 {
		m.filterOptionIdx = 0
	}
}

// filterMoveField mueve el foco entre campos y resincroniza el cursor.
func (m Model) filterMoveField(delta int) (tea.Model, tea.Cmd) {
	m.filterFieldIdx = (m.filterFieldIdx + delta + filterFieldCount) % filterFieldCount
	m.filterSyncOption()
	return m, nil
}

// filterMoveOption mueve el cursor dentro de las opciones visibles.
func (m *Model) filterMoveOption(delta int) {
	opts := m.filterVisibleOptions()
	if len(opts) == 0 {
		return
	}
	m.filterOptionIdx = (m.filterOptionIdx + delta + len(opts)) % len(opts)
}

// filterCycle aplica en vivo la opción anterior/siguiente del campo activo.
func (m *Model) filterCycle(forward bool) {
	opts := m.filterVisibleOptions()
	if len(opts) == 0 {
		return
	}
	idx := m.filterOptionIdx
	if idx < 0 || idx >= len(opts) {
		idx = 0
	}
	if forward {
		idx = (idx + 1) % len(opts)
	} else {
		idx = (idx - 1 + len(opts)) % len(opts)
	}
	m.filterApplySelection(m.filterFieldIdx, opts[idx])
	m.filterSyncOption()
}

// resetFilters vuelve todos los filtros a su valor por defecto, incluido el
// estado "all active".
func (m *Model) resetFilters() {
	m.filterProject = ""
	m.filterStatus = statusFilterAllActive
	m.filterAssignee = ""
	m.filterTag = ""
	m.filterPriority = -1
	m.invalidateFilterCache()
	m.clampKanbanCursor()
	m.filterSyncOption()
}

// handleFilterModalKey procesa las teclas del modal de filtros. Tab/Shift+Tab
// mueven el foco, ↑↓ mueven el cursor de opciones, ←→ ciclan el valor aplicado
// en vivo, escribir filtra las opciones, Enter aplica y avanza, Ctrl+R resetea
// todo y Esc cierra.
func (m Model) handleFilterModalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.filterOpen = false
		return m, nil

	case "tab":
		return m.filterMoveField(1)
	case "shift+tab":
		return m.filterMoveField(-1)

	case "up":
		m.filterMoveOption(-1)
		return m, nil
	case "down":
		m.filterMoveOption(1)
		return m, nil

	case "left":
		m.filterCycle(false)
		return m, nil
	case "right":
		m.filterCycle(true)
		return m, nil

	case "ctrl+r":
		m.resetFilters()
		return m, nil

	case "enter":
		if opts := m.filterVisibleOptions(); len(opts) > 0 {
			m.filterApplySelection(m.filterFieldIdx, opts[m.filterOptionIdx])
		}
		// En el último campo, Enter cierra; en el resto, avanza.
		if m.filterFieldIdx == filterFieldTag {
			m.filterOpen = false
			return m, nil
		}
		return m.filterMoveField(1)

	case "backspace":
		if m.filterSearch != "" {
			m.filterSearch = m.filterSearch[:len(m.filterSearch)-1]
			m.filterOptionIdx = 0
		}
		return m, nil
	}

	// Texto imprimible: filtra las opciones del campo activo.
	if len(key) == 1 && key[0] >= 33 && key[0] < 127 {
		m.filterSearch += key
		m.filterOptionIdx = 0
	}
	return m, nil
}

// renderFilterModal renderiza el modal de filtros: filas con foco resaltado y,
// en el campo activo, un input de búsqueda y el listado de opciones con el valor
// aplicado (●) y el cursor (▸). El título muestra el conteo en vivo.
func (m *Model) renderFilterModal(content string) string {
	w := m.width
	m.clampFilterOption()

	fields := []int{filterFieldProject, filterFieldStatus, filterFieldAssignee, filterFieldPriority, filterFieldTag}

	lines := []string{""}
	for _, field := range fields {
		focused := field == m.filterFieldIdx
		label := cellWidth(filterFieldLabel(field), 10)
		current := m.filterCurrentValue(field)

		if focused {
			input := m.filterSearch
			if input == "" {
				input = styleDim.Render(current)
			} else {
				input = styleTitle.Render(input)
			}
			lines = append(lines, "  ▸ "+styleTitle.Render(label)+" "+input+styleTitle.Render(cursorGlyph))
		} else {
			lines = append(lines, "    "+styleStatusDesc.Render(label)+" "+styleStatusKey.Render(current))
		}

		if !focused {
			continue
		}

		opts := m.filterVisibleOptions()
		if len(opts) == 0 {
			lines = append(lines, styleDim.Render("        (no matches)"))
			continue
		}
		start, end := visibleRange(m.filterOptionIdx, len(opts), filterMaxVisibleOptions)
		for i := start; i < end; i++ {
			applied := opts[i] == current
			mark := "  "
			if applied {
				mark = "● "
			}
			switch {
			case i == m.filterOptionIdx:
				lines = append(lines, styleSelected.Render("    ▸ "+mark+opts[i]))
			case applied:
				lines = append(lines, styleStatusKey.Render("      "+mark+opts[i]))
			default:
				lines = append(lines, "      "+mark+opts[i])
			}
		}
		if len(opts) > filterMaxVisibleOptions {
			lines = append(lines, styleDim.Render(fmt.Sprintf("      %d/%d", m.filterOptionIdx+1, len(opts))))
		}
	}

	totalWidth := modalWidthFor(54, w)
	innerWidth := totalWidth - 2
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	title := fmt.Sprintf(" Filters · %d tasks ", len(m.filteredTasks()))
	return overlayModal(content, renderModalBox(title, lines, totalWidth), totalWidth, w)
}
