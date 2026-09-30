package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"tsk/internal/model"
)

// ntKey envía una tecla al modal de alta. No reutiliza press() porque este
// handler necesita teclas que press() no construye (flechas, shift+tab,
// ctrl+s) y porque varias aserciones necesitan el cmd devuelto intacto.
func ntKey(m *Model, key string) (*Model, tea.Cmd) {
	var km tea.KeyPressMsg
	switch key {
	case "enter":
		km = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		km = tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		km = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "esc":
		km = tea.KeyPressMsg{Code: tea.KeyEsc}
	case "up":
		km = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		km = tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		km = tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		km = tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		km = tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+s":
		km = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case ",":
		km = tea.KeyPressMsg{Code: ',', Text: ","}
	default:
		km = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	next, cmd := m.Update(km)
	return asModel(next), cmd
}

// modelWithPeopleAndTags crea un modelo con las personas y tags que el
// autocompletado debe oferecer. Los fixtures por defecto sólo traen @juan y
// @maria y ninguna tag, así que los topes del dropdown nunca se alcanzan.
func modelWithPeopleAndTags(t *testing.T, people []string, tags []string) *Model {
	t.Helper()
	m := newTestModel(t)
	m.tasks = nil
	if len(people) == 0 {
		people = []string{"@ana"} // hace falta al menos una tarea para que existan tags
	}
	for i, p := range people {
		m.tasks = append(m.tasks, model.Task{
			ID:       int64(i + 1),
			Title:    "t",
			Status:   "todo",
			Assignee: p,
			Tags:     tags,
		})
	}
	m.invalidateFilterCache()
	return m
}

func openNewTask(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m, _ = ntKey(m, "i")
	if !m.newTaskOpen {
		t.Fatal("el modal de alta no se abrió")
	}
	return m
}

// --- Topes del dropdown de sugerencias -----------------------------------

// El dropdown de assignees se corta en newTaskMaxSuggestions. El corte importa:
// sin él el modal crece sin límite y `len(out) == max` invertido (que corta en
// cuanto len(out) != max, o sea en el primero) devolvería una sola sugerencia.
func TestNewTaskAssigneeSuggestionsCap(t *testing.T) {
	people := []string{"@ana", "@carla", "@david", "@elena", "@maria", "@fatima"}
	m := modelWithPeopleAndTags(t, people, nil)
	m.newTaskAssignee = "a" // las seis matchean, así que sólo el tope decide

	got := m.assigneeSuggestions()
	if len(got) != newTaskMaxSuggestions {
		t.Errorf("sugerencias = %d, want el tope %d (%v)", len(got), newTaskMaxSuggestions, got)
	}
}

func TestNewTaskTagSuggestionsCap(t *testing.T) {
	tags := []string{"alpha", "beta", "gamma", "delta", "zeta", "gamma2"}
	m := modelWithPeopleAndTags(t, nil, tags)
	m.newTaskTagInput = "a" // todas matchean, así que sólo el tope decide

	got := m.tagFieldSuggestions()
	if len(got) != newTaskMaxSuggestions {
		t.Errorf("sugerencias de tags = %d, want el tope %d (%v)", len(got), newTaskMaxSuggestions, got)
	}
}

// El match exacto no se sugiere a sí mismo: es lo que ya está escrito.
func TestNewTaskSuggestionsExcludeExactMatch(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@bruno"}, []string{"urgent", "backend"})

	m.newTaskAssignee = "@ana"
	if got := m.assigneeSuggestions(); contains(got, "@ana") {
		t.Errorf("el assignee exacto no debe sugerirse: %v", got)
	}

	m.newTaskTagInput = "urgent"
	if got := m.tagFieldSuggestions(); contains(got, "urgent") {
		t.Errorf("la tag exacta no debe sugerirse: %v", got)
	}
}

func contains(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}

// --- Ancho del textarea ---------------------------------------------------

func TestNewTaskTextareaWidth(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantMin   int
		wantExact int // > 0 cuando el ancho preferred cabe entero
	}{
		{"pantalla ancha usa el preferred", 120, 10, newTaskModalWidth - 6},
		{"pantalla mediana recorta al disponible", 40, 10, 0},
		{"pantalla diminuta nunca baja de 10", 12, 10, 0},
		{"pantalla de 1 columna", 1, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.width = tt.width

			got := m.newTaskTextareaWidth()
			if got < tt.wantMin {
				t.Errorf("newTaskTextareaWidth() = %d, want >= %d", got, tt.wantMin)
			}
			if tt.wantExact > 0 && got != tt.wantExact {
				t.Errorf("newTaskTextareaWidth() = %d, want %d", got, tt.wantExact)
			}
		})
	}
}

// --- Completado de sugerencia al tabular o enviar --------------------------

func TestNewTaskMoveFieldCompletesAssigneeSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, nil)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = 0
	if len(m.assigneeSuggestions()) == 0 {
		t.Fatal("fixture: sin sugerencias no hay nada que completar")
	}

	m, _ = ntKey(m, "tab")
	if m.newTaskAssignee == "" {
		t.Error("tabular con sugerencia activa debe completar el assignee")
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx tras completar = %d, want -1", m.newTaskAssigneeSuggIdx)
	}
	if m.newTaskFieldIdx != newTaskFieldTags {
		t.Errorf("el foco debe avanzar igualmente, campo = %d", m.newTaskFieldIdx)
	}
}

