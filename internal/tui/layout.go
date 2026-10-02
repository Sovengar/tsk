package tui

import (
	"fmt"
	"sort"
	"strings"

	"tsk/internal/config"
	"tsk/internal/model"

	"github.com/charmbracelet/x/ansi"
)

// ganttFixedLabelWidth es lo que ocupa la etiqueta del Gantt con sitio de sobra.
const ganttFixedLabelWidth = 30

// ganttLabelThreshold son las columnas interiores a partir de las cuales la
// etiqueta se queda con su ancho fijo en vez de con un tercio del total.
const ganttLabelThreshold = 70

// ganttMinLabelWidth es el mínimo de la etiqueta, para que un nombre quepa.
const ganttMinLabelWidth = 14

// ganttMinDayCols es el mínimo de columnas de día: una semana entera.
const ganttMinDayCols = 7

// minContentHeight es el alto mínimo que se reserva para el contenido de la
// vista, para que la caja no se degrade en terminales chicas.
const minContentHeight = 8

// lineCount devuelve cuántas filas ocupa un bloque de texto.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// truncateLines corta cada línea al ancho dado. Evita que el helper de bordes
// re-wrapée una línea que no entra y agregue filas de más, lo que rompería el
// cálculo de alto.
func truncateLines(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// min en vez de comparar: cuando la línea mide justo lo que el ancho, las
		// dos ramas dan el mismo texto, así que la comparación era otro mutante
		// equivalente. Recortar a lo que ya cabe no cambia nada.
		lines[i] = ansi.Truncate(line, min(ansi.StringWidth(line), width), "")
	}
	return strings.Join(lines, "\n")
}

// cellWidth ajusta un texto al ancho de display indicado: lo trunca con ".." si
// sobra y lo rellena con espacios si falta. Mide columnas de pantalla, no bytes,
// así que las celdas con color (ANSI) quedan alineadas con las que no. Es lo que
// reemplaza a %-Ns de fmt, que cuenta bytes y desalinea las celdas coloreadas.
func cellWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	s = ansi.Truncate(s, width, "..")
	// Rellenar con un max en vez de con un if: con pad cero el if no hacía nada y
	// el max repite "", que es lo mismo sin la rama que mutar.
	//
	// El suelo en 0 es por strings.Repeat, que revienta con un número negativo.
	// ansi.Truncate nunca devuelve una celda más ancha que el límite, así que no
	// se puede llegar: es la misma clase de suelo defensivo que los otros de este
	// fichero, y sus dos mutantes -- quitarlo o ponerlo en -1 -- son equivalentes
	// por el mismo motivo.
	return s + strings.Repeat(" ", max(width-ansi.StringWidth(s), 0))
}

// modalInnerWidth es el ancho útil de un modal: el ancho total menos los dos
// caracteres del borde izquierdo y el derecho.
//
// modalWidthFor deja siempre al menos dos columnas de margen (si no, el modal se
// pegaría al borde de la pantalla), así que el resultado es positivo. El suelo
// en 1 cubre el caso en que totalWidth venga ya recortado por otro camino.
//
// Estaba escrito cinco veces, y cada copia era un sitio donde un "- 2" mutado
// pasaba desapercibido: el texto se recortaba un par de columnas más o menos de
// ancho y nadie lo notaba salvo mirando el render con atención.
func modalInnerWidth(totalWidth int) int {
	return max(totalWidth-2, 1)
}

// contentBudget calcula las filas disponibles para el contenido de la vista
// descontando el preview y la barra de keybinds.
func contentBudget(total, previewH, keybindsH int) int {
	budget := total - previewH - keybindsH
	budget = max(budget, minContentHeight)
	return budget
}

// nextCommentSel avanza la selección de comentarios sin dar la vuelta: desde
// -1 (nada seleccionado) salta al primero, y arriving al último se queda ahí.
//
// Es lo que hace la tecla "j" del detalle, escrito como min/max para que la
// regla sea comprobable: envoltura por arriba, y -1 tratado como "anterior al
// primero" en vez de como una posición válida.
func nextCommentSel(sel, n int) int {
	return min(max(sel+1, 0), max(n-1, 0))
}

