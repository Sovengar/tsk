package tui

import (
	tea "charm.land/bubbletea/v2"
	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// Model es el modelo principal de la TUI.
type Model struct {
	width, height int

	database   *db.DB
	config     config.Config
	projects   []model.Project
	tasks      []model.Task
	filteredT  []model.Task // cached filtered tasks

	currentView viewKind
	cursor      int

	// Filters
	filterProject   string
	filterStatus    string
	filterAssignee  string
	filterPriority  int // -1 = all
	filterActive    bool
	filterText      string
	filterActiveOnly bool // ocultar done/cancelled por defecto

	// Kanban state
	kanbanCol int // column index in kanban
	kanbanRow int // row index within column

	// Dashboard state
	dashProjectIdx int // selected project index in dashboard

	// Detail modal
	detailOpen bool
	detailTask *model.Task

	// Help modal
	helpOpen bool

	// Filter modal
	filterOpen    bool
	filterFieldIdx int // 0=project, 1=status, 2=assignee, 3=priority

	// New task modal
	newTaskOpen             bool
	newTaskTitle            string
	newTaskPriority         int
	newTaskAssignee         string
	newTaskAssigneeSuggIdx  int // -1 = none selected
	newTaskProject          string
	newTaskFieldIdx         int // 0=title, 1=priority, 2=assignee

	// StatusBar
	statusbar StatusBar
}

// New construye el modelo con la base de datos.
func New(database *db.DB, cfg config.Config) Model {
	return Model{
		database:         database,
		config:           cfg,
		currentView:      viewList,
		filterPriority:   -1,
		filterActiveOnly: true,
		width:            80,
		height:           24,
		statusbar:        NewStatusBar(80),
	}
}

// ---- Mensajes ----

type tasksLoadedMsg struct {
	tasks []model.Task
}

type projectsLoadedMsg struct {
	projects []model.Project
}

// ---- Commands ----

func (m *Model) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.database.ListProjects()
		if err != nil {
			return projectsLoadedMsg{}
		}
		return projectsLoadedMsg{projects: projects}
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
		return m, nil

	case projectsLoadedMsg:
		m.projects = msg.projects
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

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

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
	case "tab":
		// Cycle through projects
		if len(m.projects) > 0 {
			m.dashProjectIdx = (m.dashProjectIdx + 1) % len(m.projects)
		}
	case "j", "down":
		if len(m.projects) > 0 {
			m.dashProjectIdx = (m.dashProjectIdx + 1) % len(m.projects)
		}
	case "k", "up":
		if len(m.projects) > 0 {
			m.dashProjectIdx = (m.dashProjectIdx - 1 + len(m.projects)) % len(m.projects)
		}
	case "n":
		return m, m.newTask()
	}
	return m, nil
}

func (m Model) handleListKey(key string) (tea.Model, tea.Cmd) {
	tasks := m.filteredTasks()

	switch key {
	case "tab":
		// Cycle to next view
		m.currentView = m.currentView.next()
		return m, m.loadTasks()
	case "j", "down":
		if len(tasks) > 0 {
			m.cursor = (m.cursor + 1) % len(tasks)
		}
	case "k", "up":
		if len(tasks) > 0 {
			m.cursor = (m.cursor - 1 + len(tasks)) % len(tasks)
		}
	case "e":
		// Edit task in nvim
		return m, m.editSelectedTask()
	case "n":
		// New task
		return m, m.newTask()
	case "enter":
		if len(tasks) > 0 && m.cursor < len(tasks) {
			t := tasks[m.cursor]
			m.detailOpen = true
			m.detailTask = &t
		}
	case "s":
		// Start: move to next status
		if len(tasks) > 0 && m.cursor < len(tasks) {
			t := tasks[m.cursor]
			return m, func() tea.Msg {
				_, err := m.database.StartTask(t.ID)
				if err != nil {
					return nil
				}
				tasks, _ := m.database.ListTasks("", "", "")
				return tasksLoadedMsg{tasks: tasks}
			}
		}
	case "d":
		// Done
		if len(tasks) > 0 && m.cursor < len(tasks) {
			t := tasks[m.cursor]
			return m, func() tea.Msg {
				_, err := m.database.DoneTask(t.ID)
				if err != nil {
					return nil
				}
				tasks, _ := m.database.ListTasks("", "", "")
				return tasksLoadedMsg{tasks: tasks}
			}
		}
	case "x":
		// Cancel
		if len(tasks) > 0 && m.cursor < len(tasks) {
			t := tasks[m.cursor]
			return m, func() tea.Msg {
				_, err := m.database.CancelTask(t.ID)
				if err != nil {
					return nil
				}
				tasks, _ := m.database.ListTasks("", "", "")
				return tasksLoadedMsg{tasks: tasks}
			}
		}
	case "/":
		// Open filter modal
		m.filterOpen = true
		m.filterFieldIdx = 0
	}

	return m, nil
}

