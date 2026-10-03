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
	// La condición es una llamada a esImprimible y no tres comparaciones sueltas,
	// porque el rango [32, 127) se puede probar por los dos lados y las
	// comparaciones no: con `key[0] > 31` la mitad baja del rango es un 32 que
	// Bubbletea nunca entrega -- la barra espaciadora llega como "space", y el
	// `len(key) == 1` de aquí la descarta -- así que el 32 y el 33 dan lo mismo y
	// el mutante de uno con el otro sobrevivía.
	//
	// esImprimible tiene que ser comprobable por sus dos lados, y para eso el
	// suelo se pone en el primer carácter que LLEGA, que es el 33. El 32 sigue
	// documentado en el comentario porque explica por qué no hay 33 "arbitrario".
	if len(key) == 1 && esImprimible(key[0]) {
		m.tagInput += key
		m.tagSuggestIdx = -1
	}
	return m, nil
}

// esImprimible dice si un byte es un carácter imprimible de una tecla.
//
// El suelo es el 33 ('!') y no el 32 (espacio) a propósito: Bubbletea entrega la
// barra espaciadora con nombre ("space"), no como un byte suelto, así que un 32
// suelto no llega nunca y ponerlo sólo añadía una comparación que ningún test
// podía distinguir de la siguiente. El 33 es el primer byte que sí llega, así que
// la comparación tiene los dos lados alcanzables.
//
// El techo es el 127 (DEL), que es el último carácter imprimible de ASCII: el
// 128 en adelante es no-ASCII, y aunque llegara como varios bytes el `len(key) ==
// 1` ya lo habría descartado.
func esImprimible(b byte) bool {
	return b >= primerByteImprimible && b < ultimoByteImprimible
}

const (
	// primerByteImprimible es 33, el signo de exclamación: el primer byte que
	// Bubbletea entrega como tecla de un solo carácter. El espacio (32) no cuenta
	// porque llega con nombre ("space") y el `len(key) == 1` lo descarta, así que
	// un suelo de 32 no tenía ningún byte debajo que lo distinguiera del 33.
	primerByteImprimible = 33
	// ultimoByteImprimible es 127, el DEL: el último carácter imprimible de
	// ASCII.
	ultimoByteImprimible = 127
)

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

	// El recorte va con min y sin if: a exactamente el tope, suggs[:6] es el
	// mismo slice, así que la condición no decidía nada.
	suggs := m.tagSuggestions()
	suggs = suggs[:min(len(suggs), tagMaxSuggestions)]
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