// prevCommentSel retrocede la selección. En el primer comentario vuelve al
// estado "nada seleccionado" (-1) y ahí se queda: el detalle no envuelve.
func prevCommentSel(sel int) int {
	return max(sel-1, -1)
}

// resolvePageSize devuelve el tamaño de página a usar, o el de por defecto si la
// configuración no trae uno utilizable.
//
// El suelo está en 0 y no en 1 porque un ListPageSize negativo o cero no es un
// "una sola página" pedido por el usuario, es una configuración ausente: la
// config nunca falla, avisa y sigue, y aquí se aplica el mismo criterio.
func resolvePageSize(n int) int {
	if n > 0 {
		return n
	}
	return config.DefaultPageSize
}

// editorCommand devuelve el editor externo a lanzar, o nvim si la configuración
// no trae ninguno.
//
// Estaba escrito tres veces, con el mismo literal en las tres. Cada copia era su
// propio sitio donde un mutante podía cambiar el editor por defecto sin que nada
// lo notara, porque las tres sólo se ejecutaban cuando el detalle estaba abierto.
func editorCommand(cmd string) string {
	if cmd == "" {
		return "nvim"
	}
	return cmd
}

// ---- Aritmética de índices -------------------------------------------------
//
// Estas tres funciones existen porque la misma cuenta aparecía escrita en varios
// sitios (navegación de List, de Kanban y del filtro de proyectos) con su
// propio borde. Repetida, cada copia es un sitio donde un mutante puede colgar
// el bucle y donde ningún test alcanza. Al sacarlas a funciones puras el
// contrato queda en un sitio y se puede comprobar exhaustivamente, sin montar
// una vista entera ni una base de datos.

// cycleIndex mueve idx dentro de [0, n) dando la vuelta por el final.
//
// n <= 0 devuelve idx sin tocar: no hay lista a la que moverse y es mejor dejar
// el índice como estaba que inventar un 0. delta puede ser negativo.
func cycleIndex(idx, n, delta int) int {
	if n <= 0 {
		return idx
	}
	return ((idx+delta)%n + n) % n
}

// shiftIndex mueve idx dentro de [0, n) SIN dar la vuelta: se queda en el
// extremo en lugar de saltar al otro lado.
func shiftIndex(idx, n, delta int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx+delta, 0), n-1)
}

// inRange dice si idx es un índice válido dentro de una lista de n elementos.
// Es la guarda que aparece antes de cada tasks[idx] del repo.
func inRange(idx, n int) bool {
	return idx >= 0 && idx < n
}

// taskAt devuelve la tarea en la posición idx, o nil si el índice no es válido.
//
// Se extrajo porque la guarda "cursor < len(tasks)" estaba escrita delante de
// cada tasks[m.cursor] con su propia forma. Su mutante de BOUNDARY (que pasa `<`
// a `<=`) sólo se puede matar si la función recibe el índice: desde el teclado el
// cursor siempre llega acotado, así que la diferencia es inalcanzable. Con el
// índice como parámetro se puede probar directamente el caso idx == len(tasks).
func taskAt(tasks []model.Task, idx int) *model.Task {
	if !inRange(idx, len(tasks)) {
		return nil
	}
	return &tasks[idx]
}

