package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"tsk/internal/model"
)

// The search prompt opens with "/" in every listing view.
func TestSlashOpensSearch(t *testing.T) {
	for _, view := range []string{"1", "2", "3"} {
		t.Run("view "+view, func(t *testing.T) {
			m := newTestModel(t)
			m, _ = press(m, view)
			m, _ = press(m, "/")
			if !m.searchOpen {
				t.Errorf("'/' in view %s did not open the search prompt", view)
			}
		})
	}
}

// Typing in the prompt narrows the listing while typing: title match,
// case-insensitive.
func TestSearchNarrowsByTitle(t *testing.T) {
	m := newTestModel(t)
	m, _ = press(m, "1")
	m, _ = pressKeys(t, m, "/", "c", "h", "e")

	if m.searchQuery != "che" {
		t.Fatalf("searchQuery = %q, want che", m.searchQuery)
	}
	tasks := m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Fix checkout" {
		t.Fatalf("filtered by title = %v, want only Fix checkout", titlesOf(tasks))
	}
}

// The description is part of the search too, not only the title.
func TestSearchMatchesDescription(t *testing.T) {
	m := newTestModel(t)
	mustCreateTask(t, m.database, "api", "Unrelated title", "hidden needle here", "@john", 1, "backlog")
	reloadTasks(t, m)

	m, _ = pressKeys(t, m, "/", "n", "e", "e", "d", "l", "e")

	tasks := m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Unrelated title" {
		t.Fatalf("filtered by description = %v, want only Unrelated title", titlesOf(tasks))
	}
}

// "enter" closes the prompt and keeps the query applied.
func TestSearchEnterKeepsQuery(t *testing.T) {
	m := newTestModel(t)
	m, _ = pressKeys(t, m, "/", "f", "i", "x", "enter")

	if m.searchOpen {
		t.Error("enter left the search prompt open")
	}
	if m.searchQuery != "fix" {
		t.Errorf("searchQuery = %q, want fix", m.searchQuery)
	}
	if len(m.filteredTasks()) != 2 {
		t.Errorf("query was not applied: %v", titlesOf(m.filteredTasks()))
	}
}

// "esc" restores the query committed before the prompt opened.
func TestSearchEscRestoresPreviousQuery(t *testing.T) {
	m := newTestModel(t)
	m.searchQuery = "fix"
	m, _ = pressKeys(t, m, "/", "c", "esc")

	if m.searchOpen {
		t.Error("esc left the search prompt open")
	}
	if m.searchQuery != "fix" {
		t.Errorf("searchQuery = %q, want the previous fix", m.searchQuery)
	}
}

// With the prompt closed, "esc" clears an active search.
func TestSearchEscClearsActiveQuery(t *testing.T) {
	m := newTestModel(t)
	m.searchQuery = "checkout"

	m, _ = press(m, "esc")

	if m.searchQuery != "" {
		t.Errorf("esc did not clear the search: %q", m.searchQuery)
	}
	if len(m.filteredTasks()) != 4 {
		t.Errorf("clearing the search did not restore the listing: %v", titlesOf(m.filteredTasks()))
	}
}

// Backspace edits the query and the listing follows.
func TestSearchBackspace(t *testing.T) {
	m := newTestModel(t)
	m, _ = pressKeys(t, m, "/", "c", "h", "e", "x", "backspace")

	if m.searchQuery != "che" {
		t.Fatalf("searchQuery = %q, want che", m.searchQuery)
	}
	if len(m.filteredTasks()) != 1 {
		t.Errorf("the listing did not follow the edit: %v", titlesOf(m.filteredTasks()))
	}
}

// The search composes with the other filters instead of replacing them.
func TestSearchComposesWithProjectFilter(t *testing.T) {
	m := newTestModel(t)
	m.filterProject = "web"
	m, _ = pressKeys(t, m, "/", "f", "i", "x")

	tasks := m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Fix checkout" {
		t.Fatalf("search + project = %v, want only Fix checkout", titlesOf(tasks))
	}
}

// A space is part of the query, not a key that closes the prompt.
func TestSearchAcceptsSpace(t *testing.T) {
	m := newTestModel(t)
	m, _ = pressKeys(t, m, "/", "f", "i", "x", " ", "n")

	if !m.searchOpen {
		t.Fatal("the space closed the search prompt")
	}
	if m.searchQuery != "fix n" {
		t.Errorf("searchQuery = %q, want \"fix n\"", m.searchQuery)
	}
	if len(m.filteredTasks()) != 1 || m.filteredTasks()[0].Title != "Fix N+1 query" {
		t.Errorf("filtered = %v, want only Fix N+1 query", titlesOf(m.filteredTasks()))
	}
}

// titlesOf names the rendered tasks in test failures.
func titlesOf(tasks []model.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Title
	}
	return out
}

// The filter bar shows the active query, and the open prompt shows the
// live text with a cursor.
func TestSearchShownInFilterBar(t *testing.T) {
	m := newTestModel(t)
	m.searchQuery = "checkout"
	if out := ansi.Strip(m.renderFilterBar()); !strings.Contains(out, "Search: checkout") {
		t.Errorf("committed query missing from the bar: %q", out)
	}

	m.searchOpen = true
	m.searchInput = "check"
	if out := ansi.Strip(m.renderFilterBar()); !strings.Contains(out, "Search: check▏") {
		t.Errorf("live prompt missing from the bar: %q", out)
	}
}

// Without a query and with the prompt closed the bar has no search part.
func TestSearchHiddenWhenInactive(t *testing.T) {
	m := newTestModel(t)
	if out := ansi.Strip(m.renderFilterBar()); strings.Contains(out, "Search") {
		t.Errorf("bar shows a search with nothing to search: %q", out)
	}
}
