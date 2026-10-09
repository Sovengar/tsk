// Package cli implements the command line interface of tsk.
// All commands return JSON on stdout and errors on stderr.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/harness"
	"tsk/internal/model"
)

// Run is the CLI entry point. Returns true if it handled a
// subcommand; false if the TUI should be launched.
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
	case "ask":
		cmdAsk(args[1:])
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

// stdout, stderr and exit are injectable so tests can capture the
// output and check the error paths without killing the test process. In
// production they point to os and are never touched.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
	exit             = os.Exit
)

func outputJSON(v any) {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func outputError(msg string) {
	enc := json.NewEncoder(stderr)
	_ = enc.Encode(model.ErrorResult{Error: msg})
	exit(1)
}

// flagScanner walks a command's arguments consuming flags and their
// values. It relies on re-slicing (rest = rest[1:]) rather than an i++ in the
// post of a for: that hand-written advance is exactly the operation a
// mutator can invert, and once inverted the loop never terminates (the test
// hangs and the mutant is reported as TIMED OUT, not as killed).
type flagScanner struct {
	rest []string
}

func newFlagScanner(args []string, from int) flagScanner {
	// Out-of-range from is clamped instead of blowing up: it is a clamp, not a
	// validation, and that's why it is expressed with min/max and not with a branch the
	// mutator could make equivalent (args[from:] and the clamp match
	// when from == len(args), so a `from >= len(args)` changes nothing).
	return flagScanner{rest: args[min(max(from, 0), len(args)):]}
}

// next consumes and returns the next token. It is used both to read the flag
// and, in the corresponding case, to read its value.
func (s *flagScanner) next() (string, bool) {
	if len(s.rest) == 0 {
		return "", false
	}
	tok := s.rest[0]
	s.rest = s.rest[1:]
	return tok, true
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

// closeDB closes the database at the end of a command. The Close error is
// ignored on purpose: it is end-of-process cleanup and must not alter the output
// the command already emitted.
func closeDB(database *db.DB) {
	_ = database.Close()
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

	s := newFlagScanner(args, 1)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--workflow":
			if v, ok := s.next(); ok {
				workflow = v
			}
		case "--list-order":
			if v, ok := s.next(); ok {
				listOrder = v
			}
		}
	}

	database := openDB()
	defer closeDB(database)

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
	defer closeDB(database)

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
			_, _ = fmt.Fprintln(stdout, "No archived projects.")
		} else {
			_, _ = fmt.Fprintln(stdout, "No projects registered.")
		}
		return
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tWORKFLOW\tLIST ORDER\tTASKS")
	for _, p := range result {
		wf := strings.Join(p.Workflow, ",")
		lo := strings.Join(p.ListOrder, ",")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", p.Name, wf, lo, p.TaskCount)
	}
	_ = w.Flush()
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
	defer closeDB(database)

	p, err := database.GetProject(name)
	if err != nil {
		outputError(err.Error())
	}

	if jsonOutput {
		outputJSON(map[string]any{"project": p})
		return
	}

	// Human-readable
	_, _ = fmt.Fprintf(stdout, "Project:  %s\n", p.Name)
	_, _ = fmt.Fprintf(stdout, "Workflow: %s\n", strings.Join(p.Workflow, " → "))
	listOrder := strings.Join(p.ListOrder, " → ")
	if listOrder == "" {
		listOrder = "(workflow order)"
	}
	_, _ = fmt.Fprintf(stdout, "List:     %s\n", listOrder)
	_, _ = fmt.Fprintf(stdout, "Archived: %t\n", p.Archived)
	_, _ = fmt.Fprintf(stdout, "Created:  %s\n", p.CreatedAt)
	_, _ = fmt.Fprintf(stdout, "Updated:  %s\n", p.UpdatedAt)
}

func cmdProjectUpdate(args []string) {
	name := args[0]
	var newName, workflow, listOrder string
	listOrderSet := false

	s := newFlagScanner(args, 1)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--name":
			if v, ok := s.next(); ok {
				newName = v
			}
		case "--workflow":
			if v, ok := s.next(); ok {
				workflow = v
			}
		case "--list-order":
			if v, ok := s.next(); ok {
				listOrder = v
			}
			listOrderSet = true
		}
	}

	database := openDB()
	defer closeDB(database)

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
	defer closeDB(database)

	if err := database.DeleteProject(name); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{"ok": true})
}

// cmdProjectArchive archives a project (soft delete): it hides it, its tasks and
// its comments without deleting them.
func cmdProjectArchive(name string) {
	database := openDB()
	defer closeDB(database)

	if err := database.ArchiveProject(name); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{"ok": true})
}

