package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// pasteMsg es el mensaje de pegado de bubbletea, que es lo que llega al handler.
func pasteMsg(content string) tea.PasteMsg { return tea.PasteMsg{Content: content} }

// ---- selección de tarea editable: fuente única para el editor externo y el
// inline. Sin esto, un mutant de un límite de índice pasa desapercibido. ----

func TestSelectedEditableTaskByView(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		want  string // título esperado, "" = nil
	}{
		{
			name:  "list: la tarea bajo el cursor",
			setup: func(m *Model) { m.currentView = viewList; m.cursor = 0 },
			want:  "Fix checkout",
		},
		{
			name:  "list: cursor fuera de rango",
			setup: func(m *Model) { m.currentView = viewList; m.cursor = 999 },
			want:  "",
		},
		{
			name:  "list: sin tareas",
			setup: func(m *Model) { m.currentView = viewList; m.tasks = nil; m.cursor = 0 },
			want:  "",
		},
		{
			name:  "kanban: tarea de la columna y fila actuales",
			setup: func(m *Model) { m.currentView = viewKanban; m.kanbanCol = 0; m.kanbanRow = 0 },
			want:  "Add caching",
		},
		{
			name:  "kanban: columna fuera de rango",
			setup: func(m *Model) { m.currentView = viewKanban; m.kanbanCol = 99; m.kanbanRow = 0 },
			want:  "",
		},
		{
			name:  "kanban: fila fuera de rango",
			setup: func(m *Model) { m.currentView = viewKanban; m.kanbanCol = 0; m.kanbanRow = 99 },
			want:  "",
		},
		{
			name: "dashboard: sólo con el detalle abierto",
			setup: func(m *Model) {
				m.currentView = viewDashboard
				m.detailOpen = true
				m.detailTask = &model.Task{Title: "en detalle"}
			},
			want: "en detalle",
		},
		{
			name: "dashboard: detalle cerrado",
			setup: func(m *Model) {
				m.currentView = viewDashboard
				m.detailOpen = false
				m.detailTask = &model.Task{Title: "en detalle"}
			},
			want: "",
		},
		{
			name:  "dashboard: sin tarea en detalle",
			setup: func(m *Model) { m.currentView = viewDashboard; m.detailOpen = true; m.detailTask = nil },
			want:  "",
		},
		{
			name:  "vista sin tarea editable propia (gantt)",
			setup: func(m *Model) { m.currentView = viewGantt },
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			tt.setup(m)

			got := m.selectedEditableTask()
			if tt.want == "" {
				if got != nil {
					t.Fatalf("selectedEditableTask() = %q, want nil", got.Title)
				}
				return
			}
			if got == nil {
				t.Fatalf("selectedEditableTask() = nil, want %q", tt.want)
			}
			if got.Title != tt.want {
				t.Errorf("selectedEditableTask() = %q, want %q", got.Title, tt.want)
			}
		})
	}
}

// ---- paste en los campos de texto: los saltos de línea se aplanan. ----

