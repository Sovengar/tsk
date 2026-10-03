package db

import (
	"strings"
	"testing"
)

// Stats hace tres consultas contra la misma vista: un COUNT, un GROUP BY status
// y un GROUP BY assignee. Con la base sana las tres funcionan, así que sus tres
// ramas de error sólo se ven si se estropea el ESQUEMA, no los datos.
//
// La herramienta es sustituir la tabla tasks por una VISTA. Una vista es SQL,
// no datos, y eso abre dos sabotajes distintos que hay que no confundir, porque
// cada uno falla en un punto distinto de Stats:
//
//   - Una columna QUE NO EXISTE. El COUNT no la menciona -- cuenta filas, y da
//     lo mismo)-- así que pasa; el GROUP BY sí la nombra y falla al_PREPARAR la
//     consulta. Es un fallo de db.conn.Query: todavía no ha salido ninguna fila.
//
//   - Una columna con un valor que no se puede convertir (un NULL). La consulta
//     se prepara, se ejecuta, la fila llega... y es el SCAN el que no sabe qué
//     hacer. database/sql no tiene NULL→string: no hay con qué llenarlo.
//
// Confundirlos daría un test que pasa por la razón equivocada, que es peor que
// no tener test: el día que la consulta cambie, el test seguirá verde por un
// motivo que ya no existe.

// vistaDeTasks cambia el esquema de tasks por la vista dada. Se llama con el
// cuerpo del SELECT de la vista, y la vista tiene que traer las doce columnas
// que el resto del código espera -- no es una vista de caja negra sobre el
// esquema, es el mismo esquema con una columna cambiada.
func vistaDeTasks(t *testing.T, selectDeLaVista string) *DB {
	t.Helper()
	database := newTestDB(t)
	poblarAPI(t, database)

	if _, err := database.Conn().Exec(`ALTER TABLE tasks RENAME TO tasks_real`); err != nil {
		t.Fatalf("renombrando tasks: %v", err)
	}
	if _, err := database.Conn().Exec(`CREATE VIEW tasks AS ` + selectDeLaVista + ` FROM tasks_real t0`); err != nil {
		t.Fatalf("creando la vista: %v", err)
	}
	return database
}

// sabotearColumna cambia una columna de la vista por la expresión dada, bajo el
// MISMO nombre. Es el sabotaje del Scan: la consulta funciona, la fila llega, y
// lo que no se puede convertir es el valor.
//
// Para quitar una columna del todo --el sabotaje de la consulta-- hay que pasar
// un nombre nuevo, y por eso es un helper aparte.
func sabotearColumna(t *testing.T, columna, expresion string) *DB {
	t.Helper()
	colStatus, colAssignee := "t0.status", "t0.assignee"
	switch columna {
	case "status":
		colStatus = expresion
	case "assignee":
		colAssignee = expresion
	default:
		t.Fatalf("columna desconocida: %q", columna)
	}

	cols := "t0.id, t0.project_id, t0.title, t0.description, " + colStatus + " AS status," +
		" t0.priority, " + colAssignee + " AS assignee," +
		" t0.created_at, t0.updated_at, t0.completed_at, t0.estimate, t0.tags"
	return vistaDeTasks(t, "SELECT "+cols)
}

// quitarColumna deja la columna de la vista sin el nombre que el código
// consulta. El COUNT la ignora porque no la nombra; el GROUP BY la nombra y no
// se puede preparar.
func quitarColumna(t *testing.T, columna string) *DB {
	t.Helper()
	// El truco es el ALIAS, no el valor: la columna sigue ahí con su contenido,
	// pero la vista ya no la llama status, así que "GROUP BY t.status" no
	// encuentra a quién agrupar.
	colStatus, colAssignee := "t0.status AS status", "t0.assignee AS assignee"
	switch columna {
	case "status":
		colStatus = `t0.status AS estado`
	case "assignee":
		colAssignee = `t0.assignee AS responsable`
	default:
		t.Fatalf("columna desconocida: %q", columna)
	}

	cols := "t0.id, t0.project_id, t0.title, t0.description, " + colStatus + "," +
		" t0.priority, " + colAssignee + "," +
		" t0.created_at, t0.updated_at, t0.completed_at, t0.estimate, t0.tags"
	return vistaDeTasks(t, "SELECT "+cols)
}