// cmdProjectUnarchive restores an archived project.
func cmdProjectUnarchive(name string) {
	database := openDB()
	defer closeDB(database)

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

	s := newFlagScanner(args, 1)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--project":
			if v, ok := s.next(); ok {
				project = v
			}
		case "--priority":
			if v, ok := s.next(); ok {
				p, err := strconv.Atoi(v)
				if err == nil {
					priority = p
				}
			}
		case "--assignee":
			if v, ok := s.next(); ok {
				assignee = v
			}
		case "--status":
			if v, ok := s.next(); ok {
				status = v
			}
		case "--estimate":
			if v, ok := s.next(); ok {
				e, err := strconv.ParseFloat(v, 64)
				if err == nil && e >= 0 {
					estimate = e
				}
			}
		case "--tag", "--tags":
			if v, ok := s.next(); ok {
				tags = append(tags, model.ParseTags(v)...)
			}
		}
	}

	if project == "" {
		outputError("--project is required")
	}

	database := openDB()
	defer closeDB(database)

	t, err := database.CreateTaskFull(project, title, "", assignee, priority, status, estimate, tags)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskResponse{OK: true, Task: *t})
}

func cmdList(args []string) {
	var project, status, assignee, tag string
	jsonOutput := false

	s := newFlagScanner(args, 0)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--project":
			if v, ok := s.next(); ok {
				project = v
			}
		case "--status":
			if v, ok := s.next(); ok {
				status = v
			}
		case "--assignee":
			if v, ok := s.next(); ok {
				assignee = v
			}
		case "--tag":
			if v, ok := s.next(); ok {
				tag = v
			}
		case "--json":
			jsonOutput = true
		}
	}

	database := openDB()
	defer closeDB(database)

	tasks, err := database.ListTasks(project, status, assignee)
	if err != nil {
		outputError(err.Error())
	}

	// The tag filter is applied in memory: the data set is small and
	// avoids coupling the SQL to json_each for a single tag.
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
		_, _ = fmt.Fprintln(stdout, "No tasks found.")
		return
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tPRIORITY\tSTATUS\tASSIGNEE\tTAGS\tTITLE")
	for _, t := range tasks {
		_, _ = fmt.Fprintf(w, "%d\t%s %s\t%s\t%s\t%s\t%s\n",
			t.ID,
			model.PriorityBar(t.Priority),
			model.PriorityLabel(t.Priority),
			t.Status,
			t.Assignee,
			strings.Join(t.Tags, ","),
			t.Title,
		)
	}
	_ = w.Flush()
	_, _ = fmt.Fprintf(stdout, "\nTotal: %d tasks\n", len(tasks))
}

func cmdShow(idStr string) {
	database := openDB()
	defer closeDB(database)

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

// cmdAsk hands a task off to an AI harness: tsk ask <task-id> [--harness NAME].
// It never waits for the harness; it prints a JSON result as soon as the
// process is started.
func cmdAsk(args []string) {
	if len(args) == 0 {
		outputError("usage: tsk ask <task-id> [--harness NAME]")
	}

	idStr := args[0]
	var harnessName string
	s := newFlagScanner(args, 1)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--harness":
			if v, ok := s.next(); ok {
				harnessName = v
			}
		}
	}

	cfg := config.Load()
	if err := harness.ValidateCommand(cfg.Handoff.Command); err != nil {
		outputError(err.Error())
	}

	database := openDB()
	defer closeDB(database)

	id := parseID(idStr)
	task, err := database.GetTask(id)
	if err != nil {
		outputError(err.Error())
	}

	selected, err := selectHarness(harness.Detect(cfg), harnessName)
	if err != nil {
		outputError(err.Error())
	}

	if err := harness.Execute(cfg, *task, selected); err != nil {
		outputError(err.Error())
	}

	outputJSON(map[string]any{
		"ok":       true,
		"task_id":  id,
		"harness":  selected,
		"launched": true,
	})
}

// selectHarness resolves the harness to use: the requested one, the only
// available one, or an error listing the options (never guesses between
// several, so an agent always knows which one ran).
func selectHarness(available []harness.Harness, want string) (string, error) {
	if want != "" {
		for _, h := range available {
			if h.Name == want {
				return h.Name, nil
			}
		}
		return "", fmt.Errorf("unknown harness: %s (available: %s)", want, harnessNames(available))
	}
	if len(available) == 1 {
		return available[0].Name, nil
	}
	if len(available) == 0 {
		//nolint:staticcheck // the message is pinned by behavior.feature
		return "", errors.New(harness.NoHarnessesMessage)
	}
	return "", fmt.Errorf("several harnesses available, pass --harness: %s", harnessNames(available))
}

// harnessNames joins the display names for the error messages.
func harnessNames(list []harness.Harness) string {
	names := make([]string, 0, len(list))
	for _, h := range list {
		names = append(names, h.Name)
	}
	return strings.Join(names, ", ")
}

