package tui

import (
	"fmt"
	"slices"
	"strings"

	"tsk/internal/config"
	"tsk/internal/model"

	"github.com/charmbracelet/x/ansi"
)

// ganttFixedLabelWidth is what the Gantt's label takes with room to spare.
const ganttFixedLabelWidth = 30

// ganttLabelThreshold are the inner columns from which the label keeps its
// fixed width instead of a third of the total.
const ganttLabelThreshold = 70

// ganttMinLabelWidth is the label's minimum, so that a name fits.
const ganttMinLabelWidth = 14

// ganttMinDayCols is the minimum of day columns: a whole week.
const ganttMinDayCols = 7

// minContentHeight is the minimum height reserved for the view's content,
// so that the box does not degrade on small terminals.
const minContentHeight = 8

// lineCount returns how many rows a text block takes.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// truncateLines cuts each line to the given width. It keeps the border helper
// from re-wrapping a line that does not fit and adding extra rows, which
// would break the height calculation.
func truncateLines(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// min instead of comparing: when the line measures exactly what the
		// width does, both branches give the same text, so the comparison was
		// another equivalent mutant. Truncating to what already fits changes nothing.
		lines[i] = ansi.Truncate(line, min(ansi.StringWidth(line), width), "")
	}
	return strings.Join(lines, "\n")
}

// cellWidth adjusts a text to the indicated display width: it truncates with
// ".." if there is too much and pads with spaces if there is too little. It
// measures screen columns, not bytes, so the colored (ANSI) cells stay
// aligned with the colorless ones. It is what replaces fmt's %-Ns, which counts bytes and misaligns the colored cells.
func cellWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	s = ansi.Truncate(s, width, "..")
	// Pad with a max instead of an if: with zero pad the if did nothing and
	// the max repeats "", which is the same without the branch to mutate.
	//
	// The floor at 0 is for strings.Repeat, which blows up with a negative
	// number. ansi.Truncate never returns a cell wider than the limit, so it
	// cannot be reached: it is the same class of defensive floor as the others
	// in this file, and its two mutants -- removing it or putting it at -1 --
	// are equivalent for the same reason.
	return s + strings.Repeat(" ", max(width-ansi.StringWidth(s), 0))
}

// modalInnerWidth is the useful width of a modal: the total width minus the
// two characters of the left and the right border.
//
// modalWidthFor always leaves at least two columns of margin (otherwise the
// modal would stick to the edge of the screen), so the result is positive.
// The floor at 1 covers the case where totalWidth arrives already trimmed by another path.
//
// It was written five times, and each copy was a place where a mutated
// "- 2" went unnoticed: the text was truncated a couple of columns more or
// less in width and nobody noticed unless looking at the render closely.
func modalInnerWidth(totalWidth int) int {
	return max(totalWidth-2, 1)
}

// contentBudget calculates the rows available for the view's content,
// discounting the preview and the keybinds bar.
func contentBudget(total, previewH, keybindsH int) int {
	budget := total - previewH - keybindsH
	budget = max(budget, minContentHeight)
	return budget
}

// nextCommentSel advances the comment selection without wrapping: from -1
// (nothing selected) it jumps to the first one, and arriving at the last one it stays there.
//
// That is what the detail's "j" key does, written as min/max so that the
// rule is checkable: wrap upwards, and -1 treated as "before the first
// one" instead of as a valid position.
func nextCommentSel(sel, n int) int {
	return min(max(sel+1, 0), max(n-1, 0))
}

// prevCommentSel goes back in the selection. On the first comment it goes
// back to the "nothing selected" state (-1) and stays there: the detail does not wrap.
func prevCommentSel(sel int) int {
	return max(sel-1, -1)
}

// resolvePageSize returns the page size to use, or the default one if the
// config does not bring a usable one.
//
// The floor is at 0 and not at 1 because a negative or zero ListPageSize is
// not a "single page" asked by the user, it is missing config: the config
// never fails, it warns and goes on, and here the same criterion applies.
func resolvePageSize(n int) int {
	if n > 0 {
		return n
	}
	return config.DefaultPageSize
}

