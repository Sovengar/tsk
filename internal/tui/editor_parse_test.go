package tui

import (
	"fmt"
	"strings"
	"testing"

	"tsk/internal/model"
)

// parseEditFile was already a pure function, only with weak tests. The format
// is "# title\n\n<description>\n\n---\nassignee: …\npriority: …\nestimate: …
// \ntags: …" and the parser is the boundary with the external editor: what it
// gets wrong here is stored in the database without anyone noticing.

type parsedEdit struct {
	title, description, assignee string
	priority                     int
	estimate                     float64
	tags                         []string
}

func parse(t *testing.T, content string) parsedEdit {
	t.Helper()
	title, desc, assignee, priority, estimate, tags := parseEditFile(content)
	return parsedEdit{title, desc, assignee, priority, estimate, tags}
}

func TestParseEditFileRoundTrip(t *testing.T) {
	content := "# The title\n\nthe description\n\n---\nassignee: @john\npriority: 2\nestimate: 3.5\ntags: api,web\n"
	got := parse(t, content)

	if got.title != "The title" {
		t.Errorf("title = %q, want \"The title\"", got.title)
	}
	if got.description != "the description" {
		t.Errorf("description = %q, want \"the description\"", got.description)
	}
	if got.assignee != "@john" {
		t.Errorf("assignee = %q, want @john", got.assignee)
	}
	if got.priority != 2 {
		t.Errorf("priority = %d, want 2", got.priority)
	}
	if got.estimate != 3.5 {
		t.Errorf("estimate = %v, want 3.5", got.estimate)
	}
	if len(got.tags) != 2 || got.tags[0] != "api" || got.tags[1] != "web" {
		t.Errorf("tags = %v, want [api web]", got.tags)
	}
}

// The title is only the first line after the "# ": the rest is not the title
// even if it is on the same logical line.
func TestParseEditFileTitleStopsAtNewline(t *testing.T) {
	got := parse(t, "# one\ntwo\nthree\n")
	if got.title != "one" {
		t.Errorf("title = %q, want \"one\" (only the first line)", got.title)
	}
}

// With no "# " there is no title, but the FIRST LINE is discarded anyway: the
// parser treats it as the title's position, not because of the "# ". That
// means a body starting without a hash loses its first line.
func TestParseEditFileWithoutHashTitleDropsFirstLine(t *testing.T) {
	got := parse(t, "without hash\nthe second\n")
	if got.title != "" {
		t.Errorf("title = %q, want empty without \"# \"", got.title)
	}
	if got.description != "the second" {
		t.Errorf("description = %q, want \"the second\" (the 1st line is the title's)", got.description)
	}
}

// The blank lines between the title and the description are skipped; the ones
// at the end are trimmed with TrimSpace.
func TestParseEditFileSkipsBlankLinesAfterTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"no blanks", "# t\ndesc\n", "desc"},
		{"one blank", "# t\n\ndesc\n", "desc"},
		{"several blanks", "# t\n\n\n\ndesc\n", "desc"},
		{"blanks with spaces", "# t\n   \n\t\ndesc\n", "desc"},
		{"blank at the end", "# t\n\ndesc\n\n\n", "desc"},
		{"multiline description", "# t\nline 1\nline 2\n", "line 1\nline 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse(t, tt.content)
			if got.description != tt.want {
				t.Errorf("description = %q, want %q", got.description, tt.want)
			}
		})
	}
}

// A single line is NOT the description: it is the title.
func TestParseEditFileSingleLineIsNotDescription(t *testing.T) {
	got := parse(t, "# only title\n")
	if got.description != "" {
		t.Errorf("description = %q, want empty with a single line", got.description)
	}
}

// The "---" separator cuts the body from the metadata. Without it, all the
// content counts as body.
func TestParseEditFileSeparatorSplitsBody(t *testing.T) {
	got := parse(t, "# t\ndesc\n---\nassignee: @x\n")
	if got.description != "desc" {
		t.Errorf("description = %q, want \"desc\" (the separator excludes it)", got.description)
	}
	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x", got.assignee)
	}
}

// With no separator there is no metadata: the fields stay at their zero value,
// not inherited from the body.
func TestParseEditFileWithoutMetadata(t *testing.T) {
	got := parse(t, "# t\ndesc\n")
	if got.assignee != "" {
		t.Errorf("assignee = %q, want empty without metadata", got.assignee)
	}
	if got.priority != 0 {
		t.Errorf("priority = %d, want 0 without metadata", got.priority)
	}
	if got.estimate != 0 {
		t.Errorf("estimate = %v, want 0 without metadata", got.estimate)
	}
	if len(got.tags) != 0 {
		t.Errorf("tags = %v, want none without metadata", got.tags)
	}
}

