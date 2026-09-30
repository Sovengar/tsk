package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// runJSON ejecuta un comando que debe tener éxito y devuelve su payload JSON.
func runJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	out, code := run(t, args...)
	if code != 0 {
		t.Fatalf("Run(%v) salió con %d: %s", args, code, out)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("Run(%v) no devolvió JSON: %v (%q)", args, err, out)
	}
	return v
}

// wantError ejecuta un comando que debe fallar y comprueba el código de salida.
func wantError(t *testing.T, args ...string) {
	t.Helper()
	if _, code := run(t, args...); code != 1 {
		t.Errorf("Run(%v) = %d, want 1", args, code)
	}
}

// seed crea un proyecto con el workflow por defecto.
func seed(t *testing.T, name string) {
	t.Helper()
	if _, code := run(t, "project", "add", name); code != 0 {
		t.Fatalf("project add %s falló", name)
	}
}

// addTask crea una tarea y devuelve su id.
func addTask(t *testing.T, project, title string, extra ...string) int64 {
	t.Helper()
	args := append([]string{"add", title, "--project", project}, extra...)
	payload := runJSON(t, args...)
	task, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("add %q sin payload de tarea: %v", title, payload)
	}
	id, ok := task["id"].(float64)
	if !ok {
		t.Fatalf("tarea sin id: %v", task)
	}
	return int64(id)
}

func taskField(t *testing.T, payload map[string]any, field string) any {
	t.Helper()
	task, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("payload sin tarea: %v", payload)
	}
	return task[field]
}

// strList normaliza un campo JSON lista: la API devuelve null (no []) cuando el
// slice de Go es nil, y ambos significan "vacío" para quien consume la salida.
func strList(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, item.(string))
		}
		return out
	default:
		return nil
	}
}

func str(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("valor %v (%T) no es string", v, v)
	}
	return s
}

// ---- project ----

func TestProjectAddAndList(t *testing.T) {
	withTempDB(t)

	seed(t, "api")
	payload := runJSON(t, "project", "list", "--json")
	projects, ok := payload["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("project list --json = %v", payload)
	}
	if name := str(t, projects[0].(map[string]any)["name"]); name != "api" {
		t.Errorf("name = %q, want api", name)
	}

	// Salida humana: cabecera y total.
	out, _ := run(t, "project", "list")
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "api") {
		t.Errorf("project list humano = %q", out)
	}
}

func TestProjectListEmpty(t *testing.T) {
	withTempDB(t)
	if out, _ := run(t, "project", "list"); !strings.Contains(out, "No projects registered.") {
		t.Errorf("lista vacía = %q", out)
	}
	if out, _ := run(t, "project", "list", "--archived"); !strings.Contains(out, "No archived projects.") {
		t.Errorf("lista de archivados vacía = %q", out)
	}
}

func TestProjectAddCustomWorkflow(t *testing.T) {
	withTempDB(t)
	runJSON(t, "project", "add", "api", "--workflow", "backlog,todo,reviewing,done")
	runJSON(t, "project", "add", "web", "--workflow", "backlog,todo,reviewing,done",
		"--list-order", "reviewing,todo,backlog,done")

	api := runJSON(t, "project", "show", "api", "--json")
	p := api["project"].(map[string]any)
	if p["name"] != "api" {
		t.Errorf("name = %v", p["name"])
	}
	if got := p["workflow"].([]any); len(got) != 4 || got[0] != "backlog" {
		t.Errorf("workflow = %v", got)
	}
	// Sin --list-order explicito el campo sale null, no []: CreateProject
	// recibe un slice nil y json.Marshal lo serializa como "null", que al
	// releerse deja el slice en nil. Es el contrato actual de la API; un
	// consumidor que asuma .list_order.length se rompe con ese null.
	if got := p["list_order"]; got != nil {
		t.Errorf("list_order implicito = %v, want null", got)
	}

	web := runJSON(t, "project", "show", "web", "--json")
	wp := web["project"].(map[string]any)
	if got := wp["list_order"].([]any); len(got) != 4 || got[0] != "reviewing" {
		t.Errorf("list_order = %v", got)
	}
}

