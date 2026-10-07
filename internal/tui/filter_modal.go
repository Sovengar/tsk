package tui

import (
	"fmt"
	"slices"

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

// filterFieldCount is the number of fields of the filter modal.
const filterFieldCount = 5

// filterMaxVisibleOptions bounds the listed options of the active field so
// the modal does not grow out of control with many tags or assignees.
const filterMaxVisibleOptions = 6

// statusFilterAllActive is the default value of the status filter: it shows
// all tasks except the terminal ones (done/cancelled). The "" value (shown
// as "all") is the one that does not restrict the status.
const statusFilterAllActive = "all active"

// filterFieldOptions returns the available options for a field.
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
		// The status is a per-project concept: if one is selected, only its
		// statuses are offered; in "all projects", only those common to all.
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

// filterFieldLabel returns the field's name.
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

// filterCurrentValue returns the current value of the filter for a field.
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
		// Only the four values that are a real filter. -1 is "no filter" and
		// anything else too: both fall into the final "all", so putting them
		// as one more case only added a branch indistinguishable from that one.
		switch m.filterPriority {
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

// filterApplySelection applies the selected value to the filter.
func (m *Model) filterApplySelection(field int, value string) {
	switch field {
	case filterFieldProject:
		if value == "all" {
			m.filterProject = ""
		} else {
			m.filterProject = value
		}
		// The status is per-project: if the active filter no longer exists in
		// the new context (another project, or the intersection in "all"), it is cleared.
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

// cycleProjectFilter advances (dir=+1) or goes back (dir=-1) the Project
// filter through its options ("all" + active projects). It reuses the same
// options and validations of the filter modal, so that Tab behaves the same
// in List, Kanban and Gantt. With 0 or 1 option it does nothing.
func (m *Model) cycleProjectFilter(dir int) {
	// The whole jump lives in nextOption, a pure function. Nothing is
	// left here: neither the "there is no list" nor the "the current value is
	// not in the list" -- she solves both cases and both can be tested by
	// passing her the list directly.
	//
	// The `if len(opts) <= 1` that was before was a bug, not a guard: the
	// project filter's options ALWAYS start with "all", so with a single
	// project there are TWO options and Tab did not move anything. It counted
	// wrong because it treated "all" as if it did not count.
	m.filterApplySelection(filterFieldProject,
		nextOption(m.filterFieldOptions(filterFieldProject),
			m.filterCurrentValue(filterFieldProject), dir))
}

// nextOption returns the option that goes one position before or after
// current in opts, wrapping around at the extremes.
//
// With empty opts it returns "", a value no option can have: the project
// filter's options always start with "all", so the list is never really
// empty, but the function is total and can be tested with nil.
func nextOption(opts []string, current string, dir int) string {
	if len(opts) == 0 {
		return ""
	}
	// The index starts at 0 and slices.Index returns -1 when it does not find
	// it, so the "is not in the list" is solved with a max and not with an if:
	// `max(idx, 0)` and the `if idx < 0 { idx = 0 }` give the same, and with
	// the if the mutant of its condition had no value telling it apart from
	// the index 0 that max already produces.
	//
	// The current value may not be in the list: a deleted project, or a
	// hand-written name. It starts at the beginning instead of staying where
	// it is.
	idx := max(slices.Index(opts, current), 0)
	return opts[cycleIndex(idx, len(opts), dir)]
}

// clearInvalidStatusFilter clears the status filter when it stopped existing
// in the current project's context. "" (all) is always valid.
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

// filterKeybinds lists the filter modal's keys. It is the single source for
// the keybinds bar and the help modal.
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

// openFilterModal opens the modal with focus on the first field and the
// option cursor synced with the applied value.
func (m *Model) openFilterModal() tea.Cmd {
	m.filterOpen = true
	m.filterFieldIdx = filterFieldProject
	m.filterSyncOption()
	return nil
}

// filterVisibleOptions returns the options of the active field, filtered by
// the fuzzy search. The aggregate modes ("all active"/"all") always stay
// available so you can go back without erasing the search.
// filterVisibleOptions returns the options of the active field, with the
// search written on top.
//
// The result is never empty: "all" is an aggregator and always slips in,
// search or not. The callers asking "and if there are no options" were
// protecting themselves against something that cannot happen.
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
	// The matches go first so the cursor lands on the best candidate; the
	// aggregate modes stay at the end, always accessible.
	return append(matches, aggregators...)
}

// filterSyncOption clears the search and positions the cursor on the
// applied value of the active field.
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

// clampFilterOption keeps the option cursor inside the visible list.
func (m *Model) clampFilterOption() {
	m.filterOptionIdx = clampTo(m.filterOptionIdx, len(m.filterVisibleOptions()))
}

// filterMoveField moves the focus between fields and resyncs the cursor.
func (m Model) filterMoveField(delta int) (tea.Model, tea.Cmd) {
	m.filterFieldIdx = (m.filterFieldIdx + delta + filterFieldCount) % filterFieldCount
	m.filterSyncOption()
	return m, nil
}

// filterMoveOption moves the cursor within the visible options.
func (m *Model) filterMoveOption(delta int) {
	m.filterOptionIdx = cycleIndex(m.filterOptionIdx, len(m.filterVisibleOptions()), delta)
}

// filterCycle applies the previous/next option of the active field live.
func (m *Model) filterCycle(forward bool) {
	opts := m.filterVisibleOptions()
	if len(opts) == 0 {
		return
	}
	delta := 1
	if !forward {
		delta = -1
	}
	idx := cycleIndex(firstValidIndex(m.filterOptionIdx, len(opts)), len(opts), delta)
	m.filterApplySelection(m.filterFieldIdx, opts[idx])
	m.filterSyncOption()
}

// resetFilters returns all filters to their default value, including the
// "all active" status.
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

// handleFilterModalKey processes the filter modal's keys. Tab/Shift+Tab
// move the focus, ↑↓ move the option cursor, ←→ cycle the applied value
// live, typing filters the options, Enter applies and advances, Ctrl+R
// resets everything and Esc closes.
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
		// opts is never empty, so the check that was here could
		// never be false. What does need bounding is the index: the render
		// already does it, but between the render and this key the list may
		// have changed size.
		opts := m.filterVisibleOptions()
		m.filterOptionIdx = clampTo(m.filterOptionIdx, len(opts))
		m.filterApplySelection(m.filterFieldIdx, opts[m.filterOptionIdx])
		// In the last field, Enter closes; in the rest, it advances.
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

	// Printable text: filters the options of the active field.
	if len(key) == 1 && key[0] >= 33 && key[0] < 127 {
		m.filterSearch += key
		m.filterOptionIdx = 0
	}
	return m, nil
}

// renderFilterModal renders the filter modal: rows with highlighted focus
// and, in the active field, a search input and the list of options with the
// applied value (●) and the cursor (▸). The title shows the live count.
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

		// Never without options: "all" survives any search.
		opts := m.filterVisibleOptions()
		start, end := visibleRange(m.filterOptionIdx, len(opts), filterMaxVisibleOptions)
		for i, opt := range opts[start:end] {
			i += start
			applied := opt == current
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
	innerWidth := modalInnerWidth(totalWidth)
	for i := range lines {
		lines[i] = truncateLines(lines[i], innerWidth)
	}
	title := fmt.Sprintf(" Filters · %d tasks ", len(m.filteredTasks()))
	return overlayModal(content, renderModalBox(title, lines, totalWidth), totalWidth, w)
}