// Malformed values are ignored and the field stays at zero. That is the
// important part: a "priority: high" must not be stored as a priority.
func TestParseEditFileIgnoresMalformedValues(t *testing.T) {
	tests := []struct {
		name         string
		metadata     string
		wantPrio     int
		wantEstimate float64
		wantAssign   string
	}{
		{"non-numeric priority", "priority: high\n", 0, 0, ""},
		{"empty priority", "priority:\n", 0, 0, ""},
		{"negative priority is accepted", "priority: -1\n", -1, 0, ""},
		{"non-numeric estimate", "estimate: much\n", 0, 0, ""},
		{"negative estimate is discarded", "estimate: -2\n", 0, 0, ""},
		{"zero estimate is accepted", "estimate: 0\n", 0, 0, ""},
		{"empty assignee", "assignee:\n", 0, 0, ""},
		{"assignee with spaces", "assignee:    @x   \n", 0, 0, "@x"},
		{"unknown key", "status: doing\n", 0, 0, ""},
		{"empty metadata", "\n\n", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse(t, "# t\nd\n---\n"+tt.metadata)
			if got.priority != tt.wantPrio {
				t.Errorf("priority = %d, want %d", got.priority, tt.wantPrio)
			}
			if got.estimate != tt.wantEstimate {
				t.Errorf("estimate = %v, want %v", got.estimate, tt.wantEstimate)
			}
			if got.assignee != tt.wantAssign {
				t.Errorf("assignee = %q, want %q", got.assignee, tt.wantAssign)
			}
		})
	}
}

// The negative estimate is discarded but the negative priority is not: they
// are different rules and the test should tell them apart.
func TestParseEditFileEstimateNegativeButPriorityNegativeAllowed(t *testing.T) {
	got := parse(t, "# t\nd\n---\npriority: -3\nestimate: -1\n")
	if got.priority != -3 {
		t.Errorf("priority = %d, want -3 (accepted)", got.priority)
	}
	if got.estimate != 0 {
		t.Errorf("estimate = %v, want 0 (a negative estimate is no good)", got.estimate)
	}
}

// The space around the colon IS tolerated in the value, and the line's
// indentation is trimmed before comparing the key.
func TestParseEditFileToleratesSpacing(t *testing.T) {
	tests := []string{
		"# t\nd\n---\nassignee:@x\npriority:3\nestimate:2.5\ntags:a,b\n",
		"# t\nd\n---\nassignee:    @x    \npriority:   3   \n",
		"# t\nd\n---\n   assignee: @x\n   priority: 3\n",
	}
	for i, content := range tests {
		got := parse(t, content)
		if got.assignee != "@x" {
			t.Errorf("case %d: assignee = %q, want @x", i, got.assignee)
		}
		if got.priority != 3 {
			t.Errorf("case %d: priority = %d, want 3", i, got.priority)
		}
	}
}

// A space BEFORE the colon breaks key recognition. It is pinned down as
// current behavior because that is how editTemplate writes it, and because
// changing it would loosen the prefix and accept keys that are not part of
// the format.
func TestParseEditFileNeedsColonAttached(t *testing.T) {
	got := parse(t, "# t\nd\n---\nassignee : @x\npriority : 3\n")
	if got.assignee != "" {
		t.Errorf("assignee = %q, want empty with a space before the colon", got.assignee)
	}
	if got.priority != 0 {
		t.Errorf("priority = %d, want 0 with a space before the colon", got.priority)
	}
}

// The last time a key appears wins: the parser does not abort on the first
// match.
func TestParseEditFileLastKeyWins(t *testing.T) {
	got := parse(t, "# t\nd\n---\nassignee: @one\nassignee: @two\n")
	if got.assignee != "@two" {
		t.Errorf("assignee = %q, want @two (the last one wins)", got.assignee)
	}
}

// The body does not need a separator to have a description: the "---" only cuts.
func TestParseEditFileBodyWithoutTrailingNewline(t *testing.T) {
	got := parse(t, "# t\nd")
	if got.title != "t" {
		t.Errorf("title = %q, want t", got.title)
	}
	if got.description != "d" {
		t.Errorf("description = %q, want d", got.description)
	}
}

// A "---" inside the description cuts it, even if it is what the user
// wrote. It is the documented behavior of the format.
func TestParseEditFileFirstSeparatorWins(t *testing.T) {
	got := parse(t, "# t\nbefore\n---\nassignee: @x\nafter\n")
	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x", got.assignee)
	}
	if strings.Contains(got.description, "after") {
		t.Errorf("description = %q, must not go past the first separator", got.description)
	}
}

