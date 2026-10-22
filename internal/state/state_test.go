package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathUsesXDGStateHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TSK_STATE", "")
	t.Setenv("XDG_STATE_HOME", dir)

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "tsk", fileName)
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("TSK_STATE", path)

	if err := Save(State{Project: "api"}); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got.Project != "api" {
		t.Errorf("Load().Project = %q, want api", got.Project)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file was not written: %v", err)
	}
}

func TestLoadFallsBackForMissingAndMalformedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("TSK_STATE", path)
	if got := Load(); got != (State{}) {
		t.Errorf("missing state = %#v, want zero state", got)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got != (State{}) {
		t.Errorf("malformed state = %#v, want zero state", got)
	}
}
