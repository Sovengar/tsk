package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/model"
)

// capture redirige stdout/stderr/exit durante el test y devuelve el buffer de
// stdout. exit no mata el proceso: lanza un sentinel que run() deja escapar,
// así el comando se detiene exactamente donde lo detendría el proceso real.
type exitPanic struct{ code int }

func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	origOut, origErr, origExit := stdout, stderr, exit
	stdout, stderr = out, &bytes.Buffer{}
	exit = func(code int) { panic(exitPanic{code: code}) }
	t.Cleanup(func() { stdout, stderr, exit = origOut, origErr, origExit })
	return out
}

// run ejecuta Run(args) capturando la salida. Devuelve el código de salida: 0
// si el comando terminó sin outputError, 1 si lo invocó.
func run(t *testing.T, args ...string) (stdoutText string, code int) {
	t.Helper()
	out := capture(t)
	code = 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(exitPanic)
				if !ok {
					panic(r)
				}
				code = e.code
			}
		}()
		Run(args)
	}()
	return out.String(), code
}

// withTempDB apunta la config a una base de datos temporal y devuelve su ruta.
func withTempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tsk.db")
	cfg := filepath.Join(dir, "config.toml")
	body := "[database]\npath = \"" + dbPath + "\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", cfg)
	return dbPath
}

func TestRunUnknownCommandFallsBackToTUI(t *testing.T) {
	// Sin argumentos, o con algo que no es un subcomando, Run devuelve false
	// para que main lance la TUI.
	for _, args := range [][]string{
		{},
		{"tsk"},
		{"nope"},
		{"--unknown"},
		{"Project"}, // el dispatch es case-sensitive
	} {
		if got := Run(args); got {
			t.Errorf("Run(%v) = true, want false (debe caer a la TUI)", args)
		}
	}
}

func TestRunHandlesKnownCommands(t *testing.T) {
	withTempDB(t)
	tests := []struct {
		name string
		args []string
	}{
		{"help", []string{"help"}},
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
		{"project add", []string{"project", "add", "api"}},
		{"add", []string{"add", "primera", "--project", "api"}},
		{"list", []string{"list"}},
		{"stats", []string{"stats"}},
		{"migrate", []string{"migrate"}},
		{"completion bash", []string{"completion", "bash"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, code := run(t, tt.args...); code != 0 {
				t.Errorf("Run(%v) salió con %d", tt.args, code)
			}
		})
	}
}

func TestRunMissingArgsIsAnError(t *testing.T) {
	tests := [][]string{
		{"show"},
		{"update"},
		{"move"},
		{"start"},
		{"review"},
		{"done"},
		{"cancel"},
	}
	for _, args := range tests {
		if _, code := run(t, args...); code != 1 {
			t.Errorf("Run(%v) = %d, want 1 (usage error)", args, code)
		}
	}
}

func TestParseIDRejectsGarbage(t *testing.T) {
	capture(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("parseID(\"abc\") deberia haber llamado a outputError")
		}
	}()
	parseID("abc")
}

