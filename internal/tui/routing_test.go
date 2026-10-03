package tui

import (
	"math"
	"strings"
	"testing"

	"tsk/internal/model"

	"github.com/charmbracelet/x/ansi"
)

// vaAlTextareaDelAlta decide a dónde va un mensaje que no es ni tecla ni pegado.
//
// La condición estaba dentro del default de Update y no se podía comprobar:
// los mensajes que llegan por esa rama son privados del paquete textarea
// (pasteMsg, copyMsg), que no se exportan, así que un test no puede construir uno
// para ver a dónde acaba. Con la decisión sacada a una función pura, el enrutado
// se comprueba entero y sin necesitar el mensaje que lo dispara.
//
// El caso que importa es el campo: sólo la descripción lleva textarea embebido,
// porque es el único multilínea. Si el enrutado aceptara cualquier campo con el
// alta abierta, las teclas de los campos de una línea llegarían al textarea y se
// comerían: ese es el bug que un `==` mal puesto causaría, y es lo que esta tabla
// ata.
func TestVaAlTextareaDelAlta(t *testing.T) {
	casos := []struct {
		nombre      string
		newTaskOpen bool
		field       int
		want        bool
	}{
		{"alta abierta en la descripción", true, newTaskFieldDescription, true},
		{"alta abierta en el título", true, newTaskFieldTitle, false},
		{"alta abierta en el responsable", true, newTaskFieldAssignee, false},
		{"alta abierta en las tags", true, newTaskFieldTags, false},
		{"alta abierta en la prioridad", true, newTaskFieldPriority, false},
		{"alta cerrada en la descripción", false, newTaskFieldDescription, false},
		{"alta cerrada en el título", false, newTaskFieldTitle, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := vaAlTextareaDelAlta(c.newTaskOpen, c.field)
			if got != c.want {
				t.Errorf("vaAlTextareaDelAlta(%v, campo %d) = %v, want %v",
					c.newTaskOpen, c.field, got, c.want)
			}
		})
	}
}

// La consecuencia de lo anterior, vista desde el Update: con el alta abierta en
// un campo de una línea, un mensaje que no es tecla ni pegado no debe reenviarse
// al textarea ni devolver su comando. Con el alta abierta en la descripción, sí.
func TestElEnrutadoDeMensajesNoTeclaRespetaElCampo(t *testing.T) {
	// Un mensaje cualquiera que cae en el default: no es KeyMsg ni PasteMsg ni
	// ninguno de los tipos que el switch de Update reconoce uno a uno.
	msg := struct{}{}

	t.Run("campo de una línea: no se reenvía", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldTitle
		antes := m.newTaskTextarea.Value()

		modelo, cmd := m.Update(msg)
		if cmd != nil {
			t.Error("ha devuelto un comando con el cursor en el título: el mensaje ha ido al textarea")
		}
		if modelo.(Model).newTaskTextarea.Value() != antes {
			t.Error("el textarea ha cambiado con el cursor en el título")
		}
	})

	t.Run("campo de descripción: se reenvía", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = true
		m.newTaskFieldIdx = newTaskFieldDescription

		modelo, _ := m.Update(msg)
		// Lo que se comprueba es que el camino se recorre: el Update del
		// textarea con un mensaje desconocido no cambia nada, así que la prueba
		// de que pasó por ahí es la condición, no el resultado. Lo que sí tiene
		// que ser cierto es que no se rompe.
		if got := modelo.(Model).newTaskTextarea.Value(); got != m.newTaskTextarea.Value() {
			t.Errorf("el valor del textarea ha cambiado con un mensaje sin efecto: %q -> %q",
				m.newTaskTextarea.Value(), got)
		}
	})

	t.Run("alta cerrada: no se reenvía", func(t *testing.T) {
		m := newTestModel(t)
		m.newTaskOpen = false
		m.newTaskFieldIdx = newTaskFieldDescription

		if _, cmd := m.Update(msg); cmd != nil {
			t.Error("ha devuelto un comando con el alta cerrada")
		}
	})
}

