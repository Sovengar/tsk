// tsk — Task Manager TUI + CLI para IA.
//
// Gestor de tareas diseñado para programadores que trabajan en 1-3
// proyectos simultáneamente, con integración total vía CLI para agentes IA.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"taskd/internal/cli"
	"taskd/internal/config"
	"taskd/internal/db"
	"taskd/internal/tui"
)

func main() {
	if cli.Run(os.Args[1:]) {
		return
	}

	// TUI mode
	cfg := config.Load()
	dbPath := cfg.Database.Path
	if dbPath == "" {
		var err error
		dbPath, err = db.DefaultPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "taskd:", err)
			os.Exit(1)
		}
	}

	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "taskd:", err)
		os.Exit(1)
	}
	defer database.Close()

	model := tui.New(database, cfg)
	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "taskd:", err)
		os.Exit(1)
	}
}