func TestProjectShowHumanOutput(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	out, _ := run(t, "project", "show", "api")
	for _, want := range []string{"Project:  api", "Workflow:", "(workflow order)", "Archived: false"} {
		if !strings.Contains(out, want) {
			t.Errorf("project show = %q, falta %q", out, want)
		}
	}
	wantError(t, "project", "show", "nope")
}

func TestProjectUpdate(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")

	runJSON(t, "project", "update", "api", "--name", "api-v2")
	if payload := runJSON(t, "project", "show", "api-v2", "--json"); payload["project"].(map[string]any)["name"] != "api-v2" {
		t.Errorf("rename falló: %v", payload)
	}

	runJSON(t, "project", "update", "api-v2", "--workflow", "backlog,todo,done")
	runJSON(t, "project", "update", "api-v2", "--list-order", "done,todo,backlog")
	p := runJSON(t, "project", "show", "api-v2", "--json")["project"].(map[string]any)
	if got := p["list_order"].([]any); len(got) != 3 || got[0] != "done" {
		t.Errorf("list_order tras update = %v", got)
	}

	// Nombre duplicado y workflow inválido son errores.
	wantError(t, "project", "update", "api-v2", "--name", "web")
	wantError(t, "project", "update", "api-v2", "--workflow", "sin,duplicados,sin")
}

func TestProjectArchiveLifecycle(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	runJSON(t, "project", "archive", "api")

	if out, _ := run(t, "project", "list"); strings.Contains(out, "api") {
		t.Errorf("proyecto archivado sigue en la lista: %q", out)
	}
	archived := runJSON(t, "project", "list", "--archived", "--json")
	if got := archived["projects"].([]any); len(got) != 1 {
		t.Errorf("lista de archivados = %v", got)
	}

	runJSON(t, "project", "unarchive", "api")
	if out, _ := run(t, "project", "list"); !strings.Contains(out, "api") {
		t.Errorf("tras desarchivar debería volver: %q", out)
	}
	wantError(t, "project", "archive", "nope")
	wantError(t, "project", "unarchive", "nope")
}

func TestProjectRemove(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	runJSON(t, "project", "remove", "api")
	wantError(t, "project", "remove", "api")
	wantError(t, "project", "add")
}

// ---- add / list / show ----

func TestAddRequiresProject(t *testing.T) {
	withTempDB(t)
	wantError(t, "add")
	wantError(t, "add", "sin proyecto")
}

func TestAddAppliesEveryFlag(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	payload := runJSON(t, "add", "con todo", "--project", "api",
		"--priority", "2", "--assignee", "@ana", "--status", "todo",
		"--estimate", "2.5", "--tag", "bug,urgent")
	task := payload["task"].(map[string]any)

	if task["priority"].(float64) != 2 {
		t.Errorf("priority = %v, want 2", task["priority"])
	}
	if task["assignee"] != "@ana" {
		t.Errorf("assignee = %v", task["assignee"])
	}
	if task["status"] != "todo" {
		t.Errorf("status = %v, want todo", task["status"])
	}
	if task["estimate"].(float64) != 2.5 {
		t.Errorf("estimate = %v, want 2.5", task["estimate"])
	}
	tags := strList(task["tags"])
	if len(tags) != 2 || tags[0] != "bug" || tags[1] != "urgent" {
		t.Errorf("tags = %v, want [bug urgent]", tags)
	}
}

func TestAddIgnoresMalformedNumericFlags(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	// Un --priority no numérico se ignora (priority 0), igual que un estimate
	// negativo: el comando no debe fallar, simplemente no lo aplica.
	task := runJSON(t, "add", "raro", "--project", "api",
		"--priority", "abc", "--estimate", "-3")["task"].(map[string]any)
	if task["priority"].(float64) != 0 {
		t.Errorf("priority = %v, want 0", task["priority"])
	}
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0 (negativo ignorado)", task["estimate"])
	}
}

func TestAddRejectsBadInput(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	wantError(t, "add", "x", "--project", "no-existe")
	wantError(t, "add", "x", "--project", "api", "--status", "no-existe")
}

