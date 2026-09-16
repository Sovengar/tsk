package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsPageSize(t *testing.T) {
	if got := Defaults().ListPageSize; got != DefaultPageSize {
		t.Errorf("ListPageSize = %d, want %d", got, DefaultPageSize)
	}
}

func TestLoadPageSize(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"valor válido", "list_page_size = 25\n", 25},
		{"cero usa default", "list_page_size = 0\n", DefaultPageSize},
		{"negativo usa default", "list_page_size = -3\n", DefaultPageSize},
		{"sin fichero usa default", "", DefaultPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TSK_CONFIG", path)

			if got := Load().ListPageSize; got != tt.want {
				t.Errorf("Load().ListPageSize = %d, want %d", got, tt.want)
			}
		})
	}
}
