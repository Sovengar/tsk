package db

import (
	"testing"

	"tsk/internal/model"
)

// populatedThenClosed devuelve una base de datos con un proyecto y una tarea
// reales, y luego con la conexión cerrada.
//
// La diferencia con closedDB es que aquí los métodos pasan todas sus
// validaciones previas -- el proyecto existe, el estado es válido, la tarea
// existe -- y fallan en la consulta de verdad. Es lo que cubre los `if err` que
// están detrás de un GetTask o un GetProjectByID, que con una base recién
// cerrada nunca se llegan a ejecutar.
func populatedThenClosed(t *testing.T) *DB {
	t.Helper()
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "tarea", "", "", 2, "backlog")
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return database
}

// Las transiciones de estado validan el destino contra el workflow del
// proyecto ANTES de escribir. Con la base recién cerrada el destino se rechaza
// primero y el `if err` de la escritura nunca se ve; con la base poblada y
// cerrada se llega.
func TestStatusChangesFailAtTheWriteWithAClosedDB(t *testing.T) {
	cerrada := func(t *testing.T) *DB {
		t.Helper()
		return populatedThenClosed(t)
	}

	t.Run("MoveTask", func(t *testing.T) {
		if _, err := cerrada(t).MoveTask(1, "doing"); err == nil {
			t.Error("MoveTask: want error")
		}
	})
	t.Run("StartTask", func(t *testing.T) {
		if _, err := cerrada(t).StartTask(1); err == nil {
			t.Error("StartTask: want error")
		}
	})
	t.Run("ReviewTask", func(t *testing.T) {
		if _, err := cerrada(t).ReviewTask(1); err == nil {
			t.Error("ReviewTask: want error")
		}
	})
	t.Run("DoneTask", func(t *testing.T) {
		if _, err := cerrada(t).DoneTask(1); err == nil {
			t.Error("DoneTask: want error")
		}
	})
	t.Run("CancelTask", func(t *testing.T) {
		if _, err := cerrada(t).CancelTask(1); err == nil {
			t.Error("CancelTask: want error")
		}
	})
	t.Run("UpdateTask", func(t *testing.T) {
		if _, err := cerrada(t).UpdateTask(1, map[string]any{"title": "otro"}); err == nil {
			t.Error("UpdateTask: want error")
		}
	})
	t.Run("SetTaskTags", func(t *testing.T) {
		if _, err := cerrada(t).SetTaskTags(1, []string{"x"}); err == nil {
			t.Error("SetTaskTags: want error")
		}
	})
	t.Run("AddTaskTags", func(t *testing.T) {
		if _, err := cerrada(t).AddTaskTags(1, []string{"x"}); err == nil {
			t.Error("AddTaskTags: want error")
		}
	})
	t.Run("RemoveTaskTags", func(t *testing.T) {
		if _, err := cerrada(t).RemoveTaskTags(1, []string{"x"}); err == nil {
			t.Error("RemoveTaskTags: want error")
		}
	})
}

