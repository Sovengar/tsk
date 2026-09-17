// Package cli implementa la interfaz de línea de comandos de tsk.
// Todos los comandos devuelven JSON en stdout y errores en stderr.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/model"
)

// Run es el punto de entrada del CLI. Devuelve true si manejó un
// subcomando; false si debe lanzar la TUI.
func Run(args []string) bool {
	if len(args) == 0 {
		return false
	}

	cmd := args[0]

	switch cmd {
	case "project":
		cmdProject(args[1:])
	case "add":
		cmdAdd(args[1:])
	case "list":
		cmdList(args[1:])
	case "show":
		if len(args) < 2 {
			outputError("usage: tsk show <id>")
		}
		cmdShow(args[1])
	case "comment":
		cmdComment(args[1:])
	case "offday":
		cmdOffDay(args[1:])
	case "gantt":
		cmdGantt(args[1:])
	case "update":
		if len(args) < 2 {
			outputError("usage: tsk update <id> [--title ...] [--description ...] [--priority N] [--assignee @name] [--tag T] [--untag T]")
		}
		cmdUpdate(args[1:])
	case "move":
		if len(args) < 3 {
			outputError("usage: tsk move <id> <status>")
		}
		cmdMove(args[1], args[2])
	case "start":
		if len(args) < 2 {
			outputError("usage: tsk start <id>")
		}
		cmdStart(args[1])
	case "review":
		if len(args) < 2 {
			outputError("usage: tsk review <id>")
		}
		cmdReview(args[1])
	case "done":
		if len(args) < 2 {
			outputError("usage: tsk done <id>")
		}
		cmdDone(args[1])
	case "cancel":
		if len(args) < 2 {
			outputError("usage: tsk cancel <id>")
		}
		cmdCancel(args[1])
	case "stats":
		cmdStats(args[1:])
	case "migrate":
		cmdMigrate()
	case "completion":
		cmdCompletion(args[1:])
	case "help", "--help", "-h":
		cmdHelp()
	default:
		return false
	}
	return true
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func outputError(msg string) {
	enc := json.NewEncoder(os.Stderr)
	_ = enc.Encode(model.ErrorResult{Error: msg})
	os.Exit(1)
}

func parseID(s string) int64 {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		outputError(fmt.Sprintf("invalid id: %s", s))
	}
	return id
}

func openDB() *db.DB {
	cfg := config.Load()
	dbPath := cfg.Database.Path
	if dbPath == "" {
		var err error
		dbPath, err = db.DefaultPath()
		if err != nil {
			outputError(err.Error())
		}
	}
	database, err := db.Open(dbPath)
	if err != nil {
		outputError(err.Error())
	}
	return database
}

// ---- Project commands ----

func cmdProject(args []string) {
	if len(args) == 0 {
		outputError("usage: tsk project (add|list|show|update|remove) ...")
	}
	switch args[0] {
	case "add":
		cmdProjectAdd(args[1:])
	case "list":
		cmdProjectList(args[1:])
	case "show":
		if len(args) < 2 {
			outputError("usage: tsk project show <name> [--json]")
		}
		cmdProjectShow(args[1:])
	case "update":
		if len(args) < 2 {
			outputError("usage: tsk project update <name> [--name ...] [--workflow ...] [--list-order ...]")
		}
		cmdProjectUpdate(args[1:])
	case "remove":
		if len(args) < 2 {
			outputError("usage: tsk project remove <name>")
		}
		cmdProjectRemove(args[1])
	case "archive":
		if len(args) < 2 {
			outputError("usage: tsk project archive <name>")
		}
		cmdProjectArchive(args[1])
	case "unarchive":
		if len(args) < 2 {
			outputError("usage: tsk project unarchive <name>")
		}
		cmdProjectUnarchive(args[1])
	default:
		outputError(fmt.Sprintf("unknown project subcommand: %s", args[0]))
	}
}

