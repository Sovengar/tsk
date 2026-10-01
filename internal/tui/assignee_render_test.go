package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/config"
	"tsk/internal/db"
)

// El modal de assignees tiene tres listas con tope de filas: el roster, las
// tareas activas de la persona y sus off-days. Los tres topes son independientes
// y cada uno añade una línea al render, así que los tres se comprueban por
// separado contra el mismo fixture.

// Con n assignees el roster cabe entero y no lleva pie de paginación; con uno más
// de los que caben, lleva pie. El pie aparece exactamente cuando la lista no
// entra, que es el borde que importa.
func TestAssigneeListFooterOnlyWhenRosterOverflows(t *testing.T) {
	// Los tamaños son del roster, que además de las personas metidas incluye a
	// "Me": el pie aparece cuando el roster no cabe, no cuando no caben los
	// nombres que-configurationarlo.
	tests := []struct {
		name      string
		roster    int
		wantPager bool
	}{
		{"caben todos", assigneeModalMaxRows, false},
		{"uno más de los que caben", assigneeModalMaxRows + 1, true},
		{"muchos más", assigneeModalMaxRows + 20, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAssigneeModel(t, tt.roster-1) // -1 por "Me"
			out := ansi.Strip(m.renderAssigneeModal(""))
			if got := hasPagerLine(out); got != tt.wantPager {
				t.Errorf("con un roster de %d el pie %v, want %v\n%s",
					tt.roster, got, tt.wantPager, out)
			}
		})
	}
}

// El pie dice en qué posición está el cursor sobre cuántas hay, con el índice
// más uno.
func TestAssigneeListPagerShowsSelectionPosition(t *testing.T) {
	m := newAssigneeModel(t, assigneeModalMaxRows+5)
	m.assigneeIdx = 4
	out := ansi.Strip(m.renderAssigneeModal(""))

	// El total es el del roster, que además de los nombres metidos incluye a
	// "Me": el pie no cuenta sólo las filas que el test añadió.
	want := fmt.Sprintf("%d/%d", m.assigneeIdx+1, len(m.assigneeRoster()))
	if !strings.Contains(out, want) {
		t.Errorf("el pie no dice %q\n%s", want, out)
	}
}

// La ventana del roster sigue al cursor: con la selección al final se ve la
// última persona, no las primeras.
func TestAssigneeListWindowFollowsCursor(t *testing.T) {
	m := newAssigneeModel(t, assigneeModalMaxRows+5)
	ultimo := m.assigneeRoster()[assigneeModalMaxRows+4].Name
	m.assigneeIdx = assigneeModalMaxRows + 4

	out := ansi.Strip(m.renderAssigneeModal(""))
	if !strings.Contains(out, ultimo) {
		t.Errorf("con el cursor al final no se ve %q\n%s", ultimo, out)
	}
}

// Las tareas de la persona están topadas a assigneeModalTaskRows y las que
// sobran se resumen en "... N more", con el número exacto de las que no caben.
func TestAssigneeDetailTaskOverflow(t *testing.T) {
	tests := []struct {
		name     string
		tasks    int
		wantMore bool
		wantN    int
	}{
		{"caban todas", assigneeModalTaskRows, false, 0},
		{"una de más", assigneeModalTaskRows + 1, true, 1},
		{"diez de más", assigneeModalTaskRows + 10, true, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAssigneeModel(t, 0)
			mustCreateTask(t, m.database, "api", "de @user00", "", "@user00", 1, "todo")
			for range tt.tasks - 1 {
				mustCreateTask(t, m.database, "api", "tarea", "", "@user00", 1, "todo")
			}
			reloadTasks(t, m)
			m.assigneeIdx = 0
			m.assigneeDetail = true

			out := ansi.Strip(m.renderAssigneeModal(""))
			more := strings.Contains(out, "... "+fmt.Sprint(tt.wantN)+" more")
			if more != tt.wantMore {
				t.Errorf("con %d tareas el resumen %v, want %v\n%s", tt.tasks, more, tt.wantMore, out)
			}
		})
	}
}

