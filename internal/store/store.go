// Package store implements SQLite-backed persistence for loopline,
// satisfying the app.Store contract declared in internal/app/ports.go.
//
// This package depends only on the Go standard library, domain and the
// modernc.org/sqlite driver. It never imports internal/app.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLite is a SQLite-backed implementation of app.Store.
type SQLite struct {
	db *sql.DB
}

// DefaultPath resolves loopline's XDG data location:
// $XDG_DATA_HOME/loopline/loopline.db, falling back to
// ~/.local/share/loopline/loopline.db. The directory is created with 0o700.
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "loopline")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data directory %s: %w", dir, err)
	}
	return filepath.Join(dir, "loopline.db"), nil
}

// pragmaDSN carries the connection-local PRAGMAs every physical connection
// must have. They are set via DSN query parameters rather than a one-off
// db.Exec against the pool: database/sql may silently discard and replace a
// physical connection after a driver.ErrBadConn, and the replacement would
// not inherit a PRAGMA set only on the old connection, quietly turning off
// foreign-key enforcement. journal_mode is a database-level property, not
// connection-local, so it is set once via db.Exec in open instead.
const pragmaDSN = "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

// Open opens (creating if necessary) the SQLite database at path, applies
// pragmas and runs any pending migrations. The caller is responsible for
// ensuring the parent directory exists; see DefaultPath.
func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path+pragmaDSN)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return open(db)
}

// OpenMemory opens a private in-memory database. Intended for tests.
func OpenMemory() (*SQLite, error) {
	db, err := sql.Open("sqlite", ":memory:"+pragmaDSN)
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}
	return open(db)
}

func open(db *sql.DB) (*SQLite, error) {
	// modernc.org/sqlite connections are not safe for concurrent use from
	// multiple goroutines against the same handle; pinning the pool to a
	// single physical connection also keeps :memory: databases coherent
	// across calls, since each new connection to ":memory:" would otherwise
	// start out as its own empty database.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set pragma journal_mode: %w", err)
	}

	if err := migrate(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLite{db: db}, nil
}

// Close releases the underlying database handle.
func (s *SQLite) Close() error {
	return s.db.Close()
}
