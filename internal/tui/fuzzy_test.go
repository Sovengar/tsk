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

// El score por substring es una fórmula exacta, no una escala: 1000 menos diez
// por cada carácter de desplazamiento, menos lo que sobra del target. Estos
// números son literales a propósito. Comparar sólo "mayor que cero" o "mejor
// que el otro" deja vivos los signos: cambiar un + por un - mantiene el orden y
// nadie lo nota hasta que el ranking sale raro.
func TestFuzzyScoreExactValues(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		target string
		want   int
	}{
		{"coincidencia exacta", "juan", "juan", 1000},
		{"prefijo de 3 de 4", "jua", "juan", 999},
		{"sin shift, mayúsculas", "abc", "ABC", 1000},
		{"en medio de una sola", "a", "banana", 985},
		{"al final de una sola", "na", "banana", 976},
		{"al final de un nombre", "an", "juan", 978},
		{"al inicio de algo más largo", "juan", "juan perez", 994},
		{"al final de algo más largo", "juan", "maria juan", 934},
		{"subsecuencia vale su longitud", "jn", "juan", 2},
		{"subsecuencia larga", "ao", "abaco", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := fuzzyScore(tt.query, tt.target)
			if !ok {
				t.Fatalf("fuzzyScore(%q, %q) no matchea", tt.query, tt.target)
			}
			if got != tt.want {
				t.Errorf("fuzzyScore(%q, %q) = %d, want %d", tt.query, tt.target, got, tt.want)
			}
		})
	}
}

// El desplazamiento pesa diez por carácter, así que dos coincidencias con la
// misma longitud de target pero desplazadas diez columnas tienen exactamente
// cien puntos de diferencia. Un signo cambiado aquí rompería el orden.
func TestFuzzyScoreOffsetCostsTenPerCharacter(t *testing.T) {
	// Misma longitud de target, así que lo único que cambia es el desplazamiento:
	// seis columnas de desfase son sesenta puntos, ni uno más ni uno menos.
	inicio, _ := fuzzyScore("ab", "abxxxxxx")
	despues, _ := fuzzyScore("ab", "xxxxxxab")
	if inicio-despues != 60 {
		t.Errorf("la diferencia entre desplazar 0 y 6 es %d, want 60", inicio-despues)
	}
}

// Cuando la subsecuencia se completa antes del final del target, el recorrido
// tiene que parar ahí. Si no se parara, leería fuera de la query.
func TestFuzzyScoreStopsAtQueryEndMidTarget(t *testing.T) {
	// "jy" se completa en el tercer carácter de un target de cuatro, y no es
	// subcadena: si fuera, la fórmula del substring cortaría antes el recorrido.
	got, ok := fuzzyScore("jy", "jxyz")
	if !ok {
		t.Fatal("jy debería matchear jxyz por subsecuencia")
	}
	if got != 2 {
		t.Errorf("score = %d, want 2 (la longitud de la query)", got)
	}
}

// A igual score, el desempate es alfabético. Sin esto, dos nombres con la misma
// puntuación saldrían en el orden que promotora el ranking, que es el del
// llamante y no el que espera quien lee.
func TestFuzzyFilterTieBreaksAlphabetically(t *testing.T) {
	// Las dos puntúan igual: la query aparece en la misma posición y los dos
	// nombres miden lo mismo.
	items := []string{"@zzab", "@aaab"}

	got := fuzzyFilter(items, "ab", 10)
	if strings.Join(got, ",") != "@aaab,@zzab" {
		t.Errorf("fuzzyFilter = %v, want el desempate alfabético", got)
	}

	for _, a := range got {
		if _, ok := fuzzyScore("ab", a); !ok {
			t.Errorf("%q no matchea, así que el test no mide el desempate", a)
		}
	}
	if s1, _ := fuzzyScore("ab", got[0]); s1 <= 0 {
		t.Errorf("el primero tiene score %d", s1)
	}
}

// Y el desempate no se aplica entre scores distintos: el más alto va primero
// aunque la letra diga lo contrario.
func TestFuzzyFilterScoreBeatsAlphabetical(t *testing.T) {
	// "zzab" puntúa más que "@aaaab" (aparece en 0 frente a 3).
	items := []string{"@aaaab", "@zzab"}

	got := fuzzyFilter(items, "ab", 10)
	if strings.Join(got, ",") != "@zzab,@aaaab" {
		t.Errorf("fuzzyFilter = %v, want primero el de mayor score", got)
	}
}

// El ranking es estable entre elementos con la misma puntuación y el mismo
// nombre: el orden de entrada manda cuando no hay nada que desempatar.
func TestFuzzyFilterKeepsInputOrderOnFullTie(t *testing.T) {
	items := []string{"ab", "ab", "ab"}
	got := fuzzyFilter(items, "ab", 10)
	if strings.Join(got, ",") != "ab,ab,ab" {
		t.Errorf("got %v", got)
	}
}