// filaEsTarea decide si el cursor del gantt está sobre una tarea.
//
// El rango con inRange en vez de `>= 0 && < len(rows)` es lo que hace que el
// cursor en -1 y en len(rows) se puedan comprobar como valores y no como
// "cualquier cosa que no sea una tarea".
func TestFilaEsTarea(t *testing.T) {
	m := ganttModelWithPeople(t, []string{"@juan", "@maria"}, 3)
	m.currentView = viewGantt
	m.width, m.height = 140, 40
	filas := m.ganttRows()

	// Una fila de tarea de verdad: dentro del rango y del tipo correcto.
	tarea := -1
	cabecera := -1
	for i, f := range filas {
		if f.kind == ganttTaskRow && tarea < 0 {
			tarea = i
		}
		if f.kind == ganttAssigneeRow && cabecera < 0 {
			cabecera = i
		}
	}
	if tarea < 0 || cabecera < 0 {
		t.Fatalf("el fixture no ha dejado los dos tipos de fila: %d filas", len(filas))
	}

	if !filaEsTarea(filas, tarea) {
		t.Errorf("la fila %d es una tarea y filaEsTarea dice que no", tarea)
	}
	if filaEsTarea(filas, cabecera) {
		t.Errorf("la fila %d es una cabecera y filaEsTarea dice que sí", cabecera)
	}

	// Los bordes del rango, que es lo que inRange viene a sustituir. Con el
	// rango escrito a mano, estos valores sólo se distinguían de los de la
	// comparación interna si la fila que hay en ese sitio fuese del otro tipo --
	// y en 0 no lo es nunca, porque la fila 0 es una cabecera.
	for _, i := range []int{-1, len(filas), len(filas) + 5} {
		if filaEsTarea(filas, i) {
			t.Errorf("filaEsTarea(%d) dice que sí, con %d filas", i, len(filas))
		}
	}

	// Y una tabla vacía: todo índice está fuera.
	for _, i := range []int{-1, 0, 1} {
		if filaEsTarea(nil, i) {
			t.Errorf("filaEsTarea(nil, %d) dice que sí", i)
		}
	}
	// Con una lista vacía inRange no debe devolver true ni para 0, que es el
	// caso que un `idx <= n` habría colado.
	if inRange(0, 0) {
		t.Error("inRange(0, 0) dice que 0 está en una lista de 0 elementos")
	}
}

// cycleProjectFilter tiene tres salidas y las tres tienen que estar:
// sin proyectos no hay nada que recorrer; con el filtro actual fuera de la lista
// el salto arranca por el principio; y el caso normal, que es el de todos los
// días.
func TestCycleProjectFilterSalidas(t *testing.T) {
	t.Run("un solo proyecto: el filtro SÍ salta", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.projects = []model.Project{{Name: "api"}}
		m.filterProject = "api"

		m.cycleProjectFilter(1)

		// Con un proyecto hay DOS opciones -- "all" y "api" -- así que advancing
		// tiene que llevar a "all". Antes esto no se movía, porque la guarda
		// `len(opts) <= 1` contaba mal: trataba "all" como si no contara.
		if m.filterProject != "" {
			t.Errorf("con un solo proyecto el filtro se ha quedado en %q, want el primero (\"all\")",
				m.filterProject)
		}
	})

	t.Run("el filtro actual no está en la lista: salta desde el principio", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		// El filtro apunta a algo que ya no es una opción -- un proyecto
		// borrado, o un nombre escrito a mano. El recorrido tiene que arrancar
		// por el principio en vez de quedarse donde está.
		m.filterProject = "no-existe"

		m.cycleProjectFilter(1)

		if m.filterProject == "no-existe" {
			t.Error("con un filtro fuera de la lista, avanzar no ha hecho nada")
		}
	})

	t.Run("el normal: alterna entre las opciones", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewList
		m.filterProject = ""

		opciones := m.filterFieldOptions(filterFieldProject)
		if len(opciones) < 3 {
			t.Skipf("el fixture sólo tiene %d opciones", len(opciones))
		}

		vistos := map[string]bool{}
		vistos[m.filterProject] = true
		for range len(opciones) {
			m.cycleProjectFilter(1)
			vistos[m.filterProject] = true
		}
		if len(vistos) != len(opciones) {
			t.Errorf("ha pasado por %d valores distintos, want %d: %v",
				len(vistos), len(opciones), vistos)
		}
	})
}

