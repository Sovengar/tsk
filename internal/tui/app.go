package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// Model es el modelo principal de la TUI.
type Model struct {
	width, height int

	database  *db.DB
	config    config.Config
	projects  []model.Project
	tasks     []model.Task
	filteredT []model.Task // cached filtered tasks

	currentView viewKind
	cursor      int
	pageSize    int // tareas por página en la vista List

	// Filters
	filterProject    string
	filterStatus     string
	filterAssignee   string
	filterPriority   int // -1 = all
	filterActive     bool
	filterText       string
	filterActiveOnly bool // ocultar done/cancelled por defecto

	// Kanban state
	kanbanCol int // column index in kanban
	kanbanRow int // row index within column

	// Dashboard state
	dashProjectIdx    int // selected project index in dashboard
	showArchived      bool
	archivedProjects  []model.Project
	pendingSelectName string // proyecto a seleccionar tras recargar

	// Project modal (crear/editar proyecto)
	projectModalOpen      bool
	projectModalEdit      bool
	projectModalField     int // 0=name, 1=workflow, 2=list_order
	projectNameInput      string
	projectWorkflowInput  string
	projectListOrderInput string
	projectEditingName    string // nombre original al editar

	// Confirmación (archivar/restaurar)
	confirmOpen    bool
	confirmAction  string // "archive" | "unarchive"
	confirmProject string

	// Toast de feedback transitorio
	toast     string
	toastKind string // "info" | "error"
	toastSeq  int

	// Detail modal
	detailOpen       bool
	detailTask       *model.Task
	detailComments   []model.Comment
	detailCommentSel int // -1 = ninguno seleccionado

	// Help modal
	helpOpen bool

	// Filter modal
	filterOpen     bool
	filterFieldIdx int // 0=project, 1=status, 2=assignee, 3=priority

	// New task modal
	newTaskOpen            bool
	newTaskTitle           string
	newTaskPriority        int
	newTaskAssignee        string
	newTaskAssigneeSuggIdx int // -1 = none selected
	newTaskProject         string
	newTaskFieldIdx        int // 0=title, 1=priority, 2=assignee

	// KeybindsBar
	statusbar KeybindsBar

	// Preview de la tarea seleccionada
	preview PreviewBar
}

// New construye el modelo con la base de datos.
func New(database *db.DB, cfg config.Config) Model {
	pageSize := cfg.ListPageSize
	if pageSize <= 0 {
		pageSize = config.DefaultPageSize
	}
	return Model{
		database:         database,
		config:           cfg,
		currentView:      viewList,
		filterPriority:   -1,
		filterActiveOnly: true,
		pageSize:         pageSize,
		detailCommentSel: -1,
		width:            80,
		height:           24,
		statusbar:        NewKeybindsBar(80),
		preview:          NewPreviewBar(80),
	}
}

// ---- Mensajes ----

type tasksLoadedMsg struct {
	tasks []model.Task
}

