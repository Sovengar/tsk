package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// taskByTitle busca una tarea de prueba por título.
func taskByTitle(t *testing.T, m *Model, title string) *model.Task {
	t.Helper()
	for i := range m.tasks {
		if m.tasks[i].Title == title {
			return &m.tasks[i]
		}
	}
	t.Fatalf("tarea %q no encontrada", title)
	return nil
}

// openDetail abre el modal de detalle para la tarea dada.
func openDetail(m *Model, task *model.Task) {
	cp := *task
	m.detailOpen = true
	m.detailTask = &cp
	comments, _ := m.database.ListComments(task.ID)
	m.detailComments = comments
	m.detailCommentSel = -1
}

// applyMsg aplica un mensaje al modelo, ejecuta el comando resultante y
// aplica también su mensaje (un nivel), de forma que los flujos async se
// resuelven en tests.
func applyMsg(m *Model, msg tea.Msg) *Model {
	next, cmd := m.Update(msg)
	m = asModel(next)
	if cmd != nil {
		if out := cmd(); out != nil {
			next2, _ := m.Update(out)
			m = asModel(next2)
		}
	}
	return m
}

func asModel(v tea.Model) *Model {
	switch t := v.(type) {
	case *Model:
		return t
	case Model:
		return &t
	default:
		return nil
	}
}

func TestDetailCommentSelection(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "uno")
	mustAddComment(t, m.database, task.ID, "dos")
	openDetail(m, task)

	if m.detailCommentSel != -1 {
		t.Fatalf("initial sel = %d, want -1", m.detailCommentSel)
	}

	// j entra en la lista y selecciona el primero
	m, _ = press(m, "j")
	if m.detailCommentSel != 0 {
		t.Errorf("after j: sel = %d, want 0", m.detailCommentSel)
	}
	m, _ = press(m, "j")
	if m.detailCommentSel != 1 {
		t.Errorf("after 2x j: sel = %d, want 1", m.detailCommentSel)
	}
	// j en el último no pasa de largo
	m, _ = press(m, "j")
	if m.detailCommentSel != 1 {
		t.Errorf("j at bottom: sel = %d, want 1", m.detailCommentSel)
	}
	// k sube
	m, _ = press(m, "k")
	if m.detailCommentSel != 0 {
		t.Errorf("after k: sel = %d, want 0", m.detailCommentSel)
	}
	// k en el primero deselecciona
	m, _ = press(m, "k")
	if m.detailCommentSel != -1 {
		t.Errorf("k at top: sel = %d, want -1", m.detailCommentSel)
	}
}

func TestDetailDeleteComment(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "uno")
	mustAddComment(t, m.database, task.ID, "dos")
	openDetail(m, task)

	// Selecciona el primero y lo borra con d
	m, _ = press(m, "j")
	m, cmd := press(m, "d")
	if !m.detailOpen {
		t.Error("el detalle debe seguir abierto al borrar un comentario")
	}
	if cmd == nil {
		t.Fatal("d con comentario seleccionado debe producir un comando")
	}

	m = applyMsg(m, cmd())
	if len(m.detailComments) != 1 {
		t.Fatalf("comments = %d, want 1", len(m.detailComments))
	}
	if m.detailComments[0].Body != "dos" {
		t.Errorf("quedó %q, want dos", m.detailComments[0].Body)
	}
	if m.detailCommentSel != 0 {
		t.Errorf("sel tras borrar = %d, want 0", m.detailCommentSel)
	}

	stored, _ := m.database.ListComments(task.ID)
	if len(stored) != 1 {
		t.Errorf("comentarios en DB = %d, want 1", len(stored))
	}
}

func TestDetailDeleteLastCommentClearsSelection(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "único")
	openDetail(m, task)

	m, _ = press(m, "j")
	m, cmd := press(m, "d")
	m = applyMsg(m, cmd())

	if len(m.detailComments) != 0 {
		t.Fatalf("comments = %d, want 0", len(m.detailComments))
	}
	if m.detailCommentSel != -1 {
		t.Errorf("sel = %d, want -1", m.detailCommentSel)
	}
}

func TestDetailDoneWithoutCommentSelection(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "nota")
	openDetail(m, task)

	// Sin selección, d marca done y cierra el modal.
	m, cmd := press(m, "d")
	if m.detailOpen {
		t.Error("d sin selección debe cerrar el detalle")
	}
	if cmd == nil {
		t.Fatal("d sin selección debe producir el comando Done")
	}
	applyMsg(m, cmd())

	stored, _ := m.database.GetTask(task.ID)
	if stored.Status != "done" {
		t.Errorf("status = %q, want done", stored.Status)
	}
}

func TestDetailEscDeselectsThenCloses(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "nota")
	openDetail(m, task)

	m, _ = press(m, "j")
	if m.detailCommentSel != 0 {
		t.Fatalf("sel = %d, want 0", m.detailCommentSel)
	}

	// Primer Esc deselecciona
	m, _ = press(m, "esc")
	if m.detailCommentSel != -1 {
		t.Errorf("esc 1: sel = %d, want -1", m.detailCommentSel)
	}
	if !m.detailOpen {
		t.Error("esc 1 no debe cerrar el modal")
	}

	// Segundo Esc cierra
	m, _ = press(m, "esc")
	if m.detailOpen {
		t.Error("esc 2 debe cerrar el modal")
	}
}

