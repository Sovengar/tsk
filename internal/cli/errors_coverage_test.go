package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/db"
)

// El contrato de la CLI para un agente IA es que todo fallo salga como JSON por
// stderr con código 1. Eso significa que cada `if err != nil` de cada comando es
// una línea de contrato, no una manejo interno: si uno se cuela, el agente se
// queda sin diagnóstico.
//
// Lo que no se probaba era ninguno de ellos. Todos se alcanzan por la misma vía:
// una base de datos que responde bien al leer y mal al escribir, o que no
// responde a nada.

// conDBRota apunta la config a una ruta que no es una base de datos: la
// conexión se abre -- sql.Open con modernc nunca falla -- pero cualquier
// consulta falla. Es el fallo más temprano posible.
func conDBRota(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	ruta := filepath.Join(dir, "no-es-una-base")
	if err := os.MkdirAll(ruta, 0o755); err != nil {
		t.Fatal(err)
	}
	apuntarConfig(t, ruta)
}

// conDBSoloLectura deja una base de datos real y legible pero de la que no se
// puede escribir. Es la única forma de alcanzar los errores de escritura de los
// comandos que antes leen.
//
// Se hace con triggers y no con permisos: SQLite escribe en el WAL, que sigue
// siendo escribible aunque el fichero principal no lo sea, así que chmod no
// impide la escritura. Un trigger que aborta sí, y además pinpointa la tabla.
func conDBSoloLectura(t *testing.T) string {
	t.Helper()
	ruta := conDBReal(t)

	escritor, err := db.Open(ruta)
	if err != nil {
		t.Fatalf("abrir la base para sabotearla: %v", err)
	}
	defer func() { _ = escritor.Close() }()

	for _, tabla := range []string{"tasks", "projects", "comments", "offdays"} {
		stmt := `CREATE TRIGGER solo_lectura_` + tabla + ` BEFORE INSERT ON ` + tabla +
			` BEGIN SELECT RAISE(ABORT, 'base de sólo lectura'); END`
		if _, err := escritor.Conn().Exec(stmt); err != nil {
			t.Fatalf("trigger de %s: %v", tabla, err)
		}
		stmt = `CREATE TRIGGER solo_lectura_upd_` + tabla + ` BEFORE UPDATE ON ` + tabla +
			` BEGIN SELECT RAISE(ABORT, 'base de sólo lectura'); END`
		if _, err := escritor.Conn().Exec(stmt); err != nil {
			t.Fatalf("trigger de %s: %v", tabla, err)
		}
		stmt = `CREATE TRIGGER solo_lectura_del_` + tabla + ` BEFORE DELETE ON ` + tabla +
			` BEGIN SELECT RAISE(ABORT, 'base de sólo lectura'); END`
		if _, err := escritor.Conn().Exec(stmt); err != nil {
			t.Fatalf("trigger de %s: %v", tabla, err)
		}
	}

	// Comprobación de que el sabotaje funciona: una escritura tiene que fallar.
	if _, err := escritor.Conn().Exec(`UPDATE tasks SET title = 'x'`); err == nil {
		t.Fatal("el sabotaje no impide escribir")
	}
	// Y una lectura sigue funcionando.
	if _, err := escritor.Conn().Exec(`SELECT 1`); err != nil {
		t.Fatalf("el sabotaje ha roto la lectura: %v", err)
	}

	return ruta
}

// conDBReal crea una base de datos con un proyecto y una tarea, y devuelve la
// ruta.
func conDBReal(t *testing.T) string {
	t.Helper()
	ruta := conDBVacia(t)
	salida, code := run(t, "project", "add", "api", "--workflow", "backlog,done")
	if code != 0 {
		t.Fatalf("project add: %s", salida)
	}
	if _, code := run(t, "add", "una tarea", "--project", "api", "--priority", "1"); code != 0 {
		t.Fatal("add ha fallado")
	}
	return ruta
}