func TestHandleNewTaskPasteFlattensNewlines(t *testing.T) {
	// Cada campo tiene su valor inicial y su indice de sugerencia: pegar sólo
	// toca el campo activo, y suelta la sugerencia de ESE campo.
	tests := []struct {
		name          string
		field         int
		initial       string
		content       string
		want          string
		suggReset     bool
		otherSuggHold bool
	}{
		{"titulo", newTaskFieldTitle, "pre-", "uno\ndos\rtres", "pre-uno dos tres", true, true},
		{"assignee", newTaskFieldAssignee, "@pre ", "uno\ndos", "@pre uno dos", true, true},
		{"tags", newTaskFieldTags, "pre ", "uno\ndos", "pre uno dos", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.newTaskOpen = true
			m.newTaskFieldIdx = tt.field
			m.newTaskTitle = ""
			m.newTaskAssignee = ""
			m.newTaskTagInput = ""
			_ = m
			m.newTaskAssigneeSuggIdx = 3
			m.newTaskTagSuggIdx = 3
			m.newTaskErr = "error anterior"

			set := func(mm *Model, s string) {
				switch tt.field {
				case newTaskFieldTitle:
					mm.newTaskTitle = s
				case newTaskFieldAssignee:
					mm.newTaskAssignee = s
				case newTaskFieldTags:
					mm.newTaskTagInput = s
				}
			}
			set(m, tt.initial)

			got, _ := m.handleNewTaskPaste(pasteMsg(tt.content))
			after := got.(Model)

			var value string
			switch tt.field {
			case newTaskFieldTitle:
				value = after.newTaskTitle
				if tt.suggReset && after.newTaskAssigneeSuggIdx == -1 {
					t.Error("la sugerencia de assignee no debería resetearse al pegar en el título")
				}
			case newTaskFieldAssignee:
				value = after.newTaskAssignee
			case newTaskFieldTags:
				value = after.newTaskTagInput
			}
			if value != tt.want {
				t.Errorf("campo = %q, want %q", value, tt.want)
			}

			// La sugerencia del campo pegado se suelta; las otras no.
			activeSugg := -1
			switch tt.field {
			case newTaskFieldAssignee:
				activeSugg = after.newTaskAssigneeSuggIdx
			case newTaskFieldTags:
				activeSugg = after.newTaskTagSuggIdx
			}
			if tt.suggReset && tt.field != newTaskFieldTitle && activeSugg != -1 {
				t.Errorf("la sugerencia del campo pegado debería quedar en -1, got %d", activeSugg)
			}
		})
	}
}

// El campo descripción es el único que delega en el textarea: el texto llega
// allí con sus saltos de línea intactos, no a newTaskTitle.
func TestHandleNewTaskPasteGoesToTextareaOnDescription(t *testing.T) {
	m := newTestModel(t)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldDescription
	m.newTaskTitle = "intacto"
	m.newTaskErr = "error anterior"

	got, _ := m.handleNewTaskPaste(pasteMsg("con\nsaltos"))
	after := got.(Model)

	if after.newTaskTitle != "intacto" {
		t.Errorf("newTaskTitle = %q, want intacto: la descripción va al textarea", after.newTaskTitle)
	}
	if after.newTaskErr != "error anterior" {
		t.Errorf("newTaskErr = %q: el paste en descripción no valida el campo", after.newTaskErr)
	}
}

// ---- paste de tags: mismo aplanado, y siempre resetea la sugerencia. ----

func TestHandleTagPaste(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		content string
		want    string
	}{
		{"simple", "", "bug", "bug"},
		{"aplana saltos", "a ", "b\nc\rd", "a b c d"},
		{"vacío", "a", "", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.tagOpen = true
			m.tagInput = tt.initial
			m.tagSuggestIdx = 2

			got, _ := m.handleTagPaste(tt.content)
			after := got.(Model)

			if after.tagInput != tt.want {
				t.Errorf("tagInput = %q, want %q", after.tagInput, tt.want)
			}
			if after.tagSuggestIdx != -1 {
				t.Errorf("tagSuggestIdx = %d, want -1", after.tagSuggestIdx)
			}
		})
	}
}

// ---- selección de proyecto: decide si la vista pasa a archivados. ----

func TestSelectProjectByNameSwitchesArchivedView(t *testing.T) {
	tests := []struct {
		name          string
		archived      []model.Project
		target        string
		wantIdx       int
		wantShowArch  bool
		wantUnchanged bool
	}{
		{"activo", nil, "api", 0, false, false},
		{"activo más abajo", nil, "web", 1, false, false},
		{"archivado", []model.Project{{Name: "viejo"}}, "viejo", 0, true, false},
		{"inexistente", nil, "nope", 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.archivedProjects = tt.archived
			m.dashProjectIdx = -1
			m.showArchived = !tt.wantShowArch

			m.selectProjectByName(tt.target)

			if tt.wantUnchanged {
				if m.dashProjectIdx != -1 || m.showArchived == tt.wantShowArch {
					t.Errorf("un nombre inexistente no debe tocar nada: idx=%d showArchived=%v", m.dashProjectIdx, m.showArchived)
				}
				return
			}
			if m.dashProjectIdx != tt.wantIdx {
				t.Errorf("dashProjectIdx = %d, want %d", m.dashProjectIdx, tt.wantIdx)
			}
			if m.showArchived != tt.wantShowArch {
				t.Errorf("showArchived = %v, want %v", m.showArchived, tt.wantShowArch)
			}
		})
	}
}

