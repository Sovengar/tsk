package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// closedDB devuelve una base de datos con la conexión ya cerrada.
//
// Es la palanca para los caminos de error de todos los métodos: SQLite responde
// "sql: database is closed" a cualquier sentencia, así que un test por método
// cubre el `if err != nil` que hay detrás de cada consulta sin tener que
// inventar un disco lleno ni una tabla corrupta.
func closedDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return database
}

// Cada método de la DB tiene una comprobación de error detrás de su consulta.
// Con la conexión cerrada todos esos `if` se ejecutan; lo que se comprueba es
// que el método devuelve el error en vez de tragárselo o devolver un valor vacío
// haciéndose el listo.
func TestEveryQueryFailsOnAClosedConnection(t *testing.T) {
	database := closedDB(t)

	t.Run("proyectos", func(t *testing.T) {
		if _, err := database.CreateProject("api", nil); err == nil {
			t.Error("CreateProject sin conexión: want error")
		}
		if _, err := database.CreateProjectWithListOrder("api", nil, nil); err == nil {
			t.Error("CreateProjectWithListOrder sin conexión: want error")
		}
		if _, err := database.GetProject("api"); err == nil {
			t.Error("GetProject sin conexión: want error")
		}
		if _, err := database.GetProjectByID(1); err == nil {
			t.Error("GetProjectByID sin conexión: want error")
		}
		if _, err := database.ListProjects(); err == nil {
			t.Error("ListProjects sin conexión: want error")
		}
		if _, err := database.ListArchivedProjects(); err == nil {
			t.Error("ListArchivedProjects sin conexión: want error")
		}
		if err := database.ArchiveProject("api"); err == nil {
			t.Error("ArchiveProject sin conexión: want error")
		}
		if err := database.UnarchiveProject("api"); err == nil {
			t.Error("UnarchiveProject sin conexión: want error")
		}
		if err := database.UpdateProject("api", map[string]any{"name": "api2"}); err == nil {
			t.Error("UpdateProject sin conexión: want error")
		}
		if err := database.DeleteProject("api"); err == nil {
			t.Error("DeleteProject sin conexión: want error")
		}
		if _, err := database.ProjectTaskCount(1); err == nil {
			t.Error("ProjectTaskCount sin conexión: want error")
		}
	})

	t.Run("tareas", func(t *testing.T) {
		if _, err := database.CreateTask("api", "t", "", "", 2, "backlog"); err == nil {
			t.Error("CreateTask sin conexión: want error")
		}
		if _, err := database.CreateTaskWithEstimate("api", "t", "", "", 2, "backlog", 1); err == nil {
			t.Error("CreateTaskWithEstimate sin conexión: want error")
		}
		if _, err := database.CreateTaskFull("api", "t", "", "", 2, "backlog", 1, []string{"x"}); err == nil {
			t.Error("CreateTaskFull sin conexión: want error")
		}
		if _, err := database.GetTask(1); err == nil {
			t.Error("GetTask sin conexión: want error")
		}
		if _, err := database.ListTasks("", "", ""); err == nil {
			t.Error("ListTasks sin conexión: want error")
		}
		if _, err := database.MoveTask(1, "doing"); err == nil {
			t.Error("MoveTask sin conexión: want error")
		}
		if _, err := database.StartTask(1); err == nil {
			t.Error("StartTask sin conexión: want error")
		}
		if _, err := database.ReviewTask(1); err == nil {
			t.Error("ReviewTask sin conexión: want error")
		}
		if _, err := database.DoneTask(1); err == nil {
			t.Error("DoneTask sin conexión: want error")
		}
		if _, err := database.CancelTask(1); err == nil {
			t.Error("CancelTask sin conexión: want error")
		}
		if _, err := database.UpdateTask(1, map[string]any{"title": "x"}); err == nil {
			t.Error("UpdateTask sin conexión: want error")
		}
		if _, err := database.SetTaskTags(1, []string{"x"}); err == nil {
			t.Error("SetTaskTags sin conexión: want error")
		}
		if _, err := database.AddTaskTags(1, []string{"x"}); err == nil {
			t.Error("AddTaskTags sin conexión: want error")
		}
		if _, err := database.RemoveTaskTags(1, []string{"x"}); err == nil {
			t.Error("RemoveTaskTags sin conexión: want error")
		}
		if _, err := database.Stats(""); err == nil {
			t.Error("Stats sin conexión: want error")
		}
	})

	t.Run("off-days", func(t *testing.T) {
		if _, err := database.AddOffDay("@juan", "2026-03-01", "2026-03-02", ""); err == nil {
			t.Error("AddOffDay sin conexión: want error")
		}
		if _, err := database.ListOffDays(""); err == nil {
			t.Error("ListOffDays sin conexión: want error")
		}
		if err := database.DeleteOffDay(1); err == nil {
			t.Error("DeleteOffDay sin conexión: want error")
		}
	})

	t.Run("comentarios", func(t *testing.T) {
		if _, err := database.AddComment(1, "hola"); err == nil {
			t.Error("AddComment sin conexión: want error")
		}
		if _, err := database.ListComments(1); err == nil {
			t.Error("ListComments sin conexión: want error")
		}
		if err := database.DeleteComment(1); err == nil {
			t.Error("DeleteComment sin conexión: want error")
		}
	})
}

