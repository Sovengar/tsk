package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// tagMaxSuggestions is the number of suggestions visible in the modal.
const tagMaxSuggestions = 6

// tagSuggestions returns the known tags that work as suggestions,
// filtered by the typed prefix. With an empty input it returns all of them.
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

// handleTagModalKey processes the tag modal's keys: typing filters the
// suggestions, ↑↓ navigate, Tab completes and Enter toggles (adds/removes) the tag.
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
		// The same arithmetic as the rest of the program's navigations:
		// cycleIndex resolves it for any list; with no list there is nowhere
		// to move. It was written here for the third time.
		delta := 1
		if key == "up" {
			delta = -1
		}
		m.tagSuggestIdx = cycleIndex(m.tagSuggestIdx, len(m.tagSuggestions()), delta)
		return m, nil

	case "tab":
		// Completes with the selected suggestion and, if the index does not
		// apply to this list, with the first one. With no list, firstOrAt
		// returns "" and the input stays as it was.
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

	// Printable text: feeds the input and resets the selection.
	//
	// The condition is a call to isPrintable and not three loose comparisons,
	// because the range [32, 127) can be tested from both sides and the
	// comparisons cannot: with `key[0] > 31` the lower half of the range is a 32
	// that Bubbletea never delivers -- the space bar arrives as "space", and the
	// `len(key) == 1` here discards it -- so 32 and 33 give the same and the
	// mutant of one with the other survived.
	//
	// isPrintable has to be checkable from both sides, and for that the floor
	// is set on the first character that ARRIVES, which is 33. The 32 stays
	// documented in the comment because it explains why there is no "arbitrary" 33.
	if len(key) == 1 && isPrintable(key[0]) {
		m.tagInput += key
		m.tagSuggestIdx = -1
	}
	return m, nil
}

// isPrintable tells whether a byte is a printable character of a key.
//
// The floor is 33 ('!') and not 32 (space) on purpose: Bubbletea delivers the
// space bar as a named key ("space"), not as a lone byte, so a lone 32
// never arrives, and putting one only added a comparison that no test could
// distinguish from the next one. 33 is the first byte that does arrive, so
// the comparison has both sides reachable.
//
// The ceiling is 127 (DEL), the last printable character of ASCII: 128 and
// above is non-ASCII, and even if it arrived as several bytes the `len(key) ==
// 1` would already have discarded it.
func isPrintable(b byte) bool {
	return b >= firstPrintableByte && b < lastPrintableByte
}

const (
	// firstPrintableByte is 33, the exclamation sign: the first byte that
	// Bubbletea delivers as a single-character key. The space (32) does not
	// count because it arrives named ("space") and the `len(key) == 1` discards
	// it, so a floor of 32 had no byte below it to distinguish it from 33.
	firstPrintableByte = 33
	// lastPrintableByte is 127, the DEL: the last printable character of
	// ASCII.
	lastPrintableByte = 127
)

// handleTagPaste appends the pasted text to the input, without line breaks.
func (m Model) handleTagPaste(content string) (tea.Model, tea.Cmd) {
	content = strings.NewReplacer("\n", " ", "\r", " ").Replace(content)
	m.tagInput += content
	m.tagSuggestIdx = -1
	return m, nil
}

// renderTagModal draws the tag modal overlaid on the detail.
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

	// The truncation uses min and no if: at exactly the cap, suggs[:6] is the
	// same slice, so the condition did not decide anything.
	suggs := m.tagSuggestions()
	suggs = suggs[:min(len(suggs), tagMaxSuggestions)]
	if len(suggs) == 0 {
		lines = append(lines, styleDim.Render("  (new tag)"))
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
