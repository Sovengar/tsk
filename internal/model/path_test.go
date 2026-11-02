package model

import (
	"path/filepath"
	"testing"
)

func TestResolveDirPrefersTheTaskPath(t *testing.T) {
	if got := ResolveDir("/task/dir", "/project/dir"); got != "/task/dir" {
		t.Errorf("ResolveDir = %q, want the task path", got)
	}
	if got := ResolveDir("", "/project/dir"); got != "/project/dir" {
		t.Errorf("ResolveDir = %q, want the project path", got)
	}
	if got := ResolveDir("  ", "  "); got != "" {
		t.Errorf("ResolveDir with blanks = %q, want empty", got)
	}
}

func TestResolveDirExpandsHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	tests := []struct {
		task, project, want string
	}{
		{"~", "", "/home/tester"},
		{"~/src/web", "", "/home/tester/src/web"},
		{"", "~/src/api", "/home/tester/src/api"},
		{"~other/dir", "", "~other/dir"},
	}
	for _, tc := range tests {
		if got := ResolveDir(tc.task, tc.project); got != tc.want {
			t.Errorf("ResolveDir(%q, %q) = %q, want %q", tc.task, tc.project, got, tc.want)
		}
	}
}

func TestEffectiveDirPrefersTheResolver(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	if got := EffectiveDir("/task", ""); got != "/task" {
		t.Errorf("EffectiveDir = %q, want the task path without the fallback", got)
	}
}

func TestEffectiveDirFallsBackToTasks(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	want := filepath.Join("/home/tester", DefaultDirName)
	if got := EffectiveDir("", ""); got != want {
		t.Errorf("EffectiveDir = %q, want %q", got, want)
	}
}

func TestEffectiveDirWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	if got := EffectiveDir("", ""); got != "." {
		t.Errorf("EffectiveDir without HOME = %q, want .", got)
	}
	// A tilde that cannot be expanded is kept as written.
	if got := ResolveDir("~/x", ""); got != "~/x" {
		t.Errorf("ResolveDir without HOME = %q, want the literal path", got)
	}
}
