package tui

import (
	"strings"
	"testing"
)

// The two text modals --the filter one and the tag one-- filter what comes
// in through a condition of the same shape: "a single character, printable".
//
// What has to be checked is not that a letter is accepted, but where the
// edge is. The filter's range is [33, 127) and the tag one's is [32, 127):
// the filter leaves the space out, tags accepts it. Both flip together at 127.
//
// A test with "a" says none of that: 'a' is in the middle of the two ranges
// and both accept it. Only the edge separates them, and that is why the
// characters right on the edge and the ones right outside are tested.

// edgeKeys are the characters around the two limits: 32 (space,
// which only tags accepts), 33 ('!', the filter's first), 126 (~, the
// last printable ASCII) and 127 (DEL, outside both ranges).
var edgeKeys = []struct {
	name    string
	key     string
	accepts bool
	char    rune
}{
	{"space (32)", " ", false, 0},
	{"exclamation (33)", "!", true, '!'},
	{"comma (44)", ",", true, ','},
	{"tilde (126)", "~", true, '~'},
	{"DEL (127)", "\x7f", false, 0},
}

// The filter opens with "/" and types into the field that has the focus,
// which on open is the first one.
func openFilter(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.currentView = viewList
	opened, _ := pressKeys(t, m, "/")
	if !opened.filterOpen {
		t.Fatal("the filter modal did not open with /")
	}
	return opened
}

// updateTagPaste pastes text into the tag modal through the paste path,
// which is the only one through which a space can arrive.
func updateTagPaste(t *testing.T, m *Model, text string) *Model {
	t.Helper()
	m.tagOpen = true
	pasted, _ := m.handleTagPaste(text)
	model, ok := pasted.(Model)
	if !ok {
		t.Fatalf("handleTagPaste(%q) returned %T, want Model", text, pasted)
	}
	return &model
}

// Each edge character has to end up in the search field or not, according
// to what the code's comparison says. What is looked at is the RESULT, not
// the code: if someone changes the range, the field stops growing and the test sees it.
func TestFilterModalAcceptsOnlyPrintables(t *testing.T) {
	for _, tc := range edgeKeys {
		t.Run(tc.name, func(t *testing.T) {
			m := openFilter(t)

			m, _ = pressKeys(t, m, tc.key)

			if tc.accepts {
				if m.filterSearch != string(tc.char) {
					t.Errorf("the filter did not accept %q: filterSearch = %q",
						string(tc.char), m.filterSearch)
				}
				return
			}
			if m.filterSearch != "" {
				t.Errorf("the filter accepted %q, which is not printable: filterSearch = %q",
					tc.key, m.filterSearch)
			}
		})
	}
}

// The two modals reject the space, but for different reasons, and it is
// worth leaving it on record.
//
// In the filter it is the comparison: the range starts at 33, and a space
// in the search box of a dropdown is not a search, it is a hole.
//
// In tags it is the `len(key) == 1` in front: Bubbletea delivers the space
// bar as a named key ("space"), not as a lone character, so a lone byte
// 32 never arrives. The floor of 32 in the range documents the intent but
// does not make it reachable; that is why the `>= 32` -> `> 32` is an
// EQUIVALENT mutant and it is in .mutation-allowlist for this reason, not for the hole.
//
// What does reach both is a paste, which goes through another path and
// keeps the spaces. That makes clear that the difference is of the path and
// not that spaces are forbidden.
func TestSpaceReachesNeitherFilterNorTags(t *testing.T) {
	filter := openFilter(t)
	filter, _ = pressKeys(t, filter, " ")
	if filter.filterSearch != "" {
		t.Errorf("the filter took the space: %q", filter.filterSearch)
	}

	tags := newDetailModel(t, 0)
	tags, _ = pressKeys(t, tags, "t")
	if !tags.tagOpen {
		t.Fatal("the tags modal did not open with t")
	}
	tags, _ = pressKeys(t, tags, " ")
	if tags.tagInput != "" {
		t.Errorf("tags took the space bar: %q", tags.tagInput)
	}

	tags = updateTagPaste(t, tags, "with spaces")
	if !strings.Contains(tags.tagInput, " ") {
		t.Errorf("a paste with spaces arrived without them: %q", tags.tagInput)
	}
}

// The two high edges: 126 is accepted and 127 is not. With the range written
// as `<` instead of `<=`, 127 would get in; with `<=`, 126 would stay out.
// No test with a normal letter tells those apart.
func TestPrintableRangeUpperEdge(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
	}{
		{"tilde (126)", "~"},
		{"DEL (127)", "\x7f"},
	} {
		t.Run("filter/"+tc.name, func(t *testing.T) {
			m := openFilter(t)
			m, _ = pressKeys(t, m, tc.key)
			accepted := m.filterSearch != ""
			if tc.name[0] == 't' != accepted {
				t.Errorf("filter: %s accepted=%v", tc.name, accepted)
			}
		})
		t.Run("tags/"+tc.name, func(t *testing.T) {
			m := newDetailModel(t, 0)
			m, _ = pressKeys(t, m, "t")
			if !m.tagOpen {
				t.Fatalf("the tags modal did not open with t")
			}
			m, _ = pressKeys(t, m, tc.key)
			accepted := m.tagInput != ""
			if tc.name[0] == 't' != accepted {
				t.Errorf("tags: %s accepted=%v", tc.name, accepted)
			}
		})
	}
}

// isPrintable is the range of bytes accepted as a single-character key, in
// the tag modal and in the filter one.
//
// Both edges are reachable and checkable, and that is what the refactor does:
// before, the tag modal's range started at 32 (the space), a byte that
// Bubbletea never delivers alone -- the space bar arrives named. So the floor
// of 32 and the one of 33 were indistinguishable and the mutant of one for
// the other survived. With the floor on the first byte that does arrive, both sides exist.
func TestIsPrintable(t *testing.T) {
	cases := []struct {
		name string
		b    byte
		want bool
	}{
		{"null", 0, false},
		{"tab (9)", 9, false},
		{"new line (10)", 10, false},
		{"escape (27)", 27, false},
		{"right below the floor (32, the space)", 32, false},
		{"exactly the floor (33, exclamation)", 33, true},
		{"letter", 'a', true},
		{"digit", '7', true},
		{"comma", ',', true},
		{"tilde (126)", 126, true},
		{"exactly the ceiling (127, DEL)", 127, false},
		{"non-ASCII (200)", 200, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPrintable(c.b); got != c.want {
				t.Errorf("isPrintable(%d) = %v, want %v", c.b, got, c.want)
			}
		})
	}
}

// The whole range walk, byte by byte, so that a change in either of the two
// ends shows and not only the values that happens to look at a test.
func TestIsPrintableScansTheWholeRange(t *testing.T) {
	for b := 0; b < 256; b++ {
		want := b >= firstPrintableByte && b < lastPrintableByte
		if got := isPrintable(byte(b)); got != want {
			t.Fatalf("isPrintable(%d) = %v, want %v: the declared range is [%d, %d)",
				b, got, want, firstPrintableByte, lastPrintableByte)
		}
	}
}