// Un proyecto aparece en las dos listas: manda la de activos, porque es donde
// vive y el índice corresponde a esa lista.
func TestSelectProjectByNamePrefersActiveList(t *testing.T) {
	m := newTestModel(t)
	m.archivedProjects = []model.Project{{Name: "api"}}
	m.dashProjectIdx = -1
	m.showArchived = true

	m.selectProjectByName("api")

	if m.showArchived {
		t.Error("showArchived = true, want false (gana la lista de activos)")
	}
	if m.dashProjectIdx != 0 {
		t.Errorf("dashProjectIdx = %d, want 0 (índice en la lista de activos)", m.dashProjectIdx)
	}
}

// ---- modal de ayuda: es la documentación de las keybinds dentro de la app. ----

func TestRenderHelpModalListsEveryKeybindSource(t *testing.T) {
	views := []viewKind{viewList, viewKanban, viewGantt, viewDashboard}
	for _, v := range views {
		t.Run(v.String(), func(t *testing.T) {
			m := newTestModel(t)
			m.currentView = v

			out := m.renderHelpModal("")
			plain := ansi.Strip(out)

			// Las cuatro secciones del modal.
			for _, section := range []string{
				"Task detail (modal)",
				"New task (modal)",
				"Filters (modal)",
			} {
				if !strings.Contains(plain, section) {
					t.Errorf("falta la sección %q", section)
				}
			}
			// Y al menos una key de cada fuente de verdad. Si una lista de
			// keybinds se vacía o se deja de renderizar, esto salta.
			for _, kb := range detailKeybinds() {
				if !strings.Contains(plain, kb.key) {
					t.Errorf("falta la key del detalle %q", kb.key)
				}
			}
			for _, kb := range newTaskKeybinds() {
				if !strings.Contains(plain, kb.key) {
					t.Errorf("falta la key del modal de nueva tarea %q", kb.key)
				}
			}
			for _, kb := range filterKeybinds() {
				if !strings.Contains(plain, kb.key) {
					t.Errorf("falta la key del modal de filtros %q", kb.key)
				}
			}
			if !strings.Contains(plain, "to close") {
				t.Error("falta el pie con cómo cerrar la ayuda")
			}
		})
	}
}

// ---- sugerencias de los campos con autocompletado: la lógica de "crear una
// nueva" vive aquí, no en el render. ----

func TestRenderAssigneeSuggestions(t *testing.T) {
	// El roster de las fixtures es Me, @juan y @maria. Con input vacío se
	// listan todos; el match exacto se EXCLUYE de la lista (ya está elegido) y
	// sólo aparece la pista de "crear nueva" cuando no hay nadie con ese nombre.
	tests := []struct {
		name     string
		typed    string
		wantRows int
		wantNew  bool
		wantHas  string
	}{
		{"input vacío lista todo el roster", "", 3, false, "@juan"},
		{"el match exacto no se sugiere", "@juan", 0, false, ""},
		{"prefijo: quedan los parciales", "@ma", 2, true, "@maria"},
		{"nadie con ese nombre", "@nuevo", 1, true, ""},
		{"sólo espacios cuenta como vacío", "   ", 3, false, "@juan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.newTaskOpen = true
			m.newTaskFieldIdx = newTaskFieldAssignee
			m.newTaskAssignee = tt.typed

			lines := m.renderNewTaskAssigneeSuggestions()
			plain := ansi.Strip(strings.Join(lines, " | "))

			if len(lines) != tt.wantRows {
				t.Errorf("líneas = %d, want %d (%q)", len(lines), tt.wantRows, plain)
			}
			if got := strings.Contains(plain, "new:"); got != tt.wantNew {
				t.Errorf("ofrece crear = %v, want %v (%q)", got, tt.wantNew, plain)
			}
			if tt.wantHas != "" && !strings.Contains(plain, tt.wantHas) {
				t.Errorf("falta %q en %q", tt.wantHas, plain)
			}
		})
	}
}