// Un índice de sugerencia obsoleto (el filtro cambió por debajo) no debe
// indexar fuera de rango ni completar con basura.
func TestNewTaskMoveFieldIgnoresStaleSuggestionIndex(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "escrito"
	m.newTaskAssigneeSuggIdx = 99 // más allá de la lista de sugerencias

	m, _ = ntKey(m, "tab")
	if m.newTaskAssignee != "escrito" {
		t.Errorf("assignee = %q, want el valor escrito intacto", m.newTaskAssignee)
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 (se limpia igual)", m.newTaskAssigneeSuggIdx)
	}
}

// La sugerencia sólo se completa en el campo Assignee: en otro campo el mismo
// índice no debe tocar nada.
func TestNewTaskSuggestionOnlyCompletesOnAssigneeField(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskAssignee = "intacto"
	m.newTaskAssigneeSuggIdx = 0

	m, _ = ntKey(m, "tab")
	if m.newTaskAssignee != "intacto" {
		t.Errorf("assignee = %q, want intacto fuera del campo Assignee", m.newTaskAssignee)
	}
}

// newTaskSubmit completa la sugerencia igual que el tabular, así que enviar
// mientras se navega el dropdown no pierde la selección.
func TestNewTaskSubmitCompletesAssigneeSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskTitle = "tarea"
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = 1

	if len(m.assigneeSuggestions()) < 2 {
		t.Fatalf("fixture: hacen falta 2 sugerencias, hay %d", len(m.assigneeSuggestions()))
	}

	_, cmd := ntKey(m, "ctrl+s")
	msgs := mustRun(t, cmd)

	tasks, err := m.database.ListTasks("api", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var created *model.Task
	for i := range tasks {
		if tasks[i].Title == "tarea" {
			created = &tasks[i]
		}
	}
	if created == nil {
		t.Fatal("no se creó la tarea")
	}
	if created.Assignee == "" {
		t.Error("la sugerencia activa debía completar el assignee al enviar")
	}
	if !containsMsgType(msgs, tasksLoadedMsg{}) {
		t.Errorf("msgs = %v, want un tasksLoadedMsg", msgTypes(msgs))
	}
}

// containsMsgType y msgTypes hacen legible el aserto sobre el tipo de los
// mensajes que produjo un batch.
func containsMsgType(msgs []tea.Msg, want tea.Msg) bool {
	for _, msg := range msgs {
		if reflect.TypeOf(msg) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}

func msgTypes(msgs []tea.Msg) []string {
	var out []string
	for _, msg := range msgs {
		out = append(out, fmt.Sprintf("%T", msg))
	}
	return out
}

// enter sobre el campo Assignee con una sugerencia activa completa la selección
// y NO envía el alta: el dropdown gana a la validación.
func TestNewTaskAssigneeEnterCompletesSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = 0
	suggs := m.assigneeSuggestions()
	if len(suggs) == 0 {
		t.Fatal("fixture: sin sugerencias")
	}

	m, _ = ntKey(m, "enter")

	if !m.newTaskOpen {
		t.Error("enter con sugerencia activa no debe enviar el alta")
	}
	if m.newTaskAssignee != suggs[0] {
		t.Errorf("assignee = %q, want la sugerencia %q", m.newTaskAssignee, suggs[0])
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 tras completar", m.newTaskAssigneeSuggIdx)
	}
}

// enter con un índice exactamente igual al tamaño de la lista cae en el submit,
// no en un indexado fuera de rango. Es el borde exacto del `< len(suggs)`.
func TestNewTaskAssigneeEnterAtExactListLength(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskTitle = "borde exacto"
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = len(m.assigneeSuggestions()) // uno más allá del último

	_, cmd := ntKey(m, "enter")
	mustRun(t, cmd)

	tasks, _ := m.database.ListTasks("api", "", "")
	found := false
	for _, task := range tasks {
		if task.Title == "borde exacto" {
			found = true
		}
	}
	if !found {
		t.Error("con el índice fuera de rango, enter debe enviar el alta")
	}
}

// Enviar desde otro campo con un índice de sugerencia activo NO completa el
// assignee: la condición exige estar en el campo Assignee, no sólo tener índice.
func TestNewTaskSubmitFromTitleIgnoresAssigneeSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, nil)
	m.newTaskOpen = true
	m.newTaskProject = "api"
	m.newTaskFieldIdx = newTaskFieldTitle
	m.newTaskTitle = "desde title"
	m.newTaskAssignee = ""
	// Índice 1 a propósito: la sugerencia 0 es "Me", que es justo lo que
	// espera el default del submit, así que con 0 el mutante pasaría
	// desapercibido.
	m.newTaskAssigneeSuggIdx = 1
	if suggs := m.assigneeSuggestions(); len(suggs) < 2 || suggs[1] == "Me" {
		t.Fatalf("fixture: la segunda sugerencia debe ser otra persona, hay %v", suggs)
	}

	_, cmd := ntKey(m, "ctrl+s")
	mustRun(t, cmd)

	tasks, _ := m.database.ListTasks("api", "", "")
	for _, task := range tasks {
		if task.Title == "desde title" {
			if task.Assignee != "Me" {
				t.Errorf("assignee = %q, want Me (no se completa fuera del campo)", task.Assignee)
			}
			return
		}
	}
	t.Fatal("no se creó la tarea")
}