// editTaskCmd's output is read back without loss: the write/read cycle is
// what the external editor does, and if the parser loses a field it is lost
// silently.
func TestParseEditFileSurvivesItsOwnOutput(t *testing.T) {
	original := model.Task{
		ID: 7, Title: "Title with accents: ñ", Description: "line 1\nline 2",
		Assignee: "@john", Priority: model.PriorityMedium, Estimate: 2.5,
		Tags: []string{"api", "web"},
	}
	content := editTemplate(original)

	got := parse(t, content)
	if got.title != original.Title {
		t.Errorf("title = %q, want %q", got.title, original.Title)
	}
	if got.description != original.Description {
		t.Errorf("description = %q, want %q", got.description, original.Description)
	}
	if got.assignee != original.Assignee {
		t.Errorf("assignee = %q, want %q", got.assignee, original.Assignee)
	}
	if got.priority != original.Priority {
		t.Errorf("priority = %d, want %d", got.priority, original.Priority)
	}
	if got.estimate != original.Estimate {
		t.Errorf("estimate = %v, want %v", got.estimate, original.Estimate)
	}
	if strings.Join(got.tags, ",") != strings.Join(original.Tags, ",") {
		t.Errorf("tags = %v, want %v", got.tags, original.Tags)
	}
}

// currentProjectName: the filter rules in List and Kanban, and not in Dashboard.
func TestCurrentProjectName(t *testing.T) {
	projects := []model.Project{{Name: "api"}, {Name: "web"}}

	tests := []struct {
		name   string
		view   viewKind
		filter string
		want   string
	}{
		{"list without filter uses the first", viewList, "", "api"},
		{"list with filter uses the filter", viewList, "web", "web"},
		{"kanban without filter uses the first", viewKanban, "", "api"},
		{"kanban with filter uses the filter", viewKanban, "web", "web"},
		// In Dashboard the selection goes elsewhere, so the view's filter does
		// not apply: the form goes on the first project.
		{"dashboard ignores the filter", viewDashboard, "web", "api"},
		{"gantt ignores the filter", viewGantt, "web", "api"},
		{"no projects but with a filter", viewList, "web", "web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := currentProjectName(tt.view, tt.filter, projects); got != tt.want {
				t.Errorf("currentProjectName(%v, %q) = %q, want %q", tt.view, tt.filter, got, tt.want)
			}
		})
	}
}

func TestCurrentProjectNameWithoutProjects(t *testing.T) {
	if got := currentProjectName(viewList, "", nil); got != "" {
		t.Errorf("got %q, want empty with no projects", got)
	}
	if got := currentProjectName(viewList, "solo", nil); got != "solo" {
		t.Errorf("got %q, want \"solo\": the filter does not depend on the list", got)
	}
}

// The separator counts even at position 0: a file that starts directly with
// it has an empty body and all metadata.
func TestParseEditFileSeparatorAtPositionZero(t *testing.T) {
	got := parse(t, "---\nassignee: @x\npriority: 2\n")
	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x with the separator at the start", got.assignee)
	}
	if got.priority != 2 {
		t.Errorf("priority = %d, want 2 with the separator at the start", got.priority)
	}
	if got.description != "" {
		t.Errorf("description = %q, want empty: there is no body before the separator", got.description)
	}
}

// It cuts at the FIRST separator, not the last one. With two separators, the
// first one is what divides: the metadata is everything that comes after,
// including the second "---", which does not cut anything again.
func TestParseEditFileFirstSeparatorNotLast(t *testing.T) {
	got := parse(t, "# t\nbody\n---\nassignee: @x\n---\nsomething else\n")

	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x (the first separator divides)", got.assignee)
	}
	if got.description != "body" {
		t.Errorf("description = %q, want \"body\"", got.description)
	}
	if got.priority != 0 {
		t.Errorf("priority = %d, want 0: what follows the first separator is metadata, not body", got.priority)
	}
}

// The hash needs its space after it: "#no space" is not a title.
func TestParseEditFileHashNeedsTrailingSpace(t *testing.T) {
	got := parse(t, "#no space\nbody\n")
	if got.title != "" {
		t.Errorf("title = %q, want empty: '#' without a space does not open a title", got.title)
	}
	// And with no title, line 0 is still the title's position.
	if got.description != "body" {
		t.Errorf("description = %q, want \"body\"", got.description)
	}
}

// A description starting with "---" on its own line is interpreted as a
// separator: it is the price of a format based on a text marker.
func TestParseEditFileDescriptionWithSeparator(t *testing.T) {
	got := parse(t, "# t\ndesc\n\n---\nuser note\n")
	if got.description != "desc" {
		t.Errorf("description = %q, want \"desc\"", got.description)
	}
	if got.assignee != "" {
		t.Errorf("assignee = %q, want empty: what comes after the separator is not a key", got.assignee)
	}
}