func TestRenderTagSuggestions(t *testing.T) {
	// La tarea ya tiene "bug": ni se sugiere ni se ofrece crear.
	tests := []struct {
		name     string
		typed    string
		wantRows int
		wantNew  bool
	}{
		{"input vacío sin tags conocidas", "", 0, false},
		{"ya está en la tarea", "bug", 0, false},
		{"no existe", "nueva", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.newTaskOpen = true
			m.newTaskFieldIdx = newTaskFieldTags
			m.newTaskTagInput = tt.typed
			m.newTaskTags = []string{"bug"}

			lines := m.renderTagFieldSuggestions()
			plain := ansi.Strip(strings.Join(lines, " | "))

			if len(lines) != tt.wantRows {
				t.Errorf("líneas = %d, want %d (%q)", len(lines), tt.wantRows, plain)
			}
			if got := strings.Contains(plain, "new:"); got != tt.wantNew {
				t.Errorf("ofrece crear = %v, want %v (%q)", got, tt.wantNew, plain)
			}
		})
	}
}

// La sugerencia seleccionada se marca con ▸: es lo que el usuario ve antes de
// pulsar tab, así que un mutante que marque la fila equivocada se nota aquí.
func TestRenderSuggestionsMarkTheSelectedRow(t *testing.T) {
	m := newTestModel(t)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "@ma"

	for idx, wantMarked := range []bool{true, false} {
		m.newTaskAssigneeSuggIdx = -1
		if wantMarked {
			m.newTaskAssigneeSuggIdx = idx
		}
		lines := m.renderNewTaskAssigneeSuggestions()
		if len(lines) == 0 {
			t.Fatal("sin sugerencias")
		}
		marked := strings.Contains(ansi.Strip(lines[0]), "▸")
		if marked != wantMarked {
			t.Errorf("suggIdx %d: marcada = %v, want %v", idx, marked, wantMarked)
		}
	}
}

// ---- formulario de off-day: navegación de campos y validaciones. ----

func TestHandleOffdayFormKeyFieldNavigation(t *testing.T) {
	tests := []struct {
		name string
		from int
		key  string
		want int
	}{
		{"tab avanza", 0, "tab", 1},
		{"tab da la vuelta", 2, "tab", 0},
		{"shift+tab retrocede", 1, "shift+tab", 0},
		{"shift+tab da la vuelta", 0, "shift+tab", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.offdayFormOpen = true
			m.offdayFormField = tt.from

			got, _ := m.handleOffdayFormKey(tt.key)
			if after := got.(Model); after.offdayFormField != tt.want {
				t.Errorf("campo = %d, want %d", after.offdayFormField, tt.want)
			}
		})
	}
}

func TestHandleOffdayFormKeyWritesIntoTheActiveField(t *testing.T) {
	tests := []struct {
		field int
		key   string
		check func(m Model) string
	}{
		{0, "a", func(m Model) string { return m.offdayStartInput }},
		{1, "a", func(m Model) string { return m.offdayEndInput }},
		{2, "a", func(m Model) string { return m.offdayNoteInput }},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.field), func(t *testing.T) {
			m := newTestModel(t)
			m.offdayFormOpen = true
			m.offdayFormField = tt.field

			got, _ := m.handleOffdayFormKey(tt.key)
			if v := tt.check(got.(Model)); v != "a" {
				t.Errorf("campo %d = %q, want a", tt.field, v)
			}
		})
	}
}

func TestHandleOffdayFormKeyValidations(t *testing.T) {
	t.Run("esc cierra", func(t *testing.T) {
		m := newTestModel(t)
		m.offdayFormOpen = true
		got, _ := m.handleOffdayFormKey("esc")
		if got.(Model).offdayFormOpen {
			t.Error("esc debería cerrar el formulario")
		}
	})

	t.Run("sin assignee no guarda", func(t *testing.T) {
		m := newTestModel(t)
		m.offdayFormOpen = true
		m.assigneeIdx = -1
		m.offdayStartInput = "2026-07-28"

		got, cmd := m.handleOffdayFormKey("enter")
		if cmd == nil {
			t.Error("debería emitir un toast de error")
		}
		if !got.(Model).offdayFormOpen {
			t.Error("el formulario no debería cerrarse sin assignee")
		}
	})

	t.Run("sin fecha no guarda", func(t *testing.T) {
		m := newTestModel(t)
		m.offdayFormOpen = true
		m.assigneeIdx = 0
		m.offdayStartInput = ""

		got, cmd := m.handleOffdayFormKey("enter")
		if cmd == nil {
			t.Error("debería emitir un toast de error")
		}
		if !got.(Model).offdayFormOpen {
			t.Error("el formulario no debería cerrarse sin fecha")
		}
	})
}