// --- Defaults del alta ----------------------------------------------------

// Sin assignee explícito la tarea se atribuye a "Me": ese default se pierde
// con el `== ""` invertido.
func TestNewTaskSubmitDefaultsAssigneeToMe(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTitle = "sin assignee"
	m.newTaskAssignee = "   " // sólo espacios, se trimmea a vacío

	_, cmd := ntKey(m, "ctrl+s")
	mustRun(t, cmd)

	tasks, _ := m.database.ListTasks("api", "", "")
	found := false
	for _, task := range tasks {
		if task.Title == "sin assignee" {
			found = true
			if task.Assignee != "Me" {
				t.Errorf("assignee = %q, want Me", task.Assignee)
			}
		}
	}
	if !found {
		t.Fatal("no se creó la tarea")
	}
}

func TestNewTaskCloseResetsState(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTitle = "a medias"
	m.newTaskAssignee = "@ana"
	m.newTaskErr = "algo"
	m.newTaskAssigneeSuggIdx = 2
	m.newTaskTags = []string{"x"}

	m, _ = ntKey(m, "esc")

	if m.newTaskOpen {
		t.Error("esc debe cerrar el modal")
	}
	if m.newTaskTitle != "" || m.newTaskAssignee != "" || m.newTaskErr != "" {
		t.Errorf("el estado transitorio debe limpiarse: title=%q assignee=%q err=%q",
			m.newTaskTitle, m.newTaskAssignee, m.newTaskErr)
	}
	if m.newTaskAssigneeSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 tras cerrar", m.newTaskAssigneeSuggIdx)
	}
	// newTaskClose NO limpia las tags: el reset completo ocurre en newTask(),
	// que es quien abre. Comprobado aquí para que el contrato sea explícito.
	if len(m.newTaskTags) != 1 {
		t.Errorf("tags = %v, want la tag intacta (close no las toca)", m.newTaskTags)
	}
}

// Reabrir el modal parte de un estado limpio, aunque close no limpiara todo.
func TestNewTaskReopenResetsTagsAndPriority(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTags = []string{"basura"}
	m.newTaskTagInput = "medio"
	m.newTaskPriority = model.PriorityHigh
	m.newTaskErr = "error viejo"
	m, _ = ntKey(m, "esc")
	m, _ = ntKey(m, "i")

	if m.newTaskTags != nil {
		t.Errorf("tags = %v, want nil tras reabrir", m.newTaskTags)
	}
	if m.newTaskTagInput != "" {
		t.Errorf("input = %q, want vacío tras reabrir", m.newTaskTagInput)
	}
	if m.newTaskPriority != model.PriorityLow {
		t.Errorf("prioridad = %d, want Low (default de apertura)", m.newTaskPriority)
	}
	if m.newTaskErr != "" {
		t.Errorf("err = %q, want vacío tras reabrir", m.newTaskErr)
	}
}

// --- Prioridad ------------------------------------------------------------

// left/right se detienen en los extremos. La condición es `> PriorityNone` y
// `< PriorityHigh`; sin los casos de borde el `>=`/`<=` equivalente pasa
// desapercibido.
func TestNewTaskPriorityArrowsStopAtEdges(t *testing.T) {
	tests := []struct {
		name     string
		from     int
		key      string
		wantFrom int
	}{
		{"left en none no baja", model.PriorityNone, "left", model.PriorityNone},
		{"left en low baja a none", model.PriorityLow, "left", model.PriorityNone},
		{"right en high no sube", model.PriorityHigh, "right", model.PriorityHigh},
		{"right en med sube a high", model.PriorityMedium, "right", model.PriorityHigh},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := openNewTask(t)
			m.newTaskFieldIdx = newTaskFieldPriority
			m.newTaskPriority = tt.from

			m, _ = ntKey(m, tt.key)
			if m.newTaskPriority != tt.wantFrom {
				t.Errorf("prioridad = %d tras %s desde %d, want %d",
					m.newTaskPriority, tt.key, tt.from, tt.wantFrom)
			}
		})
	}
}

func TestNewTaskPriorityDigits(t *testing.T) {
	// Las teclas 1-4 asignan el dígito tal cual, sin offset: "1" deja Low y "4"
	// deja 4, que está fuera del rango válido 0-3. Es un bug (tecla 4 produce una
	// prioridad que no existe y la tarea se guarda con ella), pero se fija aquí
	// como comportamiento actual: corregirlo es un cambio de comportamiento
	// deliberado, no un test que falte. Ver TestNewTaskPriorityDigitFourIsOutOfRange.
	for digit, want := range map[string]int{
		"1": 1,
		"2": 2,
		"3": 3,
		"4": 4,
	} {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldPriority

		m, _ = ntKey(m, digit)
		if m.newTaskPriority != want {
			t.Errorf("tecla %s: prioridad = %d, want %d", digit, m.newTaskPriority, want)
		}
	}
}

