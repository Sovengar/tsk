package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tsk/internal/model"
)

// El editor externo es la única parte del programa que depende de otro binario,
// y todo lo que hace con él está detrás de tea.ExecProcess: un Cmd que devuelve
// un execMsg que sólo el bucle de Bubbletea sabe ejecutar. Lo que se puede
// comprobar sin terminal es la mitad que decide qué hacer con lo que el editor
// dejó, y esa mitad es readEditedFile.

func TestReadEditedFile(t *testing.T) {
	t.Run("el editor falló", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "borrado.md")
		if err := os.WriteFile(path, []byte("contenido"), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := readEditedFile(path, errAccionFallida)
		if !errors.Is(err, errAccionFallida) {
			t.Errorf("el error es %v, want el del proceso", err)
		}
		if got != "" {
			t.Errorf("el contenido es %q, want vacío con el editor fallido", got)
		}
		// El temporal se borra también al fallar: si no, cada edición fallida
		// dejaría un fichero en /tmp.
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("el temporal no se ha borrado tras un editor fallido")
		}
	})

	t.Run("el fichero ya no está", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nunca-existio.md")

		if _, err := readEditedFile(path, nil); err == nil {
			t.Error("leer un temporal inexistente no ha fallado")
		}
	})

	t.Run("el editor no escribió nada", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vacio.md")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := readEditedFile(path, nil)
		if err != nil {
			t.Fatalf("readEditedFile: %v", err)
		}
		if got != "" {
			t.Errorf("el contenido es %q, want vacío", got)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("el temporal no se ha borrado")
		}
	})

	t.Run("el editor escribió algo", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "lleno.md")
		if err := os.WriteFile(path, []byte("hola\n\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		// El recorte no es cosa de aquí: quien decide si el comentario va
		// recortado es commentCmd, y el editor externo conserva los saltos.
		got, err := readEditedFile(path, nil)
		if err != nil {
			t.Fatalf("readEditedFile: %v", err)
		}
		if got != "hola\n\n" {
			t.Errorf("el contenido es %q, want %q sin recortar", got, "hola\n\n")
		}
	})
}

// Si el directorio de temporales no existe, no hay fichero que editar: el
// comando devuelve el error con el id de la tarea en vez de romperse. Es lo que
// pasa con un TMPDIR mal configurado, y antes de estos tests nadie lo había
// visto pasar.
func TestExternalEditorWithoutATempDir(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-existe"))

	t.Run("comentario", func(t *testing.T) {
		msg := mustMsg(t, commentCmd(7, "nvim"))
		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("sin directorio de temporales no ha habido error")
		}
		if finished.taskID != 7 {
			t.Errorf("el mensaje lleva taskID %d, want 7 para que la UI sepa de qué tarea es", finished.taskID)
		}
	})

	t.Run("editor de tarea", func(t *testing.T) {
		msg := mustMsg(t, editTaskCmd(model.Task{ID: 9, Title: "t"}, "nvim"))
		finished, ok := msg.(editorFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want editorFinishedMsg", msg)
		}
		if finished.err == nil {
			t.Error("sin directorio de temporales no ha habido error")
		}
		if finished.taskID != 9 {
			t.Errorf("el mensaje lleva taskID %d, want 9", finished.taskID)
		}
	})
}