// clampKanban mantiene el cursor del board dentro de lo que hay.
//
// colLens es el número de tarjetas de cada columna, en orden. Se pasa como
// parámetro en vez de leerlo del modelo para que el contrato sea comprobable sin
// construir un board: la regla (columna dentro, fila dentro de esa columna) es
// lo que se quiere fijar, no el render.
//
// El motivo de que haga falta: filtrar por estado cambia el número de columnas
// y con él el número de tarjetas de cada una, así que un índice que era válido
// hace un momento puede quedarse fuera sin que nada lo haya tocado.
func clampKanban(col, row int, colLens []int) (int, int) {
	if len(colLens) == 0 {
		return 0, 0
	}
	col = min(max(col, 0), len(colLens)-1)
	n := colLens[col]
	if n == 0 {
		return col, 0
	}
	// La fila se acota por los dos lados. El inferior no lo acotaba el código
	// anterior, pero un kanbanRow negativo indexaría por detrás: el clamp que
	// promete "el cursor está dentro del board" tiene que cumplirlo también
	// abajo, aunque hoy ningún camino del teclado produzca un negativo.
	return col, min(max(row, 0), n-1)
}

// kanbanMaxCards es cuántas tarjetas se dibujan en una columna con el alto dado.
//
// Cada tarjeta ocupa kanbanCardRows filas, y el +1 del cociente hace que la
// última se cuente aunque sólo entre su parte superior: mejor una tarjeta
// recortada abajo que un hueco vacío en el fondo de la columna.
//
// El mínimo de una tarjeta garantiza que un terminal diminuto siga mostrando
// algo en vez de degenerar en un tablero vacío.
//
// No hay un mínimo de filas de contenido porque no haría nada: con 0, 1 o 2
// filas el cociente da 0 o 1 y el mínimo de una tarjeta lo resuelve igual. Ese
// suelo era una rama que ningún test podía distinguir, igual que el
// `if x < N { x = N }` de otras partes del código.
func kanbanMaxCards(maxHeight int) int {
	content := maxHeight - kanbanBoardChrome - filterHeaderRows
	return max((content+1)/kanbanCardRows, 1)
}

// kanbanHeader rotula una columna. Cuando sólo se ve una parte de las tarjetas
// dice cuántas de cuántas; si caben todas, sólo el total. El ancho del rótulo
// es el mínimo de la columna, así que la diferencia de texto se nota.
func kanbanHeader(status string, shown, total int) string {
	if shown < total {
		return fmt.Sprintf("─ %s (%d/%d) ", status, shown, total)
	}
	return fmt.Sprintf("─ %s (%d) ", status, total)
}

// ---- Reglas del modal de personas ---------------------------------------

// firstValidIndex devuelve idx si vale para una lista de n, y 0 si no. Es el
// "si el índice no vale, el primero" sin mirar los elementos: lo que necesitan
// los ciclos que después eligen por índice.
func firstValidIndex(idx, n int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx, 0), n-1)
}

// listWindowForHeight recorta la página actual al alto disponible, moviendo la
// ventana para que el cursor quede dentro.
//
// Es el tercer uso de visibleRange en el programa (los otros son Kanban, Gantt y
// el filtro), y el único que además tiene que|traducir el cursor de índice global
// a relativo dentro de la página antes de usarlo. Con pageStart y pageEnd
// iguales, la ventana es la página entera.
//
// Se extrajo del render por lo mismo que las otras: son cuatro operaciones con
// dos condiciones, y sin sacarlas no hay forma de comprobar el recorte sin
// montar la vista entera.
func listWindowForHeight(pageStart, pageEnd, cursor, maxHeight int) (int, int) {
	start, end := pageStart, pageEnd

	// visible son las filas que quedan para tareas una vez descontadas las fijas
	// de la caja. Con zero o menos no cabe ni una, y se pinta la página entera:
	// es preferible que la caja desborde a que salga vacía.
	visible := maxHeight - listFixedRows
	if visible <= 0 || end-start <= visible {
		return start, end
	}

	relStart, relEnd := visibleRange(cursor-pageStart, end-pageStart, visible)
	return pageStart + relStart, pageStart + relEnd
}

// separatorWidth es lo que mide el separador horizontal de la barra de filtros:
// el ancho interior menos las dos columnas del recuadro.
//
// El suelo en 0 no es decorativo. strings.Repeat con un número negativo revienta,
// y w-2 es negativo en cuanto la ventana baja de dos columnas. Hoy m.width nunca
// baja de eso, pero una función que admite un int y peta con el negativo es una
// trampa para el siguiente llamante.
func separatorWidth(innerW int) int {
	return max(innerW-2, 0)
}