// Documenta el bug: la tecla 4 deja la prioridad fuera del rango 0-3, así que
// ningún radio se marca y la etiqueta cae al default "-".
func TestNewTaskPriorityDigitFourIsOutOfRange(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldPriority
	m, _ = ntKey(m, "4")

	if m.newTaskPriority <= model.PriorityHigh {
		t.Fatalf("prioridad = %d, se esperaba fuera del rango 0-3", m.newTaskPriority)
	}
	if got := ansi.Strip(m.renderNewTaskPriority()); strings.Contains(got, "●") {
		t.Errorf("render = %q, ningún radio debería quedar marcado", got)
	}
	if got := model.PriorityShortLabel(m.newTaskPriority); got != "-" {
		t.Errorf("PriorityShortLabel(%d) = %q, want - (fuera de rango)", m.newTaskPriority, got)
	}
}

// El límite del filtro de caracteres imprimibles es 33 ('!'). Con `> 33` el
// signo de exclamación dejaría de escribirse, así que el borde se fija.
func TestNewTaskPrintableKeyJumpsToTitle(t *testing.T) {
	// El umbral es 33 ('!'). El espacio es 32, así que NO cuenta como
	// imprimible y debe quedarse en el campo sin tocar el título.
	for _, ch := range []string{"a", "Z", "!", "9", "-"} {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldPriority

		m, _ = ntKey(m, ch)
		if m.newTaskFieldIdx != newTaskFieldTitle {
			t.Errorf("tecla %q: campo = %d, want title", ch, m.newTaskFieldIdx)
		}
		if !strings.Contains(m.newTaskTitle, ch) {
			t.Errorf("tecla %q: title = %q, want que contenga la tecla", ch, m.newTaskTitle)
		}
	}
}

// ctrl+s se resuelve antes de mirar el campo activo: envía el alta, no salta al
// título. Con el título vacío devuelve el error inline y devuelve el foco a
// Title sin cerrar el modal.
func TestNewTaskCtrlSSubmitsFromPriorityField(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldPriority

	m, _ = ntKey(m, "ctrl+s")

	if !m.newTaskOpen {
		t.Error("el modal no debe cerrarse sin título")
	}
	if m.newTaskFieldIdx != newTaskFieldTitle {
		t.Errorf("campo = %d, want title tras el error de validación", m.newTaskFieldIdx)
	}
	if m.newTaskErr == "" {
		t.Error("want el error inline por falta de título")
	}
}

// --- Navegación del dropdown de assignees ---------------------------------

func TestNewTaskAssigneeSuggestionWrapAround(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla", "@david"}, nil)
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldAssignee
	m.newTaskAssignee = "a"
	m.newTaskAssigneeSuggIdx = -1

	if got := len(m.assigneeSuggestions()); got != 3 {
		t.Fatalf("fixture: 3 sugerencias esperadas, hay %d", got)
	}

	// down recorre y da la vuelta al principio.
	wantDown := []int{0, 1, 2, 0}
	for i, want := range wantDown {
		m, _ = ntKey(m, "down")
		if m.newTaskAssigneeSuggIdx != want {
			t.Fatalf("down #%d: suggIdx = %d, want %d", i+1, m.newTaskAssigneeSuggIdx, want)
		}
	}

	// up desde 0 salta al último; desde el último baja a 0.
	wantUp := []int{2, 1, 0}
	for i, want := range wantUp {
		m, _ = ntKey(m, "up")
		if m.newTaskAssigneeSuggIdx != want {
			t.Fatalf("up #%d: suggIdx = %d, want %d", i+1, m.newTaskAssigneeSuggIdx, want)
		}
	}
}

// Sin sugerencias, up/down no tocan el índice. Con `>= 0` invertido el módulo
// por cero revienta el test.
func TestNewTaskAssigneeSuggestionArrowsWithNoSuggestions(t *testing.T) {
	m := openNewTask(t) // assignee "Me" no sugiere nada
	m.newTaskFieldIdx = newTaskFieldAssignee
	if got := m.assigneeSuggestions(); len(got) != 0 {
		t.Fatalf("fixture: se esperaban 0 sugerencias, hay %d (%v)", len(got), got)
	}
	m.newTaskAssigneeSuggIdx = -1

	for _, key := range []string{"up", "down", "up", "down"} {
		m, _ = ntKey(m, key)
		if m.newTaskAssigneeSuggIdx != -1 {
			t.Errorf("tras %q sin sugerencias suggIdx = %d, want -1", key, m.newTaskAssigneeSuggIdx)
		}
	}
}

// --- Tags -----------------------------------------------------------------

