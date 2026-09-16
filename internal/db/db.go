package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"
)

// DB envuelve la conexión SQLite con helpers.
type DB struct {
	conn *sql.DB
}

// Open abre la base de datos en la ruta dada, habilita WAL y ejecuta
// las migraciones pendientes.
func Open(path string) (*DB, error) {
	if path == ":memory:" {
		conn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(ON)")
		if err != nil {
			return nil, fmt.Errorf("open db: %w", err)
		}
		db := &DB{conn: conn}
		if err := db.migrate(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		return db, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// OpenMemory abre una base de datos en memoria (para tests).
func OpenMemory() (*DB, error) {
	return Open(":memory:")
}

// NewTestDB crea una DB en memoria para tests.
func NewTestDB() (*DB, error) {
	return OpenMemory()
}

// Close cierra la conexión.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn devuelve la conexión sql.DB subyacente.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// DefaultPath devuelve la ruta por defecto de la base de datos: ~/.local/share/taskd/taskd.db
func DefaultPath() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "taskd", "taskd.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "taskd", "taskd.db"), nil
}

// migrate ejecuta las migraciones pendientes.
func (db *DB) migrate() error {
	// Crear tabla _meta si no existe
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS _meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}

	// Obtener versión actual
	var currentVersion int
	row := db.conn.QueryRow(`SELECT value FROM _meta WHERE key = 'schema_version'`)
	if err := row.Scan(&currentVersion); err != nil {
		currentVersion = 0 // sin versión → primera migración
	}

	// Ejecutar migraciones pendientes
	for _, m := range migrations {
		v, _ := strconv.Atoi(m.version)
		if v <= currentVersion {
			continue
		}
		if _, err := db.conn.Exec(m.query); err != nil {
			return fmt.Errorf("migration %s: %w", m.version, err)
		}
		if _, err := db.conn.Exec(`INSERT OR REPLACE INTO _meta (key, value) VALUES ('schema_version', ?)`, m.version); err != nil {
			return fmt.Errorf("update version: %w", err)
		}
	}

	return nil
}