// Los dos fallos de la CONSULTA. El sabotaje es real y no un truco del arnés:
// sin él, Stats devuelve el mapa entero. Y cada caso comprueba el mensaje, que
// es lo que distingue "la consulta no se pudo preparar" de cualquier otro fallo
// por el que Stats pueda devolver error.
func TestStatsFailsWhenTheGroupingColumnDoesNotExist(t *testing.T) {
	t.Run("la consulta de estado", func(t *testing.T) {
		database := quitarColumna(t, "status")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats sin columna status: want error")
		}
		// El nombre en el mensaje dice QUÉ columna faltó, y dice que es la de
		// estado: si el sabotaje se hubiera colado en la de responsable, el
		// mensaje sería otro y el test no mediría lo que dice medir.
		if !strings.Contains(err.Error(), "t.status") {
			t.Errorf("el fallo no es la columna de estado: %v", err)
		}
	})

	t.Run("la consulta por responsable", func(t *testing.T) {
		database := quitarColumna(t, "assignee")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats sin columna assignee: want error")
		}
		if !strings.Contains(err.Error(), "t.assignee") {
			t.Errorf("el fallo no es la columna de responsable: %v", err)
		}
	})
}

// Los dos fallos del SCAN, que son distintos: la consulta funcionó y lo que no
// se puede convertir es el valor.
func TestStatsFailsWhenTheGroupingColumnIsNull(t *testing.T) {
	t.Run("el estado llega como NULL", func(t *testing.T) {
		database := sabotearColumna(t, "status", "NULL")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats con un estado NULL: want error")
		}
		// "Scan error on column index 0" y no un fallo de consulta: el índice 0
		// es el estado, la primera columna del GROUP BY, y es la que se saboteó.
		if !strings.Contains(err.Error(), "Scan error on column index 0") {
			t.Errorf("el fallo no es el Scan del estado: %v", err)
		}
	})

	t.Run("el responsable llega como NULL", func(t *testing.T) {
		database := sabotearColumna(t, "assignee", "NULL")

		_, err := database.Stats("")
		if err == nil {
			t.Fatal("Stats con un responsable NULL: want error")
		}
		// Sigue siendo la columna 0 porque esta consulta devuelve el
		// responsable primero.
		if !strings.Contains(err.Error(), "Scan error on column index 0") {
			t.Errorf("el fallo no es el Scan del responsable: %v", err)
		}
	})
}

// UnStats que falla a la mitad tiene que fallar entero: si devolviera lo que
// llevaba, el comando mostraría un total sin desglose y nadie notaría que falta
// una columna.
func TestStatsDevuelveNadaCuandoFalla(t *testing.T) {
	database := sabotearColumna(t, "assignee", "NULL")

	res, err := database.Stats("")
	if err == nil {
		t.Fatal("Stats con un responsable NULL: want error")
	}
	if res != nil {
		t.Errorf("Stats ha devuelto %v además del error: un error aquí significa que el mapa es inventado", res)
	}
}

// MoveTask lee la tarea y luego su proyecto. La tarea se lee con un JOIN que
// sólo trae p.name, así que sobrevive a un proyecto con el workflow corrupto;
// la segunda lectura, que trae las ocho columnas, no.
func TestMoveTaskFailsWhenTheProjectCannotBeRead(t *testing.T) {
	database := newTestDB(t)
	poblarAPI(t, database)

	// workflow es JSON y se parsea al escanear: un workflow inválido rompe
	// scanProject sin tocar el resto de la fila.
	if _, err := database.Conn().Exec(`UPDATE projects SET workflow = 'no soy json'`); err != nil {
		t.Fatalf("corrompiendo el workflow: %v", err)
	}

	// La lectura por clave de la tarea sí funciona, o el test no mediría lo que
	// dice: si fallara ahí, el error vendría de GetTask y no de GetProjectByID.
	if _, err := database.GetTask(1); err != nil {
		t.Fatalf("GetTask también falla, el test no mide lo que dice: %v", err)
	}

	if _, err := database.MoveTask(1, "done"); err == nil {
		t.Error("MoveTask con un proyecto ilegible: want error")
	}

	// Y la tarea no se ha movido: el fallo es al leer, antes de escribir.
	var status string
	if err := database.Conn().QueryRow(`SELECT status FROM tasks WHERE id = 1`).Scan(&status); err != nil {
		t.Fatalf("leyendo el estado: %v", err)
	}
	if status == "done" {
		t.Error("la tarea se movió pese a que su proyecto no se pudo leer")
	}
}

// La regla del sabotaje es que la base tiene que seguir siendo una base: si el
// esquema no cuela, la conclusión es que el test mide una bbdd rota y no un
// camino de error del código.
func TestSabotearColumnaDejaUnaBaseUsable(t *testing.T) {
	database := sabotearColumna(t, "status", "NULL")

	var count int
	if err := database.Conn().QueryRow(`SELECT COUNT(*) FROM tasks_real`).Scan(&count); err != nil {
		t.Fatalf("la tabla original ha desaparecido: %v", err)
	}
	if count == 0 {
		t.Error("el sabotage ha vaciado la base, así que no mide un fallo de Stats sino una base sin datos")
	}
	// Y el resto de la API sigue respondiendo: sólo se ha roto la lectura de
	// status, no la base entera.
	if _, err := database.GetProject("api"); err != nil {
		t.Errorf("el sabotage ha roto más de lo que dice: %v", err)
	}
}