func TestNewTaskTagSuggestionWrapAround(t *testing.T) {
	m := modelWithPeopleAndTags(t, nil, []string{"alpha", "beta", "gamma"})
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "a"
	m.newTaskTagSuggIdx = -1

	if got := len(m.tagFieldSuggestions()); got != 3 {
		t.Fatalf("fixture: 3 sugerencias esperadas, hay %d", got)
	}

	for i, want := range []int{0, 1, 2, 0} {
		m, _ = ntKey(m, "down")
		if m.newTaskTagSuggIdx != want {
			t.Fatalf("down #%d: suggIdx = %d, want %d", i+1, m.newTaskTagSuggIdx, want)
		}
	}
	for i, want := range []int{2, 1, 0} {
		m, _ = ntKey(m, "up")
		if m.newTaskTagSuggIdx != want {
			t.Fatalf("up #%d: suggIdx = %d, want %d", i+1, m.newTaskTagSuggIdx, want)
		}
	}
}

func TestNewTaskTagSuggestionArrowsWithNoSuggestions(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "qqqq" // no matchea ninguna tag
	if got := m.tagFieldSuggestions(); len(got) != 0 {
		t.Fatalf("fixture: se esperaban 0 sugerencias, hay %d (%v)", len(got), got)
	}
	m.newTaskTagSuggIdx = -1

	for _, key := range []string{"up", "down"} {
		m, _ = ntKey(m, key)
		if m.newTaskTagSuggIdx != -1 {
			t.Errorf("tras %q sin sugerencias suggIdx = %d, want -1", key, m.newTaskTagSuggIdx)
		}
	}
}

func TestNewTaskTagEnterCompletesSuggestion(t *testing.T) {
	m := modelWithPeopleAndTags(t, nil, []string{"backend", "frontend"})
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "e"
	m.newTaskTagSuggIdx = 0
	if len(m.tagFieldSuggestions()) == 0 {
		t.Fatal("fixture: sin sugerencias no hay nada que confirmar")
	}

	m, _ = ntKey(m, "enter")

	if len(m.newTaskTags) != 1 {
		t.Fatalf("tags = %v, want una tag agregada", m.newTaskTags)
	}
	if m.newTaskTagInput != "" {
		t.Errorf("input = %q, want vacío tras confirmar", m.newTaskTagInput)
	}
	if m.newTaskTagSuggIdx != -1 {
		t.Errorf("suggIdx = %d, want -1 tras confirmar", m.newTaskTagSuggIdx)
	}
}

// enter sin sugerencia activa committea lo tipeado tal cual.
func TestNewTaskTagEnterCommitsTypedText(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "nueva"
	m.newTaskTagSuggIdx = -1

	m, _ = ntKey(m, "enter")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "nueva" {
		t.Errorf("tags = %v, want [nueva]", m.newTaskTags)
	}
}

func TestNewTaskTagCommaCommitsTypedText(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "concoma"

	m, _ = ntKey(m, ",")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "concoma" {
		t.Errorf("tags = %v, want [concoma]", m.newTaskTags)
	}
}

// Un índice de sugerencia obsoleto al confirmar no debe indexar fuera de rango.
func TestNewTaskTagEnterIgnoresStaleSuggestionIndex(t *testing.T) {
	m := modelWithPeopleAndTags(t, nil, []string{"alpha", "beta"})
	m.newTaskOpen = true
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "zzz"
	m.newTaskTagSuggIdx = 99

	m, _ = ntKey(m, "enter")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "zzz" {
		t.Errorf("tags = %v, want [zzz] (el índice obsoleto se ignora)", m.newTaskTags)
	}
}

func TestNewTaskTagBackspaceRemovesLastTagWhenInputEmpty(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTags = []string{"uno", "dos"}
	m.newTaskTagInput = ""

	m, _ = ntKey(m, "backspace")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "uno" {
		t.Errorf("tags = %v, want [uno]", m.newTaskTags)
	}
}

func TestNewTaskTagBackspaceEditsNonEmptyInput(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTags = []string{"uno"}
	m.newTaskTagInput = "abc"

	m, _ = ntKey(m, "backspace")

	if m.newTaskTagInput != "ab" {
		t.Errorf("input = %q, want ab", m.newTaskTagInput)
	}
	if len(m.newTaskTags) != 1 {
		t.Errorf("tags = %v, want la pila intacta", m.newTaskTags)
	}
}

// Backspace con el input vacío y sin tags no debe intentar borrar de una
// slice vacía.
func TestNewTaskTagBackspaceWithNoTagsIsSafe(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTags = nil
	m.newTaskTagInput = ""

	m, _ = ntKey(m, "backspace")

	if len(m.newTaskTags) != 0 {
		t.Errorf("tags = %v, want ninguna", m.newTaskTags)
	}
}

