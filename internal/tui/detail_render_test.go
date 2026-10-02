package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// El modal de detalle reparte su alto entre comentarios y descripción, recorta
// ambos al ancho de la caja y marca el comentario seleccionado. Los tests que lo
// cubrían miraban si aparecía un texto; lo que hay que mirar es cuánta gente hay
// y dónde.
//
// Los asserts son de ancho y de posición: el texto sale entero en cuanto cabe, y
// una línea que se recorta se nota en su longitud.

// newDetailWithTags abre el detalle de una tarea con las tags dadas.
func newDetailWithTags(t *testing.T, tags ...string) *Model {
	t.Helper()
	m := newTestModel(t)
	task := m.tasks[0]
	task.Tags = tags
	m.detailOpen = true
	m.detailTask = &task
	m.detailComments = nil
	m.detailCommentSel = -1
	m.width = 100
	return m
}

// detailRender devuelve el detalle sin colores.
func detailRender(t *testing.T, m *Model, h int) string {
	t.Helper()
	return ansi.Strip(m.renderDetail(m.detailTask, h))
}

// Sin tags, la metadata pone un guion largo en vez de una lista vacía, que se
// vería como un hueco.
func TestDetailTagsDashWhenNone(t *testing.T) {
	m := newDetailWithTags(t)
	out := detailRender(t, m, 40)
	if !strings.Contains(out, "Tags:") {
		t.Fatalf("no sale la línea de tags:\n%s", out)
	}
	for _, linea := range strings.Split(out, "\n") {
		if !strings.Contains(linea, "Tags:") {
			continue
		}
		if !strings.Contains(linea, "—") {
			t.Errorf("sin tags la línea no lleva el guion largo: %q", linea)
		}
		return
	}
}

func TestDetailTagsJoined(t *testing.T) {
	m := newDetailWithTags(t, "uno", "dos", "tres")
	out := detailRender(t, m, 40)
	if !strings.Contains(out, "uno, dos, tres") {
		t.Errorf("las tags no salen unidas por comas:\n%s", out)
	}
}

// Una descripción larga se recorta, y la línea recortada llega al ancho interior
// de la caja. Con altura de sobra sale entera.
func TestDetailDescriptionIsClippedToBox(t *testing.T) {
	larga := strings.Repeat("palabra ", 200)
	m := newDetailWithTags(t)
	m.detailTask.Description = larga

	ancho := modalInnerWidth(m.width)

	// Con altura de sobra la descripción no se recorta en vertical, pero cada
	// línea sí se recorta en horizontal al ancho de la caja.
	out := detailRender(t, m, 200)
	masLarga := 0
	for _, linea := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(linea); w > masLarga {
			masLarga = w
		}
	}
	if masLarga > ancho+2 {
		t.Errorf("una línea mide %d, más que el ancho interior %d de la caja", masLarga, ancho)
	}
}

// Con poca altura, la descripción se recorta en vertical: entran menos líneas que
// palabras tiene.
func TestDetailDescriptionBudgetGrowsWithHeight(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailTask.Description = strings.Repeat("palabra\n", 100)

	corta := strings.Count(detailRender(t, m, 20), "palabra")
	larga := strings.Count(detailRender(t, m, 80), "palabra")
	if corta >= larga {
		t.Errorf("con 20 de alto salen %d líneas de descripción y con 80 salen %d: el alto no acota",
			corta, larga)
	}
	if corta == 0 {
		t.Error("con 20 de alto no sale ninguna línea de descripción")
	}
}

// El comentario seleccionado lleva un "> " delante y el resto "  ". Es lo que
// distingue la selección del resto de las marcas del modal.
func TestDetailCommentMarkerFollowsSelection(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{
		{ID: 1, Body: "primero", CreatedAt: "2026-01-01T10:00:00Z"},
		{ID: 2, Body: "segundo", CreatedAt: "2026-01-02T11:00:00Z"},
		{ID: 3, Body: "tercero", CreatedAt: "2026-01-03T12:00:00Z"},
	}
	m.width = 120
	m.clampOffdayIdx()

	for sel := 0; sel < 3; sel++ {
		m.detailCommentSel = sel
		out := detailRender(t, m, 60)
		esperado := "> " + formatCommentTime(m.detailComments[sel].CreatedAt)
		if !strings.Contains(out, esperado) {
			t.Errorf("con la selección en %d no sale %q:\n%s", sel, esperado, out)
		}
		// Sólo un "> " de comentario: el de la caja es "│", no "> ".
		if n := strings.Count(out, "> 20"); n != 1 {
			t.Errorf("con la selección en %d hay %d marcadores, want 1:\n%s", sel, n, out)
		}
	}
}

// Sin selección no hay ningún "> " en los comentarios.
func TestDetailNoMarkerWithoutSelection(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "uno", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 120

	out := detailRender(t, m, 60)
	if strings.Contains(out, "> "+formatCommentTime("2026-01-01T10:00:00Z")) {
		t.Errorf("sin selección sale un marcador:\n%s", out)
	}
}