func conDBVacia(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ruta := filepath.Join(dir, "tsk.db")
	apuntarConfig(t, ruta)
	return ruta
}

// apuntarConfig escribe un config que apunta a la ruta dada.
func apuntarConfig(t *testing.T, ruta string) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.toml")
	body := "[database]\npath = \"" + ruta + "\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", cfg)
}

// Cuando la base de datos no se puede ni abrir, todos los comandos tienen que
// salir con 1 y un JSON de error. openDB aborta antes de despachar, así que esto
// no ejercita las ramas de error de cada comando: las ejercitan los sabotajes
// de más abajo. Lo que comprueba es que ninguna commande se quede con un 0
// silencioso por no poder abrir la base.
func TestEveryCommandFailsWhenTheDatabaseCannotBeOpened(t *testing.T) {
	for _, args := range [][]string{
		{"project", "list"},
		{"project", "list", "--archived"},
		{"project", "show", "api"},
		{"project", "update", "api", "--name", "otro"},
		{"project", "remove", "api"},
		{"project", "archive", "api"},
		{"project", "unarchive", "api"},
		{"list"},
		{"list", "--project", "api"},
		{"show", "1"},
		{"comment", "add", "1", "hola"},
		{"comment", "list", "1"},
		{"comment", "remove", "1"},
		{"offday", "list"},
		{"offday", "add", "@juan", "2026-01-01", "2026-01-02"},
		{"offday", "delete", "1"},
		{"gantt"},
		{"gantt", "--project", "api"},
		{"start", "1"},
		{"done", "1"},
		{"cancel", "1"},
		{"stats"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			conDBRota(t)
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("el código de salida es %d, want 1", code)
			}
		})
	}
}

// Los errores de escritura de los comandos que primero leen la tarea: update con
// tags, start, done y cancel. La lectura va bien, la escritura no, y el comando
// tiene que decirlo.
func TestWriteCommandsFailWhenTheDatabaseIsReadOnly(t *testing.T) {
	conDBSoloLectura(t)

	for _, args := range [][]string{
		{"start", "1"},
		{"done", "1"},
		{"cancel", "1"},
		{"update", "1", "--tags", "nueva"},
		{"update", "1", "--untag", "nueva"},
		{"update", "1", "--tag", "nueva"},
		{"update", "1", "--title", "otro"},
		{"comment", "add", "1", "hola"},
		{"comment", "remove", "1"},
		{"offday", "add", "@juan", "2026-01-01", "2026-01-02"},
		{"offday", "delete", "1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("el código de salida es %d, want 1 con la base de sólo lectura", code)
			}
		})
	}
}

// Los errores de escritura de los comandos de proyecto, que no leen antes de
// escribir.
func TestProjectWritesFailOnAReadOnlyDatabase(t *testing.T) {
	conDBSoloLectura(t)

	for _, args := range [][]string{
		{"project", "add", "otro"},
		{"project", "update", "api", "--name", "otro"},
		{"project", "remove", "api"},
		{"project", "archive", "api"},
		{"project", "unarchive", "api"},
		{"add", "una tarea nueva", "--project", "api"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("el código de salida es %d, want 1", code)
			}
		})
	}
}

