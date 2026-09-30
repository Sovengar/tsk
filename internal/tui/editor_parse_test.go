package tui

import (
	"strings"
	"testing"

	"tsk/internal/model"
)

// parseEditFile ya era una función pura, sólo que con tests débiles. El formato
// es "# título\\n\\n<descripción>\\n\\n---\\nassignee: …\\npriority: …\\nestimate: …
// \\ntags: …" y el parser es la frontera con el editor externo: lo que se
// equivoca aquí se guarda en la base de datos sin que nadie se entere.

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
	content := "# El título\n\nla descripción\n\n---\nassignee: @juan\npriority: 2\nestimate: 3.5\ntags: api,web\n"
	got := parse(t, content)

	if got.title != "El título" {
		t.Errorf("title = %q, want \"El título\"", got.title)
	}
	if got.description != "la descripción" {
		t.Errorf("description = %q, want \"la descripción\"", got.description)
	}
	if got.assignee != "@juan" {
		t.Errorf("assignee = %q, want @juan", got.assignee)
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

// El título es sólo la primera línea tras el "# ": el resto no es título aunque
// esté en la misma línea lógica.
func TestParseEditFileTitleStopsAtNewline(t *testing.T) {
	got := parse(t, "# uno\ndos\ntres\n")
	if got.title != "uno" {
		t.Errorf("title = %q, want \"uno\" (sólo la primera línea)", got.title)
	}
}

// Sin "# " no hay título, pero la PRIMERA LÍNEA se descarta igualmente: el
// parser la trata como la posición del título, no por el "# ". Eso significa que
// un cuerpo que empiece sin almohadilla pierde su primera línea.
func TestParseEditFileWithoutHashTitleDropsFirstLine(t *testing.T) {
	got := parse(t, "sin almohadilla\nla segunda\n")
	if got.title != "" {
		t.Errorf("title = %q, want vacío sin \"# \"", got.title)
	}
	if got.description != "la segunda" {
		t.Errorf("description = %q, want \"la segunda\" (la 1ª línea es la del título)", got.description)
	}
}

// Las líneas en blanco entre el título y la descripción se saltan; las del
// final se recortan con el TrimSpace.
func TestParseEditFileSkipsBlankLinesAfterTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"sin blancos", "# t\ndesc\n", "desc"},
		{"un blanco", "# t\n\ndesc\n", "desc"},
		{"varios blancos", "# t\n\n\n\ndesc\n", "desc"},
		{"blancos con espacios", "# t\n   \n\t\ndesc\n", "desc"},
		{"blanco al final", "# t\n\ndesc\n\n\n", "desc"},
		{"descripción multilínea", "# t\nlinea 1\nlinea 2\n", "linea 1\nlinea 2"},
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

// Una sola línea NO es descripción: es el título.
func TestParseEditFileSingleLineIsNotDescription(t *testing.T) {
	got := parse(t, "# sólo título\n")
	if got.description != "" {
		t.Errorf("description = %q, want vacía con una sola línea", got.description)
	}
}

// El separador "---" corta el cuerpo de los metadatos. Sin él, todo el
// contenido cuenta como cuerpo.
func TestParseEditFileSeparatorSplitsBody(t *testing.T) {
	got := parse(t, "# t\ndesc\n---\nassignee: @x\n")
	if got.description != "desc" {
		t.Errorf("description = %q, want \"desc\" (el separador la excluye)", got.description)
	}
	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x", got.assignee)
	}
}

// Sin separador no hay metadatos: los campos quedan en su valor cero, no
// heredados del cuerpo.
func TestParseEditFileWithoutMetadata(t *testing.T) {
	got := parse(t, "# t\ndesc\n")
	if got.assignee != "" {
		t.Errorf("assignee = %q, want vacío sin metadatos", got.assignee)
	}
	if got.priority != 0 {
		t.Errorf("priority = %d, want 0 sin metadatos", got.priority)
	}
	if got.estimate != 0 {
		t.Errorf("estimate = %v, want 0 sin metadatos", got.estimate)
	}
	if len(got.tags) != 0 {
		t.Errorf("tags = %v, want ninguno sin metadatos", got.tags)
	}
}