type projectsLoadedMsg struct {
	projects         []model.Project
	archivedProjects []model.Project
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

// taskActionCmd ejecuta una acción sobre una tarea y recarga el listado.
func (m Model) taskActionCmd(id int64, action func(int64) (*model.Task, error)) tea.Cmd {
	return func() tea.Msg {
		if _, err := action(id); err != nil {
			return nil
		}
		tasks, _ := m.database.ListTasks("", "", "")
		return tasksLoadedMsg{tasks: tasks}
	}
}

// mergedWorkflow combina los workflows de todos los proyectos en uno solo.
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

// filteredTasks devuelve las tareas filtradas.
func (m *Model) filteredTasks() []model.Task {
	if m.filteredT != nil {
		return m.filteredT
	}

	var result []model.Task
	for _, t := range m.tasks {
		// Ocultar done/cancelled por defecto (a menos que el usuario filtre por esos estados)
		if m.filterActiveOnly && m.filterStatus == "" {
			if t.Status == "done" || t.Status == "cancelled" {
				continue
			}
		}
		if m.filterProject != "" && t.ProjectName != m.filterProject {
			continue
		}
		if m.filterStatus != "" && t.Status != m.filterStatus {
			continue
		}
		if m.filterAssignee != "" && t.Assignee != m.filterAssignee {
			continue
		}
		if m.filterPriority >= 0 && t.Priority != m.filterPriority {
			continue
		}
		result = append(result, t)
	}

	m.filteredT = result
	return result
}

func (m *Model) invalidateFilterCache() {
	m.filteredT = nil
	m.clampListCursor()
}

// listPageSize devuelve el tamaño de página efectivo de la vista List.
func (m *Model) listPageSize() int {
	if m.pageSize <= 0 {
		return config.DefaultPageSize
	}
	return m.pageSize
}

// totalPages devuelve el número de páginas (mínimo 1, aunque no haya tareas).
func (m *Model) totalPages() int {
	size := m.listPageSize()
	total := len(m.filteredTasks())
	pages := (total + size - 1) / size
	if pages < 1 {
		pages = 1
	}
	return pages
}

// currentPage devuelve el índice de página (0-based) que contiene al cursor.
func (m *Model) currentPage() int {
	return m.cursor / m.listPageSize()
}

// pageBounds devuelve el rango [start, end) de tareas que forman la página actual.
func (m *Model) pageBounds() (int, int) {
	size := m.listPageSize()
	total := len(m.filteredTasks())
	start := m.currentPage() * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	return start, end
}

// pageLegend describe el rango de la página: "1-10 of 306 · Page 1/31".
func (m *Model) pageLegend() string {
	total := len(m.filteredTasks())
	start, end := m.pageBounds()
	first := 0
	if total > 0 {
		first = start + 1
	}
	return fmt.Sprintf("%d-%d of %d · Page %d/%d",
		first, end, total, m.currentPage()+1, m.totalPages())
}

// clampListCursor mantiene el cursor dentro de las tareas filtradas.
func (m *Model) clampListCursor() {
	total := len(m.filteredTasks())
	if total == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= total {
		m.cursor = total - 1
	}
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

	case editorFinishedMsg:
		if msg.err != nil {
			return m, nil
		}
		if msg.file != "" {
			return m, m.updateTaskFromEdit(msg.taskID, msg.file)
		}
		return m, nil

	case newTaskFinishedMsg:
		if msg.err != nil {
			return m, nil
		}
		if msg.file != "" {
			return m, m.createTaskFromEdit(msg.projectName, msg.file)
		}
		return m, nil

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

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Confirmación de acción destructiva (overlay estricto)
	if m.confirmOpen {
		return m.handleConfirmKey(key)
	}

	// Project modal takes priority
	if m.projectModalOpen {
		return m.handleProjectModalKey(key)
	}

	// New task modal takes priority
	if m.newTaskOpen {
		return m.handleNewTaskKey(key)
	}

	// Detail modal keys
	if m.detailOpen {
		return m.handleDetailKey(key)
	}

	// Filter modal keys
	if m.filterOpen {
		return m.handleFilterModalKey(key)
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
	}

	return m, nil
}

func (m Model) handleDashboardKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab", "j", "down":
		// Cycle through visible projects
		if n := len(m.dashProjectList()); n > 0 {
			m.dashProjectIdx = (m.dashProjectIdx + 1) % n
		}
	case "k", "up":
		if n := len(m.dashProjectList()); n > 0 {
			m.dashProjectIdx = (m.dashProjectIdx - 1 + n) % n
		}
	case "i":
		// Nuevo proyecto: el Dashboard concentra la config global
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
	}
	return m, nil
}

