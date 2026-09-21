package tui

import (
	"fmt"
	"sort"

	"charm.land/bubbles/v2/textarea"
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
	filterProject  string
	filterStatus   string // "" = all; statusFilterAllActive = activos; otro = estado exacto
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
	pendingSelectName string // proyecto a seleccionar tras recargar

	// Project modal (crear/editar proyecto)
	projectModalOpen      bool
	projectModalEdit      bool
	projectModalField     int // 0=name, 1=workflow, 2=list_order
	projectNameInput      string
	projectWorkflowInput  string
	projectListOrderInput string
	projectEditingName    string // nombre original al editar

	// Confirmación (archivar/restaurar proyecto, borrar off-day)
	confirmOpen    bool
	confirmAction  string // "archive" | "unarchive" | "delete-offday"
	confirmProject string
	confirmOffday  model.OffDay

	// Assignee / off-day modal (se abre con "m" desde el Dashboard)
	assigneeModalOpen bool
	assigneeDetail    bool // false = lista de personas, true = detalle
	assigneeIdx       int
	assigneeOffdayIdx int

	// Alta de off-day
	offdayFormOpen   bool
	offdayFormField  int // 0=start, 1=end, 2=note
	offdayStartInput string
	offdayEndInput   string
	offdayNoteInput  string

	// Toast de feedback transitorio
	toast     string
	toastKind string // "info" | "error"
	toastSeq  int

	// Detail modal
	detailOpen       bool
	detailTask       *model.Task
	detailComments   []model.Comment
	detailCommentSel int // -1 = ninguno seleccionado

	// Tag modal (se abre con "t" desde el Detail)
	tagOpen       bool
	tagInput      string
	tagSuggestIdx int // -1 = ninguna sugerencia seleccionada

	// Editor inline de descripción (integrado en la caja del detalle)
	descEditOpen      bool
	descEditTaskID    int64
	descEditHadDetail bool // el detalle ya estaba abierto antes de editar
	descEditTextarea  textarea.Model

	// Help modal
	helpOpen bool

	// Filter modal
	filterOpen      bool
	filterFieldIdx  int    // 0=project, 1=status, 2=assignee, 3=priority, 4=tag
	filterSearch    string // búsqueda fuzzy del campo activo
	filterOptionIdx int    // cursor de opciones del campo activo

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
	newTaskErr             string // validación inline (ej. título requerido)
	newTaskTextarea        textarea.Model

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

// ---- Mensajes ----

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

// taskCreateFailedMsg resulta de fallar la creación de una tarea desde el modal.
type taskCreateFailedMsg struct{ err error }

// offdaySavedMsg resulta de un alta o baja de off-day. err != nil indica fallo.
type offdaySavedMsg struct {
	err    error
	action string // "add" | "delete"
	name   string
}

// tagToggledMsg resulta de alternar una tag: trae la tarea actualizada (para el
// Detail) y el listado recargado (para List/Kanban).
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

// loadOffDays trae todos los off-days para la proyección del Gantt.
func (m *Model) loadOffDays() tea.Cmd {
	return func() tea.Msg {
		offdays, err := m.database.ListOffDays("")
		if err != nil {
			return offdaysLoadedMsg{}
		}
		return offdaysLoadedMsg{offdays: offdays}
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

// toggleTagCmd agrega la tag si la tarea no la tiene, o la quita si la tiene.
// Recarga el listado para que List/Kanban reflejen el cambio.
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

// kanbanWorkflow devuelve las columnas del Kanban: el workflow del proyecto en
// contexto si hay uno seleccionado, o la unión de todos en "all projects".
func (m *Model) kanbanWorkflow() []string {
	if m.filterProject != "" {
		if p := m.projectByName(m.filterProject); p != nil {
			return p.Workflow
		}
	}
	return m.mergedWorkflow()
}

// projectByName busca un proyecto activo por nombre.
func (m *Model) projectByName(name string) *model.Project {
	for i := range m.projects {
		if m.projects[i].Name == name {
			return &m.projects[i]
		}
	}
	return nil
}

// commonWorkflow devuelve los estados presentes en TODOS los proyectos, en el
// orden del primero. Es el conjunto válido para filtrar por estado cuando no
// hay proyecto seleccionado: un estado que no exista en todos no puede filtrar
// tareas de todos. Sin proyectos cae al workflow por defecto.
func (m *Model) commonWorkflow() []string {
	if len(m.projects) == 0 {
		return model.DefaultWorkflow
	}
	var result []string
	for _, s := range m.projects[0].Workflow {
		common := true
		for i := 1; i < len(m.projects); i++ {
			if !model.HasStatus(m.projects[i].Workflow, s) {
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

// taskMatchesFilter indica si una tarea pasa los filtros activos. El estado se
// resuelve en un único lugar: statusFilterAllActive (default) deja fuera los
// terminales, "" (all) no restringe, y cualquier otro valor exige coincidencia
// exacta. Lo comparten List, Kanban y Gantt para que la cabecera de filtros sea
// consistente en todas las vistas.
func (m *Model) taskMatchesFilter(t model.Task) bool {
	switch m.filterStatus {
	case statusFilterAllActive:
		if !t.IsActive() {
			return false
		}
	case "":
		// "all": sin restricción de estado
	default:
		if t.Status != m.filterStatus {
			return false
		}
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
	if m.filterPriority >= 0 && t.Priority != m.filterPriority {
		return false
	}
	return true
}

// filteredTasks devuelve las tareas filtradas.
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
		// Los textarea del editor inline y del alta de tarea manejan el pegado.
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
		// El textarea del editor inline emite mensajes privados del paquete
		// (pasteMsg/copyMsg) como resultado de sus comandos asíncronos. No son
		// tea.KeyMsg ni tea.PasteMsg, así que hay que reenviarlos a su Update.
		if m.descEditOpen {
			return m.handleDescEditKey(msg)
		}
		if m.newTaskOpen && m.newTaskFieldIdx == newTaskFieldDescription {
			var cmd tea.Cmd
			m.newTaskTextarea, cmd = m.newTaskTextarea.Update(msg)
			return m, cmd
		}
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
		return m.handleNewTaskKey(msg)
	}

	// Editor inline de descripción (puede superponerse al detalle)
	if m.descEditOpen {
		return m.handleDescEditKey(msg)
	}

	// Tag modal (se superpone al detalle)
	if m.tagOpen {
		return m.handleTagModalKey(key)
	}

	// Detail modal keys
	if m.detailOpen {
		return m.handleDetailKey(key)
	}

	// Filter modal keys
	if m.filterOpen {
		return m.handleFilterModalKey(key)
	}

	// Assignee / off-day modal (formulario encima de la lista)
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
	case "m":
		// Gestionar off-days por persona.
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
		// Cambiar de proyecto ciclando el filtro Project, como en el Dashboard.
		m.cycleProjectFilter(1)
		m.cursor = 0
		return m, nil
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
		// Editar la descripción inline, desde la propia TUI.
		return m, m.openDescEditor()
	case "E":
		// Editor externo completo (write in nvim).
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
		return m, m.openFilterModal()
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

	// Abrir filtros antes del corte por columnas vacías, para que funcione
	// aunque el board no tenga nada.
	if key == "/" {
		return m, m.openFilterModal()
	}

	// Tab cambia de proyecto ciclando el filtro Project (como el Dashboard).
	// Las columnas se navegan con h/l o ←/→, que ya hacían lo mismo.
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
		// Move task right (advance status) según el workflow de SU proyecto:
		// el orden del merge puede no existir en el proyecto y el move sería
		// rechazado en silencio por MoveTask.
		if m.kanbanRow < len(colTasks) {
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
		// Move task left (retreat status) según el workflow de su proyecto.
		if m.kanbanRow < len(colTasks) {
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
		if m.kanbanRow < len(colTasks) {
			return m, m.taskActionCmd(colTasks[m.kanbanRow].ID, m.database.DoneTask)
		}
	case "x":
		// Cancel
		if m.kanbanRow < len(colTasks) {
			return m, m.taskActionCmd(colTasks[m.kanbanRow].ID, m.database.CancelTask)
		}
	case "e":
		// Editar la descripción inline, desde la propia TUI.
		return m, m.openDescEditor()
	case "E":
		// Editor externo completo (write in nvim).
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
	case "t":
		// Abrir el modal de tags de la tarea abierta.
		if m.detailTask != nil {
			m.tagOpen = true
			m.tagInput = ""
			m.tagSuggestIdx = -1
		}
	case "e":
		// Editar la descripción inline; el detalle queda abierto detrás.
		if m.detailTask != nil {
			return m, m.openDescEditor()
		}
	case "E":
		// Editor externo completo (write in nvim).
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

// uniqueTags devuelve las tags en uso en todas las tareas, sin repetir y
// ordenadas alfabéticamente. Es la fuente de opciones del filtro por tag.
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
	case viewGantt:
		rows := m.ganttRows()
		if m.ganttCursor >= 0 && m.ganttCursor < len(rows) && rows[m.ganttCursor].kind == ganttTaskRow {
			return &rows[m.ganttCursor].entry.Task
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
// handleKey: confirmación, proyecto, nueva tarea, editor de descripción, tags,
// detalle, filtros.
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
	// KeybindsBar siempre al fondo.
	m.statusbar.SetView(m.currentView)
	m.statusbar.SetOverlay(m.overlayKind())
	keybinds := m.statusbar.View()
	keybindsHeight := lineCount(keybinds)

	// El preview se ajusta al alto sobrante para no empujar los keybinds.
	m.preview.SetTask(m.selectedTask())
	m.preview.SetMaxLines(m.previewBudget(keybindsHeight))
	preview := m.preview.View()

	// El detalle y el editor inline ya muestran la descripción: sin preview.
	if m.detailOpen || m.descEditOpen {
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
