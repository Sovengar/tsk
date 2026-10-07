package tui

import (
	"testing"

	"tsk/internal/model"
)

// Third round on the "I write and then cannot read" paths. The difference
// with the sabotage of the second batch is which query has to fail: here
// what breaks is a READ, and the writes that come right before --
// UpdateTask, GetTask -- have to keep working.
//
// The technique: a row with an id that is not a number. Since the table is
// recreated with id TEXT -- without PRIMARY KEY, which is what forces SQLite
// to impose an integer -- a row with id "unreadable" can be inserted. No
// query by key finds it, so GetTask(id) is still fine; but ListTasks, which
// does not filter by id, walks it and blows up converting it. That is the hole.
//
// Before the sabotage the pool is limited to one connection: the test
// database is :memory:, and with more than one connection each sees a
// different database. It is a quirk of the test DSN, not of the program, but
// it has to be neutralized or the sabotage applies to one connection and the query goes to another.

func singleConnection(t *testing.T, m *Model) {
	t.Helper()
	m.database.Conn().SetMaxOpenConns(1)
}

func sabotageOnlyTheTaskList(t *testing.T, m *Model) {
	t.Helper()
	singleConnection(t, m)
	conn := m.database.Conn()

	if _, err := conn.Exec(`ALTER TABLE tasks RENAME TO tasks_healthy`); err != nil {
		t.Fatalf("ALTER TABLE tasks: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE tasks (
		id TEXT, project_id INTEGER NOT NULL, title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'backlog',
		priority INTEGER NOT NULL DEFAULT 0, assignee TEXT NOT NULL DEFAULT '',
		estimate REAL NOT NULL DEFAULT 0, tags TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')), completed_at TEXT)`); err != nil {
		t.Fatalf("CREATE TABLE tasks: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO tasks
		SELECT id, project_id, title, description, status, priority, assignee,
		       estimate, tags, created_at, updated_at, completed_at FROM tasks_healthy`); err != nil {
		t.Fatalf("copy the tasks: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO tasks (id, project_id, title) VALUES ('unreadable', ?, 'poison')`,
		projectIDOf(t, m),
	); err != nil {
		t.Fatalf("INSERT INTO tasks: %v", err)
	}
}

func TestSaveDescriptionWhenOnlyTheListFails(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	sabotageOnlyTheTaskList(t, m)

	cmd := m.saveDescriptionCmd(id, "new description")
	if cmd == nil {
		t.Fatal("saveDescriptionCmd returned nil")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("with the listing unreadable got %T, want nil", msg)
	}

	// And the save did happen: the write comes before the read, and that is
	// exactly what makes this hole interesting.
	reloaded, err := m.database.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", id, err)
	}
	if reloaded.Description != "new description" {
		t.Errorf("the description is %q, want the new one", reloaded.Description)
	}
}

func TestUpdateTaskFromEditWhenOnlyTheListFails(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	before := m.tasks[0].Title
	sabotageOnlyTheTaskList(t, m)

	cmd := m.updateTaskFromEdit(id, "Title: renamed\nStatus: todo\n")
	if cmd == nil {
		t.Fatal("updateTaskFromEdit returned nil")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("with the listing unreadable got %T, want nil", msg)
	}

	reloaded, err := m.database.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", id, err)
	}
	if reloaded.Title == before {
		t.Errorf("the title is still %q: the external editor did not save", before)
	}
}

// Adding a tag writes to tasks.tags. A trigger that aborts that UPDATE
// leaves the reads intact, which is exactly what the command needs: it reads
// the task, writes the tag, and only if the write fails it is left without a message.
func TestToggleTagWhenTheTagWriteIsRefused(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID

	if _, err := m.database.Conn().Exec(
		`CREATE TRIGGER no_tags BEFORE UPDATE OF tags ON tasks
		 BEGIN SELECT RAISE(ABORT, 'tags no editables'); END`,
	); err != nil {
		t.Fatalf("CREATE TRIGGER: %v", err)
	}

	if msg := m.toggleTagCmd(id, "new")(); msg != nil {
		t.Errorf("with the tag write refused got %T, want nil", msg)
	}

	// And with a tag that is already there, it removes it through the same path.
	mustCreateTaskWithTags(t, m.database, "api", "with tags", "", "Me", 1, "todo", []string{"already"})
	reloadTasks(t, m)

	var withTags int64
	for _, tsk := range m.tasks {
		if tsk.Title == "with tags" {
			withTags = tsk.ID
		}
	}
	if withTags == 0 {
		t.Fatal("the fixture left no task with tags")
	}
	if msg := m.toggleTagCmd(withTags, "already")(); msg != nil {
		t.Errorf("removing a tag with the write refused got %T, want nil", msg)
	}
}