// A "# " block with a single line does not invent a second one: the whole
// body stays empty instead of repeating the title.
func TestParseEditFileSingleLineTitleLeavesNoBody(t *testing.T) {
	got := parse(t, "# Only title")
	if got.title != "Only title" {
		t.Errorf("title = %q", got.title)
	}
	if strings.TrimSpace(got.description) != "" {
		t.Errorf("description = %q, want empty", got.description)
	}
}

// descEditorWidth discounts the borders and the indentation, with a floor of
// 1: a width of zero would leave the textarea with no room, which is worse than a very narrow one.
func TestDescEditorWidth(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  int
	}{
		{"roomy", 100, 96},
		{"tight", 5, 1},
		{"one less", 4, 1},
		{"zero", 0, 1},
		{"negative", -20, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{width: tt.width}
			if got := m.descEditorWidth(); got != tt.want {
				t.Errorf("descEditorWidth with %d = %d, want %d", tt.width, got, tt.want)
			}
		})
	}
}

// A title starting with a line break is truncated like any other: the result
// cannot carry the break inside, because the title ends up in a row of the list.
// It is the case where the break's index is exactly zero, which is what tells
// `>= 0` from `> 0`.
func TestParseEditFileTitleStartingWithNewline(t *testing.T) {
	got := parse(t, "# \ncontent on the second line")

	if got.title != "" {
		t.Errorf("the title is %q, want empty: the title line only had the prefix", got.title)
	}
	if !strings.Contains(got.description, "content on the second line") {
		t.Errorf("the content was not recovered:\\n%s", got.description)
	}
}

// A negative estimate in the file is not applied. That is what makes the
// `e >= 0` of the line have no second side to die on: the database rejects
// the negative, so the inner branch can never be used.
func TestParseEditFileIgnoresNegativeEstimate(t *testing.T) {
	got := parse(t, "# t\n\ndesc\n\n---\nassignee: @john\npriority: 2\nestimate: -3\n")

	if got.estimate < 0 {
		t.Errorf("estimate = %v, want not negative", got.estimate)
	}
	if got.estimate != 0 {
		t.Errorf("estimate = %v, want 0: a negative is not a valid estimate", got.estimate)
	}
}

// And a valid one, with decimals, does pass as-is.
func TestParseEditFileKeepsValidEstimate(t *testing.T) {
	got := parse(t, "# t\n\ndesc\n\n---\nassignee: @john\npriority: 2\nestimate: 2.5\n")
	if got.estimate != 2.5 {
		t.Errorf("estimate = %v, want 2.5", got.estimate)
	}
}

// An estimate of zero in the file IS applied, and it is the case that tells
// `e >= 0` from `e > 0`: both accept a positive, both reject a negative,
// and only zero separates them.
//
// The starting task has 2 days and the file says zero, so the observable
// result is 0 and not the default 0 of a task with no estimate.
func TestParseEditFileAppliesZeroEstimate(t *testing.T) {
	withZero := parse(t, "# t\n\ndesc\n\n---\nassignee: @john\npriority: 2\nestimate: 0\n")
	if withZero.estimate != 0 {
		t.Fatalf("parsed estimate = %v, want 0", withZero.estimate)
	}

	// And the full trip: editing a task with an estimate and leaving it at zero
	// stores it with zero, not with the previous value.
	m := newTestModel(t)
	task := m.tasks[0]
	if _, err := m.database.UpdateTask(task.ID, map[string]any{"estimate": 2}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	withTwo, err := m.database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if withTwo.Estimate != 2 {
		t.Fatalf("the starting estimate is %v, want 2: the test needs a task with an estimate", withTwo.Estimate)
	}

	content := fmt.Sprintf("# %s\n\n%s\n\n---\nassignee: %s\npriority: %d\nestimate: 0\ntags: \n",
		task.Title, task.Description, task.Assignee, task.Priority)
	title, desc, assignee, priority, estimate, _ := parseEditFile(content)
	// The real application path of an edit lives in the DB, so the same map
	// that it builds underneath is used.
	if _, err := m.database.UpdateTask(task.ID, map[string]any{
		"title": title, "description": desc, "assignee": assignee,
		"priority": priority, "estimate": estimate,
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	saved, err2 := m.database.GetTask(task.ID)
	if err2 != nil {
		t.Fatalf("GetTask: %v", err2)
	}
	if saved.Estimate != 0 {
		t.Errorf("the saved estimate is %v, want 0: an explicit zero is applied", saved.Estimate)
	}
}