// UpdateProject lee el proyecto antes de construir el UPDATE, así que la
// escritura sólo se intenta cuando la lectura ha funcionado. Con la base
// poblada y cerrada la lectura falla antes; lo que se cubre aquí es la validación
// de workflow y de list_order, que ocurre entre medias y no toca la base.
func TestUpdateProjectValidatesBeforeWriting(t *testing.T) {
	t.Run("workflow inválido", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		err := database.UpdateProject("api", map[string]any{"workflow": []string{"nope"}})
		if err == nil {
			t.Fatal("UpdateProject con un workflow inválido: want error")
		}
	})

	t.Run("list_order inválido", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		err := database.UpdateProject("api", map[string]any{"list_order": []string{"nope"}})
		if err == nil {
			t.Fatal("UpdateProject con un list_order inválido: want error")
		}
	})

	t.Run("list_order inválido contra un workflow nuevo", func(t *testing.T) {
		// Workflow y list_order en la misma llamada: el list_order se valida
		// contra el workflow que acaba de llegar, no contra el de antes.
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		err := database.UpdateProject("api", map[string]any{
			"workflow":   []string{"backlog", "done", "cancelled"},
			"list_order": []string{"done", "un-estado-que-no-existe"},
		})
		if err == nil {
			t.Fatal("UpdateProject con un list_order que no encaja en el workflow nuevo: want error")
		}
	})

	t.Run("sin cambios", func(t *testing.T) {
		// Un mapa de updates vacío no toca nada: ni una consulta, ni un
		// updated_at. Es la rama corta que evita un UPDATE inútil.
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		if err := database.UpdateProject("api", map[string]any{}); err != nil {
			t.Errorf("UpdateProject sin cambios: %v", err)
		}
		if err := database.UpdateProject("api", map[string]any{"force": true}); err != nil {
			t.Errorf("UpdateProject sólo con force: %v", err)
		}
	})
}

// GetProjectByID distingue "no existe" de "falló la consulta", y son dos
// mensajes distintos porque el primero le dice al usuario que el nombre está mal
// escrito y el segundo que algo se rompió.
func TestGetProjectByIDReportsMissingSeparately(t *testing.T) {
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)

	_, err := database.GetProjectByID(9999)
	if err == nil {
		t.Fatal("GetProjectByID con un id inexistente: want error")
	}
	// Y con la base cerrada el mensaje es el de la conexión, no "not found".
	cerrada := populatedThenClosed(t)
	if _, err := cerrada.GetProjectByID(1); err == nil {
		t.Fatal("GetProjectByID sin conexión: want error")
	}
}

// Escribir directamente en la tabla es la única forma de que un Scan falle:
// los tipos los fija el código, así que sólo una fila corrupta los desmiente.
// Es un test de caja blanca sobre el esquema, y por eso usa Conn().
func TestScanFailsOnACorruptRow(t *testing.T) {
	t.Run("tarea con prioridad no numérica", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateTask(t, database, "api", "tarea", "", "", 2, "backlog")

		// La columna es INTEGER y SQLite es dinámicamente tipado: un texto cabe
		// dentro sin que ninguna restricción lo impida.
		if _, err := database.Conn().Exec(`UPDATE tasks SET priority = 'alta'`); err != nil {
			t.Fatalf("corrompiendo la fila: %v", err)
		}

		if _, err := database.ListTasks("", "", ""); err == nil {
			t.Error("ListTasks con una prioridad corrupta: want error")
		}
		if _, err := database.GetTask(1); err == nil {
			t.Error("GetTask con una prioridad corrupta: want error")
		}
	})

	t.Run("proyecto con workflow que no es JSON", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		if _, err := database.Conn().Exec(`UPDATE projects SET workflow = 'no soy json'`); err != nil {
			t.Fatalf("corrompiendo la fila: %v", err)
		}

		if _, err := database.GetProject("api"); err == nil {
			t.Error("GetProject con un workflow corrupto: want error")
		}
		if _, err := database.GetProjectByID(1); err == nil {
			t.Error("GetProjectByID con un workflow corrupto: want error")
		}
		if _, err := database.ListProjects(); err == nil {
			t.Error("ListProjects con un workflow corrupto: want error")
		}
	})

	t.Run("proyecto con list_order que no es JSON", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)

		if _, err := database.Conn().Exec(`UPDATE projects SET list_order = '{'`); err != nil {
			t.Fatalf("corrompiendo la fila: %v", err)
		}

		if _, err := database.GetProject("api"); err == nil {
			t.Error("GetProject con un list_order corrupto: want error")
		}
	})

	t.Run("tarea con tags que no es JSON", func(t *testing.T) {
		database := newTestDB(t)
		mustCreateProject(t, database, "api", nil)
		mustCreateTask(t, database, "api", "tarea", "", "", 2, "backlog")

		if _, err := database.Conn().Exec(`UPDATE tasks SET tags = '['`); err != nil {
			t.Fatalf("corrompiendo la fila: %v", err)
		}

		// Un tags ilegible no es un error de Scan sino de deserializar, y la
		// lista tiene que seguir saliendo: perder todas las tareas por una fila
		// con tags raros sería peor que perder los tags.
		tasks, err := database.ListTasks("", "", "")
		if err != nil {
			t.Fatalf("ListTasks con tags corruptos: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("salen %d tareas, want 1", len(tasks))
		}
		if len(tasks[0].Tags) != 0 {
			t.Errorf("los tags corruptos han producido %v, want ninguno", tasks[0].Tags)
		}
	})
}

