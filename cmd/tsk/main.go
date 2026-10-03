// tsk — Task Manager TUI + CLI para IA.
//
// Gestor de tareas diseñado para programadores que trabajan en 1-3
// proyectos simultáneamente, con integración total vía CLI para agentes IA.
package main

import (
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"tsk/internal/cli"
	"tsk/internal/config"
	"tsk/internal/db"
	"tsk/internal/tui"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, launchTUI))
}

// run es el main con retorno en vez de os.Exit, y con las tres palabras que se
// puede sustituir en un test: argumentos, salida y error.
//
// La razón de separarlo no es testeabilidad por taste sino que main() no se
// puede probar: os.Exit mata el proceso de test en mitad. Con run() devolviendo
// un código, el test comprueba el código y el mensaje; y el test de subprocess
// cubre el main() de verdad, incluyendo su propio os.Exit.
//
// La TUI no se arranca desde aquí de forma testeable -- Bubbletea necesita una
// terminal -- así que run() recibe el arranque como un argumento más. Es la
// misma costura que el resto del programa usa para lo que no se puede ejecutar
// en un test, y aquí tiene la forma más simple posible: una función.
func run(args []string, stdout, stderr io.Writer, launch func(*db.DB, config.Config) error) int {
	if cli.Run(args) {
		return 0
	}

	// TUI mode
	cfg := config.Load()
	dbPath := cfg.Database.Path
	if dbPath == "" {
		var err error
		dbPath, err = db.DefaultPath()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "tsk:", err)
			return 1
		}
	}

	database, err := db.Open(dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "tsk:", err)
		return 1
	}
	// El error de Close se ignora: es limpieza antes de salir.
	defer func() { _ = database.Close() }()

	if err := launch(database, cfg); err != nil {
		_, _ = fmt.Fprintln(stderr, "tsk:", err)
		return 1
	}
	return 0
}

// launchTUI es el arranque real, el único trozo de run() que un test no puede
// ejecutar porque se queda esperando a que el usuario pulse ctrl+c.
func launchTUI(database *db.DB, cfg config.Config) error {
	_, err := tea.NewProgram(tui.New(database, cfg)).Run()
	return err
}