// cmdComment manages the comment subcommands: add, list, remove.
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
		defer closeDB(database)

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
		defer closeDB(database)

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
		defer closeDB(database)

		if err := database.DeleteComment(id); err != nil {
			outputError(err.Error())
		}
		outputJSON(map[string]any{"ok": true, "id": id})
	default:
		outputError("usage: tsk comment (add|list|remove) ...")
	}
}

// ---- Off-day commands ----

// cmdOffDay manages the non-working-day subcommands: add, list, remove.
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
		ns := newFlagScanner(args, 3)
		for {
			flag, ok := ns.next()
			if !ok {
				break
			}
			if flag == "--note" {
				if v, ok := ns.next(); ok {
					note = v
				}
			}
		}

		database := openDB()
		defer closeDB(database)

		o, err := database.AddOffDay(assignee, start, end, note)
		if err != nil {
			outputError(err.Error())
		}
		outputJSON(map[string]any{"ok": true, "offday": o})
	case "list":
		var assignee string
		jsonOutput := false
		s := newFlagScanner(args, 1)
		for {
			flag, ok := s.next()
			if !ok {
				break
			}
			switch flag {
			case "--assignee":
				if v, ok := s.next(); ok {
					assignee = v
				}
			case "--json":
				jsonOutput = true
			}
		}

		database := openDB()
		defer closeDB(database)

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
			_, _ = fmt.Fprintln(stdout, "No off-days registered.")
			return
		}
		w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tASSIGNEE\tFROM\tTO\tNOTE")
		for _, o := range offdays {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", o.ID, o.Assignee, o.StartDate, o.EndDate, o.Note)
		}
		_ = w.Flush()
	case "remove":
		if len(args) < 2 {
			outputError("usage: tsk offday remove <id>")
		}
		id := parseID(args[1])

		database := openDB()
		defer closeDB(database)

		if err := database.DeleteOffDay(id); err != nil {
			outputError(err.Error())
		}
		outputJSON(map[string]any{"ok": true, "id": id})
	default:
		outputError("usage: tsk offday (add|list|remove) ...")
	}
}

// ---- Gantt ----

