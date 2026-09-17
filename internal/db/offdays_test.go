package db

import "testing"

func TestCreateTaskWithEstimate(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", nil)

	task, err := db.CreateTaskWithEstimate("api", "Fix auth", "", "@juan", 2, "", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if task.Estimate != 0.5 {
		t.Errorf("estimate = %v, want 0.5", task.Estimate)
	}

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Estimate != 0.5 {
		t.Errorf("persisted estimate = %v, want 0.5", got.Estimate)
	}
}

func TestCreateTaskDefaultsToZeroEstimate(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", nil)

	task, _ := db.CreateTask("api", "task", "", "@a", 0, "")
	if task.Estimate != 0 {
		t.Errorf("estimate = %v, want 0 by default", task.Estimate)
	}
}

func TestListTasksIncludesEstimate(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", nil)
	db.CreateTaskWithEstimate("api", "task", "", "@a", 0, "", 1.25)

	tasks, err := db.ListTasks("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Estimate != 1.25 {
		t.Errorf("tasks = %+v, want estimate 1.25", tasks)
	}
}

func TestUpdateTaskEstimate(t *testing.T) {
	db := newTestDB(t)
	db.CreateProject("api", nil)
	task, _ := db.CreateTask("api", "task", "", "@a", 0, "")

	updated, err := db.UpdateTask(task.ID, map[string]any{"estimate": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Estimate != 2.0 {
		t.Errorf("estimate = %v, want 2", updated.Estimate)
	}
}

func TestAddAndListOffDays(t *testing.T) {
	db := newTestDB(t)

	o, err := db.AddOffDay("@alice", "2026-07-28", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if o.StartDate != "2026-07-28" || o.EndDate != "2026-07-28" {
		t.Errorf("single-day offday = %s→%s, want same date", o.StartDate, o.EndDate)
	}

	if _, err := db.AddOffDay("@bob", "2026-08-01", "2026-08-14", "vacaciones"); err != nil {
		t.Fatal(err)
	}

	all, err := db.ListOffDays("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("offdays = %d, want 2", len(all))
	}
	// Ordenados por fecha de inicio.
	if all[0].Assignee != "@alice" || all[1].Assignee != "@bob" {
		t.Errorf("order = %s, %s", all[0].Assignee, all[1].Assignee)
	}

	only, _ := db.ListOffDays("@bob")
	if len(only) != 1 || only[0].Note != "vacaciones" {
		t.Errorf("filtered offdays = %+v", only)
	}
}

func TestAddOffDayValidations(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.AddOffDay("", "2026-07-28", "", ""); err == nil {
		t.Error("empty assignee should error")
	}
	if _, err := db.AddOffDay("unassigned", "2026-07-28", "", ""); err == nil {
		t.Error("unassigned should error")
	}
	if _, err := db.AddOffDay("@a", "not-a-date", "", ""); err == nil {
		t.Error("invalid date should error")
	}
	if _, err := db.AddOffDay("@a", "2026-07-28", "2026-07-01", ""); err != nil {
		t.Errorf("reversed range should be normalized, got %v", err)
	}
}

func TestDeleteOffDay(t *testing.T) {
	db := newTestDB(t)
	o, _ := db.AddOffDay("@a", "2026-07-28", "", "")

	if err := db.DeleteOffDay(o.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.ListOffDays(""); len(got) != 0 {
		t.Errorf("offdays after delete = %d, want 0", len(got))
	}
	if err := db.DeleteOffDay(o.ID); err == nil {
		t.Error("deleting twice should error")
	}
}