// Un comentario multilínea se colapsa a una línea: el cuerpo entero no puede
// ocupar tres filas de la caja.
func TestDetailCommentBodyCollapsedToOneLine(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "uno\ndos\ntres", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 120

	out := detailRender(t, m, 60)
	if !strings.Contains(out, "uno dos tres") {
		t.Errorf("el cuerpo no se colapsó a una línea:\n%s", out)
	}
}

// Un comentario larguísimo se recorta al ancho de la caja: el modal entero mide
// lo que mide, sin desbordar por un cuerpo de texto.
func TestDetailLongCommentIsClipped(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{
		ID:        1,
		Body:      strings.Repeat("x", 400),
		CreatedAt: "2026-01-01T10:00:00Z",
	}}
	m.width = 80

	out := detailRender(t, m, 60)
	if !strings.Contains(out, "xxxx") {
		t.Fatalf("no sale el comentario:\n%s", out)
	}
	for _, linea := range strings.Split(out, "\n") {
		if !strings.Contains(linea, "xxxx") {
			continue
		}
		if w := ansi.StringWidth(linea); w > m.width {
			t.Errorf("la línea del comentario mide %d, más que el modal (%d)", w, m.width)
		}
	}
}

// El contenido se descentra hacia arriba: el relleno va sólo delante, en la mitad
// del alto que sobra. Abajo no hay nada -- ese hueco lo pone el layout que
// rodea el modal --, así que lo que se mide es el número de líneas de arriba, y
// tiene que ser la mitad del sobrante y no el sobrante entero.
func TestDetailPaddedDownByHalfTheSlack(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "uno", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 100

	const alto = 60
	out := detailRender(t, m, alto)
	lineas := strings.Split(out, "\n")

	arriba := 0
	for arriba < len(lineas) && strings.TrimSpace(lineas[arriba]) == "" {
		arriba++
	}
	if arriba == 0 {
		t.Fatalf("no hay relleno:\\n%q", out)
	}

	contenido := len(lineas) - arriba
	if want := (alto - contenido) / 2; arriba != want {
		t.Errorf("relleno %d líneas, want %d (la mitad de las %d que sobran)",
			arriba, want, alto-contenido)
	}
}

// Con el alto justo al tamaño del contenido no hay ni una línea de margen.
func TestDetailNotCentredWhenItFills(t *testing.T) {
	m := newDetailWithTags(t)
	m.width = 100

	out := detailRender(t, m, 3)
	if strings.HasPrefix(out, "\\n") {
		t.Errorf("con alto 3 hay margen superior:\\n%q", out)
	}
}

// Las cajas del detalle se dibujan al ancho completo del modal, bordes
// incluidos. El ancho interior es dos menos, y es lo que se usa para recortar el
// contenido.
func TestDetailBoxesShareWidth(t *testing.T) {
	m := newDetailWithTags(t)
	m.detailComments = []model.Comment{{ID: 1, Body: "uno", CreatedAt: "2026-01-01T10:00:00Z"}}
	m.width = 100

	out := detailRender(t, m, 60)
	ancho := 0
	for _, linea := range strings.Split(out, "\n") {
		i := strings.Index(linea, "╭")
		if i < 0 {
			continue
		}
		runes := []rune(linea[i:])
		for j, r := range runes {
			if r == '╮' && j+1 > ancho {
				ancho = j + 1
			}
		}
	}
	if want := m.width; ancho != want {
		t.Errorf("la caja más ancha mide %d, want %d (el ancho del modal con sus bordes)", ancho, want)
	}
}

// El límite de recorte del contenido es el ancho interior de la caja, dos menos
// que el ancho del modal.
//
// El conteo de filas es lo que lo distingue, no el ancho de la línea: la caja
// rellena con espacios hasta su borde, así que todas las líneas miden lo mismo
// se recorten donde se recorten. Con dos columnas de más -- un límite por encima
// del ancho interior -- lipgloss envuelve la línea y el modal crece una fila.
//
// Y conviene decir qué NO distingue este test: un límite más estrecho tampoco se
// ve, porque el contenido cabe de sobra en la caja y da igual que sobre espacio.
// Sólo el lado de arriba del ancho interior es observable, y es el lado que
// importa -- es el que hace que el modal crezca.
func TestDetailContentWrapRowCount(t *testing.T) {
	tests := []struct {
		name     string
		preparar func(*Model)
	}{
		{"descripción larga", func(m *Model) {
			m.detailTask.Description = strings.Repeat("palabra ", 200)
		}},
		{"comentario largo", func(m *Model) {
			m.detailComments = []model.Comment{{
				ID:        1,
				Body:      strings.Repeat("y", 400),
				CreatedAt: "2026-01-01T10:00:00Z",
			}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newDetailWithTags(t)
			tt.preparar(m)

			out := detailRender(t, m, 200)
			if got := len(strings.Split(out, "\n")); got != 107 {
				t.Errorf("el modal tiene %d filas, want 107", got)
			}
		})
	}
}