func cmdProjectAdd(args []string) {
	if len(args) == 0 {
		outputError("usage: tsk project add <name> [--workflow ...] [--list-order ...]")
	}
	name := args[0]
	var workflow, listOrder string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--workflow":
			if i+1 < len(args) {
				workflow = args[i+1]
				i++
			}
		case "--list-order":
			if i+1 < len(args) {
				listOrder = args[i+1]
				i++
			}
		}
	}

	database := openDB()
	defer database.Close()

	var wf, lo []string
	if workflow != "" {
		var err error
		wf, err = model.ParseWorkflow(workflow)
		if err != nil {
			outputError(err.Error())
		}
	}
	if listOrder != "" {
		var err error
		lo, err = model.ParseWorkflow(listOrder)
		if err != nil {
			outputError(err.Error())
		}
	}

	p, err := database.CreateProjectWithListOrder(name, wf, lo)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{
		"ok":      true,
		"project": p,
	})
}

func cmdProjectList(args []string) {
	jsonOutput := false
	showArchived := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--archived":
			showArchived = true
		}
	}

	database := openDB()
	defer database.Close()

	var projects []model.Project
	var err error
	if showArchived {
		projects, err = database.ListArchivedProjects()
	} else {
		projects, err = database.ListProjects()
	}
	if err != nil {
		outputError(err.Error())
	}

	type ProjectInfo struct {
		Name      string   `json:"name"`
		Workflow  []string `json:"workflow"`
		ListOrder []string `json:"list_order"`
		Archived  bool     `json:"archived"`
		TaskCount int      `json:"task_count"`
	}

	result := make([]ProjectInfo, 0, len(projects))
	for _, p := range projects {
		count, _ := database.ProjectTaskCount(p.ID)
		result = append(result, ProjectInfo{
			Name:      p.Name,
			Workflow:  p.Workflow,
			ListOrder: p.ListOrder,
			Archived:  p.Archived,
			TaskCount: count,
		})
	}

	if jsonOutput {
		outputJSON(map[string]any{"projects": result})
		return
	}

	// Human-readable output
	if len(result) == 0 {
		if showArchived {
			fmt.Println("No archived projects.")
		} else {
			fmt.Println("No projects registered.")
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tWORKFLOW\tLIST ORDER\tTASKS")
	for _, p := range result {
		wf := strings.Join(p.Workflow, ",")
		lo := strings.Join(p.ListOrder, ",")
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", p.Name, wf, lo, p.TaskCount)
	}
	w.Flush()
}

func cmdProjectShow(args []string) {
	name := args[0]
	jsonOutput := false
	for _, a := range args[1:] {
		if a == "--json" {
			jsonOutput = true
		}
	}

	database := openDB()
	defer database.Close()

	p, err := database.GetProject(name)
	if err != nil {
		outputError(err.Error())
	}

	if jsonOutput {
		outputJSON(map[string]any{"project": p})
		return
	}

	// Human-readable
	fmt.Printf("Project:  %s\n", p.Name)
	fmt.Printf("Workflow: %s\n", strings.Join(p.Workflow, " → "))
	listOrder := strings.Join(p.ListOrder, " → ")
	if listOrder == "" {
		listOrder = "(workflow order)"
	}
	fmt.Printf("List:     %s\n", listOrder)
	fmt.Printf("Archived: %t\n", p.Archived)
	fmt.Printf("Created:  %s\n", p.CreatedAt)
	fmt.Printf("Updated:  %s\n", p.UpdatedAt)
}

func cmdProjectUpdate(args []string) {
	name := args[0]
	var newName, workflow, listOrder string
	listOrderSet := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				newName = args[i+1]
				i++
			}
		case "--workflow":
			if i+1 < len(args) {
				workflow = args[i+1]
				i++
			}
		case "--list-order":
			if i+1 < len(args) {
				listOrder = args[i+1]
				i++
			}
			listOrderSet = true
		}
	}

	database := openDB()
	defer database.Close()

	updates := map[string]any{}
	if newName != "" {
		updates["name"] = newName
	}
	if workflow != "" {
		wf, err := model.ParseWorkflow(workflow)
		if err != nil {
			outputError(err.Error())
		}
		updates["workflow"] = wf
	}
	if listOrderSet {
		lo, err := model.ParseWorkflow(listOrder)
		if err != nil {
			outputError(err.Error())
		}
		updates["list_order"] = lo
	}

	if err := database.UpdateProject(name, updates); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{"ok": true})
}