// editorCommand returns the external editor to launch, or nvim if the config
// does not bring one.
//
// It was written three times, with the same literal in all three. Each copy
// was its own place where a mutant could change the default editor without
// anything noticing, because all three only ran when the detail was open.
func editorCommand(cmd string) string {
	if cmd == "" {
		return "nvim"
	}
	return cmd
}

// ---- Index arithmetic -------------------------------------------------
//
// These three functions exist because the same count appeared written in
// several places (List, Kanban and the project filter's navigation) with its
// own edge. Repeated, each copy is a place where a mutant can hang the loop
// and where no test reaches. By taking them out to pure functions the
// contract stays in one place and can be checked exhaustively, without
// building a whole view nor a database.

// cycleIndex moves idx within [0, n) wrapping around at the end.
//
// n <= 0 returns idx untouched: there is no list to move in and it is
// better to leave the index as it was than to invent a 0. delta can be negative.
func cycleIndex(idx, n, delta int) int {
	if n <= 0 {
		return idx
	}
	return ((idx+delta)%n + n) % n
}

// shiftIndex moves idx within [0, n) WITHOUT wrapping: it stays at the
// extreme instead of jumping to the other side.
func shiftIndex(idx, n, delta int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx+delta, 0), n-1)
}

// inRange says whether idx is a valid index within a list of n elements.
// It is the guard that appears before every tasks[idx] of the repo.
func inRange(idx, n int) bool {
	return idx >= 0 && idx < n
}

// taskAt returns the task at position idx, or nil if the index is not valid.
//
// It was extracted because the guard "cursor < len(tasks)" was written in
// front of every tasks[m.cursor] with its own shape. Its BOUNDARY mutant
// (which turns `<` into `<=`) can only be killed if the function receives
// the index: from the keyboard the cursor always arrives bounded, so the
// difference is unreachable. With the index as a parameter the case idx == len(tasks) can be tested directly.
func taskAt(tasks []model.Task, idx int) *model.Task {
	if !inRange(idx, len(tasks)) {
		return nil
	}
	return &tasks[idx]
}

// clampKanban keeps the board's cursor inside what there is.
//
// colLens is the number of cards of each column, in order. It is passed as
// a parameter instead of reading it from the model so that the contract can
// be checked without building a board: the rule (column inside, row inside
// that column) is what is meant to be pinned down, not the render.
//
// The reason it is needed: filtering by status changes the number of
// columns and with it the number of cards of each one, so an index that was
// valid a moment ago can end up out without anything having touched it.
func clampKanban(col, row int, colLens []int) (int, int) {
	if len(colLens) == 0 {
		return 0, 0
	}
	col = min(max(col, 0), len(colLens)-1)
	n := colLens[col]
	if n == 0 {
		return col, 0
	}
	// The row is bounded on both sides. The lower one was not bounded by the
	// previous code, but a negative kanbanRow would index backwards: the clamp
	// that promises "the cursor is inside the board" has to meet it downwards
	// too, although today no keyboard path produces a negative.
	return col, min(max(row, 0), n-1)
}

// kanbanMaxCards is how many cards are drawn in a column with the given height.
//
// Each card takes kanbanCardRows rows, and the +1 of the quotient makes the
// last one count even if only its top part fits: better a card truncated at
// the bottom than an empty gap at the bottom of the column.
//
// The minimum of one card guarantees that a tiny terminal keeps showing
// something instead of degenerating into an empty board.
//
// There is no minimum of content rows because it would do nothing: with 0,
// 1 or 2 rows the quotient gives 0 or 1 and the minimum of one card resolves
// it anyway. That floor was a branch no test could distinguish, just like
// the `if x < N { x = N }` of other parts of the code.
func kanbanMaxCards(maxHeight int) int {
	content := maxHeight - kanbanBoardChrome - filterHeaderRows
	return max((content+1)/kanbanCardRows, 1)
}

// kanbanHeader labels a column. When only part of the cards is seen it says
// how many of how many; if they all fit, only the total. The label's width
// is the column's minimum, so the text difference shows.
func kanbanHeader(status string, shown, total int) string {
	if shown < total {
		return fmt.Sprintf("─ %s (%d/%d) ", status, shown, total)
	}
	return fmt.Sprintf("─ %s (%d) ", status, total)
}

// ---- Rules of the people modal ---------------------------------------