func TestListFilters(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")

	addTask(t, "api", "a1", "--assignee", "@ana", "--tag", "bug")
	addTask(t, "api", "a2", "--assignee", "@bob")
	addTask(t, "web", "w1", "--status", "todo")

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"todo", []string{"list", "--json"}, []string{"a1", "a2", "w1"}},
		{"por proyecto", []string{"list", "--project", "api", "--json"}, []string{"a1", "a2"}},
		{"por estado", []string{"list", "--status", "todo", "--json"}, []string{"w1"}},
		{"por assignee", []string{"list", "--assignee", "@ana", "--json"}, []string{"a1"}},
		{"por tag", []string{"list", "--tag", "bug", "--json"}, []string{"a1"}},
		{"tag inexistente", []string{"list", "--tag", "nope", "--json"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			titles := listTitles(t, tt.args...)
			if len(titles) != len(tt.want) {
				t.Fatalf("titles = %v, want %v", titles, tt.want)
			}
			for i, want := range tt.want {
				if titles[i] != want {
					t.Errorf("titles[%d] = %q, want %q (orden del workflow: %v)", i, titles[i], want, titles)
				}
			}
		})
	}
}

func listTitles(t *testing.T, args ...string) []string {
	t.Helper()
	tasks, ok := runJSON(t, args...)["tasks"].([]any)
	if !ok {
		t.Fatalf("list %v sin tasks", args)
	}
	titles := make([]string, 0, len(tasks))
	for _, item := range tasks {
		titles = append(titles, str(t, item.(map[string]any)["title"]))
	}
	return titles
}

func TestListHumanOutput(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	addTask(t, "api", "visible")

	out, _ := run(t, "list")
	if !strings.Contains(out, "ID") || !strings.Contains(out, "visible") {
		t.Errorf("list humano = %q", out)
	}
	if !strings.Contains(out, "Total: 1 tasks") {
		t.Errorf("falta el total: %q", out)
	}

	if out, _ := run(t, "list", "--project", "vacio"); !strings.Contains(out, "No tasks found.") {
		t.Errorf("lista vacía = %q", out)
	}
}

func TestShowIncludesComments(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "con comentario")
	runJSON(t, "comment", "add", strconv.FormatInt(id, 10), "hola", "mundo")

	payload := runJSON(t, "show", strconv.FormatInt(id, 10))
	if str(t, payload["task"].(map[string]any)["title"]) != "con comentario" {
		t.Errorf("task = %v", payload["task"])
	}
	// El cuerpo se reúne con espacios: varios args son un solo comentario.
	comments := payload["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments = %v", comments)
	}
	if body := str(t, comments[0].(map[string]any)["body"]); body != "hola mundo" {
		t.Errorf("body = %q, want %q", body, "hola mundo")
	}

	wantError(t, "show", "9999")
	wantError(t, "show", "abc")
}

// ---- transiciones ----