// Los valores mal formados se ignoran y el campo queda en cero. Es lo
// importante: un "priority: alto" no debe guardarse como prioridad.
func TestParseEditFileIgnoresMalformedValues(t *testing.T) {
	tests := []struct {
		name       string
		metadata   string
		wantPrio   int
		wantEst    float64
		wantAssign string
	}{
		{"priority no numérica", "priority: alto\n", 0, 0, ""},
		{"priority vacía", "priority:\n", 0, 0, ""},
		{"priority negativa se acepta", "priority: -1\n", -1, 0, ""},
		{"estimate no numérica", "estimate: mucho\n", 0, 0, ""},
		{"estimate negativa se descarta", "estimate: -2\n", 0, 0, ""},
		{"estimate cero se acepta", "estimate: 0\n", 0, 0, ""},
		{"assignee vacía", "assignee:\n", 0, 0, ""},
		{"assignee con espacios", "assignee:    @x   \n", 0, 0, "@x"},
		{"clave desconocida", "status: doing\n", 0, 0, ""},
		{"metadatos vacíos", "\n\n", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse(t, "# t\nd\n---\n"+tt.metadata)
			if got.priority != tt.wantPrio {
				t.Errorf("priority = %d, want %d", got.priority, tt.wantPrio)
			}
			if got.estimate != tt.wantEst {
				t.Errorf("estimate = %v, want %v", got.estimate, tt.wantEst)
			}
			if got.assignee != tt.wantAssign {
				t.Errorf("assignee = %q, want %q", got.assignee, tt.wantAssign)
			}
		})
	}
}

// El estimate negativo se descarta pero la priority negativa no: son reglas
// distintas y conviene que el test las distinga.
func TestParseEditFileEstimateNegativeButPriorityNegativeAllowed(t *testing.T) {
	got := parse(t, "# t\nd\n---\npriority: -3\nestimate: -1\n")
	if got.priority != -3 {
		t.Errorf("priority = %d, want -3 (se acepta)", got.priority)
	}
	if got.estimate != 0 {
		t.Errorf("estimate = %v, want 0 (un estimate negativo no sirve)", got.estimate)
	}
}

// El espacio alrededor de los dos puntos SÍ se tolera en el valor, y la
// indentación de la línea se recorta antes de comparar la clave.
func TestParseEditFileToleratesSpacing(t *testing.T) {
	tests := []string{
		"# t\nd\n---\nassignee:@x\npriority:3\nestimate:2.5\ntags:a,b\n",
		"# t\nd\n---\nassignee:    @x    \npriority:   3   \n",
		"# t\nd\n---\n   assignee: @x\n   priority: 3\n",
	}
	for i, content := range tests {
		got := parse(t, content)
		if got.assignee != "@x" {
			t.Errorf("caso %d: assignee = %q, want @x", i, got.assignee)
		}
		if got.priority != 3 {
			t.Errorf("caso %d: priority = %d, want 3", i, got.priority)
		}
	}
}

// Un espacio ANTES del dos puntos rompe el reconocimiento de la clave. Está
// fijado como comportamiento actual porque es como lo escribe editTemplate, y
// porque cambiarlo relajaría el prefijo y aceptaría claves que no son del
// formato.
func TestParseEditFileNeedsColonAttached(t *testing.T) {
	got := parse(t, "# t\nd\n---\nassignee : @x\npriority : 3\n")
	if got.assignee != "" {
		t.Errorf("assignee = %q, want vacío con espacio antes de los dos puntos", got.assignee)
	}
	if got.priority != 0 {
		t.Errorf("priority = %d, want 0 con espacio antes de los dos puntos", got.priority)
	}
}

// La última vez que aparece una clave gana: el parser no aborta al primer
// match.
func TestParseEditFileLastKeyWins(t *testing.T) {
	got := parse(t, "# t\nd\n---\nassignee: @uno\nassignee: @dos\n")
	if got.assignee != "@dos" {
		t.Errorf("assignee = %q, want @dos (la última gana)", got.assignee)
	}
}

// El cuerpo no necesita separador para tener descripción: el "---" sólo corta.
func TestParseEditFileBodyWithoutTrailingNewline(t *testing.T) {
	got := parse(t, "# t\nd")
	if got.title != "t" {
		t.Errorf("title = %q, want t", got.title)
	}
	if got.description != "d" {
		t.Errorf("description = %q, want d", got.description)
	}
}