func (m Model) handleListKey(key string) (tea.Model, tea.Cmd) {
	m.clampListCursor()
	tasks := m.filteredTasks()

	switch key {
	case "j", "down":
		if _, end := m.pageBounds(); m.cursor < end-1 {
			m.cursor++
		}
	case "k", "up":
		if start, _ := m.pageBounds(); m.cursor > start {
			m.cursor--
		}
	case "n":
		// Página siguiente: saltar al primer elemento de la próxima página.
		start := m.currentPage() * m.listPageSize()
		if next := start + m.listPageSize(); next < len(tasks) {
			m.cursor = next
		}
	case "p":
		// Página anterior: saltar al primer elemento de la página previa.
		if start := m.currentPage() * m.listPageSize(); start > 0 {
			m.cursor = start - m.listPageSize()
		}
	case "N":
		// Ir a la última página: cursor al último elemento.
		if len(tasks) > 0 {
			m.cursor = len(tasks) - 1
		}
	case "P":
		// Ir a la primera página: cursor al primer elemento.
		m.cursor = 0
	case "e":
		// Edit task in nvim
		return m, m.editSelectedTask()
	case "i":
		// New task
		return m, m.newTask()
	case "enter":
		if len(tasks) > 0 && m.cursor < len(tasks) {
			t := tasks[m.cursor]
			m.detailOpen = true
			m.detailTask = &t
			m.detailComments = nil
			m.detailCommentSel = -1
			return m, m.loadCommentsCmd(t.ID)
		}
	case "s":
		// Start: move to next status
		if m.cursor < len(tasks) {
			return m, m.taskActionCmd(tasks[m.cursor].ID, m.database.StartTask)
		}
	case "d":
		// Done
		if m.cursor < len(tasks) {
			return m, m.taskActionCmd(tasks[m.cursor].ID, m.database.DoneTask)
		}
	case "x":
		// Cancel
		if m.cursor < len(tasks) {
			return m, m.taskActionCmd(tasks[m.cursor].ID, m.database.CancelTask)
		}
	case "/":
		// Open filter modal
		m.filterOpen = true
		m.filterFieldIdx = 0
	case "ctrl+p":
		if m.cursor < len(tasks) {
			t := tasks[m.cursor]
			var next int
			if t.Status == "backlog" {
				// backlog: none→LOW→MED→HIGH→none
				next = (t.Priority + 1) % 4
			} else {
				// fuera de backlog: LOW→MED→HIGH→LOW
				next = (t.Priority % 3) + 1
			}
			return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
				return m.database.UpdateTask(id, map[string]any{"priority": next})
			})
		}
	}

	return m, nil
}

func (m Model) handleKanbanKey(key string) (tea.Model, tea.Cmd) {
	m.clampKanbanCursor()

	cols := m.kanbanColumns()
	if len(cols) == 0 {
		return m, nil
	}
	colTasks := cols[m.kanbanCol].tasks

	switch key {
	case "tab":
		// Cycle through columns
		m.kanbanCol = (m.kanbanCol + 1) % len(cols)
		m.kanbanRow = 0
	case "h", "left":
		if m.kanbanCol > 0 {
			m.kanbanCol--
			m.kanbanRow = 0
		}
	case "l", "right":
		if m.kanbanCol < len(cols)-1 {
			m.kanbanCol++
			m.kanbanRow = 0
		}
	case "j", "down":
		if len(colTasks) > 0 {
			m.kanbanRow = (m.kanbanRow + 1) % len(colTasks)
		}
	case "k", "up":
		if len(colTasks) > 0 {
			m.kanbanRow = (m.kanbanRow - 1 + len(colTasks)) % len(colTasks)
		}
	case "s":
		// Move task right (advance status)
		if m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			if nextStatus, ok := model.NextStatus(m.mergedWorkflow(), t.Status); ok {
				return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
					return m.database.MoveTask(id, nextStatus)
				})
			}
		}
	case "S":
		// Move task left (retreat status)
		if m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			if prevStatus, ok := model.PrevStatus(m.mergedWorkflow(), t.Status); ok {
				return m, m.taskActionCmd(t.ID, func(id int64) (*model.Task, error) {
					return m.database.MoveTask(id, prevStatus)
				})
			}
		}
	case "d":
		// Done
		if m.kanbanRow < len(colTasks) {
			return m, m.taskActionCmd(colTasks[m.kanbanRow].ID, m.database.DoneTask)
		}
	case "x":
		// Cancel
		if m.kanbanRow < len(colTasks) {
			return m, m.taskActionCmd(colTasks[m.kanbanRow].ID, m.database.CancelTask)
		}
	case "e":
		// Edit task in nvim
		return m, m.editSelectedTask()
	case "i":
		// New task
		return m, m.newTask()
	case "enter":
		if m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			m.detailOpen = true
			m.detailTask = &t
			m.detailComments = nil
			m.detailCommentSel = -1
			return m, m.loadCommentsCmd(t.ID)
		}
	case "ctrl+p":
		if m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			var next int
			if t.Status == "backlog" {
				// backlog: none→LOW→MED→HIGH→none
				next = (t.Priority + 1) % 4
			} else {
				// fuera de backlog: LOW→MED→HIGH→LOW
				next = (t.Priority % 3) + 1
			}
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
		if t.Status == status {
			// Ocultar done/cancelled por defecto en kanban
			if m.filterActiveOnly && (t.Status == "done" || t.Status == "cancelled") {
				continue
			}
			result = append(result, t)
		}
	}
	return result
}

