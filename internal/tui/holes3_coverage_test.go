package tui

import (
	"testing"

	"tsk/internal/model"
)

// Tercer turno en los caminos "escribo y luego no puedo leer". La diferencia con
// el sabotaje de la segunda tanda es qué consulta tiene que fallar: aquí la que
// se rompe es una LECTURA, y las escrituras que van justo antes -- UpdateTask,
// GetTask -- tienen que seguir funcionando.
//
// La técnica: una fila con un id que no es un número. Como la tabla se recrea
// con id TEXT -- sin PRIMARY KEY, que es lo que obliga a SQLite a imponer un
// entero -- se puede meter una fila con id "ilegible". Ninguna consulta por
// clave la encuentra, así que GetTask(id) sigue bien; pero ListTasks, que no
// filtra por id, la recorre y revienta al convertirla. Eso es el hueco.
//
// Antes del sabotaje se limita el pool a una conexión: la base de los tests es
// :memory:, y con más de una conexión cada una ve una base distinta. Es una
// rareza del DSN de test, no del programa, pero hay queNeutralizarla o el
// sabotaje se aplica a una conexión y la consulta va a otra.

func unaSolaConexion(t *testing.T, m *Model) {
	t.Helper()
	m.database.Conn().SetMaxOpenConns(1)
}

func sabotearSoloLaListaDeTareas(t *testing.T, m *Model) {
	t.Helper()
	unaSolaConexion(t, m)
	conn := m.database.Conn()

	if _, err := conn.Exec(`ALTER TABLE tasks RENAME TO tasks_sana`); err != nil {
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
		       estimate, tags, created_at, updated_at, completed_at FROM tasks_sana`); err != nil {
		t.Fatalf("copiar las tareas: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO tasks (id, project_id, title) VALUES ('ilegible', ?, 'veneno')`,
		projectIDDe(t, m),
	); err != nil {
		t.Fatalf("INSERT INTO tasks: %v", err)
	}
}

func TestSaveDescriptionWhenOnlyTheListFails(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	sabotearSoloLaListaDeTareas(t, m)

	cmd := m.saveDescriptionCmd(id, "nueva descripción")
	if cmd == nil {
		t.Fatal("saveDescriptionCmd ha devuelto nil")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("con la lista ilegible ha salido %T, want nil", msg)
	}

	// Y el guardado sí ocurrió: la escritura va antes que la lectura, y eso es
	// justo lo que hace que este hueco sea interesante.
	revisada, err := m.database.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", id, err)
	}
	if revisada.Description != "nueva descripción" {
		t.Errorf("la descripción es %q, want la nueva", revisada.Description)
	}
}

func TestUpdateTaskFromEditWhenOnlyTheListFails(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	antes := m.tasks[0].Title
	sabotearSoloLaListaDeTareas(t, m)

	cmd := m.updateTaskFromEdit(id, "Title: renombrada\nStatus: todo\n")
	if cmd == nil {
		t.Fatal("updateTaskFromEdit ha devuelto nil")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("con la lista ilegible ha salido %T, want nil", msg)
	}

	revisada, err := m.database.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d): %v", id, err)
	}
	if revisada.Title == antes {
		t.Errorf("el título sigue en %q: el editor externo no guardó", antes)
	}
}

// Poner una tag escribe en tasks.tags. Un trigger que aborta ese UPDATE deja
// las lecturas intactas, que es justo lo que necesita el comando: lee la tarea,
// escribe la tag, y sólo si la escritura falla se queda sin mensaje.
func TestToggleTagWhenTheTagWriteIsRefused(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID

	if _, err := m.database.Conn().Exec(
		`CREATE TRIGGER no_tags BEFORE UPDATE OF tags ON tasks
		 BEGIN SELECT RAISE(ABORT, 'tags no editables'); END`,
	); err != nil {
		t.Fatalf("CREATE TRIGGER: %v", err)
	}

	if msg := m.toggleTagCmd(id, "nueva")(); msg != nil {
		t.Errorf("con la escritura de tags rechazada ha salido %T, want nil", msg)
	}

	// Y con una tag que ya está, la quita por el mismo camino.
	mustCreateTaskWithTags(t, m.database, "api", "con tags", "", "Me", 1, "todo", []string{"ya"})
	reloadTasks(t, m)

	var conTags int64
	for _, tsk := range m.tasks {
		if tsk.Title == "con tags" {
			conTags = tsk.ID
		}
	}
	if conTags == 0 {
		t.Fatal("el fixture no ha dejado la tarea con tags")
	}
	if msg := m.toggleTagCmd(conTags, "ya")(); msg != nil {
		t.Errorf("quitar una tag con la escritura rechazada ha salido %T, want nil", msg)
	}
}