// Escribir limpia la selección: la lista sugerida deja de corresponder a lo
// que hay escrito.
func TestNewTaskTypingResetsSuggestionIndex(t *testing.T) {
	t.Run("assignee", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, nil)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldAssignee
		m.newTaskAssignee = "a"
		m.newTaskAssigneeSuggIdx = 1

		m, _ = ntKey(m, "n")
		if m.newTaskAssigneeSuggIdx != -1 {
			t.Errorf("suggIdx = %d, want -1 al escribir", m.newTaskAssigneeSuggIdx)
		}
		if m.newTaskAssignee != "an" {
			t.Errorf("assignee = %q, want an", m.newTaskAssignee)
		}
	})

	t.Run("tags", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldTags
		m.newTaskTagInput = "a"
		m.newTaskTagSuggIdx = 0
		if len(m.tagFieldSuggestions()) == 0 {
			t.Fatal("fixture: sin sugerencias el índice no es observable")
		}

		m, _ = ntKey(m, "l")
		if m.newTaskTagSuggIdx != -1 {
			t.Errorf("suggIdx = %d, want -1 al escribir", m.newTaskTagSuggIdx)
		}
		if m.newTaskTagInput != "al" {
			t.Errorf("input = %q, want al", m.newTaskTagInput)
		}
	})
}

// --- Render: tags ---------------------------------------------------------

// Los tres estados de la fila de tags distinguen campo enfocado, campo con
// contenido y campo vacío. Son ramas distintas del render, no variantes.
func TestRenderNewTaskTagsStates(t *testing.T) {
	tests := []struct {
		name     string
		field    int
		tags     []string
		input    string
		wantHas  []string
		wantMiss []string
	}{
		{
			// El hint "type to add…" es inalcanzable: cuando el campo Tags tiene
			// el foco, el cursor se añade a input ANTES del if, así que input
			// nunca está vacío ahí y la rama que lo devuelve no se alcanza. Se
			// fija el comportamiento real; el hint muerto es un bug de UI
			// reportado aparte, no algo que un test pueda matar.
			name:     "vacío y enfocado sólo muestra el cursor",
			field:    newTaskFieldTags,
			wantHas:  []string{cursorGlyph},
			wantMiss: []string{"type to add", "—"},
		},
		{
			name:     "vacío y sin foco muestra el guion",
			field:    newTaskFieldTitle,
			wantHas:  []string{"—"},
			wantMiss: []string{"type to add", cursorGlyph},
		},
		{
			name:     "con tags muestra los chips",
			field:    newTaskFieldTags,
			tags:     []string{"api"},
			wantHas:  []string{"[api]"},
			wantMiss: []string{"type to add", "—"},
		},
		{
			name:     "input a medio tipear se muestra con los chips",
			field:    newTaskFieldTags,
			tags:     []string{"api"},
			input:    "med",
			wantHas:  []string{"[api]", "med"},
			wantMiss: []string{"type to add", "—"},
		},
		{
			name:     "varias tags",
			field:    newTaskFieldTags,
			tags:     []string{"api", "web"},
			wantHas:  []string{"[api]", "[web]"},
			wantMiss: []string{"type to add", "—"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := openNewTask(t)
			m.newTaskFieldIdx = tt.field
			m.newTaskTags = tt.tags
			m.newTaskTagInput = tt.input

			got := ansi.Strip(m.renderNewTaskTags())
			for _, want := range tt.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("render = %q, want que contenga %q", got, want)
				}
			}
			for _, bad := range tt.wantMiss {
				if strings.Contains(got, bad) {
					t.Errorf("render = %q, want que NO contenga %q", got, bad)
				}
			}
		})
	}
}

// El cursor sólo aparece en el campo enfocado.
func TestRenderNewTaskTagsCursorOnlyWhenFocused(t *testing.T) {
	m := openNewTask(t)
	m.newTaskTags = []string{"api"}

	m.newTaskFieldIdx = newTaskFieldTags
	withCursor := ansi.Strip(m.renderNewTaskTags())

	m.newTaskFieldIdx = newTaskFieldTitle
	withoutCursor := ansi.Strip(m.renderNewTaskTags())

	if withCursor == withoutCursor {
		t.Errorf("el cursor debe depender del foco:\n con foco: %q\n sin foco: %q", withCursor, withoutCursor)
	}
	if !strings.Contains(withCursor, cursorGlyph) {
		t.Errorf("con foco = %q, want el cursor %q", withCursor, cursorGlyph)
	}
	if strings.Contains(withoutCursor, cursorGlyph) {
		t.Errorf("sin foco = %q, want sin cursor", withoutCursor)
	}
}

func TestRenderNewTaskPriority(t *testing.T) {
	for p := model.PriorityNone; p <= model.PriorityHigh; p++ {
		m := openNewTask(t)
		m.newTaskPriority = p

		got := ansi.Strip(m.renderNewTaskPriority())
		labels := []string{"none", "low", "med", "high"}
		for i, l := range labels {
			marker := "○"
			if i == p {
				marker = "●"
			}
			want := marker + " " + l
			if !strings.Contains(got, want) {
				t.Errorf("prioridad %d: render = %q, want %q", p, got, want)
			}
		}
		if n := strings.Count(got, "●"); n != 1 {
			t.Errorf("prioridad %d: %d radios llenos, want exactamente 1", p, n)
		}
	}
}

// --- Render: sugerencias "new" --------------------------------------------

