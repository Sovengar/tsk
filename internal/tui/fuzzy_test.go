package tui

import (
	"strings"
	"testing"
)

// fuzzyScore decide qué suggestions se muestran en el autocompletado y en el
// modal de filtros. Es la pieza que más minion tenía sin cobertura y es puramente
// funcional: sin IO, sin Bubbletea, y aun así decide lo que el usuario ve.
func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		target   string
		wantOK   bool
		compare  string // "any" | "gt0" | "lt0"
		wantMore string // si no está vacío, otro target que debe puntuar más
	}{
		{name: "query vacío siempre matchea", query: "", target: "cualquiera", wantOK: true, compare: "any"},
		{name: "query sólo de espacios equivale a vacío", query: "   ", target: "x", wantOK: true, compare: "any"},
		{name: "substring al inicio", query: "jua", target: "juan", wantOK: true, compare: "gt0"},
		{name: "substring case-insensitive", query: "JUA", target: "juan", wantOK: true, compare: "gt0"},
		{name: "subtexto al final", query: "an", target: "juan", wantOK: true, compare: "gt0"},
		{name: "subsecuencia", query: "jn", target: "juan", wantOK: true, compare: "gt0"},
		{name: "no matchea", query: "zzz", target: "juan", wantOK: false, compare: "any"},
		{name: "no matchea por orden", query: "naj", target: "juan", wantOK: false, compare: "any"},
		{name: "query más largo que el target", query: "juanito", target: "juan", wantOK: false, compare: "any"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, ok := fuzzyScore(tt.query, tt.target)
			if ok != tt.wantOK {
				t.Fatalf("fuzzyScore(%q, %q) ok = %v, want %v (score %d)", tt.query, tt.target, ok, tt.wantOK, score)
			}
			switch tt.compare {
			case "gt0":
				if score <= 0 {
					t.Errorf("score = %d, want > 0", score)
				}
			}
		})
	}
}

// Un match por substring tiene que ganar a un match por subsecuencia: por eso el
// autocomplete ordena primero lo que se parece de verdad.
func TestFuzzyScorePrefersSubstringOverSubsequence(t *testing.T) {
	substr, _ := fuzzyScore("an", "juan")
	subseq, ok := fuzzyScore("jn", "juan")
	if !ok {
		t.Fatal("jn debería matchear por subsecuencia")
	}
	if substr <= subseq {
		t.Errorf("substring %d no puntúa por encima de subsecuencia %d", substr, subseq)
	}
}

// Entre dos substring, gana el que empieza antes.
func TestFuzzyScorePrefersEarlierMatch(t *testing.T) {
	early, _ := fuzzyScore("juan", "juan perez")
	late, _ := fuzzyScore("juan", "maria juan")
	if early <= late {
		t.Errorf("match al inicio %d no puntúa por encima del final %d", early, late)
	}
}

// Los bordes que el TestFuzzyFilter de new_task_test.go no cubre: sin tope, sin
// coincidencias y con acentos de mayúsculas.
func TestFuzzyFilterEdges(t *testing.T) {
	items := []string{"@juan", "@maria", "@ana", "@bob"}

	tests := []struct {
		name  string
		query string
		max   int
		want  []string
	}{
		{"sin tope devuelve todo", "", 0, items},
		{"sin coincidencias devuelve vacío", "zzz", 10, nil},
		{"match por subcadena", "ma", 10, []string{"@maria"}},
		{"ignora mayúsculas en el query", "MA", 10, []string{"@maria"}},
		{"ignora espacios en el query", "  ma  ", 10, []string{"@maria"}},
		{"el tope manda sobre el ranking", "a", 1, []string{"@ana"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fuzzyFilter(items, tt.query, tt.max)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("fuzzyFilter(%q, %d) = %v, want %v", tt.query, tt.max, got, tt.want)
			}
		})
	}
}

func TestFuzzyFilterEmptyInput(t *testing.T) {
	if got := fuzzyFilter(nil, "", 5); len(got) != 0 {
		t.Errorf("fuzzyFilter(nil, \"\", 5) = %v, want vacío", got)
	}
	if got := fuzzyFilter([]string{"@juan"}, "zzz", 5); len(got) != 0 {
		t.Errorf("sin coincidencias = %v, want vacío", got)
	}
}

// fuzzyFilter no toca el slice de entrada: los callers lo reutilizan.
func TestFuzzyFilterDoesNotMutateInput(t *testing.T) {
	items := []string{"@juan", "@maria", "@ana"}
	before := strings.Join(items, ",")

	_ = fuzzyFilter(items, "a", 1)
	_ = fuzzyFilter(items, "", 2)

	if after := strings.Join(items, ","); after != before {
		t.Errorf("fuzzyFilter mutó la entrada: %q -> %q", before, after)
	}
}

// containsFold es la comprobación de "¿ya está en la lista?" que usan el alta de
// tarea y las sugerencias: ignora mayúsculas y espacios.
// containsFold recorta el VALOR pero no los elementos de la lista: los callers
// pasan entradas ya normalizadas (model.ParseTags), así que un elemento con
// espacios no se encuentra. Fijado aquí para que nadie asuma lo contrario.
func TestContainsFold(t *testing.T) {
	items := []string{"Bug", "urgent", "bloqueado"}

	tests := []struct {
		value string
		want  bool
	}{
		{"bug", true},
		{"BUG", true},
		{"urgent", true},
		{"bloqueado", true},
		{"  bloqueado  ", true}, // el valor sí se recorta
		{"nope", false},
		{"", false},
		{"bug ", true},
	}
	for _, tt := range tests {
		if got := containsFold(items, tt.value); got != tt.want {
			t.Errorf("containsFold(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}

	if containsFold(nil, "x") {
		t.Error("una lista vacía no contiene nada")
	}
	// Los elementos de la lista NO se recortan.
	if containsFold([]string{" bloqueado "}, "bloqueado") {
		t.Error("los elementos de la lista no se recortan: sólo el valor")
	}
}