func TestRenderDetailCommentWindowRespectsHeight(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	for i := 0; i < 20; i++ {
		mustAddComment(t, m.database, task.ID, "comentario "+string(rune('A'+i)))
	}
	openDetail(m, task)
	m.detailCommentSel = 15

	const budget = 20
	out := m.renderDetail(task, budget)

	if n := lineCount(out); n > budget {
		t.Errorf("alto = %d, excede el presupuesto %d", n, budget)
	}
	// La ventana debe seguir a la selección.
	if !strings.Contains(ansi.Strip(out), "comentario P") {
		t.Errorf("el comentario seleccionado (#15) no está visible:\n%s", ansi.Strip(out))
	}
}

func TestRenderDetailNoComments(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Add caching")
	openDetail(m, task)

	out := ansi.Strip(m.renderDetail(task, 24))
	if !strings.Contains(out, "Comments (0)") {
		t.Errorf("falta el título de la caja de comentarios:\n%s", out)
	}
	if !strings.Contains(out, "(no comments)") {
		t.Errorf("falta el placeholder de comentarios:\n%s", out)
	}
}

// TestRenderDetailTwoBoxes verifica que el detalle dibuja dos cajas con borde
// redondeado: tarea+descripción y comentarios.
func TestRenderDetailTwoBoxes(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	openDetail(m, task)

	out := ansi.Strip(m.renderDetail(task, 24))
	if n := strings.Count(out, "╭"); n != 2 {
		t.Errorf("bordes superiores = %d, want 2:\n%s", n, out)
	}
	if n := strings.Count(out, "╰"); n != 2 {
		t.Errorf("bordes inferiores = %d, want 2:\n%s", n, out)
	}
	if !strings.Contains(out, "Description:") {
		t.Errorf("la caja de tarea debe contener la descripción:\n%s", out)
	}
}

// TestRenderDetailNoActionsLine verifica que el detalle ya no repite los
// keybinds en una línea de acciones (viven en la KeybindsBar).
func TestRenderDetailNoActionsLine(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	openDetail(m, task)

	out := ansi.Strip(m.renderDetail(task, 24))
	if strings.Contains(out, "New comment") {
		t.Errorf("el detalle no debe repetir los keybinds:\n%s", out)
	}
}

// TestRenderDetailIndentsDescription verifica que la descripción quede
// indentada igual que la metadata.
func TestRenderDetailIndentsDescription(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	taskCopy := *task
	taskCopy.Description = "no entiendo"
	openDetail(m, &taskCopy)

	out := ansi.Strip(m.renderDetail(&taskCopy, 24))
	if !strings.Contains(out, "  no entiendo") {
		t.Errorf("la descripción debe ir indentada con 2 espacios:\n%s", out)
	}
}

// TestDetailHidesPreview verifica que al abrir el detalle desaparece la caja
// de preview (la descripción ya se muestra dentro del modal).
func TestDetailHidesPreview(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")

	withPreview := ansi.Strip(m.View().Content)
	if !strings.Contains(withPreview, " Description ") {
		t.Fatalf("sin detalle debería verse el preview:\n%s", withPreview)
	}

	m, _ = press(m, "enter")
	withDetail := ansi.Strip(m.View().Content)
	if strings.Contains(withDetail, " Description ") {
		t.Errorf("con el detalle abierto no debe verse el preview:\n%s", withDetail)
	}
	if !strings.Contains(withDetail, "Description:") {
		t.Errorf("el detalle debe mostrar su propia descripción:\n%s", withDetail)
	}
}

// TestDetailViewHasThreeBoxes verifica que la vista del detalle muestra tres
// cajas con borde redondeado: tarea+descripción, comentarios y keybinds.
func TestDetailViewHasThreeBoxes(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m, _ = press(m, "enter")

	out := ansi.Strip(m.View().Content)
	if n := strings.Count(out, "╭"); n != 3 {
		t.Errorf("cajas con borde = %d, want 3 (tarea, comentarios, keybinds):\n%s", n, out)
	}
	if !strings.Contains(out, "Keybinds · Detail") {
		t.Errorf("falta la caja de keybinds del detalle:\n%s", out)
	}
}

func TestCommentCmdEmptyBodyAddsNothing(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	openDetail(m, task)

	// Un comentario vacío no debe crear nada.
	m = applyMsg(m, commentFinishedMsg{taskID: task.ID, body: ""})
	comments, _ := m.database.ListComments(task.ID)
	if len(comments) != 0 {
		t.Errorf("comentarios = %d, want 0", len(comments))
	}
}

func TestCommentAddedSelectsNewest(t *testing.T) {
	m := newTestModel(t)
	task := taskByTitle(t, m, "Fix N+1 query")
	mustAddComment(t, m.database, task.ID, "viejo")
	openDetail(m, task)

	m = applyMsg(m, commentFinishedMsg{taskID: task.ID, body: "nuevo"})
	if len(m.detailComments) != 2 {
		t.Fatalf("comments = %d, want 2", len(m.detailComments))
	}
	if m.detailCommentSel != 1 {
		t.Errorf("sel = %d, want 1 (el nuevo)", m.detailCommentSel)
	}
	if m.detailComments[1].Body != "nuevo" {
		t.Errorf("último = %q, want nuevo", m.detailComments[1].Body)
	}
}
