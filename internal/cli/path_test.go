package cli

import (
	"encoding/json"
	"strconv"
	"testing"

	"tsk/internal/model"
)

func TestAddWithTaskPath(t *testing.T) {
	withTempDB(t)
	if out, code := run(t, "project", "add", "api"); code != 0 {
		t.Fatalf("project add: %s", out)
	}

	out, code := run(t, "add", "T", "--project", "api", "--path", "/work/dir")
	if code != 0 {
		t.Fatalf("add --path exited with %d: %s", code, out)
	}
	var resp model.TaskResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Task.Path != "/work/dir" {
		t.Errorf("task path = %q, want /work/dir", resp.Task.Path)
	}
}

func TestUpdateTaskPath(t *testing.T) {
	withTempDB(t)
	if out, code := run(t, "project", "add", "api"); code != 0 {
		t.Fatalf("project add: %s", out)
	}
	out, code := run(t, "add", "T", "--project", "api")
	if code != 0 {
		t.Fatalf("add: %s", out)
	}
	var created model.TaskResponse
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(created.Task.ID, 10)

	out, code = run(t, "update", id, "--path", "/work/dir")
	if code != 0 {
		t.Fatalf("update --path exited with %d: %s", code, out)
	}
	var updated model.TaskResponse
	if err := json.Unmarshal([]byte(out), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Task.Path != "/work/dir" {
		t.Errorf("updated path = %q, want /work/dir", updated.Task.Path)
	}

	out, code = run(t, "update", id, "--path", "")
	if code != 0 {
		t.Fatalf("update --path '' exited with %d: %s", code, out)
	}
	var cleared model.TaskResponse
	if err := json.Unmarshal([]byte(out), &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.Task.Path != "" {
		t.Errorf("cleared path = %q, want empty", cleared.Task.Path)
	}
}