// cmdGantt projects each person's queue and prints it as a Gantt (text) or
// as a JSON structure with start/end dates per task.
func cmdGantt(args []string) {
	var project, assignee, fromStr string
	weeks := 0
	jsonOutput := false

	s := newFlagScanner(args, 0)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--project":
			if v, ok := s.next(); ok {
				project = v
			}
		case "--assignee":
			if v, ok := s.next(); ok {
				assignee = v
			}
		case "--from":
			if v, ok := s.next(); ok {
				fromStr = v
			}
		case "--weeks":
			// Here zero and negatives are not discarded: we accept what
			// Atoi understands and the `if weeks <= 0` below normalizes it, which is
			// where it falls back to the config. Filtering in both places forced
			// the same rule to be written twice.
			if v, ok := s.next(); ok {
				if n, err := strconv.Atoi(v); err == nil {
					weeks = n
				}
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
	defer closeDB(database)

	// The queue is ALWAYS computed with all tasks and off-days of each
	// person: their capacity is a single one and is shared across projects. --project
	// and --assignee are view filters (they do not change the dates).
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
	_, _ = fmt.Fprint(stdout, renderGanttText(sched, weeks))
}

// renderGanttText draws the Gantt in text: one row per task, one column
// per day, grouped by person. Non-working days stay inside the
// bar (the bar covers the task's real calendar range).
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

	// Week rule: label "1SEP" (week of month + month) aligned to each
	// Monday.
	ruler := make([]rune, labelW+1+totalDays)
	for i := range ruler {
		ruler[i] = ' '
	}
	for d := range totalDays {
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
	for d := range totalDays {
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
			// Effective range inside the window: it is clamped instead of
			// walking with d++ by hand, because an inverted `d++` leaves the loop
			// hanging and the mutant is reported as TIMED OUT instead of killed.
			lo, hi := max(d0, 0), min(d1, totalDays-1)
			for i := range max(hi-lo+1, 0) {
				row[lo+i] = '█'
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

// truncateLabel cuts a text to w runes, with ".." if there is leftover.
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

// padRight pads with spaces up to w runes (not bytes, so UTF-8
// titles do not misalign the columns). If the text is already wider it is returned as is,
// without branches: the `if n >= w` was an equivalent mutant (with n == w
// Repeat(0) returns the same string).
func padRight(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-len([]rune(s))))
}

func cmdUpdate(args []string) {
	id := parseID(args[0])
	updates := map[string]any{}
	var tagsSet, tagsAdd, tagsRemove []string
	hasTagsSet := false

	s := newFlagScanner(args, 1)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--title":
			if v, ok := s.next(); ok {
				updates["title"] = v
			}
		case "--description":
			if v, ok := s.next(); ok {
				updates["description"] = v
			}
		case "--priority":
			if v, ok := s.next(); ok {
				p, err := strconv.Atoi(v)
				if err == nil {
					updates["priority"] = p
				}
			}
		case "--assignee":
			if v, ok := s.next(); ok {
				updates["assignee"] = v
			}
		case "--estimate":
			if v, ok := s.next(); ok {
				e, err := strconv.ParseFloat(v, 64)
				if err == nil && e >= 0 {
					updates["estimate"] = e
				}
			}
		case "--tags":
			if v, ok := s.next(); ok {
				tagsSet = model.ParseTags(v)
				hasTagsSet = true
			}
		case "--tag":
			if v, ok := s.next(); ok {
				tagsAdd = append(tagsAdd, model.ParseTags(v)...)
			}
		case "--untag":
			if v, ok := s.next(); ok {
				tagsRemove = append(tagsRemove, model.ParseTags(v)...)
			}
		}
	}

	database := openDB()
	defer closeDB(database)

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
	defer closeDB(database)

	id := parseID(idStr)
	t, err := database.MoveTask(id, status)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdStart(idStr string) {
	database := openDB()
	defer closeDB(database)

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
	defer closeDB(database)

	id := parseID(idStr)
	t, err := database.ReviewTask(id)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdDone(idStr string) {
	database := openDB()
	defer closeDB(database)

	id := parseID(idStr)
	t, err := database.DoneTask(id)
	if err != nil {
		outputError(err.Error())
	}

	outputJSON(model.TaskActionResult{OK: true, Task: *t})
}

func cmdCancel(idStr string) {
	database := openDB()
	defer closeDB(database)

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
	s := newFlagScanner(args, 0)
	for {
		flag, ok := s.next()
		if !ok {
			break
		}
		switch flag {
		case "--project":
			if v, ok := s.next(); ok {
				project = v
			}
		case "--json":
			jsonOutput = true
		}
	}

	database := openDB()
	defer closeDB(database)

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

	_, _ = fmt.Fprintf(stdout, "Total: %d tasks\n", total)
	_, _ = fmt.Fprintln(stdout)

	_, _ = fmt.Fprintln(stdout, "By Status:")
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for status, count := range byStatus {
		bar := strings.Repeat("█", count)
		_, _ = fmt.Fprintf(w, "  %s\t%d\t%s\n", status, count, bar)
	}
	_ = w.Flush()

	_, _ = fmt.Fprintln(stdout)
	_, _ = fmt.Fprintln(stdout, "By Assignee:")
	w2 := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for assignee, count := range byAssignee {
		_, _ = fmt.Fprintf(w2, "  %s\t%d\n", assignee, count)
	}
	_ = w2.Flush()
}

func cmdMigrate() {
	database := openDB()
	defer closeDB(database)
	outputJSON(map[string]any{"ok": true, "message": "migrations applied"})
}

func cmdCompletion(args []string) {
	shell := "bash"
	if len(args) > 0 {
		shell = args[0]
	}

	switch shell {
	case "bash":
		_, _ = fmt.Fprint(stdout, bashCompletion)
	case "zsh":
		_, _ = fmt.Fprint(stdout, zshCompletion)
	case "fish":
		_, _ = fmt.Fprint(stdout, fishCompletion)
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
    commands="project add list show update move start review done cancel comment offday gantt stats ask migrate completion help"

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "--json --project --priority --assignee --status --estimate --from --weeks --note --workflow --list-order --force --archived --name --harness" -- ${cur}) )
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
        '1:command:(project add list show update move start review done cancel comment offday gantt stats ask migrate completion help)' \
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
complete -c tsk -n '__fish_use_subcommand' -a ask -d 'Hand off a task to an AI harness'
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
complete -c tsk -l harness -d 'AI harness to hand a task to'
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
		"tsk stats [--project X]":            "show statistics",
		"tsk ask <task-id> [--harness NAME]": "hand a task off to an AI harness",
		"tsk migrate":                        "run pending migrations",
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

	_, _ = fmt.Fprintln(stdout, "tsk — Task Manager TUI + CLI")
	_, _ = fmt.Fprintln(stdout, "")
	_, _ = fmt.Fprintln(stdout, "Projects:")
	for _, cmd := range projectCmds {
		_, _ = fmt.Fprintf(stdout, "  %-55s %s\n", cmd, commands[cmd])
	}
	_, _ = fmt.Fprintln(stdout, "")
	_, _ = fmt.Fprintln(stdout, "Tasks:")
	for _, cmd := range taskCmds {
		_, _ = fmt.Fprintf(stdout, "  %-55s %s\n", cmd, commands[cmd])
	}
}
