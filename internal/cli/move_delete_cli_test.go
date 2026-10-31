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

	// archived target: the move revives the project instead of being rejected
	idRevive := addTask(t, "api", "reviveme", "--status", "todo")
	runJSON(t, "project", "archive", "web")
	revived := runJSON(t, "update", strconv.FormatInt(idRevive, 10), "--project", "web")
	if task := revived["task"].(map[string]any); task["project"] != "web" {
		t.Errorf("project = %v, want web", task["project"])
	}
	if archived := runJSON(t, "project", "list", "--archived", "--json")["projects"].([]any); len(archived) != 0 {
		t.Errorf("archived list = %v, want web revived", archived)
	}
	if active := runJSON(t, "project", "list", "--json")["projects"].([]any); len(active) != 2 {
		t.Errorf("active list = %v, want api and web", active)
	}

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