// The project loading is two queries on the same table: the active ones
// (archived = 0) and the archived ones (archived = 1). Only the second can
// fail without dragging the first one along, and for that the table is
// recreated with id TEXT and an archived row with an id that is not a number
// is inserted: the active query does not see it (archived = 1) and the archived one blows up converting it.
func TestLoadProjectsWhenOnlyTheArchiveFails(t *testing.T) {
	m := newTestModel(t)
	singleConnection(t, m)
	conn := m.database.Conn()

	if _, err := conn.Exec(`PRAGMA foreign_keys(OFF)`); err != nil {
		t.Fatalf("PRAGMA foreign_keys(OFF): %v", err)
	}
	if _, err := conn.Exec(`DROP TABLE projects`); err != nil {
		t.Fatalf("DROP TABLE projects: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE projects (
		id TEXT, name TEXT NOT NULL UNIQUE, workflow TEXT NOT NULL DEFAULT '[]',
		list_order TEXT NOT NULL DEFAULT '[]', archived INTEGER NOT NULL DEFAULT 0,
		archived_at TEXT, created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		t.Fatalf("CREATE TABLE projects: %v", err)
	}
	// An active project, which the active query does have to return, and one
	// archived with the poisoned id.
	if _, err := conn.Exec(
		`INSERT INTO projects (id, name, workflow) VALUES ('1', 'api', '["todo"]')`,
	); err != nil {
		t.Fatalf("INSERT active project: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO projects (id, name, workflow, archived) VALUES ('unreadable', 'old', '["todo"]', 1)`,
	); err != nil {
		t.Fatalf("INSERT archived project: %v", err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys(ON)`); err != nil {
		t.Fatalf("PRAGMA foreign_keys(ON): %v", err)
	}

	msg := mustMsg(t, m.loadProjects())
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("the message is %T, want projectsLoadedMsg", msg)
	}
	if len(loaded.projects) != 1 || loaded.projects[0].Name != "api" {
		t.Errorf("the active projects are %+v, want only api", loaded.projects)
	}
	if loaded.archivedProjects != nil {
		t.Errorf("with the archived query broken there are %d archived, want nil",
			len(loaded.archivedProjects))
	}

	// And the view is not left lame: the archived project is still there, it
	// is the QUERY that does not know how to read it.
	if _, err := conn.Exec(`SELECT 1`); err != nil {
		t.Errorf("the database became unusable: %v", err)
	}
}

// The two loading commands carry a "if the query fails, return empty". What
// was missing was the other side: that a load that DOES work brings the data.
// Without that half, a mutant that inverts the condition -- returns empty
// exactly when everything is fine -- survives without anyone noticing.
func TestLoadCommandsReturnTheirData(t *testing.T) {
	t.Run("tasks", func(t *testing.T) {
		m := newTestModel(t)
		if len(m.tasks) == 0 {
			t.Fatal("the fixture left no tasks")
		}

		msg := mustMsg(t, m.loadTasks())
		loaded, ok := msg.(tasksLoadedMsg)
		if !ok {
			t.Fatalf("the message is %T, want tasksLoadedMsg", msg)
		}
		if len(loaded.tasks) != len(m.tasks) {
			t.Errorf("the load brought %d tasks of %d", len(loaded.tasks), len(m.tasks))
		}
	})

	t.Run("projects", func(t *testing.T) {
		m := newDashModel(t, "api")
		mustCreateProject(t, m.database, "archived", model.DefaultWorkflow)
		if err := m.database.ArchiveProject("archived"); err != nil {
			t.Fatalf("ArchiveProject: %v", err)
		}

		msg := mustMsg(t, m.loadProjects())
		loaded, ok := msg.(projectsLoadedMsg)
		if !ok {
			t.Fatalf("the message is %T, want projectsLoadedMsg", msg)
		}
		if len(loaded.projects) == 0 {
			t.Error("the load did not bring the active projects")
		}
		// The archived has to come in its own list: that is what allows
		// un-archiving without going back to the database.
		if len(loaded.archivedProjects) != 1 || loaded.archivedProjects[0].Name != "archived" {
			t.Errorf("the archived projects are %+v, want only the archived one",
				loaded.archivedProjects)
		}
	})

	t.Run("comments", func(t *testing.T) {
		m := newDetailModel(t, 3)

		msg := mustMsg(t, m.loadCommentsCmd(m.detailTask.ID))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("the message is %T, want commentsLoadedMsg", msg)
		}
		if len(loaded.comments) != 3 {
			t.Errorf("the load brought %d comments, want 3", len(loaded.comments))
		}
		// Loading does not select: the freshly created comment does, with the
		// index at the place it belongs to. If the two said the same, the
		// selection would jump to the first one on reload.
		if loaded.selectIdx != -1 {
			t.Errorf("loading leaves loaded.selectIdx = %d, want -1 (nothing selected)", loaded.selectIdx)
		}

		// And the creation does select the new one, which is the last of the list.
		msg = mustMsg(t, m.addCommentCmd(m.detailTask.ID, "another"))
		added := msg.(commentsLoadedMsg)
		if len(added.comments) != 4 {
			t.Errorf("after adding there are %d comments, want 4", len(added.comments))
		}
		if added.selectIdx != len(added.comments)-1 {
			t.Errorf("after adding the selection is %d, want the last one (%d)",
				added.selectIdx, len(added.comments)-1)
		}
	})
}