func cmdProjectRemove(name string) {
	database := openDB()
	defer database.Close()

	if err := database.DeleteProject(name); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{"ok": true})
}

// cmdProjectArchive archiva un proyecto (soft delete): oculta él, sus tareas y
// sus comentarios sin borrarlos.
func cmdProjectArchive(name string) {
	database := openDB()
	defer database.Close()

	if err := database.ArchiveProject(name); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{"ok": true})
}

// cmdProjectUnarchive restaura un proyecto archivado.
func cmdProjectUnarchive(name string) {
	database := openDB()
	defer database.Close()

	if err := database.UnarchiveProject(name); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{"ok": true})
}

// ---- Task commands ----

func cmdAdd(args []string) {
	if len(args) == 0 {
		outputError("usage: tsk add <title> --project X [--priority N] [--assignee @name] [--status S] [--tag T]")
	}

	title := args[0]
	var project, assignee, status string
	var tags []string
	priority := 0
	estimate := 0.0

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				p, err := strconv.Atoi(args[i+1])
				if err == nil {
					priority = p
				}
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				assignee = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				status = args[i+1]
				i++
			}
		case "--estimate":
			if i+1 < len(args) {
				e, err := strconv.ParseFloat(args[i+1], 64)
				if err == nil && e >= 0 {
					estimate = e
				}
				i++
			}
		case "--tag", "--tags":
			if i+1 < len(args) {
				tags = append(tags, model.ParseTags(args[i+1])...)
				i++
			}
		}
	}

	if project == "" {
		outputError("--project is required")
	}

	database := openDB()
	defer database.Close()

	t, err := database.CreateTaskFull(project, title, "", assignee, priority, status, estimate, tags)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskResponse{OK: true, Task: *t})
}

func cmdList(args []string) {
	var project, status, assignee, tag string
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				status = args[i+1]
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				assignee = args[i+1]
				i++
			}
		case "--tag":
			if i+1 < len(args) {
				tag = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	database := openDB()
	defer database.Close()

	tasks, err := database.ListTasks(project, status, assignee)
	if err != nil {
		outputError(err.Error())
	}

	// El filtro por tag se aplica en memoria: el conjunto de datos es chico y
	// evita acoplar el SQL a json_each para una sola etiqueta.
	if tag != "" {
		var byTag []model.Task
		for _, t := range tasks {
			if model.HasTag(t.Tags, tag) {
				byTag = append(byTag, t)
			}
		}
		tasks = byTag
	}

	if tasks == nil {
		tasks = []model.Task{}
	}

	if jsonOutput {
		outputJSON(model.TaskListResponse{Tasks: tasks})
		return
	}

	// Human-readable table output
	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPRIORITY\tSTATUS\tASSIGNEE\tTAGS\tTITLE")
	for _, t := range tasks {
		fmt.Fprintf(w, "%d\t%s %s\t%s\t%s\t%s\t%s\n",
			t.ID,
			model.PriorityBar(t.Priority),
			model.PriorityLabel(t.Priority),
			t.Status,
			t.Assignee,
			strings.Join(t.Tags, ","),
			t.Title,
		)
	}
	w.Flush()
	fmt.Printf("\nTotal: %d tasks\n", len(tasks))
}

func cmdShow(idStr string) {
	database := openDB()
	defer database.Close()

	id := parseID(idStr)
	t, err := database.GetTask(id)
	if err != nil {
		outputError(err.Error())
	}

	comments, err := database.ListComments(id)
	if err != nil {
		outputError(err.Error())
	}
	if comments == nil {
		comments = []model.Comment{}
	}

	outputJSON(map[string]any{"task": t, "comments": comments})
}

