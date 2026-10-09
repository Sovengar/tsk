package tui

import (
	"fmt"
	"sort"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/harness"
	"tsk/internal/model"
)

// Model is the TUI's main model.
type Model struct {
	width, height int

	database  *db.DB
	config    config.Config
	projects  []model.Project
	tasks     []model.Task
	filteredT []model.Task // cached filtered tasks

	currentView viewKind
	cursor      int
	pageSize    int // tasks per page in the List view

	// Filters
	filterProject  string
	filterStatus   string // "" = all; statusFilterAllActive = active; other = exact status
	filterAssignee string
	filterTag      string
	filterPriority int // -1 = all
	filterActive   bool
	filterText     string

	// Kanban state
	kanbanCol int // column index in kanban
	kanbanRow int // row index within column

	// Gantt state
	offdays         []model.OffDay
	ganttCursor     int // row index in the gantt
	ganttOffsetDays int // horizontal scroll (days from the projection start)

	// Dashboard state
	dashProjectIdx    int // selected project index in dashboard
	showArchived      bool
	archivedProjects  []model.Project
	pendingSelectName string // project to select after reloading

	// Project modal (create/edit project)
	projectModalOpen      bool
	projectModalEdit      bool
	projectModalField     int // 0=name, 1=workflow, 2=list_order
	projectNameInput      string
	projectWorkflowInput  string
	projectListOrderInput string
	projectEditingName    string // original name when editing

	// Confirmation (archive/restore project, delete off-day)
	confirmOpen    bool
	confirmAction  string // "archive" | "unarchive" | "delete-offday"
	confirmProject string
	confirmOffday  model.OffDay

	// Assignee / off-day modal (opened with "m" from the Dashboard)
	assigneeModalOpen bool
	assigneeDetail    bool // false = people list, true = detail
	assigneeIdx       int
	assigneeOffdayIdx int

	// Off-day form
	offdayFormOpen   bool
	offdayFormField  int // 0=start, 1=end, 2=note
	offdayStartInput string
	offdayEndInput   string
	offdayNoteInput  string

	// Transient feedback toast
	toast     string
	toastKind string // "info" | "error"
	toastSeq  int

	// Detail modal
	detailOpen       bool
	detailTask       *model.Task
	detailComments   []model.Comment
	detailCommentSel int // -1 = none selected

	// Tag modal (opened with "t" from the Detail)
	tagOpen       bool
	tagInput      string
	tagSuggestIdx int // -1 = no suggestion selected

	// Inline description editor (embedded in the detail's box)
	descEditOpen      bool
	descEditTaskID    int64
	descEditHadDetail bool // the detail was already open before editing
	descEditTextarea  textarea.Model

	// Help modal
	helpOpen bool

	// Filter modal
	filterOpen      bool
	filterFieldIdx  int    // 0=project, 1=status, 2=assignee, 3=priority, 4=tag
	filterSearch    string // fuzzy search of the active field
	filterOptionIdx int    // option cursor of the active field

	// Ask AI picker (opened with "a" on a focused task)
	askAIOpen  bool
	askAIIndex int
	askAIList  []harness.Harness
	askAITask  *model.Task

	// New task modal
	newTaskOpen            bool
	newTaskFieldIdx        int // 0=priority, 1=title, 2=description, 3=assignee, 4=tags
	newTaskPriority        int
	newTaskTitle           string
	newTaskAssignee        string
	newTaskAssigneeSuggIdx int // -1 = none selected
	newTaskTags            []string
	newTaskTagInput        string
	newTaskTagSuggIdx      int // -1 = none selected
	newTaskProject         string
	newTaskErr             string // inline validation (e.g. title required)
	newTaskTextarea        textarea.Model

	// KeybindsBar
	statusbar KeybindsBar

	// Preview of the selected task
	preview PreviewBar
}

// New builds the model with the database.
func New(database *db.DB, cfg config.Config) Model {
	pageSize := resolvePageSize(cfg.ListPageSize)
	return Model{
		database:         database,
		config:           cfg,
		currentView:      viewList,
		filterPriority:   -1,
		filterStatus:     statusFilterAllActive,
		pageSize:         pageSize,
		detailCommentSel: -1,
		tagSuggestIdx:    -1,
		width:            80,
		height:           24,
		statusbar:        NewKeybindsBar(80),
		preview:          NewPreviewBar(80),
	}
}

