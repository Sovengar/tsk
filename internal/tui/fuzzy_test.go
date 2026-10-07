package tui

import (
	"strings"
	"testing"
)

// fuzzyScore decides which suggestions are shown in the autocomplete and in
// the filter modal. It is the piece that had the most mutants uncovered and
// it is purely functional: no IO, no Bubbletea, and still it decides what the user sees.
func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		target   string
		wantOK   bool
		compare  string // "any" | "gt0" | "lt0"
		wantMore string // if not empty, another target that must score higher
	}{
		{name: "empty query always matches", query: "", target: "anything", wantOK: true, compare: "any"},
		{name: "query of only spaces is equivalent to empty", query: "   ", target: "x", wantOK: true, compare: "any"},
		{name: "substring at the start", query: "joh", target: "john", wantOK: true, compare: "gt0"},
		{name: "substring case-insensitive", query: "JOH", target: "john", wantOK: true, compare: "gt0"},
		{name: "subtext at the end", query: "hn", target: "john", wantOK: true, compare: "gt0"},
		{name: "subsequence", query: "jn", target: "john", wantOK: true, compare: "gt0"},
		{name: "does not match", query: "zzz", target: "john", wantOK: false, compare: "any"},
		{name: "does not match because of order", query: "nhoj", target: "john", wantOK: false, compare: "any"},
		{name: "query longer than the target", query: "johnny", target: "john", wantOK: false, compare: "any"},
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

// A substring match has to beat a subsequence match: that is why the
// autocomplete orders what really looks like it first.
func TestFuzzyScorePrefersSubstringOverSubsequence(t *testing.T) {
	substr, _ := fuzzyScore("oh", "john")
	subseq, ok := fuzzyScore("jn", "john")
	if !ok {
		t.Fatal("jn should match by subsequence")
	}
	if substr <= subseq {
		t.Errorf("substring %d does not score above subsequence %d", substr, subseq)
	}
}

// Between two substrings, the one starting earlier wins.
func TestFuzzyScorePrefersEarlierMatch(t *testing.T) {
	early, _ := fuzzyScore("john", "john smith")
	late, _ := fuzzyScore("john", "margo john")
	if early <= late {
		t.Errorf("match at the start %d does not score above the end %d", early, late)
	}
}

// The edges that new_task_test.go's TestFuzzyFilter does not cover: no cap,
// no matches and with uppercase accents.
func TestFuzzyFilterEdges(t *testing.T) {
	items := []string{"@john", "@margo", "@ann", "@bob"}

	tests := []struct {
		name  string
		query string
		max   int
		want  []string
	}{
		{"no cap returns everything", "", 0, items},
		{"no matches returns empty", "zzz", 10, nil},
		{"match by substring", "ma", 10, []string{"@margo"}},
		{"ignores uppercase in the query", "MA", 10, []string{"@margo"}},
		{"ignores spaces in the query", "  ma  ", 10, []string{"@margo"}},
		{"the cap rules over the ranking", "a", 1, []string{"@ann"}},
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
		t.Errorf("fuzzyFilter(nil, \"\", 5) = %v, want empty", got)
	}
	if got := fuzzyFilter([]string{"@john"}, "zzz", 5); len(got) != 0 {
		t.Errorf("no matches = %v, want empty", got)
	}
}

// fuzzyFilter does not touch the input slice: the callers reuse it.
func TestFuzzyFilterDoesNotMutateInput(t *testing.T) {
	items := []string{"@john", "@margo", "@ann"}
	before := strings.Join(items, ",")

	_ = fuzzyFilter(items, "a", 1)
	_ = fuzzyFilter(items, "", 2)

	if after := strings.Join(items, ","); after != before {
		t.Errorf("fuzzyFilter mutated the input: %q -> %q", before, after)
	}
}

// containsFold is the "is it already in the list?" check that the task form
// and the suggestions use: it ignores case and spaces.
// containsFold trims the VALUE but not the list's elements: the callers
// pass entries already normalized (model.ParseTags), so an element with
// spaces is not found. Pinned here so that nobody assumes otherwise.
func TestContainsFold(t *testing.T) {
	items := []string{"Bug", "urgent", "blocked"}

	tests := []struct {
		value string
		want  bool
	}{
		{"bug", true},
		{"BUG", true},
		{"urgent", true},
		{"blocked", true},
		{"  blocked  ", true}, // the value is trimmed
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
		t.Error("an empty list contains nothing")
	}
	// The list's elements are NOT trimmed.
	if containsFold([]string{" blocked "}, "blocked") {
		t.Error("the list's elements are not trimmed: only the value")
	}
}

// The substring score is an exact formula, not a scale: 1000 minus ten per
// shift character, minus what is left of the target. These numbers are
// literals on purpose. Comparing only "greater than zero" or "better than
// the other" leaves the signs alive: changing a + for a - keeps the order
// and nobody notices until the ranking comes out weird.
func TestFuzzyScoreExactValues(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		target string
		want   int
	}{
		{"exact match", "john", "john", 1000},
		{"prefix of 3 of 4", "joh", "john", 999},
		{"no shift, uppercase", "abc", "ABC", 1000},
		{"in the middle of a single one", "a", "banana", 985},
		{"at the end of a single one", "na", "banana", 976},
		{"at the end of a name", "hn", "john", 978},
		{"at the start of something longer", "john", "john smith", 994},
		{"at the end of something longer", "john", "margo john", 934},
		{"subsequence is worth its length", "jn", "john", 2},
		{"long subsequence", "ao", "tango", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := fuzzyScore(tt.query, tt.target)
			if !ok {
				t.Fatalf("fuzzyScore(%q, %q) does not match", tt.query, tt.target)
			}
			if got != tt.want {
				t.Errorf("fuzzyScore(%q, %q) = %d, want %d", tt.query, tt.target, got, tt.want)
			}
		})
	}
}