// Stats cuenta por estado y por responsable en dos consultas separadas. Con la
// base poblada y cerrada falla en la primera; para llegar a la segunda hace
// falta que la primera funcione, así que se usa una stats sobre una base viva
// con datos en varios estados.
func TestStatsCountsEveryStateAndAssignee(t *testing.T) {
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "uno", "", "@juan", 2, "backlog")
	mustCreateTask(t, database, "api", "dos", "", "@juan", 2, "done")
	mustCreateTask(t, database, "api", "tres", "", "@maria", 2, "done")

	stats, err := database.Stats("")
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	byStatus, ok := stats["by_status"].(map[string]int)
	if !ok {
		t.Fatalf("by_status es %T, want map[string]int", stats["by_status"])
	}
	if byStatus["backlog"] != 1 || byStatus["done"] != 2 {
		t.Errorf("by_status = %v, want backlog 1 y done 2", byStatus)
	}

	byAssignee, ok := stats["by_assignee"].(map[string]int)
	if !ok {
		t.Fatalf("by_assignee es %T, want map[string]int", stats["by_assignee"])
	}
	if byAssignee["@juan"] != 2 || byAssignee["@maria"] != 1 {
		t.Errorf("by_assignee = %v, want @juan 2 y @maria 1", byAssignee)
	}

	t.Run("con la base cerrada", func(t *testing.T) {
		if _, err := populatedThenClosed(t).Stats(""); err == nil {
			t.Error("Stats sin conexión: want error")
		}
	})
}

// ProjectTaskCount cuenta con COUNT(*) sobre una columna NOT NULL, así que la
// única forma de que falle es la conexión.
func TestProjectTaskCountFailsOnAClosedDB(t *testing.T) {
	if _, err := populatedThenClosed(t).ProjectTaskCount(1); err == nil {
		t.Error("ProjectTaskCount sin conexión: want error")
	}
}

// AddOffDay valida las fechas antes de escribir, y esos mensajes son lo que ve
// el usuario cuando escribe mal un off-day.
func TestAddOffDayRejectsBadDates(t *testing.T) {
	database := newTestDB(t)

	if _, err := database.AddOffDay("@juan", "", "2026-03-02", ""); err == nil {
		t.Error("AddOffDay sin fecha de inicio: want error")
	}
	if _, err := database.AddOffDay("@juan", "2026-03-02", "no-es-fecha", ""); err == nil {
		t.Error("AddOffDay con una fecha de fin inválida: want error")
	}
}

// El estado terminal es el que no se puede quitar del workflow, y el filtro que
// lo protege tiene su propia ruta dentro de UpdateProject.
func TestUpdateProjectKeepsTheTerminalStatus(t *testing.T) {
	database := newTestDB(t)
	mustCreateProject(t, database, "api", nil)
	mustCreateTask(t, database, "api", "tarea", "", "", 2, model.DoneStatus)

	// Quitar "done" está prohibido aunque no haya tareas en él, y también las
	// hay: las dos comprobaciones existen por separado.
	if err := database.UpdateProject("api", map[string]any{
		"workflow": []string{"backlog", "doing", "cancelled"},
	}); err == nil {
		t.Error("quitar el estado terminal del workflow: want error")
	}
}

