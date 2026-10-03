package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// errFalso es el fallo que inyectan los dobles: uno que ningún disco da.
var errFalso = errors.New("fallo inyectado")

// archivoQueFalla es un temporal falso. Sus dos fallos --escritura y cierre--
// son los que un tempfile de verdad no produce nunca en una máquina que
// funciona: harían falta un disco lleno o cerrar dos veces el mismo fichero.
// El doble existe para eso, y para comprobar que el fichero se borra igual, que
// es la parte que de verdad importa: un temporal a medias en /tmp no se limpia
// solo.
type archivoQueFalla struct {
	nombre     string
	escrito    []byte
	errWrite   error
	errClose   error
	cerrado    int
	escrituras int
}

func (a *archivoQueFalla) Write(p []byte) (int, error) {
	a.escrituras++
	if a.errWrite != nil {
		return 0, a.errWrite
	}
	a.escrito = append(a.escrito, p...)
	return len(p), nil
}

func (a *archivoQueFalla) Close() error {
	a.cerrado++
	return a.errClose
}

func (a *archivoQueFalla) Name() string { return a.nombre }

// nombreVivo devuelve una ruta real dentro de t.TempDir para que el os.Remove
// del código tenga algo que borrar de verdad.
func nombreVivo(t *testing.T) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "tsk-test.md")
	if err := os.WriteFile(ruta, []byte("contenido viejo"), 0o600); err != nil {
		t.Fatalf("preparando el temporal: %v", err)
	}
	return ruta
}

func TestEscribirYCerrarDejaElTemporalListo(t *testing.T) {
	ruta := nombreVivo(t)
	f := &archivoQueFalla{nombre: ruta}

	if err := escribirYcerrar(f, "nuevo contenido"); err != nil {
		t.Fatalf("escribirYcerrar: %v", err)
	}
	if string(f.escrito) != "nuevo contenido" {
		t.Errorf("ha escrito %q, want %q", f.escrito, "nuevo contenido")
	}
	if f.cerrado != 1 {
		t.Errorf("ha cerrado %d veces, want 1", f.cerrado)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Errorf("un temporal que se escribió bien no debe borrarse: %v", err)
	}
}

// El comentario no lleva plantilla: pasa la cadena vacía y aun así escribe, que
// es lo que lo distingue de "no he escrito nada" cuando el editor lee el fichero.
func TestEscribirYCerrarAceptaContenidoVacio(t *testing.T) {
	f := &archivoQueFalla{nombre: nombreVivo(t)}

	if err := escribirYcerrar(f, ""); err != nil {
		t.Fatalf("escribirYcerrar con contenido vacío: %v", err)
	}
	if f.escrituras != 1 {
		t.Errorf("ha escrito %d veces, want 1: el temporal tiene que existir aunque sea vacío", f.escrituras)
	}
	if f.cerrado != 1 {
		t.Errorf("ha cerrado %d veces, want 1", f.cerrado)
	}
}

// Los dos fallos. En los dos casos el fichero se borra: es la mitad del motivo
// por el que existen, porque un temporal a medias se queda en /tmp para siempre.
func TestEscribirYCerrarBorraElTemporalSiFalla(t *testing.T) {
	t.Run("falla al escribir", func(t *testing.T) {
		ruta := nombreVivo(t)
		f := &archivoQueFalla{nombre: ruta, errWrite: errFalso}

		err := escribirYcerrar(f, "nuevo contenido")
		if !errors.Is(err, errFalso) {
			t.Fatalf("error = %v, want %v", err, errFalso)
		}
		if f.escrito != nil {
			t.Errorf("ha escrito algo pese a fallar la escritura: %q", f.escrito)
		}
		if f.cerrado != 1 {
			t.Errorf("ha cerrado %d veces, want 1: un fichero abierto se cierra aunque la escritura falle", f.cerrado)
		}
		if _, err := os.Stat(ruta); !os.IsNotExist(err) {
			t.Errorf("el temporal sigue ahí tras fallar la escritura: %v", err)
		}
	})

	t.Run("falla al cerrar", func(t *testing.T) {
		ruta := nombreVivo(t)
		f := &archivoQueFalla{nombre: ruta, errClose: errFalso}

		err := escribirYcerrar(f, "nuevo contenido")
		if !errors.Is(err, errFalso) {
			t.Fatalf("error = %v, want %v", err, errFalso)
		}
		// El contenido sí llegó a escribirse: el fallo es posterior, y el editor
		// no se lanza, así que da igual lo que contenga.
		if string(f.escrito) != "nuevo contenido" {
			t.Errorf("ha escrito %q, want %q", f.escrito, "nuevo contenido")
		}
		if _, err := os.Stat(ruta); !os.IsNotExist(err) {
			t.Errorf("el temporal sigue ahí tras fallar el cierre: %v", err)
		}
	})
}