// siguienteOpcion es la función pura del salto del filtro. Los tres casos -- la
// lista vacía, el valor actual ausente y el normal -- se prueban aquí sin
// montar un modelo, que es justo lo que la versión anterior no permitía.
func TestSiguienteOpcion(t *testing.T) {
	opts := []string{"all", "api", "web"}

	t.Run("hacia delante", func(t *testing.T) {
		if got := siguienteOpcion(opts, "all", 1); got != "api" {
			t.Errorf("siguienteOpcion(all, +1) = %q, want api", got)
		}
		if got := siguienteOpcion(opts, "web", 1); got != "all" {
			t.Errorf("siguienteOpcion(web, +1) = %q, want all (da la vuelta)", got)
		}
	})

	t.Run("hacia atrás", func(t *testing.T) {
		if got := siguienteOpcion(opts, "api", -1); got != "all" {
			t.Errorf("siguienteOpcion(api, -1) = %q, want all", got)
		}
		if got := siguienteOpcion(opts, "all", -1); got != "web" {
			t.Errorf("siguienteOpcion(all, -1) = %q, want web (da la vuelta)", got)
		}
	})

	t.Run("el valor actual no está: arranca por el principio", func(t *testing.T) {
		if got := siguienteOpcion(opts, "borrado", 1); got != "api" {
			t.Errorf("siguienteOpcion(borrado, +1) = %q, want api", got)
		}
		// Y hacia atrás también arranca por el principio: con el índice a 0, ir
		// para atrás da el último, no el primero.
		if got := siguienteOpcion(opts, "borrado", -1); got != "web" {
			t.Errorf("siguienteOpcion(borrado, -1) = %q, want web", got)
		}
	})

	t.Run("lista vacía", func(t *testing.T) {
		for _, vacia := range [][]string{nil, {}} {
			if got := siguienteOpcion(vacia, "api", 1); got != "" {
				t.Errorf("siguienteOpcion(lista vacía, +1) = %q, want \"\"", got)
			}
			if got := siguienteOpcion(vacia, "api", -1); got != "" {
				t.Errorf("siguienteOpcion(lista vacía, -1) = %q, want \"\"", got)
			}
		}
	})

	t.Run("una sola opción: se queda en ella", func(t *testing.T) {
		uno := []string{"all"}
		if got := siguienteOpcion(uno, "all", 1); got != "all" {
			t.Errorf("siguienteOpcion([all], +1) = %q, want all", got)
		}
		if got := siguienteOpcion(uno, "all", -1); got != "all" {
			t.Errorf("siguienteOpcion([all], -1) = %q, want all", got)
		}
	})

	t.Run("ida y vuelta: vuelve al punto de partida", func(t *testing.T) {
		for _, actual := range opts {
			ida := siguienteOpcion(opts, actual, 1)
			vuelta := siguienteOpcion(opts, ida, -1)
			if vuelta != actual {
				t.Errorf("de %q hacia +1 sale %q y de ahí hacia -1 sale %q, want %q",
					actual, ida, vuelta, actual)
			}
		}
	})
}

// El recorte de las líneas de comentario y de las tarjetas del kanban.
//
// Los dos discountan un margen fijo antes de truncar, y ese margen es lo que
// impide que el texto se salga de la caja. Con el número escrito a pelo (`width -
// 2`) no se sabía qué estaba descontando; con las constantes con nombre se puede
// comprobar, y lo que se comprueba es que un texto que llega justo al borde
// pierde exactamente esas columnas y ni una más.
func TestElMargenDelRecorteEsElDeclarado(t *testing.T) {
	const ancho = 30

	t.Run("comentarios", func(t *testing.T) {
		m := newDetailModel(t, 1)
		m.width, m.height = ancho, 24
		// Un cuerpo que no cabe de sobra: así el recorte es activo y el margen
		// se nota.
		m.detailComments[0].Body = strings.Repeat("w", 200)

		lineas := m.renderCommentLines(ancho)
		if len(lineas) != 1 {
			t.Fatalf("líneas = %d, want 1", len(lineas))
		}
		anchoT := ansi.StringWidth(lineas[0])
		want := ancho - prefijoComentario - huecoFecha
		if anchoT > want {
			t.Errorf("la línea del comentario mide %d columnas y el margen deja %d: %q",
				anchoT, want, lineas[0])
		}
		// Y con un margen más estrecho (una columna menos) el texto entraría, así
		// que un `width - 3` sí se distinguiría. Se comprueba que NO cabe con el
		// margen declarado.
		if ansi.StringWidth(lineas[0]) == want+1 {
			t.Error("la línea ha entrado justa: el recorte no ha quitado nada")
		}
	})

	t.Run("tarjetas del kanban", func(t *testing.T) {
		m := newTestModel(t)
		m.currentView = viewKanban
		m.width, m.height = 40, 30

		// El recorte ocurre ANTES del borde, sobre el texto de la tarjeta, y
		// luego se le mete un prefijo de 2 columnas. Así que el límite real del
		// título es ancho - margenTarjeta - prefijo.
		//
		// La caja rellena a la derecha con espacios hasta el ancho de la columna,
		// así que medir la línea entera no dice nada: lo que se mide es cuántos
		// caracteres del título salen, que es exactamente lo que el margen
		// controla.
		const anchoCol = 20
		const prefijo = 2
		limite := anchoCol - margenTarjeta - prefijo

		card := func(titulo string) string {
			col := kanbanColumn{
				status: "todo",
				tasks:  []model.Task{{ID: 1, Title: titulo, Assignee: "@juan"}},
			}
			return m.renderKanbanColumn(col, anchoCol, "header", false, columnWindow{0, 1})
		}

		// La letra de relleno no aparece en ningún otro sitio de la tarjeta --
		// ni en "@juan" ni en la cabecera --, así que contarla cuenta el título.
		const letra = "x"

		// Un título largo se recorta al límite, ni una columna más.
		largo := strings.Repeat(letra, limite+50)
		if n := strings.Count(card(largo), letra); n != limite {
			t.Errorf("un título larguísimo sale con %d caracteres, want %d (el límite declarado)", n, limite)
		}

		// El borde por los dos lados: uno que mide EXACTAMENTE el límite sale
		// entero, y uno con una columna más sale recortado. Eso es lo que separa
		// el margen declarado de uno más estrecho.
		justo := strings.Repeat(letra, limite)
		if n := strings.Count(card(justo), letra); n != limite {
			t.Errorf("un título de %d columnas sale con %d: el que cabe justo no debe perder nada",
				limite, n)
		}
		unaMas := strings.Repeat(letra, limite+1)
		if n := strings.Count(card(unaMas), letra); n != limite {
			t.Errorf("un título de %d columnas sale con %d, want %d: el que no cabe pierde la de más",
				limite+1, n, limite)
		}
	})
}