// Lo tipeado que no existe todavía se insinúa como creación nueva, tanto en
// tags como en assignee.
func TestRenderSuggestionsHintAtNewItem(t *testing.T) {
	t.Run("tags", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskTagInput = "inventada"

		got := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
		if !strings.Contains(got, "+ new: inventada") {
			t.Errorf("render = %q, want la pista de tag nueva", got)
		}
	})

	t.Run("tags ya existentes no se insinúan", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskTagInput = "alpha"

		got := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
		if strings.Contains(got, "+ new:") {
			t.Errorf("render = %q, no debe insinuar una tag que ya existe", got)
		}
	})

	t.Run("tags ya agregadas no se insinúan", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha"})
		m.newTaskTagInput = "alpha"
		m.newTaskTags = []string{"alpha"}

		got := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
		if strings.Contains(got, "+ new:") {
			t.Errorf("render = %q, no debe insinuar una tag ya agregada", got)
		}
	})

	t.Run("assignee", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ana"}, nil)
		m.newTaskAssignee = "nuevo"

		got := ansi.Strip(strings.Join(m.renderNewTaskAssigneeSuggestions(), "\n"))
		if !strings.Contains(got, "✎ new: nuevo") {
			t.Errorf("render = %q, want la pista de persona nueva", got)
		}
	})

	t.Run("assignee existente no se insinúa", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ana"}, nil)
		m.newTaskAssignee = "@ana"

		got := ansi.Strip(strings.Join(m.renderNewTaskAssigneeSuggestions(), "\n"))
		if strings.Contains(got, "✎ new:") {
			t.Errorf("render = %q, no debe insinuar una persona que ya existe", got)
		}
	})
}

// El ítem activo del dropdown se marca; los demás no.
func TestRenderSuggestionsMarkActiveItem(t *testing.T) {
	m := modelWithPeopleAndTags(t, []string{"@ana", "@carla"}, []string{"alpha", "beta"})
	m.newTaskAssignee = "a"
	m.newTaskTagInput = "a"

	m.newTaskAssigneeSuggIdx = 1
	assignee := ansi.Strip(strings.Join(m.renderNewTaskAssigneeSuggestions(), "\n"))
	if !strings.Contains(assignee, "▸ @carla") {
		t.Errorf("render assignee = %q, want el segundo marcado", assignee)
	}

	m.newTaskTagSuggIdx = 0
	tags := ansi.Strip(strings.Join(m.renderTagFieldSuggestions(), "\n"))
	if !strings.Contains(tags, "▸ alpha") {
		t.Errorf("render tags = %q, want el primero marcado", tags)
	}
}

// --- Render: modal completo ----------------------------------------------

// El modal dibuja las secciones siempre, pero los bloques condicionales
// (error, sugerencias) sólo cuando toca. Cada rama es una ruta del render.
func TestRenderNewTaskModalSections(t *testing.T) {
	t.Run("prioridad enfocada", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldPriority
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "▸ Priority") {
			t.Errorf("want la fila Priority enfocada:\n%s", got)
		}
		if !strings.Contains(got, "○ none") {
			t.Errorf("want los radios de prioridad:\n%s", got)
		}
	})

	t.Run("descripción vacía y sin foco", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldTitle
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "(empty · Tab to edit)") {
			t.Errorf("want el placeholder de descripción:\n%s", got)
		}
	})

	t.Run("descripción enfocada sin contenido no muestra placeholder", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldDescription
		got := ansi.Strip(m.renderNewTaskModal(""))

		if strings.Contains(got, "(empty · Tab to edit)") {
			t.Errorf("con el foco en la descripción no debe pedir tabular:\n%s", got)
		}
		if !strings.Contains(got, "▸ Description") {
			t.Errorf("want la sección Description enfocada:\n%s", got)
		}
	})

	t.Run("error inline visible", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskFieldIdx = newTaskFieldTitle
		m.newTaskErr = "Title is required"
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "⚠ Title is required") {
			t.Errorf("want el error inline:\n%s", got)
		}
	})

	t.Run("sin error no hay línea de error", func(t *testing.T) {
		m := openNewTask(t)
		m.newTaskErr = ""
		got := ansi.Strip(m.renderNewTaskModal(""))

		if strings.Contains(got, "⚠") {
			t.Errorf("sin error no debe haber línea de aviso:\n%s", got)
		}
	})

	t.Run("cursor del título sólo en su campo", func(t *testing.T) {
		m := openNewTask(t)

		m.newTaskFieldIdx = newTaskFieldTitle
		withCursor := ansi.Strip(m.renderNewTaskModal(""))

		m.newTaskFieldIdx = newTaskFieldPriority
		withoutCursor := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(withCursor, cursorGlyph) {
			t.Errorf("con el foco en el título debe verse el cursor:\n%s", withCursor)
		}
		if strings.Contains(withoutCursor, cursorGlyph) {
			t.Errorf("sin el foco en el título no debe verse el cursor:\n%s", withoutCursor)
		}
	})

	t.Run("cursor del assignee sólo en su campo", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, []string{"@ana"}, nil)
		m.newTaskOpen = true
		m.newTaskProject = "api"
		m.newTaskFieldIdx = newTaskFieldAssignee
		m.newTaskAssignee = ""

		got := ansi.Strip(m.renderNewTaskModal(""))
		_ = got
		if !strings.Contains(got, "▸ Assignee") {
			t.Errorf("want la fila Assignee enfocada:\n%s", got)
		}
		if !strings.Contains(got, "@ana") {
			t.Errorf("con el foco en Assignee deben listarse las sugerencias:\n%s", got)
		}

		m.newTaskFieldIdx = newTaskFieldTitle
		got = ansi.Strip(m.renderNewTaskModal(""))
		if strings.Contains(got, "@ana") {
			t.Errorf("sin el foco no debe listarse el desplegable:\n%s", got)
		}
	})

	t.Run("sugerencias de tags sólo en su campo", func(t *testing.T) {
		m := modelWithPeopleAndTags(t, nil, []string{"alpha", "beta"})
		m.newTaskOpen = true
		m.newTaskProject = "api"
		m.newTaskTagInput = "a" // matchea, así el desplegable tiene contenido

		m.newTaskFieldIdx = newTaskFieldTags
		got := ansi.Strip(m.renderNewTaskModal(""))
		if !strings.Contains(got, "▸ Tags") {
			t.Errorf("want la fila Tags enfocada:\n%s", got)
		}
		if !strings.Contains(got, "alpha") {
			t.Errorf("con el foco en Tags deben listarse las sugerencias:\n%s", got)
		}

		m.newTaskFieldIdx = newTaskFieldTitle
		got = ansi.Strip(m.renderNewTaskModal(""))
		if strings.Contains(got, "alpha") {
			t.Errorf("sin el foco no debe listarse el desplegable de tags:\n%s", got)
		}
	})

	t.Run("el badge optional aparece en Tags", func(t *testing.T) {
		m := openNewTask(t)
		got := ansi.Strip(m.renderNewTaskModal(""))

		if !strings.Contains(got, "optional") {
			t.Errorf("want el badge optional:\n%s", got)
		}
	})
}

