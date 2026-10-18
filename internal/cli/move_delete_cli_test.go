package cli

import (
	"strconv"
	"testing"
)

func TestUpdateProjectMovesTask(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	runJSON(t, "project", "add", "web", "--workflow", "todo,doing,done")

	// status rejected: default task is in "backlog", web workflow lacks it
	id := addTask(t, "api", "moveme")
	wantError(t, "update", strconv.FormatInt(id, 10), "--project", "web")

	// unknown project
	wantError(t, "update", strconv.FormatInt(id, 10), "--project", "nope")

	// archived target rejected
	runJSON(t, "project", "archive", "web")
	wantError(t, "update", strconv.FormatInt(id, 10), "--project", "web")
	runJSON(t, "project", "unarchive", "web")

	// successful move: task already in a status present in web's workflow
	id2 := addTask(t, "api", "moveme2", "--status", "todo")
	payload := runJSON(t, "update", strconv.FormatInt(id2, 10), "--project", "web")
	task := payload["task"].(map[string]any)
	if task["project"] != "web" {
		t.Errorf("project = %v, want web", task["project"])
	}
	if task["status"] != "todo" {
		t.Errorf("status = %v, want todo", task["status"])
	}
}

func TestDeleteTaskCLI(t *testing.T) {
	withTempDB(t)
	seed(t, "api")
	id := addTask(t, "api", "todelete")

	// without --force: cancelled (stdin is empty in tests)
	wantError(t, "delete", strconv.FormatInt(id, 10))

	// with --force: deleted
	payload := runJSON(t, "delete", strconv.FormatInt(id, 10), "--force")
	if payload["ok"] != true {
		t.Fatalf("delete payload = %v", payload)
	}

	// afterwards show errors
	wantError(t, "show", strconv.FormatInt(id, 10))
}