// firstValidIndex returns idx if it applies to a list of n, and 0 if not. It
// is the "if the index does not apply, the first one" without looking at the
// elements: what the cycles that later choose by index need.
func firstValidIndex(idx, n int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx, 0), n-1)
}

// listWindowForHeight truncates the current page to the available height,
// moving the window so that the cursor stays inside.
//
// It is the third use of visibleRange in the program (the others are Kanban,
// Gantt and the filter), and the only one that also has to translate the
// cursor from a global index to a relative one within the page before using
// it. With pageStart and pageEnd equal, the window is the whole page.
//
// It was extracted from the render for the same reason as the others: they
// are four operations with two conditions, and without taking them out there
// is no way to check the truncation without building the whole view.
func listWindowForHeight(pageStart, pageEnd, cursor, maxHeight int) (int, int) {
	start, end := pageStart, pageEnd

	// visible is the rows left for tasks once the box's fixed ones are
	// discounted. With zero or less not even one fits, and the whole page is
	// painted: it is better for the box to overflow than to come out empty.
	//
	// The two halves are checked separately and in this order because they are
	// not interchangeable: with `visible <= 0` end-start <= visible cannot be
	// computed with meaning, and with the second half false the first can be
	// true or false depending on the sign of visible. Together in an || the
	// condition was shorter but its two edges were mixed, and the
	// `end-start` mutant did not tell itself apart from the `visible` one.
	visible := maxHeight - listFixedRows
	// `< 1` and not `<= 0`: over integers they are the SAME condition, so the
	// mutant that swaps one for the other changes nothing and no test kills it.
	// `< 1` does have an edge that exists -- visible == 1, a single row for
	// tasks -- and that edge has a case: maxHeight == listFixedRows + 1.
	if visible < 1 {
		return start, end
	}

	// No guard is needed for "the page fits whole". visibleRange with a size
	// larger than or equal to the total truncates the size to the total and
	// returns [0, total), which translated to absolute indexes is exactly
	// [pageStart, pageEnd): the same as the early return. The guard only added
	// an `if` whose edge -- end-start == visible -- gave the same result
	// through both branches, so its mutant could not be killed by any test.
	relStart, relEnd := visibleRange(cursor-pageStart, end-pageStart, visible)
	return pageStart + relStart, pageStart + relEnd
}

// separatorWidth is what the filter bar's horizontal separator measures: the
// inner width minus the two columns of the box.
//
// The floor at 0 is not decorative. strings.Repeat with a negative number
// blows up, and w-2 goes negative as soon as the window drops below two
// columns. Today m.width never goes below that, but a function that takes an
// int and crashes on the negative is a trap for the next caller.
func separatorWidth(innerW int) int {
	return max(innerW-2, 0)
}

// listContentWidth is the width left for the list's columns once the
// selection prefix ("> " or "  ") is discounted. It is the same "- 2" as the
// separator and for the same reason: both reserve the box.
func listContentWidth(innerW int) int {
	return separatorWidth(innerW)
}

// ganttLabelAndDays splits the Gantt's inner width between the label on the
// left and the day columns on the right.
//
// Two regimes: with room to spare the label stays at 30 fixed columns;
// below 70 inner columns it keeps a third, because 30 columns would eat half
// of the chart. Both have a floor: 14 of label so that the name fits, 7 of
// days so that a week fits.
//
// It was four lines in the render with its two thresholds and its two
// floors. The Gantt's box pads with spaces up to the inner width, so a
// "- 1" in the days does not show in the line's width: it shows in where the
// Monday captions fall, and that is too indirect for a test. As a pure
// function the split is checked whole over a sweep of widths.
func ganttLabelAndDays(innerW int) (labelW, dayCols int) {
	labelW = ganttFixedLabelWidth
	if innerW < ganttLabelThreshold {
		labelW = innerW / 3
	}
	labelW = max(labelW, ganttMinLabelWidth)

	dayCols = max(innerW-labelW-1, ganttMinDayCols)
	return labelW, dayCols
}