// listContentWidth es el ancho que queda para las columnas de la lista una vez
// descontado el prefijo de selección ("> " o "  "). Es el mismo "- 2" que el
// separador y por el mismo motivo: los dos reserving el recuadro.
func listContentWidth(innerW int) int {
	return separatorWidth(innerW)
}

// ganttLabelAndDays reparte el ancho interior del Gantt entre la etiqueta de la
// izquierda y las columnas de día de la derecha.
//
// Dos regímenes: con sitio de sobra la etiqueta se queda con 30 columnas
// fijas; por debajo de 70 columnas interiores se queda con un tercio, porque 30
// columns se comerían la mitad del gráfico. Ambos tienen suelo: 14 de etiqueta
// para que el nombre quepa, 7 de días para que quepa una semana.
//
// Estuvo cuatro líneas en el render con sus dos umbrales y sus dos suelos. La
// caja del Gantt rellena con espacios hasta el ancho interior, así que un "- 1"
// en los días no se ve en el ancho de la línea: se ve en dónde caen los rótulos
// de los lunes, y eso es demasiado indirecto para un test. Como función pura el
// reparto se comprueba entero sobre un barrido de anchos.
func ganttLabelAndDays(innerW int) (labelW, dayCols int) {
	labelW = ganttFixedLabelWidth
	if innerW < ganttLabelThreshold {
		labelW = innerW / 3
	}
	labelW = max(labelW, ganttMinLabelWidth)

	dayCols = max(innerW-labelW-1, ganttMinDayCols)
	return labelW, dayCols
}

// detailHeightBudget reparte el alto del detalle entre comentarios y
// descripción.
//
// fixed son las líneas que no son contenido: bordes de las dos cajas, la
// metadata, el separador y el rótulo de Description. avail es lo que queda, con
// un mínimo de 3 para que la caja tenga sentido aunque el terminal sea minúsculo.
//
// De avail se reservan 2 líneas para el separador y el rótulo de los comentarios
// antes de decidir cuántas caben: maxCommentLines. El presupuesto real es el
// número de comentarios, acotado entre 1 y maxCommentLines (al menos una línea,
// o el "(no comments)"). El resto va a la descripción, también con un mínimo de
// una línea.
//
// Se extrajo porque eran seis operaciones encadenadas dentro del render, con
// cuatro suelos distintos, y ninguna se podía comprobar sin montar el modal.
func detailHeightBudget(maxHeight, commentCount int) (commentBudget, descBudget int) {
	const fixed = 12
	avail := max(maxHeight-fixed, 3)

	maxCommentLines := max(avail-2, 1)
	commentBudget = min(max(commentCount, 1), maxCommentLines)
	// Lo que sobra para la descripción no necesita suelo: los comentarios se
	// quedan como mucho en avail-2, así que siempre quedan al menos 2 líneas. Un
	// max(..., 1) aquí era una rama que no se podía activar.
	descBudget = avail - commentBudget
	return commentBudget, descBudget
}

// truncateAt devuelve s recortada a n bytes, o el propio s si es más corta.
//
// Los bytes y no los runes a propósito: son timestamps RFC3339, que son ASCII.
// En un timestamp todos los "caracteres" son de un byte y un slice por runes
// sólo añadiría una conversión sin efecto.
//
// n <= 0 devuelve vacío. Sin ese suelo, s[:-1] revienta: los dos llamantes
// pasan constantes positivas, así que hoy es inalcanzable, pero una función que
// admite un int y peta con un negativo es una trampa esperando a un llamante
// nuevo.
// El suelo en 0 es un max y no un if: a n == 0 el slice ya devuelve "", así que
// la rama era indistinguible de no tenerla.
func truncateAt(s string, n int) string {
	return s[:min(len(s), max(n, 0))]
}