// GetTask y GetProjectByID leen cosas distintas de la misma fila del proyecto:
// GetTask sólo quiere el nombre, y GetProjectByID además deserializa el
// workflow. Rompiendo el workflow -- y sólo el workflow -- el primero pasa y el
// segundo falla, que es la única forma de alcanzar el error intermedio de
// StartTask, ReviewTask y DoneTask.
func TestStatusChangesFailWhenTheProjectWorkflowIsUnreadable(t *testing.T) {
	for _, tc := range []struct {
		nombre string
		llamar func(*DB) error
	}{
		{"StartTask", func(d *DB) error { _, err := d.StartTask(1); return err }},
		{"ReviewTask", func(d *DB) error { _, err := d.ReviewTask(1); return err }},
		{"DoneTask", func(d *DB) error { _, err := d.DoneTask(1); return err }},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			database := newTestDB(t)
			poblarAPI(t, database)
			if _, err := database.Conn().Exec(`UPDATE projects SET workflow = 'no soy json'`); err != nil {
				t.Fatalf("corrompiendo la fila: %v", err)
			}

			// GetTask sigue funcionando: sólo lee el nombre.
			if _, err := database.GetTask(1); err != nil {
				t.Fatalf("GetTask también falla, el test no mide lo que dice: %v", err)
			}
			if err := tc.llamar(database); err == nil {
				t.Error("want error")
			}
		})
	}
}

// Las columnas tienen afinidad de tipo, así que SQLite no deja meter un valor
// incompatible: un entero en una TEXT se convierte a texto y un texto en una
// INTEGER se rechaza en el UPDATE. La única forma de que un Scan reciba un tipo
// que no sabe convertir es replacing la tabla por una vista con los tipos
// cambiados, que es una vista de caja blanca sobre el esquema.
func TestScanFailsWhenTheTableIsReplacedByAWronglyTypedView(t *testing.T) {
	t.Run("comentarios", func(t *testing.T) {
		database := newTestDB(t)
		poblarAPI(t, database)
		mustAddComment(t, database, 1, "hola")

		if _, err := database.Conn().Exec(`DROP TABLE comments;
			CREATE VIEW comments AS SELECT x'00ff' AS id, 1 AS task_id, 'x' AS body, 'y' AS created_at`); err != nil {
			t.Fatalf("sustituyendo la tabla por una vista: %v", err)
		}

		if _, err := database.ListComments(1); err == nil {
			t.Error("ListComments con un id que no es un entero: want error")
		}
	})

	t.Run("off-days", func(t *testing.T) {
		database := newTestDB(t)
		if _, err := database.AddOffDay("@juan", "2026-03-01", "2026-03-02", ""); err != nil {
			t.Fatalf("AddOffDay: %v", err)
		}

		if _, err := database.Conn().Exec(`DROP TABLE offdays;
			CREATE VIEW offdays AS SELECT x'00ff' AS id, 'j' AS assignee, 'd' AS start_date, 'd' AS end_date, '' AS note`); err != nil {
			t.Fatalf("sustituyendo la tabla por una vista: %v", err)
		}

		if _, err := database.ListOffDays(""); err == nil {
			t.Error("ListOffDays con un id que no es un entero: want error")
		}
	})

	t.Run("stats sin tabla de tareas", func(t *testing.T) {
		// Las tres consultas de Stats cuentan sobre tasks; sin la tabla, la
		// primera -- el total -- ya falla.
		database := newTestDB(t)
		poblarAPI(t, database)
		if _, err := database.Conn().Exec(`DROP TABLE tasks`); err != nil {
			t.Fatalf("borrando tasks: %v", err)
		}

		if _, err := database.Stats(""); err == nil {
			t.Error("Stats sin la tabla de tareas: want error")
		}
	})
}