func TestMoveAndShorthands(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	tests := []struct {
		name       string
		args       []string
		wantStatus string
	}{
		{"move", []string{"move", "", "todo"}, "todo"},
		{"done", []string{"done", ""}, "done"},
		{"cancel", []string{"cancel", ""}, "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := addTask(t, "api", tt.name)
			args := append([]string{}, tt.args...)
			args[1] = strconv.FormatInt(id, 10)
			payload := runJSON(t, args...)
			if got := str(t, taskField(t, payload, "status")); got != tt.wantStatus {
				t.Errorf("status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestStartReportsNextStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "arrancable")

	payload := runJSON(t, "start", strconv.FormatInt(id, 10))
	if got := str(t, taskField(t, payload, "status")); got != "todo" {
		t.Errorf("start → %q, want todo", got)
	}
	// El default workflow sigue backlog,todo,doing,..., así que propose doing.
	if next := str(t, payload["next"]); next != "doing" {
		t.Errorf("next = %q, want doing", next)
	}
}

func TestReviewRequiresReviewStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "con review")
	if got := str(t, taskField(t, runJSON(t, "review", strconv.FormatInt(id, 10)), "status")); got != "reviewing" {
		t.Errorf("review → %q, want reviewing", got)
	}

	// Un proyecto sin estado de revisión no puede pasar por review.
	runJSON(t, "project", "add", "web", "--workflow", "backlog,todo,done")
	wid := addTask(t, "web", "sin review")
	wantError(t, "review", strconv.FormatInt(wid, 10))
}

func TestMoveRejectsUnknownStatus(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "x")
	wantError(t, "move", strconv.FormatInt(id, 10), "no-existe")
	wantError(t, "move", "9999", "todo")
}

// ---- update ----

func TestUpdateFieldsAndTags(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "antes", "--tag", "viejo")
	arg := strconv.FormatInt(id, 10)

	payload := runJSON(t, "update", arg, "--title", "despues",
		"--description", "notas", "--priority", "3",
		"--assignee", "@bob", "--estimate", "1.5")
	task := payload["task"].(map[string]any)
	if task["title"] != "despues" || task["description"] != "notas" {
		t.Errorf("title/description = %v/%v", task["title"], task["description"])
	}
	if task["priority"].(float64) != 3 || task["assignee"] != "@bob" || task["estimate"].(float64) != 1.5 {
		t.Errorf("priority/assignee/estimate = %v/%v/%v",
			task["priority"], task["assignee"], task["estimate"])
	}

	// --tags reemplaza, --tag añade, --untag quita.
	task = runJSON(t, "update", arg, "--tag", "nuevo")["task"].(map[string]any)
	if tags := strList(task["tags"]); len(tags) != 2 {
		t.Errorf("tras --tag = %v, want 2 tags", tags)
	}
	task = runJSON(t, "update", arg, "--untag", "viejo")["task"].(map[string]any)
	if tags := strList(task["tags"]); len(tags) != 1 || tags[0] != "nuevo" {
		t.Errorf("tras --untag = %v", tags)
	}
	task = runJSON(t, "update", arg, "--tags", "a,b")["task"].(map[string]any)
	if tags := strList(task["tags"]); len(tags) != 2 || tags[0] != "a" {
		t.Errorf("tras --tags = %v, want [a b]", tags)
	}
}

func TestUpdateNoFlagsIsANoop(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "igual")
	task := runJSON(t, "update", strconv.FormatInt(id, 10))["task"].(map[string]any)
	if task["title"] != "igual" {
		t.Errorf("title = %v, want igual", task["title"])
	}
	wantError(t, "update", "9999", "--title", "x")
}

// ---- comments ----

func TestCommentLifecycle(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "tarea")
	arg := strconv.FormatInt(id, 10)

	created := runJSON(t, "comment", "add", arg, "primero")
	commentID := int64(created["comment"].(map[string]any)["id"].(float64))

	if got := runJSON(t, "comment", "list", arg)["comments"].([]any); len(got) != 1 {
		t.Errorf("list = %v", got)
	}
	runJSON(t, "comment", "remove", strconv.FormatInt(commentID, 10))
	if got := runJSON(t, "comment", "list", arg)["comments"].([]any); len(got) != 0 {
		t.Errorf("tras remove = %v, want vacío", got)
	}

	wantError(t, "comment")
	wantError(t, "comment", "nope")
	wantError(t, "comment", "add")
	wantError(t, "comment", "add", arg)
	wantError(t, "comment", "list")
	wantError(t, "comment", "remove")
	wantError(t, "comment", "remove", "9999")
}

func TestCommentListEmpty(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "sin comentarios")
	// El slice vacío debe venir como [] y no como null: se consume desde JSON.
	got := runJSON(t, "comment", "list", strconv.FormatInt(id, 10))["comments"]
	if list, ok := got.([]any); !ok || len(list) != 0 {
		t.Errorf("comments = %v, want lista vacía", got)
	}
}

// ---- offday ----

