package harness

import (
	"os"
	"path/filepath"
	"testing"

	"tsk/internal/config"
	"tsk/internal/model"
)

func TestResolveWorkingDirOrder(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	cfg := config.Defaults()

	if got := ResolveWorkingDir(cfg, "/task/dir", "/project/dir"); got != "/task/dir" {
		t.Errorf("ResolveWorkingDir = %q, want the task path", got)
	}
	if got := ResolveWorkingDir(cfg, "", "/project/dir"); got != "/project/dir" {
		t.Errorf("ResolveWorkingDir = %q, want the project path", got)
	}

	cfg.Handoff.WorkingDir = "/configured"
	if got := ResolveWorkingDir(cfg, "", ""); got != "/configured" {
		t.Errorf("ResolveWorkingDir = %q, want the configured dir", got)
	}

	cfg.Handoff.WorkingDir = ""
	want := "/home/tester"
	if got := ResolveWorkingDir(cfg, "", ""); got != want {
		t.Errorf("ResolveWorkingDir = %q, want %q", got, want)
	}
}

func TestRealRunDetachedCreatesTheDefaultDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, model.DefaultDirName)

	if err := realRunDetached("/bin/sh", []string{"-c", "exit 0"}, dir); err != nil {
		t.Fatalf("realRunDetached = %v, want nil", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("the default dir was not created: %v", err)
	}
}

func TestRealRunDetachedReportsAnUncreatableDefaultDir(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", blocker)

	if err := realRunDetached("/bin/sh", []string{"-c", "exit 0"}, filepath.Join(blocker, model.DefaultDirName)); err == nil {
		t.Fatal("realRunDetached with an uncreatable default dir = nil, want an error")
	}
}

func TestRealRunDetachedExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := realRunDetached("/bin/sh", []string{"-c", "exit 0"}, "~"); err != nil {
		t.Fatalf("realRunDetached with ~ = %v, want nil", err)
	}
}
