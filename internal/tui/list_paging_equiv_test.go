package tui

import "testing"

// listWindowLegacy reproduce la aritmética ORIGINAL (currentPage, totalPages,
// pageBounds) tal cual estaba antes del refactor. Sirve de red de seguridad: si
// listWindow deja de dar lo mismo en un estado alcanzable, este test salta.
//
// La ÚNICA divergencia intencionada es con cursor >= total, que la propia
// vista hace inalcanzable porque clampListCursor corre antes: el legacy devolvía
// una ventana vacía y una leyenda del tipo "Page 20/2", y el nuevo recorta a la
// última página real.
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
	// Un solo modelo por (size, total): crear la DB de pruebas por cada cursor
	// convertía esta red de seguridad en 60 segundos de suite.
	for _, size := range []int{1, 2, 3, 5, 10, 37} {
		for total := 0; total <= 40; total++ {
			m := modelWithTasks(t, total, size)
			for cursor := 0; cursor <= total+5; cursor++ {
				page, pages, start, end := listWindowLegacy(total, size, cursor)

				m.cursor = cursor
				w := m.listWindow()

				if w.total != total || w.size != size {
					t.Fatalf("fixture rota: total/size = %d/%d, want %d/%d", w.total, w.size, total, size)
				}
				// cursor < total es el rango alcanzable: tras clampListCursor el
				// cursor siempre cumple cursor < total.
				if cursor < total && (w.page != page || w.pages != pages || w.start != start || w.end != end) {
					t.Fatalf("size=%d total=%d cursor=%d: legacy {p:%d pages:%d start:%d end:%d} != nuevo {p:%d pages:%d start:%d end:%d}",
						size, total, cursor, page, pages, start, end, w.page, w.pages, w.start, w.end)
				}
			}
		}
	}
}

// El clamp legacy y el nuevo coinciden para todo cursor válido.
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