func TestOffDayLifecycle(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	created := runJSON(t, "offday", "add", "@ana", "2026-07-28", "2026-08-01", "--note", "vacaciones")
	off := created["offday"].(map[string]any)
	if off["start_date"] != "2026-07-28" || off["end_date"] != "2026-08-01" {
		t.Errorf("rango = %v→%v", off["start_date"], off["end_date"])
	}
	if off["note"] != "vacaciones" {
		t.Errorf("note = %v", off["note"])
	}

	// Sin end ni note: un solo día.
	one := runJSON(t, "offday", "add", "@bob", "2026-07-28")["offday"].(map[string]any)
	if one["start_date"] != "2026-07-28" || one["end_date"] != "2026-07-28" {
		t.Errorf("un solo día = %v→%v", one["start_date"], one["end_date"])
	}

	if got := runJSON(t, "offday", "list", "--json")["offdays"].([]any); len(got) != 2 {
		t.Errorf("list = %v, want 2", got)
	}
	if got := runJSON(t, "offday", "list", "--assignee", "@ana", "--json")["offdays"].([]any); len(got) != 1 {
		t.Errorf("list filtrado = %v, want 1", got)
	}

	out, _ := run(t, "offday", "list")
	if !strings.Contains(out, "ASSIGNEE") || !strings.Contains(out, "@ana") {
		t.Errorf("offday list humano = %q", out)
	}

	id := int64(off["id"].(float64))
	runJSON(t, "offday", "remove", strconv.FormatInt(id, 10))
	if got := runJSON(t, "offday", "list", "--json")["offdays"].([]any); len(got) != 1 {
		t.Errorf("tras remove = %v, want 1", got)
	}
}

func TestOffDayValidations(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	wantError(t, "offday")
	wantError(t, "offday", "nope")
	wantError(t, "offday", "add")
	wantError(t, "offday", "add", "unassigned", "2026-07-28")
	wantError(t, "offday", "add", "@ana", "no-es-fecha")
	wantError(t, "offday", "remove")
	wantError(t, "offday", "remove", "9999")

	if out, _ := run(t, "offday", "list"); !strings.Contains(out, "No off-days registered.") {
		t.Errorf("offday list vacío = %q", out)
	}
}

// ---- gantt / stats / help ----

func TestGanttOutputs(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")
	addTask(t, "api", "de ana", "--assignee", "@ana", "--estimate", "2")
	addTask(t, "web", "de bob", "--assignee", "@bob")

	// El payload ES el schedule, sin envoltorio.
	sched := runJSON(t, "gantt", "--json", "--weeks", "2")
	if str(t, sched["start"]) == "" {
		t.Errorf("schedule sin start: %v", sched)
	}

	// Los filtros son de VISTA: ocultan a la persona pero la cola no se recalcula.
	tests := []struct {
		name       string
		args       []string
		wantPeople []string
	}{
		{"sin filtro", []string{"gantt", "--json", "--weeks", "2"}, []string{"@ana", "@bob"}},
		{"por proyecto", []string{"gantt", "--json", "--weeks", "2", "--project", "api"}, []string{"@ana"}},
		{"por assignee", []string{"gantt", "--json", "--weeks", "2", "--assignee", "@bob"}, []string{"@bob"}},
		{"proyecto y assignee a la vez", []string{"gantt", "--json", "--weeks", "2", "--project", "web", "--assignee", "@ana"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, a := range runJSON(t, tt.args...)["assignees"].([]any) {
				got = append(got, str(t, a.(map[string]any)["assignee"]))
			}
			if len(got) != len(tt.wantPeople) {
				t.Fatalf("assignees = %v, want %v", got, tt.wantPeople)
			}
			for i := range got {
				if got[i] != tt.wantPeople[i] {
					t.Errorf("assignee[%d] = %q, want %q", i, got[i], tt.wantPeople[i])
				}
			}
		})
	}

	out, _ := run(t, "gantt", "--weeks", "2")
	if !strings.Contains(out, "Gantt") {
		t.Errorf("gantt texto = %q", out)
	}
}

// TestGanttWeeksInvalidFallsBackToConfig: un --weeks no numerico o no positivo
// deja weeks en 0 y se usa el valor de la config. Si el fallback no ocurriera,
// el calendario saldria con 0 dias y un Gantt vacio que el comando reportaria
// como si fuera un filtro sin resultados.
func TestGanttWeeksInvalidFallsBackToConfig(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	addTask(t, "api", "tarea", "--assignee", "@ana")

	for _, weeks := range []string{"cero", "0", "-2", "no-es-numero"} {
		t.Run(weeks, func(t *testing.T) {
			if got := runJSON(t, "gantt", "--json", "--weeks", weeks)["assignees"].([]any); len(got) != 1 {
				t.Errorf("assignees = %v, want 1", got)
			}
			// El eje del texto tiene que tener dias: 0 semanas = 0 celdas.
			out, _ := run(t, "gantt", "--weeks", weeks)
			axis := strings.Split(out, "\n")[ganttHeadOff+1]
			if got := len([]rune(axis)) - ganttLabelW - 1; got <= 0 {
				t.Errorf("weeks=%q cayo a 0 dias en vez de usar la config: %q", weeks, axis)
			}
		})
	}
}

