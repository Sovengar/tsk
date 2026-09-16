package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// filterField indices
const (
	filterFieldProject  = 0
	filterFieldStatus   = 1
	filterFieldAssignee = 2
	filterFieldPriority = 3
)

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
		for _, s := range m.mergedWorkflow() {
			opts = append(opts, s)
		}
		return opts
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
	}
	m.invalidateFilterCache()
}

// handleFilterModalKey procesa teclas del modal de filtros.
func (m Model) handleFilterModalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.filterOpen = false
		return m, nil

	case "tab", "down":
		m.filterFieldIdx = (m.filterFieldIdx + 1) % 4
		return m, nil

	case "shift+tab", "up":
		m.filterFieldIdx = (m.filterFieldIdx + 3) % 4
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
	fields := []int{filterFieldProject, filterFieldStatus, filterFieldAssignee, filterFieldPriority}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, styleTitle.Render("  Filters"))
	lines = append(lines, styleSep.Render(strings.Repeat("─", 42)))

	for _, field := range fields {
		label := filterFieldLabel(field)
		current := m.filterCurrentValue(field)
		opts := m.filterFieldOptions(field)

		// Build option display: ← value →
		optDisplay := fmt.Sprintf("%-12s ← %s →", label, current)
		if field == m.filterFieldIdx {
			lines = append(lines, styleSelected.Render("  > "+optDisplay))
		} else {
			lines = append(lines, "    "+optDisplay)
		}

		// Show options hint on selected field
		if field == m.filterFieldIdx && len(opts) > 2 {
			hint := strings.Join(opts, "  ")
			if len(hint) > 40 {
				hint = hint[:37] + "..."
			}
			lines = append(lines, styleDim.Render("      "+hint))
		}
	}

	lines = append(lines, styleSep.Render(strings.Repeat("─", 42)))
	lines = append(lines, styleHelp.Render("  Tab/↑↓ field    ← → change    Enter close    Esc cancel"))

	modalContent := strings.Join(lines, "\n")

	// Modal width
	modalWidth := 46
	if modalWidth > w-2 {
		modalWidth = w - 2
	}

	modal := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder(), true).
		Render(modalContent)

	// Overlay on content
	bgLines := strings.Split(content, "\n")
	totalLines := len(bgLines)
	modalLines := strings.Split(modal, "\n")
	modalH := len(modalLines)

	startY := (totalLines - modalH) / 2
	if startY < 0 {
		startY = 0
	}

	totalModalW := modalWidth + 2
	startX := (w - totalModalW) / 2
	if startX < 0 {
		startX = 0
	}

	for i, ml := range modalLines {
		y := startY + i
		if y >= totalLines {
			break
		}
		lineRunes := []rune(bgLines[y])
		modalRunes := []rune(ml)

		room := len(lineRunes) - startX
		if room <= 0 {
			pad := startX + len(modalRunes) - len(lineRunes)
			if pad > 0 {
				lineRunes = append(lineRunes, make([]rune, pad)...)
			}
			room = len(modalRunes)
		}
		if len(modalRunes) > room {
			modalRunes = modalRunes[:room]
		}

		copy(lineRunes[startX:startX+len(modalRunes)], modalRunes)
		bgLines[y] = string(lineRunes)
	}

	return strings.Join(bgLines, "\n")
}
