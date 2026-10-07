package db

import (
	"testing"

	"tsk/internal/model"
)

// TestTaskTagsRoundtrip verifies that tags are normalized on create and that
// they survive the round-trip through GetTask and ListTasks.
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
		t.Fatalf("created tags = %v, want [blocked bug]", task.Tags)
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
	mustCreateProject(t, db, "api", nil)
	task, _ := db.CreateTask("api", "task", "", "@a", 0, "")

	if _, err := db.AddTaskTags(task.ID, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	// Adding a duplicate does not repeat it.
	got, err := db.AddTaskTags(task.ID, []string{"Blocked", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags after add = %v, want 2", got.Tags)
	}

	got, err = db.RemoveTaskTags(task.ID, []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	if model.HasTag(got.Tags, "bug") || !model.HasTag(got.Tags, "blocked") {
		t.Errorf("tags after remove = %v", got.Tags)
	}

	got, err = db.SetTaskTags(task.ID, []string{"urgent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "urgent" {
		t.Errorf("tags after set = %v, want [urgent]", got.Tags)
	}
}
