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
		opts := []string{"all"}
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

// handleFilterModalKey procesa teclas del modal de filtros.
func (m Model) handleFilterModalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.filterOpen = false
		return m, nil

	case "tab", "down":
		m.filterFieldIdx = (m.filterFieldIdx + 1) % filterFieldCount
		return m, nil

	case "shift+tab", "up":
		m.filterFieldIdx = (m.filterFieldIdx + filterFieldCount - 1) % filterFieldCount
		return m, nil

	case "left", "right":
		opts := m.filterFieldOptions(m.filterFieldIdx)
		current := m.filterCurrentValue(m.filterFieldIdx)
		idx := 0
		for i, o := range opts {
			if o == current {
				idx = i
				break
			}
		}
		if key == "right" {
			idx = (idx + 1) % len(opts)
		} else {
			idx = (idx - 1 + len(opts)) % len(opts)
		}
		m.filterApplySelection(m.filterFieldIdx, opts[idx])
		return m, nil

	case "enter":
		m.filterOpen = false
		return m, nil
	}

	return m, nil
}

// renderFilterModal renderiza el modal flotante de filtros.
func (m *Model) renderFilterModal(content string) string {
	w := m.width
	fields := []int{filterFieldProject, filterFieldStatus, filterFieldAssignee, filterFieldPriority, filterFieldTag}

	var lines []string
	lines = append(lines, "")

	for _, field := range fields {
		label := filterFieldLabel(field)
		current := m.filterCurrentValue(field)
		opts := m.filterFieldOptions(field)

		// Build option display: Label ← Value →
		optPrefix := fmt.Sprintf("%-12s ← ", label)
		optSuffix := " →"
		valueStr := styleStatusKey.Render(current)
		optDisplay := styleFilterDim.Render(optPrefix) + valueStr + styleFilterDim.Render(optSuffix)
		if field == m.filterFieldIdx {
			lines = append(lines, styleSelected.Render("  > ")+" "+optDisplay)
		} else {
			lines = append(lines, "      "+optDisplay)
		}

		// Show options as vertical list on selected field
		if field == m.filterFieldIdx && len(opts) > 2 {
			maxShow := 5
			if len(opts) < maxShow {
				maxShow = len(opts)
			}
			for _, opt := range opts[:maxShow] {
				if opt == current {
					lines = append(lines, styleSelected.Render("        > "+opt))
				} else {
					lines = append(lines, "          "+opt)
				}
			}
			if len(opts) > maxShow {
				lines = append(lines, styleDim.Render(fmt.Sprintf("          ... %d more", len(opts)-maxShow)))
			}
		}
	}

	totalWidth := modalWidthFor(48, w)
	return overlayModal(content, renderModalBox(" Filters ", lines, totalWidth), totalWidth, w)
}
