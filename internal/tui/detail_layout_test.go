package tui

import (
	"strings"
	"testing"
)

// El reparto de alto del detalle son seis operaciones encadenadas con cuatro
// suelos distintos. Salía del render sin ninguna forma de comprobarla.

func TestDetailHeightBudget(t *testing.T) {
	tests := []struct {
		name                  string
		maxHeight, comments   int
		wantComment, wantDesc int
	}{
		// avail = maxHeight - 12, con suelo de 3. El presupuesto de comentarios
		// se lleva lo que haya hasta maxCommentLines = avail-2, y la descripción
		// se queda con el resto (mínimo 1).
		{"holgado con muchos comentarios", 40, 20, 20, 8},
		{"holgado con pocos comentarios", 40, 3, 3, 25},
		{"holgado sin comentarios", 40, 0, 1, 27},
		{"holgado con 25 comentarios", 25, 4, 4, 9},

		// El presupuesto de comentarios no puede pasar del espacio reservado:
		// con avail=3 sólo cabe 1 aunque haya 20.
		{"muchos comentarios en poco alto", 15, 20, 1, 2},

		// avail tiene un suelo de 3: por debajo, la caja sigue teniendo sentido.
		{"altura mínima", 12, 0, 1, 2},
		{"altura cero", 0, 5, 1, 2},
		{"altura negativa", -10, 5, 1, 2},

		// Un comentario siempre tiene al menos una línea (o el "(no comments)").
		{"cero comentarios da 1", 30, 0, 1, 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotC, gotD := detailHeightBudget(tt.maxHeight, tt.comments)
			if gotC != tt.wantComment || gotD != tt.wantDesc {
				t.Errorf("detailHeightBudget(%d, %d) = (%d,%d), want (%d,%d)",
					tt.maxHeight, tt.comments, gotC, gotD, tt.wantComment, tt.wantDesc)
			}
		})
	}
}

// Los dos presupuestos son 1 como mínimo y la suma nunca excede avail.
func TestDetailHeightBudgetInvariants(t *testing.T) {
	for h := -5; h <= 60; h++ {
		for comments := -2; comments <= 40; comments++ {
			c, d := detailHeightBudget(h, comments)
			if c < 1 {
				t.Fatalf("h=%d comments=%d: presupuesto de comentarios %d, want >= 1", h, comments, c)
			}
			if d < 1 {
				t.Fatalf("h=%d comments=%d: presupuesto de descripción %d, want >= 1", h, comments, d)
			}
			avail := max(h-12, 3)
			if c+d > avail {
				t.Fatalf("h=%d comments=%d: %d+%d excede avail=%d", h, comments, c, d, avail)
			}
		}
	}
}

// Más comentarios nunca quitan presupuesto a la descripción por debajo de 1.
func TestDetailHeightBudgetCommentsMonotonic(t *testing.T) {
	for h := 13; h <= 60; h++ {
		prevDesc := 1 << 30
		for comments := 0; comments <= 40; comments++ {
			_, d := detailHeightBudget(h, comments)
			if d > prevDesc {
				t.Fatalf("h=%d: con %d comentarios la descripción=%d, más que antes (%d)", h, comments, d, prevDesc)
			}
			prevDesc = d
		}
	}
}

// truncateAt: recorte por bytes, que es lo que corresponde a un timestamp ASCII.
func TestTruncateAt(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"más corto que el límite", "abc", 5, "abc"},
		{"exactamente el límite", "abcde", 5, "abcde"},
		{"un byte de más", "abcdef", 5, "abcde"},
		{"mucho de más", strings.Repeat("x", 40), 5, "xxxxx"},
		{"límite cero", "abc", 0, ""},
		{"límite negativo devuelve vacío, no revienta", "abc", -1, ""},
		{"límite muy negativo", "abc", -99, ""},
		{"vacío", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateAt(tt.s, tt.n); got != tt.want {
				t.Errorf("truncateAt(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// Los formateadores de fecha: vacío siempre es el guion largo, y el resto se
// recorta al formato pedido.
func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"vacío", "", "—"},
		// 19 bytes: "2026-09-30T18:27:52" entra entero, la Z final se cae.
		{"completo con zona", "2026-09-30T18:27:52Z", "2026-09-30T18:27:52"},
		{"de 19 exacto", "2026-09-30T18:27:52", "2026-09-30T18:27:52"},
		{"corto", "2026-09-30", "2026-09-30"},
		{"basura corta", "x", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTime(tt.in); got != tt.want {
				t.Errorf("formatTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// formatCommentTime cambia la "T" por un espacio y recorta a 16: fecha y hora
// hasta el minuto.
func TestFormatCommentTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"vacío", "", "—"},
		{"completo", "2026-09-30T18:27:52Z", "2026-09-30 18:27"},
		{"sin zona", "2026-09-30T18:27:52", "2026-09-30 18:27"},
		{"de 16 exacto", "2026-09-30 18:27", "2026-09-30 18:27"},
		{"corto", "2026", "2026"},
		{"sin T", "2026-09-30", "2026-09-30"},
		{"sólo espacios", "   ", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCommentTime(tt.in); got != tt.want {
				t.Errorf("formatCommentTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Sólo se cambia la PRIMERA "T". Aquí la segunda sobrevive al recorte y sigue
// siendo una T, lo que demuestra que el Replace tiene conteo 1.
func TestFormatCommentTimeReplacesOnlyFirstT(t *testing.T) {
	got := formatCommentTime("T2026-09-30T18:2")
	if got != " 2026-09-30T18:2" {
		t.Errorf("formatCommentTime = %q, want \" 2026-09-30T18:2\" (sólo la 1ª T)", got)
	}
	if strings.Count(got, "T") != 1 {
		t.Errorf("got %q, want una sola T: la segunda no se sustituye", got)
	}
}

// formatCompleted es formatTime: el caso vacío no necesita su propia rama.
func TestFormatCompletedEqualsFormatTime(t *testing.T) {
	for _, in := range []string{
		"", "2026-09-30T18:27:52Z", "2026-09-30", "corto", "x", "2026-09-30T18:27:5",
	} {
		if got, want := formatCompleted(in), formatTime(in); got != want {
			t.Errorf("formatCompleted(%q) = %q pero formatTime = %q", in, got, want)
		}
	}
}

func TestFormatCompletedEmpty(t *testing.T) {
	if got := formatCompleted(""); got != "—" {
		t.Errorf("formatCompleted(\"\") = %q, want —", got)
	}
}