// La ruta de la base de datos sale de la config; si la config no la trae, sale
// de XDG. Sin ninguno de los dos -- y sin HOME -- no hay dónde abrir nada.
func TestDBPathResolution(t *testing.T) {
	t.Run("sin HOME ni XDG", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(cfg, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", cfg)
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", "")

		_, code := run(t, "project", "list")
		if code != 1 {
			t.Errorf("sin ruta de base de datos el código es %d, want 1", code)
		}
	})

	t.Run("una ruta que no se puede crear", func(t *testing.T) {
		bloque := filepath.Join(t.TempDir(), "bloque")
		if err := os.WriteFile(bloque, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		apuntarConfig(t, filepath.Join(bloque, "sub", "tsk.db"))

		_, code := run(t, "project", "list")
		if code != 1 {
			t.Errorf("con una ruta imposible el código es %d, want 1", code)
		}
	})

	t.Run("sin ruta en la config se usa la de XDG", func(t *testing.T) {
		dir := t.TempDir()
		cfg := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(cfg, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", cfg)
		t.Setenv("XDG_DATA_HOME", dir)

		salida, code := run(t, "project", "add", "api")
		if code != 0 {
			t.Fatalf("project add: %s", salida)
		}
		if _, err := os.Stat(filepath.Join(dir, "tsk", "tsk.db")); err != nil {
			t.Errorf("no se ha creado la base de datos de XDG: %v", err)
		}
	})
}

// Los mensajes de uso son parte del contrato: un agente que se equivoca de
// argumentos tiene que leer un texto que le diga cuáles son válidos.
func TestUsageErrors(t *testing.T) {
	conDBVacia(t)

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"project"}, "usage: tsk project"},
		{[]string{"project", "show"}, "usage: tsk project show"},
		{[]string{"project", "update"}, "usage: tsk project update"},
		{[]string{"project", "remove"}, "usage: tsk project remove"},
		{[]string{"project", "archive"}, "usage: tsk project archive"},
		{[]string{"project", "unarchive"}, "usage: tsk project unarchive"},
		{[]string{"project", "lo-que-sea"}, "unknown project subcommand: lo-que-sea"},
		{[]string{"completion", "tcsh"}, "usage: tsk completion"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			capture(t)
			errs := &cola{}
			origErr := stderr
			stderr = errs
			t.Cleanup(func() { stderr = origErr })

			code := 0
			func() {
				defer func() {
					if r := recover(); r != nil {
						e, ok := r.(exitPanic)
						if !ok {
							panic(r)
						}
						code = e.code
					}
				}()
				Run(tc.args)
			}()

			if code != 1 {
				t.Errorf("el código de salida es %d, want 1", code)
			}
			if !strings.Contains(errs.String(), tc.want) {
				t.Errorf("el error no dice %q: %s", tc.want, errs.String())
			}
		})
	}
}

// gantt acepta una fecha de arranque y un número de semanas. Una fecha que no
// es una fecha tiene que producir un error que la diga, no una gantt con fechas a
// cero; y un --weeks que no es un número se ignora en vez de tumbar el comando.
func TestGanttRejectsABadDate(t *testing.T) {
	conDBReal(t)

	t.Run("--from inválido", func(t *testing.T) {
		_, code := run(t, "gantt", "--from", "ayer")
		if code != 1 {
			t.Errorf("con --from inválido el código es %d, want 1", code)
		}
	})

	t.Run("--from sin valor", func(t *testing.T) {
		// El flag sin lo que sigue se ignora en vez de comerse el argumento
		// siguiente: es el contrato de los flags de la CLI.
		if _, code := run(t, "gantt", "--from"); code != 0 {
			t.Errorf("con --from solo el código es %d, want 0", code)
		}
	})

	t.Run("sin shell", func(t *testing.T) {
		// Sin argumento no hay shell y el comando no hace nada, en vez de
		// imprimir el completado equivocado.
		if _, code := run(t, "completion"); code != 0 {
			t.Errorf("completion sin shell tiene el código %d, want 0", code)
		}
	})

	t.Run("--weeks que no es un número", func(t *testing.T) {
		// Un --weeks inválido no es un error: se ignora y gana el de la config.
		// Lo que no puede ser es un rango de semanas negativo.
		if _, code := run(t, "gantt", "--weeks", "muchas"); code != 0 {
			t.Errorf("con --weeks inválido el código es %d, want 0", code)
		}
		if _, code := run(t, "gantt", "--weeks", "-3"); code != 0 {
			t.Errorf("con --weeks negativo el código es %d, want 0", code)
		}
	})

	t.Run("--from bueno", func(t *testing.T) {
		if _, code := run(t, "gantt", "--from", "2026-01-01", "--json"); code != 0 {
			t.Errorf("con una fecha válida el código es %d, want 0", code)
		}
	})
}