// ---- Messages ----

type tasksLoadedMsg struct {
	tasks []model.Task
}

type projectsLoadedMsg struct {
	projects         []model.Project
	archivedProjects []model.Project
}

type offdaysLoadedMsg struct {
	offdays []model.OffDay
}

// taskCreateFailedMsg results from a failed task creation from the modal.
type taskCreateFailedMsg struct{ err error }

// askLaunchedMsg confirms a handoff was started (fire-and-forget).
type askLaunchedMsg struct{ name string }

// askFailedMsg carries a handoff spawn failure.
type askFailedMsg struct{ err error }

// offdaySavedMsg results from an off-day create or delete. err != nil indicates failure.
type offdaySavedMsg struct {
	err    error
	action string // "add" | "delete"
	name   string
}

// tagToggledMsg results from toggling a tag: it brings the updated task (for
// the Detail) and the reloaded listing (for List/Kanban).
type tagToggledMsg struct {
	taskID int64
	task   *model.Task
	tasks  []model.Task
}

// ---- Commands ----

func (m *Model) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.database.ListProjects()
		if err != nil {
			return projectsLoadedMsg{}
		}
		archived, err := m.database.ListArchivedProjects()
		if err != nil {
			archived = nil
		}
		return projectsLoadedMsg{projects: projects, archivedProjects: archived}
	}
}

