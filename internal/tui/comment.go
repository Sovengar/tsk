package tui

import (
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// commentFinishedMsg se envía cuando el editor de comentarios termina.
type commentFinishedMsg struct {
	err    error
	taskID int64
	body   string
}

// commentsLoadedMsg lleva la lista de comentarios de una tarea.
type commentsLoadedMsg struct {
	taskID    int64
	comments  []model.Comment
	selectIdx int // índice a seleccionar tras recargar (-1 = ninguno)
}

// crearTemporal es una variable y no una llamada directa a os.CreateTemp para
// que un test pueda devolver un temporal que falla al escribirse. La escritura
// sí es inyectable (escribirYcerrar toma una interfaz), pero sin esto el
// llamador no se puede probar: crear el temporal de verdad nunca falla al
// escribir, así que su rama de error no tiene otro camino.
var crearTemporal = func(dir, pattern string) (archivoTemporal, error) {
	return os.CreateTemp(dir, pattern)
}

// commentCmd abre el editor externo para escribir un comentario nuevo.
func commentCmd(taskID int64, editorCmd string) tea.Cmd {
	tmpFile, err := crearTemporal("", "tsk-comment-*.md")
	if err != nil {
		return func() tea.Msg {
			return commentFinishedMsg{err: err, taskID: taskID}
		}
	}

	if err := escribirYcerrar(tmpFile, ""); err != nil {
		return func() tea.Msg {
			return commentFinishedMsg{err: err, taskID: taskID}
		}
	}

	args := strings.Fields(editorCmd)
	cmd := exec.Command(args[0], append(args[1:], tmpFile.Name())...)

	return tea.ExecProcess(cmd, commentCallback(taskID, tmpFile.Name()))
}

// archivoTemporal es lo mínimo que escribirYcerrar necesita de un fichero.
// Existe como interfaz y no como *os.File para que un test pueda pasar un
// fichero que falla: los errores de escritura y de cierre no se pueden provocar
// de otra forma -- harían falta un disco lleno o un doble cierre -- y sin este
// punto de inyección esas dos ramas no tienen test posible.
type archivoTemporal interface {
	Write(p []byte) (int, error)
	Close() error
	Name() string
}

// escribirYcerrar escribe el contenido en el temporal y lo cierra. Si algo
// falla, borra el fichero: un temporal a medias en /tmp es basura que se queda
// ahí para siempre, y el editor va a abrir el nombre que le demos.
//
// El comentario pasa la cadena vacía porque no lleva plantilla: se abre vacío
// y lo escribe la persona. La ruta es la misma que la de la tarea para que las
// dos compartan la mitad que falla.
func escribirYcerrar(f archivoTemporal, contenido string) error {
	if _, err := f.Write([]byte(contenido)); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return nil
}

// commentCallback es el final de commentCmd, fuera de él porque tea.ExecProcess
// se traga la función dentro de un mensaje privado: desde un test no se puede
// llegar al cuerpo de un cierre. Sacándolo, los tres desenlaces del editor --
// falla, el fichero ya no está, escribió algo -- se comprueban directamente.
func commentCallback(taskID int64, path string) tea.ExecCallback {
	return func(execErr error) tea.Msg {
		body, err := readEditedFile(path, execErr)
		if err != nil {
			return commentFinishedMsg{err: err, taskID: taskID}
		}
		return commentFinishedMsg{
			taskID: taskID,
			body:   strings.TrimSpace(body),
		}
	}
}

// readEditedFile lee el fichero temporal que el editor acaba de editar y lo
// borra, tanto si el editor funcionó como si no.
//
// Se extrae de commentCmd y editTaskCmd porque los dos hacen exactamente esto y
// lo que importa de su resultado -- qué pasa si el editor falla, si el fichero ya
// no está, si el editor no escribió nada -- no necesita un Bubbletea alrededor
// para comprobarse.
func readEditedFile(path string, execErr error) (string, error) {
	if execErr != nil {
		_ = os.Remove(path)
		return "", execErr
	}

	data, err := os.ReadFile(path)
	_ = os.Remove(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadCommentsCmd carga los comentarios de una tarea sin seleccionar ninguno.
func (m *Model) loadCommentsCmd(taskID int64) tea.Cmd {
	return func() tea.Msg {
		comments, err := m.database.ListComments(taskID)
		if err != nil {
			return commentsLoadedMsg{taskID: taskID, selectIdx: -1}
		}
		return commentsLoadedMsg{taskID: taskID, comments: comments, selectIdx: -1}
	}
}

// addCommentCmd crea un comentario y recarga la lista seleccionando el nuevo.
func (m *Model) addCommentCmd(taskID int64, body string) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.database.AddComment(taskID, body); err != nil {
			return commentsLoadedMsg{taskID: taskID, selectIdx: -1}
		}
		comments, err := m.database.ListComments(taskID)
		if err != nil {
			return commentsLoadedMsg{taskID: taskID, selectIdx: -1}
		}
		return commentsLoadedMsg{taskID: taskID, comments: comments, selectIdx: len(comments) - 1}
	}
}

// deleteCommentCmd elimina un comentario y recarga la lista manteniendo una
// selección válida (el que ocupaba su lugar, o el anterior si era el último).
func (m *Model) deleteCommentCmd(taskID, commentID int64, sel int) tea.Cmd {
	return func() tea.Msg {
		if err := m.database.DeleteComment(commentID); err != nil {
			return commentsLoadedMsg{taskID: taskID, selectIdx: sel}
		}
		comments, err := m.database.ListComments(taskID)
		if err != nil {
			return commentsLoadedMsg{taskID: taskID, selectIdx: -1}
		}
		next := sel
		if next >= len(comments) {
			next = len(comments) - 1
		}
		return commentsLoadedMsg{taskID: taskID, comments: comments, selectIdx: next}
	}
}