func (m Model) handleKanbanKey(key string) (tea.Model, tea.Cmd) {
	workflow := m.mergedWorkflow()

	switch key {
	case "tab":
		// Cycle through columns
		m.kanbanCol = (m.kanbanCol + 1) % len(workflow)
		m.kanbanRow = 0
	case "h", "left":
		if m.kanbanCol > 0 {
			m.kanbanCol--
			m.kanbanRow = 0
		}
	case "l", "right":
		if m.kanbanCol < len(workflow)-1 {
			m.kanbanCol++
			m.kanbanRow = 0
		}
	case "j", "down":
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 {
			m.kanbanRow = (m.kanbanRow + 1) % len(colTasks)
		}
	case "k", "up":
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 {
			m.kanbanRow = (m.kanbanRow - 1 + len(colTasks)) % len(colTasks)
		}
	case "s":
		// Move task right (advance status)
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			nextStatus, ok := model.NextStatus(workflow, t.Status)
			if ok {
				return m, func() tea.Msg {
					_, err := m.database.MoveTask(t.ID, nextStatus)
					if err != nil {
						return nil
					}
					tasks, _ := m.database.ListTasks("", "", "")
					return tasksLoadedMsg{tasks: tasks}
				}
			}
		}
	case "S":
		// Move task left (retreat status)
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			prevStatus, ok := model.PrevStatus(workflow, t.Status)
			if ok {
				return m, func() tea.Msg {
					_, err := m.database.MoveTask(t.ID, prevStatus)
					if err != nil {
						return nil
					}
					tasks, _ := m.database.ListTasks("", "", "")
					return tasksLoadedMsg{tasks: tasks}
				}
			}
		}
	case "d":
		// Done
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			return m, tea.Batch(
				func() tea.Msg {
					_, err := m.database.DoneTask(t.ID)
					if err != nil {
						return nil
					}
					return m.loadTasks()
				},
			)
		}
	case "x":
		// Cancel
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			return m, tea.Batch(
				func() tea.Msg {
					_, err := m.database.CancelTask(t.ID)
					if err != nil {
						return nil
					}
					return m.loadTasks()
				},
			)
		}
	case "e":
		// Edit task in nvim
		return m, m.editSelectedTask()
	case "n":
		// New task
		return m, m.newTask()
	case "enter":
		colTasks := m.tasksInColumn(workflow[m.kanbanCol])
		if len(colTasks) > 0 && m.kanbanRow < len(colTasks) {
			t := colTasks[m.kanbanRow]
			m.detailOpen = true
			m.detailTask = &t
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
		m.detailOpen = false
		m.detailTask = nil
		return m, nil
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
			return m, func() tea.Msg {
				_, err := m.database.StartTask(id)
				if err != nil {
					return nil
				}
				tasks, _ := m.database.ListTasks("", "", "")
				return tasksLoadedMsg{tasks: tasks}
			}
		}
	case "d":
		if m.detailTask != nil {
			id := m.detailTask.ID
			m.detailOpen = false
			m.detailTask = nil
			return m, func() tea.Msg {
				_, err := m.database.DoneTask(id)
				if err != nil {
					return nil
				}
				tasks, _ := m.database.ListTasks("", "", "")
				return tasksLoadedMsg{tasks: tasks}
			}
		}
	case "x":
		if m.detailTask != nil {
			id := m.detailTask.ID
			m.detailOpen = false
			m.detailTask = nil
			return m, func() tea.Msg {
				_, err := m.database.CancelTask(id)
				if err != nil {
					return nil
				}
				tasks, _ := m.database.ListTasks("", "", "")
				return tasksLoadedMsg{tasks: tasks}
			}
		}
	}
	return m, nil
}

func (m Model) uniqueAssignees() []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range m.tasks {
		if !seen[t.Assignee] {
			seen[t.Assignee] = true
			result = append(result, t.Assignee)
		}
	}
	return result
}

func (m Model) View() tea.View {
	var content string
	switch m.currentView {
	case viewDashboard:
		content = m.renderDashboard()
	case viewList:
		content = m.renderList()
	case viewKanban:
		content = m.renderKanban()
	default:
		content = m.renderDashboard()
	}

	if m.detailOpen && m.detailTask != nil {
		content = m.renderDetail(m.detailTask)
	}

	if m.filterOpen {
		content = m.renderFilterModal(content)
	}

	if m.newTaskOpen {
		content = m.renderNewTaskModal(content)
	}

	if m.helpOpen {
		content = m.renderHelpModal(content)
	}

	// StatusBar always at the bottom
	m.statusbar.SetView(m.currentView)
	statusBar := m.statusbar.View()

	v := tea.NewView(content + "\n" + statusBar)
	v.AltScreen = true
	return v
}