// project add con un workflow que no se puede parsear: la lista es sólo de
// comas, que es el caso que el parser rechaza.
func TestProjectAddWithUnparseableLists(t *testing.T) {
	conDBVacia(t)

	for _, args := range [][]string{
		{"project", "add", "api", "--workflow", " , "},
		{"project", "add", "api", "--list-order", " , "},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, code := run(t, args...)
			if code != 1 {
				t.Errorf("el código de salida es %d, want 1", code)
			}
		})
	}
}

// Las ramas de error por comando necesitan una base de datos que abra y migre
// bien -- si no, openDB aborta antes de llegar al comando -- pero cuya lectura
// falle. La forma es la misma que en internal/db: recrear la tabla sin clave
// primaria y meter una fila con un id que no es un número, para que cualquier
// Scan sobre ella reviente.
//
// Cada fila lleva todos sus valores porque las columnas se recrean sin DEFAULT:
// una columna a NULL no cumple un WHERE ni una igualdad, y el veneno se
// colaría sin disparar el error que se quiere provocar.

const (
	ddlProjects = `CREATE TABLE %s (
		id TEXT, name TEXT, workflow TEXT, list_order TEXT, archived INTEGER,
		archived_at TEXT, created_at TEXT, updated_at TEXT)`
	ddlTasks = `CREATE TABLE %s (
		id TEXT, project_id INTEGER, title TEXT, description TEXT, status TEXT,
		priority INTEGER, assignee TEXT, estimate REAL, tags TEXT,
		created_at TEXT, updated_at TEXT, completed_at TEXT)`
	ddlComments = `CREATE TABLE %s (id TEXT, task_id INTEGER, body TEXT, created_at TEXT)`
	ddlOffDays  = `CREATE TABLE %s (id TEXT, assignee TEXT, start_date TEXT, end_date TEXT, note TEXT)`

	// Dos filas de veneno en projects: una activa y una archivada. Con una sola,
	// la consulta que filtra por el otro valor no la ve y sale sin error.
	filaProjects = `INSERT INTO projects VALUES
		('ilegible', 'veneno', '[]', '[]', 0, NULL, '2020-01-01', '2020-01-01'),
		('ilegible', 'veneno-archivado', '[]', '[]', 1, '2020-01-01', '2020-01-01', '2020-01-01')`
	filaTasks = `INSERT INTO tasks VALUES
		('ilegible', 1, 'veneno', '', 'backlog', 0, '', 0, '[]', '2020-01-01', '2020-01-01', NULL)`
	filaComments = `INSERT INTO comments VALUES
		('ilegible', 1, 'veneno', '2020-01-01')`
	filaOffDays = `INSERT INTO offdays VALUES
		('ilegible', '@juan', '2020-01-01', '2020-01-02', 'veneno')`
)

// romperTabla deja la tabla indicada ilegible y devuelve la ruta de la base de
// datos, ya sembrada.
func romperTabla(t *testing.T, tabla, ddl, fila string) string {
	t.Helper()
	ruta := conDBReal(t)

	escritor, err := db.Open(ruta)
	if err != nil {
		t.Fatalf("abrir la base: %v", err)
	}
	defer func() { _ = escritor.Close() }()

	conn := escritor.Conn()
	if _, err := conn.Exec(`PRAGMA foreign_keys(OFF)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`ALTER TABLE ` + tabla + ` RENAME TO ` + tabla + `_sana`); err != nil {
		t.Fatalf("renombrar %s: %v", tabla, err)
	}
	if _, err := conn.Exec(fmt.Sprintf(ddl, tabla)); err != nil {
		t.Fatalf("recrear %s: %v", tabla, err)
	}
	if _, err := conn.Exec(fila); err != nil {
		t.Fatalf("insertar la fila venenosa en %s: %v", tabla, err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys(ON)`); err != nil {
		t.Fatal(err)
	}

	return ruta
}

