package tui

import (
	"sort"
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
	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		if q[qi] == t[ti] {
			qi++
		}
	}
	if qi == len(q) {
		return qi, true
	}
	return 0, false
}

// fuzzyFilter devuelve hasta max items que matchean query, ordenados por score
// descendente y nombre ascendente como desempate. Con query vacío devuelve los
// primeros max en el orden original.
func fuzzyFilter(items []string, query string, max int) []string {
	// Sin query no hay ranking: se devuelve el orden original (el tope aplica).
	if strings.TrimSpace(query) == "" {
		if max > 0 && len(items) > max {
			items = items[:max]
		}
		return append([]string(nil), items...)
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
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].item < out[j].item
	})
	if max > 0 && len(out) > max {
		out = out[:max]
	}
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
