package db

import (
	"testing"

	"tsk/internal/model"
)

// TestTaskTagsRoundtrip verifica que las tags se normalicen al crear y que
// sobrevivan el round-trip por GetTask y ListTasks.
func TestTaskTagsRoundtrip(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.CreateProject("api", nil); err != nil {
		t.Fatal(err)
	}

	task, err := db.CreateTaskFull("api", "task", "", "@a", 0, "", 0, []string{" Blocked ", "bug", "BLOCKED"})
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Tags) != 2 || task.Tags[0] != "blocked" || task.Tags[1] != "bug" {
		t.Fatalf("tags creadas = %v, want [blocked bug]", task.Tags)
	}

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !model.HasTag(got.Tags, "blocked") || !model.HasTag(got.Tags, "bug") {
		t.Errorf("GetTask tags = %v", got.Tags)
	}

	tasks, err := db.ListTasks("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !model.HasTag(tasks[0].Tags, "blocked") {
		t.Errorf("ListTasks tags = %v", tasks)
	}
}

func TestAddRemoveSetTaskTags(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", nil)
	task, _ := db.CreateTask("api", "task", "", "@a", 0, "")

	if _, err := db.AddTaskTags(task.ID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	// Agregar una duplicada no la repite.
	got, err := db.AddTaskTags(task.ID, []string{"Blocked", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags tras add = %v, want 2", got.Tags)
	}

	got, err = db.RemoveTaskTags(task.ID, []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	if model.HasTag(got.Tags, "bug") || !model.HasTag(got.Tags, "blocked") {
		t.Errorf("tags tras remove = %v", got.Tags)
	}

	got, err = db.SetTaskTags(task.ID, []string{"urgent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "urgent" {
		t.Errorf("tags tras set = %v, want [urgent]", got.Tags)
	}
}

// TestMigrateAddDelivered verifica que la migración inserte delivered antes de
// reviewing (o done) en workflows existentes, y también en list_order.
func TestMigrateAddDelivered(t *testing.T) {
	db := newTestDB(t)

	old := []string{"backlog", "todo", "doing", "reviewing", "done", "cancelled"}
	oldOrder := []string{"reviewing", "doing", "todo", "backlog", "done", "cancelled"}
	if _, err := db.CreateProjectWithListOrder("api", old, oldOrder); err != nil {
		t.Fatal(err)
	}
	// Proyecto sin reviewing: delivered debe caer antes de done.
	noReview := []string{"todo", "doing", "done"}
	if _, err := db.CreateProjectWithListOrder("web", noReview, nil); err != nil {
		t.Fatal(err)
	}

	if err := migrateAddDelivered(db); err != nil {
		t.Fatal(err)
	}

	api, _ := db.GetProject("api")
	wf, lo := api.Workflow, api.ListOrder
	if !model.HasStatus(wf, "delivered") {
		t.Fatalf("api workflow sin delivered: %v", wf)
	}
	if idx(wf, "delivered") > idx(wf, "reviewing") {
		t.Errorf("delivered debe ir antes de reviewing: %v", wf)
	}
	if !model.HasStatus(lo, "delivered") {
		t.Errorf("list_order sin delivered: %v", lo)
	}

	web, _ := db.GetProject("web")
	if !model.HasStatus(web.Workflow, "delivered") {
		t.Fatalf("web workflow sin delivered: %v", web.Workflow)
	}
	if idx(web.Workflow, "delivered") > idx(web.Workflow, "done") {
		t.Errorf("sin reviewing, delivered debe ir antes de done: %v", web.Workflow)
	}

	// Correr de nuevo no debe duplicar.
	if err := migrateAddDelivered(db); err != nil {
		t.Fatal(err)
	}
	api, _ = db.GetProject("api")
	count := 0
	for _, s := range api.Workflow {
		if s == "delivered" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("delivered duplicado en %v", api.Workflow)
	}
}

func idx(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// TestMigrateDeliveredBeforeCustomReview verifica que con un estado de revisión
// con otro nombre ("review") delivered se inserte antes, y que la migración
// correctiva lo reordene si ya había quedado después.
func TestMigrateDeliveredBeforeCustomReview(t *testing.T) {
	db := newTestDB(t)

	custom := []string{"backlog", "todo", "in_progress", "review", "done"}
	if _, err := db.CreateProjectWithListOrder("app", custom, nil); err != nil {
		t.Fatal(err)
	}
	if err := migrateAddDelivered(db); err != nil {
		t.Fatal(err)
	}
	p, _ := db.GetProject("app")
	if idx(p.Workflow, "delivered") > idx(p.Workflow, "review") {
		t.Fatalf("delivered quedó después de review: %v", p.Workflow)
	}

	// Simular el estado que dejó la 009 con el bug: delivered después de review.
	buggy := []string{"backlog", "todo", "in_progress", "review", "delivered", "done"}
	if _, err := db.CreateProjectWithListOrder("legacy", buggy, nil); err != nil {
		t.Fatal(err)
	}
	if err := migrateRepositionDelivered(db); err != nil {
		t.Fatal(err)
	}
	legacy, _ := db.GetProject("legacy")
	if idx(legacy.Workflow, "delivered") > idx(legacy.Workflow, "review") {
		t.Errorf("reposition no movió delivered antes de review: %v", legacy.Workflow)
	}
}

// TestMigrateRenameReview verifica que la migración 011 renombre el estado
// "review" a "reviewing" preservando la posición, tanto en el workflow como en
// list_order, y que mueva las tareas de "review" a "reviewing".
func TestMigrateRenameReview(t *testing.T) {
	db := newTestDB(t)

	old := []string{"backlog", "todo", "in_progress", "delivered", "review", "done"}
	oldOrder := []string{"review", "delivered", "todo", "backlog", "done"}
	if _, err := db.CreateProjectWithListOrder("api", old, oldOrder); err != nil {
		t.Fatal(err)
	}
	task, err := db.CreateTask("api", "en revisión", "", "@a", 0, "review")
	if err != nil {
		t.Fatal(err)
	}
	// Proyecto moderno sin "review": no debe tocarse.
	modern, _ := db.CreateProject("web", nil)

	if err := migrateRenameReview(db); err != nil {
		t.Fatal(err)
	}

	api, _ := db.GetProject("api")
	if model.HasStatus(api.Workflow, "review") {
		t.Errorf("api workflow todavía tiene review: %v", api.Workflow)
	}
	if !model.HasStatus(api.Workflow, "reviewing") {
		t.Fatalf("api workflow sin reviewing: %v", api.Workflow)
	}
	if idx(api.Workflow, "delivered") > idx(api.Workflow, "reviewing") {
		t.Errorf("delivered debe seguir antes de reviewing: %v", api.Workflow)
	}
	if model.HasStatus(api.ListOrder, "review") || !model.HasStatus(api.ListOrder, "reviewing") {
		t.Errorf("api list_order mal renombrado: %v", api.ListOrder)
	}

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "reviewing" {
		t.Errorf("task status = %q, want reviewing", got.Status)
	}

	// Correr de nuevo no debe cambiar nada (idempotente).
	if err := migrateRenameReview(db); err != nil {
		t.Fatal(err)
	}
	api, _ = db.GetProject("api")
	count := 0
	for _, s := range api.Workflow {
		if s == "reviewing" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("reviewing duplicado en %v", api.Workflow)
	}

	web, _ := db.GetProject("web")
	if !sameStrings(web.Workflow, modern.Workflow) || !sameStrings(web.ListOrder, modern.ListOrder) {
		t.Errorf("web se modificó: workflow=%v list_order=%v", web.Workflow, web.ListOrder)
	}
}

// TestMigrateRenameReviewDedupes verifica que si un proyecto ya tenía
// "reviewing" además de "review", la migración no duplica el estado.
func TestMigrateRenameReviewDedupes(t *testing.T) {
	db := newTestDB(t)

	both := []string{"todo", "review", "reviewing", "done"}
	if _, err := db.CreateProjectWithListOrder("api", both, nil); err != nil {
		t.Fatal(err)
	}

	if err := migrateRenameReview(db); err != nil {
		t.Fatal(err)
	}

	api, _ := db.GetProject("api")
	if model.HasStatus(api.Workflow, "review") {
		t.Errorf("quedó review: %v", api.Workflow)
	}
	count := 0
	for _, s := range api.Workflow {
		if s == "reviewing" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("reviewing duplicado: %v", api.Workflow)
	}
}