// ---- modal de proyecto: navegación, escritura y validaciones. ----

func TestHandleProjectModalKeyFieldNavigation(t *testing.T) {
	tests := []struct {
		name string
		from int
		key  string
		want int
	}{
		{"tab avanza", 0, "tab", 1},
		{"tab da la vuelta", 2, "tab", 0},
		{"shift+tab retrocede", 1, "shift+tab", 0},
		{"shift+tab da la vuelta", 0, "shift+tab", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.projectModalOpen = true
			m.projectModalField = tt.from

			got, _ := m.handleProjectModalKey(tt.key)
			if after := got.(Model); after.projectModalField != tt.want {
				t.Errorf("campo = %d, want %d", after.projectModalField, tt.want)
			}
		})
	}
}

func TestHandleProjectModalKeyWritesIntoTheActiveField(t *testing.T) {
	tests := []struct {
		field int
		check func(m Model) string
	}{
		{0, func(m Model) string { return m.projectNameInput }},
		{1, func(m Model) string { return m.projectWorkflowInput }},
		{2, func(m Model) string { return m.projectListOrderInput }},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.field), func(t *testing.T) {
			m := newTestModel(t)
			m.projectModalOpen = true
			m.projectModalField = tt.field
			m.projectNameInput = ""
			m.projectWorkflowInput = ""
			m.projectListOrderInput = ""

			got, _ := m.handleProjectModalKey("x")
			if v := tt.check(got.(Model)); v != "x" {
				t.Errorf("campo %d = %q, want x", tt.field, v)
			}
		})
	}
}

func TestHandleProjectModalKeyEnter(t *testing.T) {
	t.Run("esc cierra", func(t *testing.T) {
		m := newTestModel(t)
		m.projectModalOpen = true
		got, _ := m.handleProjectModalKey("esc")
		if got.(Model).projectModalOpen {
			t.Error("esc debería cerrar el modal")
		}
	})

	t.Run("nombre vacío no guarda", func(t *testing.T) {
		m := newTestModel(t)
		m.projectModalOpen = true
		m.projectNameInput = "   "

		got, cmd := m.handleProjectModalKey("enter")
		if cmd == nil {
			t.Error("debería emitir un toast de error")
		}
		if !got.(Model).projectModalOpen {
			t.Error("el modal no debería cerrarse sin nombre")
		}
	})

	t.Run("nombre válido cierra y persiste", func(t *testing.T) {
		m := newTestModel(t)
		m.projectModalOpen = true
		m.projectModalField = 0
		m.projectNameInput = "nuevo"
		m.projectWorkflowInput = "backlog,todo,done"
		m.projectListOrderInput = ""

		got, cmd := m.handleProjectModalKey("enter")
		if cmd == nil {
			t.Fatal("debería devolver la cmd de guardado")
		}
		if got.(Model).projectModalOpen {
			t.Error("el modal debería cerrarse al guardar")
		}

		msg, ok := mustMsg(t, cmd).(projectSavedMsg)
		if !ok {
			t.Fatalf("msg = %T, want projectSavedMsg", msg)
		}
		if msg.err != nil {
			t.Fatalf("guardar falló: %v", msg.err)
		}
		if _, err := m.database.GetProject("nuevo"); err != nil {
			t.Errorf("el proyecto no se creó: %v", err)
		}
	})
}

