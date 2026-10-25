package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// searchMaxQuery bounds the query so the bar cannot overflow the box with a
// pasted line.
const searchMaxQuery = 80

// openSearch starts the search prompt. The committed query is kept as the
// starting text: "/" on an active search lets it be edited instead of retyped.
func (m *Model) openSearch() tea.Cmd {
	m.searchOpen = true
	m.searchInput = m.searchQuery
	m.searchBefore = m.searchQuery
	return nil
}

// taskMatchesSearch reports whether a task matches the query in its title or
// in its description, case-insensitively.
func taskMatchesSearch(t model.Task, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(t.Title), needle) ||
		strings.Contains(strings.ToLower(t.Description), needle)
}

// handleSearchKey edits the search prompt. Every edit commits the query at
// once, so the listing narrows while typing; "enter" only closes the prompt
// and "esc" restores the query that was committed before opening it.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.searchOpen = false
		m.searchInput = ""
		m.searchQuery = m.searchBefore
		m.invalidateFilterCache()
		return m, nil
	case "enter":
		m.searchOpen = false
		m.searchInput = ""
		return m, nil
	case "backspace":
		if m.searchInput != "" {
			runes := []rune(m.searchInput)
			m.searchInput = string(runes[:len(runes)-1])
		}
		m.searchQuery = m.searchInput
		m.invalidateFilterCache()
		return m, nil
	}

	// The space bar can arrive as a named key ("space") or as a lone
	// printable rune: both are typed into the query.
	if key == "space" {
		m.searchInput = appendSearchRune(m.searchInput, ' ')
		m.searchQuery = m.searchInput
		m.invalidateFilterCache()
		return m, nil
	}
	if r := []rune(key); len(r) == 1 && r[0] >= 32 {
		m.searchInput = appendSearchRune(m.searchInput, r[0])
		m.searchQuery = m.searchInput
		m.invalidateFilterCache()
	}
	return m, nil
}

// appendSearchRune adds a rune to the query while it fits in the limit.
func appendSearchRune(query string, r rune) string {
	if len([]rune(query)) >= searchMaxQuery {
		return query
	}
	return query + string(r)
}

// renderSearchPart renders the search line of the filter bar. While the
// prompt is open the cursor is shown; once committed only the query remains
// (or nothing at all when there is no search).
func (m *Model) renderSearchPart() string {
	if !m.searchOpen && m.searchQuery == "" {
		return ""
	}
	value := m.searchQuery
	if m.searchOpen {
		value = m.searchInput + "▏"
	}
	return styleFilterDim.Render("Search: ") + styleStatusKey.Render(value)
}