// cmdComment gestiona los subcomandos de comentarios: add, list, remove.
func cmdComment(args []string) {
	if len(args) == 0 {
		outputError("usage: tsk comment (add|list|remove) ...")
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			outputError(`usage: tsk comment add <task-id> "<text>"`)
		}
		id := parseID(args[1])
		body := strings.Join(args[2:], " ")

		database := openDB()
		defer database.Close()

		c, err := database.AddComment(id, body)
		if err != nil {
			outputError(err.Error())
		}
		outputJSON(model.CommentResponse{OK: true, Comment: *c})
	case "list":
		if len(args) < 2 {
			outputError("usage: tsk comment list <task-id>")
		}
		id := parseID(args[1])

		database := openDB()
		defer database.Close()

		comments, err := database.ListComments(id)
		if err != nil {
			outputError(err.Error())
		}
		if comments == nil {
			comments = []model.Comment{}
		}
		outputJSON(model.CommentListResponse{Comments: comments})
	case "remove":
		if len(args) < 2 {
			outputError("usage: tsk comment remove <comment-id>")
		}
		id := parseID(args[1])

		database := openDB()
		defer database.Close()

		if err := database.DeleteComment(id); err != nil {
			outputError(err.Error())
		}
		outputJSON(map[string]any{"ok": true, "id": id})
	default:
		outputError("usage: tsk comment (add|list|remove) ...")
	}
}

// ---- Off-day commands ----

// cmdOffDay gestiona los subcomandos de días no laborables: add, list, remove.
func cmdOffDay(args []string) {
	if len(args) == 0 {
		outputError("usage: tsk offday (add|list|remove) ...")
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			outputError(`usage: tsk offday add <assignee> <start> [end] [--note "..."]`)
		}
		assignee, start := args[1], args[2]
		end, note := "", ""
		if len(args) > 3 && !strings.HasPrefix(args[3], "--") {
			end = args[3]
		}
		for i := 3; i < len(args); i++ {
			if args[i] == "--note" && i+1 < len(args) {
				note = args[i+1]
				i++
			}
		}

		database := openDB()
		defer database.Close()

		o, err := database.AddOffDay(assignee, start, end, note)
		if err != nil {
			outputError(err.Error())
		}
		outputJSON(map[string]any{"ok": true, "offday": o})
	case "list":
		var assignee string
		jsonOutput := false
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--assignee":
				if i+1 < len(args) {
					assignee = args[i+1]
					i++
				}
			case "--json":
				jsonOutput = true
			}
		}

		database := openDB()
		defer database.Close()

		offdays, err := database.ListOffDays(assignee)
		if err != nil {
			outputError(err.Error())
		}
		if offdays == nil {
			offdays = []model.OffDay{}
		}

		if jsonOutput {
			outputJSON(map[string]any{"offdays": offdays})
			return
		}
		if len(offdays) == 0 {
			fmt.Println("No off-days registered.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tASSIGNEE\tFROM\tTO\tNOTE")
		for _, o := range offdays {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", o.ID, o.Assignee, o.StartDate, o.EndDate, o.Note)
		}
		w.Flush()
	case "remove":
		if len(args) < 2 {
			outputError("usage: tsk offday remove <id>")
		}
		id := parseID(args[1])

		database := openDB()
		defer database.Close()

		if err := database.DeleteOffDay(id); err != nil {
			outputError(err.Error())
		}
		outputJSON(map[string]any{"ok": true, "id": id})
	default:
		outputError("usage: tsk offday (add|list|remove) ...")
	}
}

// ---- Gantt ----