// El modal abierto para editar prellena los campos con el proyecto elegido; el
// de crear los deja con los defaults y vacíos.
func TestOpenProjectModalCreateAndEdit(t *testing.T) {
	t.Run("crear", func(t *testing.T) {
		m := newTestModel(t)
		m.projectNameInput = "basura"
		m.projectEditingName = "basura"

		m.openProjectModal(false)

		if !m.projectModalOpen || m.projectModalEdit {
			t.Errorf("modal = %v edit=%v, want abierto y en modo crear", m.projectModalOpen, m.projectModalEdit)
		}
		if m.projectNameInput != "" || m.projectEditingName != "" {
			t.Errorf("crear debería limpiar los campos: %q/%q", m.projectNameInput, m.projectEditingName)
		}
		if m.projectWorkflowInput != "backlog,todo,doing,delivered,reviewing,done,cancelled" {
			t.Errorf("workflow por defecto = %q", m.projectWorkflowInput)
		}
	})

	t.Run("editar", func(t *testing.T) {
		m := newTestModel(t)
		p, err := m.database.GetProject("web")
		if err != nil {
			t.Fatal(err)
		}

		m.openProjectModalForEdit(p)

		if !m.projectModalOpen || !m.projectModalEdit {
			t.Errorf("modal = %v edit=%v, want abierto y en modo editar", m.projectModalOpen, m.projectModalEdit)
		}
		if m.projectNameInput != "web" || m.projectEditingName != "web" {
			t.Errorf("nombre = %q, original = %q", m.projectNameInput, m.projectEditingName)
		}
		if !strings.Contains(m.projectWorkflowInput, "todo") {
			t.Errorf("workflow precargado = %q", m.projectWorkflowInput)
		}
	})
}

// El índice del proyecto resaltado se recorta al rango válido: es lo que evita
// que un filtro deje el cursor apuntando a la nada.
func TestClampDashProjectIdx(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		want  int
	}{
		{"dentro de rango", func(m *Model) { m.dashProjectIdx = 1 }, 1},
		{"muy alto", func(m *Model) { m.dashProjectIdx = 99 }, 1},
		{"negativo", func(m *Model) { m.dashProjectIdx = -5 }, 0},
		{"sin proyectos", func(m *Model) { m.projects = nil; m.dashProjectIdx = 3 }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			tt.setup(m)
			m.clampDashProjectIdx()
			if m.dashProjectIdx != tt.want {
				t.Errorf("dashProjectIdx = %d, want %d", m.dashProjectIdx, tt.want)
			}
		})
	}
}

// ---- toast: aparece, se recorta al ancho y cambia de estilo según el tipo. ----

func TestRenderToast(t *testing.T) {
	tests := []struct {
		name      string
		toast     string
		kind      string
		width     int
		wantEmpty bool
		wantText  string
	}{
		{"sin toast", "", "info", 80, true, ""},
		{"normal", "guardado", "info", 80, false, "guardado"},
		{"largo se recorta", "0123456789012345678901234567890123456789", "info", 20, false, "…"},
		{"ancho mínimo no recorta", "ab", "info", 4, false, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.toast = tt.toast
			m.toastKind = tt.kind
			m.width = tt.width

			got := ansi.Strip(m.renderToast())
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("renderToast() = %q, want \"\"", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("renderToast() vacío, want %q", tt.wantText)
			}
			if tt.wantText == "…" && !strings.Contains(got, "…") {
				t.Errorf("renderToast() = %q, quiere la elipsis de recorte", got)
			}
			if tt.wantText != "…" && !strings.Contains(got, tt.wantText) {
				t.Errorf("renderToast() = %q, want %q", got, tt.wantText)
			}
		})
	}
}

// Un toast de error se pinta con el estilo de error, no con el de estado: si se
// intercambian, el usuario ve un fallo con color de éxito.
func TestRenderToastErrorUsesErrorStyle(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.toast = "boom"
	m.toastKind = "error"

	styled := m.renderToast()
	if !strings.Contains(styled, "\x1b[") {
		t.Fatalf("sin estilos ANSI: %q", styled)
	}
	if styleError.Render(" boom") != styled {
		t.Errorf("un toast de error debería usar styleError: %q", styled)
	}

	m.toastKind = "info"
	if styled := m.renderToast(); styleError.Render(" boom") == styled {
		t.Error("un toast informativo no debería usar el estilo de error")
	}
}

// ---- formulario de off-day: el cursor "_" marca el campo activo. ----