func (m *Model) loadTasks() tea.Cmd {
	return func() tea.Msg {
		tasks, err := m.database.ListTasks("", "", "")
		if err != nil {
			return tasksLoadedMsg{}
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

// loadOffDays brings all the off-days for the Gantt's projection.
func (m *Model) loadOffDays() tea.Cmd {
	return func() tea.Msg {
		offdays, err := m.database.ListOffDays("")
		if err != nil {
			return offdaysLoadedMsg{}
		}
		return offdaysLoadedMsg{offdays: offdays}
	}
}

// taskActionCmd runs an action on a task and reloads the listing.
func (m Model) taskActionCmd(id int64, action func(int64) (*model.Task, error)) tea.Cmd {
	return func() tea.Msg {
		if _, err := action(id); err != nil {
			return nil
		}
		tasks, _ := m.database.ListTasks("", "", "")
		return tasksLoadedMsg{tasks: tasks}
	}
}

// toggleTagCmd adds the tag if the task does not have it, or removes it if it does.
// It reloads the listing so List/Kanban reflect the change.
func (m *Model) toggleTagCmd(taskID int64, tag string) tea.Cmd {
	return func() tea.Msg {
		t, err := m.database.GetTask(taskID)
		if err != nil {
			return nil
		}
		var updated *model.Task
		if model.HasTag(t.Tags, tag) {
			updated, err = m.database.RemoveTaskTags(taskID, []string{tag})
		} else {
			updated, err = m.database.AddTaskTags(taskID, []string{tag})
		}
		if err != nil {
			return nil
		}
		tasks, _ := m.database.ListTasks("", "", "")
		return tagToggledMsg{taskID: taskID, task: updated, tasks: tasks}
	}
}

// mergedWorkflow combines all projects' workflows into a single one.
func (m *Model) mergedWorkflow() []string {
	seen := make(map[string]bool)
	var result []string
	for _, p := range m.projects {
		for _, status := range p.Workflow {
			if !seen[status] {
				seen[status] = true
				result = append(result, status)
			}
		}
	}
	if len(result) == 0 {
		return model.DefaultWorkflow
	}
	return result
}

// kanbanWorkflow returns the Kanban's columns: the workflow of the project in
// context if one is selected, or the union of all of them in "all projects".
func (m *Model) kanbanWorkflow() []string {
	if m.filterProject != "" {
		if p := m.projectByName(m.filterProject); p != nil {
			return p.Workflow
		}
	}
	return m.mergedWorkflow()
}

// projectByName looks up an active project by name.
func (m *Model) projectByName(name string) *model.Project {
	for i := range m.projects {
		if m.projects[i].Name == name {
			return &m.projects[i]
		}
	}
	return nil
}

// commonWorkflow returns the statuses present in ALL projects, in the order
// of the first one. It is the valid set for filtering by status when no
// project is selected: a status that does not exist in all of them cannot
// filter tasks of all of them. With no projects it falls back to the default workflow.
func (m *Model) commonWorkflow() []string {
	if len(m.projects) == 0 {
		return model.DefaultWorkflow
	}
	var result []string
	for _, s := range m.projects[0].Workflow {
		common := true
		for _, p := range m.projects[1:] {
			if !model.HasStatus(p.Workflow, s) {
				common = false
				break
			}
		}
		if common {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return model.DefaultWorkflow
	}
	return result
}

// taskMatchesFilter tells whether a task passes the active filters. The
// status is solved in a single place: statusFilterAllActive (default) leaves
// out the terminal ones, "" (all) does not restrict, and any other value
// demands an exact match. It is shared by List, Kanban and Gantt so that the
// filter header is consistent across all views.
func (m *Model) taskMatchesFilter(t model.Task) bool {
	if !matchesStatus(m.filterStatus, t.Status, t.IsActive()) {
		return false
	}
	if m.filterProject != "" && t.ProjectName != m.filterProject {
		return false
	}
	if m.filterAssignee != "" && t.Assignee != m.filterAssignee {
		return false
	}
	if m.filterTag != "" && !model.HasTag(t.Tags, m.filterTag) {
		return false
	}
	return matchesPriority(m.filterPriority, t.Priority)
}

// filteredTasks returns the filtered tasks.
func (m *Model) filteredTasks() []model.Task {
	if m.filteredT != nil {
		return m.filteredT
	}

	var result []model.Task
	for _, t := range m.tasks {
		if m.taskMatchesFilter(t) {
			result = append(result, t)
		}
	}

	m.filteredT = result
	return result
}

func (m *Model) invalidateFilterCache() {
	m.filteredT = nil
	m.clampListCursor()
}

// listPageSize returns the effective page size of the List view.
func (m *Model) listPageSize() int {
	if m.pageSize <= 0 {
		return config.DefaultPageSize
	}
	return m.pageSize
}

// listWindow is the List view's pagination solved in a single place.
//
// It existed because the calculation was spread: the page was derived from the
// cursor multiplied by the size, and five callers repeated that arithmetic,
// each with its own edge. Spread out, each copy is a place where a mutant can
// hang the loop (a `cursor++` inside an inverted `if` never terminates) or die
// without any test noticing. Here there is no manual advancement at all:
// everything comes out of min/max over integers, and every consumer reads the same window.
type listWindow struct {
	total int // tasks after filtering
	size  int // rows per page
	page  int // current page, base 0
	pages int // total pages, always >= 1
	start int // first visible index
	end   int // index right after the last visible
}

func (m *Model) listWindow() listWindow {
	size := m.listPageSize()
	total := len(m.filteredTasks())
	pages := max(1, (total+size-1)/size)
	// The page is bounded to [0, pages-1]. Before it was not bounded and an
	// out-of-range cursor gave an empty window with a caption like "Page 20/2";
	// it is unreachable from the UI because clampListCursor runs first, but a
	// view that preserves itself should not depend on that order.
	page := min(max(m.cursor/size, 0), pages-1)
	start := page * size
	return listWindow{
		total: total,
		size:  size,
		page:  page,
		pages: pages,
		start: start,
		end:   min(start+size, total),
	}
}

// clampCursor puts a cursor inside the visible window. With an empty list
// the cursor is 0; otherwise it is bounded to the shown range. It is
// expressed with min/max instead of a chain of if: the chain has branches
// that only tell themselves apart if the test reaches each one, and its
// negated version leaves a cursor at -1 that the rest of the view does not recover from.
func (w listWindow) clampCursor(cursor int) int {
	if w.total == 0 {
		return 0
	}
	return min(max(cursor, w.start), w.end-1)
}

// nextPageStart returns the cursor to the first element of the next page, and
// false if the current one is the last.
func (w listWindow) nextPageStart() (int, bool) {
	if w.page >= w.pages-1 {
		return 0, false
	}
	return w.start + w.size, true
}

// prevPageStart returns the cursor to the first element of the previous page,
// and false if it is already on the first.
func (w listWindow) prevPageStart() (int, bool) {
	if w.page == 0 {
		return 0, false
	}
	return w.start - w.size, true
}

// pageBounds returns the range [start, end) of tasks that make up the current
// page. It is the only access left to the window: totalPages and currentPage
// disappeared because each caller repeated its own arithmetic and both things
// now come out of listWindow.
func (m *Model) pageBounds() (int, int) {
	w := m.listWindow()
	return w.start, w.end
}

// pageLegend describes the page's range: "1-10 of 306 · Page 1/31".
func (m *Model) pageLegend() string {
	w := m.listWindow()
	first := 0
	if w.total > 0 {
		first = w.start + 1
	}
	return fmt.Sprintf("%d-%d of %d · Page %d/%d",
		first, w.end, w.total, w.page+1, w.pages)
}

// clampListCursor keeps the cursor inside the filtered tasks.
func (m *Model) clampListCursor() {
	m.cursor = m.listWindow().clampCursor(m.cursor)
}

// ---- Init / Update / View ----

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadProjects(), m.loadTasks())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.statusbar.SetWidth(msg.Width)
		m.preview.SetWidth(msg.Width)
		if m.descEditOpen {
			m.resizeDescEditor()
		}
		if m.newTaskOpen {
			m.newTaskTextarea.SetWidth(m.newTaskTextareaWidth())
		}
		return m, nil

	case projectsLoadedMsg:
		m.projects = msg.projects
		m.archivedProjects = msg.archivedProjects
		if m.pendingSelectName != "" {
			m.selectProjectByName(m.pendingSelectName)
			m.pendingSelectName = ""
		}
		m.clampDashProjectIdx()
		return m, nil

	case projectSavedMsg:
		return m.handleProjectSaved(msg)

	case toastExpiredMsg:
		if msg.seq == m.toastSeq {
			m.toast = ""
			m.toastKind = ""
		}
		return m, nil

	case tasksLoadedMsg:
		m.tasks = msg.tasks
		m.invalidateFilterCache()
		return m, nil

	case tagToggledMsg:
		if msg.task != nil {
			m.tasks = msg.tasks
			m.invalidateFilterCache()
			if m.detailTask != nil && m.detailTask.ID == msg.taskID {
				m.detailTask = msg.task
			}
		}
		return m, nil

	case offdaysLoadedMsg:
		m.offdays = msg.offdays
		return m, nil

	case offdaySavedMsg:
		return m.handleOffdaySaved(msg)

	case editorFinishedMsg:
		if msg.err != nil {
			return m, nil
		}
		if msg.file != "" {
			return m, m.updateTaskFromEdit(msg.taskID, msg.file)
		}
		return m, nil

	case taskCreateFailedMsg:
		if msg.err != nil {
			return m, m.setToast(msg.err.Error(), "error")
		}
		return m, nil

	case askLaunchedMsg:
		return m, m.setToast("Handoff launched to "+msg.name, "info")

	case askFailedMsg:
		return m, m.setToast(msg.err.Error(), "error")

	case commentsLoadedMsg:
		if m.detailOpen && m.detailTask != nil && m.detailTask.ID == msg.taskID {
			m.detailComments = msg.comments
			m.detailCommentSel = msg.selectIdx
		}
		return m, nil

	case commentFinishedMsg:
		if msg.err != nil || msg.body == "" {
			return m, nil
		}
		return m, m.addCommentCmd(msg.taskID, msg.body)

	case tea.PasteMsg:
		// The inline editor's and the task form's textareas handle the paste.
		if m.descEditOpen {
			return m.handleDescEditKey(msg)
		}
		if m.newTaskOpen {
			return m.handleNewTaskPaste(msg)
		}
		if m.tagOpen {
			return m.handleTagPaste(msg.Content)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	default:
		// The inline editor's textarea emits private package messages
		// (pasteMsg/copyMsg) as a result of its async commands. They are
		// neither tea.KeyMsg nor tea.PasteMsg, so they have to be forwarded to its Update.
		if m.descEditOpen {
			return m.handleDescEditKey(msg)
		}
		if goesToNewTaskTextarea(m.newTaskOpen, m.newTaskFieldIdx) {
			var cmd tea.Cmd
			m.newTaskTextarea, cmd = m.newTaskTextarea.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// goesToNewTaskTextarea says whether a message that is neither key nor paste
// has to be forwarded to the form's description textarea.
//
// The decision was written inside Update's default, and there it could not
// be checked: the messages arriving through that branch are private to the
// textarea package (pasteMsg, copyMsg), which are not exported, so a test
// cannot build one and see where it ends up. With the condition extracted to
// a pure function, the routing can be checked whole: which combinations go
// to the textarea and which do not, without needing the message that triggers it.
//
// Note on the edge: the description field is the ONLY one carrying an
// embedded textarea, because it is the only multiline one. The other fields
// are one-line and are typed by hand.
func goesToNewTaskTextarea(newTaskOpen bool, fieldIdx int) bool {
	return newTaskOpen && fieldIdx == newTaskFieldDescription
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Destructive action confirmation (strict overlay)
	if m.confirmOpen {
		return m.handleConfirmKey(key)
	}

	// Project modal takes priority
	if m.projectModalOpen {
		return m.handleProjectModalKey(key)
	}

	// New task modal takes priority
	if m.newTaskOpen {
		return m.handleNewTaskKey(msg)
	}

	// Inline description editor (can overlay the detail)
	if m.descEditOpen {
		return m.handleDescEditKey(msg)
	}

	// Tag modal (overlays the detail)
	if m.tagOpen {
		return m.handleTagModalKey(key)
	}

	// Ask AI picker (overlays the detail)
	if m.askAIOpen {
		return m.handleAskAIKey(key)
	}

	// Detail modal keys
	if m.detailOpen {
		return m.handleDetailKey(key)
	}

	// Filter modal keys
	if m.filterOpen {
		return m.handleFilterModalKey(key)
	}

	// Assignee / off-day modal (form above the list)
	if m.offdayFormOpen {
		return m.handleOffdayFormKey(key)
	}
	if m.assigneeModalOpen {
		return m.handleAssigneeModalKey(key)
	}

	// Global keys
	if newM, cmd, handled := handleGlobalKeys(&m, msg); handled {
		return newM, cmd
	}

	// View-specific keys
	switch m.currentView {
	case viewDashboard:
		return m.handleDashboardKey(key)
	case viewList:
		return m.handleListKey(key)
	case viewKanban:
		return m.handleKanbanKey(key)
	case viewGantt:
		return m.handleGanttKey(key)
	}

	return m, nil
}

func (m Model) handleDashboardKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab", "j", "down":
		// Cycle through visible projects
		m.dashProjectIdx = cycleIndex(m.dashProjectIdx, len(m.dashProjectList()), 1)
	case "k", "up":
		m.dashProjectIdx = cycleIndex(m.dashProjectIdx, len(m.dashProjectList()), -1)
	case "i":
		// New project: the Dashboard holds the global config
		return m, m.openProjectModal(false)
	case "e":
		if p := m.selectedDashProject(); p != nil {
			return m, m.openProjectModalForEdit(p)
		}
	case "d":
		if p := m.selectedDashProject(); p != nil && !m.showArchived {
			m.confirmOpen = true
			m.confirmAction = "archive"
			m.confirmProject = p.Name
		}
	case "r":
		if p := m.selectedDashProject(); p != nil && m.showArchived {
			m.confirmOpen = true
			m.confirmAction = "unarchive"
			m.confirmProject = p.Name
		}
	case "A":
		m.showArchived = !m.showArchived
		m.dashProjectIdx = 0
	case "m":
		// Manage off-days per person.
		m.openAssigneeModal()
		return m, m.loadOffDays()
	}
	return m, nil
}

func (m Model) handleListKey(key string) (tea.Model, tea.Cmd) {
	m.clampListCursor()
	tasks := m.filteredTasks()

	switch key {
	case "tab":
		// Switch project by cycling the Project filter, as in the Dashboard.
		m.cycleProjectFilter(1)
		m.cursor = 0
		return m, nil
	case "j", "down":
		// Go down to the end of the page; it bounds instead of incrementing
		// inside an if, which is the shape that hangs when the increment is inverted.
		w := m.listWindow()
		m.cursor = w.clampCursor(m.cursor + 1)
	case "k", "up":
		w := m.listWindow()
		m.cursor = w.clampCursor(m.cursor - 1)
	case "n":
		// Next page: jump to the first element of the next page.
		if next, ok := m.listWindow().nextPageStart(); ok {
			m.cursor = next
		}
	case "p":
		// Previous page: jump to the first element of the previous page.
		if prev, ok := m.listWindow().prevPageStart(); ok {
			m.cursor = prev
		}
	case "N":
		// Go to the last page: cursor to the last element.
		if w := m.listWindow(); w.total > 0 {
			m.cursor = w.total - 1
		}
	case "P":
		// Go to the first page: cursor to the first element.
		m.cursor = 0
	case "e":
		// Edit the description inline, from the TUI itself.
		return m, m.openDescEditor()
	case "E":
		// Full external editor (write in nvim).
		return m, m.editSelectedTask()
	case "i":
		// New task
		return m, m.newTask()
	case "a":
		// Ask AI: hand the focused task off to a harness.
		return m, m.openAskAI(m.selectedTask())
	case "enter":
		if inRange(m.cursor, len(tasks)) {
			t := tasks[m.cursor]
			m.detailOpen = true
			m.detailTask = &t
			m.detailComments = nil
			m.detailCommentSel = -1
			return m, m.loadCommentsCmd(t.ID)
		}
	case "s":
		// Start: move to next status
		if inRange(m.cursor, len(tasks)) {
			return m, m.taskActionCmd(tasks[m.cursor].ID, m.database.StartTask)
		}
	case "d":
		// Done
		if inRange(m.cursor, len(tasks)) {
			return m, m.taskActionCmd(tasks[m.cursor].ID, m.database.DoneTask)
		}
	case "x":
		// Cancel
		if inRange(m.cursor, len(tasks)) {
			return m, m.taskActionCmd(tasks[m.cursor].ID, m.database.CancelTask)
		}
	case "/":
		// Open filter modal
		return m, m.openFilterModal()
	case "ctrl+p":
		if inRange(m.cursor, len(tasks)) {
			t := tasks[m.cursor]
			next := nextPriority(t.Status, t.Priority)
			return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
				return m.database.UpdateTask(id, map[string]any{"priority": next})
			})
		}
	}

	return m, nil
}

func (m Model) handleKanbanKey(key string) (tea.Model, tea.Cmd) {
	m.clampKanbanCursor()

	// Open the filters before the empty-columns cut, so that it works even if
	// the board has nothing.
	if key == "/" {
		return m, m.openFilterModal()
	}

	// Tab changes project by cycling the Project filter (like the Dashboard).
	// The columns are navigated with h/l or ←/→, which already did the same.
	if key == "tab" {
		m.cycleProjectFilter(1)
		m.clampKanbanCursor()
		return m, nil
	}

	cols := m.kanbanColumns()
	if len(cols) == 0 {
		return m, nil
	}
	colTasks := cols[m.kanbanCol].tasks

	switch key {
	case "h", "left":
		// Changing column resets the row: the row index is per column, so
		// leaving it pointing where it no longer is would be a "selectedTask"
		// of another column.
		if prev := shiftIndex(m.kanbanCol, len(cols), -1); prev != m.kanbanCol {
			m.kanbanCol = prev
			m.kanbanRow = 0
		}
	case "l", "right":
		if next := shiftIndex(m.kanbanCol, len(cols), 1); next != m.kanbanCol {
			m.kanbanCol = next
			m.kanbanRow = 0
		}
	case "j", "down":
		m.kanbanRow = cycleIndex(m.kanbanRow, len(colTasks), 1)
	case "k", "up":
		m.kanbanRow = cycleIndex(m.kanbanRow, len(colTasks), -1)
	case "s":
		// Move task right (advance status) according to ITS project's workflow:
		// the merge's order may not exist in the project and the move would be
		// silently rejected by MoveTask.
		if inRange(m.kanbanRow, len(colTasks)) {
			t := colTasks[m.kanbanRow]
			if p := m.projectByName(t.ProjectName); p != nil {
				if nextStatus, ok := model.NextStatus(p.Workflow, t.Status); ok {
					return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
						return m.database.MoveTask(id, nextStatus)
					})
				}
			}
		}
	case "S":
		// Move task left (retreat status) according to its project's workflow.
		if inRange(m.kanbanRow, len(colTasks)) {
			t := colTasks[m.kanbanRow]
			if p := m.projectByName(t.ProjectName); p != nil {
				if prevStatus, ok := model.PrevStatus(p.Workflow, t.Status); ok {
					return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
						return m.database.MoveTask(id, prevStatus)
					})
				}
			}
		}
	case "d":
		// Done
		if inRange(m.kanbanRow, len(colTasks)) {
			return m, m.taskActionCmd(colTasks[m.kanbanRow].ID, m.database.DoneTask)
		}
	case "x":
		// Cancel
		if inRange(m.kanbanRow, len(colTasks)) {
			return m, m.taskActionCmd(colTasks[m.kanbanRow].ID, m.database.CancelTask)
		}
	case "e":
		// Edit the description inline, from the TUI itself.
		return m, m.openDescEditor()
	case "E":
		// Full external editor (write in nvim).
		return m, m.editSelectedTask()
	case "i":
		// New task
		return m, m.newTask()
	case "a":
		// Ask AI: hand the focused task off to a harness.
		return m, m.openAskAI(m.selectedTask())
	case "enter":
		if inRange(m.kanbanRow, len(colTasks)) {
			t := colTasks[m.kanbanRow]
			m.detailOpen = true
			m.detailTask = &t
			m.detailComments = nil
			m.detailCommentSel = -1
			return m, m.loadCommentsCmd(t.ID)
		}
	case "ctrl+p":
		if inRange(m.kanbanRow, len(colTasks)) {
			t := colTasks[m.kanbanRow]
			next := nextPriority(t.Status, t.Priority)
			return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
				return m.database.UpdateTask(id, map[string]any{"priority": next})
			})
		}
	}

	return m, nil
}