// clampTo acota un índice a [0, n). Con n <= 0 devuelve 0: sin lista no hay
// posición, y 0 es lo que todos los llamantes muestran como "nada".
//
// Es la forma que repetían clampFilterOption, clampAssigneeIdx, clampOffdayIdx y
// el clamp del cursor de Kanban, cada uno con su propio borde y su propia
// especialización del caso "no hay lista".
func clampTo(idx, n int) int {
	if n <= 0 {
		return 0
	}
	return min(max(idx, 0), n-1)
}

// taskAt2 es taskAt para listas de texto: el elemento en la posición idx, o ""
// si el índice no es válido.
//
// El valor vacío es el que los callers ya trataban como "nada seleccionado", y
// centralizarlo evita el `if idx >= 0 && idx < len(s)` repetido delante de cada
// suggestions[idx] del programa.
func taskAt2(items []string, idx int) string {
	if !inRange(idx, len(items)) {
		return ""
	}
	return items[idx]
}

// firstOrAt devuelve items[idx], o el primer elemento si el índice no vale para
// esa lista.
//
// Es la regla de los desplegables al tabular: si no hay nada seleccionado, se
// completa con lo primero que hay. Distinto de taskAt2, que devuelve "" en ese
// caso; por eso son dos funciones y no una con un flag.
func firstOrAt(items []string, idx int) string {
	if len(items) == 0 {
		return ""
	}
	return items[min(max(idx, 0), len(items)-1)]
}

// currentProjectName es el proyecto sobre el que se opera: el del filtro si
// lo hay, y si no el primero de la lista.
//
// El filtro sólo cuenta en List y Kanban: en Dashboard la selección va aparte y
// el alta de tarea se hace sobre el primer proyecto, que es lo que se ve.
//
// Se extrajo como función pura porque la regla del "primer proyecto" estaba
// escrita dentro de un switch sobre la vista, y sus bordes (sin proyectos,
// filtro puesto en una vista que no lo usa) sólo se alcanzaban desde el teclado.
func currentProjectName(view viewKind, filterProject string, projects []model.Project) string {
	switch view {
	case viewList, viewKanban:
		if filterProject != "" {
			return filterProject
		}
	}
	if len(projects) > 0 {
		return projects[0].Name
	}
	return ""
}