// La carga de proyectos son dos consultas con la misma tabla: la de activos
// (archived = 0) y la de archivados (archived = 1). Sólo la segunda puede fallar
// sin arrastrar a la primera, y para eso la tabla se recrea con id TEXT y se le
// mete una fila archivada con un id que no es un número: la consulta de activos no
// la ve (archived = 1) y la de archivados revienta al convertirla.
func TestLoadProjectsWhenOnlyTheArchiveFails(t *testing.T) {
	m := newTestModel(t)
	unaSolaConexion(t, m)
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
	// Un proyecto activo, que la consulta de activos sí tiene que devolver, y uno
	// archivado con el id envenenado.
	if _, err := conn.Exec(
		`INSERT INTO projects (id, name, workflow) VALUES ('1', 'api', '["todo"]')`,
	); err != nil {
		t.Fatalf("INSERT proyecto activo: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO projects (id, name, workflow, archived) VALUES ('ilegible', 'viejo', '["todo"]', 1)`,
	); err != nil {
		t.Fatalf("INSERT proyecto archivado: %v", err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys(ON)`); err != nil {
		t.Fatalf("PRAGMA foreign_keys(ON): %v", err)
	}

	msg := mustMsg(t, m.loadProjects())
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("el mensaje es %T, want projectsLoadedMsg", msg)
	}
	if len(loaded.projects) != 1 || loaded.projects[0].Name != "api" {
		t.Errorf("los proyectos activos son %+v, want sólo api", loaded.projects)
	}
	if loaded.archivedProjects != nil {
		t.Errorf("con la consulta de archivados rota hay %d archivados, want nil",
			len(loaded.archivedProjects))
	}

	// Y la vista no se queda cojea: el proyecto archivado sigue ahí, es la
	// CONSULTA la que no sabe leerlo.
	if _, err := conn.Exec(`SELECT 1`); err != nil {
		t.Errorf("la base ha quedado inservible: %v", err)
	}
}

// Los dos comandos de carga llevan un "si la consulta falla, devuelve vacío".
// Lo que faltaba era el otro lado: que una carga que SÍ funciona traiga los
// datos. Sin esa mitad, un mutante que invierte la condición -- devuelve vacío
// justo cuando todo va bien -- sobrevive sin que nadie lo note.
func TestLoadCommandsReturnTheirData(t *testing.T) {
	t.Run("tareas", func(t *testing.T) {
		m := newTestModel(t)
		if len(m.tasks) == 0 {
			t.Fatal("el fixture no ha dejado tareas")
		}

		msg := mustMsg(t, m.loadTasks())
		loaded, ok := msg.(tasksLoadedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want tasksLoadedMsg", msg)
		}
		if len(loaded.tasks) != len(m.tasks) {
			t.Errorf("la carga ha traído %d tareas de %d", len(loaded.tasks), len(m.tasks))
		}
	})

	t.Run("proyectos", func(t *testing.T) {
		m := newDashModel(t, "api")
		mustCreateProject(t, m.database, "archivado", model.DefaultWorkflow)
		if err := m.database.ArchiveProject("archivado"); err != nil {
			t.Fatalf("ArchiveProject: %v", err)
		}

		msg := mustMsg(t, m.loadProjects())
		loaded, ok := msg.(projectsLoadedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want projectsLoadedMsg", msg)
		}
		if len(loaded.projects) == 0 {
			t.Error("la carga no ha traído los proyectos activos")
		}
		// Elarchived tiene que venir en su propia lista: es lo que permite
		// desarchivar sin volver a la base.
		if len(loaded.archivedProjects) != 1 || loaded.archivedProjects[0].Name != "archivado" {
			t.Errorf("los proyectos archivados son %+v, want sólo el archivado",
				loaded.archivedProjects)
		}
	})

	t.Run("comentarios", func(t *testing.T) {
		m := newDetailModel(t, 3)

		msg := mustMsg(t, m.loadCommentsCmd(m.detailTask.ID))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentsLoadedMsg", msg)
		}
		if len(loaded.comments) != 3 {
			t.Errorf("la carga ha traído %d comentarios, want 3", len(loaded.comments))
		}
		// Cargar no selecciona: el comentario recién creado sí, con el índice en
		// el sitio que le toca. Si los dos dijeran lo mismo, la selección
		// saltaría al primero al recargar.
		if loaded.selectIdx != -1 {
			t.Errorf("cargar deja loaded.selectIdx = %d, want -1 (nada seleccionado)", loaded.selectIdx)
		}

		// Y el alta sí selecciona el nuevo, que es el último de la lista.
		msg = mustMsg(t, m.addCommentCmd(m.detailTask.ID, "otro"))
		anadido := msg.(commentsLoadedMsg)
		if len(anadido.comments) != 4 {
			t.Errorf("tras añadir hay %d comentarios, want 4", len(anadido.comments))
		}
		if anadido.selectIdx != len(anadido.comments)-1 {
			t.Errorf("tras añadir la selección es %d, want el último (%d)",
				anadido.selectIdx, len(anadido.comments)-1)
		}
	})
}
