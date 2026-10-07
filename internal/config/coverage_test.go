package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Path has three routes and only the test-environment one was tested: the program's own
// environment variable and the XDG one.
func TestPathResolution(t *testing.T) {
	t.Run("TSK_CONFIG overrides everything", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "/mi/ruta/config.toml")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")
		t.Setenv("HOME", "/home/someone")

		got, err := Path()
		if err != nil {
			t.Fatalf("Path: %v", err)
		}
		if got != "/mi/ruta/config.toml" {
			t.Errorf("Path = %q, want the TSK_CONFIG one", got)
		}
	})

	t.Run("XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")

		got, err := Path()
		if err != nil {
			t.Fatalf("Path: %v", err)
		}
		if want := filepath.Join("/xdg", "tsk", FileName); got != want {
			t.Errorf("Path = %q, want %q", got, want)
		}
	})

	t.Run("HOME", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/someone")

		got, err := Path()
		if err != nil {
			t.Fatalf("Path: %v", err)
		}
		if want := filepath.Join("/home/someone", ".config", "tsk", FileName); got != want {
			t.Errorf("Path = %q, want %q", got, want)
		}
	})

	t.Run("without HOME", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		if _, err := Path(); err == nil {
			t.Error("Path without HOME nor XDG: want error")
		}
	})
}

// Load never fails: an unreadable config is a config that does not exist. It is the
// package policy -- "config never fails, defaults + warning" -- and what was
// not tested was exactly the case that justifies it.
func TestLoadNeverFails(t *testing.T) {
	t.Run("no file", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.toml"))

		cfg := Load()
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("ListPageSize = %d, want the default %d", cfg.ListPageSize, DefaultPageSize)
		}
		if cfg.DefaultEstimateDays != DefaultEstimateDays {
			t.Errorf("DefaultEstimateDays = %v, want the default %v",
				cfg.DefaultEstimateDays, DefaultEstimateDays)
		}
	})

	t.Run("malformed file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "roto.toml")
		if err := os.WriteFile(path, []byte("this = [is not toml"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", path)

		// Neither an error nor a panic: the defaults, period.
		cfg := Load()
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("with a broken TOML ListPageSize = %d, want the default %d",
				cfg.ListPageSize, DefaultPageSize)
		}
	})

	t.Run("no resolvable path", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		// Without Path() there is nowhere to read: defaults, with the database
		// path empty so the app resolves it on its own.
		cfg := Load()
		if cfg.Database.Path != "" {
			t.Errorf("Database.Path = %q, want empty when the path cannot be resolved",
				cfg.Database.Path)
		}
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("ListPageSize = %d, want the default %d", cfg.ListPageSize, DefaultPageSize)
		}
	})

	t.Run("impossible values", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte("list_page_size = -5\ndefault_estimate_days = -1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", path)

		// A negative page size would make the list show nothing, so
		// the default acts as the floor.
		cfg := Load()
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("ListPageSize = %d with a -5 in the file, want the default %d",
				cfg.ListPageSize, DefaultPageSize)
		}
		if cfg.DefaultEstimateDays != DefaultEstimateDays {
			t.Errorf("DefaultEstimateDays = %v with a -1 in the file, want the default %v",
				cfg.DefaultEstimateDays, DefaultEstimateDays)
		}
	})
}