func (m Model) tasksInColumn(status string) []model.Task {
	var result []model.Task
	for _, t := range m.tasks {
		if t.Status != status {
			continue
		}
		if !m.taskMatchesFilter(t) {
			continue
		}
		result = append(result, t)
	}
	return result
}

func (m Model) handleDetailKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// First deselect the comment; if there is none, close the modal.
		if m.detailCommentSel >= 0 {
			m.detailCommentSel = -1
			return m, nil
		}
		m.detailOpen = false
		m.detailTask = nil
		m.detailComments = nil
		return m, nil
	case "j", "down":
		if len(m.detailComments) == 0 {
			return m, nil
		}
		m.detailCommentSel = nextCommentSel(m.detailCommentSel, len(m.detailComments))
		return m, nil
	case "k", "up":
		m.detailCommentSel = prevCommentSel(m.detailCommentSel)
		return m, nil
	case "c":
		// New comment in the external editor.
		if m.detailTask != nil {
			editorCmd := editorCommand(m.config.Editor.Command)
			return m, commentCmd(m.detailTask.ID, editorCmd)
		}
	case "t":
		// Open the opened task's tag modal.
		if m.detailTask != nil {
			m.tagOpen = true
			m.tagInput = ""
			m.tagSuggestIdx = -1
		}
	case "a":
		// Ask AI: hand the opened task off to a harness.
		return m, m.openAskAI(m.detailTask)
	case "e":
		// Edit the description inline; the detail stays open behind.
		if m.detailTask != nil {
			return m, m.openDescEditor()
		}
	case "E":
		// Full external editor (write in nvim).
		if m.detailTask != nil {
			editorCmd := editorCommand(m.config.Editor.Command)
			cmd := editTaskCmd(*m.detailTask, editorCmd)
			m.detailOpen = false
			m.detailTask = nil
			return m, cmd
		}
	case "s":
		if m.detailTask != nil {
			id := m.detailTask.ID
			m.detailOpen = false
			m.detailTask = nil
			return m, m.taskActionCmd(id, m.database.StartTask)
		}
	case "d":
		if m.detailTask == nil {
			return m, nil
		}
		// With a comment selected, delete the comment.
		if inRange(m.detailCommentSel, len(m.detailComments)) {
			comment := m.detailComments[m.detailCommentSel]
			return m, m.deleteCommentCmd(m.detailTask.ID, comment.ID, m.detailCommentSel)
		}
		// With no selection, mark the task as done.
		id := m.detailTask.ID
		m.detailOpen = false
		m.detailTask = nil
		m.detailComments = nil
		return m, m.taskActionCmd(id, m.database.DoneTask)
	case "x":
		if m.detailTask != nil {
			id := m.detailTask.ID
			m.detailOpen = false
			m.detailTask = nil
			m.detailComments = nil
			return m, m.taskActionCmd(id, m.database.CancelTask)
		}
	}
	return m, nil
}