func TestStats(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	seed(t, "web")
	addTask(t, "api", "a1", "--assignee", "@ana")
	addTask(t, "api", "a2", "--assignee", "@ana", "--status", "todo")
	addTask(t, "web", "w1", "--assignee", "@bob")

	stats := runJSON(t, "stats", "--json")["stats"].(map[string]any)
	if stats["total"].(float64) != 3 {
		t.Errorf("total = %v, want 3", stats["total"])
	}

	byProject := runJSON(t, "stats", "--json", "--project", "api")["stats"].(map[string]any)
	if byProject["total"].(float64) != 2 {
		t.Errorf("total de api = %v, want 2", byProject["total"])
	}

	out, _ := run(t, "stats")
	for _, want := range []string{"Total: 3 tasks", "By Status:", "By Assignee:", "@ana"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats = %q, falta %q", out, want)
		}
	}
}

func TestHelpListsCommands(t *testing.T) {
	withTempDB(t)
	out, _ := run(t, "help")
	for _, want := range []string{"Projects:", "Tasks:", "add", "list", "gantt", "stats"} {
		if !strings.Contains(out, want) {
			t.Errorf("help = %q, falta %q", out, want)
		}
	}
}

func TestMigrateIsANoop(t *testing.T) {
	withTempDB(t)
	if _, code := run(t, "migrate"); code != 0 {
		t.Errorf("migrate salió con %d", code)
	}
}

// TestProjectSubcommandUsageAtExactlyTwoArgs: el guard de args de los
// subcomandos de `project` es `len(args) < 2`, así que el borde que lo separa
// de `<= 2` es un subcomando con exactamente 2 tokens. Con 1 debe fallar y con
// 2 no.
func TestProjectSubcommandUsageAtExactlyTwoArgs(t *testing.T) {
	withTempDB(t)
	for _, sub := range []string{"show", "update"} {
		wantError(t, "project", sub)
	}
	seed(t, "api")
	// Exactamente 3 tokens (subcomando + nombre): es el otro lado del borde de
	// `len(args) < 2` y tiene que seguir siendo un comando válido.
	if _, code := run(t, "project", "show", "api"); code != 0 {
		t.Errorf("project show api salió con %d", code)
	}
	if _, code := run(t, "project", "update", "api"); code != 0 {
		t.Errorf("project update api salió con %d", code)
	}
	if _, code := run(t, "project", "show", "api", "--json"); code != 0 {
		t.Errorf("project show api --json salió con %d", code)
	}
}

// TestUpdateEstimateZeroOverwrites pins el borde `e >= 0` del flag --estimate:
// con 0 el valor NO es "no informado", es un estimate explícito de cero días. Si
// el flag se ignorase, la tarea conservaría el estimate anterior.
func TestUpdateEstimateZeroOverwrites(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "con días", "--estimate", "3")
	arg := strconv.FormatInt(id, 10)

	task := runJSON(t, "update", arg, "--estimate", "0")["task"].(map[string]any)
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0 (un 0 explícito pisa al anterior)", task["estimate"])
	}
	// Y sigue siendo cero tras releer, no un estimate sin aplicar.
	again := runJSON(t, "show", arg)["task"].(map[string]any)
	if again["estimate"].(float64) != 0 {
		t.Errorf("estimate persistido = %v, want 0", again["estimate"])
	}
}

// TestAddEstimateZeroIsValid: en el alta, --estimate 0 deja el estimate a 0
// (que es el valor por defecto de la columna), sin error.
func TestAddEstimateZeroIsValid(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	task := runJSON(t, "add", "sin días", "--project", "api", "--estimate", "0")["task"].(map[string]any)
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0", task["estimate"])
	}
}