// Un "---" dentro de la descripción la corta, aunque sea lo que el usuario
// escribiera. Es el comportamiento documentado del formato.
func TestParseEditFileFirstSeparatorWins(t *testing.T) {
	got := parse(t, "# t\nantes\n---\nassignee: @x\ndespués\n")
	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x", got.assignee)
	}
	if strings.Contains(got.description, "después") {
		t.Errorf("description = %q, no debería pasar del primer separador", got.description)
	}
}

// La salida de editTaskCmd se relee sin pérdida: el ciclo escribir/leer es lo
// que hace el editor externo, y si el parser pierde un campo se pierde en
// silencio.
func TestParseEditFileSurvivesItsOwnOutput(t *testing.T) {
	original := model.Task{
		ID: 7, Title: "Título con acentos: ñ", Description: "línea 1\nlínea 2",
		Assignee: "@juan", Priority: model.PriorityMedium, Estimate: 2.5,
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

// currentProjectName: el filtro manda en List y Kanban, y no en Dashboard.
func TestCurrentProjectName(t *testing.T) {
	projects := []model.Project{{Name: "api"}, {Name: "web"}}

	tests := []struct {
		name   string
		view   viewKind
		filter string
		want   string
	}{
		{"list sin filtro usa el primero", viewList, "", "api"},
		{"list con filtro usa el filtro", viewList, "web", "web"},
		{"kanban sin filtro usa el primero", viewKanban, "", "api"},
		{"kanban con filtro usa el filtro", viewKanban, "web", "web"},
		// En Dashboard la selección va por otro lado, así que el filtro de la
		// vista no aplica: el alta va sobre el primer proyecto.
		{"dashboard ignora el filtro", viewDashboard, "web", "api"},
		{"gantt ignora el filtro", viewGantt, "web", "api"},
		{"sin proyectos pero con filtro", viewList, "web", "web"},
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
		t.Errorf("got %q, want vacío sin proyectos", got)
	}
	if got := currentProjectName(viewList, "solo", nil); got != "solo" {
		t.Errorf("got %q, want \"solo\": el filtro no depende de la lista", got)
	}
}

// El separador cuenta aunque esté en la posición 0: un archivo que empieza
// directamente por él tiene cuerpo vacío y todo metadato.
func TestParseEditFileSeparatorAtPositionZero(t *testing.T) {
	got := parse(t, "---\nassignee: @x\npriority: 2\n")
	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x con el separador al principio", got.assignee)
	}
	if got.priority != 2 {
		t.Errorf("priority = %d, want 2 con el separador al principio", got.priority)
	}
	if got.description != "" {
		t.Errorf("description = %q, want vacía: no hay cuerpo antes del separador", got.description)
	}
}

// Corta por el PRIMER separador, no por el último. Con dos separadores, el
// primero es el que divide: los metadatos son todo lo que viene detrás,
// incluido el segundo "---", que no vuelve a cortar nada.
func TestParseEditFileFirstSeparatorNotLast(t *testing.T) {
	got := parse(t, "# t\ncuerpo\n---\nassignee: @x\n---\nalgo más\n")

	if got.assignee != "@x" {
		t.Errorf("assignee = %q, want @x (el primer separador divide)", got.assignee)
	}
	if got.description != "cuerpo" {
		t.Errorf("description = %q, want \"cuerpo\"", got.description)
	}
	if got.priority != 0 {
		t.Errorf("priority = %d, want 0: lo que sigue al primer separador es metadato, no cuerpo", got.priority)
	}
}

// La almohadilla necesita su espacio detrás: "#sin espacio" no es un título.
func TestParseEditFileHashNeedsTrailingSpace(t *testing.T) {
	got := parse(t, "#sin espacio\ncuerpo\n")
	if got.title != "" {
		t.Errorf("title = %q, want vacío: '#' sin espacio no abre título", got.title)
	}
	// Y al no haber título, la línea 0 sigue siendo la posición del título.
	if got.description != "cuerpo" {
		t.Errorf("description = %q, want \"cuerpo\"", got.description)
	}
}

// Una descripción que empieza por "---" en línea propia se interpreta como
// separador: es el precio de un formatobased en un marcador de texto.
func TestParseEditFileDescriptionWithSeparator(t *testing.T) {
	got := parse(t, "# t\ndesc\n\n---\nnota del usuario\n")
	if got.description != "desc" {
		t.Errorf("description = %q, want \"desc\"", got.description)
	}
	if got.assignee != "" {
		t.Errorf("assignee = %q, want vacío: lo de después del separador no es una clave", got.assignee)
	}
}
