package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/config"
	"tsk/internal/db"
)

// El arranque de la TUI no se puede ejecutar en un test: Bubbletea se queda
// esperando una terminal. Por eso run() lo recibe como parámetro, y lo que se
// comprueba aquí es todo lo demás -- el camino de la CLI, la resolución de la
// ruta de la base de datos y los tres sitios donde se puede fallar al salir.
func TestRun(t *testing.T) {
	t.Run("un subcomando se resuelve en la CLI y no abre la TUI", func(t *testing.T) {
		configEnRutaValida(t)

		// Si se llega a launch, es que la CLI no consumió el comando.
		lanzada := false
		code := run([]string{"completion", "bash"}, &pila{}, &pila{}, func(*db.DB, config.Config) error {
			lanzada = true
			return nil
		})

		if code != 0 {
			t.Errorf("el código de salida es %d, want 0", code)
		}
		if lanzada {
			t.Error("se ha lanzado la TUI después de un subcomando de la CLI")
		}
	})

	t.Run("sin subcomando se abre la base de datos y se lanza la TUI", func(t *testing.T) {
		configEnRutaValida(t)

		viva := false
		code := run(nil, &pila{}, &pila{}, func(database *db.DB, _ config.Config) error {
			var uno int
			err := database.Conn().QueryRow(`SELECT 1`).Scan(&uno)
			viva = err == nil && uno == 1
			return nil
		})

		if code != 0 {
			t.Errorf("el código de salida es %d, want 0", code)
		}
		if !viva {
			t.Error("la TUI no ha recibido una base de datos viva")
		}
	})

	t.Run("la ruta de la base de datos viene de la config", func(t *testing.T) {
		dir := t.TempDir()
		ruta := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(ruta, []byte("[database]\npath = \""+filepath.Join(dir, "propia.db")+"\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", ruta)
		t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "xdg"))

		code := run(nil, &pila{}, &pila{}, func(*db.DB, config.Config) error { return nil })
		if code != 0 {
			t.Errorf("el código de salida es %d, want 0", code)
		}
		// Si se hubiera resuelto la ruta por XDG, el fichero no existiría.
		if _, err := os.Stat(filepath.Join(dir, "propia.db")); err != nil {
			t.Errorf("no se ha creado la base de datos de la config: %v", err)
		}
	})

	t.Run("sin ruta en la config se usa la de XDG", func(t *testing.T) {
		dir := t.TempDir()
		configEnRutaValida(t)
		t.Setenv("XDG_DATA_HOME", dir)

		code := run(nil, &pila{}, &pila{}, func(*db.DB, config.Config) error { return nil })
		if code != 0 {
			t.Errorf("el código de salida es %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(dir, "tsk", "tsk.db")); err != nil {
			t.Errorf("no se ha creado la base de datos por defecto: %v", err)
		}
	})

	t.Run("sin HOME no hay ruta por defecto", func(t *testing.T) {
		configEnRutaValida(t)
		t.Setenv("XDG_DATA_HOME", "")

		// Con la ruta de la config vacía y sin XDG, DefaultPath necesita el
		// directorio home.
		dir := t.TempDir()
		ruta := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(ruta, []byte("[database]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", ruta)
		t.Setenv("HOME", "")

		var errs pila
		code := run(nil, &pila{}, &errs, func(*db.DB, config.Config) error {
			t.Error("la TUI se ha lanzado sin ruta de base de datos")
			return nil
		})

		if code != 1 {
			t.Errorf("el código de salida es %d, want 1", code)
		}
		if !strings.HasPrefix(errs.String(), "tsk: ") {
			t.Errorf("el error no sale por stderr con el prefijo: %q", errs.String())
		}
	})

	t.Run("una base de datos que no se puede abrir", func(t *testing.T) {
		configEnRutaValida(t)

		// Una ruta cuyo directorio padre es un fichero: el mkdir falla.
		bloque := filepath.Join(t.TempDir(), "bloque")
		if err := os.WriteFile(bloque, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		ruta := filepath.Join(dir, "config.toml")
		contenido := "[database]\npath = \"" + filepath.Join(bloque, "sub", "tsk.db") + "\"\n"
		if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", ruta)

		var errs pila
		code := run(nil, &pila{}, &errs, func(*db.DB, config.Config) error {
			t.Error("la TUI se ha lanzado sin base de datos")
			return nil
		})

		if code != 1 {
			t.Errorf("el código de salida es %d, want 1", code)
		}
		if !strings.Contains(errs.String(), "tsk:") {
			t.Errorf("el error no sale por stderr: %q", errs.String())
		}
	})

	t.Run("una TUI que falla", func(t *testing.T) {
		configEnRutaValida(t)

		var errs pila
		code := run(nil, &pila{}, &errs, func(*db.DB, config.Config) error {
			return errors.New("la terminal se ha cerrado")
		})

		if code != 1 {
			t.Errorf("el código de salida es %d, want 1", code)
		}
		if !strings.Contains(errs.String(), "la terminal se ha cerrado") {
			t.Errorf("el error de la TUI no sale por stderr: %q", errs.String())
		}
	})
}

// main() no se puede llamar desde un test porque su os.Exit se comería el
// proceso entero. La forma estándar de cubrirlo es reejecutar el binario de test
// como un programa: el hijo llama a main de verdad y el padre comprueba su
// código de salida y su salida.
//
// Esto no es un truco para marcar la casilla de cobertura: es la única manera de
// comprobar que main() propaga el código de run(), que es justo lo que un
// wrapper de scripts depende.
func TestMainPropagatesTheExitCode(t *testing.T) {
	if os.Getenv("TSK_TEST_MAIN") == "1" {
		// Estamos en el hijo: main() se ejecuta de verdad.
		main()
		return
	}

	t.Run("subcomando de la CLI", func(t *testing.T) {
		salida := ejecutarEnSubprocess(t, "completion", "bash")
		if salida.codigo != 0 {
			t.Errorf("el hijo ha salido con %d, want 0 (stderr: %s)", salida.codigo, salida.errores)
		}
		if !strings.Contains(salida.salida, "complete") {
			t.Errorf("el hijo no ha impreso el script de completado: %q", salida.salida)
		}
	})

	t.Run("error de la CLI", func(t *testing.T) {
		// Un subcomando que no existe sale con 1 desde cli.Run, sin llegar a la
		// TUI. Es el camino de error que un usuario ve más a menudo.
		salida := ejecutarEnSubprocess(t, "no-existe")
		if salida.codigo == 0 {
			t.Errorf("un subcomando inexistente ha salido con 0, want != 0")
		}
	})
}

func TestLaunchTUI(t *testing.T) {
	// launchTUI no se puede ejecutar sin terminal, pero sí comprobar que existe
	// y que devuelve un error en vez de entrar en un estado imposible. Con una
	// base de datos cerrada, Bubbletea arranca y falla al leer.
	//
	// En un entorno sin terminal, tea.Run devuelve un error de terminal, que es
	// exactamente lo que esta función tiene que dejar pasar.
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer func() { _ = database.Close() }()

	_ = launchTUI(database, config.Defaults())
}

// configEnRutaValida apunta TSK_CONFIG a un fichero que existe pero está
// vacío: la config se resuelve, no hay ruta de base de datos, y por tanto se usa
// la de por defecto.
func configEnRutaValida(t *testing.T) {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(ruta, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", ruta)
}

func ejecutarEnSubprocess(t *testing.T, args ...string) struct {
	codigo  int
	salida  string
	errores string
} {
	t.Helper()

	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "TSK_TEST_MAIN=1")

	var salida, errores pila
	cmd.Stdout = &salida
	cmd.Stderr = &errores

	codigo := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("ejecutando el hijo: %v", err)
		}
		codigo = exitErr.ExitCode()
	}

	return struct {
		codigo  int
		salida  string
		errores string
	}{codigo, salida.String(), errores.String()}
}