// The shift weighs ten per character, so two matches with the same target
// length but shifted ten columns apart have exactly
// a hundred points of difference. A changed sign here would break the order.
func TestFuzzyScoreOffsetCostsTenPerCharacter(t *testing.T) {
	// Same target length, so the only thing that changes is the shift:
	// six columns of offset are sixty points, neither one more nor one less.
	start, _ := fuzzyScore("ab", "abxxxxxx")
	after, _ := fuzzyScore("ab", "xxxxxxab")
	if start-after != 60 {
		t.Errorf("the difference between shifting 0 and 6 is %d, want 60", start-after)
	}
}

// When the subsequence completes before the end of the target, the walk has
// to stop there. If it did not stop, it would read outside the query.
func TestFuzzyScoreStopsAtQueryEndMidTarget(t *testing.T) {
	// "jy" completes on the third character of a target of four, and it is not
	// a substring: if it were, the substring formula would cut the walk earlier.
	got, ok := fuzzyScore("jy", "jxyz")
	if !ok {
		t.Fatal("jy should match jxyz by subsequence")
	}
	if got != 2 {
		t.Errorf("score = %d, want 2 (the query's length)", got)
	}
}

// At equal score the tiebreaker is alphabetical. Without it, two names with
// the same score would come out in the order the ranking promotes, which is
// the caller's and not the one the reader expects.
func TestFuzzyFilterTieBreaksAlphabetically(t *testing.T) {
	// Both score the same: the query appears at the same position and both
	// names measure the same.
	items := []string{"@zzab", "@aaab"}

	got := fuzzyFilter(items, "ab", 10)
	if strings.Join(got, ",") != "@aaab,@zzab" {
		t.Errorf("fuzzyFilter = %v, want the alphabetical tiebreak", got)
	}

	for _, a := range got {
		if _, ok := fuzzyScore("ab", a); !ok {
			t.Errorf("%q does not match, so the test does not measure the tiebreak", a)
		}
	}
	if s1, _ := fuzzyScore("ab", got[0]); s1 <= 0 {
		t.Errorf("the first has score %d", s1)
	}
}

// And the tiebreaker does not apply between different scores: the higher one
// goes first even if the letter says otherwise.
func TestFuzzyFilterScoreBeatsAlphabetical(t *testing.T) {
	// "zzab" scores higher than "@aaaab" (it appears at 0 versus 3).
	items := []string{"@aaaab", "@zzab"}

	got := fuzzyFilter(items, "ab", 10)
	if strings.Join(got, ",") != "@zzab,@aaaab" {
		t.Errorf("fuzzyFilter = %v, want the higher-scoring one first", got)
	}
}

// The ranking is stable between elements with the same score and the same
// name: the input order rules when there is nothing to break the tie.
func TestFuzzyFilterKeepsInputOrderOnFullTie(t *testing.T) {
	items := []string{"ab", "ab", "ab"}
	got := fuzzyFilter(items, "ab", 10)
	if strings.Join(got, ",") != "ab,ab,ab" {
		t.Errorf("got %v", got)
	}
}