// cmdGantt proyecta la cola de cada persona y la imprime como Gantt (texto) o
// como estructura JSON con fechas de inicio/fin por tarea.
func cmdGantt(args []string) {
	var project, assignee, fromStr string
	weeks := 0
	jsonOutput := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				assignee = args[i+1]
				i++
			}
		case "--from":
			if i+1 < len(args) {
				fromStr = args[i+1]
				i++
			}
		case "--weeks":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					weeks = n
				}
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	cfg := config.Load()
	if weeks <= 0 {
		weeks = cfg.GanttWeeks
	}

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if fromStr != "" {
		d, err := model.ParseDate(fromStr)
		if err != nil {
			outputError(fmt.Sprintf("invalid --from %q (want YYYY-MM-DD)", fromStr))
		}
		start = d
	}

	database := openDB()
	defer database.Close()

	// La cola se calcula SIEMPRE con todas las tareas y off-days de cada
	// persona: su capacidad es una sola y se reparte entre proyectos. --project
	// y --assignee son filtros de vista (no cambian las fechas).
	tasks, err := database.ListTasks("", "", "")
	if err != nil {
		outputError(err.Error())
	}
	offdays, err := database.ListOffDays("")
	if err != nil {
		outputError(err.Error())
	}

	sched := model.BuildSchedule(tasks, offdays, start, cfg.DefaultEstimateDays)
	sched = model.FilterSchedule(sched, func(t model.Task) bool {
		if project != "" && t.ProjectName != project {
			return false
		}
		if assignee != "" && t.Assignee != assignee {
			return false
		}
		return true
	})

	if jsonOutput {
		outputJSON(sched)
		return
	}
	fmt.Print(renderGanttText(sched, weeks))
}

// renderGanttText dibuja el Gantt en texto: una fila por tarea, una columna
// por día, agrupadas por persona. Los días no laborables quedan dentro de la
// barra (la barra cubre el rango calendario real de la tarea).
func renderGanttText(s *model.Schedule, weeks int) string {
	start, err := model.ParseDate(s.Start)
	if err != nil {
		return ""
	}
	totalDays := weeks * 7
	const labelW = 30

	var b strings.Builder
	fmt.Fprintf(&b, "Gantt · start %s · %d weeks\n\n", s.Start, weeks)

	dayIndex := func(dateStr string) int {
		d, err := model.ParseDate(dateStr)
		if err != nil {
			return -1
		}
		return int(d.Sub(start).Hours() / 24)
	}

	// Regla de semanas: etiqueta "1SEP" (semana del mes + mes) alineada a cada
	// lunes.
	ruler := make([]rune, labelW+1+totalDays)
	for i := range ruler {
		ruler[i] = ' '
	}
	for d := 0; d < totalDays; d++ {
		day := start.AddDate(0, 0, d)
		if day.Weekday() != time.Monday {
			continue
		}
		label := model.WeekOfMonthLabel(day)
		col := labelW + 1 + d
		for i, r := range label {
			if col+i < len(ruler) {
				ruler[col+i] = r
			}
		}
	}
	b.WriteString(strings.TrimRight(string(ruler), " "))
	b.WriteString("\n")

	axis := make([]rune, labelW+1)
	for i := range axis {
		axis[i] = ' '
	}
	b.WriteString(string(axis))
	for d := 0; d < totalDays; d++ {
		if start.AddDate(0, 0, d).Weekday() == time.Monday {
			b.WriteString("|")
		} else {
			b.WriteString("-")
		}
	}
	b.WriteString("\n")

	for _, a := range s.Assignees {
		fmt.Fprintf(&b, "%s  ends %s\n", a.Assignee, a.End)
		for _, e := range a.Entries {
			row := make([]rune, totalDays)
			for i := range row {
				row[i] = ' '
			}
			d0, d1 := dayIndex(e.Start), dayIndex(e.End)
			for d := d0; d <= d1 && d < totalDays; d++ {
				if d >= 0 {
					row[d] = '█'
				}
			}
			mark := ""
			if e.EstimateDefaulted {
				mark = " ~"
			}
			fmt.Fprintf(&b, "%s %s  %s→%s [%s]%s\n",
				padRight(truncateLabel(fmt.Sprintf("  #%d %s", e.Task.ID, e.Task.Title), labelW), labelW),
				string(row), e.Start, e.End, model.FormatEstimate(e.Estimate), mark)
		}
		b.WriteString("\n")
	}

	if len(s.Unassigned) > 0 {
		fmt.Fprintf(&b, "Unassigned (%d):\n", len(s.Unassigned))
		for _, t := range s.Unassigned {
			fmt.Fprintf(&b, "  #%d %s\n", t.ID, t.Title)
		}
	}
	return b.String()
}

