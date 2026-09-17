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

// commentCmd abre el editor externo para escribir un comentario nuevo.
func commentCmd(taskID int64, editorCmd string) tea.Cmd {
	tmpFile, err := os.CreateTemp("", "tsk-comment-*.md")
	if err != nil {
		return func() tea.Msg {
			return commentFinishedMsg{err: err, taskID: taskID}
		}
	}

	tmpFile.Close()

	args := strings.Fields(editorCmd)
	cmd := exec.Command(args[0], append(args[1:], tmpFile.Name())...)

	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		if execErr != nil {
			os.Remove(tmpFile.Name())
			return commentFinishedMsg{err: execErr, taskID: taskID}
		}
		data, readErr := os.ReadFile(tmpFile.Name())
		os.Remove(tmpFile.Name())
		if readErr != nil {
			return commentFinishedMsg{err: readErr, taskID: taskID}
		}
		return commentFinishedMsg{
			taskID: taskID,
			body:   strings.TrimSpace(string(data)),
		}
	})
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