// Los listados que se ejecutan con la tabla correspondiente rota.
func TestReadsFailWhenTheirTableIsUnreadable(t *testing.T) {
	t.Run("proyectos", func(t *testing.T) {
		apuntarConfig(t, romperTabla(t, "projects", ddlProjects, filaProjects))
		comprobarFalla(t, "project", "list")
		comprobarFalla(t, "project", "list", "--archived")
		comprobarFalla(t, "project", "show", "api")
		comprobarFalla(t, "project", "update", "api", "--name", "otro")
	})

	t.Run("tareas", func(t *testing.T) {
		apuntarConfig(t, romperTabla(t, "tasks", ddlTasks, filaTasks))
		comprobarFalla(t, "list")
		comprobarFalla(t, "show", "1")
		comprobarFalla(t, "update", "1", "--tag", "nueva")
		comprobarFalla(t, "update", "1")
		comprobarFalla(t, "gantt")
	})

	t.Run("comentarios", func(t *testing.T) {
		apuntarConfig(t, romperTabla(t, "comments", ddlComments, filaComments))
		comprobarFalla(t, "show", "1")
		comprobarFalla(t, "comment", "list", "1")
		comprobarFalla(t, "comment", "delete", "1")
	})

	t.Run("off-days", func(t *testing.T) {
		apuntarConfig(t, romperTabla(t, "offdays", ddlOffDays, filaOffDays))
		comprobarFalla(t, "offday", "list")
		comprobarFalla(t, "gantt")
	})

	// Una tabla que no existe es el otro modo de fallo: la consulta se prepara
	// bien pero no encuentra la tabla. Afecta a todo lo que la use en un JOIN,
	// que es el caso de stats: su COUNT cuenta sólo tareas de proyectos no
	// archivados, así que sin projects no hay nada que contar y el comando tiene
	// que decirlo en vez de sacar un total de cero.
	t.Run("proyectos desaparecido", func(t *testing.T) {
		apuntarConfig(t, borrarTabla(t, "projects"))
		comprobarFalla(t, "stats")
	})
}

// borrarTabla elimina la tabla sin dejar nada en su lugar: la consulta se
// prepara pero falla al ejecutarse.
func borrarTabla(t *testing.T, tabla string) string {
	t.Helper()
	ruta := conDBReal(t)

	escritor, err := db.Open(ruta)
	if err != nil {
		t.Fatalf("abrir la base: %v", err)
	}
	defer func() { _ = escritor.Close() }()

	conn := escritor.Conn()
	if _, err := conn.Exec(`PRAGMA foreign_keys(OFF)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`DROP TABLE ` + tabla); err != nil {
		t.Fatalf("DROP TABLE %s: %v", tabla, err)
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys(ON)`); err != nil {
		t.Fatal(err)
	}

	return ruta
}

func comprobarFalla(t *testing.T, args ...string) {
	t.Helper()
	t.Run(strings.Join(args, " "), func(t *testing.T) {
		if _, code := run(t, args...); code != 1 {
			t.Errorf("el código es %d, want 1", code)
		}
	})
}

// Editar un proyecto con una lista que no se puede parsear falla antes de
// tocar la base: el error es del parser, no de la escritura.
func TestProjectUpdateRejectsUnparseableLists(t *testing.T) {
	conDBReal(t)

	for _, args := range [][]string{
		{"project", "update", "api", "--workflow", " , "},
		{"project", "update", "api", "--list-order", " , "},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, code := run(t, args...); code != 1 {
				t.Errorf("el código es %d, want 1", code)
			}
		})
	}
}