// El modal se recorta al ancho disponible; el ancho interior es el total menos
// los dos bordes, y de eso depende que nada se desborde.
func TestRenderNewTaskModalFitsWidth(t *testing.T) {
	for _, width := range []int{120, 80, 70, 66, 40, 20} {
		m := openNewTask(t)
		m.width = width
		m.newTaskFieldIdx = newTaskFieldTags
		m.newTaskTags = []string{"una-etiqueta-muy-larga-de-verdad-que-no-cabe"}

		rendered := m.renderNewTaskModal("")
		for i, line := range strings.Split(ansi.Strip(rendered), "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width=%d: línea %d de %d columnas excede el terminal:\n%q", width, i, w, line)
			}
		}
	}
}

// El contenido que no cabe se RECORTA, no se envuelve. Es lo que hace que el
// alto del modal no dependa del contenido: con un ancho de recorte mayor que el
// interior, la fila de tags se parte en dos líneas y el modal crece.
func TestRenderNewTaskModalTruncatesInsteadOfWrapping(t *testing.T) {
	for _, width := range []int{120, 80, 64, 50, 40} {
		short := openNewTask(t)
		short.width = width
		short.newTaskFieldIdx = newTaskFieldTags
		short.newTaskTags = []string{"api"}

		long := openNewTask(t)
		long.width = width
		long.newTaskFieldIdx = newTaskFieldTags
		long.newTaskTags = []string{"una-etiqueta-muy-larga-de-verdad-que-no-cabe-para-nada"}

		shortLines := len(strings.Split(short.renderNewTaskModal(""), "\n"))
		longLines := len(strings.Split(long.renderNewTaskModal(""), "\n"))
		if shortLines != longLines {
			t.Errorf("width=%d: el modal pasa de %d a %d líneas con una tag larga; debe recortar, no envolver",
				width, shortLines, longLines)
		}
	}
}

// --- Sincronía del foco con el textarea ----------------------------------

// El textarea sólo recibe el foco cuando es el campo activo; si no, se desenfoca
// para que no capture teclas.
func TestNewTaskSyncFocusFollowsActiveField(t *testing.T) {
	m := openNewTask(t)

	m.newTaskFieldIdx = newTaskFieldDescription
	if cmd := m.newTaskSyncFocus(); cmd == nil {
		t.Error("enfocar la descripción debe emitir un comando de focus")
	}

	m.newTaskFieldIdx = newTaskFieldTitle
	if cmd := m.newTaskSyncFocus(); cmd != nil {
		t.Error("al salir de la descripción el textarea debe quedar desenfocado (sin cmd)")
	}
}

// El tabulador desde Tags confirma la tag a medio tipear en vez de perderla.
func TestNewTaskMoveFieldCommitsPendingTag(t *testing.T) {
	m := openNewTask(t)
	m.newTaskFieldIdx = newTaskFieldTags
	m.newTaskTagInput = "pendiente"

	m, _ = ntKey(m, "tab")

	if len(m.newTaskTags) != 1 || m.newTaskTags[0] != "pendiente" {
		t.Errorf("tags = %v, want [pendiente]", m.newTaskTags)
	}
	if m.newTaskFieldIdx != newTaskFieldPriority {
		t.Errorf("campo = %d, want priority tras el wrap", m.newTaskFieldIdx)
	}
}
