package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// readonlyDB abre una base de datos en sólo lectura.
//
// Es la segunda palanca, y cubre lo que la conexión cerrada no: aquí las
// lecturas funcionan y las escrituras fallan. Los métodos que primero leen la
// tarea o el proyecto y después escriben -- MoveTask, SetTaskTags,
// UpdateProject -- sólo alcanzan su comprobación de error de la escritura con
// esta palanca, porque con la base cerrada se quedan en la lectura.
func readonlyDB(t *testing.T, populate func(*testing.T, *DB)) *DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ro.db")
	escritura, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	populate(t, escritura)
	if err := escritura.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	conn, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("abriendo en sólo lectura: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ro := &DB{conn: conn}

	// Comprobación de que la palanca es la que creemos: lee, y no escribe.
	if _, err := ro.ListProjects(); err != nil {
		t.Fatalf("una base en sólo lectura debería poder leerse: %v", err)
	}
	if _, err := ro.CreateProject("escritura", nil); err == nil {
		t.Fatal("una base en sólo lectura no debería poder escribir")
	}
	return ro
}

func poblarAPI(t *testing.T, database *DB) {
	t.Helper()
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "tarea", "", "@juan", 2, "backlog")
}

// Los métodos que validan, leen y luego escriben fallan al final, en la
// escritura. Con la conexión cerrada fallarían antes, en la lectura, y su
// comprobación de error quedaría sin cubrir.
func TestWritesFailAfterTheReadsSucceed(t *testing.T) {
	t.Run("CreateTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).CreateTask("api", "nueva", "", "", 2, "backlog"); err == nil {
			t.Error("CreateTask en sólo lectura: want error")
		}
	})
	t.Run("MoveTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).MoveTask(1, "doing"); err == nil {
			t.Error("MoveTask en sólo lectura: want error")
		}
	})
	t.Run("StartTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).StartTask(1); err == nil {
			t.Error("StartTask en sólo lectura: want error")
		}
	})
	t.Run("ReviewTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).ReviewTask(1); err == nil {
			t.Error("ReviewTask en sólo lectura: want error")
		}
	})
	t.Run("DoneTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).DoneTask(1); err == nil {
			t.Error("DoneTask en sólo lectura: want error")
		}
	})
	t.Run("CancelTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).CancelTask(1); err == nil {
			t.Error("CancelTask en sólo lectura: want error")
		}
	})
	t.Run("UpdateTask", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).UpdateTask(1, map[string]any{"title": "otro"}); err == nil {
			t.Error("UpdateTask en sólo lectura: want error")
		}
	})
	t.Run("SetTaskTags", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).SetTaskTags(1, []string{"x"}); err == nil {
			t.Error("SetTaskTags en sólo lectura: want error")
		}
	})
	t.Run("AddTaskTags", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).AddTaskTags(1, []string{"x"}); err == nil {
			t.Error("AddTaskTags en sólo lectura: want error")
		}
	})
	t.Run("RemoveTaskTags", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).RemoveTaskTags(1, []string{"x"}); err == nil {
			t.Error("RemoveTaskTags en sólo lectura: want error")
		}
	})
	t.Run("ArchiveProject", func(t *testing.T) {
		if err := readonlyDB(t, poblarAPI).ArchiveProject("api"); err == nil {
			t.Error("ArchiveProject en sólo lectura: want error")
		}
	})
	t.Run("UnarchiveProject", func(t *testing.T) {
		if err := readonlyDB(t, poblarAPI).UnarchiveProject("api"); err == nil {
			t.Error("UnarchiveProject en sólo lectura: want error")
		}
	})
	t.Run("DeleteProject", func(t *testing.T) {
		if err := readonlyDB(t, poblarAPI).DeleteProject("api"); err == nil {
			t.Error("DeleteProject en sólo lectura: want error")
		}
	})
	t.Run("AddComment", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).AddComment(1, "hola"); err == nil {
			t.Error("AddComment en sólo lectura: want error")
		}
	})
	t.Run("AddOffDay", func(t *testing.T) {
		if _, err := readonlyDB(t, poblarAPI).AddOffDay("@juan", "2026-03-01", "2026-03-02", ""); err == nil {
			t.Error("AddOffDay en sólo lectura: want error")
		}
	})
	t.Run("DeleteOffDay", func(t *testing.T) {
		if err := readonlyDB(t, poblarAPI).DeleteOffDay(1); err == nil {
			t.Error("DeleteOffDay en sólo lectura: want error")
		}
	})
	t.Run("UpdateProject renombrando", func(t *testing.T) {
		if err := readonlyDB(t, poblarAPI).UpdateProject("api", map[string]any{"name": "api2"}); err == nil {
			t.Error("UpdateProject en sólo lectura: want error")
		}
	})
}

// UpdateProject cuenta las tareas de cada estado que se va a eliminar del
// workflow. Esa cuenta es una lectura, así que ni la conexión cerrada ni el
// sólo lectura la alcanzan: hace falta que la consulta exista en el momento de
// la validación y que no exista un instante después.
func TestUpdateProjectCountsTasksOfRemovedStates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sabotaje.db")

	viva, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	poblarAPI(t, viva)

	// Se renombra la tabla entre medias: la lectura del proyecto funciona, la
	// cuenta de tareas no.
	if _, err := viva.Conn().Exec(`ALTER TABLE tasks RENAME TO tasks_bak`); err != nil {
		t.Fatalf("renombrando tasks: %v", err)
	}

	err = viva.UpdateProject("api", map[string]any{
		"workflow": []string{"doing", "done", "cancelled"},
	})
	if err == nil {
		t.Fatal("UpdateProject contando sobre una tabla que no existe: want error")
	}

	_ = viva.Close()
}

// CreateProject también valida el workflow antes de insertar, y con la base
// cerrada la validación salta primero: por eso el error de la base no aparece
// nunca con un workflow inválido.
func TestCreateProjectFailsAtTheInsert(t *testing.T) {
	if _, err := readonlyDB(t, func(*testing.T, *DB) {}).CreateProject("api", nil); err == nil {
		t.Error("CreateProject en sólo lectura: want error")
	}
}