func (m Model) uniqueAssignees() []string {
	seen := make(map[string]bool)
	result := []string{"Me"} // always present, it is the user
	seen["Me"] = true
	for _, t := range m.tasks {
		if !seen[t.Assignee] && t.Assignee != "" {
			seen[t.Assignee] = true
			result = append(result, t.Assignee)
		}
	}
	return result
}

// uniqueTags returns the tags in use across all tasks, without repeats and
// sorted alphabetically. It is the option source of the tag filter.
func (m Model) uniqueTags() []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range m.tasks {
		for _, tag := range t.Tags {
			if !seen[tag] {
				seen[tag] = true
				result = append(result, tag)
			}
		}
	}
	sort.Strings(result)
	return result
}

// selectedTask returns the task selected in the current view, or nil if there is none.
func (m *Model) selectedTask() *model.Task {
	switch m.currentView {
	case viewList:
		tasks := m.filteredTasks()
		if inRange(m.cursor, len(tasks)) {
			return &tasks[m.cursor]
		}
	case viewKanban:
		cols := m.kanbanColumns()
		if !inRange(m.kanbanCol, len(cols)) {
			break
		}
		tasks := cols[m.kanbanCol].tasks
		if inRange(m.kanbanRow, len(tasks)) {
			return &tasks[m.kanbanRow]
		}
	case viewGantt:
		rows := m.ganttRows()
		if rowIsTask(rows, m.ganttCursor) {
			return &rows[m.ganttCursor].entry.Task
		}
	}
	return nil
}

