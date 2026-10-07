package tui

import (
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"tsk/internal/model"
)

// commentFinishedMsg is sent when the comment editor finishes.
type commentFinishedMsg struct {
	err    error
	taskID int64
	body   string
}

// commentsLoadedMsg carries the comment list of a task.
type commentsLoadedMsg struct {
	taskID    int64
	comments  []model.Comment
	selectIdx int // index to select after reload (-1 = none)
}

// createTemp is a variable and not a direct call to os.CreateTemp so that
// a test can return a temp file that fails when written. The write
// itself is injectable (writeAndClose takes an interface), but without this
// the caller cannot be tested: creating a real temp file never fails on
// write, so its error branch has no other path.
var createTemp = func(dir, pattern string) (tempFile, error) {
	return os.CreateTemp(dir, pattern)
}

// commentCmd opens the external editor to write a new comment.
func commentCmd(taskID int64, editorCmd string) tea.Cmd {
	tmpFile, err := createTemp("", "tsk-comment-*.md")
	if err != nil {
		return func() tea.Msg {
			return commentFinishedMsg{err: err, taskID: taskID}
		}
	}

	if err := writeAndClose(tmpFile, ""); err != nil {
		return func() tea.Msg {
			return commentFinishedMsg{err: err, taskID: taskID}
		}
	}

	args := strings.Fields(editorCmd)
	cmd := exec.Command(args[0], append(args[1:], tmpFile.Name())...)

	return tea.ExecProcess(cmd, commentCallback(taskID, tmpFile.Name()))
}

// tempFile is the minimum that writeAndClose needs from a file.
// It exists as an interface and not as *os.File so a test can pass a file
// that fails: write and close errors cannot be provoked
// any other way -- they would need a full disk or a double close -- and
// without this injection point those two branches have no possible test.
type tempFile interface {
	Write(p []byte) (int, error)
	Close() error
	Name() string
}

// writeAndClose writes the content to the temp file and closes it. If
// anything fails, it deletes the file: a half-written temp file in /tmp is
// trash that stays there forever, and the editor is going to open the name we give it.
//
// The comment passes the empty string because it carries no template: it opens
// empty and the person writes it. The path is the same as the task's so that
// the two share the half that fails.
func writeAndClose(f tempFile, content string) error {
	if _, err := f.Write([]byte(content)); err != nil {
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

// commentCallback is the tail of commentCmd, outside of it because tea.ExecProcess
// swallows the function inside a private message: from a test you cannot
// reach the body of a closure. By taking it out, the three outcomes of the
// editor -- it fails, the file is gone, it wrote something -- are checked directly.
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

// readEditedFile reads the temp file the editor has just edited and deletes
// it, whether the editor worked or not.
//
// It is extracted from commentCmd and editTaskCmd because both do exactly this,
// and what matters about their result -- what happens if the editor fails, if the
// file is gone, if the editor wrote nothing -- does not need a Bubbletea around
// it to be checked.
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

// loadCommentsCmd loads a task's comments without selecting any of them.
func (m *Model) loadCommentsCmd(taskID int64) tea.Cmd {
	return func() tea.Msg {
		comments, err := m.database.ListComments(taskID)
		if err != nil {
			return commentsLoadedMsg{taskID: taskID, selectIdx: -1}
		}
		return commentsLoadedMsg{taskID: taskID, comments: comments, selectIdx: -1}
	}
}

// addCommentCmd creates a comment and reloads the list selecting the new one.
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

// deleteCommentCmd deletes a comment and reloads the list keeping a
// valid selection (the one taking its place, or the previous one if it was the last).
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
