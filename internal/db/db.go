package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection with helpers.
type DB struct {
	conn *sql.DB
}

// Open opens the database at the given path, enables WAL and runs
// the pending migrations.
func Open(path string) (*DB, error) {
	if path == ":memory:" {
		// The sql.Open error is discarded on purpose, not by oversight: this
		// driver does not implement driver.DriverContext, so sql.Open can only
		// fail with an unregistered driver, which is not the case. The DSN is
		// validated on the first connection, and that is the one migrate() makes, which
		// does return an error.
		conn, _ := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(ON)")
		db := &DB{conn: conn}
		if err := db.migrate(); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		return db, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	conn, _ := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// OpenMemory opens an in-memory database (for tests).
func OpenMemory() (*DB, error) {
	return Open(":memory:")
}

// NewTestDB creates an in-memory DB for tests.
func NewTestDB() (*DB, error) {
	return OpenMemory()
}

// Close closes the connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB connection.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// DefaultPath returns the default database path: ~/.local/share/tsk/tsk.db
func DefaultPath() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "tsk", "tsk.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "tsk", "tsk.db"), nil
}

// migrate runs the pending migrations.
func (db *DB) migrate() error {
	// Create _meta table if it does not exist
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS _meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}

	// Get current version
	var currentVersion int
	row := db.conn.QueryRow(`SELECT value FROM _meta WHERE key = 'schema_version'`)
	if err := row.Scan(&currentVersion); err != nil {
		currentVersion = 0 // no version → first migration
	}

	// Run pending migrations
	for _, m := range migrations {
		v, _ := strconv.Atoi(m.version)
		if v <= currentVersion {
			continue
		}
		if m.run != nil {
			if err := m.run(db); err != nil {
				return fmt.Errorf("migration %s: %w", m.version, err)
			}
		} else if _, err := db.conn.Exec(m.query); err != nil {
			return fmt.Errorf("migration %s: %w", m.version, err)
		}
		if _, err := db.conn.Exec(`INSERT OR REPLACE INTO _meta (key, value) VALUES ('schema_version', ?)`, m.version); err != nil {
			return fmt.Errorf("update version: %w", err)
		}
	}

	return nil
}
