package tui

import (
	"cmp"
	"slices"
	"strings"
)

// fuzzyScore scores how well query matches target (case-insensitive).
// Returns (score, ok); the higher the score, the better the match. It prioritizes
// substring matches (closer to the start = better) and, if there are none, subsequences.
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

// cappedLen returns how many elements of n remain after applying a cap.
//
// It is a min() with the rule "zero (or negative) cap means no cap", which is
// the one the callers used and that was written three times as `if max > 0 &&
// len(x) > max`. With the if, the `len(x) > max` only differed from the `>=`
// when len(x) == max, and there both branches give the same slice; the min has
// no edge to mutate.
func cappedLen(n, max int) int {
	if max <= 0 {
		return n
	}
	return min(n, max)
}

// fuzzyFilter returns up to max items that match query, sorted by descending
// score and ascending name as a tiebreaker. With an empty query it returns
// the first max in the original order.
func fuzzyFilter(items []string, query string, max int) []string {
	// With no query there is no ranking: the original order is returned (the cap applies).
	if strings.TrimSpace(query) == "" {
		return append([]string(nil), items[:cappedLen(len(items), max)]...)
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
	// slices.SortStableFunc with a cmp of three Outcomes instead of sort.SliceStable
	// with a boolean comparator.
	//
	// With the boolean comparator, two distinct lines --"lower score" and
	// "on equal score, lower name"--Ggremlins mutated them separately, and the
	// two pieces had a `<` against which no test could be placed: the
	// names come out of a list without repeats and the scores are only
	// compared when it is already known they are not equal, so on both edges
	// the opposite branch gave the same result.
	//
	// The cmp returns a number, and the tiebreaker stays in the SAME expression as
	// the main comparison, so there are not two lines to mutate but one.
	slices.SortStableFunc(out, func(a, b scored) int {
		if a.score != b.score {
			// Descending: the one with MORE score goes first.
			return cmp.Compare(b.score, a.score)
		}
		return strings.Compare(a.item, b.item)
	})
	out = out[:cappedLen(len(out), max)]
	res := make([]string, len(out))
	for i, s := range out {
		res[i] = s.item
	}
	return res
}

// containsFold tells whether the list contains value, ignoring case.
func containsFold(items []string, value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	for _, it := range items {
		if strings.ToLower(it) == v {
			return true
		}
	}
	return false
}
