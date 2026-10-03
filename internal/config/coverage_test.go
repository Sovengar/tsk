package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Path tiene tres rutas y sólo se probaba la del entorno de pruebas: la variable
// de entorno propia del programa y la de XDG.
func TestPathResolution(t *testing.T) {
	t.Run("TSK_CONFIG manda sobre todo", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "/mi/ruta/config.toml")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")
		t.Setenv("HOME", "/home/alguien")

		got, err := Path()
		if err != nil {
			t.Fatalf("Path: %v", err)
		}
		if got != "/mi/ruta/config.toml" {
			t.Errorf("Path = %q, want la de TSK_CONFIG", got)
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
		t.Setenv("HOME", "/home/alguien")

		got, err := Path()
		if err != nil {
			t.Fatalf("Path: %v", err)
		}
		if want := filepath.Join("/home/alguien", ".config", "tsk", FileName); got != want {
			t.Errorf("Path = %q, want %q", got, want)
		}
	})

	t.Run("sin HOME", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		if _, err := Path(); err == nil {
			t.Error("Path sin HOME ni XDG: want error")
		}
	})
}

// Load nunca falla: un config ilegible es un config que no existe. Es la
// política del paquete -- "config nunca falla, defaults + warning" -- y lo que no
// se probaba era justo el caso que la justifica.
func TestLoadNeverFails(t *testing.T) {
	t.Run("sin fichero", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", filepath.Join(t.TempDir(), "no-existe.toml"))

		cfg := Load()
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("ListPageSize = %d, want el default %d", cfg.ListPageSize, DefaultPageSize)
		}
		if cfg.DefaultEstimateDays != DefaultEstimateDays {
			t.Errorf("DefaultEstimateDays = %v, want el default %v",
				cfg.DefaultEstimateDays, DefaultEstimateDays)
		}
	})

	t.Run("fichero malformado", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "roto.toml")
		if err := os.WriteFile(path, []byte("esto = [no es toml"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", path)

		// Ni un error ni un panic: los defaults, y punto.
		cfg := Load()
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("con un TOML roto ListPageSize = %d, want el default %d",
				cfg.ListPageSize, DefaultPageSize)
		}
	})

	t.Run("sin ruta resoluble", func(t *testing.T) {
		t.Setenv("TSK_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		// Sin Path() no hay dónde leer: defaults, y con la ruta de base de
		// datos vacía para que la app la resuelva por su cuenta.
		cfg := Load()
		if cfg.Database.Path != "" {
			t.Errorf("Database.Path = %q, want vacío si no se puede resolver la ruta",
				cfg.Database.Path)
		}
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("ListPageSize = %d, want el default %d", cfg.ListPageSize, DefaultPageSize)
		}
	})

	t.Run("valores imposibles", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte("list_page_size = -5\ndefault_estimate_days = -1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", path)

		// Un tamaño de página negativo haría que la lista no mostrara nada, así
		// que el default hace de suelo.
		cfg := Load()
		if cfg.ListPageSize != DefaultPageSize {
			t.Errorf("ListPageSize = %d con un -5 en el fichero, want el default %d",
				cfg.ListPageSize, DefaultPageSize)
		}
		if cfg.DefaultEstimateDays != DefaultEstimateDays {
			t.Errorf("DefaultEstimateDays = %v con un -1 en el fichero, want el default %v",
				cfg.DefaultEstimateDays, DefaultEstimateDays)
		}
	})
}