// NewTestDB es la base de datos de todos los tests de la TUI, así que estaba
// sin cubrir sin que nada se notara: si se rompiera, los otros tests seguirían
// en verde porque no llegarían ni a abrirla.
func TestNewTestDBAndConnAreUsable(t *testing.T) {
	database, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	conn := database.Conn()
	if conn == nil {
		t.Fatal("Conn devolvió nil")
	}
	// Vive de verdad: una sentencia de escritura y una de lectura.
	if _, err := conn.Exec(`CREATE TABLE probe (n INTEGER)`); err != nil {
		t.Fatalf("Exec sobre Conn: %v", err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&n); err != nil {
		t.Fatalf("QueryRow sobre Conn: %v", err)
	}
	if n != 0 {
		t.Errorf("la tabla recién creada tiene %d filas, want 0", n)
	}
}

// DefaultPath tiene dos ramas -- XDG_DATA_HOME puesto y no puesto -- y la segunda
// necesita el directorio home, que en un entorno de CI puede no existir.
func TestDefaultPathFollowsXDGDataHome(t *testing.T) {
	t.Run("xdg", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "/xdg/data")
		got, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath: %v", err)
		}
		if want := filepath.Join("/xdg/data", "tsk", "tsk.db"); got != want {
			t.Errorf("DefaultPath = %q, want %q", got, want)
		}
	})

	t.Run("home", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath: %v", err)
		}
		if want := filepath.Join(home, ".local", "share", "tsk", "tsk.db"); got != want {
			t.Errorf("DefaultPath = %q, want %q", got, want)
		}
	})

	t.Run("sin home", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		// En Linux os.UserHomeDir sólo mira $HOME, así que vaciarlo es una
		// forma limpia de provocar el error sin desconfigurar la máquina.
		t.Setenv("HOME", "")
		if _, err := DefaultPath(); err == nil {
			t.Error("DefaultPath sin HOME: want error")
		}
	})
}

// Open falla de tres maneras distintas y cada una merece su propio mensaje,
// porque son fallos que el usuario ve en la terminal.
func TestOpenFailureModes(t *testing.T) {
	t.Run("el directorio padre no se puede crear", func(t *testing.T) {
		// Un fichero normal en el sitio donde debería ir el directorio: mkdir
		// encima de un fichero falla con ENOTDIR.
		bloque := filepath.Join(t.TempDir(), "bloque")
		if err := os.WriteFile(bloque, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Open(filepath.Join(bloque, "sub", "tsk.db"))
		if err == nil {
			t.Fatal("Open con un padre que es un fichero: want error")
		}
		if !strings.Contains(err.Error(), "create db dir") {
			t.Errorf("el error no menciona la creación del directorio: %v", err)
		}
	})

	t.Run("la ruta no es una base de datos", func(t *testing.T) {
		// Un fichero de texto donde SQLite espera un fichero de base de datos:
		// sql.Open no falla, pero la migración sí.
		ruta := filepath.Join(t.TempDir(), "no-es-db")
		if err := os.WriteFile(ruta, []byte("esto no es sqlite"), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Open(ruta)
		if err == nil {
			t.Fatal("Open sobre un fichero que no es una DB: want error")
		}
		if !strings.Contains(err.Error(), "migrate") {
			t.Errorf("el error no menciona la migración: %v", err)
		}
	})

	t.Run("una migración que devuelve error", func(t *testing.T) {
		restore := migrations
		t.Cleanup(func() { migrations = restore })
		migrations = []struct {
			version string
			query   string
			run     func(*DB) error
		}{
			{version: "9999", run: func(*DB) error { return os.ErrPermission }},
		}

		_, err := Open(":memory:")
		if err == nil {
			t.Fatal("Open con una migración que falla: want error")
		}
		if !strings.Contains(err.Error(), "migration 9999") {
			t.Errorf("el error no nombra la migración: %v", err)
		}
	})

	t.Run("una migración con SQL inválido", func(t *testing.T) {
		restore := migrations
		t.Cleanup(func() { migrations = restore })
		migrations = []struct {
			version string
			query   string
			run     func(*DB) error
		}{
			{version: "9999", query: "ESTO NO ES SQL"},
		}

		if _, err := Open(":memory:"); err == nil {
			t.Fatal("Open con una migración de SQL inválido: want error")
		}
	})

	t.Run("una migración que se carga el _meta", func(t *testing.T) {
		// Si la migración borra la tabla donde después se anota la versión, el
		// INSERT falla. Es el único camino para llegar a ese error sin tocar el
		// driver.
		restore := migrations
		t.Cleanup(func() { migrations = restore })
		migrations = []struct {
			version string
			query   string
			run     func(*DB) error
		}{
			{version: "9999", query: "DROP TABLE _meta"},
		}

		_, err := Open(":memory:")
		if err == nil {
			t.Fatal("Open con una migración que borra _meta: want error")
		}
		if !strings.Contains(err.Error(), "update version") {
			t.Errorf("el error no menciona la actualización de versión: %v", err)
		}
	})
}

// CreateProject valida el workflow antes de tocar la base de datos, así que un
// workflow inválido se detecta con la DB cerrada: prueba de que la validación no
// depende de la conexión.
func TestProjectValidationHappensBeforeTheQuery(t *testing.T) {
	database := closedDB(t)

	_, err := database.CreateProject("api", []string{"nope"})
	if err == nil {
		t.Fatal("CreateProject con un workflow inválido: want error")
	}
	// El mensaje viene del validador, no de SQLite: eso es justo lo que se
	// quiere comprobar, porque con la DB cerrada cualquier error sería de la
	// conexión y el test pasaría sin probar nada.
	if strings.Contains(err.Error(), "closed") {
		t.Errorf("el error es de la conexión, no del validador: %v", err)
	}
}
