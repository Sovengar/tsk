package tui

import "testing"

// listWindowLegacy reproduces the ORIGINAL arithmetic (currentPage, totalPages,
// pageBounds) just as it stood before the refactor. It works as a safety net: if
// listWindow stops giving the same result in a reachable state, this test trips.
//
// The ONLY intentional divergence is with cursor >= total, which the view itself
// makes unreachable because clampListCursor runs first: the legacy returned
// an empty window and a caption like "Page 20/2", and the new one clamps to the
// last real page.
func listWindowLegacy(total, size, cursor int) (page, pages, start, end int) {
	pages = (total + size - 1) / size
	if pages < 1 {
		pages = 1
	}
	page = cursor / size
	start = page * size
	if start > total {
		start = total
	}
	end = start + size
	if end > total {
		end = total
	}
	return
}

func TestListWindowMatchesLegacyArithmetic(t *testing.T) {
	// A single model per (size, total): creating the test DB for every cursor
	// turned this safety net into 60 seconds of suite.
	for _, size := range []int{1, 2, 3, 5, 10, 37} {
		for total := 0; total <= 40; total++ {
			m := modelWithTasks(t, total, size)
			for cursor := 0; cursor <= total+5; cursor++ {
				page, pages, start, end := listWindowLegacy(total, size, cursor)

				m.cursor = cursor
				w := m.listWindow()

				if w.total != total || w.size != size {
					t.Fatalf("broken fixture: total/size = %d/%d, want %d/%d", w.total, w.size, total, size)
				}
				// cursor < total is the reachable range: after clampListCursor the
				// cursor always satisfies cursor < total.
				if cursor < total && (w.page != page || w.pages != pages || w.start != start || w.end != end) {
					t.Fatalf("size=%d total=%d cursor=%d: legacy {p:%d pages:%d start:%d end:%d} != new {p:%d pages:%d start:%d end:%d}",
						size, total, cursor, page, pages, start, end, w.page, w.pages, w.start, w.end)
				}
			}
		}
	}
}

// The legacy clamp and the new one match for every valid cursor.
func TestClampMatchesLegacyForValidCursors(t *testing.T) {
	for _, size := range []int{1, 3, 5, 10} {
		for total := 1; total <= 30; total++ {
			m := modelWithTasks(t, total, size)
			for cursor := 0; cursor < total; cursor++ {
				m.cursor = cursor

				legacy := cursor
				if legacy >= total {
					legacy = total - 1
				}
				if got := m.listWindow().clampCursor(cursor); got != legacy {
					t.Errorf("size=%d total=%d cursor=%d: clampCursor=%d, legacy=%d", size, total, cursor, got, legacy)
				}
			}
		}
	}
}
