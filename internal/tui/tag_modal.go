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
			tag = taskAt2(m.tagSuggestions(), m.tagSuggestIdx)
		}
		if tag == "" || m.detailTask == nil {
			return m, nil
		}
		taskID := m.detailTask.ID
		m.tagInput = ""
		m.tagSuggestIdx = -1
		return m, m.toggleTagCmd(taskID, tag)

	case "up", "down":
		// La misma aritmética que el resto de navegaciones del programa:
		// cicloIndex la resuelve para cualquier lista, sin lista no hay a dónde
		// moverse. Estaba escrita aquí por tercera vez.
		delta := 1
		if key == "up" {
			delta = -1
		}
		m.tagSuggestIdx = cycleIndex(m.tagSuggestIdx, len(m.tagSuggestions()), delta)
		return m, nil

	case "tab":
		// Completa con la sugerencia seleccionada y, si el índice no vale para
		// esta lista, con la primera. Sin lista, firstOrAt devuelve "" y el
		// input se queda como estaba.
		m.tagInput = firstOrAt(m.tagSuggestions(), m.tagSuggestIdx)
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
	//
	// El suelo es 32 (el espacio) porque Bubbletea nunca entrega una tecla de un
	// solo carácter por debajo de ahí -- las de control llegan con nombre ("esc",
	// "tab"), no como un byte suelto -- pero dejarlo escrito.documenta la
	// intención en vez de dejar un 33 que parece arbitrario. El precio es que el
	// espacio no se puede escribir: llega como "space" y lo descarta el == 1.
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

	// A exactamente el tope el recorte es una identidad, así que `>` y `>=` dan
	// lo mismo aquí.
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
