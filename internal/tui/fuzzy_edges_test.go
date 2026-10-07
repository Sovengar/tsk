package tui

import (
	"strings"
	"testing"
)

// fuzzyFilter has two caps: a default one and a cap (max). The four
// edges that separate the comparisons are:
//
//	len(items) > max	-- how many elements there are under the cap
//	max > 0		-- whether the cap is active
//	score > score	-- the ranking's tiebreaker
//	item < item		-- the tiebreaker by name
//
// None of them is told apart with a short list and a normal query: if there
// are fewer elements than the cap, both sides give the same, and if there are
// no ties, the tiebreaker never runs. What separates them are three concrete
// situations: the EXACT cap, the cap disabled, and the list with ties.

func TestFuzzyCutsExactlyAtTheCap(t *testing.T) {
	// "bx" and "by": both match the query and both have the same
	// score, so both come out whatever the query is.
	items := []string{"bx", "by"}

	got := fuzzyFilter(items, "b", 2)
	if len(got) != 2 {
		t.Errorf("with 2 elements and cap 2, %d come out, want 2: the cap is a maximum, not a target", len(got))
	}

	// And with an empty query, which goes through the other branch of the if.
	got = fuzzyFilter(items, "", 2)
	if len(got) != 2 {
		t.Errorf("with an empty query and cap 2, %d come out, want 2", len(got))
	}

	// One element under the cap: nothing is touched.
	if got := fuzzyFilter(items, "b", 5); len(got) != 2 {
		t.Errorf("with plenty of cap, %d come out, want 2", len(got))
	}

	// And one over the cap, where the cut really shows.
	if got := fuzzyFilter(items, "b", 1); len(got) != 1 {
		t.Errorf("with cap 1, %d come out, want 1", len(got))
	}
}

// The ranking with an empty query is not reordered: it comes out in the
// original order. An alphabetical order there would be a visible difference as
// soon as the list did not match the alphabet, which is the normal case.
func TestFuzzyWithoutQueryKeepsOriginalOrder(t *testing.T) {
	items := []string{"@zoe", "@ann", "@margo"}

	got := fuzzyFilter(items, "", 0)
	want := "@zoe,@ann,@margo"
	if strings.Join(got, ",") != want {
		t.Errorf("without a query the order is %v, want %s (the input's)", got, want)
	}
}

// The tiebreak by score: two elements with the same score are ordered by
// name. With a badly made comparator (`<` instead of `>`) they would come out
// reversed, and it only shows if there are two with the SAME score.
func TestFuzzyTieBreaksByNameWithEqualScore(t *testing.T) {
	// The score is 1000 - idx*10 - (len - len(query)), so two elements
	// tie if they have the same length and the query appears at the same
	// position. "bx" and "by" tie at 999; "@ann" and "@ava" too.
	cases := []struct {
		name  string
		items []string
		query string
		want  string
	}{
		{"different letters after the match", []string{"by", "bx"}, "b", "bx,by"},
		{"person names", []string{"@ava", "@ann"}, "a", "@ann,@ava"},
		{"three at once", []string{"@cxa", "@ann", "@bxa"}, "a", "@ann,@bxa,@cxa"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fuzzyFilter(tc.items, tc.query, 0)
			if strings.Join(got, ",") != tc.want {
				t.Errorf("fuzzyFilter(%v, %q) = %v, want %s (same score: alphabetical)",
					tc.items, tc.query, got, tc.want)
			}
			// And the other way around: the shuffled input gives the same output,
			// which is what proves that the order does not come from the input.
			reversed := append([]string{tc.items[len(tc.items)-1]}, tc.items[:len(tc.items)-1]...)
			if rev := fuzzyFilter(reversed, tc.query, 0); strings.Join(rev, ",") != tc.want {
				t.Errorf("with the reversed input, %v comes out, want %s", rev, tc.want)
			}
		})
	}
}

// The score ranking rules over the name: "api" beats "hola" even though
// "hola" comes first alphabetically. Without this case, a well-made
// tiebreak with a badly-made ranking would still give the same result.
func TestFuzzyScoreBeatsName(t *testing.T) {
	// "b" scores 999 (match at position 0, length 1) and "ab" scores 989
	// (match at 1, length 2). Alphabetically "ab" goes first, so this case
	// is what separates "I order by score" from "I order by name": if the
	// score comparator were reversed, "ab,b" would come out.
	items := []string{"ab", "b"}

	got := fuzzyFilter(items, "b", 0)
	if strings.Join(got, ",") != "b,ab" {
		t.Errorf("fuzzyFilter(%v, \"b\") = %v, want b,ab: the score rules, even though the name is different", items, got)
	}
}

// max at zero means "no cap", not "zero results". It is the difference
// between a `max > 0` and a `max >= 0`, and with an empty result list it does not show.
func TestFuzzyZeroCapMeansNoCap(t *testing.T) {
	items := []string{"@ann", "@bxa", "@cxa"}

	for _, max := range []int{0, 1, 5} {
		got := fuzzyFilter(items, "a", max)
		if len(got) == 0 {
			t.Errorf("with cap %d and one match it comes out empty", max)
		}
		if len(got) > len(items) {
			t.Errorf("with cap %d, %d elements of %d come out", max, len(got), len(items))
		}
	}

	// With cap 0 EVERYTHING that matches comes out, not nothing.
	if got := fuzzyFilter(items, "a", 0); len(got) != 3 {
		t.Errorf("with cap 0, %d of 3 come out, want 3: cap zero means no cap", len(got))
	}

	// With an empty query there is no ranking, but the cap still cuts.
	if got := fuzzyFilter(items, "", 2); len(got) != 2 {
		t.Errorf("without a query and with cap 2, %d come out, want 2", len(got))
	}
}