// Una persona sin tareas activas y sin off-days pone "(none)" en las dos
// secciones. "Me" es quien cumple eso en el fixture: las tareas del test todas
// tienen responsable.
func TestAssigneeDetailNoTasksNoOffdays(t *testing.T) {
	m := newAssigneeModel(t, 2)
	m.assigneeIdx = m.meIndex(t)
	m.assigneeDetail = true

	out := ansi.Strip(m.renderAssigneeModal(""))
	if n := strings.Count(out, "(none)"); n != 2 {
		t.Errorf("la persona sin nada tiene %d apariciones de (none), want 2\n%s", n, out)
	}
}

// Los off-days también:topados, con su propio pie.
func TestAssigneeDetailOffdayOverflow(t *testing.T) {
	tests := []struct {
		name      string
		offs      int
		wantPager bool
	}{
		{"caban todos", assigneeModalOffRows, false},
		{"uno más", assigneeModalOffRows + 1, true},
		{"muchos más", assigneeModalOffRows + 15, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newAssigneeModel(t, 1)
			for i := range tt.offs {
				dia := fmt.Sprintf("2026-01-%02d", (i%28)+1)
				mustAddOffDay(t, m.database, "@user00", dia, dia, "")
			}
			reloadOffDays(t, m)
			m.assigneeDetail = true

			out := ansi.Strip(m.renderAssigneeModal(""))
			if got := hasPagerLine(out); got != tt.wantPager {
				t.Errorf("con %d off-days el pie %v, want %v\n%s", tt.offs, got, tt.wantPager, out)
			}
		})
	}
}

// Con tareas activas pero sin off-days, sólo la sección de off-days dice
// "(none)". La de tareas tiene su propio marcador y no debe aparecer.
func TestAssigneeDetailNoOffdays(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	m.assigneeDetail = true

	out := ansi.Strip(m.renderAssigneeModal(""))
	if n := strings.Count(out, "(none)"); n != 1 {
		t.Errorf("con tareas y sin off-days hay %d apariciones de (none), want 1\n%s", n, out)
	}
	if strings.Contains(out, "Active tasks") && strings.Count(out, "#") == 0 {
		t.Errorf("la sección de tareas salió vacía sin decir (none):\n%s", out)
	}
}

// Una nota de off-day larga es lo único del modal que no viene ya recortado por
// su campo, así que es lo que hace trabajar al recorte del modal. La línea tiene
// que acabar justo en el ancho interior.
func TestAssigneeDetailTruncatesLongNoteToInnerWidth(t *testing.T) {
	m := newAssigneeModel(t, 1)
	m.assigneeIdx = 0
	mustAddOffDay(t, m.database, "@user00", "2026-05-01", "2026-05-02", strings.Repeat("n", 200))
	reloadOffDays(t, m)
	m.assigneeDetail = true

	out := ansi.Strip(m.renderAssigneeModal(""))
	ancho := modalInnerWidth(modalWidthFor(58, m.width))
	got := widestBoxLine(out, "nnn")
	if got == 0 {
		t.Fatalf("no se encontró la línea con la nota:\n%s", out)
	}
	if got != ancho {
		t.Errorf("la línea con la nota mide %d, want %d (el ancho interior del modal)", got, ancho)
	}
}

// Borrar un off-day que falla viaja en el mensaje con el error, para que el
// formulario se reabra en vez de cerrar como si hubiera ido bien.
func TestDeleteOffDayCmdReportsError(t *testing.T) {
	m := newAssigneeModel(t, 1)
	msg, ok := m.deleteOffDayCmd(9999, "@user00")().(offdaySavedMsg)
	if !ok {
		t.Fatal("no devolvió offdaySavedMsg")
	}
	if msg.err == nil {
		t.Error("borrar un id inexistente no reportó error")
	}
	if msg.action != "delete" || msg.name != "@user00" {
		t.Errorf("mensaje %+v, want action=delete name=@user00", msg)
	}
}

// Y el camino feliz no lleva error, y el off-day desaparece.
func TestDeleteOffDayCmdSuccess(t *testing.T) {
	m := newAssigneeModel(t, 1)
	off, err := m.database.AddOffDay("@user00", "2026-03-01", "2026-03-02", "vacaciones")
	if err != nil {
		t.Fatal(err)
	}

	msg, ok := m.deleteOffDayCmd(off.ID, "@user00")().(offdaySavedMsg)
	if !ok {
		t.Fatal("no devolvió offdaySavedMsg")
	}
	if msg.err != nil {
		t.Errorf("borrar un off-day existente falló: %v", msg.err)
	}
	rest, _ := m.database.ListOffDays("@user00")
	if len(rest) != 0 {
		t.Errorf("quedaron %d off-days, want 0", len(rest))
	}
}