// detailHeightBudget splits the detail's height between comments and
// description.
//
// fixed are the lines that are not content: borders of the two boxes, the
// metadata, the separator and the Description label. avail is what is left,
// with a minimum of 3 so that the box makes sense even if the terminal is tiny.
//
// Out of avail, 2 lines are reserved for the separator and the comments
// label before deciding how many fit: maxCommentLines. The real budget is
// the number of comments, bounded between 1 and maxCommentLines (at least
// one line, or the "(no comments)"). The rest goes to the description, also
// with a minimum of one line.
//
// It was extracted because they were six chained operations inside the
// render, with four different floors, and none could be checked without building the modal.
func detailHeightBudget(maxHeight, commentCount int) (commentBudget, descBudget int) {
	const fixed = 12
	avail := max(maxHeight-fixed, 3)

	maxCommentLines := max(avail-2, 1)
	commentBudget = min(max(commentCount, 1), maxCommentLines)
	// What is left for the description does not need a floor: the comments
	// stay at most at avail-2, so there are always at least 2 lines left. A
	// max(..., 1) here was a branch that could not be activated.
	descBudget = avail - commentBudget
	return commentBudget, descBudget
}

// truncateAt returns s truncated to n bytes, or s itself if it is shorter.
//
// Bytes and not runes on purpose: they are RFC3339 timestamps, which are
// ASCII. In a timestamp all the "characters" are one byte long and a rune
// slice would only add a conversion with no effect.
//
// n <= 0 returns empty. Without that floor, s[:-1] blows up: the two callers
// pass positive constants, so today it is unreachable, but a function that
// takes an int and crashes on a negative is a trap waiting for a new
// caller.
// The floor at 0 is a max and not an if: at n == 0 the slice already
// returns "", so the branch was indistinguishable from not having one.
func truncateAt(s string, n int) string {
	return s[:min(len(s), max(n, 0))]
}

// clampTo bounds an index to [0, n). With n <= 0 it returns 0: with no list
// there is no position, and 0 is what all the callers show as "nothing".
//
// It is the shape that clampFilterOption, clampAssigneeIdx, clampOffdayIdx
// and the Kanban cursor's clamp repeated, each with its own edge and its
// own specialization of the "no list" case.
func clampTo(idx, n int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx, 0), n-1)
}

// taskAt2 is taskAt for text lists: the element at position idx, or ""
// if the index is not valid.
//
// The empty value is the one the callers already treated as "nothing
// selected", and centralizing it avoids the `if idx >= 0 && idx < len(s)`
// repeated in front of every suggestions[idx] of the program.
func taskAt2(items []string, idx int) string {
	if !inRange(idx, len(items)) {
		return ""
	}
	return items[idx]
}

// firstOrAt returns items[idx], or the first element if the index does not
// apply to that list.
//
// It is the rule of the dropdowns on tab: if nothing is selected, it
// completes with the first thing there is. Different from taskAt2, which
// returns "" in that case; that is why they are two functions and not one with a flag.
func firstOrAt(items []string, idx int) string {
	if len(items) == 0 {
		return ""
	}
	return items[min(max(idx, 0), len(items)-1)]
}

// currentProjectName is the project being operated on: the filter's one if
// there is one, and otherwise the first of the list.
//
// The filter only counts in List and Kanban: in Dashboard the selection goes
// elsewhere and the task form is done on the first project, which is what is seen.
//
// It was extracted as a pure function because the "first project" rule was
// written inside a switch on the view, and its edges (no projects, filter on
// a view that does not use it) were only reachable from the keyboard.
func currentProjectName(view viewKind, filterProject string, projects []model.Project) string {
	switch view {
	case viewList, viewKanban:
		if filterProject != "" {
			return filterProject
		}
	}
	if len(projects) > 0 {
		return projects[0].Name
	}
	return ""
}