// Con la base cerrada, las tres operaciones de comentarios fallan y el mensaje
// que sale es el de "recargado sin comentarios", no un panic. Lo importante es
// que el detalle se queda usable y no con un índice de selección imposible.
func TestCommentCommandsWithAClosedDB(t *testing.T) {
	m := newDetailModel(t, 2)
	taskID := m.detailTask.ID
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	t.Run("cargar", func(t *testing.T) {
		msg := mustMsg(t, m.loadCommentsCmd(taskID))
		loaded, ok := msg.(commentsLoadedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentsLoadedMsg", msg)
		}
		if len(loaded.comments) != 0 || loaded.selectIdx != -1 {
			t.Errorf("con la base cerrada han salido %d comentarios y sel=%d, want 0 y -1",
				len(loaded.comments), loaded.selectIdx)
		}
	})

	t.Run("añadir", func(t *testing.T) {
		msg := mustMsg(t, m.addCommentCmd(taskID, "nuevo"))
		loaded := msg.(commentsLoadedMsg)
		if len(loaded.comments) != 0 || loaded.selectIdx != -1 {
			t.Errorf("con la base cerrada han salido %d comentarios y sel=%d, want 0 y -1",
				len(loaded.comments), loaded.selectIdx)
		}
	})

	t.Run("borrar", func(t *testing.T) {
		msg := mustMsg(t, m.deleteCommentCmd(taskID, 1, 2))
		loaded := msg.(commentsLoadedMsg)
		if len(loaded.comments) != 0 {
			t.Errorf("con la base cerrada han salido %d comentarios, want 0", len(loaded.comments))
		}
		// El borrado fallido conserva la selección para que el usuario vea qué
		//评论 sigue ahí; el éxito la deja en -1.
		if loaded.selectIdx != 2 {
			t.Errorf("selectIdx = %d, want 2 (mantener la selección al fallar el borrado)",
				loaded.selectIdx)
		}
	})
}

// El guardado del editor externo reescribe cinco campos de golpe. Si la base
// falla a mitad, el modelo tiene que quedarse como estaba en vez de mostrar una
// tarea a medio guardar.
func TestUpdateTaskFromEditWithAClosedDB(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	if err := m.database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if msg := m.updateTaskFromEdit(id, "Title: otro\nStatus: todo\n")(); msg != nil {
		t.Errorf("un guardado fallido ha devuelto %T, want nil", msg)
	}
}

// Ni la tarea seleccionada ni el proyecto actual existen cuando el filtro no
// deja nada: abrir el editor o el alta entonces no puede hacer nada.
func TestExternalEditorNeedsASelectedTask(t *testing.T) {
	m := newTestModel(t)
	m.filteredT = nil
	m.tasks = nil

	if cmd := m.editSelectedTask(); cmd != nil {
		t.Error("sin tarea seleccionada el editor externo se ha lanzado igualmente")
	}
}

func TestNewTaskNeedsACurrentProject(t *testing.T) {
	// El filtro de proyecto sólo cuenta en lista y kanban; en el dashboard, sin
	// proyectos, no hay proyecto actual y el alta no puede abrirse.
	m := newTestModel(t)
	m.currentView = viewDashboard
	m.filterProject = ""
	m.projects = nil

	if cmd := m.newTask(); cmd != nil {
		t.Error("sin proyecto actual el alta se ha lanzado igualmente")
	}
	if m.newTaskOpen {
		t.Error("el alta se ha abierto sin proyecto")
	}
}

// El comando del editor externo es lo que se indexa al componer el exec.Command,
// así que un comando vacío haría panear. La configuración por defecto trae
// "nvim"; una config con el campo vacío es posible y es el caso que esto cubre.
func TestEditorCommandFallsBackWhenUnset(t *testing.T) {
	if got := editorCommand(""); got != "nvim" {
		t.Errorf("editorCommand(%q) = %q, want nvim", "", got)
	}
	if got := editorCommand("hx"); got != "hx" {
		t.Errorf("editorCommand(%q) = %q, want el comando de la config", "hx", got)
	}
}

// Un editor externo que devuelve un fichero ilegible no deja la tarea en un
// estado intermedio: se descarta el contenido y el modelo sigue como estaba.
func TestEditorFinishedWithUnparseableContent(t *testing.T) {
	m := newTestModel(t)
	id := m.tasks[0].ID
	antes := m.tasks[0].Title

	siguiente, _ := updateMsg(t, m, editorFinishedMsg{taskID: id, file: "esto no es el formato"})
	tarea := taskByTitle(t, siguiente, antes)
	if tarea == nil {
		t.Fatalf("la tarea %q ha desaparecido tras un editor con basura", antes)
	}
	if tarea.Title != antes {
		t.Errorf("el título es %q, want %q", tarea.Title, antes)
	}
}