// Si el nombre no está en disco, os.Remove falla y se ignora. No es una ruta
// real de producción -- el temporal lo acaba de crear os.CreateTemp -- pero
// documenta que un fallo al borrar no tapa el error que sí importa.
func TestEscribirYCerrarNoTapaElErrorSiNoPuedeBorrar(t *testing.T) {
	f := &archivoQueFalla{
		nombre:   filepath.Join(t.TempDir(), "nunca-existio.md"),
		errWrite: errFalso,
	}

	if err := escribirYcerrar(f, "x"); !errors.Is(err, errFalso) {
		t.Errorf("error = %v, want el de escritura y no el del borrado", err)
	}
}

// Los dos llamadores de escribirYcerrar. Sin la indirección de crearTemporal
// sus ramas de error no se podrían probar: un temporal de verdad no falla al
// escribirse en una máquina que funciona, así que nunca se llegaría a ese if.
//
// Y lo que se comprueba es lo que de verdad importa para la persona que está
// delante: el error sale con el id de la tarea, para que la TUI sepa a qué tarea
// pertenece el fallo y no lo pierda.
func TestLosDosEditoresDanElIdDeLaTareaCuandoElTemporalFalla(t *testing.T) {
	t.Run("editor de tarea", func(t *testing.T) {
		restaurar := sustituirTemporal(t, func(string, string) (archivoTemporal, error) {
			return &archivoQueFalla{nombre: nombreVivo(t), errWrite: errFalso}, nil
		})
		defer restaurar()

		msg := editTaskCmd(model.Task{ID: 42, Title: "con título"}, "vi /tmp/x")()
		finished, ok := msg.(editorFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want editorFinishedMsg", msg)
		}
		if !errors.Is(finished.err, errFalso) {
			t.Errorf("err = %v, want el inyectado", finished.err)
		}
		if finished.taskID != 42 {
			t.Errorf("taskID = %d, want 42: sin él la TUI no sabe a qué tarea volver", finished.taskID)
		}
	})

	t.Run("editor de comentario", func(t *testing.T) {
		restaurar := sustituirTemporal(t, func(string, string) (archivoTemporal, error) {
			return &archivoQueFalla{nombre: nombreVivo(t), errClose: errFalso}, nil
		})
		defer restaurar()

		msg := commentCmd(7, "vi /tmp/x")()
		finished, ok := msg.(commentFinishedMsg)
		if !ok {
			t.Fatalf("el mensaje es %T, want commentFinishedMsg", msg)
		}
		if !errors.Is(finished.err, errFalso) {
			t.Errorf("err = %v, want el inyectado", finished.err)
		}
		if finished.taskID != 7 {
			t.Errorf("taskID = %d, want 7", finished.taskID)
		}
	})
}

// El fallo de la creación del temporal es el otro camino, y es el único que se
// probaba antes: TMPDIR apunta a un directorio que no existe.
func TestLosDosEditoresDanElIdCuandoNoSePuedeCrearElTemporal(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "no-existe"))

	for _, tc := range []struct {
		nombre string
		llamar func() tea.Msg
		taskID func(tea.Msg) (int64, error)
	}{
		{"tarea", func() tea.Msg { return editTaskCmd(model.Task{ID: 5}, "vi /tmp/x")() },
			func(msg tea.Msg) (int64, error) {
				finished := msg.(editorFinishedMsg)
				return finished.taskID, finished.err
			}},
		{"comentario", func() tea.Msg { return commentCmd(9, "vi /tmp/x")() },
			func(msg tea.Msg) (int64, error) {
				finished := msg.(commentFinishedMsg)
				return finished.taskID, finished.err
			}},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			taskID, err := tc.taskID(tc.llamar())
			if err == nil {
				t.Fatal("want error: TMPDIR no existe")
			}
			if taskID == 0 {
				t.Error("el error ha salido sin id de tarea")
			}
		})
	}
}

// sustituirTemporal cambia crearTemporal durante el test y devuelve la función
// que lo deja como estaba. Los tests corre en paralelo no podrían usar esto, y
// por eso ninguno lo hace: es un global.
func sustituirTemporal(t *testing.T, f func(string, string) (archivoTemporal, error)) func() {
	t.Helper()
	anterior := crearTemporal
	crearTemporal = f
	return func() { crearTemporal = anterior }
}