// newAssigneeModel abre el modal con n assignees, cada uno con una tarea activa.
func newAssigneeModel(t *testing.T, assignees int) *Model {
	t.Helper()
	if assignees == 0 {
		return newEmptyDBModel(t)
	}
	names := make([]string, assignees)
	for i := range assignees {
		names[i] = fmt.Sprintf("@user%02d", i)
	}
	m := newEmptyDBModel(t)
	for _, name := range names {
		mustCreateTask(t, m.database, "api", "tarea de "+name, "", name, 1, "todo")
	}
	reloadTasks(t, m)
	m.assigneeModalOpen = true
	m.clampAssigneeIdx()
	return m
}

// newEmptyDBModel parte de una base de datos sin nada, que es lo que hace falta
// para el roster vacío: el fixture normal trae tareas con responsables.
func newEmptyDBModel(t *testing.T) *Model {
	t.Helper()
	database, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	mustCreateProject(t, database, "api", nil)

	m := New(database, config.Defaults())
	m.width = 120
	m.height = 30
	m.assigneeModalOpen = true
	m.clampAssigneeIdx()
	return &m
}

// reloadTasks vuelve a leer la lista de tareas, como haría un tasksLoadedMsg.
func reloadTasks(t *testing.T, m *Model) {
	t.Helper()
	var err error
	if m.tasks, err = m.database.ListTasks("", "", ""); err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	m.filteredT = nil
}

// meIndex devuelve la posición de "Me" en el roster del modelo.
func (m *Model) meIndex(t *testing.T) int {
	t.Helper()
	for i, s := range m.assigneeRoster() {
		if s.Name == "Me" {
			return i
		}
	}
	t.Fatal("el roster no tiene a \"Me\"")
	return 0
}

// reloadOffDays vuelve a leer los off-days, como haría un offDaysLoadedMsg.
func reloadOffDays(t *testing.T, m *Model) {
	t.Helper()
	var err error
	if m.offdays, err = m.database.ListOffDays(""); err != nil {
		t.Fatalf("ListOffDays: %v", err)
	}
}

func mustAddOffDay(t *testing.T, database *db.DB, assignee, start, end, note string) {
	t.Helper()
	if _, err := database.AddOffDay(assignee, start, end, note); err != nil {
		t.Fatalf("AddOffDay(%s, %s): %v", assignee, start, err)
	}
}

// hasPagerLine busca el pie "n/total" de cualquiera de las listas con tope.
//
// Busca dentro del recuadro, no en la línea entera: la línea renderizada lleva
// el borde, el centrado y el relleno de la derecha, así que partla en campos
// whitespaces no encuentra nada. Sólo se acepta un "n/m" con ambos lados
// numéricos y nada más en el interior del borde, para no confundirlo con una
// fecha ("2026-01-01 → 2026-01-02", que también lleva una barra).
func hasPagerLine(rendered string) bool {
	for _, line := range strings.Split(rendered, "\n") {
		inicio := strings.Index(line, "│")
		if inicio < 0 {
			continue
		}
		resto := line[inicio+len("│"):]
		if fin := strings.Index(resto, "│"); fin >= 0 {
			resto = resto[:fin]
		}
		if isPager(resto) {
			return true
		}
	}
	return false
}

// isPager dice si un fragmento de línea es el pie "n/total" y nada más.
func isPager(fragment string) bool {
	fields := strings.Fields(fragment)
	if len(fields) != 1 || !strings.Contains(fields[0], "/") {
		return false
	}
	slash := strings.Index(fields[0], "/")
	return isAllDigits(fields[0][:slash]) && isAllDigits(fields[0][slash+1:])
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// widestBoxLine devuelve el ancho de la línea más ancha del recuadro del modal
// que contenga marker, midiendo lo que hay entre los dos bordes verticales.
func widestBoxLine(rendered, marker string) int {
	ancho := 0
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		inicio := strings.Index(line, "│")
		if inicio < 0 {
			continue
		}
		resto := line[inicio+len("│"):]
		if fin := strings.Index(resto, "│"); fin >= 0 {
			resto = resto[:fin]
		}
		if w := len([]rune(resto)); w > ancho {
			ancho = w
		}
	}
	return ancho
}