func TestParseIDAcceptsNumbers(t *testing.T) {
	capture(t)
	for in, want := range map[string]int64{"1": 1, "42": 42, "-7": -7} {
		if got := parseID(in); got != want {
			t.Errorf("parseID(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTruncateLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		w    int
		want string
	}{
		{"cabe entero", "abc", 5, "abc"},
		{"cabe exacto", "abc", 3, "abc"},
		{"corta con puntos", "abcdefgh", 5, "abc.."},
		{"ancho 1", "abc", 1, "a"},
		{"ancho 2", "abc", 2, "ab"},
		{"ancho 0", "abc", 0, ""},
		{"utf-8 por runas", "áéíóú", 3, "á.."},
		{"vacío", "", 3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateLabel(tt.in, tt.w); got != tt.want {
				t.Errorf("truncateLabel(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		in   string
		w    int
		want string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"abc", 0, "abc"},
		{"", 2, "  "},
		{"áé", 4, "áé  "},
	}
	for _, tt := range tests {
		if got := padRight(tt.in, tt.w); got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
		}
	}
}

func TestRenderGanttTextInvalidStart(t *testing.T) {
	if got := renderGanttText(&model.Schedule{Start: "no-es-fecha"}, 4); got != "" {
		t.Errorf("renderGanttText con start inválido = %q, want \"\"", got)
	}
}

func TestRenderGanttText(t *testing.T) {
	s := &model.Schedule{
		Start: "2026-09-14",
		Assignees: []model.AssigneeSchedule{{
			Assignee: "@a",
			End:      "2026-09-16",
			Entries: []model.ScheduleEntry{
				{Task: model.Task{ID: 1, Title: "primera"}, Start: "2026-09-14", End: "2026-09-15", Estimate: 2},
				{Task: model.Task{ID: 2, Title: "segunda"}, Start: "2026-09-16", End: "2026-09-16", Estimate: 1, EstimateDefaulted: true},
			},
		}},
		Unassigned: []model.Task{{ID: 9, Title: "sin dueño"}},
	}
	out := renderGanttText(s, 3)

	for _, want := range []string{
		"Gantt · start 2026-09-14 · 3 weeks",
		"@a  ends 2026-09-16",
		"#1 primera",
		"#2 segunda",
		"2026-09-14→2026-09-15 [2d]",
		"2026-09-16→2026-09-16 [1d] ~", // la tilde marca el estimate por defecto
		"Unassigned (1):",
		"#9 sin dueño",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("salida sin %q:\n%s", want, out)
		}
	}

	// La barra de 2 días ocupa dos celdas y la de 1 día una.
	if !strings.Contains(out, "██") {
		t.Errorf("no hay barra de 2 días:\n%s", out)
	}
	// La regla lleva la etiqueta de semana del lunes de arranque.
	if !strings.Contains(out, "3SEP") {
		t.Errorf("falta la etiqueta de semana:\n%s", out)
	}
}

func TestRenderGanttTextNoUnassignedHidesSection(t *testing.T) {
	out := renderGanttText(&model.Schedule{Start: "2026-09-14"}, 1)
	if strings.Contains(out, "Unassigned") {
		t.Errorf("sin tareas sin dueño no debe imprimir la sección:\n%s", out)
	}
}

func TestOutputJSONShape(t *testing.T) {
	out := capture(t)
	outputJSON(map[string]any{"ok": true})
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout no es JSON válido: %v (%q)", err, out.String())
	}
	if got["ok"] != true {
		t.Errorf("payload = %v", got)
	}
}

func TestCompletionShells(t *testing.T) {
	withTempDB(t)
	for _, sh := range []string{"bash", "zsh", "fish"} {
		out, code := run(t, "completion", sh)
		if code != 0 {
			t.Errorf("completion %s salió con %d", sh, code)
		}
		if !strings.Contains(out, "tsk") {
			t.Errorf("completion %s no menciona tsk: %q", sh, out)
		}
	}
	if out, _ := run(t, "completion"); !strings.Contains(out, "bash") {
		t.Errorf("completion sin shell debería listar los shells: %q", out)
	}
}

// runErr es run pero devolviendo stderr, que es donde outputError escribe. Los
// errores de escritura se leen ahí; la salida normal, en stdout.
func runErr(t *testing.T, args ...string) (stderrText string, code int) {
	t.Helper()
	out := &bytes.Buffer{}
	origOut, origErr, origExit := stdout, stderr, exit
	stdout, stderr = &bytes.Buffer{}, out
	exit = func(c int) { panic(exitPanic{c}) }
	t.Cleanup(func() { stdout, stderr, exit = origOut, origErr, origExit })

	code = 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(exitPanic)
				if !ok {
					panic(r)
				}
				code = e.code
			}
		}()
		Run(args)
	}()
	return out.String(), code
}