// truncateLabel corta un texto a w runas, con ".." si sobra.
func truncateLabel(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 2 {
		return string(r[:w])
	}
	return string(r[:w-2]) + ".."
}

// padRight rellena con espacios hasta w runas (no bytes, así los títulos UTF-8
// no desalinean las columnas).
func padRight(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func cmdUpdate(args []string) {
	id := parseID(args[0])
	updates := map[string]any{}
	var tagsSet, tagsAdd, tagsRemove []string
	hasTagsSet := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 < len(args) {
				updates["title"] = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				updates["description"] = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				p, err := strconv.Atoi(args[i+1])
				if err == nil {
					updates["priority"] = p
				}
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				updates["assignee"] = args[i+1]
				i++
			}
		case "--estimate":
			if i+1 < len(args) {
				e, err := strconv.ParseFloat(args[i+1], 64)
				if err == nil && e >= 0 {
					updates["estimate"] = e
				}
				i++
			}
		case "--tags":
			if i+1 < len(args) {
				tagsSet = model.ParseTags(args[i+1])
				hasTagsSet = true
				i++
			}
		case "--tag":
			if i+1 < len(args) {
				tagsAdd = append(tagsAdd, model.ParseTags(args[i+1])...)
				i++
			}
		case "--untag":
			if i+1 < len(args) {
				tagsRemove = append(tagsRemove, model.ParseTags(args[i+1])...)
				i++
			}
		}
	}

	database := openDB()
	defer database.Close()

	if len(updates) > 0 {
		if _, err := database.UpdateTask(id, updates); err != nil {
			outputError(err.Error())
		}
	}
	if hasTagsSet {
		if _, err := database.SetTaskTags(id, tagsSet); err != nil {
			outputError(err.Error())
		}
	}
	if len(tagsAdd) > 0 {
		if _, err := database.AddTaskTags(id, tagsAdd); err != nil {
			outputError(err.Error())
		}
	}
	if len(tagsRemove) > 0 {
		if _, err := database.RemoveTaskTags(id, tagsRemove); err != nil {
			outputError(err.Error())
		}
	}

	t, err := database.GetTask(id)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskResponse{OK: true, Task: *t})
}