// buildAssigneeRoster builds the modal's people list: each one with its
// total of tasks, how many are still active and how many off-days it has.
//
// "Me" is always there, even if it has nothing assigned: it is the default
// value new tasks are attributed to, so if it did not appear, whatever is
// highlighted in the modal would be the default.
//
// It skips whoever has no assignee (empty or "unassigned"): with no name
// there is nobody to attribute an off-day to. It returns the list sorted by
// name, which is what makes the index stable when the map reorders.
// buildAssigneeRoster summarizes tasks and off-days per person.
//
// The result is never empty: "Me" is always put in, even if there is no
// task without an assignee. That is on purpose — there is always somebody
// to look at, and that way the modal never opens without content. The
// callers asking "if the roster is empty" were protecting themselves against something that cannot happen.
func buildAssigneeRoster(tasks []model.Task, offdays []model.OffDay) []assigneeSummary {
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
	for _, t := range tasks {
		if model.IsUnassigned(t.Assignee) {
			continue
		}
		s := ensure(t.Assignee)
		s.Total++
		if t.IsActive() {
			s.Active++
		}
	}
	for _, o := range offdays {
		if model.IsUnassigned(o.Assignee) {
			continue
		}
		ensure(o.Assignee).OffDayCount++
	}

	result := make([]assigneeSummary, 0, len(byName))
	for _, s := range byName {
		result = append(result, *s)
	}
	// slices.SortFunc with strings.Compare instead of sort.Slice with a
	// hand-made comparator: the names come out of a map, so there are never
	// two equal ones, and that is why the comparator's `<` was equivalent to
	// `<=`. The comparison goes inside the library and there is no line to mutate.
	slices.SortFunc(result, func(a, b assigneeSummary) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result
}

// tasksForAssignee are the non-closed tasks of a person, in the order of
// entry.
func tasksForAssignee(tasks []model.Task, name string) []model.Task {
	var result []model.Task
	for _, t := range tasks {
		if t.Assignee == name && t.IsActive() {
			result = append(result, t)
		}
	}
	return result
}

// offDaysForAssignee are the off-days of a person, in the load order
// (ascending start date, which is how the DB returns them).
func offDaysForAssignee(offdays []model.OffDay, name string) []model.OffDay {
	var result []model.OffDay
	for _, o := range offdays {
		if o.Assignee == name {
			result = append(result, o)
		}
	}
	return result
}

// nameAt is the name of the person at position idx of the roster, or "" if
// the index is not valid. The guard is inside so that the caller does not
// have to repeat it before every roster[idx].
func nameAt(roster []assigneeSummary, idx int) string {
	if !inRange(idx, len(roster)) {
		return ""
	}
	return roster[idx].Name
}

// ---- Rules of the Dashboard ------------------------------------------------
//
// The Dashboard counts over m.tasks five different times (by project, by
// status, by person) and each count came with its own loop and its own
// project filter inside the render function. Here they are as pure
// functions: they take the tasks and the selected project, and they know nothing about the model.

// dashProjectTasks counts the active tasks of a project.
//
// project == "" counts the whole project, which is what the panel wants
// when there is none selected on the board.
func dashProjectTasks(tasks []model.Task, project string) int {
	n := 0
	for _, t := range tasks {
		if project != "" && t.ProjectName != project {
			continue
		}
		if t.IsActive() {
			n++
		}
	}
	return n
}

// dashStatusCounts splits tasks by status and returns the three totals that
// the Overview shows.
//
// done and cancelled are counted separately; everything else is "active",
// including each project's own workflow statuses (backlog, reviewing…). The
// byStatus map carries the same distribution and feeds the bars.
func dashStatusCounts(tasks []model.Task, project string) (active, done, cancelled int, byStatus map[string]int) {
	byStatus = map[string]int{}
	for _, t := range tasks {
		if project != "" && t.ProjectName != project {
			continue
		}
		switch t.Status {
		case model.CancelledStatus:
			cancelled++
		case model.DoneStatus:
			done++
		default:
			active++
		}
		byStatus[t.Status]++
	}
	return active, done, cancelled, byStatus
}

// dashAssigneeCounts counts tasks per person, with their active subcount.
func dashAssigneeCounts(tasks []model.Task, project string) (total, active map[string]int) {
	total = map[string]int{}
	active = map[string]int{}
	for _, t := range tasks {
		if project != "" && t.ProjectName != project {
			continue
		}
		total[t.Assignee]++
		if t.IsActive() {
			active[t.Assignee]++
		}
	}
	return total, active
}

// dashRowsAvailable are the rows left for content in a Dashboard column: the
// column's height minus what is already used and minus the fixed header.
//
// The floor is 0, not 1: a budget of 0 rows means "nothing else fits",
// which is different from "at least one line fits" and is what keeps the
// block from growing over the calculated height.
func dashRowsAvailable(colLines, used, headerRows int) int {
	return max(colLines-used-headerRows, 0)
}

// dashColumnWidths splits the inner width into two equal columns with a gap
// of 1 column in between.
func dashColumnWidths(innerW int) (left, right int) {
	half := innerW/2 - 1
	return half, half
}

// previewBudgetFor are the description lines that fit without pushing the
// content nor the keybinds off the screen.
//
// height - keybinds - minContentHeight, minus the two borders of the
// preview's box, and bounded between 1 and previewMaxLines. The minimum of 1
// guarantees that the description always has at least one line, even if the terminal is tiny.
func previewBudgetFor(height, keybindsHeight int) int {
	budget := height - keybindsHeight - minContentHeight - 2 // 2 = box borders
	return min(max(budget, 1), previewMaxLines)
}

// matchesStatus says whether a task's status passes the status filter.
//
// There are three modes: the "all active" one (everything but done and
// cancelled), the "all" one (no restriction) and the one of a concrete
// status (exact match).
func matchesStatus(statusFilter, taskStatus string, active bool) bool {
	switch statusFilter {
	case statusFilterAllActive:
		return active
	case "":
		return true
	default:
		return taskStatus == statusFilter
	}
}

// matchesPriority applies the priority filter. -1 means "any".
func matchesPriority(filterPriority, taskPriority int) bool {
	return filterPriority < 0 || filterPriority == taskPriority
}

// nextPriority is the cycle of the priority key (ctrl+p).
//
// The cycle depends on the status: in backlog it includes "none"
// (none→low→med→high→none, four steps) and out of backlog it does not
// (low→med→high→low, three). That is why it is not a global `% 4`.
//
// priority is truncated before cycling because it comes from the database
// and there is no guarantee it is in 0..3: an out-of-range 7 would make
// low→low with the three-step cycle. Truncating makes the function total.
func nextPriority(status string, priority int) int {
	p := min(max(priority, model.PriorityNone), model.PriorityHigh)
	if status == "backlog" {
		return (p + 1) % 4
	}
	return (p % 3) + 1
}

// joinSections joins blocks vertically without leaving empty rows between them.
func joinSections(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, s := range sections {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, "\n")
}

// visibleRange returns the range [start, end) of a window of size elements
// over a total, keeping the cursor inside the window.
func visibleRange(cursor, total, size int) (int, int) {
	// With no list or no window there is nothing to show.
	//
	// The two halves go separated, and compare against 1 and not against 0,
	// for two reasons that are the same:
	//
	//   - Separated: mixed in an `||`, one of the two's edge stays hidden
	//     behind the other's and neither can be checked on its own.
	//
	//   - Against 1: `total <= 0` and `total < 1` are the SAME condition over
	//     integers, so a mutant that swaps one for the other changes nothing
	//     and no test can kill it. `< 1` does have a reachable edge -- the
	//     total 1, which is a list with one element and a window containing
	//     it -- and that edge is a real case that deserves a test.
	if total < 1 {
		return 0, 0
	}
	if size < 1 {
		return 0, 0
	}
	// A window larger than the total is truncated to the total. The edge --
	// window equal to the total -- gave the same result through the count
	// below, so leaving the ">=" was another equivalent mutant.
	size = min(size, total)

	// The window is centered on the cursor and bounded on both sides at once.
	// With min instead of two ifs, the edge "start == total - size" stops
	// being a decision: it is the same number through both branches.
	start := min(max(cursor-size/2, 0), total-size)
	return start, start + size
}

// truncateSuffix is what is put in place of what is cut.
const truncateSuffix = ".."

// truncate cuts a text to limit characters by adding a suffix.
//
// Below the suffix's size the ellipsis does not fit, so it cuts raw: a
// slice with a negative index blows up. Today's callers pass positive
// constants, so the floor is defensive, but a function that takes an int
// and crashes on the negative is a trap for the next one.
//
// The parameter is called limit and not max because max is the built-in
// function, and here it is needed to bound from below.
func truncate(s string, limit int) string {
	if limit < len(truncateSuffix) {
		return s[:min(len(s), max(limit, 0))]
	}
	if len(s) <= limit {
		return s
	}
	return s[:limit-len(truncateSuffix)] + truncateSuffix
}

// singleLine collapses a multiline text to a single line so that it does
// not break a table row.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