func (m Model) handleDetailKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Primero deselecciona el comentario; si no hay, cierra el modal.
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
		if m.detailCommentSel < 0 {
			m.detailCommentSel = 0
		} else if m.detailCommentSel < len(m.detailComments)-1 {
			m.detailCommentSel++
		}
		return m, nil
	case "k", "up":
		if m.detailCommentSel > 0 {
			m.detailCommentSel--
		} else if m.detailCommentSel == 0 {
			m.detailCommentSel = -1
		}
		return m, nil
	case "c":
		// Nuevo comentario en el editor externo.
		if m.detailTask != nil {
			editorCmd := m.config.Editor.Command
			if editorCmd == "" {
				editorCmd = "nvim"
			}
			return m, commentCmd(m.detailTask.ID, editorCmd)
		}
	case "e":
		// Edit task in editor
		if m.detailTask != nil {
			editorCmd := m.config.Editor.Command
			if editorCmd == "" {
				editorCmd = "nvim"
			}
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
		// Con un comentario seleccionado, borra el comentario.
		if m.detailCommentSel >= 0 && m.detailCommentSel < len(m.detailComments) {
			comment := m.detailComments[m.detailCommentSel]
			return m, m.deleteCommentCmd(m.detailTask.ID, comment.ID, m.detailCommentSel)
		}
		// Sin selección, marca la tarea como done.
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
	result := []string{"Me"} // siempre presente, es el usuario
	seen["Me"] = true
	for _, t := range m.tasks {
		if !seen[t.Assignee] && t.Assignee != "" {
			seen[t.Assignee] = true
			result = append(result, t.Assignee)
		}
	}
	return result
}

// selectedTask devuelve la tarea seleccionada en la vista actual, o nil si no hay.
func (m *Model) selectedTask() *model.Task {
	switch m.currentView {
	case viewList:
		tasks := m.filteredTasks()
		if m.cursor >= 0 && m.cursor < len(tasks) {
			return &tasks[m.cursor]
		}
	case viewKanban:
		cols := m.kanbanColumns()
		if m.kanbanCol < 0 || m.kanbanCol >= len(cols) {
			break
		}
		tasks := cols[m.kanbanCol].tasks
		if m.kanbanRow >= 0 && m.kanbanRow < len(tasks) {
			return &tasks[m.kanbanRow]
		}
	}
	return nil
}

// previewBudget calcula cuántas líneas de descripción puede mostrar el preview
// sin empujar el contenido ni los keybinds fuera de la pantalla.
func (m Model) previewBudget(keybindsHeight int) int {
	budget := m.height - keybindsHeight - minContentHeight - 2 // 2 = bordes de la caja
	if budget < 1 {
		budget = 1
	}
	if budget > previewMaxLines {
		budget = previewMaxLines
	}
	return budget
}

// overlayKind devuelve el modal activo, en el mismo orden de prioridad que
// handleKey: confirmación, proyecto, nueva tarea, detalle, filtros.
func (m Model) overlayKind() overlayKind {
	switch {
	case m.confirmOpen:
		return overlayConfirm
	case m.projectModalOpen:
		return overlayProject
	case m.newTaskOpen:
		return overlayNewTask
	case m.detailOpen:
		return overlayDetail
	case m.filterOpen:
		return overlayFilter
	}
	return overlayNone
}

func (m Model) View() tea.View {
	// KeybindsBar siempre al fondo.
	m.statusbar.SetView(m.currentView)
	m.statusbar.SetOverlay(m.overlayKind())
	keybinds := m.statusbar.View()
	keybindsHeight := lineCount(keybinds)

	// El preview se ajusta al alto sobrante para no empujar los keybinds.
	m.preview.SetTask(m.selectedTask())
	m.preview.SetMaxLines(m.previewBudget(keybindsHeight))
	preview := m.preview.View()

	// El detalle ya muestra la descripción: sin preview duplicado.
	if m.detailOpen {
		preview = ""
	}

	// Toast de feedback: ocupa una línea por encima del preview.
	toastView := m.renderToast()

	budget := contentBudget(m.height, lineCount(preview)+lineCount(toastView), keybindsHeight)

	var content string
	switch m.currentView {
	case viewDashboard:
		content = m.renderDashboard(budget)
	case viewKanban:
		content = m.renderKanban(budget)
	case viewList:
		content = m.renderList(budget)
	default:
		content = m.renderDashboard(budget)
	}

	if m.detailOpen && m.detailTask != nil {
		content = m.renderDetail(m.detailTask, budget)
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
