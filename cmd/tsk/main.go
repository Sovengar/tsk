// tsk — Task Manager TUI + CLI for AI.
//
// Task manager designed for developers working on 1-3
// projects simultaneously, with full CLI integration for AI agents.
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

// run is main with a return value instead of os.Exit, and with the three things that can
// be substituted in a test: arguments, output and error.
//
// The reason for separating it is not testability by taste but that main() cannot
// be tested: os.Exit kills the test process halfway. With run() returning a
// code, the test checks the code and the message; and the subprocess test
// covers the real main(), including its own os.Exit.
//
// The TUI is not started from here in a testable way -- Bubbletea needs a
// terminal -- so run() receives the launcher as one more argument. It is the
// same seam the rest of the program uses for what cannot be executed
// in a test, and here it has the simplest possible shape: a function.
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
	// The Close error is ignored: it is cleanup before exiting.
	defer func() { _ = database.Close() }()

	if err := launch(database, cfg); err != nil {
		_, _ = fmt.Fprintln(stderr, "tsk:", err)
		return 1
	}
	return 0
}

// launchTUI is the real startup, the only piece of run() that a test cannot
// execute because it waits for the user to press ctrl+c.
func launchTUI(database *db.DB, cfg config.Config) error {
	_, err := tea.NewProgram(tui.New(database, cfg)).Run()
	return err
}
