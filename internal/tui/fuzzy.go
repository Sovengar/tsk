package tui

import (
	"cmp"
	"slices"
	"strings"
)

// fuzzyScore puntúa qué tan bien query matchea target (case-insensitive).
// Devuelve (score, ok); a mayor score, mejor match. Prioriza coincidencias por
// substring (más cerca del inicio = mejor) y, si no hay, subsecuencias.
func fuzzyScore(query, target string) (int, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(target)
	if q == "" {
		return 0, true
	}
	if idx := strings.Index(t, q); idx >= 0 {
		return 1000 - idx*10 - (len(t) - len(q)), true
	}
	qi := 0
	for ti := range len(t) {
		if qi >= len(q) {
			break
		}
		if q[qi] == t[ti] {
			qi++
		}
	}
	if qi == len(q) {
		return qi, true
	}
	return 0, false
}

// topeDe devuelve cuántos elementos de n se quedan tras aplicar un tope.
//
// Es un min() con la regla "tope cero (o negativo) significa sin tope", que es la
// que usaban los llamadores y que estaba escrita tres veces como `if max > 0 &&
// len(x) > max`. Con el if, el `len(x) > max` sólo se distinguía del `>=` cuando
// len(x) == max, y ahí las dos ramas dan el mismo slice; el min no tiene borde que
// mutar.
func topeDe(n, max int) int {
	if max <= 0 {
		return n
	}
	return min(n, max)
}

// fuzzyFilter devuelve hasta max items que matchean query, ordenados por score
// descendente y nombre ascendente como desempate. Con query vacío devuelve los
// primeros max en el orden original.
func fuzzyFilter(items []string, query string, max int) []string {
	// Sin query no hay ranking: se devuelve el orden original (el tope aplica).
	if strings.TrimSpace(query) == "" {
		return append([]string(nil), items[:topeDe(len(items), max)]...)
	}

	type scored struct {
		item  string
		score int
	}
	out := make([]scored, 0, len(items))
	for _, it := range items {
		if s, ok := fuzzyScore(query, it); ok {
			out = append(out, scored{it, s})
		}
	}
	// slices.SortStableFunc con un cmp de tres Outcomes en vez de sort.SliceStable
	// con un comparador booleano.
	//
	// Con el comparador booleano, dos líneas distintas --"menor puntuación" y
	// "a igual puntuación, menor nombre"--Ggremlins las mutaba por separado, y las
	// dos piezas tenían un `<` contra el que no se podía colocar un test: los
	// nombres salen de una lista sin repetidos y las puntuaciones sólo se
	// comparan cuando ya se sabe que no son iguales, así que en ambos bordes la
	// rama opuesta daba el mismo resultado.
	//
	// El cmp devuelve un número, y el desempate queda en la MISMA expresión que
	// la comparación principal, así que no hay dos líneas que mutar sino una.
	slices.SortStableFunc(out, func(a, b scored) int {
		if a.score != b.score {
			// Descendente: el que tiene MÁS puntuación va antes.
			return cmp.Compare(b.score, a.score)
		}
		return strings.Compare(a.item, b.item)
	})
	out = out[:topeDe(len(out), max)]
	res := make([]string, len(out))
	for i, s := range out {
		res[i] = s.item
	}
	return res
}

// containsFold indica si la lista contiene value, ignorando mayúsculas.
func containsFold(items []string, value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	for _, it := range items {
		if strings.ToLower(it) == v {
			return true
		}
	}
	return false
}