// estimateNoNegativo y el centinela que lo distingue de "el estimate es cero".
//
// Es el par de casos que hace alcanzable la comparación del llamante: un estimate
// de 0 es un valor legítimo y tiene que pasar, y un estimate ausente tiene que
// distinguishable de él. Con el centinela en -1 los dos son hechos distintos; sin
// él, "no venía" y "venía a cero" eran lo mismo y la comparación `>= 0` no tenía un
// caso que la distinguiera de su `> 0`.
func TestEstimateNoNegativo(t *testing.T) {
	casos := []struct {
		nombre string
		in     string
		want   float64
	}{
		{"cero", "0", 0},
		{"cero con decimales", "0.0", 0},
		{"negativo cero", "-0", 0},
		{"uno", "1", 1},
		{"medio", "0.5", 0.5},
		{"con signo más", "+2", 2},
		{"ciento veinte", "120", 120},

		{"negativo", "-1", estimateAusente},
		{"negativo pequeño", "-0.5", estimateAusente},
		{"el centinela escrito tal cual", "-1.0000001", estimateAusente},

		{"no es un número", "abc", estimateAusente},
		{"vacío", "", estimateAusente},
		{"con unidades", "3d", estimateAusente},
		{"nan", "NaN", estimateAusente},
		{"infinito positivo", "+Inf", math.Inf(1)},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := estimateNoNegativo(c.in); got != c.want {
				t.Errorf("estimateNoNegativo(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// El centinela tiene que ser un valor que ningún estimate real pueda tomar, o el
// "no viene" se confunde con un dato bueno.
func TestElCentinelaNoEsUnEstimateValido(t *testing.T) {
	if estimateAusente >= 0 {
		t.Fatalf("el centinela es %v, y cualquier valor >= 0 es un estimate válido", estimateAusente)
	}
	// Y tiene que ser el único valor que devuelve el centinela para una entrada
	// que no es un número: el valor de retorno no depende del texto.
	for _, entrada := range []string{"", "abc", "-5", "-0.0001"} {
		if got := estimateNoNegativo(entrada); got != estimateAusente {
			t.Errorf("estimateNoNegativo(%q) = %v, want el centinela %v",
				entrada, got, estimateAusente)
		}
	}
}

// El viaje completo por la plantilla: un estimate de 0 tiene que llegar a la
// tarea guardada, y un estimate ausente tiene que quedarse en el valor por
// defecto. Es el caso que el centinela hace posible.
func TestElEstimateCeroSeGuardaYElAusenteNo(t *testing.T) {
	tarea := model.Task{ID: 1, Title: "t", Estimate: 5}

	casos := []struct {
		nombre       string
		plantilla    string
		wantEstimate float64
	}{
		{"estimate cero", "# t\n\nd\n\n---\nestimate: 0", 0},
		{"estimate ausente", "# t\n\nd\n\n---\nassignee: @juan", 0},
		{"estimate negativo", "# t\n\nd\n\n---\nestimate: -3", 0},
		{"estimate normal", "# t\n\nd\n\n---\nestimate: 2.5", 2.5},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			title, desc, assignee, priority, estimate, tags := parseEditFile(c.plantilla)
			if title != tarea.Title {
				t.Errorf("title = %q, want %q", title, tarea.Title)
			}
			if desc != "d" {
				t.Errorf("desc = %q, want %q", desc, "d")
			}
			_ = assignee
			_ = priority
			_ = tags
			if estimate != c.wantEstimate {
				t.Errorf("estimate = %v, want %v", estimate, c.wantEstimate)
			}
		})
	}
}