// previewBudget calculates how many description lines the preview can show
// without pushing the content nor the keybinds off the screen.
func (m Model) previewBudget(keybindsHeight int) int {
	return previewBudgetFor(m.height, keybindsHeight)
}

// overlayKind returns the active modal, in the same priority order as
// handleKey: confirmation, project, new task, description editor, tags,
// detail, filters.
func (m Model) overlayKind() overlayKind {
	switch {
	case m.confirmOpen:
		return overlayConfirm
	case m.projectModalOpen:
		return overlayProject
	case m.newTaskOpen:
		return overlayNewTask
	case m.descEditOpen:
		return overlayDescEdit
	case m.tagOpen:
		return overlayTag
	case m.askAIOpen:
		return overlayAskAI
	case m.detailOpen:
		return overlayDetail
	case m.filterOpen:
		return overlayFilter
	case m.offdayFormOpen:
		return overlayOffdayForm
	case m.assigneeModalOpen && m.assigneeDetail:
		return overlayAssigneeDetail
	case m.assigneeModalOpen:
		return overlayAssignee
	}
	return overlayNone
}

func (m Model) View() tea.View {
	// KeybindsBar always at the bottom.
	m.statusbar.SetView(m.currentView)
	m.statusbar.SetOverlay(m.overlayKind())
	keybinds := m.statusbar.View()
	keybindsHeight := lineCount(keybinds)

	// The preview adjusts to the leftover height so as not to push the keybinds.
	m.preview.SetTask(m.selectedTask())
	m.preview.SetMaxLines(m.previewBudget(keybindsHeight))
	preview := m.preview.View()

	// The detail and the inline editor already show the description: no preview.
	if m.detailOpen || m.descEditOpen {
		preview = ""
	}

	// Feedback toast: it takes one line above the preview.
	toastView := m.renderToast()

	budget := contentBudget(m.height, lineCount(preview)+lineCount(toastView), keybindsHeight)

	var content string
	switch m.currentView {
	case viewDashboard:
		content = m.renderDashboard(budget)
	case viewKanban:
		content = m.renderKanban(budget)
	case viewGantt:
		content = m.renderGantt(budget)
	case viewList:
		content = m.renderList(budget)
	default:
		content = m.renderDashboard(budget)
	}

	if m.detailOpen && m.detailTask != nil {
		content = m.renderDetail(m.detailTask, budget)
	}

	if m.tagOpen {
		content = m.renderTagModal(content)
	}

	if m.askAIOpen {
		content = m.renderAskAIModal(content)
	}

	if m.filterOpen {
		content = m.renderFilterModal(content)
	}

	if m.newTaskOpen {
		content = m.renderNewTaskModal(content)
	}

	if m.projectModalOpen {
		content = m.renderProjectModal(content)
	}

	if m.assigneeModalOpen {
		content = m.renderAssigneeModal(content)
	}

	if m.offdayFormOpen {
		content = m.renderOffdayForm(content)
	}

	if m.confirmOpen {
		content = m.renderConfirmModal(content)
	}

	if m.helpOpen {
		content = m.renderHelpModal(content)
	}

	v := tea.NewView(joinSections(content, toastView, preview, keybinds))
	v.AltScreen = true
	return v
}