// buildAssigneeRoster construye el listado de personas del modal: cada una con
// su total de tareas, cuántas siguen activas y cuántos off-days tiene.
//
// "Me" está siempre, aunque no tenga nada asignado: es el valor por defecto al
// que se atribuyen las tareas nuevas, así que si no apareciera, highlighted en
// el modal valdría por defecto.
//
// Se salta a quien no tenga responsable (vacío o "unassigned"): sin nombre no
// hay a quién atribuirle un off-day. Devuelve la lista ordenada por nombre, que
// es lo que hace estable el índice al reordenar el mapa.
// buildAssigneeRoster resume las tareas y los off-days por persona.
//
// El resultado nunca está vacío: "Me" se mete siempre, aunque no haya ninguna
// tarea sin responsable. Eso es a propósito — siempre hay alguien a quien mirar,
// y con eso el modal nunca se abre sin contenido. Los llamantes que preguntaban
// "si el roster está vacío" estaban protegiéndose de algo que no puede pasar.
func buildAssigneeRoster(tasks []model.Task, offdays []model.OffDay) []assigneeSummary {
	byName := map[string]*assigneeSummary{}

	ensure := func(name string) *assigneeSummary {
		if s, ok := byName[name]; ok {
			return s
		}
		s := &assigneeSummary{Name: name}
		byName[name] = s
		return s
	}

	ensure("Me")
	for _, t := range tasks {
		if model.IsUnassigned(t.Assignee) {
			continue
		}
		s := ensure(t.Assignee)
		s.Total++
		if t.IsActive() {
			s.Active++
		}
	}
	for _, o := range offdays {
		if model.IsUnassigned(o.Assignee) {
			continue
		}
		ensure(o.Assignee).OffDayCount++
	}

	result := make([]assigneeSummary, 0, len(byName))
	for _, s := range byName {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// tasksForAssignee son las tareas no cerradas de una persona, en el orden de
// entrada.
func tasksForAssignee(tasks []model.Task, name string) []model.Task {
	var result []model.Task
	for _, t := range tasks {
		if t.Assignee == name && t.IsActive() {
			result = append(result, t)
		}
	}
	return result
}

// offDaysForAssignee son los off-days de una persona, en el orden de carga
// (fecha de inicio ascendente, que es como los devuelve la DB).
func offDaysForAssignee(offdays []model.OffDay, name string) []model.OffDay {
	var result []model.OffDay
	for _, o := range offdays {
		if o.Assignee == name {
			result = append(result, o)
		}
	}
	return result
}

// nameAt es el nombre de la persona en la posición idx del roster, o "" si el
// índice no es válido. La guarda va dentro para que el llamante no tenga que
// repetirla antes de cada roster[idx].
func nameAt(roster []assigneeSummary, idx int) string {
	if !inRange(idx, len(roster)) {
		return ""
	}
	return roster[idx].Name
}

// ---- Reglas del Dashboard ------------------------------------------------
//
// El Dashboard cuenta sobre m.tasks cinco veces distintas (por proyecto, por
// estado, por persona) y cada cuenta iba con su propio bucle y su propio filtro
// de proyecto dentro de la función de render. Aquí están como funciones puras:
// reciben las tareas y el proyecto seleccionado, y no saben nada del modelo.

// dashProjectTasks cuenta las tareas activas de un proyecto.
//
// project == "" cuenta el proyecto entero, que es lo que quiere el panel cuando
// no hay ninguno seleccionado en el board.
func dashProjectTasks(tasks []model.Task, project string) int {
	n := 0
	for _, t := range tasks {
		if project != "" && t.ProjectName != project {
			continue
		}
		if t.IsActive() {
			n++
		}
	}
	return n
}

// dashStatusCounts reparte las tareas por estado y devuelve los tres totales que
// muestra el Overview.
//
// done y cancelled se cuentan aparte; todo lo demás es "activa", incluidos los
// estados propios del workflow de cada proyecto (backlog, reviewing…). El mapa
// porStatus lleva la misma distribución y alimenta las barras.
func dashStatusCounts(tasks []model.Task, project string) (active, done, cancelled int, byStatus map[string]int) {
	byStatus = map[string]int{}
	for _, t := range tasks {
		if project != "" && t.ProjectName != project {
			continue
		}
		switch t.Status {
		case model.CancelledStatus:
			cancelled++
		case model.DoneStatus:
			done++
		default:
			active++
		}
		byStatus[t.Status]++
	}
	return active, done, cancelled, byStatus
}

// dashAssigneeCounts cuenta tareas por persona, con su subcuenta de activas.
func dashAssigneeCounts(tasks []model.Task, project string) (total, active map[string]int) {
	total = map[string]int{}
	active = map[string]int{}
	for _, t := range tasks {
		if project != "" && t.ProjectName != project {
			continue
		}
		total[t.Assignee]++
		if t.IsActive() {
			active[t.Assignee]++
		}
	}
	return total, active
}

// dashRowsAvailable son las filas que quedan para contenido en una columna del
// Dashboard: el alto de la columna menos lo ya usado y menos la cabecera fija.
//
// El suelo es 0, no 1: un presupuesto de 0 filas significa "no cabe nada más",
// que es distinto de "cabe al menos una línea" y es lo que evita que el bloque
// crezca sobre el alto calculado.
func dashRowsAvailable(colLines, used, headerRows int) int {
	return max(colLines-used-headerRows, 0)
}

// dashColumnWidths reparte el ancho interior en dos columnas iguales con un
// hueco de 1 columna entre medias.
func dashColumnWidths(innerW int) (left, right int) {
	half := innerW/2 - 1
	return half, half
}

// previewBudgetFor son las líneas de descripción que caben sin empujar el
// contenido ni los keybinds fuera de la pantalla.
//
// height - keybinds - minContentHeight, menos los dos bordes de la caja del
// preview, y acotado entre 1 y previewMaxLines. El mínimo de 1 garantiza que la
// descripción siempre tiene al menos una línea, aunque el terminal sea minúsculo.
func previewBudgetFor(height, keybindsHeight int) int {
	budget := height - keybindsHeight - minContentHeight - 2 // 2 = bordes de la caja
	return min(max(budget, 1), previewMaxLines)
}

// matchesStatus dice si el estado de una tarea pasa el filtro de estado.
//
// Hay tres modos: el de "todas las activas" (todo menos done y cancelled), el
// de "todas" (sin restricción) y el de un estado concreto (coincidencia
// exacta).
func matchesStatus(statusFilter, taskStatus string, active bool) bool {
	switch statusFilter {
	case statusFilterAllActive:
		return active
	case "":
		return true
	default:
		return taskStatus == statusFilter
	}
}

// matchesPriority aplica el filtro de prioridad. -1 significa "cualquiera".
func matchesPriority(filterPriority, taskPriority int) bool {
	return filterPriority < 0 || filterPriority == taskPriority
}

// nextPriority es el ciclo de la tecla de prioridad (ctrl+p).
//
// El ciclo depende del estado: en backlog incluye "none" (none→low→med→high→none,
// cuatro peldaños) y fuera de backlog no (low→med→high→low, tres). Por eso no es
// un `% 4` global.
//
// priority se recorta antes de ciclar porque llega de la base y allí no hay
// garantía de que esté en 0..3: un 7 fuera de rango haría que low→low con el
// ciclo de tres peldaños. Recortar hace que la función sea total.
func nextPriority(status string, priority int) int {
	p := min(max(priority, model.PriorityNone), model.PriorityHigh)
	if status == "backlog" {
		return (p + 1) % 4
	}
	return (p % 3) + 1
}

// joinSections une bloques verticalmente sin dejar filas vacías entre ellos.
func joinSections(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, s := range sections {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, "\n")
}

// visibleRange devuelve el rango [start, end) de una ventana de size elementos
// sobre un total, manteniendo el cursor dentro de la ventana.
func visibleRange(cursor, total, size int) (int, int) {
	// Sin lista o sin ventana no hay nada que mostrar.
	if total <= 0 || size <= 0 {
		return 0, 0
	}
	// Una ventana mayor que el total se recorta al total. El borde -- ventana igual
	// al total -- daba el mismo resultado por la cuenta de más abajo, así que
	// dejar el ">=" era otro mutante equivalente.
	size = min(size, total)

	// La ventana se centra en el cursor y se acota a los dos lados de golpe. Con
	// min en vez de dos if, el borde "start == total - size" deja de ser una
	// decisión: es el mismo número por las dos ramas.
	start := min(max(cursor-size/2, 0), total-size)
	return start, start + size
}

// truncateSuffix es lo que se pone en lugar de lo que se corta.
const truncateSuffix = ".."

// truncate corta un texto a limit caracteres agregando un sufijo.
//
// Por debajo del tamaño del sufijo no cabe la elipsis, así que se recorta a pelo:
// un slice con índice negativo revienta. Los llamantes de hoy pasan constantes
// positivas, así que el suelo es defensivo, pero una función que acepta un int y
// peta con el negativo es una trampa para el siguiente.
//
// El parámetro se llama limit y no max porque max es la función integrada, y aquí
// hace falta para acotar por abajo.
func truncate(s string, limit int) string {
	if limit < len(truncateSuffix) {
		return s[:min(len(s), max(limit, 0))]
	}
	if len(s) <= limit {
		return s
	}
	return s[:limit-len(truncateSuffix)] + truncateSuffix
}

// singleLine colapsa un texto multilínea a una sola línea para que no rompa
// una fila de tabla.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
