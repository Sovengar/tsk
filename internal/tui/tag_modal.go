package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// tagMaxSuggestions es la cantidad de sugerencias visibles en el modal.
const tagMaxSuggestions = 6

// tagSuggestions devuelve las tags conocidas que sirven como sugerencia,
// filtradas por el prefijo escrito. Con el input vacío devuelve todas.
func (m Model) tagSuggestions() []string {
	all := m.uniqueTags()
	prefix := strings.ToLower(strings.TrimSpace(m.tagInput))
	if prefix == "" {
		return all
	}
	var out []string
	for _, tag := range all {
		if strings.HasPrefix(tag, prefix) {
			out = append(out, tag)
		}
	}
	return out
}

// handleTagModalKey procesa las teclas del modal de tags: escribir filtra las
// sugerencias, ↑↓ navegan, Tab completa y Enter alterna (agrega/quita) la tag.
func (m Model) handleTagModalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.tagOpen = false
		m.tagInput = ""
		m.tagSuggestIdx = -1
		return m, nil

	case "enter":
		tag := strings.TrimSpace(m.tagInput)
		if tag == "" {
			if suggs := m.tagSuggestions(); m.tagSuggestIdx >= 0 && m.tagSuggestIdx < len(suggs) {
				tag = suggs[m.tagSuggestIdx]
			}
		}
		if tag == "" || m.detailTask == nil {
			return m, nil
		}
		taskID := m.detailTask.ID
		m.tagInput = ""
		m.tagSuggestIdx = -1
		return m, m.toggleTagCmd(taskID, tag)

	case "up":
		suggs := m.tagSuggestions()
		if len(suggs) == 0 {
			return m, nil
		}
		if m.tagSuggestIdx <= 0 {
			m.tagSuggestIdx = len(suggs) - 1
		} else {
			m.tagSuggestIdx--
		}
		return m, nil

	case "down":
		suggs := m.tagSuggestions()
		if len(suggs) == 0 {
			return m, nil
		}
		m.tagSuggestIdx = (m.tagSuggestIdx + 1) % len(suggs)
		return m, nil

	case "tab":
		// Completa con la sugerencia seleccionada o, si no hay, la primera.
		suggs := m.tagSuggestions()
		if len(suggs) == 0 {
			return m, nil
		}
		idx := m.tagSuggestIdx
		if idx < 0 || idx >= len(suggs) {
			idx = 0
		}
		m.tagInput = suggs[idx]
		m.tagSuggestIdx = -1
		return m, nil

	case "backspace":
		if m.tagInput != "" {
			r := []rune(m.tagInput)
			m.tagInput = string(r[:len(r)-1])
			m.tagSuggestIdx = -1
		}
		return m, nil
	}

	// Texto imprimible: alimenta el input y reinicia la selección.
	if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
		m.tagInput += key
		m.tagSuggestIdx = -1
	}
	return m, nil
}

// handleTagPaste agrega el texto pegado al input, sin saltos de línea.
func (m Model) handleTagPaste(content string) (tea.Model, tea.Cmd) {
	content = strings.NewReplacer("\n", " ", "\r", " ").Replace(content)
	m.tagInput += content
	m.tagSuggestIdx = -1
	return m, nil
}

// renderTagModal dibuja el modal de tags superpuesto al detalle.
func (m *Model) renderTagModal(content string) string {
	w := m.width
	width := modalWidthFor(52, w)

	lines := []string{}

	if m.detailTask != nil {
		lines = append(lines, styleDim.Render("  "+m.detailTask.Title))
	}

	current := "(none)"
	if m.detailTask != nil && len(m.detailTask.Tags) > 0 {
		current = strings.Join(m.detailTask.Tags, ", ")
	}
	lines = append(lines, "  Current: "+current)
	lines = append(lines, "")
	lines = append(lines, "  > "+m.tagInput+styleTitle.Render("▏"))

	suggs := m.tagSuggestions()
	if len(suggs) > tagMaxSuggestions {
		suggs = suggs[:tagMaxSuggestions]
	}
	if len(suggs) == 0 {
		lines = append(lines, styleDim.Render("  (nueva tag)"))
	}
	for i, tag := range suggs {
		applied := m.detailTask != nil && model.HasTag(m.detailTask.Tags, tag)
		row := "    " + tag
		if applied {
			row = "  ✓ " + tag
		}
		switch {
		case i == m.tagSuggestIdx:
			row = styleSelected.Render(row)
		case applied:
			row = styleStatusDesc.Render(row)
		}
		lines = append(lines, row)
	}

	return overlayModal(content, renderModalBox(" Tags ", lines, width), width, w)
}