func TestRenderOffdayFormMarksTheActiveField(t *testing.T) {
	// El cursor "_" va detrás del campo activo: si se pone en otro sitio, el
	// usuario escribe en el campo equivocado sin darse cuenta.
	tests := []struct {
		field  int
		marker string
	}{
		{0, "2026-07-28_"},
		{1, "2026-08-01_"},
		{2, "nota_"},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.field), func(t *testing.T) {
			m := newTestModel(t)
			m.assigneeIdx = 0
			m.offdayFormOpen = true
			m.offdayFormField = tt.field
			m.offdayStartInput = "2026-07-28"
			m.offdayEndInput = "2026-08-01"
			m.offdayNoteInput = "nota"

			out := ansi.Strip(m.renderOffdayForm(""))
			if !strings.Contains(out, tt.marker) {
				t.Errorf("campo %d: falta el cursor %q:\n%s", tt.field, tt.marker, out)
			}
		})
	}
}

// El formulario siempre enseña sus cuatro campos con etiqueta y la pista de
// formato: si un campo desaparece del render, nadie se entera salvo aquí.
func TestRenderOffdayFormShowsEveryField(t *testing.T) {
	m := newTestModel(t)
	m.assigneeIdx = 0
	m.offdayFormOpen = true
	m.offdayFormField = 0

	out := ansi.Strip(m.renderOffdayForm(""))
	for _, want := range []string{"Person:", "Start:", "End:", "Note:", "YYYY-MM-DD"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q:\n%s", want, out)
		}
	}
}

// El modal se dibuja incluso sin contenido detrás: es un caso real al abrirlo
// desde un estado vacío, y si el overlay no tolera un fondo corto revienta.
func TestRenderOffdayFormOverEmptyBackground(t *testing.T) {
	m := newTestModel(t)
	m.assigneeIdx = 0
	m.offdayFormOpen = true
	m.width = 100

	out := ansi.Strip(m.renderOffdayForm(""))
	if !strings.Contains(out, "New off-day") {
		t.Errorf("sin fondo tampoco debe faltar el modal:\n%s", out)
	}
}

// ---- PreviewBar y el editor de descripción: ajustes de tamaño. ----

func TestPreviewBarSetWidth(t *testing.T) {
	var p PreviewBar
	p.SetWidth(42)
	if p.width != 42 {
		t.Errorf("width = %d, want 42", p.width)
	}
	p.SetMaxLines(7)
	if p.maxLines != 7 {
		t.Errorf("maxLines = %d, want 7", p.maxLines)
	}
}

// El editor de descripción seDimensiona al ancho de la caja del detalle, no al
// de la terminal. Se prueba por el camino real (abrir el editor y luego
// redimensionar) porque el textarea sólo existe mientras el editor está
// abierto: llamar a resizeDescEditor() sobre un modelo recién creado no es un
// estado alcanzable.
func TestResizeDescEditorFollowsTerminal(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "e")
	if !m.descEditOpen {
		t.Fatal("no se pudo abrir el editor inline")
	}

	m.width = 100
	m, _ = send(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	wide := m.descEditTextarea.Width()
	if want := m.descEditorWidth(); wide != want {
		t.Errorf("ancho del textarea = %d, want %d", wide, want)
	}

	m, _ = send(m, tea.WindowSizeMsg{Width: 40, Height: 40})
	narrow := m.descEditTextarea.Width()
	if want := m.descEditorWidth(); narrow != want {
		t.Errorf("tras estrechar: ancho = %d, want %d", narrow, want)
	}
	if narrow >= wide {
		t.Errorf("estrechar la terminal no estrechó el editor: %d -> %d", wide, narrow)
	}

	// Un terminal estrechísimo nunca deja el ancho en un valor inválido.
	m, _ = send(m, tea.WindowSizeMsg{Width: 2, Height: 10})
	if got := m.descEditorWidth(); got < 1 {
		t.Errorf("descEditorWidth() = %d, want >= 1", got)
	}

	// Con el editor cerrado, un resize no debe tocar nada.
	m.descEditOpen = false
	before := m.descEditTextarea.Width()
	m, _ = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if got := m.descEditTextarea.Width(); got != before {
		t.Errorf("con el editor cerrado el ancho cambió: %d -> %d", before, got)
	}
}