// TestUpdateTagsFlagsAreGreedy documenta la semántica actual del parser de
// flags: un flag con valor se come el siguiente token SEA CUAL SEA, incluso si
// parece otro flag. `update --tag --untag` acaba añadiendo una tag llamada
// "--untag". No es lo ideal (un parser real daría error por falta de valor),
// pero está fijado aquí a propósito: si algún día se cambia, este test lo dice.
func TestUpdateTagsFlagsAreGreedy(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "tags", "--tag", "uno")
	arg := strconv.FormatInt(id, 10)

	task := runJSON(t, "update", arg, "--tag", "--untag")["task"].(map[string]any)
	tags := strList(task["tags"])
	if len(tags) != 2 || tags[0] != "uno" || tags[1] != "--untag" {
		t.Errorf("tags = %v, want [uno --untag] (parser greedy)", tags)
	}

	// Un flag huerfano al final si se ignora: no hay token que consumirse.
	task = runJSON(t, "update", arg, "--untag")["task"].(map[string]any)
	if got := strList(task["tags"]); len(got) != 2 {
		t.Errorf("tags = %v, want las 2 intactas", got)
	}

	// --tags con lista vacia SI vacia las tags (semantica de reemplazo).
	task = runJSON(t, "update", arg, "--tags", "")["task"].(map[string]any)
	if got := strList(task["tags"]); len(got) != 0 {
		t.Errorf("--tags vacio = %v, want sin tags", got)
	}
}

// TestGanttWeeksExactDayCount fija el número exacto de días por --weeks. Con el
// `--weeks` válido, la rejilla tiene 7 días por semana; si el flag se ignorara
// (el mutador del `err == nil`) o se leyera al revés, caería a la config.
func TestGanttWeeksExactDayCount(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	addTask(t, "api", "tarea", "--assignee", "@ana")

	for weeks, days := range map[string]int{"1": 7, "2": 14, "4": 28} {
		t.Run(weeks, func(t *testing.T) {
			out, _ := run(t, "gantt", "--weeks", weeks)
			axis := strings.Split(out, "\n")[ganttHeadOff+1]
			if got := len([]rune(axis)) - ganttLabelW - 1; got != days {
				t.Errorf("--weeks %s → %d días, want %d (%q)", weeks, got, days, axis)
			}
		})
	}
}

// TestAddLastFlagWinsAndZeroIsExplicit: con dos --estimate, gana el último. Y un
// 0 explícito tiene que pisar un valor anterior: `e >= 0` distingue 0 de "no
// informado" y por eso el borde importa.
func TestAddLastFlagWinsAndZeroIsExplicit(t *testing.T) {
	withTempDB(t)
	seed(t, "api")

	task := runJSON(t, "add", "x", "--project", "api",
		"--estimate", "2", "--estimate", "0")["task"].(map[string]any)
	if task["estimate"].(float64) != 0 {
		t.Errorf("estimate = %v, want 0 (gana el último flag)", task["estimate"])
	}

	task = runJSON(t, "add", "y", "--project", "api",
		"--priority", "3", "--priority", "0")["task"].(map[string]any)
	if task["priority"].(float64) != 0 {
		t.Errorf("priority = %v, want 0", task["priority"])
	}
}

// TestUpdateWithoutFlagsIsAnExactNoop documenta que `update <id>` sin flags no
// escribe nada. El guard `if len(updates) > 0` existe justo para eso: sin él la
// llamada abriría un UPDATE que sólo mueve updated_at.
func TestUpdateWithoutFlagsIsAnExactNoop(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "quieto", "--tag", "conservada")
	arg := strconv.FormatInt(id, 10)

	before := runJSON(t, "show", arg)["task"].(map[string]any)
	runJSON(t, "update", arg)
	after := runJSON(t, "show", arg)["task"].(map[string]any)

	if before["title"] != after["title"] || before["updated_at"] != after["updated_at"] {
		t.Errorf("update sin flags cambió algo: %v → %v", before["updated_at"], after["updated_at"])
	}
	if got := strList(after["tags"]); len(got) != 1 || got[0] != "conservada" {
		t.Errorf("tags = %v, want [conservada]", got)
	}
}