func cmdMove(idStr, status string) {
	database := openDB()
	defer database.Close()

	id := parseID(idStr)
	t, err := database.MoveTask(id, status)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdStart(idStr string) {
	database := openDB()
	defer database.Close()

	id := parseID(idStr)
	t, err := database.StartTask(id)
	if err != nil {
		outputError(err.Error())
	}

	result := model.TaskActionResult{OK: true, Task: *t}

	p, _ := database.GetProjectByID(t.ProjectID)
	if p != nil {
		next, ok := model.NextStatus(p.Workflow, t.Status)
		if ok {
			result.Next = next
		}
	}

	outputJSON(result)
}

func cmdReview(idStr string) {
	database := openDB()
	defer database.Close()

	id := parseID(idStr)
	t, err := database.ReviewTask(id)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdDone(idStr string) {
	database := openDB()
	defer database.Close()

	id := parseID(idStr)
	t, err := database.DoneTask(id)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdCancel(idStr string) {
	database := openDB()
	defer database.Close()

	id := parseID(idStr)
	t, err := database.CancelTask(id)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdStats(args []string) {
	var project string
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	database := openDB()
	defer database.Close()

	stats, err := database.Stats(project)
	if err != nil {
		outputError(err.Error())
	}

	if jsonOutput {
		outputJSON(map[string]any{"stats": stats})
		return
	}

	// Human-readable
	total := stats["total"].(int)
	byStatus := stats["by_status"].(map[string]int)
	byAssignee := stats["by_assignee"].(map[string]int)

	fmt.Printf("Total: %d tasks\n", total)
	fmt.Println()

	fmt.Println("By Status:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for status, count := range byStatus {
		bar := strings.Repeat("█", count)
		fmt.Fprintf(w, "  %s\t%d\t%s\n", status, count, bar)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("By Assignee:")
	w2 := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for assignee, count := range byAssignee {
		fmt.Fprintf(w2, "  %s\t%d\n", assignee, count)
	}
	w2.Flush()
}

func cmdMigrate() {
	database := openDB()
	defer database.Close()
	outputJSON(map[string]any{"ok": true, "message": "migrations applied"})
}

func cmdCompletion(args []string) {
	shell := "bash"
	if len(args) > 0 {
		shell = args[0]
	}

	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		outputError("usage: tsk completion (bash|zsh|fish)")
	}
}

const bashCompletion = `#!/bin/bash
_tsk_completions() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="project add list show update move start review done cancel comment offday gantt stats migrate completion help"

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "--json --project --priority --assignee --status --estimate --from --weeks --note --workflow --list-order --force --archived --name" -- ${cur}) )
        return 0
    fi

    case ${prev} in
        project)
            COMPREPLY=( $(compgen -W "add list show update remove archive unarchive" -- ${cur}) )
            return 0
            ;;
        comment)
            COMPREPLY=( $(compgen -W "add list remove" -- ${cur}) )
            return 0
            ;;
        offday)
            COMPREPLY=( $(compgen -W "add list remove" -- ${cur}) )
            return 0
            ;;
        add|list|show|update|move|start|review|done|cancel|gantt|stats)
            return 0
            ;;
    esac

    COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
    return 0
}
complete -F _tsk_completions tsk
`

const zshCompletion = `#compdef tsk

_tsk() {
    _arguments \
        '1:command:(project add list show update move start review done cancel comment offday gantt stats migrate completion help)' \
        '*::arg:->args'
}

_tsk "$@"
`

const fishCompletion = `complete -c tsk -f
complete -c tsk -n '__fish_use_subcommand' -a project -d 'Manage projects'
complete -c tsk -n '__fish_seen_subcommand_from project' -a add -d 'Register a project'
complete -c tsk -n '__fish_seen_subcommand_from project' -a list -d 'List projects'
complete -c tsk -n '__fish_seen_subcommand_from project' -a show -d 'Show project detail'
complete -c tsk -n '__fish_seen_subcommand_from project' -a update -d 'Update project'
complete -c tsk -n '__fish_seen_subcommand_from project' -a remove -d 'Delete project + tasks'
complete -c tsk -n '__fish_seen_subcommand_from project' -a archive -d 'Archive project'
complete -c tsk -n '__fish_seen_subcommand_from project' -a unarchive -d 'Restore archived project'
complete -c tsk -n '__fish_use_subcommand' -a add -d 'Create a task'
complete -c tsk -n '__fish_use_subcommand' -a list -d 'List tasks'
complete -c tsk -n '__fish_use_subcommand' -a show -d 'Show task detail'
complete -c tsk -n '__fish_use_subcommand' -a update -d 'Update task metadata'
complete -c tsk -n '__fish_use_subcommand' -a move -d 'Move task to status'
complete -c tsk -n '__fish_use_subcommand' -a start -d 'Start a task'
complete -c tsk -n '__fish_use_subcommand' -a review -d 'Move to review'
complete -c tsk -n '__fish_use_subcommand' -a done -d 'Complete a task'
complete -c tsk -n '__fish_use_subcommand' -a cancel -d 'Cancel a task'
complete -c tsk -n '__fish_use_subcommand' -a comment -d 'Manage task comments'
complete -c tsk -n '__fish_use_subcommand' -a offday -d 'Manage off-days'
complete -c tsk -n '__fish_seen_subcommand_from offday' -a add -d 'Add off-days'
complete -c tsk -n '__fish_seen_subcommand_from offday' -a list -d 'List off-days'
complete -c tsk -n '__fish_seen_subcommand_from offday' -a remove -d 'Delete off-day'
complete -c tsk -n '__fish_use_subcommand' -a gantt -d 'Project the schedule'
complete -c tsk -n '__fish_use_subcommand' -a stats -d 'Show statistics'
complete -c tsk -n '__fish_use_subcommand' -a migrate -d 'Run migrations'
complete -c tsk -n '__fish_use_subcommand' -a completion -d 'Generate shell completions'
complete -c tsk -n '__fish_use_subcommand' -a help -d 'Show help'
complete -c tsk -l json -d 'Output as JSON'
complete -c tsk -l project -d 'Filter by project'
complete -c tsk -l priority -d 'Set priority (0-3)'
complete -c tsk -l assignee -d 'Set assignee'
complete -c tsk -l status -d 'Filter by status'
complete -c tsk -l estimate -d 'Estimate in days (0.25 steps)'
complete -c tsk -l from -d 'Gantt start date (YYYY-MM-DD)'
complete -c tsk -l weeks -d 'Gantt horizon in weeks'
complete -c tsk -l note -d 'Off-day note'
complete -c tsk -l archived -d 'List archived projects'
`

func cmdHelp() {
	commands := map[string]string{
		"tsk": "launch the TUI (default when no arguments)",
		"tsk project add <name> [--workflow] [--list-order]": "register a project",
		"tsk project list":        "list all projects",
		"tsk project show <name>": "show project detail + workflow",
		"tsk project update <name> [--name] [--workflow] [--list-order]": "update project (rename/workflow/list order)",
		"tsk project remove <name>":                                      "delete project + tasks",
		"tsk project archive <name>":                                     "archive project (hides tasks, reversible)",
		"tsk project unarchive <name>":                                   "restore an archived project",
		"tsk project list [--archived]":                                  "list active or archived projects",
		"tsk add <title> --project X [--priority N] [--assignee @name] [--estimate N] [--tag T]": "create a task",
		"tsk list [--project X] [--status S] [--assignee A] [--tag T]":                           "list tasks",
		"tsk show <id>": "show task detail",
		"tsk update <id> [--title] [--description] [--priority N] [--assignee @name] [--estimate N] [--tag T] [--untag T] [--tags a,b]": "update task metadata (--tag adds, --untag removes, --tags replaces)",
		"tsk move <id> <status>":                               "move task to a specific status",
		"tsk start <id>":                                       "move task to 2nd workflow status",
		"tsk review <id>":                                      "move task to review status",
		"tsk done <id>":                                        "move task to terminal status",
		"tsk cancel <id>":                                      "cancel a task",
		"tsk comment add <task-id> \"<text>\"":                 "add a comment to a task",
		"tsk comment list <task-id>":                           "list comments of a task",
		"tsk comment remove <comment-id>":                      "delete a comment",
		"tsk offday add <assignee> <start> [end] [--note ...]": "mark non-working days for a person",
		"tsk offday list [--assignee A] [--json]":              "list off-days",
		"tsk offday remove <id>":                               "delete an off-day",
		"tsk gantt [--project X] [--assignee A] [--from DATE] [--weeks N] [--json]": "project the schedule per person (--project/--assignee filter the view, not the dates)",
		"tsk stats [--project X]": "show statistics",
		"tsk migrate":             "run pending migrations",
	}

	// Group by category
	projectCmds := []string{}
	taskCmds := []string{}
	for cmd := range commands {
		if strings.HasPrefix(cmd, "tsk project") {
			projectCmds = append(projectCmds, cmd)
		} else {
			taskCmds = append(taskCmds, cmd)
		}
	}

	fmt.Println("tsk — Task Manager TUI + CLI")
	fmt.Println("")
	fmt.Println("Projects:")
	for _, cmd := range projectCmds {
		fmt.Printf("  %-55s %s\n", cmd, commands[cmd])
	}
	fmt.Println("")
	fmt.Println("Tasks:")
	for _, cmd := range taskCmds {
		fmt.Printf("  %-55s %s\n", cmd, commands[cmd])
	}
}
